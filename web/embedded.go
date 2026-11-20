package web

import (
	"embed"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/SwodLore/sire-api/internal/middleware"
	"github.com/gin-gonic/gin"
)

//go:embed index.html
var indexHTML []byte

//go:embed assets/*
var assetsFS embed.FS

// RegisterRoutes registra la ruta raíz, los activos de marca embebidos y sirve la interfaz web con el token de sesión inyectado
func RegisterRoutes(r *gin.Engine) {
	r.GET("/", func(c *gin.Context) {
		token := middleware.GetSessionToken()
		html := strings.ReplaceAll(string(indexHTML), "{{LOCAL_SESSION_TOKEN}}", token)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	r.GET("/favicon.ico", func(c *gin.Context) {
		data, err := assetsFS.ReadFile("assets/favicon.ico")
		if err != nil {
			c.Status(http.StatusNoContent)
			return
		}
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, "image/x-icon", data)
	})

	r.GET("/assets/:file", func(c *gin.Context) {
		file := filepath.Base(strings.TrimSpace(c.Param("file")))
		if file == "" || file == "." || file == ".." || strings.Contains(file, "/") || strings.Contains(file, "\\") {
			c.Status(http.StatusNotFound)
			return
		}
		data, err := assetsFS.ReadFile("assets/" + file)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		contentType := "application/octet-stream"
		switch strings.ToLower(filepath.Ext(file)) {
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".ico":
			contentType = "image/x-icon"
		case ".svg":
			contentType = "image/svg+xml"
		case ".webp":
			contentType = "image/webp"
		}
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, contentType, data)
	})
}

// GetEmbeddedHTML retorna el contenido crudo del archivo HTML embebido
func GetEmbeddedHTML() []byte {
	return indexHTML
}
