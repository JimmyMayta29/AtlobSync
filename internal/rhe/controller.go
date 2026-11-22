package rhe

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	svc *Service
}

func NewController(svc *Service) *Controller {
	return &Controller{svc: svc}
}

func (ctrl *Controller) RegisterRoutes(api *gin.RouterGroup, empresaMiddleware gin.HandlerFunc) {
	g := api.Group("/empresas/:ruc/rhe", empresaMiddleware)
	{
		g.GET("", ctrl.Listar)
		g.POST("/consultar", ctrl.Consultar)
		g.GET("/exportar", ctrl.Exportar)
	}
}

// GET /api/empresas/:ruc/rhe?periodo=202610&estado=TODOS&moneda=TODOS&q=...
func (ctrl *Controller) Listar(c *gin.Context) {
	ruc := strings.TrimSpace(c.Param("ruc"))
	filter := RHEListFilter{
		Periodo: strings.TrimSpace(c.Query("periodo")),
		Estado:  strings.TrimSpace(c.Query("estado")),
		Moneda:  strings.TrimSpace(c.Query("moneda")),
		Query:   strings.TrimSpace(c.Query("q")),
	}

	res, err := ctrl.svc.List(ruc, filter)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "No se pudo listar los Recibos por Honorarios",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    res,
	})
}

type consultarRHERequest struct {
	Periodo string `json:"periodo"`
}

// POST /api/empresas/:ruc/rhe/consultar
func (ctrl *Controller) Consultar(c *gin.Context) {
	ruc := strings.TrimSpace(c.Param("ruc"))
	var req consultarRHERequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Solicitud inválida",
			"detalle":     "Debe enviar el periodo en formato YYYYMM (ej. {\"periodo\":\"202610\"})",
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	res, err := ctrl.svc.ConsultarEnSUNAT(c.Request.Context(), ruc, req.Periodo)
	if err != nil {
		status := http.StatusBadGateway
		code := "ERROR_CONSULTA_RHE"
		switch {
		case strings.Contains(err.Error(), "periodo inválido"), strings.Contains(err.Error(), "aún no ha iniciado"):
			status = http.StatusBadRequest
			code = "PERIODO_INVALIDO"
		case errors.Is(err, ErrSolCaptchaRequired):
			status = http.StatusTooManyRequests
			code = "SOL_CAPTCHA_REQUERIDO"
		case errors.Is(err, ErrSolCooldown):
			status = http.StatusTooManyRequests
			code = "SOL_COOLDOWN"
		case errors.Is(err, ErrSolAuthRejected):
			status = http.StatusUnauthorized
			code = "SOL_CREDENCIALES_INVALIDAS"
		case errors.Is(err, ErrSolBrowserMissing):
			status = http.StatusPreconditionFailed
			code = "NAVEGADOR_NO_ENCONTRADO"
		}
		c.JSON(status, gin.H{
			"success":     false,
			"codigo":      code,
			"error":       "No se pudo consultar Recibos por Honorarios en SUNAT",
			"detalle":     err.Error(),
			"codigo_http": status,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mensaje": res.Mensaje,
		"data":    res,
	})
}

// GET /api/empresas/:ruc/rhe/exportar?formato=xlsx|txt&periodo=202610&estado=TODOS&q=...
func (ctrl *Controller) Exportar(c *gin.Context) {
	ruc := strings.TrimSpace(c.Param("ruc"))
	formato := strings.TrimSpace(c.DefaultQuery("formato", "xlsx"))
	filter := RHEListFilter{
		Periodo: strings.TrimSpace(c.Query("periodo")),
		Estado:  strings.TrimSpace(c.Query("estado")),
		Moneda:  strings.TrimSpace(c.Query("moneda")),
		Query:   strings.TrimSpace(c.Query("q")),
	}

	file, err := ctrl.svc.Exportar(ruc, filter, formato)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "No se pudo exportar los Recibos por Honorarios",
			"detalle":     err.Error(),
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.FileName))
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, file.ContentType, file.Data)
}
