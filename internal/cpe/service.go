package cpe

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SwodLore/sire-api/internal/sire"
)

// CredentialsProvider obtiene las credenciales descifradas de la empresa (implementado por *company.Service)
type CredentialsProvider interface {
	GetDecryptedCredentials(ruc string) (rucOut, razonSocial, usuarioSOL, claveSOL, clientSecret string, err error)
}

// SireDownloadLister permite consultar los archivos de propuestas SIRE descargados por empresa
type SireDownloadLister interface {
	ListByEmpresa(ruc, libro, periodo string) ([]sire.DescargaSire, error)
	GetByID(id int64) (*sire.DescargaSire, error)
}

type Service struct {
	repo          *Repository
	creds         CredentialsProvider
	sireDownloads SireDownloadLister
	storageDir    string
	soapEndpoint  string
	solTokens     SolTokenProvider
	sunatCPE      *SunatCPEClient
	bulk          *BulkDownloader
}

func NewService(repo *Repository, creds CredentialsProvider, sireDownloads SireDownloadLister, storageDir string) *Service {
	if storageDir == "" {
		storageDir = "descargas"
	}
	s := &Service{
		repo:          repo,
		creds:         creds,
		sireDownloads: sireDownloads,
		storageDir:    storageDir,
		soapEndpoint:  DefaultBillConsultSOAPURL,
		solTokens:     NewSolTokenManager(),
		sunatCPE:      NewSunatCPEClient(),
	}
	s.bulk = NewBulkDownloader(s)
	return s
}

// SetSoapEndpoint permite sobreescribir el endpoint SOAP billConsultService (útil en pruebas unitarias)
func (s *Service) SetSoapEndpoint(url string) {
	s.soapEndpoint = url
}

// SetSolTokenProvider permite inyectar un proveedor de token SOL (útil en pruebas unitarias)
func (s *Service) SetSolTokenProvider(p SolTokenProvider) {
	if p != nil {
		s.solTokens = p
	}
}

// SetSunatCPEBaseURL permite sobreescribir el endpoint REST api-cpe.sunat.gob.pe (útil en pruebas unitarias)
func (s *Service) SetSunatCPEBaseURL(url string) {
	s.sunatCPE.SetBaseURL(url)
}

// SetBulkDelays configura pausas del worker pool (útil en pruebas unitarias)
func (s *Service) SetBulkDelays(jitterMin, jitterMax, retryDelay time.Duration) {
	s.bulk.SetDelays(jitterMin, jitterMax, retryDelay)
}

func (s *Service) List(ruc, origen, periodo, search string) ([]ComprobanteCPE, error) {
	return s.repo.List(ruc, origen, periodo, search)
}

func (s *Service) ListFiltered(ruc, origen, periodo, tipoCP, search string, soloVigentes bool) ([]ComprobanteCPE, error) {
	return s.repo.ListFiltered(ruc, origen, periodo, tipoCP, search, soloVigentes)
}

func (s *Service) GetByID(id int64, ruc string) (*ComprobanteCPE, error) {
	return s.repo.GetByID(id, ruc)
}

// SyncFromDescarga sincroniza los comprobantes contra la propuesta SIRE vigente del archivo indicado,
// marcando como no vigentes aquellos registros previos del periodo que ya no figuren en esta propuesta.
func (s *Service) SyncFromDescarga(d *sire.DescargaSire) (int, error) {
	if d == nil || d.RutaLocal == "" || d.Estado != string(sire.EstadoCompletado) {
		return 0, nil
	}
	origen := strings.ToUpper(strings.TrimSpace(d.Libro))
	if origen != "COMPRAS" {
		origen = "VENTAS"
	}
	safePath, err := s.resolveSafePath(d.RutaLocal)
	if err != nil {
		return 0, err
	}
	items, err := ParseSIREProposalArchive(safePath, d.EmpresaRUC, origen, d.Periodo)
	if err != nil {
		return 0, err
	}
	vigentes, fuera, err := s.repo.SyncProposalSnapshot(d.EmpresaRUC, origen, d.Periodo, items)
	if err != nil {
		return 0, err
	}
	log.Printf("[CPE-SYNC] RUC %s (%s %s): %d comprobantes vigentes sincronizados (%d fuera de propuesta) desde '%s'",
		d.EmpresaRUC, origen, d.Periodo, vigentes, fuera, d.NombreArchivo)
	return vigentes, nil
}

// SyncFromSIREDownloads sincroniza únicamente la propuesta SIRE COMPLETADA MÁS RECIENTE (vigente)
// por cada combinación (libro, periodo) de la empresa.
func (s *Service) SyncFromSIREDownloads(ruc string, descargaID int64) (*SyncResult, error) {
	res := &SyncResult{}
	if s.sireDownloads == nil {
		return res, nil
	}

	var descargas []sire.DescargaSire
	if descargaID > 0 {
		d, err := s.sireDownloads.GetByID(descargaID)
		if err != nil {
			return nil, fmt.Errorf("descarga #%d no encontrada: %w", descargaID, err)
		}
		if d.EmpresaRUC != ruc {
			return nil, errors.New("la descarga no pertenece a la empresa seleccionada")
		}
		descargas = append(descargas, *d)
	} else {
		list, err := s.sireDownloads.ListByEmpresa(ruc, "", "")
		if err != nil {
			return nil, err
		}
		// ListByEmpresa viene ordenado por ID DESC: conservar solo la propuesta MÁS RECIENTE por (libro, periodo)
		seenKey := make(map[string]bool)
		for _, d := range list {
			if d.Estado != string(sire.EstadoCompletado) || d.RutaLocal == "" {
				continue
			}
			key := strings.ToUpper(strings.TrimSpace(d.Libro)) + "|" + strings.TrimSpace(d.Periodo)
			if seenKey[key] {
				continue
			}
			seenKey[key] = true
			descargas = append(descargas, d)
		}
	}

	for i := range descargas {
		d := descargas[i]
		if d.Estado != string(sire.EstadoCompletado) || d.RutaLocal == "" {
			continue
		}
		if !s.safeFileExists(d.RutaLocal) {
			res.Advertencias = append(res.Advertencias, fmt.Sprintf("Archivo local no encontrado para ticket %s (%s)", d.Ticket, d.NombreArchivo))
			continue
		}
		n, err := s.SyncFromDescarga(&d)
		if err != nil {
			res.Advertencias = append(res.Advertencias, fmt.Sprintf("No se pudo analizar %s: %v", d.NombreArchivo, err))
			continue
		}
		res.ArchivosProcesados++
		res.ComprobantesNuevos += n
	}

	return res, nil
}

