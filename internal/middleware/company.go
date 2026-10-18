package middleware

import (
	"net/http"

	"github.com/SwodLore/sire-api/internal/company"
	"github.com/gin-gonic/gin"
)

// RequireActiveCompany valida que el RUC sea válido, que la empresa exista en BD y esté activa
func RequireActiveCompany(repo *company.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruc := c.Param("ruc")
		if ruc == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success":     false,
				"error":       "Parámetro RUC obligatorio",
				"detalle":     "El parámetro RUC es obligatorio en la ruta (/api/empresas/:ruc/...)",
				"codigo_http": http.StatusBadRequest,
			})
			return
		}

		if err := company.ValidateRUC(ruc); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success":     false,
				"error":       "RUC inválido",
				"detalle":     err.Error(),
				"codigo_http": http.StatusBadRequest,
			})
			return
		}

		emp, err := repo.GetByRUC(ruc)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"success":     false,
				"error":       "Empresa no encontrada",
				"detalle":     "La empresa con RUC " + ruc + " no existe o fue eliminada",
				"codigo_http": http.StatusNotFound,
			})
			return
		}

		if emp.Activo == 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success":     false,
				"error":       "Empresa inactiva",
				"detalle":     "La empresa con RUC " + ruc + " se encuentra inactiva",
				"codigo_http": http.StatusForbidden,
			})
			return
		}

		c.Set("ruc", ruc)
		c.Set("empresa", emp)
		c.Next()
	}
}

// RequireDefaultCompany resuelve la empresa configurada en DEFAULT_RUC para soportar rutas heredadas (/api/sunat/...)
func RequireDefaultCompany(repo *company.Repository, defaultRUC string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if defaultRUC == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success":     false,
				"error":       "DEFAULT_RUC no configurado",
				"detalle":     "No se ha configurado la variable DEFAULT_RUC en .env para atender rutas heredadas (/api/sunat/...). Configure DEFAULT_RUC en su .env o migre sus llamadas a /api/empresas/:ruc/...",
				"codigo_http": http.StatusBadRequest,
			})
			return
		}

		if err := company.ValidateRUC(defaultRUC); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success":     false,
				"error":       "DEFAULT_RUC inválido",
				"detalle":     "La variable DEFAULT_RUC en .env no contiene un RUC válido: " + err.Error(),
				"codigo_http": http.StatusBadRequest,
			})
			return
		}

		emp, err := repo.GetByRUC(defaultRUC)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"success":     false,
				"error":       "Empresa predeterminada no encontrada",
				"detalle":     "La empresa configurada en DEFAULT_RUC (" + defaultRUC + ") no existe o fue eliminada en la base de datos.",
				"codigo_http": http.StatusNotFound,
			})
			return
		}

		if emp.Activo == 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success":     false,
				"error":       "Empresa predeterminada inactiva",
				"detalle":     "La empresa configurada en DEFAULT_RUC (" + defaultRUC + ") se encuentra inactiva.",
				"codigo_http": http.StatusForbidden,
			})
			return
		}

		c.Set("ruc", emp.RUC)
		c.Set("empresa", emp)
		c.Header("X-Sire-Legacy-Route", "deprecated; migre a /api/empresas/:ruc/...")
		c.Next()
	}
}
