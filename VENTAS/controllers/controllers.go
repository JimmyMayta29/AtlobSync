package controllers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/SwodLore/sire-api/VENTAS/models"
	"github.com/SwodLore/sire-api/VENTAS/services"
	"github.com/gin-gonic/gin"
)

// 2 Servicio Web Api consultar año y mes
func GetPeriodos(c *gin.Context) {
	codLibro := c.Param("codLibro")

	token := os.Getenv("TOKEN")

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Falta token en .env (TOKEN)"})
		return
	}

	periodos, err := services.ConsultarPeriodos(token, codLibro)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"periodos": periodos})
}

// 3 Servicio Web Api importar reemplazo de la propuesta
func ImportarReemplazoPropuesta(c *gin.Context) {
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	filename := c.GetHeader("filename")
	filetype := c.GetHeader("filetype")
	codOrigenEnvio := c.GetHeader("codOrigenEnvio")
	codProceso := c.GetHeader("codProceso")
	codTipoCorrelativo := c.GetHeader("codTipoCorrelativo")
	nomArchivoImportacion := c.GetHeader("nomArchivoImportacion")
	codLibro := c.GetHeader("codLibro")

	if filename == "" || filetype == "" || codOrigenEnvio == "" || codProceso == "" ||
		codTipoCorrelativo == "" || nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los headers metadata son requeridos"})
		return
	}

	err := services.ImportarReemplazoPropuestaSUNAT(perTributario, filename, filetype,
		codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Importación de reemplazo de propuesta exitosa"})
}

// 4 Servicio Web Api importar nuevos comprobantes propuesta
func ImportarNuevosComprobantes(c *gin.Context) {
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es requerido"})
		return
	}

	filename := c.GetHeader("filename")
	filetype := c.GetHeader("filetype")
	codOrigenEnvio := c.GetHeader("codOrigenEnvio")
	codProceso := c.GetHeader("codProceso")
	codTipoCorrelativo := c.GetHeader("codTipoCorrelativo")
	nomArchivoImportacion := c.GetHeader("nomArchivoImportacion")
	codLibro := c.GetHeader("codLibro")

	if filename == "" || filetype == "" || codOrigenEnvio == "" || codProceso == "" ||
		codTipoCorrelativo == "" || nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los headers metadata son requeridos"})
		return
	}

	err := services.ImportarNuevosComprobantesSUNAT(perTributario, filename, filetype,
		codOrigenEnvio, codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Importación de nuevos comprobantes exitosa"})
}

// 5 Servicio Web Api importar nuevos comprobantes preliminar
func ImportarComprobantesPreliminar(c *gin.Context) {
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	filename := c.PostForm("filename")
	filetype := c.PostForm("filetype")
	codOrigenEnvio := c.PostForm("codOrigenEnvio")
	codProceso := c.PostForm("codProceso")
	codTipoCorrelativo := c.PostForm("codTipoCorrelativo")
	nomArchivoImportacion := c.PostForm("nomArchivoImportacion")
	codLibro := c.PostForm("codLibro")

	if filename == "" || filetype == "" || codOrigenEnvio == "" ||
		codProceso == "" || codTipoCorrelativo == "" ||
		nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los parámetros son obligatorios"})
		return
	}

	ticket, err := services.ImportarComprobantesPreliminarSUNAT(
		perTributario, filename, filetype, codOrigenEnvio,
		codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":   true,
		"numTicket": ticket,
	})
}

// 6 Servicio Web Api importar ajustes posteriores
func ImportarAjustesPosteriores(c *gin.Context) {
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	filename := c.PostForm("filename")
	filetype := c.PostForm("filetype")
	codOrigenEnvio := c.PostForm("codOrigenEnvio")
	codProceso := c.PostForm("codProceso")
	codTipoCorrelativo := c.PostForm("codTipoCorrelativo")
	nomArchivoImportacion := c.PostForm("nomArchivoImportacion")
	codLibro := c.PostForm("codLibro")

	if filename == "" || filetype == "" || codOrigenEnvio == "" ||
		codProceso == "" || codTipoCorrelativo == "" ||
		nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los parámetros son obligatorios"})
		return
	}

	ticket, err := services.ImportarAjustesPosterioresSUNAT(
		perTributario, filename, filetype, codOrigenEnvio,
		codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":   true,
		"numTicket": ticket,
	})
}