// RegisterManual registra un comprobante manualmente y opcionalmente consulta su CDR oficial vía SOAP SUNAT
func (s *Service) RegisterManual(ruc string, req *ManualCPERequest) (*CdrConsultaResult, error) {
	origen := strings.ToUpper(strings.TrimSpace(req.Origen))
	if origen != "COMPRAS" {
		origen = "VENTAS"
	}
	moneda := strings.ToUpper(strings.TrimSpace(req.Moneda))
	if moneda == "" {
		moneda = "PEN"
	}

	c := &ComprobanteCPE{
		EmpresaRUC:       ruc,
		Origen:           origen,
		Periodo:          strings.TrimSpace(req.Periodo),
		TipoCP:           padTipoCP(req.TipoCP),
		Serie:            strings.ToUpper(strings.TrimSpace(req.Serie)),
		Numero:           strings.TrimSpace(req.Numero),
		FechaEmision:     strings.TrimSpace(req.FechaEmision),
		RucContraparte:   strings.TrimSpace(req.RucContraparte),
		RazonContraparte: strings.TrimSpace(req.RazonContraparte),
		BaseImponible:    req.BaseImponible,
		IGV:              req.IGV,
		MontoTotal:       req.MontoTotal,
		Moneda:           moneda,
		EstadoSUNAT:      "REGISTRADO",
	}

	saved, err := s.repo.Upsert(c)
	if err != nil {
		return nil, err
	}

	if req.ConsultarCDR {
		return s.ConsultarYGuardarCDR(saved.ID, ruc)
	}

	return &CdrConsultaResult{
		Comprobante:   saved,
		CdrDescargado: saved.TieneCDR,
	}, nil
}

// ConsultarYGuardarCDR consulta el Web Service SOAP oficial billConsultService (getStatusCdr) de SUNAT
// utilizando las credenciales SOL de la empresa y guarda el archivo CDR .zip en el repositorio local.
func (s *Service) ConsultarYGuardarCDR(id int64, ruc string) (*CdrConsultaResult, error) {
	if s.creds == nil {
		return nil, errors.New("proveedor de credenciales no configurado")
	}

	comp, err := s.repo.GetByID(id, ruc)
	if err != nil {
		return nil, errors.New("comprobante no encontrado para esta empresa")
	}

	_, _, usuarioSOL, claveSOL, _, err := s.creds.GetDecryptedCredentials(ruc)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron obtener las credenciales SOL de la empresa: %w", err)
	}

	rucEmisor := comp.EmpresaRUC
	if strings.ToUpper(comp.Origen) == "COMPRAS" && len(comp.RucContraparte) == 11 {
		rucEmisor = comp.RucContraparte
	}

	cdrBytes, statusCode, statusMsg, err := ConsultarCdrSOAP(
		s.soapEndpoint,
		ruc,
		usuarioSOL,
		claveSOL,
		rucEmisor,
		comp.TipoCP,
		comp.Serie,
		comp.Numero,
	)
	if err != nil {
		return nil, err
	}

	estado := strings.TrimSpace(statusCode)
	if estado == "0004" || estado == "0" {
		estado = "CDR_ACEPTADO"
	} else if estado != "" {
		estado = "SUNAT_" + estado
	}

	cdrDescargado := false
	if len(cdrBytes) > 0 {
		fileName := fmt.Sprintf("R-%s-%s-%s-%s.zip", rucEmisor, comp.TipoCP, comp.Serie, comp.Numero)
		targetPath, hashHex, writeErr := s.saveFile(ruc, comp.Origen, fileName, cdrBytes)
		if writeErr != nil {
			return nil, fmt.Errorf("error guardando CDR en disco: %w", writeErr)
		}
		if err := s.repo.UpdateFilePath(comp.ID, ruc, "cdr", targetPath, estado, hashHex); err != nil {
			return nil, err
		}
		cdrDescargado = true
	}

	updated, _ := s.repo.GetByID(comp.ID, ruc)
	if updated == nil {
		updated = comp
	}

	return &CdrConsultaResult{
		Comprobante:   updated,
		StatusCode:    statusCode,
		StatusMessage: statusMsg,
		CdrDescargado: cdrDescargado,
	}, nil
}

// ImportUploadedFile guarda un archivo XML, PDF o ZIP (CDR o lote de comprobantes) y lo asocia en BD
func (s *Service) ImportUploadedFile(ruc, origen, filename string, data []byte, targetCPEID int64) (*ImportResult, error) {
	res := &ImportResult{Comprobantes: make([]ComprobanteCPE, 0)}
	if len(data) == 0 {
		return nil, errors.New("el archivo subido está vacío")
	}

	origen = strings.ToUpper(strings.TrimSpace(origen))
	if origen != "COMPRAS" {
		origen = "VENTAS"
	}

	cleanName := filepath.Base(filepath.Clean(filename))
	ext := strings.ToLower(filepath.Ext(cleanName))

	// Caso 1: Adjuntar archivo directamente a un comprobante específico por ID
	if targetCPEID > 0 {
		comp, err := s.repo.GetByID(targetCPEID, ruc)
		if err != nil {
			return nil, errors.New("comprobante destino no encontrado")
		}
		tipoArchivo := ""
		switch ext {
		case ".xml":
			tipoArchivo = "xml"
		case ".pdf":
			tipoArchivo = "pdf"
		case ".zip":
			tipoArchivo = "cdr"
		default:
			return nil, fmt.Errorf("formato no soportado '%s'; use .xml, .pdf o .zip (CDR)", ext)
		}

		path, hashHex, err := s.saveFile(ruc, comp.Origen, cleanName, data)
		if err != nil {
			return nil, err
		}
		if err := s.repo.UpdateFilePath(comp.ID, ruc, tipoArchivo, path, "", hashHex); err != nil {
			return nil, err
		}
		updated, _ := s.repo.GetByID(comp.ID, ruc)
		res.ArchivosGuardados = 1
		if updated != nil {
			res.Comprobantes = append(res.Comprobantes, *updated)
		}
		return res, nil
	}

	// Caso 2: Archivo ZIP (puede ser CDR individual R-*.zip o paquete con múltiples XML/PDF/CDR)
	if ext == ".zip" || isZipBytes(data) {
		if rucFile, tipoCP, serie, numero, isCDR, ok := ParseFilenameCPE(cleanName); ok && isCDR {
			saved, err := s.attachSingleFile(ruc, origen, rucFile, tipoCP, serie, numero, "cdr", cleanName, data, nil)
			if err != nil {
				return nil, err
			}
			res.ArchivosGuardados = 1
			res.Comprobantes = append(res.Comprobantes, *saved)
			return res, nil
		}

		// Inspeccionar contenido del ZIP
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("archivo ZIP inválido: %w", err)
		}
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			innerBytes, _ := io.ReadAll(rc)
			_ = rc.Close()

			innerName := filepath.Base(f.Name)
			innerExt := strings.ToLower(filepath.Ext(innerName))
			if innerExt != ".xml" && innerExt != ".pdf" && innerExt != ".zip" {
				continue
			}
			sub, subErr := s.processStandaloneFile(ruc, origen, innerName, innerBytes)
			if subErr != nil {
				res.Advertencias = append(res.Advertencias, fmt.Sprintf("%s: %v", innerName, subErr))
				continue
			}
			res.ArchivosGuardados++
			res.Comprobantes = append(res.Comprobantes, *sub)
		}
		return res, nil
	}

	// Caso 3: Archivo individual XML o PDF
	saved, err := s.processStandaloneFile(ruc, origen, cleanName, data)
	if err != nil {
		return nil, err
	}
	res.ArchivosGuardados = 1
	res.Comprobantes = append(res.Comprobantes, *saved)
	return res, nil
}

