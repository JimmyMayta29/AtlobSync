package license

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireValidLicense bloquea las rutas de negocio (/api/*) cuando el licenciamiento comercial está activo
// y la licencia aún no fue activada, expiró, fue revocada o falló la verificación de equipo.
func RequireValidLicense(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if isLicenseExemptPath(path) {
			c.Next()
			return
		}

		st := svc.GetStatus()
		if !st.Enforced || st.Valid {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
			"success":     false,
			"codigo":      "LICENCIA_" + st.Code,
			"error":       "Licencia requerida o vencida",
			"detalle":     st.Message,
			"codigo_http": http.StatusPaymentRequired,
			"licencia":    st,
		})
	}
}

func isLicenseExemptPath(path string) bool {
	if path == "/api/salud" ||
		path == "/api/sistema/salud" ||
		path == "/api/sistema/ping" ||
		path == "/api/sistema/info" ||
		path == "/api/sistema/abrir-url" {
		return true
	}
	return strings.HasPrefix(path, "/api/licencia")
}