// 7 Servicio Web Api importar ajustes posteriores de periodos anteriores
func ImportarAjustesPeriodosAnteriores(c *gin.Context) {
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	filename := c.PostForm("filename")
	filetype := c.PostForm("filetype")
	codOrigenEnvio := c.PostForm("codOrigenEnvio")
	codProceso := c.PostForm("codProceso")
	codTipoCorrelativo := c.PostForm("codTipoCorrelativo")
	nomArchivoImportacion := c.PostForm("nomArchivoImportacion")
	codLibro := c.PostForm("codLibro")

	if filename == "" || filetype == "" || codOrigenEnvio == "" ||
		codProceso == "" || codTipoCorrelativo == "" ||
		nomArchivoImportacion == "" || codLibro == "" {
		c.JSON(400, gin.H{"error": "Todos los parámetros son obligatorios"})
		return
	}

	ticket, err := services.ImportarAjustesPeriodosAnterioresSUNAT(
		perTributario, filename, filetype, codOrigenEnvio,
		codProceso, codTipoCorrelativo, nomArchivoImportacion, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":   true,
		"numTicket": ticket,
	})
}

// 8 Servicio Web Api aceptar propuesta del RVIE
func AceptarPropuestaRVIE(c *gin.Context) {
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	ticket, err := services.AceptarPropuestaRVIESUNAT(perTributario)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":   true,
		"numTicket": ticket,
	})
}

// 9 Servicio Web Api registrar preliminar
func RegistrarPreliminarController(c *gin.Context) {
	perTributario := c.Param("perTributario")
	if perTributario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro perTributario es obligatorio"})
		return
	}

	// Llamada al servicio
	err := services.RegistrarPreliminarSUNAT(perTributario)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Preliminar registrado correctamente"})
}

// 10 Servicio Web Api exclusión definitiva de notas de crédito y facturas
func ExclusionDefinitivaController(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codCar := c.Query("codCar")
	codSituacion := c.Query("codSituacion")

	// Validaciones básicas
	if perTributario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro perTributario es obligatorio"})
		return
	}

	if codCar == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro codCar es obligatorio"})
		return
	}

	if codSituacion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro codSituacion es obligatorio"})
		return
	}

	// Llamada al servicio
	err := services.ExclusionDefinitivaSUNAT(perTributario, codCar, codSituacion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comprobante excluido definitivamente"})
}

// 11 Servicio Web Api agregar tipo de cambio masivo
func TipoCambioMasivoController(c *gin.Context) {
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro perTributario es obligatorio"})
		return
	}

	var tipoCambios []models.TipoCambioRequest
	if err := c.ShouldBindJSON(&tipoCambios); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato de datos inválido: " + err.Error()})
		return
	}

	if len(tipoCambios) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Debe enviar al menos un registro de tipo de cambio"})
		return
	}

	// Llamada al servicio
	err := services.TipoCambioMasivoSUNAT(perTributario, tipoCambios)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Tipos de cambio actualizados correctamente",
		"registros": len(tipoCambios),
	})
}

// 12 Servicio Web Api editar tipo de cambio individual
func EditarTipoCambioController(c *gin.Context) {
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro perTributario es obligatorio"})
		return
	}

	var tipoCambio models.TipoCambioIndividualRequest
	if err := c.ShouldBindJSON(&tipoCambio); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato de datos inválido: " + err.Error()})
		return
	}

	// Llamada al servicio
	err := services.EditarTipoCambioSUNAT(perTributario, tipoCambio)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Tipo de cambio actualizado correctamente",
		"codCar":  tipoCambio.CodCar,
	})
}

