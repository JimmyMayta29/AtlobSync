package cpe

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const DefaultSunatCPEBaseURL = "https://api-cpe.sunat.gob.pe/v1/contribuyente/consultacpe/comprobantes"

var (
	// ErrSolTokenExpired indica HTTP 401: el JWT SUNAT.token expiró y debe renovarse.
	ErrSolTokenExpired = errors.New("token SOL de SUNAT expirado (HTTP 401)")
	// ErrComprobanteNoDisponible indica que SUNAT respondió que no existe archivo disponible (ej. codError 113 o 404).
	ErrComprobanteNoDisponible = errors.New("archivo no disponible en SUNAT para este comprobante")
	// ErrSunatSolicitudRechazada indica error permanente de validación (HTTP 4xx distinto de 401/404/113).
	ErrSunatSolicitudRechazada = errors.New("SUNAT rechazó la solicitud del comprobante")
	// ErrSunatTransitorio indica error intermitente de red o HTTP 5xx (reintentable).
	ErrSunatTransitorio = errors.New("error transitorio de comunicación con SUNAT")
)

// SunatCPEDownloadResult contiene el archivo principal extraído (XML o PDF) y opcionalmente un CDR embebido.
type SunatCPEDownloadResult struct {
	Data        []byte
	FileName    string
	EmbeddedCDR []byte
	CDRFileName string
}

// SunatCPEClient consume la API REST de Consulta de Comprobantes de SUNAT (api-cpe.sunat.gob.pe)
// autenticada con el JWT Bearer capturado desde el Menú SOL (módulo 11.38.1.1.1).
type SunatCPEClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewSunatCPEClient() *SunatCPEClient {
	return &SunatCPEClient{
		baseURL: DefaultSunatCPEBaseURL,
		httpClient: &http.Client{
			Timeout: 35 * time.Second,
		},
	}
}

// SetBaseURL permite sobreescribir el endpoint base (útil para tests unitarios con httptest).
func (c *SunatCPEClient) SetBaseURL(u string) {
	if strings.TrimSpace(u) != "" {
		c.baseURL = strings.TrimRight(strings.TrimSpace(u), "/")
	}
}

func (c *SunatCPEClient) DownloadPDF(ctx context.Context, token, rucAuth, rucEmisor, tipoCP, serie, numero string) (*SunatCPEDownloadResult, error) {
	return c.download(ctx, token, rucAuth, rucEmisor, tipoCP, serie, numero, "01", ".pdf")
}

func (c *SunatCPEClient) DownloadXML(ctx context.Context, token, rucAuth, rucEmisor, tipoCP, serie, numero string) (*SunatCPEDownloadResult, error) {
	return c.download(ctx, token, rucAuth, rucEmisor, tipoCP, serie, numero, "02", ".xml")
}

// DownloadCDR descarga la Constancia de Recepción (CDR) oficial desde api-cpe.sunat.gob.pe (sufijo 03).
// Funciona tanto para comprobantes emitidos (-1) como recibidos de proveedores (-2).
func (c *SunatCPEClient) DownloadCDR(ctx context.Context, token, rucAuth, rucEmisor, tipoCP, serie, numero string) (*SunatCPEDownloadResult, error) {
	return c.download(ctx, token, rucAuth, rucEmisor, tipoCP, serie, numero, "03", ".zip")
}

