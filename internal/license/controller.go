package license

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	svc *Service
}

func NewController(svc *Service) *Controller {
	return &Controller{svc: svc}
}

func (ctrl *Controller) RegisterRoutes(rg *gin.RouterGroup) {
	lic := rg.Group("/licencia")
	{
		lic.GET("/estado", ctrl.Estado)
		lic.POST("/activar", ctrl.Activar)
		lic.POST("/validar", ctrl.Validar)
	}
}

func (ctrl *Controller) Estado(c *gin.Context) {
	if ctrl.svc == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "licencia": LicenseStatus{Enforced: false, Valid: true, Code: CodeDevBypass}})
		return
	}

	if strings.TrimSpace(c.Query("refresh")) == "1" {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		st, _ := ctrl.svc.Refresh(ctx)
		c.JSON(http.StatusOK, gin.H{"success": true, "licencia": st})
		return
	}

	st := ctrl.svc.GetStatus()
	c.JSON(http.StatusOK, gin.H{"success": true, "licencia": st})
}

type activarRequest struct {
	LicenseKey string `json:"license_key" binding:"required"`
}

func (ctrl *Controller) Activar(c *gin.Context) {
	if ctrl.svc == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "mensaje": "Modo desarrollo local"})
		return
	}

	var req activarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       "Código de licencia requerido",
			"detalle":     "Ingresa tu código de prueba o licencia comercial (ej. ATLOB-DEMO-XXXX-XXXX).",
			"codigo_http": http.StatusBadRequest,
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	st, err := ctrl.svc.Activate(ctx, req.LicenseKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":     false,
			"error":       err.Error(),
			"detalle":     st.Message,
			"codigo":      "LICENCIA_" + st.Code,
			"codigo_http": http.StatusBadRequest,
			"licencia":    st,
		})
		return
	}

	if !st.Valid {
		httpCode := http.StatusBadRequest
		if st.Code == CodeExpired || st.Code == CodeRevoked || st.Code == CodeSuspended || st.Code == CodeMachineMismatch {
			httpCode = http.StatusPaymentRequired
		}
		c.JSON(httpCode, gin.H{
			"success":     false,
			"error":       st.Message,
			"detalle":     st.Message,
			"codigo":      "LICENCIA_" + st.Code,
			"codigo_http": httpCode,
			"licencia":    st,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"mensaje":  st.Message,
		"licencia": st,
	})
}

func (ctrl *Controller) Validar(c *gin.Context) {
	if ctrl.svc == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "licencia": LicenseStatus{Enforced: false, Valid: true, Code: CodeDevBypass}})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	st, err := ctrl.svc.Refresh(ctx)
	if err != nil && !st.Valid {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"success":     false,
			"error":       st.Message,
			"detalle":     err.Error(),
			"codigo":      "LICENCIA_" + st.Code,
			"codigo_http": http.StatusPaymentRequired,
			"licencia":    st,
		})
		return
	}

	if !st.Valid {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"success":     false,
			"error":       st.Message,
			"detalle":     st.Message,
			"codigo":      "LICENCIA_" + st.Code,
			"codigo_http": http.StatusPaymentRequired,
			"licencia":    st,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"mensaje":  st.Message,
		"licencia": st,
	})
}
