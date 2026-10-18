package services

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"

	models "github.com/SwodLore/sire-api/VENTAS/models"
	"github.com/SwodLore/sire-api/internal/sunat"
)

// Package services contiene las implementaciones de servicios del diseño original.
//
// Deprecated: Este paquete y sus servicios pertenecen a la versión monolítica anterior.
// Para exportaciones, descargas, consultas de tickets y procesamiento de propuestas conforme
// a los manuales vigentes de SUNAT (Ventas v31 y Compras v28), utilice el cliente unificado
// y asíncrono en internal/sire (SunatSireClient y Worker). No utilizar estos servicios para nuevas funciones.
type Service struct {
	client *sunat.Client
	ruc    string
}

func New(client *sunat.Client, ruc string) *Service {
	return &Service{
		client: client,
		ruc:    ruc,
	}
}


// 2 Servicio Web Api consultar año y mes
func (srv *Service) ConsultarPeriodos(codLibro string) ([]models.Periodo, error) {
	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/padron/web/omisos/%s/periodos", codLibro)

	log.Println("GET a:", url)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, fmt.Errorf("error al consultar periodos: %s - %s", resp.Status(), string(resp.Body()))
	}

	var periodos []models.Periodo
	if err := json.Unmarshal(resp.Body(), &periodos); err != nil {
		return nil, fmt.Errorf("error al parsear JSON: %w", err)
	}

	return periodos, nil
}

// 3 Servicio Web Api importar reemplazo de la propuesta
func (srv *Service) ImportarReemplazoPropuestaSUNAT(perTributario, filename, filetype, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {




	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpropuesta/web/propuesta/upload")

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Tus-Resumable", "1.0.0").
		SetHeader("Upload-Length", "0").
		SetHeader("Upload-Metadata", "filename "+filename+",filetype "+filetype+",numRuc "+srv.ruc+",perTributario "+perTributario+",codOrigenEnvio "+codOrigenEnvio+",codProceso "+codProceso+",codTipoCorrelativo "+codTipoCorrelativo+",nomArchivoImportacion "+nomArchivoImportacion+",codLibro "+codLibro).
		Post(url)

	if err != nil {
		return fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), resp.String())
	}

	return nil
}

// 4 Servicio Web Api importar nuevos comprobantes propuesta
func (srv *Service) ImportarNuevosComprobantesSUNAT(perTributario, filename, filetype, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {




	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpropuesta/web/propuesta/upload")

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Tus-Resumable", "1.0.0").
		SetHeader("Upload-Length", "0").
		SetHeader("Upload-Metadata", "filename "+filename+",filetype "+filetype+",numRuc "+srv.ruc+",perTributario "+perTributario+",codOrigenEnvio "+codOrigenEnvio+",codProceso "+codProceso+",codTipoCorrelativo "+codTipoCorrelativo+",nomArchivoImportacion "+nomArchivoImportacion+",codLibro "+codLibro).
		Post(url)

	if err != nil {
		return fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), resp.String())
	}

	return nil
}

// 5 Servicio Web Api importar nuevos comprobantes preliminar
func (srv *Service) ImportarComprobantesPreliminarSUNAT(perTributario, filename, filetype, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) (string, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpreliminar/web/preliminar/upload"

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeaders(map[string]string{
			"Content-Type":  "application/x-www-form-urlencoded",
			"Accept":        "application/json",
			"Tus-Resumable": "1.0.0",
			"Upload-Length": "0",
			"Upload-Metadata": fmt.Sprintf("filename %s,filetype %s,perTributario %s,codOrigenEnvio %s,codProceso %s,codTipoCorrelativo %s,nomArchivoImportacion %s,codLibro %s",
				base64.StdEncoding.EncodeToString([]byte(filename)),
				base64.StdEncoding.EncodeToString([]byte(filetype)),
				base64.StdEncoding.EncodeToString([]byte(perTributario)),
				base64.StdEncoding.EncodeToString([]byte(codOrigenEnvio)),
				base64.StdEncoding.EncodeToString([]byte(codProceso)),
				base64.StdEncoding.EncodeToString([]byte(codTipoCorrelativo)),
				base64.StdEncoding.EncodeToString([]byte(nomArchivoImportacion)),
				base64.StdEncoding.EncodeToString([]byte(codLibro))),
		}).
		Post(url)

	if err != nil {
		return "", fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 201 {
		return "", fmt.Errorf("error SUNAT - Status: %d, Body: %s", resp.StatusCode(), resp.String())
	}

	bodyStr := resp.String()
	if bodyStr == "" {
		return fmt.Sprintf("Success-%d", resp.StatusCode()), nil
	}

	var response struct {
		NumTicket string `json:"numTicket"`
	}

	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return bodyStr, nil
	}

	if response.NumTicket != "" {
		return response.NumTicket, nil
	}

	return bodyStr, nil
}

