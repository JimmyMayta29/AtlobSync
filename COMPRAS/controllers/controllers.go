package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/SwodLore/sire-api/COMPRAS/services"
	"github.com/SwodLore/sire-api/internal/sunat"
	"github.com/gin-gonic/gin"
)

var SunatClient *sunat.Client

func Init(client *sunat.Client) {
	SunatClient = client
}

func getRuc(c *gin.Context) string {
	if r := c.GetString("ruc"); r != "" {
		return r
	}
	return c.Param("ruc")
}


// 2 Servicio Web Api aceptar propuesta
func AceptarPropuestaController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	result, err := svc.AceptarPropuestaService(perTributario)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message": "Propuesta aceptada exitosamente",
		"data":    result,
	})
}

// 3 Servicio Web Api importar reemplazo de la propuesta
func ReemplazoPropuestaController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	filename := c.PostForm("filename")
	filetype := c.PostForm("filetype")
	numRuc := c.PostForm("numRuc")
	codOrigenEnvio := c.PostForm("codOrigenEnvio")
	codProceso := c.PostForm("codProceso")
	codTipoCorrelativo := c.PostForm("codTipoCorrelativo")
	nomArchivoImportacion := c.PostForm("nomArchivoImportacion")
	codLibro := c.PostForm("codLibro")

	if filename == "" || filetype == "" || numRuc == "" || codOrigenEnvio == "" || codProceso == "" || codTipoCorrelativo == "" || nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los campos son obligatorios"})
		return
	}

	err := svc.ReemplazoPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Reemplazo de propuesta procesado correctamente"})
}

// 4 Servicio Web Api registrar preliminar
func RegistrarPreliminarController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	err := svc.RegistrarPreliminarService(perTributario)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Preliminar registrado correctamente"})
}

// 5 Servicio Web Api cargar registro de compra no domiciliados
func CargarNoDomiciliadosController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	filename := c.PostForm("filename")
	filetype := c.PostForm("filetype")
	numRuc := c.PostForm("numRuc")
	codOrigenEnvio := c.PostForm("codOrigenEnvio")
	codProceso := c.PostForm("codProceso")
	codTipoCorrelativo := c.PostForm("codTipoCorrelativo")
	nomArchivoImportacion := c.PostForm("nomArchivoImportacion")
	codLibro := c.PostForm("codLibro")

	if filename == "" || filetype == "" || numRuc == "" || codOrigenEnvio == "" || codProceso == "" || codTipoCorrelativo == "" || nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los campos son obligatorios"})
		return
	}

	err := svc.CargarNoDomiciliadosService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Carga de no domiciliados procesada correctamente"})
}

// 6 Servicio Web Api importar datos complementarios de los CP de la propuesta
func ImportarComplementoPropuestaController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	filename := c.PostForm("filename")
	filetype := c.PostForm("filetype")
	numRuc := c.PostForm("numRuc")
	codOrigenEnvio := c.PostForm("codOrigenEnvio")
	codProceso := c.PostForm("codProceso")
	codTipoCorrelativo := c.PostForm("codTipoCorrelativo")
	nomArchivoImportacion := c.PostForm("nomArchivoImportacion")
	codLibro := c.PostForm("codLibro")

	if filename == "" || filetype == "" || numRuc == "" || codOrigenEnvio == "" || codProceso == "" || codTipoCorrelativo == "" || nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los campos son obligatorios"})
		return
	}

	err := svc.ImportarComplementoPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Complemento de propuesta importado correctamente"})
}

// 7 Servicio Web Api importar nuevos comprobantes preliminar
func ImportarComprobantesPreliminarController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	filename := c.PostForm("filename")
	filetype := c.PostForm("filetype")
	numRuc := c.PostForm("numRuc")
	codOrigenEnvio := c.PostForm("codOrigenEnvio")
	codProceso := c.PostForm("codProceso")
	codTipoCorrelativo := c.PostForm("codTipoCorrelativo")
	nomArchivoImportacion := c.PostForm("nomArchivoImportacion")
	codLibro := c.PostForm("codLibro")

	if filename == "" || filetype == "" || numRuc == "" || codOrigenEnvio == "" || codProceso == "" || codTipoCorrelativo == "" || nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los campos son obligatorios"})
		return
	}

	err := svc.ImportarComprobantesPreliminarService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Comprobantes preliminares importados correctamente"})
}

// 8 Servicio Web Api incluir-excluir comprobantes de la propuesta
func IncluirExcluirPropuestaController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	filename := c.PostForm("filename")
	filetype := c.PostForm("filetype")
	numRuc := c.PostForm("numRuc")
	codOrigenEnvio := c.PostForm("codOrigenEnvio")
	codProceso := c.PostForm("codProceso")
	codTipoCorrelativo := c.PostForm("codTipoCorrelativo")
	nomArchivoImportacion := c.PostForm("nomArchivoImportacion")
	codLibro := c.PostForm("codLibro")

	if filename == "" || filetype == "" || numRuc == "" || codOrigenEnvio == "" || codProceso == "" || codTipoCorrelativo == "" || nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los campos son obligatorios"})
		return
	}

	err := svc.IncluirExcluirPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Incluir/excluir propuesta procesado correctamente"})
}

// 9 Servicio Web Api importar nuevos comprobantes de pago
func ImportarComprobantesPropuestaController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	filename := c.PostForm("filename")
	filetype := c.PostForm("filetype")
	numRuc := c.PostForm("numRuc")
	codOrigenEnvio := c.PostForm("codOrigenEnvio")
	codProceso := c.PostForm("codProceso")
	codTipoCorrelativo := c.PostForm("codTipoCorrelativo")
	nomArchivoImportacion := c.PostForm("nomArchivoImportacion")
	codLibro := c.PostForm("codLibro")

	if filename == "" || filetype == "" || numRuc == "" || codOrigenEnvio == "" || codProceso == "" || codTipoCorrelativo == "" || nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los campos son obligatorios"})
		return
	}

	err := svc.ImportarComprobantesPropuestaService(perTributario, filename, filetype, numRuc, codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Comprobantes de propuesta importados correctamente"})
}

// 10 Servicio Web Api importar tipo de cambio masivo
func ImportarTipoCambioMasivoController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codLibro := c.Param("codLibro")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es requerido"})
		return
	}

	file, header, err := c.Request.FormFile("archivo")
	if err != nil {
		c.JSON(400, gin.H{"error": "Archivo es requerido - " + err.Error()})
		return
	}
	defer file.Close()

	err = svc.ImportarTipoCambioMasivoService(perTributario, codLibro, file, header)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Tipo de cambio masivo importado correctamente"})
}