func (c *SunatCPEClient) download(ctx context.Context, token, rucAuth, rucEmisor, tipoCP, serie, numero, sufijo, wantExt string) (*SunatCPEDownloadResult, error) {
	rucEmisor = strings.TrimSpace(rucEmisor)
	rucAuth = strings.TrimSpace(rucAuth)
	if rucAuth == "" {
		rucAuth = ExtractRUCFromJWT(token)
	}
	tipoCP = padTipoCP(tipoCP)
	serie = strings.ToUpper(strings.TrimSpace(serie))
	correlativo := normalizeCorrelativo(numero)

	if len(rucEmisor) != 11 || tipoCP == "" || serie == "" || correlativo == "" {
		return nil, fmt.Errorf("%w: parámetros incompletos (ruc=%s tipo=%s serie=%s num=%s)",
			ErrSunatSolicitudRechazada, rucEmisor, tipoCP, serie, numero)
	}

	// Modo 1 = emitido por el RUC autenticado; Modo 2 = recibido de proveedor
	modos := []string{"2", "1"}
	if rucAuth != "" {
		if rucEmisor == rucAuth {
			modos = []string{"1", "2"}
		} else {
			modos = []string{"2", "1"}
		}
	}

	llave := fmt.Sprintf("%s-%s-%s-%s", rucEmisor, tipoCP, serie, correlativo)
	var lastErr error

	for idx, modo := range modos {
		reqURL := fmt.Sprintf("%s/%s-%s/%s", c.baseURL, llave, modo, sufijo)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("error preparando petición SUNAT CPE: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", defaultSolUserAgent)
		req.Header.Set("Origin", "https://e-factura.sunat.gob.pe")
		req.Header.Set("Referer", "https://e-factura.sunat.gob.pe/")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrSunatTransitorio, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("%w: error leyendo respuesta SUNAT: %v", ErrSunatTransitorio, readErr)
		}

		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrSolTokenExpired
		}

		if resp.StatusCode == http.StatusNotFound {
			if idx+1 < len(modos) && rucAuth == "" {
				continue
			}
			return nil, fmt.Errorf("%w (%s)", ErrComprobanteNoDisponible, llave)
		}

		if resp.StatusCode == http.StatusUnprocessableEntity {
			codigos, detalle := parseSunatCPEErrors(body)
			// 105: el emisor corresponde al otro modo (emitido vs recibido)
			if codigos["105"] && idx+1 < len(modos) {
				lastErr = fmt.Errorf("SUNAT 422 (%s): %s", llave, detalle)
				continue
			}
			// 113: no hay resultados para la consulta (permanente)
			if codigos["113"] {
				if idx+1 < len(modos) && rucAuth == "" {
					continue
				}
				return nil, fmt.Errorf("%w (%s): %s", ErrComprobanteNoDisponible, llave, detalle)
			}
			return nil, fmt.Errorf("%w (%s): %s", ErrSunatSolicitudRechazada, llave, detalle)
		}

		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("%w (HTTP %d para %s)", ErrSunatTransitorio, resp.StatusCode, llave)
		}

		if resp.StatusCode != http.StatusOK {
			snippet := strings.TrimSpace(string(body))
			if len(snippet) > 220 {
				snippet = snippet[:220] + "..."
			}
			return nil, fmt.Errorf("%w (HTTP %d para %s): %s", ErrSunatSolicitudRechazada, resp.StatusCode, llave, snippet)
		}

		// Parsear JSON { nomArchivo / nombreArchivo, valArchivo / valorArchivo }
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("%w: respuesta SUNAT no es JSON válido para %s", ErrSunatTransitorio, llave)
		}

		b64 := firstNonEmptyString(payload, "valArchivo", "valorArchivo", "archivo", "contenido", "content", "data")
		if b64 == "" {
			return nil, fmt.Errorf("%w (%s): SUNAT devolvió valArchivo vacío", ErrComprobanteNoDisponible, llave)
		}

		rawBytes, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.TrimSpace(b64), "\n", ""))
		if err != nil {
			return nil, fmt.Errorf("%w: base64 inválido para %s: %v", ErrSunatTransitorio, llave, err)
		}

		nomArchivo := firstNonEmptyString(payload, "nomArchivo", "nombreArchivo", "fileName", "filename")
		defaultName := fmt.Sprintf("%s%s", llave, wantExt)
		if nomArchivo == "" {
			nomArchivo = defaultName
		}

		res, err := extractSunatPayload(rawBytes, wantExt, defaultName)
		if err != nil {
			return nil, fmt.Errorf("%w (%s): %v", ErrSunatTransitorio, llave, err)
		}
		return res, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrSunatSolicitudRechazada, lastErr)
	}
	return nil, fmt.Errorf("%w (%s)", ErrComprobanteNoDisponible, llave)
}