// 6 Servicio Web Api importar ajustes posteriores
func (srv *Service) ImportarAjustesPosterioresSUNAT(perTributario, filename, filetype, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) (string, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorajustesposteriores/web/ajustesposteriores/upload"

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeaders(map[string]string{
			"Content-Type":  "application/x-www-form-urlencoded",
			"Accept":        "application/json",
			"Tus-Resumable": "1.0.0",
			"Upload-Length": "0",
			"Upload-Metadata": fmt.Sprintf("filename %s,filetype %s,perTributario %s,codOrigenEnvio %s,codProceso %s,codTipoCorrelativo %s,nomArchivoImportacion %s,codLibro %s",
				base64.StdEncoding.EncodeToString([]byte(filename)),
				base64.StdEncoding.EncodeToString([]byte(filetype)),
				base64.StdEncoding.EncodeToString([]byte(perTributario)),
				base64.StdEncoding.EncodeToString([]byte(codOrigenEnvio)),
				base64.StdEncoding.EncodeToString([]byte(codProceso)),
				base64.StdEncoding.EncodeToString([]byte(codTipoCorrelativo)),
				base64.StdEncoding.EncodeToString([]byte(nomArchivoImportacion)),
				base64.StdEncoding.EncodeToString([]byte(codLibro))),
		}).
		Post(url)

	if err != nil {
		return "", fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 201 {
		return "", fmt.Errorf("error SUNAT - Status: %d, Body: %s", resp.StatusCode(), resp.String())
	}

	bodyStr := resp.String()
	if bodyStr == "" {
		return fmt.Sprintf("Success-%d", resp.StatusCode()), nil
	}

	var response struct {
		NumTicket string `json:"numTicket"`
	}

	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return bodyStr, nil
	}

	if response.NumTicket != "" {
		return response.NumTicket, nil
	}

	return bodyStr, nil
}

// 7 Servicio Web Api importar ajustes posteriores de periodos anteriores
func (srv *Service) ImportarAjustesPeriodosAnterioresSUNAT(perTributario, filename, filetype, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) (string, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorajustesposteriores/web/ajustesposteriores/upload"

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeaders(map[string]string{
			"Content-Type":  "application/x-www-form-urlencoded",
			"Accept":        "application/json",
			"Tus-Resumable": "1.0.0",
			"Upload-Length": "0",
			"Upload-Metadata": fmt.Sprintf("filename %s,filetype %s,perTributario %s,codOrigenEnvio %s,codProceso %s,codTipoCorrelativo %s,nomArchivoImportacion %s,codLibro %s",
				base64.StdEncoding.EncodeToString([]byte(filename)),
				base64.StdEncoding.EncodeToString([]byte(filetype)),
				base64.StdEncoding.EncodeToString([]byte(perTributario)),
				base64.StdEncoding.EncodeToString([]byte(codOrigenEnvio)),
				base64.StdEncoding.EncodeToString([]byte(codProceso)),
				base64.StdEncoding.EncodeToString([]byte(codTipoCorrelativo)),
				base64.StdEncoding.EncodeToString([]byte(nomArchivoImportacion)),
				base64.StdEncoding.EncodeToString([]byte(codLibro))),
		}).
		Post(url)

	if err != nil {
		return "", fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 201 {
		return "", fmt.Errorf("error SUNAT - Status: %d, Body: %s", resp.StatusCode(), resp.String())
	}

	bodyStr := resp.String()
	if bodyStr == "" {
		return fmt.Sprintf("Success-%d", resp.StatusCode()), nil
	}

	var response struct {
		NumTicket string `json:"numTicket"`
	}

	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return bodyStr, nil
	}

	if response.NumTicket != "" {
		return response.NumTicket, nil
	}

	return bodyStr, nil
}