func (s *Service) processStandaloneFile(ruc, origen, cleanName string, data []byte) (*ComprobanteCPE, error) {
	ext := strings.ToLower(filepath.Ext(cleanName))
	switch ext {
	case ".xml":
		meta, err := ParseUBLXML(data)
		if err != nil {
			return nil, err
		}
		rucFile, tipoFile, serieFile, numFile, isCDRFile, okName := ParseFilenameCPE(cleanName)
		tipoCP := meta.TipoCP
		serie := meta.Serie
		numero := meta.Numero
		rucDoc := meta.RucEmisor
		if okName {
			if tipoCP == "" {
				tipoCP = tipoFile
			}
			if serie == "" {
				serie = serieFile
			}
			if numero == "" {
				numero = numFile
			}
			if rucDoc == "" {
				rucDoc = rucFile
			}
		}
		if serie == "" || numero == "" {
			return nil, errors.New("no se pudo identificar Serie y Número en el XML ni en el nombre del archivo")
		}
		if tipoCP == "" {
			if strings.HasPrefix(serie, "F") {
				tipoCP = "01"
			} else if strings.HasPrefix(serie, "B") {
				tipoCP = "03"
			} else {
				tipoCP = "01"
			}
		}
		// Autodetectar origen si el RUC emisor del XML coincide o difiere de la empresa
		detectedOrigen := origen
		if meta.RucEmisor != "" {
			if meta.RucEmisor == ruc {
				detectedOrigen = "VENTAS"
			} else {
				detectedOrigen = "COMPRAS"
			}
		}
		tipoArchivo := "xml"
		if meta.EsCDR || isCDRFile {
			tipoArchivo = "cdr"
		}
		return s.attachSingleFile(ruc, detectedOrigen, rucDoc, tipoCP, serie, numero, tipoArchivo, cleanName, data, meta)

	case ".pdf":
		rucFile, tipoCP, serie, numero, _, ok := ParseFilenameCPE(cleanName)
		if !ok {
			return nil, errors.New("para subir un PDF suelto use el formato SUNAT 'RUC-TIPO-SERIE-NUMERO.pdf' (ej. 20000000001-01-F001-00000123.pdf) o adjúntelo desde la fila del comprobante")
		}
		return s.attachSingleFile(ruc, origen, rucFile, tipoCP, serie, numero, "pdf", cleanName, data, nil)

	case ".zip":
		rucFile, tipoCP, serie, numero, _, ok := ParseFilenameCPE(cleanName)
		if !ok {
			return nil, errors.New("nombre de archivo CDR ZIP no reconocido; use formato 'R-RUC-TIPO-SERIE-NUMERO.zip'")
		}
		return s.attachSingleFile(ruc, origen, rucFile, tipoCP, serie, numero, "cdr", cleanName, data, nil)

	default:
		return nil, fmt.Errorf("extensión '%s' no soportada (use .xml, .pdf o .zip)", ext)
	}
}

func (s *Service) attachSingleFile(ruc, origen, rucDoc, tipoCP, serie, numero, tipoArchivo, cleanName string, data []byte, meta *UBLMetadata) (*ComprobanteCPE, error) {
	path, hashHex, err := s.saveFile(ruc, origen, cleanName, data)
	if err != nil {
		return nil, err
	}

	c := &ComprobanteCPE{
		EmpresaRUC:  ruc,
		Origen:      origen,
		TipoCP:      padTipoCP(tipoCP),
		Serie:       strings.ToUpper(strings.TrimSpace(serie)),
		Numero:      strings.TrimSpace(numero),
		Moneda:      "PEN",
		EstadoSUNAT: "IMPORTADO",
		HashArchivo: hashHex,
	}

	if meta != nil {
		c.FechaEmision = meta.FechaEmision
		if len(meta.FechaEmision) >= 7 {
			c.Periodo = strings.ReplaceAll(meta.FechaEmision[:7], "-", "")
		}
		if meta.Moneda != "" {
			c.Moneda = meta.Moneda
		}
		c.BaseImponible = meta.BaseImponible
		c.IGV = meta.IGV
		c.MontoTotal = meta.MontoTotal
		if origen == "COMPRAS" {
			c.RucContraparte = meta.RucEmisor
			c.RazonContraparte = meta.RazonEmisor
		} else {
			c.RucContraparte = meta.RucCliente
			c.RazonContraparte = meta.RazonCliente
		}
		if meta.EsCDR && meta.CdrResponseCode == "0" {
			c.EstadoSUNAT = "CDR_ACEPTADO"
		}
	} else if rucDoc != "" && rucDoc != ruc {
		c.RucContraparte = rucDoc
	}

	switch tipoArchivo {
	case "xml":
		c.RutaXML = path
		if meta != nil && !meta.EsCDR {
			conc, det, sumItems := EvaluateConcordanceSIRE(c, meta)
			c.ConcordanciaSIRE = conc
			c.DetalleConcordancia = det
			c.ResumenItems = sumItems
		}
	case "pdf":
		c.RutaPDF = path
	case "cdr":
		c.RutaCDR = path
	}

	return s.repo.Upsert(c)
}

