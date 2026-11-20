package sire

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SwodLore/sire-api/internal/sunat"
	"github.com/gin-gonic/gin"
)

type Controller struct {
	service *Service
}

func NewController(service *Service) *Controller {
	return &Controller{service: service}
}

func (ctrl *Controller) RegisterRoutes(apiGroup *gin.RouterGroup, empresaAuth gin.HandlerFunc) {
	// Endpoints globales directos (usados por la interfaz web y clientes)
	apiGroup.GET("/descargas", ctrl.ListarDescargasGlobal)
	apiGroup.GET("/descargas/:id/archivo", ctrl.DescargarArchivoDirecto)
	apiGroup.GET("/descargas/:id/propuesta-txt", ctrl.AnalizarPropuestaTXT)
	apiGroup.POST("/descargas/:id/reintentar", ctrl.ReintentarDescarga)
	apiGroup.POST("/sire/propuesta/exportar", ctrl.SolicitarExportacionGlobal)
	apiGroup.GET("/sire/propuesta/detalle", ctrl.ConsultarDetallePropuestaGlobal)
	apiGroup.GET("/sire/tickets/:ruc/:ticket", ctrl.ConsultarTicket)

	// Endpoints protegidos por RUC de empresa
	sireGroup := apiGroup.Group("/empresas/:ruc/sire", empresaAuth)
	{
		// 1. Exportar propuesta (flujo asíncrono con ticket)
		sireGroup.POST("/exportar/:libro/:periodo", ctrl.SolicitarExportacion)

		// 2. Consultar detalle paginado de la propuesta con filtros
		sireGroup.GET("/propuesta/:libro/:periodo/detalle", ctrl.ConsultarDetallePropuesta)

		// 3. Listar historial de descargas y tickets
		sireGroup.GET("/descargas", ctrl.ListarDescargas)

		// 4. Consultar estado de una descarga específica por ID
		sireGroup.GET("/descargas/:id", ctrl.ObtenerDescarga)

		// 5. Descargar archivo físico por ID de base de datos
		sireGroup.GET("/descargas/:id/archivo", ctrl.DescargarArchivoEmpresa)

		// 6. Descargar Constancia de Recepción (CDR) del libro SIRE (RVIE o RCE)
		sireGroup.GET("/constancia", ctrl.DescargarConstanciaRecepcion)
	}
}

func (ctrl *Controller) SolicitarExportacion(c *gin.Context) {
	ruc := c.GetString("ruc")
	libro := c.Param("libro")
	periodo := c.Param("periodo")

	var filtros FiltrosExportacion
	_ = c.ShouldBindJSON(&filtros)

	if filtros.CodTipoArchivo == "" {
		filtros.CodTipoArchivo = c.DefaultQuery("codTipoArchivo", "0")
	}
	if filtros.CodOrigenEnvio == "" || filtros.CodOrigenEnvio == "1" {
		filtros.CodOrigenEnvio = c.DefaultQuery("codOrigenEnvio", "2")
		if filtros.CodOrigenEnvio == "1" {
			filtros.CodOrigenEnvio = "2"
		}
	}

	descarga, err := ctrl.service.SolicitarExportacion(ruc, libro, periodo, filtros)
	if err != nil {
		sunat.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"mensaje": fmt.Sprintf("Solicitud de exportación para %s (%s) aceptada con ticket %s. El archivo se descargará automáticamente en segundo plano.", libro, periodo, descarga.Ticket),
		"data":    descarga,
	})
}

func (ctrl *Controller) ConsultarDetallePropuesta(c *gin.Context) {
	ruc := c.GetString("ruc")
	libro := c.Param("libro")
	periodo := c.Param("periodo")

	var filtros FiltrosPropuestaDetalle
	if err := c.ShouldBindQuery(&filtros); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Parámetros de consulta inválidos"})
		return
	}

	detalle, err := ctrl.service.ConsultarDetallePropuesta(ruc, libro, periodo, filtros)
	if err != nil {
		sunat.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    detalle,
	})
}

func (ctrl *Controller) ListarDescargas(c *gin.Context) {
	ruc := c.GetString("ruc")
	libro := c.Query("libro")
	periodo := c.Query("periodo")

	descargas, err := ctrl.service.ListarDescargas(ruc, libro, periodo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    descargas,
		"total":   len(descargas),
	})
}

func (ctrl *Controller) ObtenerDescarga(c *gin.Context) {
	ruc := c.GetString("ruc")
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ID de descarga inválido"})
		return
	}

	descarga, err := ctrl.service.ObtenerDescarga(id, ruc)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    descarga,
	})
}