// 8 Servicio Web Api aceptar propuesta del RVIE
func (srv *Service) AceptarPropuestaRVIESUNAT(perTributario string) (string, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvie/propuesta/web/propuesta/%s/aceptapropuesta", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeaders(map[string]string{
			"Content-Type":  "application/json",
			"Accept":        "application/json",
		}).
		Post(url)

	if err != nil {
		return "", fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 201 {
		return "", fmt.Errorf("error SUNAT - Status: %d, Body: %s", resp.StatusCode(), resp.String())
	}

	bodyStr := resp.String()
	if bodyStr == "" {
		return fmt.Sprintf("Success-%d", resp.StatusCode()), nil
	}

	var response struct {
		NumTicket string `json:"numTicket"`
	}

	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return bodyStr, nil
	}

	if response.NumTicket != "" {
		return response.NumTicket, nil
	}

	return bodyStr, nil
}

// 9 Servicio Web Api registrar preliminar
func (srv *Service) RegistrarPreliminarSUNAT(perTributario string) error {

	url := fmt.Sprintf(
		"https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/registroslibros/%s/registrapreliminar",
		perTributario,
	)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded"). // ✅ CAMBIO AQUÍ
		SetHeader("Accept", "application/json").
		Post(url)

	if err != nil {
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("error al registrar preliminar: %s - %s", resp.Status(), string(resp.Body()))
	}

	return nil
}

// 10 Servicio Web Api exclusión definitiva de notas de crédito y facturas
func (srv *Service) ExclusionDefinitivaSUNAT(perTributario, codCar, codSituacion string) error {

	url := fmt.Sprintf(
		"https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvie/propuesta/web/propuesta/%s/retiracomprobante?codCar=%s&codSituacion=%s",
		perTributario,
		codCar,
		codSituacion,
	)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		Post(url)

	if err != nil {
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("error al excluir comprobante: %s - %s", resp.Status(), string(resp.Body()))
	}

	return nil
}

// 11 Servicio Web Api agregar tipo de cambio masivo
func (srv *Service) TipoCambioMasivoSUNAT(perTributario string, tipoCambios []models.TipoCambioRequest) error {

	url := fmt.Sprintf(
		"https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvie/propuesta/web/masivo/%s/guardacomplementomasivo",
		perTributario,
	)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(tipoCambios).
		Post(url)

	if err != nil {
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("error al actualizar tipos de cambio: %s - %s", resp.Status(), string(resp.Body()))
	}

	return nil
}

// 12 Servicio Web Api editar tipo de cambio individual
func (srv *Service) EditarTipoCambioSUNAT(perTributario string, tipoCambio models.TipoCambioIndividualRequest) error {

	url := fmt.Sprintf(
		"https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvie/propuesta/web/propuesta/%s/complementoindividual",
		perTributario,
	)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(tipoCambio).
		Put(url)

	if err != nil {
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("error al editar tipo de cambio: %s - %s", resp.Status(), string(resp.Body()))
	}

	return nil
}

// 13 Servicio Web Api eliminar comprobante propuesta
func (srv *Service) EliminarComprobanteSUNAT(perTributario string, comprobantes []models.EliminarComprobanteRequest) error {

	url := fmt.Sprintf(
		"https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvie/propuesta/web/propuesta/%s/eliminacomprobante",
		perTributario,
	)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(comprobantes).
		Delete(url)

	if err != nil {
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("error al eliminar comprobantes: %s - %s", resp.Status(), string(resp.Body()))
	}

	return nil
}

// 14 Servicio Web Api eliminar comprobante preliminar
func (srv *Service) EliminarComprobantePreliminarSUNAT(perTributario string, comprobantes []models.ComprobanteEliminar) error {

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/registroslibros/%s/comprobantepreliminar", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(comprobantes).
		Put(url)

	if err != nil {
		return fmt.Errorf("error HTTP: %v", err)
	}

	switch resp.StatusCode() {
	case 200:
		return nil
	case 401:
		return fmt.Errorf("token inválido: %s", string(resp.Body()))
	case 422:
		return fmt.Errorf("error validación SUNAT: %s", string(resp.Body()))
	case 500:
		return fmt.Errorf("error servidor SUNAT: %s", string(resp.Body()))
	default:
		return fmt.Errorf("error %d: %s", resp.StatusCode(), string(resp.Body()))
	}
}