// 11 Servicio Web Api actualizar reintegro del crédito fiscal
func ActualizarReintegroCreditoController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	var requestBody struct {
		DatosFV621 struct {
			ValorRCF     float64 `json:"valorRCF" binding:"required"`
			ValorCFE     float64 `json:"valorCFE" binding:"required"`
			FactProrrata float64 `json:"factProrrata" binding:"required"`
		} `json:"datosFV621" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(400, gin.H{"error": "Datos inválidos: " + err.Error()})
		return
	}

	err := svc.ActualizarReintegroCreditoService(perTributario, requestBody.DatosFV621.ValorRCF, requestBody.DatosFV621.ValorCFE, requestBody.DatosFV621.FactProrrata)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Reintegro de crédito fiscal actualizado correctamente"})
}

// 12 Servicio Web Api actualizar crédito fiscal especial
func ActualizarCreditoFiscalEspecialController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	var requestBody struct {
		DatosFV621 struct {
			ValorCFE float64 `json:"valorCFE" binding:"required"`
		} `json:"datosFV621" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(400, gin.H{"error": "Datos inválidos: " + err.Error()})
		return
	}

	err := svc.ActualizarCreditoFiscalEspecialService(perTributario, requestBody.DatosFV621.ValorCFE)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Crédito fiscal especial actualizado correctamente"})
}

// 13 Servicio Web Api actualizar coeficiente de prorrata
func ActualizarCoeficienteProrrataController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	var requestBody struct {
		DatosFV621 struct {
			FactProrrata float64 `json:"factProrrata" binding:"required"`
			ValorRCF     float64 `json:"valorRCF" binding:"required"`
			ValorCFE     float64 `json:"valorCFE" binding:"required"`
		} `json:"datosFV621" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(400, gin.H{"error": "Datos inválidos: " + err.Error()})
		return
	}

	err := svc.ActualizarCoeficienteProrrataService(perTributario, requestBody.DatosFV621.FactProrrata, requestBody.DatosFV621.ValorRCF, requestBody.DatosFV621.ValorCFE)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Coeficiente de prorrata actualizado correctamente"})
}

// 14 Servicio Web Api consultar FV0621
func ConsultarFV0621Controller(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	periodoSeleccionado := c.Query("periodoSeleccionado")
	tipoInfo := c.Query("tipoInfo")

	if periodoSeleccionado == "" {
		c.JSON(400, gin.H{"error": "periodoSeleccionado es requerido"})
		return
	}

	result, err := svc.ConsultarFV0621Service(periodoSeleccionado, tipoInfo)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 15 Servicio Web Api eliminar comprobante de la propuesta
func EliminarComprobantePropuestaController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "Error al leer body: " + err.Error()})
		return
	}

	if len(body) == 0 {
		c.JSON(400, gin.H{"error": "Body no puede estar vacío"})
		return
	}

	var requestBody []struct {
		NumSerieCDP string `json:"numSerieCDP"`
		NumCDP      string `json:"numCDP"`
		CodCar      string `json:"codCar"`
		CodTipoCDP  string `json:"codTipoCDP"`
	}

	if err := json.Unmarshal(body, &requestBody); err != nil {
		c.JSON(400, gin.H{"error": "JSON inválido: " + err.Error()})
		return
	}

	if len(requestBody) == 0 {
		c.JSON(400, gin.H{"error": "Debe enviar al menos un comprobante"})
		return
	}

	for i, comp := range requestBody {
		if comp.NumSerieCDP == "" || comp.NumCDP == "" || comp.CodCar == "" || comp.CodTipoCDP == "" {
			c.JSON(400, gin.H{"error": fmt.Sprintf("Comprobante %d: todos los campos son obligatorios", i+1)})
			return
		}
	}

	err = svc.EliminarComprobantePropuestaService(perTributario, requestBody)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Comprobante(s) eliminado(s) correctamente"})
}

// 16 Servicio Web Api eliminar comprobante del preliminar
func EliminarComprobantePreliminarController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "Error al leer body: " + err.Error()})
		return
	}

	if len(body) == 0 {
		c.JSON(400, gin.H{"error": "Body no puede estar vacío"})
		return
	}

	var requestBody []struct {
		CodTipoCDP  string `json:"codTipoCDP"`
		NumSerieCDP string `json:"numSerieCDP"`
		NumCDP      string `json:"numCDP"`
		CodCar      string `json:"codCar"`
	}

	if err := json.Unmarshal(body, &requestBody); err != nil {
		c.JSON(400, gin.H{"error": "JSON inválido: " + err.Error()})
		return
	}

	if len(requestBody) == 0 {
		c.JSON(400, gin.H{"error": "Debe enviar al menos un comprobante"})
		return
	}

	for i, comp := range requestBody {
		if comp.CodTipoCDP == "" || comp.NumSerieCDP == "" || comp.NumCDP == "" || comp.CodCar == "" {
			c.JSON(400, gin.H{"error": fmt.Sprintf("Comprobante %d: todos los campos son obligatorios", i+1)})
			return
		}
	}

	err = svc.EliminarComprobantePreliminarService(perTributario, requestBody)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Comprobante(s) eliminado(s) del preliminar correctamente"})
}

// 17 Servicio Web Api eliminar preliminar
func EliminarPreliminar(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	indEliminar := c.Param("indEliminar")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	if indEliminar == "" {
		c.JSON(400, gin.H{"error": "indEliminar es requerido"})
		return
	}

	if indEliminar != "1" && indEliminar != "2" {
		c.JSON(400, gin.H{"error": "indEliminar debe ser 1 o 2"})
		return
	}

	err := svc.EliminarPreliminarSUNAT(perTributario, indEliminar)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Preliminar eliminado exitosamente"})
}

// 18 Servicio Web Api cargar ajustes posteriores
func CargarAjustesPosteriores(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	var request struct {
		Filename              string `json:"filename"`
		Filetype              string `json:"filetype"`
		NumRuc                string `json:"numRuc"`
		PerTributario         string `json:"perTributario"`
		CodOrigenEnvio        string `json:"codOrigenEnvio"`
		CodProceso            string `json:"codProceso"`
		CodTipoCorrelativo    string `json:"codTipoCorrelativo"`
		NomArchivoImportacion string `json:"nomArchivoImportacion"`
		CodLibro              string `json:"codLibro"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "JSON inválido: " + err.Error()})
		return
	}

	if request.NumRuc == "" {
		c.JSON(400, gin.H{"error": "numRuc es requerido"})
		return
	}

	if request.PerTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	err := svc.CargarAjustesPosterioresSUNAT(request.Filename, request.Filetype, request.NumRuc,
		request.PerTributario, request.CodOrigenEnvio, request.CodProceso, request.CodTipoCorrelativo,
		request.NomArchivoImportacion, request.CodLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Ajustes posteriores cargados exitosamente"})
}

