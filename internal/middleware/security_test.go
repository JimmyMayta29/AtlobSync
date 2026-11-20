package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestServerWithSecurity() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetSessionToken("token_prueba_secreto_1234567890123456")
	r.Use(SecurityMiddleware())

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "HTML UI")
	})

	r.GET("/api/sistema/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "app": "sire-api"})
	})

	r.GET("/api/empresas", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"empresas": []string{}})
	})

	return r
}

func TestSecurity_LocalSessionTokenRequiredForAPI(t *testing.T) {
	r := setupTestServerWithSecurity()

	// 1. Petición a /api sin token -> 403 Forbidden
	req1, _ := http.NewRequest(http.MethodGet, "/api/empresas", nil)
	req1.Host = "127.0.0.1:8080"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusForbidden {
		t.Fatalf("Se esperaba 403 sin token de sesión, pero se obtuvo %d", w1.Code)
	}

	// 2. Petición a /api con token inválido -> 403 Forbidden
	req2, _ := http.NewRequest(http.MethodGet, "/api/empresas", nil)
	req2.Host = "127.0.0.1:8080"
	req2.Header.Set("X-Local-Token", "token_falso_invalido")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Fatalf("Se esperaba 403 con token erróneo, pero se obtuvo %d", w2.Code)
	}

	// 3. Petición a /api con token válido vía X-Local-Token -> 200 OK
	req3, _ := http.NewRequest(http.MethodGet, "/api/empresas", nil)
	req3.Host = "127.0.0.1:8080"
	req3.Header.Set("X-Local-Token", "token_prueba_secreto_1234567890123456")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("Se esperaba 200 con token correcto, pero se obtuvo %d (cuerpo: %s)", w3.Code, w3.Body.String())
	}

	// 4. Petición a /api con token válido vía X-CSRF-Token -> 200 OK
	req4, _ := http.NewRequest(http.MethodGet, "/api/empresas", nil)
	req4.Host = "127.0.0.1:8080"
	req4.Header.Set("X-CSRF-Token", "token_prueba_secreto_1234567890123456")
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("Se esperaba 200 con X-CSRF-Token, pero se obtuvo %d", w4.Code)
	}
}

func TestSecurity_PublicEndpointsAndRootAllowedWithoutToken(t *testing.T) {
	r := setupTestServerWithSecurity()

	// 1. Ruta raíz / (para cargar la UI embebida) no debe requerir token
	reqRoot, _ := http.NewRequest(http.MethodGet, "/", nil)
	reqRoot.Host = "127.0.0.1:8080"
	wRoot := httptest.NewRecorder()
	r.ServeHTTP(wRoot, reqRoot)

	if wRoot.Code != http.StatusOK {
		t.Fatalf("La ruta raíz / debe permitirse sin token, se obtuvo %d", wRoot.Code)
	}

	// 2. Ruta /api/sistema/ping (para detección de instancia corriendo) no debe requerir token
	reqPing, _ := http.NewRequest(http.MethodGet, "/api/sistema/ping", nil)
	reqPing.Host = "127.0.0.1:8080"
	wPing := httptest.NewRecorder()
	r.ServeHTTP(wPing, reqPing)

	if wPing.Code != http.StatusOK {
		t.Fatalf("/api/sistema/ping debe permitirse sin token, se obtuvo %d", wPing.Code)
	}
}

