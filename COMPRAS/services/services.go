package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strings"

	"github.com/go-resty/resty/v2"
)

// 2 Servicio Web Api aceptar propuesta
func AceptarPropuestaService(perTributario string) (map[string]interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/registroslibros/%s/aceptarpropuesta", perTributario)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ReemplazoPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpropuesta/web/propuesta/upload"

	_, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func RegistrarPreliminarService(perTributario string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/preliminar/web/registroslibros/%s/registrapreliminares", perTributario)

	_, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 5 Servicio Web Api cargar registro de compra no domiciliados
func CargarNoDomiciliadosService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpreliminar/web/preliminar/upload"

	_, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ImportarComplementoPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpropuesta/web/propuesta/upload"

	_, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ImportarComprobantesPreliminarService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpreliminar/web/preliminar/upload"

	_, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func IncluirExcluirPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpropuesta/web/propuesta/upload"

	_, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ImportarComprobantesPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorpropuesta/web/propuesta/upload"

	_, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ImportarTipoCambioMasivoService(perTributario, codLibro string, file multipart.File, header *multipart.FileHeader) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/%s/%s/resumenfechatipocambio", perTributario, codLibro)

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error al leer archivo: %v", err)
	}

	_, err = client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
		SetFileReader("archivo", header.Filename, bytes.NewReader(fileContent)).
		Post(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 11 Servicio Web Api actualizar reintegro del crédito fiscal
func ActualizarReintegroCreditoService(perTributario string, valorRCF, valorCFE, factProrrata float64) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/%s/grabacreditofiscal", perTributario)

	requestBody := map[string]interface{}{
		"datosFV621": map[string]float64{
			"valorRCF":     valorRCF,
			"valorCFE":     valorCFE,
			"factProrrata": factProrrata,
		},
	}

	_, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
		SetBody(requestBody).
		Put(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 12 Servicio Web Api actualizar crédito fiscal especial
func ActualizarCreditoFiscalEspecialService(perTributario string, valorCFE float64) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/%s/grabacreditofiscalespecial", perTributario)

	requestBody := map[string]interface{}{
		"datosFV621": map[string]float64{
			"valorCFE": valorCFE,
		},
	}

	_, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
		SetBody(requestBody).
		Put(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 13 Servicio Web Api actualizar coeficiente de prorrata
func ActualizarCoeficienteProrrataService(perTributario string, factProrrata, valorRCF, valorCFE float64) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/%s/grabacreditofiscal", perTributario)

	requestBody := map[string]interface{}{
		"datosFV621": map[string]float64{
			"factProrrata": factProrrata,
			"valorRCF":     valorRCF,
			"valorCFE":     valorCFE,
		},
	}

	_, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
		SetBody(requestBody).
		Put(url)

	if err != nil {
		return fmt.Errorf("error al conectar con SUNAT: %v", err)
	}

	return nil
}

// 14 Servicio Web Api consultar FV0621
func ConsultarFV0621Service(periodoSeleccionado, tipoInfo string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no encontrado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web"

	params := map[string]string{
		"periodoSeleccionado": periodoSeleccionado,
	}

	if tipoInfo != "" {
		params["tipoInfo"] = tipoInfo
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func EliminarComprobantePropuestaService(perTributario string, comprobantes interface{}) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web/propuestarce/%s", perTributario)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func EliminarComprobantePreliminarService(perTributario string, comprobantes interface{}) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no encontrado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/preliminar/web/comprobanteslibroscompras/%s/eliminacomprobante", perTributario)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func EliminarPreliminarSUNAT(perTributario, indEliminar string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/preliminar/web/registroslibros/%s/%s/eliminapreliminar", perTributario, indEliminar)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func CargarAjustesPosterioresSUNAT(filename, filetype, numRuc, perTributario, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no configurado")
	}

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
	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+token).
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
func EnviarAjustesPosterioresSUNAT(perTributario, codOrigenEnvio, numAjustePosterior, codLibro, numTicket string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%s/registrarajustesposterioresrc", perTributario, codOrigenEnvio)

	data := fmt.Sprintf("numAjustePosterior=%s&codLibro=%s&numTicket=%s", numAjustePosterior, codLibro, numTicket)

	resp, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func EliminarComprobanteAjustesSUNAT(perTributario, indTipoAjustePosterior string, request interface{}) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%s/eliminarcomprobanteaprc", perTributario, indTipoAjustePosterior)

	requestBody, _ := json.Marshal(request)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func CargarAjustesNoDomiciliadosSUNAT(filename, filetype, numRuc, perTributario, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) (string, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return "", fmt.Errorf("token no configurado")
	}

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

	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+token).
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
func EnviarAjustesNoDomiciliadosSUNAT(perTributario, numAjustePosterior, codLibro, numTicket string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%s/%s/%s/registrarajustesposterioresrcnd",
		perTributario, numAjustePosterior, codLibro, numTicket)

	// Enviar body vacío pero válido
	emptyBody := "{}"

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func EliminarComprobanteAjustesPosteriorNoDOM(perTributario string, indTipoAjustePosterior int, codAjustePosterior string, detalleAjustes []struct {
	CodTipoCDP  string `json:"codTipoCDP"`
	NumSerieCDP string `json:"numSerieCDP"`
	NumCDP      string `json:"numCDP"`
	CodCar      string `json:"codCar"`
	ID          string `json:"id"`
}) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

	if len(codAjustePosterior) != 24 {
		return fmt.Errorf("codAjustePosterior debe ser un ObjectId válido de 24 caracteres hexadecimales")
	}

	requestBody := map[string]interface{}{
		"codAjustePosterior": codAjustePosterior,
		"detalleAjustes":     detalleAjustes,
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%d/eliminarcomprobanteaprcnd", perTributario, indTipoAjustePosterior)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func CargarAjustesPosterioresPeriodosAnteriores(filename, filetype, numRuc, perTributario, codOrigenEnvio,
	codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) (map[string]interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("TOKEN no configurado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorajustesposteriores/web/ajustesposteriores/upload"

	_, err := client.R().
		SetHeader("Authorization", "Bearer "+token).
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

	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+token).
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
func EnviarAjustesPosterioresPeriodosAnteriores(perTributario, numAjustePosterior, codLibro, numTicket,
	codAjustePosterior string, detalleAjustes []struct {
	CodTipoCDP  string `json:"codTipoCDP"`
	NumSerieCDP string `json:"numSerieCDP"`
	NumCDP      string `json:"numCDP"`
	NumTicket   string `json:"numTicket"`
}) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

	requestBody := map[string]interface{}{
		"codAjustePosterior": codAjustePosterior,
		"detalleAjustes":     detalleAjustes,
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%s/%s/%s/registrarajustesposterioresparc",
		perTributario, numAjustePosterior, codLibro, numTicket)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func EliminarComprobanteAjustesPosterioresPeriodosAnteriores(indTipoAjustePosterior int, perTributario,
	codAjustePosterior string, detalleAjustes []struct {
	CodTipoCDP  string `json:"codTipoCDP"`
	NumSerieCDP string `json:"numSerieCDP"`
	NumCDP      string `json:"numCDP"`
}) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

	requestBody := map[string]interface{}{
		"codAjustePosterior": codAjustePosterior,
		"detalleAjustes":     detalleAjustes,
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/ajustesposteriores/%d/%s/eliminarcomprobanteapparc",
		indTipoAjustePosterior, perTributario)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func CargarAjustesPosterioresPeriodosAnterioresNoDOM(filename, filetype, numRuc, perTributario, codOrigenEnvio,
	codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro string) (map[string]interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("TOKEN no configurado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/receptorajustesposteriores/web/ajustesposteriores/upload"

	_, err := client.R().
		SetHeader("Authorization", "Bearer "+token).
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

	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+token).
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
func EnviarAjustesPosterioresService(perTributario, numAjustePosterior, codLibro, numTicket string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/%s/%s/%s/registrarajustesposterioresparend", perTributario, numAjustePosterior, codLibro, numTicket)

	resp, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func EliminarComprobanteAjustesRCESUNAT(numAjustePosterior, perTributario string, body map[string]interface{}) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/%s/%s/eliminarcomprobanteapparcnd", numAjustePosterior, perTributario)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ConsultarEstadoTicketsSUNAT(perIni, perFin, page, perPage, numTicket string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionprocesosmasivos/web/masivo/consultaestadotickets?perIni=%s&perFin=%s&page=%s&perPage=%s&numTicket=%s", perIni, perFin, page, perPage, numTicket)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarArchivoReporteSUNAT(nomArchivoReporte, codTipoArchivoReporte string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionprocesosmasivos/web/masivo/archivoreporte?nomArchivoReporte=%s&codTipoArchivoReporte=%s", nomArchivoReporte, codTipoArchivoReporte)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ConsultarPeriodosRCESUNAT(codLibro string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/padron/web/omisos/%s/periodos", codLibro)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarPropuestaRCESUNAT(perTributario string, params map[string]string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

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

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarResumenRCESUNAT(perTributario, codTipoResumen, codTipoArchivo, codLibro string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/resumen/web/resumencomprobantes/%s/%s/%s/exporta?codLibro=%s", perTributario, codTipoResumen, codTipoArchivo, codLibro)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarExcluidosRCESUNAT(perTributario string, params map[string]string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

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

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func EliminarComprobanteNoDomiciliadoSUNAT(perTributario string, requestBody interface{}) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/libronodomiciliado/web/nodomiciliados/%s/eliminarcomprobantepreliminarnd", perTributario)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ExportarPreliminarNoDomiciliadosSUNAT(perTributario string, queryParams map[string]string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/preliminar/web/nodomiciliados/%s/exportapreliminarnd", perTributario)

	request := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token)

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
func ExportarPreliminarRegistroComprasSUNAT(perTributario string, queryParams map[string]string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/preliminar/web/registroslibros/%s/exportareportepreliminar", perTributario)

	request := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token)

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
func DescargarReporteCasillasSUNAT(perTributario, tipoReporte, tipoDescarga string) ([]byte, string, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, "", fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/casillas/e/casillaspropuestas/%s/reporte/%s/%s", perTributario, tipoReporte, tipoDescarga)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarInconsistenciasPreliminarSUNAT(perTributario, codTipoArchivo, cntlimite string) ([]byte, string, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, "", fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/casillas/inconsistenciaslibros/%s/reporteinconsistencia/%s", perTributario, codTipoArchivo)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarInconsistenciasMontosTotalesSUNAT(perTributario string, queryParams map[string]string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/inconsistencias/web/periodoinconsistencias/%s/exportarinconsistenciasportotales", perTributario)

	request := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token)

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
func DescargarInconsistenciasComprobantesSUNAT(perTributario, codLibro string, queryParams map[string]string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/inconsistencias/web/periodoinconsistencias/%s/%s/exportarinconsistenciasporcomprobantes", perTributario, codLibro)

	request := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token)

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
func DescargarAjustesPosterioresSUNAT(perTributario string, queryParams map[string]string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/exportaajustesposterioresrc", perTributario)

	request := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token)

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
func DescargarAjustesPosterioresNoDomiciliadosSUNAT(perTributario, codTipoArchivo string) (interface{}, error) {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return nil, fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/exportarajustesposterioresrcnd", perTributario)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ExportarAjustesPosterioresSUNAT(perTributario, codTipoArchivo, indAjustePosteriorPle, codOrigenEnvio string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/exportaajustesposterioresparc", perTributario)

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ExportarAjustesPosterioresNDSUNAT(perTributario, codTipoArchivo, codOrigenEnvio string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/exportaajustesposterioresparcnd", perTributario)

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarConstanciaRecepcionSUNAT(nomConstanciaRecepcion string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/registroslibros/constancia/constanciarecepcion"

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarReporteConsolidadoSUNAT(perTributario, codTipoArchivo, codMoneda, codProceso, codOrigenEnvio, lisPeriodos string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/ajustesposteriores/%s/solicitardescarga", perTributario)

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarRCEPorPeriodoSUNAT(perTributario, codTipoArchivo, codMoneda, codProceso, codOrigen, lisPeriodos string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/ajustesposteriores/%s/solicitardescarga", perTributario)

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarInconsistenciasPeriodoSUNAT(nomArchivo, codTipoArchivoReporte, codOrigen string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionconsultas/web/registrolibro/archivoreporte"

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarReporteCARPorPeriodoSUNAT(perTributario, codOrigenEnvio, codLibro, codFase string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/gestionlibro/web/comprobanteslibros/%s/reportecar", perTributario)

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarReporteEstadisticoComprasSUNAT(numRuc, perTributario, fechaini, fechafin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

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

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarReporteEstadisticoNCNDSUNAT(numRuc, perTributario, fechaini, fechafin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

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

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarReporteEstadisticoComprasDiaSUNAT(numRuc, perTributario, fechaIni, fechaFin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

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

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarReporteEstadisticoComprasCIIUSUNAT(numRuc, perTributario, fechaIni, fechaFin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("TOKEN no configurado")
	}

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

	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func DescargarReporteCumplimientoService(perTributario, codLibro string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/cumplimiento/web/omisos/%s/%s/consultaReporteCumplimiento/exportardocumento", perTributario, codLibro)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
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
func ConsultarAjustesPosterioresRCEService(periodoSeleccionado, tipoInfo string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/propuesta/web?periodoSeleccionado=%s&tipoInfo=%s", periodoSeleccionado, tipoInfo)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
		Put(url)

	if err != nil {
		return fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

func ListarComprobantesAjustesPostService(periodoSeleccionado, page, perPage string) error {
	client := resty.New()
	token := os.Getenv("TOKEN")

	if token == "" {
		return fmt.Errorf("token no configurado")
	}

	url := fmt.Sprintf("https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rce/ajustesposteriores/web/comprobantesajuspost/%s/listarcap?page=%s&perPage=%s", periodoSeleccionado, page, perPage)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
		Get(url)

	if err != nil {
		return fmt.Errorf("error en petición: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("error SUNAT: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}