// 19 Servicio Web Api enviar ajustes posteriores
func EnviarAjustesPosteriores(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codOrigenEnvio := c.Param("codOrigenEnvio")

	var request struct {
		NumAjustePosterior string `json:"numAjustePosterior"`
		CodLibro           string `json:"codLibro"`
		NumTicket          string `json:"numTicket"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "JSON inválido: " + err.Error()})
		return
	}

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es requerido"})
		return
	}

	if request.NumAjustePosterior == "" {
		c.JSON(400, gin.H{"error": "numAjustePosterior es requerido"})
		return
	}

	if request.CodLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es requerido"})
		return
	}

	if request.NumTicket == "" {
		c.JSON(400, gin.H{"error": "numTicket es requerido"})
		return
	}

	err := svc.EnviarAjustesPosterioresSUNAT(perTributario, codOrigenEnvio, request.NumAjustePosterior, request.CodLibro, request.NumTicket)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Ajustes posteriores enviados exitosamente"})
}

// 20 Servicio Web Api eliminar comprobante de ajustes posteriores
func EliminarComprobanteAjustes(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	indTipoAjustePosterior := c.Param("indTipoAjustePosterior")

	var request struct {
		CodAjustePosterior string `json:"codAjustePosterior"`
		DetalleAjustes     []struct {
			CodTipoCDP  string `json:"codTipoCDP"`
			NumSerieCDP string `json:"numSerieCDP"`
			NumCDP      string `json:"numCDP"`
			CodCar      string `json:"codCar"`
			Id          string `json:"id"`
		} `json:"detalleAjustes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "JSON inválido: " + err.Error()})
		return
	}

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	if indTipoAjustePosterior == "" {
		c.JSON(400, gin.H{"error": "indTipoAjustePosterior es requerido"})
		return
	}

	if request.CodAjustePosterior == "" {
		c.JSON(400, gin.H{"error": "codAjustePosterior es requerido"})
		return
	}

	if len(request.DetalleAjustes) == 0 {
		c.JSON(400, gin.H{"error": "detalleAjustes es requerido"})
		return
	}

	err := svc.EliminarComprobanteAjustesSUNAT(perTributario, indTipoAjustePosterior, request)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Comprobante de ajustes eliminado exitosamente"})
}

// 21 Servicio Web Api cargar ajustes posteriores no domiciliados
func CargarAjustesNoDomiciliados(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	var request struct {
		Filename              string `json:"filename"`
		Filetype              string `json:"filetype"`
		NumRuc                string `json:"numRuc"`
		PerTributario         string `json:"perTributario"`
		CodOrigenEnvio        string `json:"codOrigenEnvio"`
		CodProceso            string `json:"codProceso"`
		CodTipoCorrelativo    string `json:"codTipoCorrelativo"`
		NomArchivoImportacion string `json:"nomArchivoImportacion"`
		CodLibro              string `json:"codLibro"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "JSON inválido: " + err.Error()})
		return
	}

	if request.NumRuc == "" {
		c.JSON(400, gin.H{"error": "numRuc es requerido"})
		return
	}

	if request.PerTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	// Valores por defecto específicos para no domiciliados
	if request.CodOrigenEnvio == "" {
		request.CodOrigenEnvio = "2"
	}
	if request.CodProceso == "" {
		request.CodProceso = "60" // Código específico para no domiciliados
	}
	if request.CodTipoCorrelativo == "" {
		request.CodTipoCorrelativo = "01"
	}
	if request.CodLibro == "" {
		request.CodLibro = "080000"
	}

	numTicket, err := svc.CargarAjustesNoDomiciliadosSUNAT(request.Filename, request.Filetype, request.NumRuc,
		request.PerTributario, request.CodOrigenEnvio, request.CodProceso, request.CodTipoCorrelativo,
		request.NomArchivoImportacion, request.CodLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message":   "Ajustes no domiciliados cargados exitosamente",
		"numTicket": numTicket,
	})
}

// 22 Servicio Web Api enviar ajustes posteriores no domiciliados
func EnviarAjustesNoDomiciliados(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	numAjustePosterior := c.Param("numAjustePosterior")
	codLibro := c.Param("codLibro")
	numTicket := c.Param("numTicket")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	if numAjustePosterior == "" {
		c.JSON(400, gin.H{"error": "numAjustePosterior es requerido"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es requerido"})
		return
	}

	if numTicket == "" {
		c.JSON(400, gin.H{"error": "numTicket es requerido"})
		return
	}

	err := svc.EnviarAjustesNoDomiciliadosSUNAT(perTributario, numAjustePosterior, codLibro, numTicket)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Ajustes no domiciliados enviados exitosamente"})
}

// 23 Servicio Web Api eliminar comprobante de ajustes posteriores no domiciliados
func EliminarComprobanteAjustesPosteriorNoDOM(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	indTipoAjustePosteriorStr := c.Param("indTipoAjustePosterior")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	if indTipoAjustePosteriorStr == "" {
		c.JSON(400, gin.H{"error": "indTipoAjustePosterior es requerido"})
		return
	}

	indTipoAjustePosterior, err := strconv.Atoi(indTipoAjustePosteriorStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "indTipoAjustePosterior debe ser un número"})
		return
	}

	var requestData struct {
		CodAjustePosterior string `json:"codAjustePosterior"`
		DetalleAjustes     []struct {
			CodTipoCDP  string `json:"codTipoCDP"`
			NumSerieCDP string `json:"numSerieCDP"`
			NumCDP      string `json:"numCDP"`
			CodCar      string `json:"codCar"`
			ID          string `json:"id"`
		} `json:"detalleAjustes"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{"error": "Datos inválidos en el body"})
		return
	}

	err = svc.EliminarComprobanteAjustesPosteriorNoDOM(perTributario, indTipoAjustePosterior, requestData.CodAjustePosterior, requestData.DetalleAjustes)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Comprobante eliminado exitosamente"})
}