// 15 Servicio Web Api eliminar reemplazo propuesta
func (srv *Service) EliminarReemplazoPropuestaSUNAT(perTributario string, codLibro string) error {

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/registroslibros/%s/eliminarreemplazo?codLibro=%s", perTributario, codLibro)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Put(url)

	if err != nil {
		return fmt.Errorf("error HTTP: %v", err)
	}

	switch resp.StatusCode() {
	case 200:
		return nil
	case 401:
		return fmt.Errorf("token inválido: %s", string(resp.Body()))
	case 422:
		return fmt.Errorf("error validación SUNAT: %s", string(resp.Body()))
	case 500:
		return fmt.Errorf("error servidor SUNAT: %s", string(resp.Body()))
	default:
		return fmt.Errorf("error %d: %s", resp.StatusCode(), string(resp.Body()))
	}
}

// 16 Servicio Web Api consultar estado de envío de ticket
func (srv *Service) ConsultarEstadoTicketSUNAT(perIni, perFin, page, perPage, numTicket string) (map[string]interface{}, error) {

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionprocesosmasivos/web/masivo/consultaestadotickets?perIni=%s&perFin=%s&page=%s&perPage=%s", perIni, perFin, page, perPage)

	if numTicket != "" {
		url += "&numTicket=" + numTicket
	}

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error HTTP: %v", err)
	}

	switch resp.StatusCode() {
	case 200:
		var respuesta map[string]interface{}
		if err := json.Unmarshal(resp.Body(), &respuesta); err != nil {
			return nil, fmt.Errorf("error parsing: %v", err)
		}
		return respuesta, nil
	case 401:
		return nil, fmt.Errorf("token inválido: %s", string(resp.Body()))
	case 422:
		return nil, fmt.Errorf("error validación SUNAT: %s", string(resp.Body()))
	case 500:
		return nil, fmt.Errorf("error servidor SUNAT: %s", string(resp.Body()))
	default:
		return nil, fmt.Errorf("error %d: %s", resp.StatusCode(), string(resp.Body()))
	}
}

// 17 Servicio Web Api descargar archivo
func (srv *Service) DescargarArchivoSUNAT(nomArchivoReporte, codTipoArchivoReporte, codLibro string) ([]byte, error) {

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionprocesosmasivos/web/masivo/archivoreporte"

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetQueryParam("nomArchivoReporte", nomArchivoReporte).
		SetQueryParam("codTipoArchivoReporte", codTipoArchivoReporte).
		SetQueryParam("codLibro", codLibro).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}

// 18 Servicio Web Api descargar propuesta
func (srv *Service) DescargarPropuestaSUNAT(perTributario, codTipoArchivo, mtoTotalDesde, mtoTotalHasta, fecDocumentoDesde, fecDocumentoHasta, numRucAdquiriente, numCarSunat, codTipoCDP, codTipoInconsistencia string) (map[string]interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvie/propuesta/web/propuesta/%s/exportapropuesta", perTributario)

	req := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParam("codTipoArchivo", codTipoArchivo)

	if mtoTotalDesde != "" {
		req.SetQueryParam("mtoTotalDesde", mtoTotalDesde)
	}
	if mtoTotalHasta != "" {
		req.SetQueryParam("mtoTotalHasta", mtoTotalHasta)
	}
	if fecDocumentoDesde != "" {
		req.SetQueryParam("fecDocumentoDesde", fecDocumentoDesde)
	}
	if fecDocumentoHasta != "" {
		req.SetQueryParam("fecDocumentoHasta", fecDocumentoHasta)
	}
	if numRucAdquiriente != "" {
		req.SetQueryParam("numRucAdquiriente", numRucAdquiriente)
	}
	if numCarSunat != "" {
		req.SetQueryParam("numCarSunat", numCarSunat)
	}
	if codTipoCDP != "" {
		req.SetQueryParam("codTipoCDP", codTipoCDP)
	}
	if codTipoInconsistencia != "" {
		req.SetQueryParam("codTipoInconsistencia", codTipoInconsistencia)
	}

	resp, err := req.Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), resp.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta: %v", err)
	}

	return result, nil
}

