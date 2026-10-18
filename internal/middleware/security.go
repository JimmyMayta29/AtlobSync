package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	localSessionToken string
	tokenMu           sync.RWMutex
)

// InitSessionToken genera un token criptográfico aleatorio único por ejecución del servidor
func InitSessionToken() string {
	tokenMu.Lock()
	defer tokenMu.Unlock()

	b := make([]byte, 32)
	_, _ = rand.Read(b)
	localSessionToken = hex.EncodeToString(b)
	return localSessionToken
}

// GetSessionToken retorna el token de sesión local actual
func GetSessionToken() string {
	tokenMu.RLock()
	defer tokenMu.RUnlock()
	if localSessionToken == "" {
		tokenMu.RUnlock()
		return InitSessionToken()
	}
	return localSessionToken
}

// SetSessionToken permite fijar un token específico (útil para pruebas unitarias)
func SetSessionToken(token string) {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	localSessionToken = token
}

// SecurityMiddleware implementa:
// 1. Validación estricta de Host (solo 127.0.0.1 y localhost)
// 2. Validación y rechazo de Origin ajeno (prevención de ataques drive-by desde otras webs en el navegador)
// 3. Exigencia obligatoria del token de sesión local en todas las rutas /api/* (excepto /api/sistema/ping)
// 4. Cabeceras HTTP de protección y CORS cerrado a orígenes locales
func SecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 1. Validar Host
		host := c.Request.Host
		hostname := host
		if h, _, err := net.SplitHostPort(host); err == nil {
			hostname = h
		}
		hostname = strings.ToLower(strings.TrimSpace(hostname))
		if hostname != "" && hostname != "127.0.0.1" && hostname != "localhost" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success":     false,
				"error":       "Host no autorizado",
				"detalle":     "La aplicación solo puede accederse vía 127.0.0.1 o localhost.",
				"codigo_http": http.StatusForbidden,
			})
			return
		}

		// 2. Validar Origin (CORS cerrado: solo orígenes locales)
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			if !isLocalOrigin(origin) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success":     false,
					"error":       "Origen no autorizado",
					"detalle":     "Petición bloqueada desde un origen externo no autorizado (" + origin + ").",
					"codigo_http": http.StatusForbidden,
				})
				return
			}
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-Local-Token, X-CSRF-Token, Authorization, Accept, Origin, Cache-Control, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		}

		// Cabeceras de seguridad del navegador
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		// Manejar preflight OPTIONS
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		// 3. Validación de Token de Sesión Local para rutas /api/*
		if strings.HasPrefix(path, "/api") {
			// Endpoints públicos para salud y detección de instancia única
			if path == "/api/sistema/ping" || path == "/api/salud" || path == "/api/sistema/salud" {
				c.Next()
				return
			}

			clientToken := c.Request.Header.Get("X-Local-Token")
			if clientToken == "" {
				clientToken = c.Request.Header.Get("X-CSRF-Token")
			}

			expectedToken := GetSessionToken()
			if clientToken == "" || clientToken != expectedToken {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success":     false,
					"error":       "Token de sesión local inválido o ausente",
					"detalle":     "Recargue la aplicación web para renovar la sesión de seguridad local.",
					"codigo_http": http.StatusForbidden,
				})
				return
			}
		}

		c.Next()
	}
}

func isLocalOrigin(origin string) bool {
	lower := strings.ToLower(origin)
	return strings.HasPrefix(lower, "http://127.0.0.1") ||
		strings.HasPrefix(lower, "http://localhost") ||
		strings.HasPrefix(lower, "https://127.0.0.1") ||
		strings.HasPrefix(lower, "https://localhost")
}