func (ctrl *Controller) DescargarArchivoEmpresa(c *gin.Context) {
	ruc := c.GetString("ruc")
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ID de descarga inválido"})
		return
	}

	descarga, filePath, err := ctrl.service.ObtenerArchivoParaDescarga(id, ruc)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", descarga.NombreArchivo))
	c.Header("Content-Type", detectDownloadContentType(descarga.NombreArchivo))
	c.File(filePath)
}

func (ctrl *Controller) DescargarConstanciaRecepcion(c *gin.Context) {
	ruc := c.GetString("ruc")
	libro := c.DefaultQuery("libro", "VENTAS")
	nomArchivo := strings.TrimSpace(c.Query("nomArchivo"))
	if nomArchivo == "" {
		nomArchivo = strings.TrimSpace(c.Query("nomConstanciaRecepcion"))
	}
	if nomArchivo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Debe indicar el nombre de archivo de la constancia de recepción (nomArchivo)",
			"detalle":     "Ejemplo: nombre exacto de constancia devuelto por SUNAT al presentar el libro SIRE",
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	data, fileName, contentType, err := ctrl.service.DescargarConstanciaRecepcion(ruc, libro, nomArchivo)
	if err != nil {
		sunat.RespondWithError(c, err)
		return
	}

	if contentType == "" || contentType == "application/octet-stream" {
		contentType = detectDownloadContentType(fileName)
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Data(http.StatusOK, contentType, data)
}

func detectDownloadContentType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".zip":
		return "application/zip"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".csv":
		return "text/csv; charset=utf-8"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	case ".pdf":
		return "application/pdf"
	case ".xml":
		return "application/xml"
	default:
		return "application/octet-stream"
	}
}

func (ctrl *Controller) DescargarArchivoDirecto(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ID de descarga inválido"})
		return
	}

	// Sin validación de RUC en contexto, solo verifica existencia e integridad del registro en BD
	descarga, filePath, err := ctrl.service.ObtenerArchivoParaDescarga(id, "")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":     false,
			"error":       "Archivo no encontrado",
			"detalle":     err.Error(),
			"codigo_http": http.StatusNotFound,
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", descarga.NombreArchivo))
	c.Header("Content-Type", detectDownloadContentType(descarga.NombreArchivo))
	c.File(filePath)
}

func (ctrl *Controller) ReintentarDescarga(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ID de descarga inválido"})
		return
	}

	descarga, err := ctrl.service.ReintentarDescarga(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"mensaje": fmt.Sprintf("Ticket %s reencolado para descarga automática.", descarga.Ticket),
		"data":    descarga,
	})
}

func (ctrl *Controller) ListarDescargasGlobal(c *gin.Context) {
	ruc := c.Query("ruc")
	libro := c.Query("libro")
	periodo := c.Query("periodo")

	descargas, err := ctrl.service.ListarDescargas(ruc, libro, periodo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":     false,
			"error":       "Error al listar historial de descargas",
			"detalle":     err.Error(),
			"codigo_http": http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    descargas,
		"total":   len(descargas),
	})
}

type SolicitudExportarGlobal struct {
	RUC            string `json:"ruc"`
	CodLibro       string `json:"cod_libro"`
	Libro          string `json:"libro"`
	Periodo        string `json:"periodo"`
	CodTipoArchivo string `json:"cod_tipo_archivo"`
	CodOrigenEnvio string `json:"cod_origen_envio"`
}

func (ctrl *Controller) SolicitarExportacionGlobal(c *gin.Context) {
	var req SolicitudExportarGlobal
	_ = c.ShouldBindJSON(&req)

	if req.RUC == "" {
		req.RUC = c.Query("ruc")
	}
	if req.Periodo == "" {
		req.Periodo = c.Query("periodo")
	}
	libro := req.CodLibro
	if libro == "" {
		libro = req.Libro
	}
	if libro == "" {
		libro = c.Query("codLibro")
	}

	if req.RUC == "" || req.Periodo == "" || libro == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Parámetros incompletos para exportación",
			"detalle":     "Debe proporcionar ruc, periodo y cod_libro en el cuerpo JSON o query params",
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	filtros := FiltrosExportacion{
		CodTipoArchivo: req.CodTipoArchivo,
		CodOrigenEnvio: req.CodOrigenEnvio,
	}
	if filtros.CodTipoArchivo == "" {
		filtros.CodTipoArchivo = c.DefaultQuery("codTipoArchivo", "0")
	}
	if filtros.CodOrigenEnvio == "" || filtros.CodOrigenEnvio == "1" {
		filtros.CodOrigenEnvio = c.DefaultQuery("codOrigenEnvio", "2")
		if filtros.CodOrigenEnvio == "1" {
			filtros.CodOrigenEnvio = "2"
		}
	}

	descarga, err := ctrl.service.SolicitarExportacion(req.RUC, libro, req.Periodo, filtros)
	if err != nil {
		sunat.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"ticket":  descarga.Ticket,
		"id":      descarga.ID,
		"mensaje": fmt.Sprintf("Solicitud de exportación para %s (%s) aceptada con ticket %s.", libro, req.Periodo, descarga.Ticket),
		"data":    descarga,
	})
}