// 13 Servicio Web Api eliminar comprobante propuesta
func EliminarComprobanteController(c *gin.Context) {
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro perTributario es obligatorio"})
		return
	}

	var comprobantes []models.EliminarComprobanteRequest
	if err := c.ShouldBindJSON(&comprobantes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato de datos inválido: " + err.Error()})
		return
	}

	if len(comprobantes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Debe enviar al menos un comprobante para eliminar"})
		return
	}

	// Llamada al servicio
	err := services.EliminarComprobanteSUNAT(perTributario, comprobantes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Comprobantes eliminados correctamente",
		"eliminados": len(comprobantes),
	})
}

// 14 Servicio Web Api eliminar comprobante preliminar
func EliminarComprobantePreliminar(c *gin.Context) {
	perTributario := strings.TrimSpace(c.Param("perTributario"))

	if perTributario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perTributario debe ser 6 dígitos YYYYMM"})
		return
	}

	for _, r := range perTributario {
		if r < '0' || r > '9' {
			c.JSON(http.StatusBadRequest, gin.H{"error": "perTributario debe contener solo dígitos"})
			return
		}
	}

	var comprobantes []models.ComprobanteEliminar
	if err := c.ShouldBindJSON(&comprobantes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON inválido: " + err.Error()})
		return
	}

	if len(comprobantes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Debe enviar al menos un comprobante"})
		return
	}

	for i, comp := range comprobantes {
		if comp.CodCar == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("codCar obligatorio en elemento %d", i+1)})
			return
		}

		if len(comp.CodCar) > 27 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("codCar máximo 27 dígitos en elemento %d", i+1)})
			return
		}

		for _, r := range comp.CodCar {
			if r < '0' || r > '9' {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("codCar debe ser numérico en elemento %d", i+1)})
				return
			}
		}

		if comp.CodTipoCDP == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("codTipoCDP obligatorio en elemento %d", i+1)})
			return
		}

		if comp.NumSerieCDP == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("numSerieCDP obligatorio en elemento %d", i+1)})
			return
		}

		if comp.NumCDP == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("numCDP obligatorio en elemento %d", i+1)})
			return
		}
	}

	err := services.EliminarComprobantePreliminarSUNAT(perTributario, comprobantes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Comprobantes eliminados exitosamente",
		"perTributario": perTributario,
		"cantidad":      len(comprobantes),
	})
}

// 15 Servicio Web Api eliminar reemplazo propuesta
func EliminarReemplazoPropuesta(c *gin.Context) {
	perTributario := strings.TrimSpace(c.Param("perTributario"))
	codLibro := strings.TrimSpace(c.Query("codLibro"))

	if perTributario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if len(perTributario) != 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perTributario debe ser 6 dígitos YYYYMM"})
		return
	}

	for _, r := range perTributario {
		if r < '0' || r > '9' {
			c.JSON(http.StatusBadRequest, gin.H{"error": "perTributario debe contener solo dígitos"})
			return
		}
	}

	if codLibro == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	err := services.EliminarReemplazoPropuestaSUNAT(perTributario, codLibro)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Reemplazo eliminado exitosamente",
		"perTributario": perTributario,
		"codLibro":      codLibro,
	})
}

