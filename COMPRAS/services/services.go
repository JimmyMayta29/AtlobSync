package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

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


// 2 Servicio Web Api aceptar propuesta
func (srv *Service) AceptarPropuestaService(perTributario string) (map[string]interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/registroslibros/%s/aceptarpropuesta", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Post(url)

	if err != nil {
		return nil, fmt.Errorf("error en la petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta: %v", err)
	}

	return result, nil
}

// 3 Servicio Web Api importar reemplazo de la propuesta
func (srv *Service) ReemplazoPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpropuesta/web/propuesta/upload"

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetFormData(map[string]string{
			"filename":              filename,
			"filetype":              filetype,
			"numRuc":                numRuc,
			"perTributario":         perTributario,
			"codOrigenEnvio":        codOrigenEnvio,
			"codProceso":            codProceso,
			"codTipoCorrelativo":    codTipoCorrelativo,
			"nomArchivoImportacion": nomArchivoImportacion,
			"codLibro":              codLibro,
		}).
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 4 Servicio Web Api registrar preliminar
func (srv *Service) RegistrarPreliminarService(perTributario string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/preliminar/web/registroslibros/%s/registrapreliminares", perTributario)

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 5 Servicio Web Api cargar registro de compra no domiciliados
func (srv *Service) CargarNoDomiciliadosService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpreliminar/web/preliminar/upload"

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetFormData(map[string]string{
			"filename":              filename,
			"filetype":              filetype,
			"numRuc":                numRuc,
			"perTributario":         perTributario,
			"codOrigenEnvio":        codOrigenEnvio,
			"codProceso":            codProceso,
			"codTipoCorrelativo":    codTipoCorrelativo,
			"nomArchivoImportacion": nomArchivoImportacion,
			"codLibro":              codLibro,
		}).
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 6 Servicio Web Api importar datos complementarios de los CP de la propuesta
func (srv *Service) ImportarComplementoPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpropuesta/web/propuesta/upload"

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetFormData(map[string]string{
			"filename":              filename,
			"filetype":              filetype,
			"numRuc":                numRuc,
			"perTributario":         perTributario,
			"codOrigenEnvio":        codOrigenEnvio,
			"codProceso":            codProceso,
			"codTipoCorrelativo":    codTipoCorrelativo,
			"nomArchivoImportacion": nomArchivoImportacion,
			"codLibro":              codLibro,
		}).
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 7 Servicio Web Api importar nuevos comprobantes preliminar
func (srv *Service) ImportarComprobantesPreliminarService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpreliminar/web/preliminar/upload"

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetFormData(map[string]string{
			"filename":              filename,
			"filetype":              filetype,
			"numRuc":                numRuc,
			"perTributario":         perTributario,
			"codOrigenEnvio":        codOrigenEnvio,
			"codProceso":            codProceso,
			"codTipoCorrelativo":    codTipoCorrelativo,
			"nomArchivoImportacion": nomArchivoImportacion,
			"codLibro":              codLibro,
		}).
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 8 Servicio Web Api incluir-excluir comprobantes de la propuesta
func (srv *Service) IncluirExcluirPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpropuesta/web/propuesta/upload"

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetFormData(map[string]string{
			"filename":              filename,
			"filetype":              filetype,
			"numRuc":                numRuc,
			"perTributario":         perTributario,
			"codOrigenEnvio":        codOrigenEnvio,
			"codProceso":            codProceso,
			"codTipoCorrelativo":    codTipoCorrelativo,
			"nomArchivoImportacion": nomArchivoImportacion,
			"codLibro":              codLibro,
		}).
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 9 Servicio Web Api importar nuevos comprobantes de pago
func (srv *Service) ImportarComprobantesPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpropuesta/web/propuesta/upload"

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetFormData(map[string]string{
			"filename":              filename,
			"filetype":              filetype,
			"numRuc":                numRuc,
			"perTributario":         perTributario,
			"codOrigenEnvio":        codOrigenEnvio,
			"codProceso":            codProceso,
			"codTipoCorrelativo":    codTipoCorrelativo,
			"nomArchivoImportacion": nomArchivoImportacion,
			"codLibro":              codLibro,
		}).
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 10 Servicio Web Api importar tipo de cambio masivo
func (srv *Service) ImportarTipoCambioMasivoService(perTributario, codLibro string, file multipart.File, header *multipart.FileHeader) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/%s/%s/resumenfechatipocambio", perTributario, codLibro)

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error al leer archivo: %v", err)
	}

	_, err = srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetFileReader("archivo", header.Filename, bytes.NewReader(fileContent)).
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 11 Servicio Web Api actualizar reintegro del crédito fiscal
func (srv *Service) ActualizarReintegroCreditoService(perTributario string, valorRCF, valorCFE, factProrrata float64) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/%s/grabacreditofiscal", perTributario)

	requestBody := map[string]interface{}{
		"datosFV621": map[string]float64{
			"valorRCF":     valorRCF,
			"valorCFE":     valorCFE,
			"factProrrata": factProrrata,
		},
	}

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(requestBody).
		Put(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 12 Servicio Web Api actualizar crédito fiscal especial
func (srv *Service) ActualizarCreditoFiscalEspecialService(perTributario string, valorCFE float64) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/%s/grabacreditofiscalespecial", perTributario)

	requestBody := map[string]interface{}{
		"datosFV621": map[string]float64{
			"valorCFE": valorCFE,
		},
	}

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(requestBody).
		Put(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 13 Servicio Web Api actualizar coeficiente de prorrata
func (srv *Service) ActualizarCoeficienteProrrataService(perTributario string, factProrrata, valorRCF, valorCFE float64) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/%s/grabacreditofiscal", perTributario)

	requestBody := map[string]interface{}{
		"datosFV621": map[string]float64{
			"factProrrata": factProrrata,
			"valorRCF":     valorRCF,
			"valorCFE":     valorCFE,
		},
	}

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(requestBody).
		Put(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 14 Servicio Web Api consultar FV0621
func (srv *Service) ConsultarFV0621Service(periodoSeleccionado, tipoInfo string) (interface{}, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web"

	params := map[string]string{
		"periodoSeleccionado": periodoSeleccionado,
	}

	if tipoInfo != "" {
		params["tipoInfo"] = tipoInfo
	}

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetQueryParams(params).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error HTTP %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	contentType := resp.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil, fmt.Errorf("respuesta no es JSON (Content-Type: %s): %s", contentType, string(resp.Body()))
	}

	var result interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error al procesar JSON: %v. Respuesta: %s", err, string(resp.Body()))
	}

	return result, nil
}

// 15 Servicio Web Api eliminar comprobante de la propuesta
func (srv *Service) EliminarComprobantePropuestaService(perTributario string, comprobantes interface{}) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/propuestarce/%s", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(comprobantes).
		Delete(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error HTTP %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 16 Servicio Web Api eliminar comprobante del preliminar
func (srv *Service) EliminarComprobantePreliminarService(perTributario string, comprobantes interface{}) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/preliminar/web/comprobanteslibroscompras/%s/eliminacomprobante", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(comprobantes).
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error HTTP %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 17 Servicio Web Api eliminar preliminar
func (srv *Service) EliminarPreliminarSUNAT(perTributario, indEliminar string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/preliminar/web/registroslibros/%s/%s/eliminapreliminar", perTributario, indEliminar)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Put(url)

	if err != nil {
		return fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 18 Servicio Web Api cargar ajustes posteriores
func (srv *Service) CargarAjustesPosterioresSUNAT(filename, filetype, numRuc, perTributario, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorajustesposteriores/web/ajustesposteriores/upload"

	// Codificar en base64
	filenameB64 := base64.StdEncoding.EncodeToString([]byte(filename))
	filetypeB64 := base64.StdEncoding.EncodeToString([]byte(filetype))
	numRucB64 := base64.StdEncoding.EncodeToString([]byte(numRuc))
	perTributarioB64 := base64.StdEncoding.EncodeToString([]byte(perTributario))
	codOrigenEnvioB64 := base64.StdEncoding.EncodeToString([]byte(codOrigenEnvio))
	codProcesoB64 := base64.StdEncoding.EncodeToString([]byte(codProceso))
	codTipoCorrelativoB64 := base64.StdEncoding.EncodeToString([]byte(codTipoCorrelativo))
	nomArchivoImportacionB64 := base64.StdEncoding.EncodeToString([]byte(nomArchivoImportacion))
	codLibroB64 := base64.StdEncoding.EncodeToString([]byte(codLibro))

	// Headers TUS específicos
	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Tus-Resumable", "1.0.0").
		SetHeader("Upload-Length", "0").
		SetHeader("Upload-Metadata", fmt.Sprintf("filename %s,filetype %s,numRuc %s,perTributario %s,codOrigenEnvio %s,codProceso %s,codTipoCorrelativo %s,nomArchivoImportacion %s,codLibro %s",
			filenameB64, filetypeB64, numRucB64, perTributarioB64, codOrigenEnvioB64, codProcesoB64, codTipoCorrelativoB64, nomArchivoImportacionB64, codLibroB64)).
		Post(url)

	if err != nil {
		return fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 201 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 19 Servicio Web Api enviar ajustes posteriores
func (srv *Service) EnviarAjustesPosterioresSUNAT(perTributario, codOrigenEnvio, numAjustePosterior, codLibro, numTicket string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%s/registrarajustesposterioresrc", perTributario, codOrigenEnvio)

	data := fmt.Sprintf("numAjustePosterior=%s&codLibro=%s&numTicket=%s", numAjustePosterior, codLibro, numTicket)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetBody(data).
		Post(url)

	if err != nil {
		return fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 20 Servicio Web Api eliminar comprobante de ajustes posteriores
func (srv *Service) EliminarComprobanteAjustesSUNAT(perTributario, indTipoAjustePosterior string, request interface{}) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%s/eliminarcomprobanteaprc", perTributario, indTipoAjustePosterior)

	requestBody, _ := json.Marshal(request)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(string(requestBody)).
		Delete(url)

	if err != nil {
		return fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 21 Servicio Web Api cargar ajustes posteriores no domiciliados
func (srv *Service) CargarAjustesNoDomiciliadosSUNAT(filename, filetype, numRuc, perTributario, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) (string, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorajustesposteriores/web/ajustesposteriores/upload"

	// Codificar en base64
	filenameB64 := base64.StdEncoding.EncodeToString([]byte(filename))
	filetypeB64 := base64.StdEncoding.EncodeToString([]byte(filetype))
	numRucB64 := base64.StdEncoding.EncodeToString([]byte(numRuc))
	perTributarioB64 := base64.StdEncoding.EncodeToString([]byte(perTributario))
	codOrigenEnvioB64 := base64.StdEncoding.EncodeToString([]byte(codOrigenEnvio))
	codProcesoB64 := base64.StdEncoding.EncodeToString([]byte(codProceso))
	codTipoCorrelativoB64 := base64.StdEncoding.EncodeToString([]byte(codTipoCorrelativo))
	nomArchivoImportacionB64 := base64.StdEncoding.EncodeToString([]byte(nomArchivoImportacion))
	codLibroB64 := base64.StdEncoding.EncodeToString([]byte(codLibro))

	// Metadata TUS
	metadata := fmt.Sprintf("filename %s,filetype %s,numRuc %s,perTributario %s,codOrigenEnvio %s,codProceso %s,codTipoCorrelativo %s,nomArchivoImportacion %s,codLibro %s",
		filenameB64, filetypeB64, numRucB64, perTributarioB64, codOrigenEnvioB64, codProcesoB64, codTipoCorrelativoB64, nomArchivoImportacionB64, codLibroB64)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Tus-Resumable", "1.0.0").
		SetHeader("Upload-Length", "0").
		SetHeader("Upload-Metadata", metadata).
		Post(url)

	if err != nil {
		return "", fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 201 {
		return "", fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	// Extraer numTicket de la respuesta si viene
	var response map[string]interface{}
	if json.Unmarshal(resp.Body(), &response) == nil {
		if numTicket, exists := response["numTicket"].(string); exists {
			return numTicket, nil
		}
	}

	return "", nil
}

// 22 Servicio Web Api enviar ajustes posteriores no domiciliados
func (srv *Service) EnviarAjustesNoDomiciliadosSUNAT(perTributario, numAjustePosterior, codLibro, numTicket string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%s/%s/%s/registrarajustesposterioresrcnd",
		perTributario, numAjustePosterior, codLibro, numTicket)

	// Enviar body vacío pero válido
	emptyBody := "{}"

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(emptyBody).
		Post(url)

	if err != nil {
		return fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 23 Servicio Web Api eliminar comprobante de ajustes posteriores no domiciliados
func (srv *Service) EliminarComprobanteAjustesPosteriorNoDOM(perTributario string, indTipoAjustePosterior int, codAjustePosterior string, detalleAjustes []struct {
	CodTipoCDP  string `json:"codTipoCDP"`
	NumSerieCDP string `json:"numSerieCDP"`
	NumCDP      string `json:"numCDP"`
	CodCar      string `json:"codCar"`
	ID          string `json:"id"`
}) error {


	if len(codAjustePosterior) != 24 {
		return fmt.Errorf("codAjustePosterior debe ser un ObjectId válido de 24 caracteres hexadecimales")
	}

	requestBody := map[string]interface{}{
		"codAjustePosterior": codAjustePosterior,
		"detalleAjustes":     detalleAjustes,
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%d/eliminarcomprobanteaprcnd", perTributario, indTipoAjustePosterior)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(requestBody).
		Delete(url)

	if err != nil {
		return fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 24 Servicio Web Api cargar ajustes posteriores de periodos anteriores
func (srv *Service) CargarAjustesPosterioresPeriodosAnteriores(filename, filetype, numRuc, perTributario, codOrigenEnvio,
	codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) (map[string]interface{}, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorajustesposteriores/web/ajustesposteriores/upload"

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Tus-Resumable", "1.0.0").
		Options(url)

	if err != nil {
		return nil, fmt.Errorf("error en OPTIONS: %v", err)
	}

	filenameEncoded := base64.StdEncoding.EncodeToString([]byte(filename))
	filetypeEncoded := base64.StdEncoding.EncodeToString([]byte(filetype))
	numRucEncoded := base64.StdEncoding.EncodeToString([]byte(numRuc))
	perTributarioEncoded := base64.StdEncoding.EncodeToString([]byte(perTributario))
	codOrigenEnvioEncoded := base64.StdEncoding.EncodeToString([]byte(codOrigenEnvio))
	codProcesoEncoded := base64.StdEncoding.EncodeToString([]byte(codProceso))
	codTipoCorrelativoEncoded := base64.StdEncoding.EncodeToString([]byte(codTipoCorrelativo))
	nomArchivoImportacionEncoded := base64.StdEncoding.EncodeToString([]byte(nomArchivoImportacion))
	codLibroEncoded := base64.StdEncoding.EncodeToString([]byte(codLibro))

	metadata := fmt.Sprintf("filename %s,filetype %s,numRuc %s,perTributario %s,codOrigenEnvio %s,codProceso %s,codTipoCorrelativo %s,nomArchivoImportacion %s,codLibro %s",
		filenameEncoded, filetypeEncoded, numRucEncoded, perTributarioEncoded, codOrigenEnvioEncoded,
		codProcesoEncoded, codTipoCorrelativoEncoded, nomArchivoImportacionEncoded, codLibroEncoded)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Tus-Resumable", "1.0.0").
		SetHeader("Upload-Length", "1024").
		SetHeader("Upload-Metadata", metadata).
		Post(url)

	if err != nil {
		return nil, fmt.Errorf("error en POST: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 201 {
		return nil, fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	return map[string]interface{}{
		"status":    resp.StatusCode(),
		"response":  string(resp.Body()),
		"location":  resp.Header().Get("Location"),
		"uploadUrl": "https://api-sire.sunat.gob.pe" + resp.Header().Get("Location"),
	}, nil
}

// 25 Servicio Web Api enviar ajustes posteriores de periodos anteriores
func (srv *Service) EnviarAjustesPosterioresPeriodosAnteriores(perTributario, numAjustePosterior, codLibro, numTicket,
	codAjustePosterior string, detalleAjustes []struct {
	CodTipoCDP  string `json:"codTipoCDP"`
	NumSerieCDP string `json:"numSerieCDP"`
	NumCDP      string `json:"numCDP"`
	NumTicket   string `json:"numTicket"`
}) error {


	requestBody := map[string]interface{}{
		"codAjustePosterior": codAjustePosterior,
		"detalleAjustes":     detalleAjustes,
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%s/%s/%s/registrarajustesposterioresparc",
		perTributario, numAjustePosterior, codLibro, numTicket)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(requestBody).
		Post(url)

	if err != nil {
		return fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 26 Servicio Web Api eliminar comprobante de ajustes posteriores de periodos anteriores
func (srv *Service) EliminarComprobanteAjustesPosterioresPeriodosAnteriores(indTipoAjustePosterior int, perTributario,
	codAjustePosterior string, detalleAjustes []struct {
	CodTipoCDP  string `json:"codTipoCDP"`
	NumSerieCDP string `json:"numSerieCDP"`
	NumCDP      string `json:"numCDP"`
}) error {


	requestBody := map[string]interface{}{
		"codAjustePosterior": codAjustePosterior,
		"detalleAjustes":     detalleAjustes,
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/ajustesposteriores/%d/%s/eliminarcomprobanteapparc",
		indTipoAjustePosterior, perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(requestBody).
		Delete(url)

	if err != nil {
		return fmt.Errorf("error en petición HTTP: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 27 Servicio Web Api cargar ajustes posteriores de periodos anteriores no domiciliados
func (srv *Service) CargarAjustesPosterioresPeriodosAnterioresNoDOM(filename, filetype, numRuc, perTributario, codOrigenEnvio,
	codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) (map[string]interface{}, error) {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorajustesposteriores/web/ajustesposteriores/upload"

	_, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Tus-Resumable", "1.0.0").
		Options(url)

	if err != nil {
		return nil, fmt.Errorf("error en OPTIONS: %v", err)
	}

	filenameEncoded := base64.StdEncoding.EncodeToString([]byte(filename))
	filetypeEncoded := base64.StdEncoding.EncodeToString([]byte(filetype))
	numRucEncoded := base64.StdEncoding.EncodeToString([]byte(numRuc))
	perTributarioEncoded := base64.StdEncoding.EncodeToString([]byte(perTributario))
	codOrigenEnvioEncoded := base64.StdEncoding.EncodeToString([]byte(codOrigenEnvio))
	codProcesoEncoded := base64.StdEncoding.EncodeToString([]byte(codProceso))
	codTipoCorrelativoEncoded := base64.StdEncoding.EncodeToString([]byte(codTipoCorrelativo))
	nomArchivoImportacionEncoded := base64.StdEncoding.EncodeToString([]byte(nomArchivoImportacion))
	codLibroEncoded := base64.StdEncoding.EncodeToString([]byte(codLibro))

	metadata := fmt.Sprintf("filename %s,filetype %s,numRuc %s,perTributario %s,codOrigenEnvio %s,codProceso %s,codTipoCorrelativo %s,nomArchivoImportacion %s,codLibro %s",
		filenameEncoded, filetypeEncoded, numRucEncoded, perTributarioEncoded, codOrigenEnvioEncoded,
		codProcesoEncoded, codTipoCorrelativoEncoded, nomArchivoImportacionEncoded, codLibroEncoded)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Tus-Resumable", "1.0.0").
		SetHeader("Upload-Length", "1024").
		SetHeader("Upload-Metadata", metadata).
		Post(url)

	if err != nil {
		return nil, fmt.Errorf("error en POST: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 201 {
		return nil, fmt.Errorf("error SUNAT %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	return map[string]interface{}{
		"status":    resp.StatusCode(),
		"response":  string(resp.Body()),
		"location":  resp.Header().Get("Location"),
		"uploadUrl": "https://api-sire.sunat.gob.pe" + resp.Header().Get("Location"),
	}, nil
}

// 28 Servicio Web Api enviar ajustes posteriores de periodos anteriores no domiciliados
func (srv *Service) EnviarAjustesPosterioresService(perTributario, numAjustePosterior, codLibro, numTicket string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%s/%s/%s/registrarajustesposterioresparend", perTributario, numAjustePosterior, codLibro, numTicket)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		Post(url)

	if err != nil {
		return fmt.Errorf("error al realizar petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), resp.String())
	}

	return nil
}

// 29 Servicio Web Api eliminar comprobante de ajustes posteriores de periodos anteriores no domiciliados
func (srv *Service) EliminarComprobanteAjustesRCESUNAT(numAjustePosterior, perTributario string, body map[string]interface{}) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/%s/%s/eliminarcomprobanteapparcnd", numAjustePosterior, perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(body).
		Delete(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 31 Servicio Web Api consultar estado ticket
func (srv *Service) ConsultarEstadoTicketsSUNAT(perIni, perFin, page, perPage, numTicket string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionprocesosmasivos/web/masivo/consultaestadotickets?perIni=%s&perFin=%s&page=%s&perPage=%s&numTicket=%s", perIni, perFin, page, perPage, numTicket)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 32 Servicio Web Api descargar archivo
func (srv *Service) DescargarArchivoReporteSUNAT(nomArchivoReporte, codTipoArchivoReporte string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionprocesosmasivos/web/masivo/archivoreporte?nomArchivoReporte=%s&codTipoArchivoReporte=%s", nomArchivoReporte, codTipoArchivoReporte)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 33 Servicio Web Api consultar año y mes del RCE
func (srv *Service) ConsultarPeriodosRCESUNAT(codLibro string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/padron/web/omisos/%s/periodos", codLibro)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 34 Servicio Web Api descargar propuesta
func (srv *Service) DescargarPropuestaRCESUNAT(perTributario string, params map[string]string) (interface{}, error) {


	baseURL := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/propuesta/%s/exportacioncomprobantepropuesta", perTributario)

	queryParams := ""
	for key, value := range params {
		if value != "" {
			if queryParams != "" {
				queryParams += "&"
			}
			queryParams += fmt.Sprintf("%s=%s", key, value)
		}
	}

	url := baseURL + "?" + queryParams

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 35 Servicio Web Api descargar resumen
func (srv *Service) DescargarResumenRCESUNAT(perTributario, codTipoResumen, codTipoArchivo, codLibro string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/resumen/web/resumencomprobantes/%s/%s/%s/exporta?codLibro=%s", perTributario, codTipoResumen, codTipoArchivo, codLibro)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 36 Servicio Web Api descargar resumen inconsistencias RCE

// 37 Servicio Web Api descargar excluidos
func (srv *Service) DescargarExcluidosRCESUNAT(perTributario string, params map[string]string) (interface{}, error) {


	baseURL := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/excluidos/%s/exportaexcluidos", perTributario)

	queryParams := ""
	for key, value := range params {
		if value != "" {
			if queryParams != "" {
				queryParams += "&"
			}
			queryParams += fmt.Sprintf("%s=%s", key, value)
		}
	}

	url := baseURL + "?" + queryParams

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var result interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %v", err)
	}

	return result, nil
}

// 38 Servicio Web Api eliminar comprobante no domiciliado
func (srv *Service) EliminarComprobanteNoDomiciliadoSUNAT(perTributario string, requestBody interface{}) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/libronodomiciliado/web/nodomiciliados/%s/eliminarcomprobantepreliminarnd", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(requestBody).
		Put(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() == 401 {
		return fmt.Errorf("token expirado o inválido")
	}

	if resp.StatusCode() == 422 {
		return fmt.Errorf("error de validación SUNAT: %s", string(resp.Body()))
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 39 Servicio Web Api exportar preliminar de registro de compras no domiciliados
func (srv *Service) ExportarPreliminarNoDomiciliadosSUNAT(perTributario string, queryParams map[string]string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/preliminar/web/nodomiciliados/%s/exportapreliminarnd", perTributario)

	request := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json")

	for key, value := range queryParams {
		if value != "" {
			request.SetQueryParam(key, value)
		}
	}

	resp, err := request.Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() == 401 {
		return nil, fmt.Errorf("token expirado o inválido")
	}

	if resp.StatusCode() == 422 {
		return nil, fmt.Errorf("error de validación SUNAT: %s", string(resp.Body()))
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta: %v", err)
	}

	return result, nil
}

// 40 Servicio Web Api exportar preliminar de registro de compras
func (srv *Service) ExportarPreliminarRegistroComprasSUNAT(perTributario string, queryParams map[string]string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/preliminar/web/registroslibros/%s/exportareportepreliminar", perTributario)

	request := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json")

	for key, value := range queryParams {
		if value != "" {
			request.SetQueryParam(key, value)
		}
	}

	resp, err := request.Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() == 401 {
		return nil, fmt.Errorf("token expirado o inválido")
	}

	if resp.StatusCode() == 422 {
		return nil, fmt.Errorf("error de validación SUNAT: %s", string(resp.Body()))
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta: %v", err)
	}

	return result, nil
}

// 41 Servicio Web Api descargar reporte de casillas
func (srv *Service) DescargarReporteCasillasSUNAT(perTributario, tipoReporte, tipoDescarga string) ([]byte, string, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/casillas/e/casillaspropuestas/%s/reporte/%s/%s", perTributario, tipoReporte, tipoDescarga)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return nil, "", fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() == 401 {
		return nil, "", fmt.Errorf("token expirado o inválido")
	}

	if resp.StatusCode() == 422 {
		return nil, "", fmt.Errorf("error de validación SUNAT: %s", string(resp.Body()))
	}

	if resp.StatusCode() != 200 {
		return nil, "", fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	contentType := "application/octet-stream"
	if tipoDescarga == "txt" {
		contentType = "text/plain"
	} else if tipoDescarga == "xlsx" {
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	} else if tipoDescarga == "pdf" {
		contentType = "application/pdf"
	}

	return resp.Body(), contentType, nil
}

// 42 Servicio Web Api descargar inconsistencias en registros preliminar registrado
func (srv *Service) DescargarInconsistenciasPreliminarSUNAT(perTributario, codTipoArchivo, cntlimite string) ([]byte, string, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/casillas/inconsistenciaslibros/%s/reporteinconsistencia/%s", perTributario, codTipoArchivo)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetQueryParam("cntlimite", cntlimite).
		Get(url)

	if err != nil {
		return nil, "", fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() == 401 {
		return nil, "", fmt.Errorf("token expirado o inválido")
	}

	if resp.StatusCode() == 422 {
		return nil, "", fmt.Errorf("error de validación SUNAT: %s", string(resp.Body()))
	}

	if resp.StatusCode() != 200 {
		return nil, "", fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	contentType := "application/octet-stream"
	if codTipoArchivo == "txt" {
		contentType = "text/plain"
	} else if codTipoArchivo == "xls" || codTipoArchivo == "xlsx" {
		contentType = "application/vnd.ms-excel"
	} else if codTipoArchivo == "pdf" {
		contentType = "application/pdf"
	}

	return resp.Body(), contentType, nil
}

// 43 Servicio Web Api descargar inconsistencias por montos totales
func (srv *Service) DescargarInconsistenciasMontosTotalesSUNAT(perTributario string, queryParams map[string]string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/inconsistencias/web/periodoinconsistencias/%s/exportarinconsistenciasportotales", perTributario)

	request := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json")

	for key, value := range queryParams {
		if value != "" {
			request.SetQueryParam(key, value)
		}
	}

	resp, err := request.Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() == 401 {
		return nil, fmt.Errorf("token expirado o inválido")
	}

	if resp.StatusCode() == 422 {
		return nil, fmt.Errorf("error de validación SUNAT: %s", string(resp.Body()))
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta: %v", err)
	}

	return result, nil
}

// 44 Servicio Web Api descargar inconsistencias por comprobante pago
func (srv *Service) DescargarInconsistenciasComprobantesSUNAT(perTributario, codLibro string, queryParams map[string]string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/inconsistencias/web/periodoinconsistencias/%s/%s/exportarinconsistenciasporcomprobantes", perTributario, codLibro)

	request := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json")

	for key, value := range queryParams {
		if value != "" {
			request.SetQueryParam(key, value)
		}
	}

	resp, err := request.Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() == 401 {
		return nil, fmt.Errorf("token expirado o inválido")
	}

	if resp.StatusCode() == 422 {
		return nil, fmt.Errorf("error de validación SUNAT: %s", string(resp.Body()))
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta: %v", err)
	}

	return result, nil
}

// 45 Servicio Web Api descargar ajustes posteriores
func (srv *Service) DescargarAjustesPosterioresSUNAT(perTributario string, queryParams map[string]string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/exportaajustesposterioresrc", perTributario)

	request := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json")

	for key, value := range queryParams {
		if value != "" {
			request.SetQueryParam(key, value)
		}
	}

	resp, err := request.Get(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() == 401 {
		return nil, fmt.Errorf("token expirado o inválido")
	}

	if resp.StatusCode() == 422 {
		return nil, fmt.Errorf("error de validación SUNAT: %s", string(resp.Body()))
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta: %v", err)
	}

	return result, nil
}

// 46 Servicio Web Api descargar ajustes posteriores no domiciliados
func (srv *Service) DescargarAjustesPosterioresNoDomiciliadosSUNAT(perTributario, codTipoArchivo string) (interface{}, error) {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/exportarajustesposterioresrcnd", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetQueryParam("codTipoArchivo", codTipoArchivo).
		Post(url)

	if err != nil {
		return nil, fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() == 401 {
		return nil, fmt.Errorf("token expirado o inválido")
	}

	if resp.StatusCode() == 422 {
		return nil, fmt.Errorf("error de validación SUNAT: %s", string(resp.Body()))
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta: %v", err)
	}

	return result, nil
}

// 47 Servicio Web Api descargar ajustes posteriores de periodos anteriores
func (srv *Service) ExportarAjustesPosterioresSUNAT(perTributario, codTipoArchivo, indAjustePosteriorPle, codOrigenEnvio string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/exportaajustesposterioresparc", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"codTipoArchivo":        codTipoArchivo,
			"indAjustePosteriorPle": indAjustePosteriorPle,
			"codOrigenEnvio":        codOrigenEnvio,
		}).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 48 Servicio Web Api descargar ajustes posteriores de periodos anteriores no domiciliados
func (srv *Service) ExportarAjustesPosterioresNDSUNAT(perTributario, codTipoArchivo, codOrigenEnvio string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/exportaajustesposterioresparcnd", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"codTipoArchivo": codTipoArchivo,
			"codOrigenEnvio": codOrigenEnvio,
		}).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 49 Servicio Web Api descargar constancia de recepción
func (srv *Service) DescargarConstanciaRecepcionSUNAT(nomConstanciaRecepcion string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/registroslibros/constancia/constanciarecepcion"

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"nomConstanciaRecepcion": nomConstanciaRecepcion,
		}).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 50 Servicio Web Api descargar reporte consolidado registro por periodo
func (srv *Service) DescargarReporteConsolidadoSUNAT(perTributario, codTipoArchivo, codMoneda, codProceso, codOrigenEnvio, lisPeriodos string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/ajustesposteriores/%s/solicitardescarga", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"codTipoArchivo": codTipoArchivo,
			"codMoneda":      codMoneda,
			"codProceso":     codProceso,
			"codOrigen":      codOrigenEnvio,
			"lisPeriodos":    lisPeriodos,
		}).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 51 Servicio Web Api descargar RCE por periodo
func (srv *Service) DescargarRCEPorPeriodoSUNAT(perTributario, codTipoArchivo, codMoneda, codProceso, codOrigen, lisPeriodos string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/ajustesposteriores/%s/solicitardescarga", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"codTipoArchivo": codTipoArchivo,
			"codMoneda":      codMoneda,
			"codProceso":     codProceso,
			"codOrigen":      codOrigen,
			"lisPeriodos":    lisPeriodos,
		}).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 52 Servicio Web Api descargar reporte inconsistencias por periodo
func (srv *Service) DescargarInconsistenciasPeriodoSUNAT(nomArchivo, codTipoArchivoReporte, codOrigen string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionconsultas/web/registrolibro/archivoreporte"

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"nomArchivo":            nomArchivo,
			"codTipoArchivoReporte": codTipoArchivoReporte,
			"codOrigen":             codOrigen,
		}).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 53 Servicio Web Api descargar reporte CAR por periodo y fase
func (srv *Service) DescargarReporteCARPorPeriodoSUNAT(perTributario, codOrigenEnvio, codLibro, codFase string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/comprobanteslibros/%s/reportecar", perTributario)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"codOrigenEnvio": codOrigenEnvio,
			"codLibro":       codLibro,
			"codFase":        codFase,
		}).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 54 Servicio Web Api descargar reporte estadístico compras por proveedor por periodo
func (srv *Service) DescargarReporteEstadisticoComprasSUNAT(numRuc, perTributario, fechaini, fechafin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/estadistica/web/resumenestadistico/exporta"

	queryParams := map[string]string{
		"perTributario":  perTributario,
		"codTipoArchivo": codTipoArchivo,
		"codTipoReporte": codTipoReporte,
		"codLibro":       codLibro,
	}

	if numRuc != "" {
		queryParams["numRuc"] = numRuc
	}
	if fechaini != "" {
		queryParams["fechaini"] = fechaini
	}
	if fechafin != "" {
		queryParams["fechafin"] = fechafin
	}
	if numRucproveedor != "" {
		queryParams["numRucproveedor"] = numRucproveedor
	}
	if codTipoCDP != "" {
		queryParams["codTipoCDP"] = codTipoCDP
	}

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(queryParams).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 204 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 55 Servicio Web Api descargar reporte estadístico NC-ND por proveedor y periodo
func (srv *Service) DescargarReporteEstadisticoNCNDSUNAT(numRuc, perTributario, fechaini, fechafin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/estadistica/web/resumenestadistico/exporta"

	queryParams := map[string]string{
		"numRuc":         numRuc,
		"perTributario":  perTributario,
		"codTipoArchivo": codTipoArchivo,
		"codTipoReporte": codTipoReporte,
		"codLibro":       codLibro,
	}

	if fechaini != "" {
		queryParams["fechaini"] = fechaini
	}
	if fechafin != "" {
		queryParams["fechafin"] = fechafin
	}
	if numRucproveedor != "" {
		queryParams["numRucproveedor"] = numRucproveedor
	}
	if codTipoCDP != "" {
		queryParams["codTipoCDP"] = codTipoCDP
	}

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(queryParams).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 204 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 56 Servicio Web Api descargar reporte estadístico Compras por día y periodo
func (srv *Service) DescargarReporteEstadisticoComprasDiaSUNAT(numRuc, perTributario, fechaIni, fechaFin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/estadistica/web/resumenestadistico/exporta"

	queryParams := map[string]string{
		"numRuc":         numRuc,
		"perTributario":  perTributario,
		"codTipoArchivo": codTipoArchivo,
		"codTipoReporte": codTipoReporte,
		"codLibro":       codLibro,
	}

	if fechaIni != "" {
		queryParams["fechaIni"] = fechaIni
	}
	if fechaFin != "" {
		queryParams["fechaFin"] = fechaFin
	}
	if numRucproveedor != "" {
		queryParams["numRucproveedor"] = numRucproveedor
	}
	if codTipoCDP != "" {
		queryParams["codTipoCDP"] = codTipoCDP
	}

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(queryParams).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 204 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 57 Servicio Web Api descargar reporte estadístico Compras por CIIU
func (srv *Service) DescargarReporteEstadisticoComprasCIIUSUNAT(numRuc, perTributario, fechaIni, fechaFin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro string) error {


	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/estadistica/web/resumenestadistico/exporta"

	queryParams := map[string]string{
		"numRuc":         numRuc,
		"perTributario":  perTributario,
		"codTipoArchivo": codTipoArchivo,
		"codTipoReporte": codTipoReporte,
		"codLibro":       codLibro,
	}

	if fechaIni != "" {
		queryParams["fechaIni"] = fechaIni
	}
	if fechaFin != "" {
		queryParams["fechaFin"] = fechaFin
	}
	if numRucproveedor != "" {
		queryParams["numRucproveedor"] = numRucproveedor
	}
	if codTipoCDP != "" {
		queryParams["codTipoCDP"] = codTipoCDP
	}

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Accept", "application/json").
		SetQueryParams(queryParams).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición a SUNAT: %v", err)
	}

	if resp.StatusCode() != 200 && resp.StatusCode() != 204 {
		return fmt.Errorf("error SUNAT [%d]: %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 58 Servicio Web Api descargar reporte de cumplimiento
func (srv *Service) DescargarReporteCumplimientoService(perTributario, codLibro string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/cumplimiento/web/omisos/%s/%s/consultaReporteCumplimiento/exportardocumento", perTributario, codLibro)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// 59 Servicio Web Api consultar ajustes posteriores RCE
func (srv *Service) ConsultarAjustesPosterioresRCEService(periodoSeleccionado, tipoInfo string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web?periodoSeleccionado=%s&tipoInfo=%s", periodoSeleccionado, tipoInfo)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Put(url)

	if err != nil {
		return fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

func (srv *Service) ListarComprobantesAjustesPostService(periodoSeleccionado, page, perPage string) error {


	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/listarcap?page=%s&perPage=%s", periodoSeleccionado, page, perPage)

	resp, err := srv.client.NewRequest(srv.ruc).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}