func TestSecurity_RejectionOfForeignOriginAndHost(t *testing.T) {
	r := setupTestServerWithSecurity()
	validToken := "token_prueba_secreto_1234567890123456"

	// 1. Petición con Origin malicioso desde otra web en el navegador -> 403 Forbidden
	reqEvilOrigin, _ := http.NewRequest(http.MethodGet, "/api/empresas", nil)
	reqEvilOrigin.Host = "127.0.0.1:8080"
	reqEvilOrigin.Header.Set("Origin", "http://malicious-website.com")
	reqEvilOrigin.Header.Set("X-Local-Token", validToken)
	wEvil := httptest.NewRecorder()
	r.ServeHTTP(wEvil, reqEvilOrigin)

	if wEvil.Code != http.StatusForbidden {
		t.Fatalf("Se esperaba 403 para Origin externo malicioso, pero se obtuvo %d", wEvil.Code)
	}

	// 2. Petición con Origin local permitido -> 200 OK
	reqLocalOrigin, _ := http.NewRequest(http.MethodGet, "/api/empresas", nil)
	reqLocalOrigin.Host = "127.0.0.1:8080"
	reqLocalOrigin.Header.Set("Origin", "http://127.0.0.1:8080")
	reqLocalOrigin.Header.Set("X-Local-Token", validToken)
	wLocal := httptest.NewRecorder()
	r.ServeHTTP(wLocal, reqLocalOrigin)

	if wLocal.Code != http.StatusOK {
		t.Fatalf("Se esperaba 200 para Origin local, pero se obtuvo %d", wLocal.Code)
	}

	// 3. Petición con Host externo no permitido (DNS rebinding attack) -> 403 Forbidden
	reqEvilHost, _ := http.NewRequest(http.MethodGet, "/api/empresas", nil)
	reqEvilHost.Host = "attacker-domain.com"
	reqEvilHost.Header.Set("X-Local-Token", validToken)
	wHost := httptest.NewRecorder()
	r.ServeHTTP(wHost, reqEvilHost)

	if wHost.Code != http.StatusForbidden {
		t.Fatalf("Se esperaba 403 para Host no permitido, pero se obtuvo %d", wHost.Code)
	}
}

func TestSanitizedLoggerMiddleware_OmitsQueryAndMasks11DigitSegments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logBuf bytes.Buffer

	r := gin.New()
	r.Use(SanitizedLoggerMiddlewareWithWriter(&logBuf))
	r.GET("/api/salud", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/descargas/:id/propuesta-txt", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/empresas/:ruc/cpe", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/sire/tickets/:ruc/:ticket", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/sire/propuesta/detalle", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/items/:code", func(c *gin.Context) { c.Status(http.StatusOK) })

	cases := []struct {
		name         string
		target       string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:         "query string omitido completamente",
			target:       "/api/sire/propuesta/detalle?ruc=20000000001&token=secreto_xyz&periodo=202610",
			wantContains: []string{`"/api/sire/propuesta/detalle"`},
			wantAbsent:   []string{"?", "20000000001", "secreto_xyz", "periodo=202610"},
		},
		{
			name:         "ruc enmascarado en ruta de empresa",
			target:       "/api/empresas/20000000001/cpe?origen=COMPRAS&q=factura",
			wantContains: []string{`"/api/empresas/[RUC]/cpe"`},
			wantAbsent:   []string{"20000000001", "origen=", "factura"},
		},
		{
			name:         "ruc enmascarado en ruta de tickets conservando ticket de 14 digitos",
			target:       "/api/sire/tickets/20000000001/20260300000080",
			wantContains: []string{`"/api/sire/tickets/[RUC]/20260300000080"`},
			wantAbsent:   []string{"20000000001"},
		},
		{
			name:         "segmento de 10 digitos no enmascarado",
			target:       "/api/items/1234567890",
			wantContains: []string{`"/api/items/1234567890"`},
			wantAbsent:   []string{"[RUC]"},
		},
		{
			name:         "segmento de 12 digitos no enmascarado",
			target:       "/api/items/123456789012",
			wantContains: []string{`"/api/items/123456789012"`},
			wantAbsent:   []string{"[RUC]"},
		},
		{
			name:         "id numerico de 11 digitos no-RUC enmascarado por seguridad",
			target:       "/api/descargas/99999999999/propuesta-txt?limit=50",
			wantContains: []string{`"/api/descargas/[RUC]/propuesta-txt"`},
			wantAbsent:   []string{"99999999999", "limit=50"},
		},
		{
			name:         "ruta sin ruc permanece intacta",
			target:       "/api/descargas/42/propuesta-txt",
			wantContains: []string{`"/api/descargas/42/propuesta-txt"`},
			wantAbsent:   []string{"[RUC]"},
		},
		{
			name:         "ruta salud intacta",
			target:       "/api/salud",
			wantContains: []string{`"/api/salud"`},
			wantAbsent:   []string{"[RUC]"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logBuf.Reset()
			req, _ := http.NewRequest(http.MethodGet, tc.target, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status inesperado %d", w.Code)
			}
			line := logBuf.String()
			for _, want := range tc.wantContains {
				if !strings.Contains(line, want) {
					t.Errorf("log debería contener %q, obtenido: %s", want, line)
				}
			}
			for _, bad := range tc.wantAbsent {
				if strings.Contains(line, bad) {
					t.Errorf("log NO debería contener %q, obtenido: %s", bad, line)
				}
			}
		})
	}
}