// 16 Servicio Web Api consultar estado de envío de ticket
func ConsultarEstadoTicket(c *gin.Context) {
	perIni := strings.TrimSpace(c.Query("perIni"))
	perFin := strings.TrimSpace(c.Query("perFin"))
	page := strings.TrimSpace(c.Query("page"))
	perPage := strings.TrimSpace(c.Query("perPage"))
	numTicket := strings.TrimSpace(c.Query("numTicket"))

	if perIni == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perIni es obligatorio"})
		return
	}

	if len(perIni) != 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perIni debe ser 6 dígitos YYYYMM"})
		return
	}

	for _, r := range perIni {
		if r < '0' || r > '9' {
			c.JSON(http.StatusBadRequest, gin.H{"error": "perIni debe contener solo dígitos"})
			return
		}
	}

	if perFin == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perFin es obligatorio"})
		return
	}

	if len(perFin) != 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perFin debe ser 6 dígitos YYYYMM"})
		return
	}

	for _, r := range perFin {
		if r < '0' || r > '9' {
			c.JSON(http.StatusBadRequest, gin.H{"error": "perFin debe contener solo dígitos"})
			return
		}
	}

	if page == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page es obligatorio"})
		return
	}

	pageInt := 0
	for _, r := range page {
		if r < '0' || r > '9' {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page debe ser numérico"})
			return
		}
		pageInt = pageInt*10 + int(r-'0')
	}

	if pageInt <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page debe ser mayor a cero"})
		return
	}

	if perPage == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perPage es obligatorio"})
		return
	}

	perPageInt := 0
	for _, r := range perPage {
		if r < '0' || r > '9' {
			c.JSON(http.StatusBadRequest, gin.H{"error": "perPage debe ser numérico"})
			return
		}
		perPageInt = perPageInt*10 + int(r-'0')
	}

	if perPageInt <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perPage debe ser mayor a cero"})
		return
	}

	respuesta, err := services.ConsultarEstadoTicketSUNAT(perIni, perFin, page, perPage, numTicket)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, respuesta)
}

// 17 Servicio Web Api descargar archivo
func DescargarArchivo(c *gin.Context) {
	nomArchivoReporte := c.Query("nomArchivoReporte")
	codTipoArchivoReporte := c.Query("codTipoArchivoReporte")
	codLibro := c.Query("codLibro")

	if nomArchivoReporte == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nomArchivoReporte es requerido"})
		return
	}

	if codLibro == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "codLibro es requerido"})
		return
	}

	fileBuffer, err := services.DescargarArchivoSUNAT(nomArchivoReporte, codTipoArchivoReporte, codLibro)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename="+nomArchivoReporte)
	c.Data(http.StatusOK, "application/zip", fileBuffer)
}

// 18 Servicio Web Api descargar propuesta
func DescargarPropuesta(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codTipoArchivo := c.Query("codTipoArchivo")
	mtoTotalDesde := c.Query("mtoTotalDesde")
	mtoTotalHasta := c.Query("mtoTotalHasta")
	fecDocumentoDesde := c.Query("fecDocumentoDesde")
	fecDocumentoHasta := c.Query("fecDocumentoHasta")
	numRucAdquiriente := c.Query("numRucAdquiriente")
	numCarSunat := c.Query("numCarSunat")
	codTipoCDP := c.Query("codTipoCDP")
	codTipoInconsistencia := c.Query("codTipoInconsistencia")

	if perTributario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perTributario es requerido"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "codTipoArchivo es requerido"})
		return
	}

	result, err := services.DescargarPropuestaSUNAT(perTributario, codTipoArchivo, mtoTotalDesde, mtoTotalHasta,
		fecDocumentoDesde, fecDocumentoHasta, numRucAdquiriente, numCarSunat, codTipoCDP, codTipoInconsistencia)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// 19 Servicio Web Api descargar no incluidos
func DescargarNoIncluidos(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codTipoArchivo := c.Query("codTipoArchivo")
	mtoTotalDesde := c.Query("mtoTotalDesde")
	mtoTotalHasta := c.Query("mtoTotalHasta")
	fecDocumentoDesde := c.Query("fecDocumentoDesde")
	fecDocumentoHasta := c.Query("fecDocumentoHasta")
	numRucAdquiriente := c.Query("numRucAdquiriente")
	numCarSunat := c.Query("numCarSunat")
	codTipoCDP := c.Query("codTipoCDP")
	codTipoInconsistencia := c.Query("codTipoInconsistencia")
	codOrigenEnvio := c.Query("codOrigenEnvio")

	if perTributario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perTributario es requerido"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "codTipoArchivo es requerido"})
		return
	}

	result, err := services.DescargarNoIncluidosSUNAT(perTributario, codTipoArchivo, mtoTotalDesde, mtoTotalHasta,
		fecDocumentoDesde, fecDocumentoHasta, numRucAdquiriente, numCarSunat, codTipoCDP, codTipoInconsistencia, codOrigenEnvio)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// 20 Servicio Web Api descargar resumen