// 19 Servicio Web Api descargar no incluidos
func (srv *Service) DescargarNoIncluidosSUNAT(perTributario, codTipoArchivo, mtoTotalDesde, mtoTotalHasta, fecDocumentoDesde, fecDocumentoHasta, numRucAdquiriente, numCarSunat, codTipoCDP, codTipoInconsistencia, codOrigenEnvio string) (map[string]interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvie/propuesta/web/noincluidos/%s/exportanoincluidos", perTributario)

	req := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParam("codTipoArchivo", codTipoArchivo)

	if mtoTotalDesde != "" {
		req.SetQueryParam("mtoTotalDesde", mtoTotalDesde)
	}
	if mtoTotalHasta != "" {
		req.SetQueryParam("mtoTotalHasta", mtoTotalHasta)
	}
	if fecDocumentoDesde != "" {
		req.SetQueryParam("fecDocumentoDesde", fecDocumentoDesde)
	}
	if fecDocumentoHasta != "" {
		req.SetQueryParam("fecDocumentoHasta", fecDocumentoHasta)
	}
	if numRucAdquiriente != "" {
		req.SetQueryParam("numRucAdquiriente", numRucAdquiriente)
	}
	if numCarSunat != "" {
		req.SetQueryParam("numCarSunat", numCarSunat)
	}
	if codTipoCDP != "" {
		req.SetQueryParam("codTipoCDP", codTipoCDP)
	}
	if codTipoInconsistencia != "" {
		req.SetQueryParam("codTipoInconsistencia", codTipoInconsistencia)
	}
	if codOrigenEnvio != "" {
		req.SetQueryParam("codOrigenEnvio", codOrigenEnvio)
	}

	resp, err := req.Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), resp.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta: %v", err)
	}

	return result, nil
}

// 20 Servicio Web Api descargar resumen
func (srv *Service) DescargarResumenSUNAT(perTributario, codTipoResumen, codTipoArchivo, codLibro string) ([]byte, error) {

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/resumen/web/resumencomprobantes/%s/%s/%s/exporta",
		perTributario, codTipoResumen, codTipoArchivo)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParam("codLibro", codLibro).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}

// 21 Servicio Web Api descargar resumen inconsistencias
func (srv *Service) DescargarResumenInconsistenciasSUNAT(perPeriodoTributario, codTipoResumen, codLibro string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/resumen/web/resumeninconsistencias/%s?codTipoResumen=%s&codLibro=%s",
		perPeriodoTributario, codTipoResumen, codLibro)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeaders(map[string]string{
			"Content-Type":  "application/json",
			"Accept":        "application/json",
		}).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT - Status: %d, Body: %s", resp.StatusCode(), resp.String())
	}

	var result interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	return result, nil
}

// 22 Servicio Web Api exportar preliminar de registro de Ventas
func (srv *Service) ExportarPreliminarVentasSUNAT(perTributario, codTipoArchivo, codOrigenEnvio, codLibro, mtoTotalDesde, mtoTotalHasta, fecDocumentoDesde, fecDocumentoHasta, numRucAdquiriente, numCarSunat, codTipoCDP, codTipoInconsistencia string) (string, error) {


	baseURL := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/registroslibros/%s/reportepreliminar", perTributario)

	params := url.Values{}
	params.Add("codTipoArchivo", codTipoArchivo)
	params.Add("codOrigenEnvio", codOrigenEnvio)
	params.Add("codLibro", codLibro)

	if mtoTotalDesde != "" {
		params.Add("mtoTotalDesde", mtoTotalDesde)
	}
	if mtoTotalHasta != "" {
		params.Add("mtoTotalHasta", mtoTotalHasta)
	}
	if fecDocumentoDesde != "" {
		params.Add("fecDocumentoDesde", fecDocumentoDesde)
	}
	if fecDocumentoHasta != "" {
		params.Add("fecDocumentoHasta", fecDocumentoHasta)
	}
	if numRucAdquiriente != "" {
		params.Add("numRucAdquiriente", numRucAdquiriente)
	}
	if numCarSunat != "" {
		params.Add("numCarSunat", numCarSunat)
	}
	if codTipoCDP != "" {
		params.Add("codTipoCDP", codTipoCDP)
	}
	if codTipoInconsistencia != "" {
		params.Add("codTipoInconsistencia", codTipoInconsistencia)
	}

	finalURL := baseURL + "?" + params.Encode()

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeaders(map[string]string{
			"Content-Type":  "application/json",
			"Accept":        "application/json",
		}).
		Get(finalURL)

	if err != nil {
		return "", fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("error SUNAT - Status: %d, Body: %s", resp.StatusCode(), resp.String())
	}

	bodyStr := resp.String()
	if bodyStr == "" {
		return fmt.Sprintf("Success-%d", resp.StatusCode()), nil
	}

	var response struct {
		NumTicket string `json:"numTicket"`
	}

	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return bodyStr, nil
	}

	if response.NumTicket != "" {
		return response.NumTicket, nil
	}

	return bodyStr, nil
}

