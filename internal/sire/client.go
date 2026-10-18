package sire

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/SwodLore/sire-api/internal/sunat"
)

type SunatSireClient struct {
	client *sunat.Client
}

func NewSunatSireClient(client *sunat.Client) *SunatSireClient {
	return &SunatSireClient{client: client}
}

// ExportarPropuestaVentas solicita a SUNAT la exportación asíncrona de la propuesta RVIE (Ventas)
// Manual Ventas v31, Secc. 5.18 (Pág. 49): codTipoArchivo solo acepta 0 (txt) o 1 (xls).
// Nota: 5.18 NO incluye codOrigenEnvio en URL ni parámetros.
func (s *SunatSireClient) ExportarPropuestaVentas(ruc, periodo string, f FiltrosExportacion) (string, error) {
	req, err := s.client.R(ruc)
	if err != nil {
		return "", fmt.Errorf("error al inicializar autenticación SUNAT: %w", err)
	}

	url := fmt.Sprintf("%s/v1/contribuyente/migeigv/libros/rvie/propuesta/web/propuesta/%s/exportapropuesta", s.client.BaseURL, periodo)

	codTipo := f.CodTipoArchivo
	if codTipo != "1" {
		codTipo = "0" // Por defecto y norma de 5.18: 0 = txt, 1 = xls
	}

	req.SetHeader("Accept", "application/json").
		SetQueryParam("codTipoArchivo", codTipo)

	if f.MtoTotalDesde != "" {
		req.SetQueryParam("mtoTotalDesde", f.MtoTotalDesde)
	}
	if f.MtoTotalHasta != "" {
		req.SetQueryParam("mtoTotalHasta", f.MtoTotalHasta)
	}
	if f.FecDocumentoDesde != "" {
		req.SetQueryParam("fecDocumentoDesde", f.FecDocumentoDesde)
	}
	if f.FecDocumentoHasta != "" {
		req.SetQueryParam("fecDocumentoHasta", f.FecDocumentoHasta)
	}
	if f.NumRucAdquiriente != "" {
		req.SetQueryParam("numRucAdquiriente", f.NumRucAdquiriente)
	}
	if f.NumCarSunat != "" {
		req.SetQueryParam("numCarSunat", f.NumCarSunat)
	}
	if f.CodTipoCDP != "" {
		req.SetQueryParam("codTipoCDP", f.CodTipoCDP)
	}
	if f.CodTipoInconsistencia != "" {
		req.SetQueryParam("codTipoInconsistencia", f.CodTipoInconsistencia)
	}

	resp, err := req.Get(url)
	if err != nil {
		return "", fmt.Errorf("error de red al solicitar exportación RVIE: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("SUNAT rechazó solicitud de exportación (HTTP %d): %s", resp.StatusCode(), string(resp.Body()))
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &data); err != nil {
		return "", fmt.Errorf("respuesta de SUNAT con JSON inválido: %w", err)
	}

	ticket := extractTicket(data)
	if ticket == "" {
		return "", fmt.Errorf("SUNAT no devolvió número de ticket en la respuesta: %s", string(resp.Body()))
	}

	return ticket, nil
}

// ExportarPropuestaCompras solicita a SUNAT la exportación asíncrona de la propuesta RCE (Compras)
// Manual Compras v28, Secc. 5.34 (Pág. 88-89): codTipoArchivo solo acepta 0 (txt) o 1 (csv).
// codOrigenEnvio es OBLIGATORIO con valor 2 (Servicio API).
func (s *SunatSireClient) ExportarPropuestaCompras(ruc, periodo string, f FiltrosExportacion) (string, error) {
	req, err := s.client.R(ruc)
	if err != nil {
		return "", fmt.Errorf("error al inicializar autenticación SUNAT: %w", err)
	}

	url := fmt.Sprintf("%s/v1/contribuyente/migeigv/libros/rce/propuesta/web/propuesta/%s/exportacioncomprobantepropuesta", s.client.BaseURL, periodo)

	codTipo := f.CodTipoArchivo
	if codTipo != "1" {
		codTipo = "0" // Por defecto y norma de 5.34: 0 = txt, 1 = csv
	}
	// Manual Compras v28, Secc. 5.34 (Pág. 89): "codOrigenEnvio: 2 Servicio API (Obligatorio)"
	codOrigen := "2"

	req.SetHeader("Accept", "application/json").
		SetQueryParam("codTipoArchivo", codTipo).
		SetQueryParam("codOrigenEnvio", codOrigen)

	if f.MtoTotalDesde != "" {
		req.SetQueryParam("mtoTotalDesde", f.MtoTotalDesde)
	}
	if f.MtoTotalHasta != "" {
		req.SetQueryParam("mtoTotalHasta", f.MtoTotalHasta)
	}
	if f.FecDocumentoDesde != "" {
		req.SetQueryParam("fecDocumentoDesde", f.FecDocumentoDesde)
	}
	if f.FecDocumentoHasta != "" {
		req.SetQueryParam("fecDocumentoHasta", f.FecDocumentoHasta)
	}
	if f.CodTipoCDP != "" {
		req.SetQueryParam("codTipoCDP", f.CodTipoCDP)
	}

	resp, err := req.Get(url)
	if err != nil {
		return "", fmt.Errorf("error de red al solicitar exportación RCE: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("SUNAT rechazó solicitud de exportación (HTTP %d): %s", resp.StatusCode(), string(resp.Body()))
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &data); err != nil {
		return "", fmt.Errorf("respuesta de SUNAT con JSON inválido: %w", err)
	}

	ticket := extractTicket(data)
	if ticket == "" {
		return "", fmt.Errorf("SUNAT no devolvió número de ticket en la respuesta: %s", string(resp.Body()))
	}

	return ticket, nil
}

// ConsultarEstadoTicket consulta en SUNAT el estado de procesamiento de un ticket
// Manual Ventas v31, Secc. 5.16 (Pág. 45): requiere codLibro=140000 y codOrigenEnvio=2
// Manual Compras v28, Secc. 5.31 (Pág. 83-84): requiere codLibro=080000 y codOrigenEnvio=2
func (s *SunatSireClient) ConsultarEstadoTicket(ruc, periodoIni, periodoFin, ticket, codLibro string) (*TicketStatusSUNAT, error) {
	req, err := s.client.R(ruc)
	if err != nil {
		return nil, fmt.Errorf("error al inicializar autenticación: %w", err)
	}

	url := fmt.Sprintf("%s/v1/contribuyente/migeigv/libros/rvierce/gestionprocesosmasivos/web/masivo/consultaestadotickets", s.client.BaseURL)

	if periodoIni == "" {
		periodoIni = "202401"
	}
	if periodoFin == "" {
		periodoFin = periodoIni
	}

	req.SetHeader("Accept", "application/json").
		SetQueryParam("perIni", periodoIni).
		SetQueryParam("perFin", periodoFin).
		SetQueryParam("page", "1").
		SetQueryParam("perPage", "20").
		SetQueryParam("codOrigenEnvio", "2") // Obligatorio en Ventas 5.16 y Compras 5.31

	if codLibro != "" {
		req.SetQueryParam("codLibro", codLibro)
	}
	if ticket != "" {
		req.SetQueryParam("numTicket", ticket)
	}

	resp, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error de red al consultar ticket: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("SUNAT rechazó consulta de ticket (HTTP %d): %s", resp.StatusCode(), string(resp.Body()))
	}

	var res ConsultaEstadoTicketsResponse
	if err := json.Unmarshal(resp.Body(), &res); err != nil {
		return nil, fmt.Errorf("respuesta de estado de ticket no es JSON válido: %w", err)
	}

	if len(res.Registros) == 0 {
		return nil, fmt.Errorf("ticket %s no encontrado en SUNAT", ticket)
	}

	// Buscar el ticket exacto si hay varios
	for _, reg := range res.Registros {
		if ticket == "" || reg.NumTicket == ticket {
			return &reg, nil
		}
	}

	return &res.Registros[0], nil
}

// DescargarArchivoReporte descarga el archivo binario generado por SUNAT
// Manual Ventas v31, Secc. 5.17 (Pág. 47-48) y Manual Compras v28, Secc. 5.32 (Pág. 86-87):
// Exigen obligatoriamente: nomArchivoReporte, codTipoArchivoReporte (o "null"), perTributario, codProceso, numTicket, codLibro
// Headers: Content-Type: application/json, Accept: application/json
func (s *SunatSireClient) DescargarArchivoReporte(ruc, nomArchivoReporte, codTipoArchivoReporte, perTributario, codProceso, numTicket, codLibro string) ([]byte, error) {
	req, err := s.client.R(ruc)
	if err != nil {
		return nil, fmt.Errorf("error al inicializar autenticación: %w", err)
	}

	url := fmt.Sprintf("%s/v1/contribuyente/migeigv/libros/rvierce/gestionprocesosmasivos/web/masivo/archivoreporte", s.client.BaseURL)

	if codTipoArchivoReporte == "" {
		codTipoArchivoReporte = "null"
	}
	if codProceso == "" {
		return nil, fmt.Errorf("parámetro 'codProceso' es obligatorio para archivoreporte y no fue provisto")
	}

	req.SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetQueryParam("nomArchivoReporte", nomArchivoReporte).
		SetQueryParam("codTipoArchivoReporte", codTipoArchivoReporte).
		SetQueryParam("perTributario", perTributario).
		SetQueryParam("codProceso", codProceso).
		SetQueryParam("numTicket", numTicket).
		SetQueryParam("codLibro", codLibro)

	resp, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error de red al descargar archivo de reporte: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("SUNAT rechazó descarga de archivo (HTTP %d): %s", resp.StatusCode(), string(resp.Body()))
	}

	body := resp.Body()
	if len(body) == 0 {
		return nil, fmt.Errorf("SUNAT entregó archivo vacío (0 bytes)")
	}

	return body, nil
}