func DescargarResumen(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codTipoResumen := c.Param("codTipoResumen")
	codTipoArchivo := c.Param("codTipoArchivo")
	codLibro := c.Query("codLibro")

	if perTributario == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perTributario es requerido"})
		return
	}

	if codTipoResumen == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "codTipoResumen es requerido"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "codTipoArchivo es requerido"})
		return
	}

	if codLibro == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "codLibro es requerido"})
		return
	}

	fileBuffer, err := services.DescargarResumenSUNAT(perTributario, codTipoResumen, codTipoArchivo, codLibro)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("resumen_%s_%s_%s.zip", perTributario, codTipoResumen, codTipoArchivo)
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/zip", fileBuffer)
}

// 21 Servicio Web Api descargar resumen inconsistencias
func DescargarResumenInconsistencias(c *gin.Context) {
	perPeriodoTributario := c.Param("perPeriodoTributario")
	codTipoResumen := c.Query("codTipoResumen")
	codLibro := c.Query("codLibro")

	if perPeriodoTributario == "" {
		c.JSON(400, gin.H{"error": "perPeriodoTributario es obligatorio"})
		return
	}

	if codTipoResumen == "" {
		c.JSON(400, gin.H{"error": "codTipoResumen es obligatorio"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	resumen, err := services.DescargarResumenInconsistenciasSUNAT(perPeriodoTributario, codTipoResumen, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"data":    resumen,
	})
}

// 22 Servicio Web Api exportar preliminar de registro de Ventas
func ExportarPreliminarVentas(c *gin.Context) {
	perTributario := c.Param("perTributario")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	codTipoArchivo := c.Query("codTipoArchivo")
	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	codOrigenEnvio := c.DefaultQuery("codOrigenEnvio", "2")
	codLibro := c.DefaultQuery("codLibro", "140000")
	mtoTotalDesde := c.Query("mtoTotalDesde")
	mtoTotalHasta := c.Query("mtoTotalHasta")
	fecDocumentoDesde := c.Query("fecDocumentoDesde")
	fecDocumentoHasta := c.Query("fecDocumentoHasta")
	numRucAdquiriente := c.Query("numRucAdquiriente")
	numCarSunat := c.Query("numCarSunat")
	codTipoCDP := c.Query("codTipoCDP")
	codTipoInconsistencia := c.Query("codTipoInconsistencia")

	ticket, err := services.ExportarPreliminarVentasSUNAT(
		perTributario, codTipoArchivo, codOrigenEnvio, codLibro, mtoTotalDesde, mtoTotalHasta,
		fecDocumentoDesde, fecDocumentoHasta, numRucAdquiriente,
		numCarSunat, codTipoCDP, codTipoInconsistencia)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":   true,
		"numTicket": ticket,
	})
}

// 23 Servicio Web Api descargar reporte de casillas
func DescargarReporteCasillas(c *gin.Context) {
	perTributario := c.Param("perTributario")
	tipoReporte := c.Param("tipoReporte")
	tipoDescarga := c.Param("tipoDescarga")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if tipoReporte == "" {
		c.JSON(400, gin.H{"error": "tipoReporte es obligatorio"})
		return
	}

	if tipoDescarga == "" {
		c.JSON(400, gin.H{"error": "tipoDescarga es obligatorio"})
		return
	}

	ticket, err := services.DescargarReporteCasillasSUNAT(perTributario, tipoReporte, tipoDescarga)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":   true,
		"numTicket": ticket,
	})
}