func (ctrl *Controller) ConsultarDetallePropuestaGlobal(c *gin.Context) {
	ruc := c.Query("ruc")
	libro := c.DefaultQuery("codLibro", c.Query("libro"))
	periodo := c.Query("periodo")

	if ruc == "" || libro == "" || periodo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Parámetros incompletos",
			"detalle":     "Debe indicar ruc, codLibro y periodo en los parámetros de consulta",
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	var filtros FiltrosPropuestaDetalle
	if err := c.ShouldBindQuery(&filtros); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Parámetros de consulta inválidos",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	detalle, err := ctrl.service.ConsultarDetallePropuesta(ruc, libro, periodo, filtros)
	if err != nil {
		sunat.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, detalle)
}

func (ctrl *Controller) ConsultarTicket(c *gin.Context) {
	ruc := c.Param("ruc")
	ticket := c.Param("ticket")

	descarga, err := ctrl.service.ObtenerDescargaPorTicket(ticket, ruc)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":     false,
			"error":       "Ticket no encontrado",
			"detalle":     fmt.Sprintf("No se encontró el ticket %s para la empresa %s", ticket, ruc),
			"codigo_http": http.StatusNotFound,
		})
		return
	}

	c.JSON(http.StatusOK, descarga)
}

// AnalizarPropuestaTXT analiza en memoria el archivo TXT oficial dentro del ZIP de una descarga completada
// y retorna resumen tributario + filas paginadas (sin exponer datos sensibles en errores ni duplicar sincronización CPE).
func (ctrl *Controller) AnalizarPropuestaTXT(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"codigo":      "PARAMETRO_INVALIDO",
			"error":       "ID de descarga inválido",
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	limit := DefaultProposalPageLimit
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		v, convErr := strconv.Atoi(rawLimit)
		if convErr != nil || v <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success":     false,
				"codigo":      "PARAMETRO_INVALIDO",
				"error":       "Parámetro limit inválido",
				"detalle":     fmt.Sprintf("limit debe ser un entero positivo (máximo %d)", MaxProposalPageLimit),
				"codigo_http": http.StatusBadRequest,
			})
			return
		}
		if v > MaxProposalPageLimit {
			v = MaxProposalPageLimit
		}
		limit = v
	}

	offset := 0
	if rawOffset := strings.TrimSpace(c.Query("offset")); rawOffset != "" {
		v, convErr := strconv.Atoi(rawOffset)
		if convErr != nil || v < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success":     false,
				"codigo":      "PARAMETRO_INVALIDO",
				"error":       "Parámetro offset inválido",
				"detalle":     "offset debe ser un entero mayor o igual a 0",
				"codigo_http": http.StatusBadRequest,
			})
			return
		}
		offset = v
	}

	estadoFilter := EstadoMarcaActivo
	if rawEstado := strings.TrimSpace(c.Query("estado")); rawEstado != "" {
		norm, ok := normalizeEstadoFilter(rawEstado)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"success":     false,
				"codigo":      "PARAMETRO_INVALIDO",
				"error":       "Parámetro estado inválido",
				"detalle":     "estado debe ser 'activo', 'anulado' o 'todos'",
				"codigo_http": http.StatusBadRequest,
			})
			return
		}
		estadoFilter = norm
	}

	result, err := ctrl.service.AnalizarPropuestaTXT(id, ProposalQueryOptions{
		Limit:  limit,
		Offset: offset,
		Estado: estadoFilter,
	})
	if err != nil {
		var pErr *ProposalParseError
		if errors.As(err, &pErr) {
			status := pErr.HTTPStatus
			if status == 0 {
				status = http.StatusUnprocessableEntity
			}
			body := gin.H{
				"success":     false,
				"codigo":      pErr.Code,
				"error":       pErr.Message,
				"codigo_http": status,
			}
			if pErr.Detail != "" {
				body["detalle"] = pErr.Detail
			}
			if pErr.FormatInfo != nil {
				body["detalles"] = pErr.FormatInfo
			}
			c.JSON(status, body)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":     false,
			"codigo":      "ERROR_INTERNO",
			"error":       "Error al analizar el archivo de propuesta",
			"codigo_http": http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"resumen":    result.Resumen,
		"paginacion": result.Paginacion,
		"filas":      result.Filas,
		"anulados":   result.Anulados,
	})
}
