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

	// 4. Probar validación estricta de URLs externas (whitelist + HTTPS)
	allowed := []string{
		"https://wa.me/51940781298?text=Hola",
		"https://api.whatsapp.com/send?phone=51940781298",
		"https://e-menu.sunat.gob.pe/cl-ti-itmenu/MenuInternet.htm",
	}
	for _, u := range allowed {
		if _, err := config.ValidateExternalURL(u); err != nil {
			t.Errorf("URL oficial debía permitirse (%s): %v", u, err)
		}
	}

	blocked := []string{
		"http://wa.me/51940781298",
		"javascript:alert(1)",
		"file:///C:/Windows/System32/calc.exe",
		"https://evil.example.com/phish",
		"https://wa.me:8443/51940781298",
		"https://user@wa.me/51940781298",
	}
	for _, u := range blocked {
		if _, err := config.ValidateExternalURL(u); err == nil {
			t.Errorf("URL peligrosa o no autorizada debía bloquearse: %s", u)
		}
		reqURL, _ := http.NewRequest(http.MethodPost, "/api/sistema/abrir-url", strings.NewReader(`{"url": "`+u+`"}`))
		reqURL.Header.Set("Content-Type", "application/json")
		wURL := httptest.NewRecorder()
		r.ServeHTTP(wURL, reqURL)
		if wURL.Code != http.StatusBadRequest {
			t.Errorf("POST /api/sistema/abrir-url con URL bloqueada %s dio %d, se esperaba 400", u, wURL.Code)
		}
	}
}