// GetDownloadFile retorna el contenido local (o generado si existe XML/SIRE) sin llamadas de red externas.
func (s *Service) GetDownloadFile(id int64, ruc, tipo string) (data []byte, filename, contentType, source string, err error) {
	return s.GetDownloadFileWithContext(context.Background(), id, ruc, tipo, false, true)
}

// GetDownloadFileWithContext retorna el archivo real del comprobante (PDF/XML/CDR/JSON).
// Cuando autoFetchSunat=true y el archivo original aún no está en disco, lo descarga automáticamente
// en tiempo real desde el portal SUNAT SOL (api-cpe.sunat.gob.pe) y valida concordancia con el SIRE.
func (s *Service) GetDownloadFileWithContext(ctx context.Context, id int64, ruc, tipo string, autoFetchSunat, allowSireFallback bool) (data []byte, filename, contentType, source string, err error) {
	comp, err := s.repo.GetByID(id, ruc)
	if err != nil {
		return nil, "", "", "", errors.New("comprobante no encontrado")
	}

	rucBase := comp.EmpresaRUC
	if strings.ToUpper(comp.Origen) == "COMPRAS" && len(comp.RucContraparte) == 11 {
		rucBase = comp.RucContraparte
	}

	razon := "EMPRESA RUC " + ruc
	if s.creds != nil {
		if _, rSoc, _, _, _, cErr := s.creds.GetDecryptedCredentials(ruc); cErr == nil && rSoc != "" {
			razon = rSoc
		}
	}

	tipoLower := strings.ToLower(strings.TrimSpace(tipo))

	// Si se solicita descarga real automática y aún falta el archivo en disco, consultar en SUNAT SOL
	// únicamente el formato solicitado (evitando descargar XML+PDF+CDR+SOAP innecesariamente).
	var fetchErr error
	if autoFetchSunat {
		needFetch := false
		switch tipoLower {
		case "pdf":
			needFetch = !s.safeFileExists(comp.RutaPDF) && !s.safeFileExists(comp.RutaXML)
		case "xml":
			needFetch = !s.safeFileExists(comp.RutaXML)
		case "cdr":
			needFetch = !s.safeFileExists(comp.RutaCDR)
		case "json":
			needFetch = !s.safeFileExists(comp.RutaXML)
		}
		if needFetch {
			_, fetchErr = s.DownloadFormatFromSUNAT(ctx, id, ruc, tipoLower)
			if refreshed, rErr := s.repo.GetByID(id, ruc); rErr == nil && refreshed != nil {
				comp = refreshed
			}
		}
	}

	switch tipoLower {
	case "pdf":
		if comp.RutaPDF != "" {
			if raw, rErr := s.readSafeFile(comp.RutaPDF); rErr == nil && len(raw) > 0 {
				return raw, filepath.Base(comp.RutaPDF), "application/pdf", "original", nil
			}
		}
		fname := fmt.Sprintf("%s-%s-%s-%s.pdf", rucBase, comp.TipoCP, comp.Serie, comp.Numero)
		// Si ya tenemos el XML UBL 2.1 original de SUNAT en disco, generar Representación Impresa completa con ítems, Hash y QR SUNAT
		if comp.RutaXML != "" {
			if rawXML, rErr := s.readSafeFile(comp.RutaXML); rErr == nil && len(rawXML) > 0 {
				if meta, pErr := ParseUBLXML(rawXML); pErr == nil && meta != nil && !meta.EsCDR {
					pdfBytes := GenerateComprobantePDFFromUBL(comp, meta, razon)
					return pdfBytes, fname, "application/pdf", "xml-ubl", nil
				}
			}
		}
		if !allowSireFallback {
			if fetchErr != nil && !errors.Is(fetchErr, ErrComprobanteNoDisponible) {
				return nil, "", "", "", fetchErr
			}
			return nil, "", "", "", fmt.Errorf("%w: SUNAT no dispone del PDF ni del XML original para %s-%s-%s en el portal Consulta CPE (frecuente en Boletas 03 enviadas por Resumen Diario). Si desea solo una ficha resumen del SIRE, agregue ?fallback=sire",
				ErrComprobanteNoDisponible, comp.TipoCP, comp.Serie, comp.Numero)
		}
		// Respaldo explícito: ficha PDF generada desde el registro SIRE
		pdfBytes := GenerateComprobantePDF(comp, razon)
		return pdfBytes, fname, "application/pdf", "sire-resumen", nil

	case "xml":
		if comp.RutaXML == "" {
			if fetchErr != nil && !errors.Is(fetchErr, ErrComprobanteNoDisponible) {
				return nil, "", "", "", fetchErr
			}
			return nil, "", "", "", fmt.Errorf("%w: SUNAT no devolvió el XML original para %s-%s-%s (en Boletas 03 por Resumen Diario SUNAT no expone XML individual)",
				ErrComprobanteNoDisponible, comp.TipoCP, comp.Serie, comp.Numero)
		}
		raw, rErr := s.readSafeFile(comp.RutaXML)
		if rErr != nil {
			return nil, "", "", "", fmt.Errorf("archivo XML no encontrado en disco: %w", rErr)
		}
		return raw, filepath.Base(comp.RutaXML), "application/xml", "original", nil

	case "cdr":
		if comp.RutaCDR == "" {
			if fetchErr != nil && !errors.Is(fetchErr, ErrComprobanteNoDisponible) {
				return nil, "", "", "", fetchErr
			}
			return nil, "", "", "", fmt.Errorf("%w: SUNAT no devolvió el CDR oficial para %s-%s-%s",
				ErrComprobanteNoDisponible, comp.TipoCP, comp.Serie, comp.Numero)
		}
		raw, rErr := s.readSafeFile(comp.RutaCDR)
		if rErr != nil {
			return nil, "", "", "", fmt.Errorf("archivo CDR no encontrado en disco: %w", rErr)
		}
		ct := "application/zip"
		if strings.HasSuffix(strings.ToLower(comp.RutaCDR), ".xml") {
			ct = "application/xml"
		}
		return raw, filepath.Base(comp.RutaCDR), ct, "original", nil

	case "json":
		var ublMeta *UBLMetadata
		src := "sire-resumen"
		if comp.RutaXML != "" {
			if rawXML, rErr := s.readSafeFile(comp.RutaXML); rErr == nil && len(rawXML) > 0 {
				if meta, pErr := ParseUBLXML(rawXML); pErr == nil && meta != nil && !meta.EsCDR {
					ublMeta = meta
					src = "xml-ubl"
				}
			}
		}
		doc := buildComprobanteJSON(comp, ublMeta, razon)
		rawJSON, mErr := json.MarshalIndent(doc, "", "  ")
		if mErr != nil {
			return nil, "", "", "", fmt.Errorf("error serializando JSON: %w", mErr)
		}
		fname := fmt.Sprintf("%s-%s-%s-%s.json", rucBase, comp.TipoCP, comp.Serie, comp.Numero)
		return rawJSON, fname, "application/json; charset=utf-8", src, nil

	default:
		return nil, "", "", "", fmt.Errorf("tipo de descarga '%s' no válido (use pdf, xml, cdr o json)", tipo)
	}
}