func extractSunatPayload(raw []byte, wantExt, defaultName string) (*SunatCPEDownloadResult, error) {
	// Para CDR (/03), conservar el ZIP oficial intacto (contiene R-*.xml firmado por SUNAT)
	if wantExt == ".zip" {
		if isZipBytes(raw) {
			return &SunatCPEDownloadResult{
				Data:     raw,
				FileName: defaultName,
			}, nil
		}
		trimmed := bytes.TrimLeft(raw, "\xef\xbb\xbf \r\n\t")
		if bytes.HasPrefix(bytes.ToLower(trimmed), []byte("<?xml")) || bytes.Contains(trimmed[:minInt(len(trimmed), 256)], []byte("ApplicationResponse")) {
			return &SunatCPEDownloadResult{
				Data:     trimmed,
				FileName: strings.TrimSuffix(defaultName, ".zip") + ".xml",
			}, nil
		}
		return nil, errors.New("el contenido CDR recibido de SUNAT no es un ZIP ni XML válido")
	}

	trimmed := bytes.TrimLeft(raw, "\xef\xbb\xbf \r\n\t")
	// Algunos comprobantes llegan directos sin ZIP
	if (wantExt == ".xml" && bytes.HasPrefix(bytes.ToLower(trimmed), []byte("<?xml"))) ||
		(wantExt == ".pdf" && bytes.HasPrefix(trimmed, []byte("%PDF-"))) {
		return &SunatCPEDownloadResult{
			Data:     trimmed,
			FileName: defaultName,
		}, nil
	}

	// Si no comienza como ZIP, verificar si es XML sin cabecera <?xml
	if !isZipBytes(raw) {
		if wantExt == ".xml" && (bytes.Contains(trimmed[:minInt(len(trimmed), 256)], []byte("<Invoice")) ||
			bytes.Contains(trimmed[:minInt(len(trimmed), 256)], []byte("<CreditNote")) ||
			bytes.Contains(trimmed[:minInt(len(trimmed), 256)], []byte("<DebitNote"))) {
			return &SunatCPEDownloadResult{
				Data:     trimmed,
				FileName: defaultName,
			}, nil
		}
		return nil, errors.New("el contenido recibido de SUNAT no es un ZIP ni un archivo válido")
	}

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("ZIP inválido recibido de SUNAT: %w", err)
	}

	var bestPrimary *zip.File
	var bestCDR *zip.File

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		lower := strings.ToLower(base)
		if strings.HasPrefix(lower, "dummy") {
			continue
		}

		if wantExt == ".xml" && strings.HasSuffix(lower, ".xml") {
			if strings.HasPrefix(strings.ToUpper(base), "R-") {
				if bestCDR == nil || f.UncompressedSize64 > bestCDR.UncompressedSize64 {
					bestCDR = f
				}
				continue
			}
			if bestPrimary == nil || f.UncompressedSize64 > bestPrimary.UncompressedSize64 {
				bestPrimary = f
			}
		} else if strings.HasSuffix(lower, wantExt) {
			if bestPrimary == nil || f.UncompressedSize64 > bestPrimary.UncompressedSize64 {
				bestPrimary = f
			}
		}
	}

	// Fallback: primer archivo no-directorio si ninguno coincidió por extensión
	if bestPrimary == nil {
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			if bestPrimary == nil || f.UncompressedSize64 > bestPrimary.UncompressedSize64 {
				bestPrimary = f
			}
		}
	}
	if bestPrimary == nil {
		return nil, errors.New("el ZIP devuelto por SUNAT está vacío")
	}

	primaryBytes, err := readZipEntry(bestPrimary)
	if err != nil {
		return nil, err
	}
	outName := filepath.Base(bestPrimary.Name)
	if !strings.HasSuffix(strings.ToLower(outName), wantExt) {
		outName = defaultName
	}

	res := &SunatCPEDownloadResult{
		Data:     primaryBytes,
		FileName: outName,
	}

	if bestCDR != nil {
		if cdrBytes, cdrErr := readZipEntry(bestCDR); cdrErr == nil && len(cdrBytes) > 0 {
			res.EmbeddedCDR = cdrBytes
			res.CDRFileName = filepath.Base(bestCDR.Name)
		}
	}

	return res, nil
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func parseSunatCPEErrors(body []byte) (map[string]bool, string) {
	codigos := make(map[string]bool)
	var data struct {
		Msg    string `json:"msg"`
		Cod    string `json:"cod"`
		Errors []struct {
			CodError interface{} `json:"codError"`
			DesError string      `json:"desError"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		s := strings.TrimSpace(string(body))
		if len(s) > 250 {
			s = s[:250] + "..."
		}
		return codigos, s
	}

	var parts []string
	if data.Cod != "" {
		codigos[strings.TrimSpace(data.Cod)] = true
	}
	for _, e := range data.Errors {
		codeStr := strings.TrimSpace(fmt.Sprintf("%v", e.CodError))
		if codeStr != "" && codeStr != "<nil>" {
			codigos[codeStr] = true
		}
		if e.DesError != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", codeStr, e.DesError))
		}
	}
	if len(parts) == 0 && data.Msg != "" {
		parts = append(parts, data.Msg)
	}
	if len(parts) == 0 {
		s := strings.TrimSpace(string(body))
		if len(s) > 250 {
			s = s[:250] + "..."
		}
		return codigos, s
	}
	return codigos, strings.Join(parts, "; ")
}

func firstNonEmptyString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func normalizeCorrelativo(num string) string {
	s := strings.TrimSpace(num)
	if s == "" {
		return ""
	}
	if reDigitsOnly.MatchString(s) {
		trimmed := strings.TrimLeft(s, "0")
		if trimmed == "" {
			return "0"
		}
		return trimmed
	}
	return s
}