// 24 Servicio Web Api cargar ajustes posteriores de periodos anteriores
func CargarAjustesPosterioresPeriodosAnteriores(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	var requestData struct {
		Filename              string `json:"filename"`
		Filetype              string `json:"filetype"`
		NumRuc                string `json:"numRuc"`
		PerTributario         string `json:"perTributario"`
		CodOrigenEnvio        string `json:"codOrigenEnvio"`
		CodProceso            string `json:"codProceso"`
		CodTipoCorrelativo    string `json:"codTipoCorrelativo"`
		NomArchivoImportacion string `json:"nomArchivoImportacion"`
		CodLibro              string `json:"codLibro"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{"error": "Datos inválidos en el body"})
		return
	}

	if requestData.Filename == "" || requestData.Filetype == "" || requestData.NumRuc == "" ||
		requestData.PerTributario == "" || requestData.CodOrigenEnvio == "" || requestData.CodProceso == "" ||
		requestData.CodTipoCorrelativo == "" || requestData.NomArchivoImportacion == "" || requestData.CodLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los campos son requeridos"})
		return
	}

	response, err := svc.CargarAjustesPosterioresPeriodosAnteriores(
		requestData.Filename, requestData.Filetype, requestData.NumRuc, requestData.PerTributario,
		requestData.CodOrigenEnvio, requestData.CodProceso, requestData.CodTipoCorrelativo,
		requestData.NomArchivoImportacion, requestData.CodLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, response)
}

// 25 Servicio Web Api enviar ajustes posteriores de periodos anteriores
func EnviarAjustesPosterioresPeriodosAnteriores(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	numAjustePosterior := c.Param("numAjustePosterior")
	codLibro := c.Param("codLibro")
	numTicket := c.Param("numTicket")

	if perTributario == "" || numAjustePosterior == "" || codLibro == "" || numTicket == "" {
		c.JSON(400, gin.H{"error": "Todos los parámetros de URL son requeridos"})
		return
	}

	var requestData struct {
		CodAjustePosterior string `json:"codAjustePosterior"`
		DetalleAjustes     []struct {
			CodTipoCDP  string `json:"codTipoCDP"`
			NumSerieCDP string `json:"numSerieCDP"`
			NumCDP      string `json:"numCDP"`
			NumTicket   string `json:"numTicket"`
		} `json:"detalleAjustes"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{"error": "Datos inválidos en el body"})
		return
	}

	if requestData.CodAjustePosterior == "" {
		c.JSON(400, gin.H{"error": "codAjustePosterior es requerido"})
		return
	}

	err := svc.EnviarAjustesPosterioresPeriodosAnteriores(perTributario, numAjustePosterior, codLibro,
		numTicket, requestData.CodAjustePosterior, requestData.DetalleAjustes)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Ajustes posteriores enviados exitosamente"})
}

// 26 Servicio Web Api eliminar comprobante de ajustes posteriores de periodos anteriores
func EliminarComprobanteAjustesPosterioresPeriodosAnteriores(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	indTipoAjustePosteriorStr := c.Param("indTipoAjustePosterior")
	perTributario := c.Param("perTributario")

	if indTipoAjustePosteriorStr == "" || perTributario == "" {
		c.JSON(400, gin.H{"error": "indTipoAjustePosterior y perTributario son requeridos"})
		return
	}

	indTipoAjustePosterior, err := strconv.Atoi(indTipoAjustePosteriorStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "indTipoAjustePosterior debe ser un número"})
		return
	}

	var requestData struct {
		CodAjustePosterior string `json:"codAjustePosterior"`
		DetalleAjustes     []struct {
			CodTipoCDP  string `json:"codTipoCDP"`
			NumSerieCDP string `json:"numSerieCDP"`
			NumCDP      string `json:"numCDP"`
		} `json:"detalleAjustes"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{"error": "Datos inválidos en el body"})
		return
	}

	if requestData.CodAjustePosterior == "" {
		c.JSON(400, gin.H{"error": "codAjustePosterior es requerido"})
		return
	}

	err = svc.EliminarComprobanteAjustesPosterioresPeriodosAnteriores(indTipoAjustePosterior, perTributario,
		requestData.CodAjustePosterior, requestData.DetalleAjustes)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Comprobante eliminado exitosamente"})
}

// 27 Servicio Web Api cargar ajustes posteriores de periodos anteriores no domiciliados
func CargarAjustesPosterioresPeriodosAnterioresNoDOM(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	var requestData struct {
		Filename              string `json:"filename"`
		Filetype              string `json:"filetype"`
		NumRuc                string `json:"numRuc"`
		PerTributario         string `json:"perTributario"`
		CodOrigenEnvio        string `json:"codOrigenEnvio"`
		CodProceso            string `json:"codProceso"`
		CodTipoCorrelativo    string `json:"codTipoCorrelativo"`
		NomArchivoImportacion string `json:"nomArchivoImportacion"`
		CodLibro              string `json:"codLibro"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{"error": "Datos inválidos en el body"})
		return
	}

	if requestData.Filename == "" || requestData.Filetype == "" || requestData.NumRuc == "" ||
		requestData.PerTributario == "" || requestData.CodOrigenEnvio == "" || requestData.CodProceso == "" ||
		requestData.CodTipoCorrelativo == "" || requestData.NomArchivoImportacion == "" || requestData.CodLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los campos son requeridos"})
		return
	}

	response, err := svc.CargarAjustesPosterioresPeriodosAnterioresNoDOM(
		requestData.Filename, requestData.Filetype, requestData.NumRuc, requestData.PerTributario,
		requestData.CodOrigenEnvio, requestData.CodProceso, requestData.CodTipoCorrelativo,
		requestData.NomArchivoImportacion, requestData.CodLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, response)
}

