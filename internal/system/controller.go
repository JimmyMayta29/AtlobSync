package system

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SwodLore/sire-api/internal/company"
	"github.com/SwodLore/sire-api/internal/config"
	"github.com/SwodLore/sire-api/internal/sire"
	"github.com/SwodLore/sire-api/internal/sunat"
	"github.com/SwodLore/sire-api/internal/token"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

type Controller struct {
	cfg         *config.Config
	tokenMgr    *token.TokenManager
	companyRepo *company.Repository
	sunatClient *sunat.Client
	sireWorker  *sire.Worker
}

func NewController(
	cfg *config.Config,
	tokenMgr *token.TokenManager,
	companyRepo *company.Repository,
	sunatClient *sunat.Client,
	sireWorker *sire.Worker,
) *Controller {
	return &Controller{
		cfg:         cfg,
		tokenMgr:    tokenMgr,
		companyRepo: companyRepo,
		sunatClient: sunatClient,
		sireWorker:  sireWorker,
	}
}

func (ctrl *Controller) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/salud", ctrl.Salud)
	sys := rg.Group("/sistema")
	{
		sys.GET("/salud", ctrl.Salud)
		sys.GET("/ping", ctrl.Salud)
		sys.GET("/info", ctrl.GetInfo)
		sys.POST("/abrir-carpeta", ctrl.AbrirCarpeta)
		sys.POST("/diagnostico/:ruc", ctrl.DiagnosticoPeriodos)
		sys.GET("/estado-worker", ctrl.EstadoWorker)
	}
}

