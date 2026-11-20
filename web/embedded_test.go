package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/SwodLore/sire-api/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestEmbeddedWeb_PageLoadsWithoutSunatCallsAndHasSessionToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Servidor simulado de SUNAT para asegurar que NO se le envía ninguna petición al cargar la UI
	var sunatCalls atomic.Int64
	sunatMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sunatCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer sunatMock.Close()

	r := gin.New()
	testToken := "token_sesion_local_prueba_998877665544332211"
	middleware.SetSessionToken(testToken)

	RegisterRoutes(r)

	// Simular petición GET / que realiza el navegador al abrir la aplicación
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 1. Verificar respuesta 200 OK
	if w.Code != http.StatusOK {
		t.Fatalf("Se esperaba status 200 al servir interfaz embebida, se obtuvo %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type debe ser text/html, se obtuvo %s", contentType)
	}

	htmlBody := w.Body.String()

	// 2. Verificar inyección del token de sesión local
	if !strings.Contains(htmlBody, testToken) {
		t.Fatalf("La página HTML servida debe contener el token de sesión local inyectado")
	}

	// 3. Confirmar que no se disparó ninguna llamada a SUNAT
	if sunatCalls.Load() != 0 {
		t.Fatalf("La carga inicial de la UI NO debe llamar a SUNAT. Peticiones recibidas: %d", sunatCalls.Load())
	}

	// 4. Verificar que NO existan dependencias a CDNs externos (debe funcionar 100% offline)
	externalCDNs := []string{
		"cdn.jsdelivr.net",
		"cdnjs.cloudflare.com",
		"unpkg.com",
		"fonts.googleapis.com",
		"stackpath.bootstrapcdn.com",
		"code.jquery.com",
	}
	for _, cdn := range externalCDNs {
		if strings.Contains(htmlBody, cdn) {
			t.Errorf("Se detectó dependencia externa a CDN no permitida: %s", cdn)
		}
	}
}

func TestEmbeddedWeb_ServesBrandAssetsAndFavicon(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r)

	cases := []struct {
		path        string
		contentType string
	}{
		{"/favicon.ico", "image/x-icon"},
		{"/assets/atlob-icon.png", "image/png"},
		{"/assets/atlob-isotipo.png", "image/png"},
		{"/assets/atlob-logo-horizontal.png", "image/png"},
		{"/assets/atlob-logo-horizontal-dark.png", "image/png"},
		{"/assets/atlob-logo-vertical.png", "image/png"},
	}

	for _, tc := range cases {
		req, _ := http.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s devolvió %d, se esperaba 200", tc.path, w.Code)
		}
		if got := w.Header().Get("Content-Type"); !strings.Contains(got, tc.contentType) {
			t.Fatalf("GET %s Content-Type = %q, se esperaba %q", tc.path, got, tc.contentType)
		}
		if w.Body.Len() == 0 {
			t.Fatalf("GET %s devolvió cuerpo vacío", tc.path)
		}
	}

	// Ruta inexistente debe devolver 404
	req404, _ := http.NewRequest(http.MethodGet, "/assets/no-existe.png", nil)
	w404 := httptest.NewRecorder()
	r.ServeHTTP(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Fatalf("Asset inexistente devolvió %d, se esperaba 404", w404.Code)
	}
}
