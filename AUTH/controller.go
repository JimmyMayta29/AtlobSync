package auth

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func ObtenerTokenController(c *gin.Context) {
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	ruc := os.Getenv("RUC")
	usuario := os.Getenv("USUARIO")
	claveSOL := os.Getenv("CLAVE_SOL")

	if clientID == "" || clientSecret == "" || ruc == "" || usuario == "" || claveSOL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Faltan variables de entorno"})
		return
	}

	token, err := ObtenerTokenService(clientID, clientSecret, ruc, usuario, claveSOL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"access_token": token})
}
