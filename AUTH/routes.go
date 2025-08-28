package auth

import (
	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(r *gin.Engine) {
	sunat := r.Group("/api/sunat/auth")
	{
		sunat.GET("/obtener-token", ObtenerTokenController)
	}
}
