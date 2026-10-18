package web

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/SwodLore/sire-api/internal/middleware"
	"github.com/gin-gonic/gin"
)

//go:embed index.html
var indexHTML []byte

// RegisterRoutes registra la ruta raíz y sirve la interfaz web embebida con el token de sesión inyectado
func RegisterRoutes(r *gin.Engine) {
	r.GET("/", func(c *gin.Context) {
		token := middleware.GetSessionToken()
		html := strings.ReplaceAll(string(indexHTML), "{{LOCAL_SESSION_TOKEN}}", token)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
}

// GetEmbeddedHTML retorna el contenido crudo del archivo HTML embebido
func GetEmbeddedHTML() []byte {
	return indexHTML
}