// GetDetalleComprobante devuelve el detalle completo (ítems reales, montos, hash, QR y concordancia XML vs SIRE).
// Si autoSync=true y todavía no se ha descargado el XML desde SUNAT, lo descarga automáticamente primero.
func (s *Service) GetDetalleComprobante(ctx context.Context, id int64, ruc string, autoSync bool) (map[string]interface{}, error) {
	comp, err := s.repo.GetByID(id, ruc)
	if err != nil {
		return nil, errors.New("comprobante no encontrado")
	}

	var syncAdvertencia string
	if autoSync && !s.safeFileExists(comp.RutaXML) {
		if dlRes, dlErr := s.DownloadFormatFromSUNAT(ctx, id, ruc, "xml"); dlErr != nil {
			syncAdvertencia = dlErr.Error()
		} else if dlRes != nil && len(dlRes.Advertencias) > 0 {
			syncAdvertencia = strings.Join(dlRes.Advertencias, " | ")
		}
		if refreshed, rErr := s.repo.GetByID(id, ruc); rErr == nil && refreshed != nil {
			comp = refreshed
		}
	}

	razon := "EMPRESA RUC " + ruc
	if s.creds != nil {
		if _, rSoc, _, _, _, cErr := s.creds.GetDecryptedCredentials(ruc); cErr == nil && rSoc != "" {
			razon = rSoc
		}
	}

	var ublMeta *UBLMetadata
	if comp.RutaXML != "" {
		if rawXML, rErr := s.readSafeFile(comp.RutaXML); rErr == nil && len(rawXML) > 0 {
			if meta, pErr := ParseUBLXML(rawXML); pErr == nil && meta != nil && !meta.EsCDR {
				ublMeta = meta
				// Recalcular concordancia si aún estaba pendiente
				if comp.ConcordanciaSIRE == "" || comp.ConcordanciaSIRE == "PENDIENTE" {
					conc, det, sumItems := EvaluateConcordanceSIRE(comp, meta)
					_ = s.repo.UpdateConcordancia(comp.ID, ruc, conc, det, sumItems)
					comp.ConcordanciaSIRE = conc
					comp.DetalleConcordancia = det
					comp.ResumenItems = sumItems
				}
			}
		}
	}

	doc := buildComprobanteJSON(comp, ublMeta, razon)
	return map[string]interface{}{
		"comprobante":          comp,
		"detalle":              doc,
		"concordancia_sire":    comp.ConcordanciaSIRE,
		"detalle_concordancia": comp.DetalleConcordancia,
		"advertencia_sync":     syncAdvertencia,
	}, nil
}

// DownloadSingleFromSUNAT descarga XML (/02), PDF (/01) y CDR (/03 o SOAP) de un comprobante individual directo desde SUNAT SOL.
func (s *Service) DownloadSingleFromSUNAT(ctx context.Context, id int64, ruc string) (*SunatSingleDownloadResult, error) {
	return s.DownloadFormatFromSUNAT(ctx, id, ruc, "todos")
}

