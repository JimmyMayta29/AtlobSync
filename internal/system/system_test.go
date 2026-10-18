package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SwodLore/sire-api/internal/config"
	"github.com/gin-gonic/gin"
)

func TestSystemEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	cfg := &config.Config{
		Host:       "127.0.0.1",
		Port:       "8080",
		BaseDir:    ".",
		StorageDir: "descargas",
		KeySource:  "test",
	}

	ctrl := NewController(cfg, nil, nil, nil, nil)
	api := r.Group("/api")
	ctrl.RegisterRoutes(api)

	// 1. Probar Ping
	reqPing, _ := http.NewRequest(http.MethodGet, "/api/sistema/ping", nil)
	wPing := httptest.NewRecorder()
	r.ServeHTTP(wPing, reqPing)

	if wPing.Code != http.StatusOK {
		t.Fatalf("Ping debió responder 200, dio %d", wPing.Code)
	}

	var pingRes map[string]interface{}
	_ = json.Unmarshal(wPing.Body.Bytes(), &pingRes)
	if pingRes["app"] != "sire-api" {
		t.Fatalf("Ping debió responder app: sire-api, dio %v", pingRes)
	}

	// 2. Probar Info
	reqInfo, _ := http.NewRequest(http.MethodGet, "/api/sistema/info", nil)
	wInfo := httptest.NewRecorder()
	r.ServeHTTP(wInfo, reqInfo)

	if wInfo.Code != http.StatusOK {
		t.Fatalf("Info debió responder 200, dio %d", wInfo.Code)
	}

	var infoRes map[string]interface{}
	_ = json.Unmarshal(wInfo.Body.Bytes(), &infoRes)
	if !strings.Contains(infoRes["aviso_llave"].(string), "llave maestra") {
		t.Fatalf("Info debe contener aviso de llave maestra: %v", infoRes["aviso_llave"])
	}

	// 3. Probar validación de AbrirCarpeta
	reqErr, _ := http.NewRequest(http.MethodPost, "/api/sistema/abrir-carpeta", strings.NewReader(`{"tipo": "invalido"}`))
	reqErr.Header.Set("Content-Type", "application/json")
	wErr := httptest.NewRecorder()
	r.ServeHTTP(wErr, reqErr)

	if wErr.Code != http.StatusBadRequest {
		t.Fatalf("AbrirCarpeta con tipo inválido debió responder 400, dio %d", wErr.Code)
	}
}