// 23 Servicio Web Api descargar reporte de casillas
func (srv *Service) DescargarReporteCasillasSUNAT(perTributario, tipoReporte, tipoDescarga string) (string, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/casillas/e/casillaspropuestas/%s/reporte/%s/%s",
		perTributario, tipoReporte, tipoDescarga)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeaders(map[string]string{
			"Content-Type":  "application/json",
			"Accept":        "application/json",
		}).
		Get(url)

	if err != nil {
		return "", fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("error SUNAT - Status: %d, Body: %s", resp.StatusCode(), resp.String())
	}

	bodyStr := resp.String()
	if bodyStr == "" {
		return fmt.Sprintf("Success-%d", resp.StatusCode()), nil
	}

	var response struct {
		NumTicket string `json:"numTicket"`
	}

	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return bodyStr, nil
	}

	if response.NumTicket != "" {
		return response.NumTicket, nil
	}

	return bodyStr, nil
}

// 24 Servicio Web Api descargar inconsistencias en registros preliminar registrado
func (srv *Service) DescargarInconsistenciasPreliminarSUNAT(perTributario, numCas, cntlimite, mtoNoSustentado, desMtoNoSustentado string) ([]byte, error) {


	baseURL := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/casillas/inconsistenciaslibros/%s/%s/reporteinconsistencia",
		perTributario, numCas)

	params := url.Values{}
	params.Add("cntlimite", cntlimite)

	if mtoNoSustentado != "" {
		params.Add("mtoNoSustentado", mtoNoSustentado)
	}
	if desMtoNoSustentado != "" {
		params.Add("desMtoNoSustentado", desMtoNoSustentado)
	}

	finalURL := baseURL + "?" + params.Encode()

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeaders(map[string]string{
			"Content-Type":  "application/json",
			"Accept":        "application/json",
		}).
		Get(finalURL)

	if err != nil {
		return nil, fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 201 && resp.StatusCode() != 204 {
		return nil, fmt.Errorf("error SUNAT - Status: %d, Body: %s", resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}

// 25 Servicio Web Api descargar inconsistencias por comprobante pago
func (srv *Service) DescargarInconsistenciasComprobantesSUNAT(perTributario, codTipoArchivo, fecEmisionIni, fecEmisionFin, codInconsistencia, numDocAdquiriente, codTipoCDP, numSerieCDP, numCDP, s, ord string) (map[string]interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvie/inconsistencias/web/periodoinconsistencias/%s/exporta", perTributario)

	req := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetQueryParam("codTipoArchivo", codTipoArchivo)

	if fecEmisionIni != "" {
		req.SetQueryParam("fecEmisionIni", fecEmisionIni)
	}
	if fecEmisionFin != "" {
		req.SetQueryParam("fecEmisionFin", fecEmisionFin)
	}
	if codInconsistencia != "" {
		req.SetQueryParam("codInconsistencia", codInconsistencia)
	}
	if numDocAdquiriente != "" {
		req.SetQueryParam("numDocAdquiriente", numDocAdquiriente)
	}
	if codTipoCDP != "" {
		req.SetQueryParam("codTipoCDP", codTipoCDP)
	}
	if numSerieCDP != "" {
		req.SetQueryParam("numSerieCDP", numSerieCDP)
	}
	if numCDP != "" {
		req.SetQueryParam("numCDP", numCDP)
	}
	if s != "" {
		req.SetQueryParam("s", s)
	}
	if ord != "" {
		req.SetQueryParam("ord", ord)
	}

	resp, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 26 Servicio Web Api descargar constancia de recepción
func (srv *Service) DescargarConstanciaRecepcionSUNAT(nomArchivo string) (map[string]interface{}, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/registroslibros/constancia/archivo"

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetQueryParam("nomArchivo", nomArchivo).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 27 Servicio Web Api descargar RVIE por periodo
// 28 Servicio Web Api descargar reporte consolidado por periodo
// 29 Servicio Web Api descargar ajustes posteriores
// 30 Servicio Web Api descargar ajustes posteriores de periodos anteriores
// utiliza un mismo service para dos
func (srv *Service) DescargarRVIEPorPeriodoSUNAT(perTributario, codOrigenEnvio, codTipoArchivo, codProceso, codLibro string) (map[string]interface{}, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/registroslibros/descarga"

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetQueryParam("perTributario", perTributario).
		SetQueryParam("codOrigenEnvio", codOrigenEnvio).
		SetQueryParam("codTipoArchivo", codTipoArchivo).
		SetQueryParam("codProceso", codProceso).
		SetQueryParam("codLibro", codLibro).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 31 Servicio Web Api descargar reporte inconsistencias por periodo
func (srv *Service) DescargarInconsistenciasPorPeriodoSUNAT(codRegistroLibro, codOrigenEnvio string) (map[string]interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvie/inconsistencias/web/inconsistencia/%s/exporta", codRegistroLibro)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetQueryParam("codOrigenEnvio", codOrigenEnvio).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 32 Servicio Web Api descargar reporte CAR
func (srv *Service) DescargarReporteCARSUNAT(perTributario, codOrigenEnvio, codLibro, codFase string) (map[string]interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/comprobanteslibros/%s/reportecar", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetQueryParam("codOrigenEnvio", codOrigenEnvio).
		SetQueryParam("codLibro", codLibro).
		SetQueryParam("codFase", codFase).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 33 Servicio Web Api descargar reporte estadístico
func (srv *Service) DescargarReporteEstadisticoSUNAT(perTributario, codTipoArchivo, codTipoReporte, codLibro, fechaIni, fechaFin, codTipoCDP, codTipoDocIdentidadAdquiriente, numDocIdentidadAdquiriente string) (map[string]interface{}, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/estadistica/web/resumenestadistico/exportarvie"

	req := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetQueryParam("perTributario", perTributario).
		SetQueryParam("codTipoArchivo", codTipoArchivo).
		SetQueryParam("codTipoReporte", codTipoReporte).
		SetQueryParam("codLibro", codLibro)

	if srv.ruc != "" {
		req.SetQueryParam("numRuc", srv.ruc)
	}
	if fechaIni != "" {
		req.SetQueryParam("fechaIni", fechaIni)
	}
	if fechaFin != "" {
		req.SetQueryParam("fechaFin", fechaFin)
	}
	if codTipoCDP != "" {
		req.SetQueryParam("codTipoCDP", codTipoCDP)
	}
	if codTipoDocIdentidadAdquiriente != "" {
		req.SetQueryParam("codTipoDocIdentidadAdquiriente", codTipoDocIdentidadAdquiriente)
	}
	if numDocIdentidadAdquiriente != "" {
		req.SetQueryParam("numDocIdentidadAdquiriente", numDocIdentidadAdquiriente)
	}

	resp, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 34 Servicio Web Api descargar reporte de cumplimiento
func (srv *Service) DescargarReporteCumplimientoSUNAT(perTributario, codLibro string) (map[string]interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/cumplimiento/web/omisos/%s/%s/consultaReporteCumplimiento/exportardocumento", perTributario, codLibro)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 35 Servicio Web Api reporte de exportadores
func (srv *Service) ReporteExportadoresSUNAT(perTributario, codRegistroLibro string) (map[string]interface{}, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/exportadores/reporte"

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetQueryParam("perTributario", perTributario).
		SetQueryParam("codRegistroLibro", codRegistroLibro).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	contentType := resp.Header().Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			return nil, fmt.Errorf("error parseando JSON: %v", err)
		}
		return result, nil
	} else {
		return map[string]interface{}{
			"contenido": string(resp.Body()),
			"tipo":      contentType,
			"tamaño":    len(resp.Body()),
		}, nil
	}
}

// 36 Servicio Web Api eliminar preliminar registrado
func (srv *Service) EliminarPreliminarService(perTributario, codLibro, id, codTipoRegistro string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/registroslibros/%s/eliminapreliminar", perTributario)

	bodyData := map[string]string{
		"codTipoRegistro": codTipoRegistro,
	}

	if id != "" {
		bodyData["id"] = id
	}

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetQueryParam("codLibro", codLibro).
		SetBody(bodyData).
		Put(url)

	if err != nil {
		return fmt.Errorf("error en la petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}