// DownloadFormatFromSUNAT descarga bajo demanda únicamente el formato solicitado ("pdf", "xml", "cdr", "json" o "todos")
// usando reintentos acotados para evitar bloqueos largos en peticiones interactivas:
//   - "pdf": consulta primero solo /01 (PDF); solo si no está disponible en SUNAT, consulta /02 (XML) para generar PDF+QR (nunca bloquea en CDR/SOAP).
//   - "xml" / "json": consulta solo /02 (XML) (conservando CDR embebido si viene en el mismo ZIP).
//   - "cdr": consulta solo /03 (+ respaldo SOAP si /03 falla).
//   - "todos": descarga XML + PDF + CDR con reintentos acotados.
func (s *Service) DownloadFormatFromSUNAT(ctx context.Context, id int64, ruc, tipo string) (*SunatSingleDownloadResult, error) {
	if s.creds == nil {
		return nil, errors.New("proveedor de credenciales no configurado")
	}
	comp, err := s.repo.GetByID(id, ruc)
	if err != nil {
		return nil, errors.New("comprobante no encontrado para esta empresa")
	}
	_, razonEmpresa, usuarioSOL, claveSOL, _, err := s.creds.GetDecryptedCredentials(ruc)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron obtener las credenciales SOL: %w", err)
	}
	if razonEmpresa == "" {
		razonEmpresa = "EMPRESA RUC " + ruc
	}

	tok, err := s.solTokens.GetToken(ctx, ruc, usuarioSOL, claveSOL, false)
	if err != nil {
		return nil, err
	}

	getTok := func() string { return tok }
	refreshTok := func() (string, error) {
		nt, rErr := s.solTokens.GetToken(ctx, ruc, usuarioSOL, claveSOL, true)
		if rErr != nil {
			return "", rErr
		}
		tok = nt
		return nt, nil
	}

	const interactiveMaxAttempts = 2
	tipoLower := strings.ToLower(strings.TrimSpace(tipo))
	var itemRes BulkDownloadItemResult

	switch tipoLower {
	case "pdf":
		// 1. Intentar solo PDF oficial (/01) sin tocar XML ni CDR/SOAP
		itemRes = s.bulk.processOneItemWithAttempts(ctx, comp, razonEmpresa, usuarioSOL, claveSOL, false, true, false, false, false, interactiveMaxAttempts, getTok, refreshTok)
		// 2. Solo si SUNAT no entregó el PDF y no fue un error fatal de sesión SOL, intentar XML (/02) para generar PDF+QR
		if itemRes.PDF != "ok" && !isFatalSolAuthError(itemRes.rawErr) && !s.safeFileExists(comp.RutaXML) {
			xmlRes := s.bulk.processOneItemWithAttempts(ctx, comp, razonEmpresa, usuarioSOL, claveSOL, true, false, false, true, false, interactiveMaxAttempts, getTok, refreshTok)
			itemRes.XML = xmlRes.XML
			if xmlRes.CDR != "" {
				itemRes.CDR = xmlRes.CDR
			}
			if xmlRes.XML == "ok" {
				// Ya tenemos el XML real en disco: GetDownloadFileWithContext generará el PDF+QR (source="xml-ubl")
				// sin sobrescribir RutaPDF con un PDF generado localmente ni llamar a CDR/SOAP.
				itemRes.rawErr = nil
				itemRes.Error = ""
				itemRes.PDF = "generado_xml"
			} else {
				if xmlRes.Error != "" {
					itemRes.Error = appendErr(itemRes.Error, xmlRes.Error)
				}
				itemRes.rawErr = mergeItemError(itemRes.rawErr, xmlRes.rawErr)
			}
		}
	case "xml", "json":
		itemRes = s.bulk.processOneItemWithAttempts(ctx, comp, razonEmpresa, usuarioSOL, claveSOL, true, false, false, false, false, interactiveMaxAttempts, getTok, refreshTok)
	case "cdr":
		itemRes = s.bulk.processOneItemWithAttempts(ctx, comp, razonEmpresa, usuarioSOL, claveSOL, false, false, true, false, false, interactiveMaxAttempts, getTok, refreshTok)
	default:
		itemRes = s.bulk.processOneItemWithAttempts(ctx, comp, razonEmpresa, usuarioSOL, claveSOL, true, true, true, false, true, interactiveMaxAttempts, getTok, refreshTok)
	}

	updated, _ := s.repo.GetByID(comp.ID, ruc)
	if updated == nil {
		updated = comp
	}

	out := &SunatSingleDownloadResult{
		Comprobante:    updated,
		XmlDescargado:  itemRes.XML == "ok",
		PdfDescargado:  itemRes.PDF == "ok",
		PdfGeneradoXML: itemRes.PDF == "generado_xml",
		CdrDescargado:  itemRes.CDR == "ok",
	}
	if itemRes.Error != "" {
		out.Advertencias = append(out.Advertencias, itemRes.Error)
	}
	if !out.XmlDescargado && !out.PdfDescargado && !out.PdfGeneradoXML && !out.CdrDescargado {
		if itemRes.rawErr != nil {
			return out, itemRes.rawErr
		}
		if itemRes.Error != "" {
			return out, errors.New(itemRes.Error)
		}
	}
	return out, nil
}

// EvaluateConcordanceSIRE compara el XML oficial UBL 2.1 descargado de SUNAT contra el registro de la propuesta vigente del SIRE.
func EvaluateConcordanceSIRE(c *ComprobanteCPE, meta *UBLMetadata) (estado, detalle, resumenItems string) {
	if meta == nil {
		return "PENDIENTE", "", ""
	}

	// Construir resumen compacto de ítems reales del XML
	if len(meta.Items) > 0 {
		var parts []string
		for i, it := range meta.Items {
			if i >= 2 {
				break
			}
			d := strings.TrimSpace(it.Descripcion)
			if d != "" {
				parts = append(parts, truncateStr(d, 48))
			}
		}
		if len(parts) > 0 {
			resumenItems = fmt.Sprintf("%d ítem(s): %s", len(meta.Items), strings.Join(parts, " | "))
		} else {
			resumenItems = fmt.Sprintf("%d ítem(s)", len(meta.Items))
		}
	}

	if c == nil {
		return "CONCORDANTE", "XML UBL 2.1 válido", resumenItems
	}

	var diffs []string
	// 1. Verificar RUC emisor
	expectedEmisor := c.EmpresaRUC
	if strings.ToUpper(c.Origen) == "COMPRAS" && len(c.RucContraparte) == 11 {
		expectedEmisor = c.RucContraparte
	}
	if meta.RucEmisor != "" && expectedEmisor != "" && meta.RucEmisor != expectedEmisor {
		diffs = append(diffs, fmt.Sprintf("RUC emisor XML (%s) != SIRE (%s)", meta.RucEmisor, expectedEmisor))
	}

	// 2. Verificar Serie y Número (ignorando ceros a la izquierda)
	if meta.Serie != "" && c.Serie != "" && !strings.EqualFold(meta.Serie, c.Serie) {
		diffs = append(diffs, fmt.Sprintf("Serie XML (%s) != SIRE (%s)", meta.Serie, c.Serie))
	}
	if meta.Numero != "" && c.Numero != "" && normalizeCorrelativo(meta.Numero) != normalizeCorrelativo(c.Numero) {
		diffs = append(diffs, fmt.Sprintf("Número XML (%s) != SIRE (%s)", meta.Numero, c.Numero))
	}

	// 3. Verificar Moneda
	if meta.Moneda != "" && c.Moneda != "" && !strings.EqualFold(meta.Moneda, c.Moneda) {
		diffs = append(diffs, fmt.Sprintf("Moneda XML (%s) != SIRE (%s)", meta.Moneda, c.Moneda))
	}

	// 4. Verificar Monto Total (valor absoluto para soportar Notas de Crédito negativas en SIRE; tolerancia 0.05 por redondeo)
	if meta.MontoTotal > 0 && c.MontoTotal != 0 {
		sireTotal := math.Abs(c.MontoTotal)
		if math.Abs(meta.MontoTotal-sireTotal) > 0.05 {
			diffs = append(diffs, fmt.Sprintf("Total XML (%.2f) != SIRE (%.2f)", meta.MontoTotal, sireTotal))
		}
	}

	// 5. Verificar IGV (cuando ambos son mayores que cero)
	if meta.IGV > 0 && c.IGV != 0 {
		sireIGV := math.Abs(c.IGV)
		if math.Abs(meta.IGV-sireIGV) > 0.05 {
			diffs = append(diffs, fmt.Sprintf("IGV XML (%.2f) != SIRE (%.2f)", meta.IGV, sireIGV))
		}
	}

	if len(diffs) > 0 {
		return "DIFERENCIA", strings.Join(diffs, "; "), resumenItems
	}
	return "CONCORDANTE", fmt.Sprintf("Validado: XML oficial SUNAT coincide 100%% con propuesta SIRE (Total %s %.2f)", valueOrDash(meta.Moneda), meta.MontoTotal), resumenItems
}

