package cpe

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	service *Service
}

func NewController(service *Service) *Controller {
	return &Controller{service: service}
}

// RegisterRoutes registra los endpoints del módulo de Comprobantes bajo /api/empresas/:ruc/cpe
func (ctrl *Controller) RegisterRoutes(apiGroup *gin.RouterGroup, empresaAuth gin.HandlerFunc) {
	g := apiGroup.Group("/empresas/:ruc/cpe", empresaAuth)
	{
		g.GET("", ctrl.List)
		g.GET("/estado-sol", ctrl.GetSolStatus)
		g.POST("/probar-sol", ctrl.TestSolSession)
		g.POST("/sincronizar-sire", ctrl.SyncFromSIRE)
		g.POST("/manual", ctrl.RegisterManual)
		g.POST("/importar", ctrl.ImportFile)

		// Descarga Masiva SUNAT SOL (XML / PDF / CDR / JSON + ZIP consolidado) y exportación de ítems
		g.POST("/descarga-masiva", ctrl.StartBulkDownload)
		g.GET("/descarga-masiva", ctrl.ListBulkJobs)
		g.GET("/descarga-masiva/:job_id", ctrl.GetBulkJob)
		g.DELETE("/descarga-masiva/:job_id", ctrl.CancelBulkJob)
		g.GET("/descarga-masiva/:job_id/zip", ctrl.DownloadBulkZip)
		g.GET("/exportar-items", ctrl.ExportItemsCSV)

		// Acciones por comprobante individual
		g.POST("/:id/consultar-cdr", ctrl.ConsultarCDR)
		g.POST("/:id/descargar-sunat", ctrl.DownloadSingleFromSUNAT)
		g.GET("/:id/detalle", ctrl.GetDetalle)
		g.GET("/:id/pdf", ctrl.DownloadPDF)
		g.GET("/:id/xml", ctrl.DownloadXML)
		g.GET("/:id/cdr", ctrl.DownloadCDR)
		g.GET("/:id/json", ctrl.DownloadJSON)
	}
}

func (ctrl *Controller) GetSolStatus(c *gin.Context) {
	ruc := c.GetString("ruc")
	st := ctrl.service.GetSolStatus(ruc)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    st,
	})
}

func (ctrl *Controller) TestSolSession(c *gin.Context) {
	ruc := c.GetString("ruc")
	var req struct {
		ForceRefresh *bool `json:"force_refresh"`
	}
	_ = c.ShouldBindJSON(&req)
	forceRefresh := true
	if req.ForceRefresh != nil {
		forceRefresh = *req.ForceRefresh
	}

	st, err := ctrl.service.ProbarSesionSOL(c.Request.Context(), ruc, forceRefresh)
	if err != nil {
		status, errCode, errTitle := classifyCPEError(err, http.StatusBadGateway, "No se pudo validar la sesión Portal SOL para comprobantes CPE")
		c.JSON(status, gin.H{
			"success":      false,
			"error":        errTitle,
			"detalle":      err.Error(),
			"codigo_error": errCode,
			"data":         st,
			"codigo_http":  status,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": "Sesión Portal SOL (Consulta CPE) verificada correctamente. Ya puede descargar PDF, XML y CDR.",
		"data":    st,
	})
}

func (ctrl *Controller) List(c *gin.Context) {
	ruc := c.GetString("ruc")
	origen := c.Query("origen")
	periodo := c.Query("periodo")
	tipoCP := c.Query("tipo_cp")
	search := c.Query("q")
	soloVigentes := c.Query("incluir_historicos") != "1"

	items, err := ctrl.service.ListFiltered(ruc, origen, periodo, tipoCP, search, soloVigentes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":     false,
			"error":       "Error al listar comprobantes",
			"detalle":     err.Error(),
			"codigo_http": http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
		"total":   len(items),
	})
}

func (ctrl *Controller) SyncFromSIRE(c *gin.Context) {
	ruc := c.GetString("ruc")
	var req struct {
		DescargaID int64 `json:"descarga_id"`
	}
	_ = c.ShouldBindJSON(&req)

	res, err := ctrl.service.SyncFromSIREDownloads(ruc, req.DescargaID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Error sincronizando propuestas SIRE",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": fmt.Sprintf("Sincronización completada: %d archivo(s) analizado(s), %d comprobante(s) indexado(s).", res.ArchivosProcesados, res.ComprobantesNuevos),
		"data":    res,
	})
}

func (ctrl *Controller) RegisterManual(c *gin.Context) {
	ruc := c.GetString("ruc")
	var req ManualCPERequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Datos del comprobante incompletos (tipo_cp, serie y numero son obligatorios)",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	res, err := ctrl.service.RegisterManual(ruc, &req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success":     false,
			"error":       err.Error(),
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadGateway,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": "Comprobante registrado correctamente",
		"data":    res,
	})
}