// ConsultarDetallePropuesta consulta los comprobantes individuales de la propuesta con paginación y filtros
func (s *SunatSireClient) ConsultarDetallePropuesta(ruc, codLibro, periodo string, f FiltrosPropuestaDetalle) (map[string]interface{}, error) {
	req, err := s.client.R(ruc)
	if err != nil {
		return nil, fmt.Errorf("error al inicializar autenticación: %w", err)
	}

	var endpoint string
	if codLibro == "140000" {
		endpoint = fmt.Sprintf("%s/v1/contribuyente/migeigv/libros/rvie/propuesta/web/propuesta/%s/comprobantes", s.client.BaseURL, periodo)
	} else {
		endpoint = fmt.Sprintf("%s/v1/contribuyente/migeigv/libros/rce/propuesta/web/propuesta/%s/comprobantes", s.client.BaseURL, periodo)
	}

	page := f.Page
	if page == "" {
		page = "1"
	}
	perPage := f.PerPage
	if perPage == "" {
		perPage = "20"
	}

	req.SetHeader("Accept", "application/json").
		SetQueryParam("page", page).
		SetQueryParam("perPage", perPage)

	if f.CodTipoCDP != "" {
		req.SetQueryParam("codTipoCDP", f.CodTipoCDP)
	}
	if f.NumSerieCDP != "" {
		req.SetQueryParam("numSerieCDP", f.NumSerieCDP)
	}
	if f.NumCDP != "" {
		req.SetQueryParam("numCDP", f.NumCDP)
	}
	if f.FecEmisionIni != "" {
		req.SetQueryParam("fecEmisionIni", f.FecEmisionIni)
	}
	if f.FecEmisionFin != "" {
		req.SetQueryParam("fecEmisionFin", f.FecEmisionFin)
	}
	if f.NumDocIdentidad != "" {
		req.SetQueryParam("numDocIdentidad", f.NumDocIdentidad)
	}

	resp, err := req.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("error de red al consultar detalle de propuesta: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("SUNAT respondió HTTP %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("respuesta de SUNAT con JSON inválido: %w", err)
	}

	return result, nil
}

func extractTicket(data map[string]interface{}) string {
	if t, ok := data["numTicket"].(string); ok && t != "" {
		return strings.TrimSpace(t)
	}
	if t, ok := data["ticket"].(string); ok && t != "" {
		return strings.TrimSpace(t)
	}
	return ""
}