func (ctrl *Controller) Salud(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"app":     "sire-api",
		"version": config.AppVersion,
		"pid":     os.Getpid(),
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (ctrl *Controller) GetInfo(c *gin.Context) {
	dataDir, _ := filepath.Abs(filepath.Join(ctrl.cfg.BaseDir, "data"))
	logsDir, _ := filepath.Abs(filepath.Join(ctrl.cfg.BaseDir, "data", "logs"))
	descargasDir, _ := filepath.Abs(ctrl.cfg.StorageDir)

	c.JSON(http.StatusOK, gin.H{
		"version":       "2.0.0",
		"puerto":        ctrl.cfg.Port,
		"host":          ctrl.cfg.Host,
		"data_dir":      dataDir,
		"logs_dir":      logsDir,
		"descargas_dir": descargasDir,
		"key_source":    ctrl.cfg.KeySource,
		"aviso_llave":   "Haga copia de la llave maestra; sin ella no se pueden descifrar las credenciales.",
	})
}

type AbrirCarpetaRequest struct {
	Tipo string `json:"tipo" binding:"required"` // data, logs, descargas
}

func (ctrl *Controller) AbrirCarpeta(c *gin.Context) {
	var req AbrirCarpetaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Parámetro 'tipo' inválido (valores permitidos: data, logs, descargas)",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	var targetPath string
	switch strings.ToLower(req.Tipo) {
	case "data":
		targetPath = filepath.Join(ctrl.cfg.BaseDir, "data")
	case "logs":
		targetPath = filepath.Join(ctrl.cfg.BaseDir, "data", "logs")
	case "descargas":
		targetPath = ctrl.cfg.StorageDir
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Tipo de carpeta no reconocido",
			"detalle":     "Los valores permitidos son: data, logs, descargas",
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	absPath, _ := filepath.Abs(targetPath)
	if err := config.OpenFolder(absPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":     false,
			"error":       "Error al abrir explorador de archivos",
			"detalle":     err.Error(),
			"codigo_http": http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": "Carpeta abierta en el explorador de archivos",
		"ruta":    absPath,
	})
}

func (ctrl *Controller) DiagnosticoPeriodos(c *gin.Context) {
	ruc := c.Param("ruc")
	if err := company.ValidateRUC(ruc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "RUC inválido",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	// 1. Verificar estado de la empresa y enfriamiento
	emp, err := ctrl.companyRepo.GetByRUC(ruc)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":     false,
			"error":       "Empresa no encontrada",
			"detalle":     fmt.Sprintf("No se encontró la empresa con RUC %s", ruc),
			"codigo_http": http.StatusNotFound,
		})
		return
	}

	inCooldown, until, reason := ctrl.tokenMgr.IsCooldown(ruc)

	// 2. Obtener token del TokenManager (usa memoria o SQLite cifrado si sigue vigente)
	tokenStr, err := ctrl.tokenMgr.GetToken(ruc)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ejecutado":           false,
			"mensaje":             fmt.Sprintf("No se pudo obtener token: %v", err),
			"cooldown_activo":     inCooldown,
			"cooldown_hasta":      until.Format("15:04:05"),
			"cooldown_motivo":     reason,
			"estado_credenciales": emp.EstadoCredenciales,
		})
		return
	}

	// 3. Ejecutar UNA SOLA llamada exacta (GET periodos/140000)
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := resty.New()
	client.SetTransport(transport)
	client.SetTimeout(60 * time.Second)

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/padron/web/omisos/140000/periodos"
	start := time.Now()
	resp, reqErr := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+tokenStr).
		Get(url)
	dur := time.Since(start)

	var statusCode int
	var resultado string
	if reqErr != nil {
		resultado = "ERROR_RED"
	} else {
		statusCode = resp.StatusCode()
		if statusCode >= 200 && statusCode < 300 {
			resultado = "EXITO"
		} else {
			resultado = fmt.Sprintf("HTTP_%d", statusCode)
		}
	}
	sunat.LogAttempt(ruc, "GET", url, resultado, statusCode, dur)

	if reqErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"ejecutado":       true,
			"status_code":     0,
			"status_text":     "Error de Conexión",
			"error_conexion":  reqErr.Error(),
			"cooldown_activo": inCooldown,
		})
		return
	}

	// 4. Filtrar headers relevantes (excluyendo de forma estricta cualquier cookie)
	filteredHeaders := make(map[string]string)
	headerKeys := make([]string, 0, len(resp.Header()))
	for k := range resp.Header() {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)

	for _, k := range headerKeys {
		lowerK := strings.ToLower(k)
		if strings.Contains(lowerK, "cookie") {
			continue // Excluir Set-Cookie, Cookie, etc.
		}
		filteredHeaders[k] = resp.Header().Get(k)
	}

	// 5. Sanitizar cuerpo de respuesta
	bodyStr := resp.String()
	var prettyJSON interface{}
	isJSON := json.Unmarshal([]byte(bodyStr), &prettyJSON) == nil

	cleanSnippet := bodyStr
	if strings.Contains(bodyStr, "<html") || strings.Contains(bodyStr, "<!DOCTYPE") {
		cleanSnippet = strings.ReplaceAll(bodyStr, "\n", " ")
		cleanSnippet = strings.ReplaceAll(cleanSnippet, "\r", " ")
		for strings.Contains(cleanSnippet, "  ") {
			cleanSnippet = strings.ReplaceAll(cleanSnippet, "  ", " ")
		}
		if len(cleanSnippet) > 300 {
			cleanSnippet = cleanSnippet[:300] + "..."
		}
		cleanSnippet = "[HTML Sanitizado]: " + strings.TrimSpace(cleanSnippet)
	}

	c.JSON(http.StatusOK, gin.H{
		"ejecutado":           true,
		"status_code":         resp.StatusCode(),
		"status_text":         http.StatusText(resp.StatusCode()),
		"headers":             filteredHeaders,
		"cuerpo_sanitizado":   cleanSnippet,
		"es_json":             isJSON,
		"json_data":           prettyJSON,
		"cooldown_activo":     inCooldown,
		"cooldown_hasta":      until.Format("15:04:05"),
		"cooldown_motivo":     reason,
		"estado_credenciales": emp.EstadoCredenciales,
	})
}

func (ctrl *Controller) EstadoWorker(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"worker_activo": true,
		"descripcion":   "Worker asíncrono en segundo plano operativo (polling y descargas con backoff)",
	})
}