// 28 Servicio Web Api enviar ajustes posteriores de periodos anteriores no domiciliados
func EnviarAjustesPosterioresController(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	numAjustePosterior := c.Param("numAjustePosterior")
	codLibro := c.Param("codLibro")
	numTicket := c.Param("numTicket")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if numAjustePosterior == "" {
		c.JSON(400, gin.H{"error": "numAjustePosterior es obligatorio"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	if numTicket == "" {
		c.JSON(400, gin.H{"error": "numTicket es obligatorio"})
		return
	}

	err := svc.EnviarAjustesPosterioresService(perTributario, numAjustePosterior, codLibro, numTicket)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Ajustes posteriores enviados exitosamente"})
}

// 29 Servicio Web Api eliminar comprobante de ajustes posteriores de periodos anteriores no domiciliados
func EliminarComprobanteAjustesRCE(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	numAjustePosterior := c.Param("numAjustePosterior")
	perTributario := c.Param("perTributario")

	if numAjustePosterior == "" {
		c.JSON(400, gin.H{"error": "numAjustePosterior es requerido"})
		return
	}

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(400, gin.H{"error": "Body JSON inválido"})
		return
	}

	result, err := svc.EliminarComprobanteAjustesRCESUNAT(numAjustePosterior, perTributario, requestBody)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 31 Servicio Web Api consultar estado ticket
func ConsultarEstadoTickets(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perIni := c.Query("perIni")
	perFin := c.Query("perFin")
	page := c.Query("page")
	perPage := c.Query("perPage")
	numTicket := c.Query("numTicket")

	if perIni == "" {
		c.JSON(400, gin.H{"error": "perIni es requerido"})
		return
	}

	if perFin == "" {
		c.JSON(400, gin.H{"error": "perFin es requerido"})
		return
	}

	if page == "" {
		c.JSON(400, gin.H{"error": "page es requerido"})
		return
	}

	if perPage == "" {
		c.JSON(400, gin.H{"error": "perPage es requerido"})
		return
	}

	result, err := svc.ConsultarEstadoTicketsSUNAT(perIni, perFin, page, perPage, numTicket)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 32 Servicio Web Api descargar archivo
func DescargarArchivoReporte(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	nomArchivoReporte := c.Query("nomArchivoReporte")
	codTipoArchivoReporte := c.Query("codTipoArchivoReporte")

	if nomArchivoReporte == "" {
		c.JSON(400, gin.H{"error": "nomArchivoReporte es requerido"})
		return
	}

	if codTipoArchivoReporte == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivoReporte es requerido"})
		return
	}

	result, err := svc.DescargarArchivoReporteSUNAT(nomArchivoReporte, codTipoArchivoReporte)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 33 Servicio Web Api consultar año y mes del RCE
func ConsultarPeriodosRCE(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	codLibro := c.Param("codLibro")
	if codLibro == "" {
		codLibro = "080000"
	}

	result, err := svc.ConsultarPeriodosRCESUNAT(codLibro)
	if err != nil {
		sunat.RespondWithError(c, err)
		return
	}

	c.JSON(200, result)
}

// 34 Servicio Web Api descargar propuesta
func DescargarPropuestaRCE(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codTipoArchivo := c.Query("codTipoArchivo")
	codOrigenEnvio := c.Query("codOrigenEnvio")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es requerido"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es requerido"})
		return
	}

	params := map[string]string{
		"codTipoArchivo":    codTipoArchivo,
		"codOrigenEnvio":    codOrigenEnvio,
		"fecEmisionIni":     c.Query("fecEmisionIni"),
		"fecEmisionFin":     c.Query("fecEmisionFin"),
		"codTipoCDP":        c.Query("codTipoCDP"),
		"numSerieCDP":       c.Query("numSerieCDP"),
		"numCDP":            c.Query("numCDP"),
		"codInconsistencia": c.Query("codInconsistencia"),
		"codCar":            c.Query("codCar"),
		"numDocAdquiriente": c.Query("numDocAdquiriente"),
		"mtoDesde":          c.Query("mtoDesde"),
		"mtoHasta":          c.Query("mtoHasta"),
	}

	result, err := svc.DescargarPropuestaRCESUNAT(perTributario, params)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 35 Servicio Web Api descargar resumen
func DescargarResumenRCE(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codTipoResumen := c.Param("codTipoResumen")
	codTipoArchivo := c.Param("codTipoArchivo")
	codLibro := c.Query("codLibro")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	if codTipoResumen == "" {
		c.JSON(400, gin.H{"error": "codTipoResumen es requerido"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es requerido"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es requerido"})
		return
	}

	result, err := svc.DescargarResumenRCESUNAT(perTributario, codTipoResumen, codTipoArchivo, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 36 Servicio Web Api descargar resumen inconsistencias RCE

// 37 Servicio Web Api descargar excluidos
func DescargarExcluidosRCE(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codTipoArchivo := c.Query("codTipoArchivo")
	codOrigenEnvio := c.Query("codOrigenEnvio")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es requerido"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es requerido"})
		return
	}

	params := map[string]string{
		"montoDesde":        c.Query("montoDesde"),
		"montoHasta":        c.Query("montoHasta"),
		"fecEmisionIni":     c.Query("fecEmisionIni"),
		"fecEmisionFin":     c.Query("fecEmisionFin"),
		"numRucCliente":     c.Query("numRucCliente"),
		"codCar":            c.Query("codCar"),
		"tipoDocumento":     c.Query("tipoDocumento"),
		"codInconsistencia": c.Query("codInconsistencia"),
		"numSerieCDP":       c.Query("numSerieCDP"),
		"numCDP":            c.Query("numCDP"),
		"codTipoArchivo":    codTipoArchivo,
		"codOrigenEnvio":    codOrigenEnvio,
	}

	result, err := svc.DescargarExcluidosRCESUNAT(perTributario, params)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 38 Servicio Web Api eliminar comprobante no domiciliado
func EliminarComprobanteNoDomiciliado(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(422, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(422, gin.H{"error": "perTributario debe tener formato YYYYMM"})
		return
	}

	var requestBody struct {
		NoDomiciliados []struct {
			CodCar      string `json:"codCar" binding:"required"`
			CodTipoCDP  string `json:"codTipoCDP" binding:"required"`
			NumSerieCDP string `json:"numSerieCDP" binding:"required"`
			NumCDP      string `json:"numCDP" binding:"required"`
		} `json:"noDomiciliados" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(400, gin.H{"error": "Formato de JSON inválido", "details": err.Error()})
		return
	}

	if len(requestBody.NoDomiciliados) == 0 {
		c.JSON(422, gin.H{"error": "noDomiciliados no puede estar vacío"})
		return
	}

	err := svc.EliminarComprobanteNoDomiciliadoSUNAT(perTributario, requestBody)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Comprobante eliminado exitosamente"})
}

// 39 Servicio Web Api exportar preliminar de registro de compras no domiciliados
func ExportarPreliminarNoDomiciliados(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(422, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(422, gin.H{"error": "perTributario debe tener formato YYYYMM"})
		return
	}

	codTipoArchivo := c.Query("codTipoArchivo")
	if codTipoArchivo == "" {
		c.JSON(422, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	codOrigenEnvio := c.Query("codOrigenEnvio")
	if codOrigenEnvio == "" {
		c.JSON(422, gin.H{"error": "codOrigenEnvio es obligatorio"})
		return
	}

	queryParams := map[string]string{
		"codTipoArchivo":                  codTipoArchivo,
		"codOrigenEnvio":                  codOrigenEnvio,
		"mtoTotalDesde":                   c.Query("mtoTotalDesde"),
		"mtoTotalHasta":                   c.Query("mtoTotalHasta"),
		"fecEmisionIni":                   c.Query("fecEmisionIni"),
		"fecEmisionFin":                   c.Query("fecEmisionFin"),
		"numDocIdentidadClienteProveedor": c.Query("numDocIdentidadClienteProveedor"),
		"numSerieCDP":                     c.Query("numSerieCDP"),
		"numCDP":                          c.Query("numCDP"),
		"codTipoCDP":                      c.Query("codTipoCDP"),
		"numDocAdquiriente":               c.Query("numDocAdquiriente"),
	}

	result, err := svc.ExportarPreliminarNoDomiciliadosSUNAT(perTributario, queryParams)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 40 Servicio Web Api exportar preliminar de registro de compras
func ExportarPreliminarRegistroCompras(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(422, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(422, gin.H{"error": "perTributario debe tener formato YYYYMM"})
		return
	}

	codTipoArchivo := c.Query("codTipoArchivo")
	if codTipoArchivo == "" {
		c.JSON(422, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	codOrigenEnvio := c.Query("codOrigenEnvio")
	if codOrigenEnvio == "" {
		c.JSON(422, gin.H{"error": "codOrigenEnvio es obligatorio"})
		return
	}

	queryParams := map[string]string{
		"codTipoArchivo":                  codTipoArchivo,
		"codOrigenEnvio":                  codOrigenEnvio,
		"mtoTotalDesde":                   c.Query("mtoTotalDesde"),
		"mtoTotalHasta":                   c.Query("mtoTotalHasta"),
		"fecEmisionIni":                   c.Query("fecEmisionIni"),
		"fecEmisionFin":                   c.Query("fecEmisionFin"),
		"numDocIdentidadClienteProveedor": c.Query("numDocIdentidadClienteProveedor"),
		"numSerieCDP":                     c.Query("numSerieCDP"),
		"numCDP":                          c.Query("numCDP"),
		"codTipoCDP":                      c.Query("codTipoCDP"),
		"numDocAdquiriente":               c.Query("numDocAdquiriente"),
	}

	result, err := svc.ExportarPreliminarRegistroComprasSUNAT(perTributario, queryParams)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 41 Servicio Web Api descargar reporte de casillas
func DescargarReporteCasillas(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	tipoReporte := c.Param("tipoReporte")
	tipoDescarga := c.Param("tipoDescarga")

	if perTributario == "" {
		c.JSON(422, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(422, gin.H{"error": "perTributario debe tener formato YYYYMM"})
		return
	}

	if tipoReporte == "" {
		c.JSON(422, gin.H{"error": "tipoReporte es obligatorio"})
		return
	}

	if tipoReporte != "1" && tipoReporte != "2" {
		c.JSON(422, gin.H{"error": "tipoReporte debe ser 1 (Preliminar) o 2 (Comparada)"})
		return
	}

	if tipoDescarga == "" {
		c.JSON(422, gin.H{"error": "tipoDescarga es obligatorio"})
		return
	}

	fileData, contentType, err := svc.DescargarReporteCasillasSUNAT(perTributario, tipoReporte, tipoDescarga)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("reporte_casillas_%s_%s.%s", perTributario, tipoReporte, tipoDescarga)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(200, contentType, fileData)
}

// 42 Servicio Web Api descargar inconsistencias en registros preliminar registrado
func DescargarInconsistenciasPreliminar(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codTipoArchivo := c.Param("codTipoArchivo")

	if perTributario == "" {
		c.JSON(422, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(422, gin.H{"error": "perTributario debe tener formato YYYYMM"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(422, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	cntlimite := c.Query("cntlimite")
	if cntlimite == "" {
		c.JSON(422, gin.H{"error": "cntlimite es obligatorio"})
		return
	}

	fileData, contentType, err := svc.DescargarInconsistenciasPreliminarSUNAT(perTributario, codTipoArchivo, cntlimite)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("inconsistencias_%s.%s", perTributario, codTipoArchivo)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(200, contentType, fileData)
}

// 43 Servicio Web Api descargar inconsistencias por montos totales
func DescargarInconsistenciasMontosTotales(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(422, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(422, gin.H{"error": "perTributario debe tener formato YYYYMM"})
		return
	}

	codTipoArchivo := c.Query("codTipoArchivo")
	if codTipoArchivo == "" {
		c.JSON(422, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	codLibro := c.Query("codLibro")
	if codLibro == "" {
		c.JSON(422, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	queryParams := map[string]string{
		"codTipoArchivo": codTipoArchivo,
		"codLibro":       codLibro,
	}

	result, err := svc.DescargarInconsistenciasMontosTotalesSUNAT(perTributario, queryParams)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 44 Servicio Web Api descargar inconsistencias por comprobante pago
func DescargarInconsistenciasComprobantes(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codLibro := c.Param("codLibro")

	if perTributario == "" {
		c.JSON(422, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(422, gin.H{"error": "perTributario debe tener formato YYYYMM"})
		return
	}

	if codLibro == "" {
		c.JSON(422, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	codTipoArchivo := c.Query("codTipoArchivo")
	if codTipoArchivo == "" {
		c.JSON(422, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	codOrigenEnvio := c.Query("codOrigenEnvio")
	if codOrigenEnvio == "" {
		c.JSON(422, gin.H{"error": "codOrigenEnvio es obligatorio"})
		return
	}

	queryParams := map[string]string{
		"fecEmisionInicio":                c.Query("fecEmisionInicio"),
		"fecEmisionFin":                   c.Query("fecEmisionFin"),
		"codInconsistencia":               c.Query("codInconsistencia"),
		"numDocIdentidadClienteProveedor": c.Query("numDocIdentidadClienteProveedor"),
		"codTipoCDP":                      c.Query("codTipoCDP"),
		"numSerieCDP":                     c.Query("numSerieCDP"),
		"numCDP":                          c.Query("numCDP"),
		"codTipoArchivo":                  codTipoArchivo,
		"mtoTotalDesde":                   c.Query("mtoTotalDesde"),
		"mtoTotalHasta":                   c.Query("mtoTotalHasta"),
		"codEstado":                       c.Query("codEstado"),
		"codOrigenEnvio":                  codOrigenEnvio,
	}

	result, err := svc.DescargarInconsistenciasComprobantesSUNAT(perTributario, codLibro, queryParams)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 45 Servicio Web Api descargar ajustes posteriores
func DescargarAjustesPosteriores(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(422, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(422, gin.H{"error": "perTributario debe tener formato YYYYMM"})
		return
	}

	codTipoArchivo := c.Query("codTipoArchivo")
	if codTipoArchivo == "" {
		c.JSON(422, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	codOrigenEnvio := c.Query("codOrigenEnvio")
	if codOrigenEnvio == "" {
		c.JSON(422, gin.H{"error": "codOrigenEnvio es obligatorio"})
		return
	}

	queryParams := map[string]string{
		"codTipoArchivo": codTipoArchivo,
		"codOrigenEnvio": codOrigenEnvio,
	}

	result, err := svc.DescargarAjustesPosterioresSUNAT(perTributario, queryParams)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 46 Servicio Web Api descargar ajustes posteriores no domiciliados
func DescargarAjustesPosterioresNoDomiciliados(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(422, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(422, gin.H{"error": "perTributario debe tener formato YYYYMM"})
		return
	}

	codTipoArchivo := c.Query("codTipoArchivo")
	if codTipoArchivo == "" {
		c.JSON(422, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	result, err := svc.DescargarAjustesPosterioresNoDomiciliadosSUNAT(perTributario, codTipoArchivo)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 47 Servicio Web Api descargar ajustes posteriores de periodos anteriores
func ExportarAjustesPosteriores(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codTipoArchivo := c.Query("codTipoArchivo")
	indAjustePosteriorPle := c.Query("indAjustePosteriorPle")
	codOrigenEnvio := c.Query("codOrigenEnvio")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	if indAjustePosteriorPle == "" {
		c.JSON(400, gin.H{"error": "indAjustePosteriorPle es obligatorio"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es obligatorio"})
		return
	}

	if indAjustePosteriorPle != "G" && indAjustePosteriorPle != "S" {
		c.JSON(400, gin.H{"error": "indAjustePosteriorPle debe ser G o S"})
		return
	}

	err := svc.ExportarAjustesPosterioresSUNAT(perTributario, codTipoArchivo, indAjustePosteriorPle, codOrigenEnvio)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Solicitud de exportación procesada correctamente"})
}

// 48 Servicio Web Api descargar ajustes posteriores de periodos anteriores no domiciliados
func ExportarAjustesPosterioresND(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codTipoArchivo := c.Query("codTipoArchivo")
	codOrigenEnvio := c.Query("codOrigenEnvio")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es obligatorio"})
		return
	}

	err := svc.ExportarAjustesPosterioresNDSUNAT(perTributario, codTipoArchivo, codOrigenEnvio)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Solicitud de exportación procesada correctamente"})
}

// 49 Servicio Web Api descargar constancia de recepción
func DescargarConstanciaRecepcion(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	nomConstanciaRecepcion := c.Query("nomConstanciaRecepcion")

	if nomConstanciaRecepcion == "" {
		c.JSON(400, gin.H{"error": "nomConstanciaRecepcion es obligatorio"})
		return
	}

	err := svc.DescargarConstanciaRecepcionSUNAT(nomConstanciaRecepcion)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Constancia de recepción descargada correctamente"})
}

// 50 Servicio Web Api descargar reporte consolidado registro por periodo
func DescargarReporteConsolidado(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codTipoArchivo := c.Query("codTipoArchivo")
	codMoneda := c.Query("codMoneda")
	codProceso := c.Query("codProceso")
	codOrigenEnvio := c.Query("codOrigen")
	lisPeriodos := c.Query("lisPeriodos")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	if codProceso == "" {
		c.JSON(400, gin.H{"error": "codProceso es obligatorio"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigen es obligatorio"})
		return
	}

	if lisPeriodos == "" {
		c.JSON(400, gin.H{"error": "lisPeriodos es obligatorio"})
		return
	}

	err := svc.DescargarReporteConsolidadoSUNAT(perTributario, codTipoArchivo, codMoneda, codProceso, codOrigenEnvio, lisPeriodos)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Solicitud de descarga procesada correctamente"})
}

// 51 Servicio Web Api descargar RCE por periodo
func DescargarRCEPorPeriodo(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codTipoArchivo := c.Query("codTipoArchivo")
	codMoneda := c.Query("codMoneda")
	codProceso := c.Query("codProceso")
	codOrigen := c.Query("codOrigen")
	lisPeriodos := c.Query("lisPeriodos")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	if codProceso == "" {
		c.JSON(400, gin.H{"error": "codProceso es obligatorio"})
		return
	}

	if codOrigen == "" {
		c.JSON(400, gin.H{"error": "codOrigen es obligatorio"})
		return
	}

	if lisPeriodos == "" {
		c.JSON(400, gin.H{"error": "lisPeriodos es obligatorio"})
		return
	}

	err := svc.DescargarRCEPorPeriodoSUNAT(perTributario, codTipoArchivo, codMoneda, codProceso, codOrigen, lisPeriodos)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Solicitud de descarga RCE procesada correctamente"})
}

// 52 Servicio Web Api descargar reporte inconsistencias por periodo
func DescargarInconsistenciasPeriodo(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	nomArchivo := c.Query("nomArchivo")
	codTipoArchivoReporte := c.Query("codTipoArchivoReporte")
	codOrigen := c.Query("codOrigen")

	if nomArchivo == "" {
		c.JSON(400, gin.H{"error": "nomArchivo es obligatorio"})
		return
	}

	if codTipoArchivoReporte == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivoReporte es obligatorio"})
		return
	}

	if codOrigen == "" {
		c.JSON(400, gin.H{"error": "codOrigen es obligatorio"})
		return
	}

	err := svc.DescargarInconsistenciasPeriodoSUNAT(nomArchivo, codTipoArchivoReporte, codOrigen)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Archivo de inconsistencias descargado correctamente"})
}

// 53 Servicio Web Api descargar reporte CAR por periodo y fase
func DescargarReporteCARPorPeriodo(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codOrigenEnvio := c.Query("codOrigenEnvio")
	codLibro := c.Query("codLibro")
	codFase := c.Query("codFase")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es obligatorio"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	if codFase == "" {
		c.JSON(400, gin.H{"error": "codFase es obligatorio"})
		return
	}

	err := svc.DescargarReporteCARPorPeriodoSUNAT(perTributario, codOrigenEnvio, codLibro, codFase)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Reporte CAR generado correctamente"})
}

// 54 Servicio Web Api descargar reporte estadístico compras por proveedor por periodo
func DescargarReporteEstadisticoCompras(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	numRuc := c.Query("numRuc")
	perTributario := c.Query("perTributario")
	fechaini := c.Query("fechaini")
	fechafin := c.Query("fechafin")
	numRucproveedor := c.Query("numRucproveedor")
	codTipoCDP := c.Query("codTipoCDP")
	codTipoArchivo := c.Query("codTipoArchivo")
	codTipoReporte := c.Query("codTipoReporte")
	codLibro := c.Query("codLibro")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	if codTipoReporte == "" {
		c.JSON(400, gin.H{"error": "codTipoReporte es obligatorio"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	err := svc.DescargarReporteEstadisticoComprasSUNAT(numRuc, perTributario, fechaini, fechafin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Reporte estadístico de compras generado correctamente"})
}

// 55 Servicio Web Api descargar reporte estadístico NC-ND por proveedor y periodo
func DescargarReporteEstadisticoNCND(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	numRuc := c.Query("numRuc")
	perTributario := c.Query("perTributario")
	fechaini := c.Query("fechaini")
	fechafin := c.Query("fechafin")
	numRucproveedor := c.Query("numRucproveedor")
	codTipoCDP := c.Query("codTipoCDP")
	codTipoArchivo := c.Query("codTipoArchivo")
	codTipoReporte := c.Query("codTipoReporte")
	codLibro := c.Query("codLibro")

	if numRuc == "" {
		c.JSON(400, gin.H{"error": "numRuc es obligatorio"})
		return
	}

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	if codTipoReporte == "" {
		c.JSON(400, gin.H{"error": "codTipoReporte es obligatorio"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	err := svc.DescargarReporteEstadisticoNCNDSUNAT(numRuc, perTributario, fechaini, fechafin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Reporte estadístico NC-ND generado correctamente"})
}

// 56 Servicio Web Api descargar reporte estadístico Compras por día y periodo
func DescargarReporteEstadisticoComprasDia(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	numRuc := c.Query("numRuc")
	perTributario := c.Query("perTributario")
	fechaIni := c.Query("fechaIni")
	fechaFin := c.Query("fechaFin")
	numRucproveedor := c.Query("numRucproveedor")
	codTipoCDP := c.Query("codTipoCDP")
	codTipoArchivo := c.Query("codTipoArchivo")
	codTipoReporte := c.Query("codTipoReporte")
	codLibro := c.Query("codLibro")

	if numRuc == "" {
		c.JSON(400, gin.H{"error": "numRuc es obligatorio"})
		return
	}

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	if codTipoReporte == "" {
		c.JSON(400, gin.H{"error": "codTipoReporte es obligatorio"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	err := svc.DescargarReporteEstadisticoComprasDiaSUNAT(numRuc, perTributario, fechaIni, fechaFin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Reporte estadístico compras por día generado correctamente"})
}

// 57 Servicio Web Api descargar reporte estadístico Compras por CIIU
func DescargarReporteEstadisticoComprasCIIU(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	numRuc := c.Query("numRuc")
	perTributario := c.Query("perTributario")
	fechaIni := c.Query("fechaIni")
	fechaFin := c.Query("fechaFin")
	numRucproveedor := c.Query("numRucproveedor")
	codTipoCDP := c.Query("codTipoCDP")
	codTipoArchivo := c.Query("codTipoArchivo")
	codTipoReporte := c.Query("codTipoReporte")
	codLibro := c.Query("codLibro")

	if numRuc == "" {
		c.JSON(400, gin.H{"error": "numRuc es obligatorio"})
		return
	}

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	if codTipoReporte == "" {
		c.JSON(400, gin.H{"error": "codTipoReporte es obligatorio"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	err := svc.DescargarReporteEstadisticoComprasCIIUSUNAT(numRuc, perTributario, fechaIni, fechaFin, numRucproveedor, codTipoCDP, codTipoArchivo, codTipoReporte, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Reporte estadístico compras por CIIU generado correctamente"})
}

// 58 Servicio Web Api descargar reporte de cumplimiento
func DescargarReporteCumplimiento(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	perTributario := c.Param("perTributario")
	codLibro := c.Param("codLibro")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(400, gin.H{"error": "perTributario debe tener formato YYYYMM"})
		return
	}

	if codLibro != "080000" {
		c.JSON(400, gin.H{"error": "codLibro debe ser 080000"})
		return
	}

	err := svc.DescargarReporteCumplimientoService(perTributario, codLibro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Reporte de cumplimiento descargado exitosamente"})
}

// 59 Servicio Web Api consultar ajustes posteriores RCE
func ConsultarAjustesPosterioresRCE(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	periodoSeleccionado := c.Param("periodoSeleccionado")
	tipoInfo := c.Query("tipoInfo")

	if periodoSeleccionado == "" {
		c.JSON(400, gin.H{"error": "periodoSeleccionado es obligatorio"})
		return
	}

	if len(periodoSeleccionado) != 6 {
		c.JSON(400, gin.H{"error": "periodoSeleccionado debe tener formato YYYYMM"})
		return
	}

	if tipoInfo == "" {
		tipoInfo = "FV0621"
	}

	err := svc.ConsultarAjustesPosterioresRCEService(periodoSeleccionado, tipoInfo)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Ajustes posteriores RCE consultados exitosamente"})
}

func ListarComprobantesAjustesPost(c *gin.Context) {
	svc := services.New(SunatClient, getRuc(c))
	periodoSeleccionado := c.Param("periodoSeleccionado")
	page := c.Query("page")
	perPage := c.Query("perPage")

	if periodoSeleccionado == "" {
		c.JSON(400, gin.H{"error": "periodoSeleccionado es obligatorio"})
		return
	}

	if len(periodoSeleccionado) != 6 {
		c.JSON(400, gin.H{"error": "periodoSeleccionado debe tener formato YYYYMM"})
		return
	}

	if page == "" {
		page = "1"
	}

	if perPage == "" {
		perPage = "10"
	}

	err := svc.ListarComprobantesAjustesPostService(periodoSeleccionado, page, perPage)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Comprobantes de ajustes posteriores listados exitosamente"})
}
