package auth

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

func ObtenerTokenService(clientID, clientSecret, ruc, usuario, claveSOL string) (string, error) {
	client := resty.New()

	// Limpiar posibles espacios en credenciales
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	ruc = strings.TrimSpace(ruc)
	usuario = strings.TrimSpace(usuario)
	claveSOL = strings.TrimSpace(claveSOL)

	url := fmt.Sprintf("https://api-seguridad.sunat.gob.pe/v1/clientessol/%s/oauth2/token/", clientID)

	// Username: RUC + usuario SOL sin espacio
	formData := map[string]string{
		"grant_type":    "password",
		"scope":         "https://api-sire.sunat.gob.pe",
		"client_id":     clientID,
		"client_secret": clientSecret,
		"username":      fmt.Sprintf("%s%s", ruc, usuario), // <- Sin espacio
		"password":      claveSOL,
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(formData).
		Post(url)

	if err != nil {
		return "", err
	}

	if resp.IsError() {
		return "", fmt.Errorf("error al obtener token: %s", resp.Status())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("error al parsear JSON: %w", err)
	}

	accessToken, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("no se encontró el campo 'access_token' en la respuesta")
	}

	return accessToken, nil
}