func (ctrl *Controller) ImportFile(c *gin.Context) {
	ruc := c.GetString("ruc")
	origen := c.DefaultPostForm("origen", "COMPRAS")
	targetIDStr := c.PostForm("cpe_id")
	var targetID int64
	if targetIDStr != "" {
		targetID, _ = strconv.ParseInt(targetIDStr, 10, 64)
	}

	fileHeader, err := c.FormFile("archivo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Debe adjuntar un archivo en el campo 'archivo' (.xml, .pdf o .zip)",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	// Límite razonable: 30 MB
	if fileHeader.Size > 30*1024*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"success":     false,
			"error":       "El archivo excede el límite permitido de 30 MB",
			"detalle":     "Divida el lote en archivos más pequeños",
			"codigo_http": http.StatusRequestEntityTooLarge,
		})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":     false,
			"error":       "No se pudo leer el archivo subido",
			"detalle":     err.Error(),
			"codigo_http": http.StatusInternalServerError,
		})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":     false,
			"error":       "Error leyendo contenido del archivo",
			"detalle":     err.Error(),
			"codigo_http": http.StatusInternalServerError,
		})
		return
	}

	res, err := ctrl.service.ImportUploadedFile(ruc, origen, fileHeader.Filename, data, targetID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       err.Error(),
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": fmt.Sprintf("Importación exitosa: %d archivo(s) guardado(s).", res.ArchivosGuardados),
		"data":    res,
	})
}

func (ctrl *Controller) ConsultarCDR(c *gin.Context) {
	ruc := c.GetString("ruc")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "ID de comprobante inválido",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	res, err := ctrl.service.ConsultarYGuardarCDR(id, ruc)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success":     false,
			"error":       "No se pudo obtener el CDR desde SUNAT SOAP",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadGateway,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": fmt.Sprintf("Consulta CDR SUNAT completada (código %s): %s", res.StatusCode, res.StatusMessage),
		"data":    res,
	})
}

func (ctrl *Controller) DownloadPDF(c *gin.Context) {
	ctrl.serveDownload(c, "pdf")
}

func (ctrl *Controller) DownloadXML(c *gin.Context) {
	ctrl.serveDownload(c, "xml")
}

func (ctrl *Controller) DownloadCDR(c *gin.Context) {
	ctrl.serveDownload(c, "cdr")
}

func (ctrl *Controller) DownloadJSON(c *gin.Context) {
	ctrl.serveDownload(c, "json")
}

func (ctrl *Controller) GetDetalle(c *gin.Context) {
	ruc := c.GetString("ruc")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "ID de comprobante inválido",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	autoSync := c.Query("sync") != "0"
	data, err := ctrl.service.GetDetalleComprobante(c.Request.Context(), id, ruc, autoSync)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":     false,
			"error":       err.Error(),
			"detalle":     err.Error(),
			"codigo_http": http.StatusNotFound,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func (ctrl *Controller) DownloadSingleFromSUNAT(c *gin.Context) {
	ruc := c.GetString("ruc")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "ID de comprobante inválido",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	res, err := ctrl.service.DownloadSingleFromSUNAT(c.Request.Context(), id, ruc)
	if err != nil {
		status, errCode, errTitle := classifyCPEError(err, http.StatusBadGateway, "No se pudo descargar el comprobante desde SUNAT SOL")
		c.JSON(status, gin.H{
			"success":      false,
			"error":        errTitle,
			"detalle":      err.Error(),
			"codigo_error": errCode,
			"data":         res,
			"codigo_http":  status,
		})
		return
	}

	msg := "Comprobante descargado desde SUNAT SOL"
	if res.XmlDescargado && res.PdfDescargado {
		msg = "XML y PDF oficiales descargados desde SUNAT SOL"
	} else if res.XmlDescargado && res.PdfGeneradoXML {
		msg = "XML oficial descargado desde SUNAT SOL y PDF con QR generado automáticamente"
	} else if res.XmlDescargado {
		msg = "XML oficial descargado desde SUNAT SOL"
	} else if res.PdfDescargado {
		msg = "PDF oficial descargado desde SUNAT SOL"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": msg,
		"data":    res,
	})
}

func (ctrl *Controller) StartBulkDownload(c *gin.Context) {
	ruc := c.GetString("ruc")
	var req BulkDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Parámetros de descarga masiva inválidos",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	job, err := ctrl.service.StartBulkDownload(ruc, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       err.Error(),
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"mensaje": fmt.Sprintf("Descarga masiva iniciada (%d comprobantes en cola)", job.Total),
		"data":    job,
	})
}

func (ctrl *Controller) ListBulkJobs(c *gin.Context) {
	ruc := c.GetString("ruc")
	jobs := ctrl.service.ListBulkJobs(ruc)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    jobs,
		"total":   len(jobs),
	})
}

func (ctrl *Controller) GetBulkJob(c *gin.Context) {
	ruc := c.GetString("ruc")
	jobID := c.Param("job_id")
	job, err := ctrl.service.GetBulkJob(ruc, jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":     false,
			"error":       err.Error(),
			"detalle":     err.Error(),
			"codigo_http": http.StatusNotFound,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    job,
	})
}