// StartBulkDownload inicia un job asíncrono de descarga masiva (XML, PDF, CDR, JSON + ZIP).
func (s *Service) StartBulkDownload(ruc string, req BulkDownloadRequest) (*BulkDownloadJob, error) {
	return s.bulk.StartJob(ruc, req)
}

func (s *Service) GetBulkJob(ruc, jobID string) (*BulkDownloadJob, error) {
	return s.bulk.GetJob(ruc, jobID)
}

func (s *Service) ListBulkJobs(ruc string) []BulkDownloadJob {
	return s.bulk.ListJobs(ruc)
}

func (s *Service) CancelBulkJob(ruc, jobID string) (*BulkDownloadJob, error) {
	return s.bulk.CancelJob(ruc, jobID)
}

func (s *Service) GetBulkZip(ruc, jobID string) ([]byte, string, error) {
	job, err := s.bulk.GetJob(ruc, jobID)
	if err != nil {
		return nil, "", err
	}
	if !job.TieneZIP || job.RutaZIP == "" {
		return nil, "", errors.New("el paquete ZIP del lote aún no está disponible")
	}
	raw, rErr := s.readSafeFile(job.RutaZIP)
	if rErr != nil {
		return nil, "", fmt.Errorf("no se pudo leer el ZIP del lote: %w", rErr)
	}
	return raw, job.NombreZIP, nil
}

// ExportItemsCSV genera un CSV compatible con Excel (UTF-8 BOM) con todas las líneas/descripciones
// extraídas de los archivos XML UBL 2.1 almacenados para la empresa/origen/periodo.
func (s *Service) ExportItemsCSV(ruc, origen, periodo, tipoCP string) ([]byte, string, error) {
	var tipos []string
	switch strings.ToUpper(strings.TrimSpace(tipoCP)) {
	case "FACTURAS_NOTAS":
		tipos = []string{"01", "07", "08"}
	case "", "TODOS":
	default:
		tipos = []string{padTipoCP(tipoCP)}
	}
	items, err := s.repo.ListForBulk(ruc, origen, periodo, tipos, true, nil)
	if err != nil {
		return nil, "", err
	}

	razon := "EMPRESA RUC " + ruc
	if s.creds != nil {
		if _, rSoc, _, _, _, cErr := s.creds.GetDecryptedCredentials(ruc); cErr == nil && rSoc != "" {
			razon = rSoc
		}
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM UTF-8 para tildes correctas en Excel
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"Origen", "Periodo", "TipoCP", "Serie", "Numero", "FechaEmision",
		"RucEmisor", "RazonEmisor", "DocCliente", "RazonCliente", "Moneda",
		"ItemNro", "Codigo", "Cantidad", "Unidad", "Descripcion",
		"ValorUnitario", "PrecioUnitario", "IGVItem", "ValorVenta", "TotalComprobante", "HashXML",
	})

	for i := range items {
		comp := &items[i]
		var ublMeta *UBLMetadata
		if comp.RutaXML != "" {
			if rawXML, rErr := s.readSafeFile(comp.RutaXML); rErr == nil && len(rawXML) > 0 {
				ublMeta, _ = ParseUBLXML(rawXML)
			}
		}
		doc := buildComprobanteJSON(comp, ublMeta, razon)

		if len(doc.Items) == 0 {
			_ = w.Write([]string{
				doc.Origen, doc.Periodo, doc.TipoCP, doc.Serie, doc.Numero, doc.FechaEmision,
				doc.RucEmisor, doc.RazonEmisor, doc.NumDocCliente, doc.RazonCliente, doc.Moneda,
				"", "", "", "", "(Sin XML adjunto - descargue XML desde SUNAT para ver detalle de ítems)",
				"", "", "", "", fmt.Sprintf("%.2f", doc.MontoTotal), doc.HashXML,
			})
			continue
		}

		for _, it := range doc.Items {
			_ = w.Write([]string{
				doc.Origen, doc.Periodo, doc.TipoCP, doc.Serie, doc.Numero, doc.FechaEmision,
				doc.RucEmisor, doc.RazonEmisor, doc.NumDocCliente, doc.RazonCliente, doc.Moneda,
				it.ID, it.Codigo, formatCantidad(it.Cantidad), it.Unidad, it.Descripcion,
				fmt.Sprintf("%.4f", it.ValorUnitario), fmt.Sprintf("%.4f", it.PrecioUnitario),
				fmt.Sprintf("%.2f", it.IGV), fmt.Sprintf("%.2f", it.ValorVenta),
				fmt.Sprintf("%.2f", doc.MontoTotal), doc.HashXML,
			})
		}
	}

	w.Flush()
	tagOrigen := strings.ToUpper(strings.TrimSpace(origen))
	if tagOrigen == "" {
		tagOrigen = "TODOS"
	}
	tagPeriodo := strings.TrimSpace(periodo)
	if tagPeriodo == "" {
		tagPeriodo = "TODOS"
	}
	fname := fmt.Sprintf("ITEMS_CPE_%s_%s_%s.csv", ruc, tagOrigen, tagPeriodo)
	return buf.Bytes(), fname, nil
}