// 24 Servicio Web Api descargar inconsistencias en registros preliminar registrado
func DescargarInconsistenciasPreliminar(c *gin.Context) {
	perTributario := c.Param("perTributario")
	numCas := c.Param("numCas")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if numCas == "" {
		c.JSON(400, gin.H{"error": "numCas es obligatorio"})
		return
	}

	cntlimite := c.Query("cntlimite")
	if cntlimite == "" {
		c.JSON(400, gin.H{"error": "cntlimite es obligatorio"})
		return
	}

	mtoNoSustentado := c.Query("mtoNoSustentado")
	desMtoNoSustentado := c.Query("desMtoNoSustentado")

	buffer, err := services.DescargarInconsistenciasPreliminarSUNAT(
		perTributario, numCas, cntlimite, mtoNoSustentado, desMtoNoSustentado)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=inconsistencias_%s_%s.txt", perTributario, numCas))
	c.Data(200, "application/octet-stream", buffer)
}

// 25 Servicio Web Api descargar inconsistencias por comprobante pago
func DescargarInconsistenciasComprobantes(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codTipoArchivo := c.Query("codTipoArchivo")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codTipoArchivo == "" {
		c.JSON(400, gin.H{"error": "codTipoArchivo es obligatorio"})
		return
	}

	fecEmisionIni := c.Query("fecEmisionIni")
	fecEmisionFin := c.Query("fecEmisionFin")
	codInconsistencia := c.Query("codInconsistencia")
	numDocAdquiriente := c.Query("numDocAdquiriente")
	codTipoCDP := c.Query("codTipoCDP")
	numSerieCDP := c.Query("numSerieCDP")
	numCDP := c.Query("numCDP")
	s := c.Query("s")
	ord := c.Query("ord")

	result, err := services.DescargarInconsistenciasComprobantesSUNAT(
		perTributario, codTipoArchivo, fecEmisionIni, fecEmisionFin,
		codInconsistencia, numDocAdquiriente, codTipoCDP, numSerieCDP,
		numCDP, s, ord)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 26 Servicio Web Api descargar constancia de recepción
func DescargarConstanciaRecepcion(c *gin.Context) {
	nomArchivo := c.Query("nomArchivo")

	if nomArchivo == "" {
		c.JSON(400, gin.H{"error": "nomArchivo es obligatorio"})
		return
	}

	result, err := services.DescargarConstanciaRecepcionSUNAT(nomArchivo)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 27 Servicio Web Api descargar RVIE por periodo
func DescargarRVIEPorPeriodo(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codOrigenEnvio := c.Query("codOrigenEnvio")
	codTipoArchivo := c.Query("codTipoArchivo")
	codProceso := c.Query("codProceso")
	codLibro := c.Query("codLibro")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es obligatorio"})
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

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	result, err := services.DescargarRVIEPorPeriodoSUNAT(
		perTributario, codOrigenEnvio, codTipoArchivo, codProceso, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 28 Servicio Web Api descargar reporte consolidado por periodo
func DescargarReporteConsolidadoPorPeriodo(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codOrigenEnvio := c.Query("codOrigenEnvio")
	codTipoArchivo := c.Query("codTipoArchivo")
	codProceso := c.Query("codProceso")
	codLibro := c.Query("codLibro")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es obligatorio"})
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

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	result, err := services.DescargarRVIEPorPeriodoSUNAT(
		perTributario, codOrigenEnvio, codTipoArchivo, codProceso, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 29 Servicio Web Api descargar ajustes posteriores
func DescargarAjustesPosteriores(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codOrigenEnvio := c.Query("codOrigenEnvio")
	codTipoArchivo := c.Query("codTipoArchivo")
	codProceso := c.Query("codProceso")
	codLibro := c.Query("codLibro")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es obligatorio"})
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

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	result, err := services.DescargarRVIEPorPeriodoSUNAT(
		perTributario, codOrigenEnvio, codTipoArchivo, codProceso, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 30 Servicio Web Api descargar ajustes posteriores de periodos anteriores
func DescargarAjustesPosterioresPeriodosAnteriores(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codOrigenEnvio := c.Query("codOrigenEnvio")
	codTipoArchivo := c.Query("codTipoArchivo")
	codProceso := c.Query("codProceso")
	codLibro := c.Query("codLibro")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es obligatorio"})
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

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	result, err := services.DescargarRVIEPorPeriodoSUNAT(
		perTributario, codOrigenEnvio, codTipoArchivo, codProceso, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 31 Servicio Web Api descargar reporte inconsistencias por periodo
func DescargarInconsistenciasPorPeriodo(c *gin.Context) {
	codRegistroLibro := c.Param("codRegistroLibro")
	codOrigenEnvio := c.Query("codOrigenEnvio")

	if codRegistroLibro == "" {
		c.JSON(400, gin.H{"error": "codRegistroLibro es obligatorio"})
		return
	}

	if codOrigenEnvio == "" {
		c.JSON(400, gin.H{"error": "codOrigenEnvio es obligatorio"})
		return
	}

	result, err := services.DescargarInconsistenciasPorPeriodoSUNAT(codRegistroLibro, codOrigenEnvio)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 32 Servicio Web Api descargar reporte CAR
func DescargarReporteCAR(c *gin.Context) {
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

	result, err := services.DescargarReporteCARSUNAT(perTributario, codOrigenEnvio, codLibro, codFase)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 33 Servicio Web Api descargar reporte estadístico
func DescargarReporteEstadistico(c *gin.Context) {
	perTributario := c.Param("perTributario")
	numRuc := c.Query("numRuc")
	codTipoArchivo := c.Query("codTipoArchivo")
	codTipoReporte := c.Query("codTipoReporte")
	codLibro := c.Query("codLibro")
	fechaIni := c.Query("fechaIni")
	fechaFin := c.Query("fechaFin")
	codTipoCDP := c.Query("codTipoCDP")
	codTipoDocIdentidadAdquiriente := c.Query("codTipoDocIdentidadAdquiriente")
	numDocIdentidadAdquiriente := c.Query("numDocIdentidadAdquiriente")

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

	result, err := services.DescargarReporteEstadisticoSUNAT(
		perTributario, numRuc, codTipoArchivo, codTipoReporte, codLibro,
		fechaIni, fechaFin, codTipoCDP, codTipoDocIdentidadAdquiriente, numDocIdentidadAdquiriente)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 34 Servicio Web Api descargar reporte de cumplimiento
func DescargarReporteCumplimiento(c *gin.Context) {
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

	result, err := services.DescargarReporteCumplimientoSUNAT(perTributario, codLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 35 Servicio Web Api reporte de exportadores
func ReporteExportadores(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codRegistroLibro := c.Query("codRegistroLibro")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codRegistroLibro == "" {
		c.JSON(400, gin.H{"error": "codRegistroLibro es obligatorio"})
		return
	}

	result, err := services.ReporteExportadoresSUNAT(perTributario, codRegistroLibro)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}

// 36 Servicio Web Api eliminar preliminar registrado
func EliminarPreliminarController(c *gin.Context) {
	perTributario := c.Param("perTributario")
	codLibro := c.Query("codLibro")

	if perTributario == "" {
		c.JSON(400, gin.H{"error": "perTributario es obligatorio"})
		return
	}

	if codLibro == "" {
		c.JSON(400, gin.H{"error": "codLibro es obligatorio"})
		return
	}

	var body struct {
		Id              string `json:"id"`
		CodTipoRegistro string `json:"codTipoRegistro"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "Error en el formato del body"})
		return
	}

	if body.CodTipoRegistro == "" {
		c.JSON(400, gin.H{"error": "codTipoRegistro es obligatorio"})
		return
	}

	err := services.EliminarPreliminarService(perTributario, codLibro, body.Id, body.CodTipoRegistro)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Preliminar eliminado exitosamente"})
}