func (ctrl *Controller) CancelBulkJob(c *gin.Context) {
	ruc := c.GetString("ruc")
	jobID := c.Param("job_id")
	job, err := ctrl.service.CancelBulkJob(ruc, jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":     false,
			"error":       err.Error(),
			"detalle":     err.Error(),
			"codigo_http": http.StatusNotFound,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": "Descarga masiva cancelada",
		"data":    job,
	})
}

func (ctrl *Controller) DownloadBulkZip(c *gin.Context) {
	ruc := c.GetString("ruc")
	jobID := c.Param("job_id")
	data, fname, err := ctrl.service.GetBulkZip(ruc, jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":     false,
			"error":       err.Error(),
			"detalle":     err.Error(),
			"codigo_http": http.StatusNotFound,
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fname))
	c.Data(http.StatusOK, "application/zip", data)
}

func (ctrl *Controller) ExportItemsCSV(c *gin.Context) {
	ruc := c.GetString("ruc")
	origen := c.Query("origen")
	periodo := c.Query("periodo")
	tipoCP := c.Query("tipo_cp")

	data, fname, err := ctrl.service.ExportItemsCSV(ruc, origen, periodo, tipoCP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":     false,
			"error":       "Error exportando detalle de ítems",
			"detalle":     err.Error(),
			"codigo_http": http.StatusInternalServerError,
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fname))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}

func (ctrl *Controller) serveDownload(c *gin.Context, tipo string) {
	ruc := c.GetString("ruc")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "ID de comprobante inválido",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	// Por defecto intenta descargar automáticamente el archivo ORIGINAL real desde SUNAT SOL si falta en disco.
	// Nunca devuelve una ficha SIRE falsa salvo que el cliente pida explícitamente ?fallback=sire.
	autoFetch := c.Query("auto") != "0"
	allowFallback := c.Query("fallback") == "sire"

	data, filename, contentType, source, err := ctrl.service.GetDownloadFileWithContext(c.Request.Context(), id, ruc, tipo, autoFetch, allowFallback)
	if err != nil {
		status, errCode, errTitle := classifyCPEError(err, http.StatusNotFound, err.Error())
		c.JSON(status, gin.H{
			"success":      false,
			"error":        errTitle,
			"detalle":      err.Error(),
			"codigo_error": errCode,
			"codigo_http":  status,
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("X-File-Source", source)
	c.Data(http.StatusOK, contentType, data)
}

func classifyCPEError(err error, defaultStatus int, defaultTitle string) (int, string, string) {
	switch {
	case errors.Is(err, ErrSolCaptchaRequired):
		return http.StatusTooManyRequests, "SOL_CAPTCHA_REQUERIDO",
			"SUNAT solicitó un reto CAPTCHA en el acceso SOL. Inicie sesión manualmente una vez en el portal web de SUNAT para restablecerlo y vuelva a intentar."
	case errors.Is(err, ErrSolCooldown):
		return http.StatusTooManyRequests, "SOL_COOLDOWN",
			"El acceso al Portal SOL está en pausa temporal por seguridad tras un error reciente. Verifique las credenciales SOL o pruebe la sesión SOL en Conexión."
	case errors.Is(err, ErrSolAuthRejected):
		return http.StatusUnauthorized, "SOL_CREDENCIALES_INVALIDAS",
			"SUNAT rechazó el RUC, Usuario SOL o Clave SOL configurados para esta empresa."
	case errors.Is(err, ErrSolTokenExpired):
		return http.StatusUnauthorized, "SOL_TOKEN_EXPIRADO",
			"La sesión SOL de SUNAT expiró (HTTP 401). Vuelva a intentar o renueve la sesión SOL desde Conexión."
	case errors.Is(err, ErrSolBrowserMissing):
		return http.StatusServiceUnavailable, "SOL_NAVEGADOR_NO_ENCONTRADO",
			"No se detectó Microsoft Edge ni Google Chrome en el equipo para abrir sesión SOL (puede configurar CHROME_PATH)."
	case errors.Is(err, ErrComprobanteNoDisponible):
		return http.StatusNotFound, "CPE_NO_DISPONIBLE", err.Error()
	case errors.Is(err, ErrSunatTransitorio):
		return http.StatusBadGateway, "SUNAT_TRANSITORIO",
			"SUNAT devolvió un error temporal de comunicación (HTTP 5xx/timeout) al consultar el comprobante. Intente nuevamente en unos segundos."
	case errors.Is(err, ErrSunatSolicitudRechazada):
		return http.StatusUnprocessableEntity, "SUNAT_SOLICITUD_RECHAZADA",
			"SUNAT rechazó la consulta para este comprobante."
	default:
		if defaultTitle == "" {
			defaultTitle = err.Error()
		}
		return defaultStatus, "CPE_ERROR", defaultTitle
	}
}