func buildComprobanteJSON(c *ComprobanteCPE, meta *UBLMetadata, razonEmpresa string) ComprobanteJSON {
	doc := ComprobanteJSON{
		EmpresaRUC:    c.EmpresaRUC,
		Origen:        c.Origen,
		Periodo:       c.Periodo,
		TipoCP:        padTipoCP(c.TipoCP),
		TipoNombre:    tipoCPNombreCorto(c.TipoCP),
		Serie:         c.Serie,
		Numero:        c.Numero,
		FechaEmision:  c.FechaEmision,
		Moneda:        c.Moneda,
		BaseImponible: c.BaseImponible,
		IGV:           c.IGV,
		MontoTotal:    c.MontoTotal,
		EstadoSUNAT:   c.EstadoSUNAT,
		CarSUNAT:      c.CarSUNAT,
		FuenteDatos:   "sire-resumen",
	}

	if strings.ToUpper(c.Origen) == "COMPRAS" {
		doc.RucEmisor = c.RucContraparte
		doc.RazonEmisor = c.RazonContraparte
		doc.NumDocCliente = c.EmpresaRUC
		doc.RazonCliente = razonEmpresa
		doc.TipoDocCliente = "6"
	} else {
		doc.RucEmisor = c.EmpresaRUC
		doc.RazonEmisor = razonEmpresa
		doc.NumDocCliente = c.RucContraparte
		doc.RazonCliente = c.RazonContraparte
		if len(c.RucContraparte) == 11 {
			doc.TipoDocCliente = "6"
		} else if len(c.RucContraparte) == 8 {
			doc.TipoDocCliente = "1"
		}
	}

	if meta != nil && !meta.EsCDR {
		doc.FuenteDatos = "xml-ubl"
		if meta.RucEmisor != "" {
			doc.RucEmisor = meta.RucEmisor
		}
		if meta.RazonEmisor != "" {
			doc.RazonEmisor = meta.RazonEmisor
		}
		doc.DireccionEmisor = meta.DireccionEmisor
		if meta.TipoDocCliente != "" {
			doc.TipoDocCliente = meta.TipoDocCliente
		}
		if meta.RucCliente != "" {
			doc.NumDocCliente = meta.RucCliente
		}
		if meta.RazonCliente != "" {
			doc.RazonCliente = meta.RazonCliente
		}
		doc.DireccionCliente = meta.DireccionCliente
		if meta.FechaEmision != "" {
			doc.FechaEmision = meta.FechaEmision
		}
		doc.HoraEmision = meta.HoraEmision
		doc.FechaVencimiento = meta.FechaVencimiento
		if meta.Moneda != "" {
			doc.Moneda = meta.Moneda
		}
		if meta.BaseImponible != 0 {
			doc.BaseImponible = meta.BaseImponible
		}
		doc.OpGravada = meta.OpGravada
		doc.OpExonerada = meta.OpExonerada
		doc.OpInafecta = meta.OpInafecta
		doc.OpGratuita = meta.OpGratuita
		if meta.IGV != 0 {
			doc.IGV = meta.IGV
		}
		if meta.MontoTotal != 0 {
			doc.MontoTotal = meta.MontoTotal
		}
		doc.FormaPago = meta.FormaPago
		doc.Cuotas = meta.Cuotas
		doc.Leyendas = meta.Leyendas
		doc.DocModificadoRef = meta.DocModificadoRef
		doc.MotivoNota = meta.MotivoNota
		doc.HashXML = meta.DigestValue
		doc.Items = meta.Items
	}

	doc.CodigoQR = BuildSunatQRString(meta, c)
	return doc
}

func (s *Service) saveFile(ruc, origen, cleanName string, data []byte) (string, string, error) {
	safeName := filepath.Base(filepath.Clean(cleanName))
	if safeName == "" || safeName == "." {
		safeName = "comprobante.bin"
	}
	origenLower := strings.ToLower(origen)
	relPath := filepath.ToSlash(filepath.Join(ruc, "cpe", origenLower, safeName))
	targetDir := filepath.Join(s.storageDir, ruc, "cpe", origenLower)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", "", fmt.Errorf("error creando directorio CPE: %w", err)
	}
	targetPath := filepath.Join(targetDir, safeName)
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return "", "", fmt.Errorf("error escribiendo archivo CPE: %w", err)
	}
	sum := sha256.Sum256(data)
	return relPath, hex.EncodeToString(sum[:]), nil
}

func (s *Service) resolveSafePath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", errors.New("ruta de archivo vacía")
	}

	rawAbsStorage, err := filepath.Abs(filepath.Clean(s.storageDir))
	if err != nil {
		return "", errors.New("violación de seguridad: no se pudo resolver el directorio permitido")
	}
	evalStorage := rawAbsStorage
	if resolved, rErr := filepath.EvalSymlinks(rawAbsStorage); rErr == nil {
		evalStorage = resolved
	}

	cleaned := filepath.Clean(trimmed)
	var candidate string
	if filepath.IsAbs(cleaned) {
		candidate = cleaned
	} else {
		if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "\\") {
			return "", errors.New("violación de seguridad: archivo fuera del directorio permitido")
		}
		candidate = filepath.Join(rawAbsStorage, filepath.FromSlash(cleaned))
	}

	absTarget, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", errors.New("violación de seguridad: ruta de archivo inválida")
	}
	if !isPathWithinBaseCPE(rawAbsStorage, absTarget) && !isPathWithinBaseCPE(evalStorage, absTarget) {
		return "", errors.New("violación de seguridad: archivo fuera del directorio permitido")
	}

	if evalTarget, eErr := filepath.EvalSymlinks(absTarget); eErr == nil {
		if !isPathWithinBaseCPE(evalStorage, evalTarget) {
			return "", errors.New("violación de seguridad: archivo fuera del directorio permitido")
		}
		return evalTarget, nil
	} else if !os.IsNotExist(eErr) {
		return "", errors.New("violación de seguridad: no se pudo validar la ruta real del archivo")
	}

	return absTarget, nil
}

func (s *Service) safeFileExists(rawPath string) bool {
	resolved, err := s.resolveSafePath(rawPath)
	if err != nil {
		return false
	}
	info, err := os.Stat(resolved)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func isPathWithinBaseCPE(baseDir, targetPath string) bool {
	baseClean := filepath.Clean(baseDir)
	targetClean := filepath.Clean(targetPath)
	if filepath.Separator == '\\' {
		baseClean = strings.ToLower(baseClean)
		targetClean = strings.ToLower(targetClean)
	}
	rel, err := filepath.Rel(baseClean, targetClean)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	return true
}

func (s *Service) readSafeFile(path string) ([]byte, error) {
	resolved, err := s.resolveSafePath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(resolved)
}
