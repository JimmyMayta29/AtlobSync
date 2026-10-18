package sunat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type mockTokenProvider struct {
	mu           sync.Mutex
	tokenCount   int
	invalidated  bool
	tokensToGive []string
	cooldownRUC  string
}

func (m *mockTokenProvider) GetToken(ruc string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cooldownRUC == ruc {
		return "", errors.New("empresa en enfriamiento")
	}
	if m.tokenCount < len(m.tokensToGive) {
		tok := m.tokensToGive[m.tokenCount]
		m.tokenCount++
		return tok, nil
	}
	return "token_default", nil
}

func (m *mockTokenProvider) InvalidateToken(ruc string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invalidated = true
}

func (m *mockTokenProvider) IsCooldown(ruc string) (bool, time.Time, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cooldownRUC == ruc {
		return true, time.Now().Add(15 * time.Minute), "Credenciales SOL inválidas"
	}
	return false, time.Time{}, ""
}

func (m *mockTokenProvider) ClearCooldown(ruc string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cooldownRUC == ruc {
		m.cooldownRUC = ""
	}
}

func TestClientNativeRetryOn401(t *testing.T) {
	var requestCount int
	var receivedTokens []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		auth := r.Header.Get("Authorization")
		receivedTokens = append(receivedTokens, auth)

		if auth == "Bearer token_vencido" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "token expirado"}`))
			return
		}

		if auth == "Bearer token_renovado" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"resultado": "exito_tras_renovacion"}`))
			return
		}

		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	mockToken := &mockTokenProvider{
		tokensToGive: []string{"token_vencido", "token_renovado"},
	}

	client := NewClient(mockToken)
	client.SetBaseURL(server.URL)

	req, err := client.R("20000000001")
	if err != nil {
		t.Fatalf("Error al crear request: %v", err)
	}

	resp, err := req.Get(server.URL + "/periodos")
	if err != nil {
		t.Fatalf("Error durante llamada con reintento: %v", err)
	}

	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("Se esperaba 200 OK tras reintento, se obtuvo: %d", resp.StatusCode())
	}

	if requestCount != 2 {
		t.Fatalf("Se esperaban exactamente 2 intentos (inicial + reintento), ocurrieron %d", requestCount)
	}

	if !mockToken.invalidated {
		t.Fatalf("El gestor de tokens debió haber recibido InvalidateToken tras el 401")
	}

	if receivedTokens[0] != "Bearer token_vencido" || receivedTokens[1] != "Bearer token_renovado" {
		t.Fatalf("Secuencia de tokens incorrecta: %v", receivedTokens)
	}
}

type failingTokenProvider struct {
	invalidated bool
}

func (f *failingTokenProvider) GetToken(ruc string) (string, error) {
	if f.invalidated {
		return "", errors.New("falla intencional al renovar token")
	}
	return "token_inicial_invalido", nil
}

func (f *failingTokenProvider) InvalidateToken(ruc string) {
	f.invalidated = true
}

func (f *failingTokenProvider) IsCooldown(ruc string) (bool, time.Time, string) {
	return false, time.Time{}, ""
}

func (f *failingTokenProvider) ClearCooldown(ruc string) {}

func TestClient_401RenewalFails_NoLoop(t *testing.T) {
	var requestCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Unauthorized"}`))
	}))
	defer server.Close()

	mockProvider := &failingTokenProvider{}
	client := NewClient(mockProvider)
	client.SetBaseURL(server.URL)

	req, err := client.R("20000000001")
	if err != nil {
		t.Fatalf("Error al crear request: %v", err)
	}

	resp, err := req.Get(server.URL + "/periodos")
	if err != nil {
		t.Fatalf("Error inesperado en request: %v", err)
	}

	if resp.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("Se esperaba 401, se obtuvo: %d", resp.StatusCode())
	}

	// Al fallar la renovación de token, NO debe haber reintento (requestCount debe ser 1)
	if requestCount != 1 {
		t.Fatalf("Se esperaba exactamente 1 petición sin reintento al fallar la renovación, ocurrieron: %d", requestCount)
	}

	if !mockProvider.invalidated {
		t.Fatalf("El token debió ser invalidado en el primer 401")
	}
}

func TestClient_NoTokenNeverSendsRequestToSunat(t *testing.T) {
	var requestsReceived int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsReceived++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	mockToken := &mockTokenProvider{
		cooldownRUC: "20999999999", // En enfriamiento / sin token
	}

	client := NewClient(mockToken)
	client.SetBaseURL(server.URL)

	// NewRequest debe abortar y nunca emitir la llamada por la red
	req := client.NewRequest("20999999999")
	resp, err := req.Get(server.URL + "/comprobantes")

	if err == nil {
		t.Fatalf("Se esperaba error al despachar request sin token, pero se obtuvo resp: %v", resp)
	}

	// Servidor no debe haber recibido NINGUNA petición
	if requestsReceived != 0 {
		t.Fatalf("El servidor SUNAT recibió %d peticiones cuando debió recibir CERO (abortado preventivamente)", requestsReceived)
	}
}

func TestClient_OnlyExpectedHeadersSent(t *testing.T) {
	var capturedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	mockToken := &mockTokenProvider{
		tokensToGive: []string{"test_token_123"},
	}

	client := NewClient(mockToken)
	client.SetBaseURL(server.URL)

	req, err := client.R("20000000001")
	if err != nil {
		t.Fatalf("Error al crear request: %v", err)
	}

	req.SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json")

	resp, err := req.Get(server.URL + "/periodos")
	if err != nil {
		t.Fatalf("Error en request: %v", err)
	}

	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("Se esperaba 200, se obtuvo %d", resp.StatusCode())
	}

	// Verificar que NO exista ningún header X-Tenant-RUC, X-Abort-Reason ni similar
	for name := range capturedHeaders {
		if strings.HasPrefix(strings.ToLower(name), "x-") {
			t.Errorf("Header personalizado no permitido detectado en la petición: %s = %s", name, capturedHeaders.Get(name))
		}
	}

	// Verificar headers del código original
	if capturedHeaders.Get("Authorization") != "Bearer test_token_123" {
		t.Errorf("Authorization incorrecto: %s", capturedHeaders.Get("Authorization"))
	}
	if capturedHeaders.Get("Accept") != "application/json" {
		t.Errorf("Accept incorrecto: %s", capturedHeaders.Get("Accept"))
	}
	if capturedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type incorrecto: %s", capturedHeaders.Get("Content-Type"))
	}
}

func TestFormatSunatResponse_SanitizesHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Simular error con HTML crudo de pasarela Dynatrace / Nginx
	rawHTMLErr := errors.New("<html><head><title>502 Bad Gateway</title></head><body><h1>Bad Gateway</h1><p>The proxy server received an invalid response.</p></body></html>")
	RespondWithError(c, rawHTMLErr)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("Se esperaba status 502, se obtuvo %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type debe ser application/json, se obtuvo %s", contentType)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("La respuesta no es JSON válido: %v", err)
	}

	if res["success"] != false {
		t.Errorf("success debe ser false, se obtuvo %v", res["success"])
	}
	if res["codigo_http"] != float64(http.StatusBadGateway) {
		t.Errorf("codigo_http debe ser 502, se obtuvo %v", res["codigo_http"])
	}

	detalle := fmt.Sprintf("%v", res["detalle"])
	if strings.Contains(detalle, "<html>") || strings.Contains(detalle, "<body>") || strings.Contains(detalle, "<p>") {
		t.Errorf("El detalle técnico no debe contener tags HTML: %s", detalle)
	}
}

func TestClassifyError_AllFiveCategories(t *testing.T) {
	// 1. gateway_sire (401 con HTML de Nginx ¡CRÍTICO: NO clasificar como servidor no disponible / 5xx!)
	nginx401HTML := "<html><head><title>401 Authorization Required</title></head><body><center><h1>401 Authorization Required</h1></center><hr><center>nginx</center></body></html>"
	err401 := &SunatHttpError{
		StatusCode: 401,
		Host:       "api-sire.sunat.gob.pe",
		Duration:   120 * time.Millisecond,
		RawBody:    nginx401HTML,
		Headers:    map[string]string{"Server": "nginx", "Content-Type": "text/html"},
		Ruta:       "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/padron/web/omisos/140000/periodos",
	}
	ce401 := ClassifyError(err401)
	if ce401.Tipo != TipoGatewaySire {
		t.Fatalf("Se esperaba tipo %s para 401 nginx, pero se obtuvo %s", TipoGatewaySire, ce401.Tipo)
	}
	if ce401.Tipo == TipoSunat5xx {
		t.Fatalf("¡VIOLACIÓN DE REGLA!: 401 fue clasificado erróneamente como sunat_5xx")
	}
	if ce401.StatusCode != 401 {
		t.Errorf("Se esperaba código 401, se obtuvo %d", ce401.StatusCode)
	}
	expectedUserMsg := "SUNAT aceptó el inicio de sesión pero rechazó el token en SIRE. Puede faltar una habilitación de la cuenta o de la aplicación. Contacte a SUNAT con los datos de abajo."
	if ce401.UserMsg != expectedUserMsg {
		t.Errorf("Mensaje de usuario incorrecto: %s", ce401.UserMsg)
	}
	if ce401.TechDetail == ce401.UserMsg {
		t.Errorf("El detalle técnico no debe ser igual al mensaje de usuario")
	}
	if !strings.Contains(ce401.TechDetail, "Host: api-sire.sunat.gob.pe") ||
		!strings.Contains(ce401.TechDetail, "Código HTTP: 401") ||
		!strings.Contains(ce401.TechDetail, "Tipo: gateway_sire") {
		t.Errorf("Detalle técnico no contiene los campos requeridos: %s", ce401.TechDetail)
	}
	if strings.Contains(ce401.TechDetail, "<html>") || strings.Contains(ce401.TechDetail, "<title>") {
		t.Errorf("Detalle técnico contiene HTML sin sanitizar: %s", ce401.TechDetail)
	}

	// 2. credenciales (invalid_grant)
	errCred := &SunatHttpError{
		StatusCode: 400,
		Host:       "api-seguridad.sunat.gob.pe",
		Duration:   250 * time.Millisecond,
		RawBody:    `{"error": "invalid_grant", "error_description": "Error en la autenticacion del usuario SOL"}`,
	}
	ceCred := ClassifyError(errCred)
	if ceCred.Tipo != TipoCredenciales {
		t.Errorf("Se esperaba tipo %s para invalid_grant, se obtuvo %s", TipoCredenciales, ceCred.Tipo)
	}
	if !strings.Contains(ceCred.UserMsg, "Credenciales SOL inválidas o vencidas") {
		t.Errorf("Mensaje de usuario incorrecto para credenciales: %s", ceCred.UserMsg)
	}

	// 3. red (DNS / timeout)
	errRed := errors.New("dial tcp 190.223.32.1:443: connect: connection refused")
	ceRed := ClassifyError(errRed)
	if ceRed.Tipo != TipoRed {
		t.Errorf("Se esperaba tipo %s para error de red, se obtuvo %s", TipoRed, ceRed.Tipo)
	}
	if !strings.Contains(ceRed.TechDetail, "Mensaje Go:") {
		t.Errorf("Detalle técnico de red debe incluir mensaje de Go: %s", ceRed.TechDetail)
	}

	// 4. sunat_5xx (500, 502, 503, 504)
	err502 := &SunatHttpError{
		StatusCode: 502,
		Host:       "api-sire.sunat.gob.pe",
		Duration:   500 * time.Millisecond,
		RawBody:    "Bad Gateway upstream service timed out",
	}
	ce502 := ClassifyError(err502)
	if ce502.Tipo != TipoSunat5xx {
		t.Errorf("Se esperaba tipo %s para 502, se obtuvo %s", TipoSunat5xx, ce502.Tipo)
	}

	// 5. respuesta_inesperada (404, 403, 400 otros)
	err404 := &SunatHttpError{
		StatusCode: 404,
		Host:       "api-sire.sunat.gob.pe",
		Duration:   80 * time.Millisecond,
		RawBody:    "Endpoint /v1/desconocido not found",
	}
	ce404 := ClassifyError(err404)
	if ce404.Tipo != TipoRespuestaInesperada {
		t.Errorf("Se esperaba tipo %s para 404, se obtuvo %s", TipoRespuestaInesperada, ce404.Tipo)
	}
}

func TestLogAttempt_FormatAndZeroSecrets(t *testing.T) {
	tempDir := t.TempDir()
	SetAttemptLogDir(tempDir)

	tokenSecreto := "eyJhGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.doNotLeakThisSecret"
	endpointConToken := fmt.Sprintf("/v1/periodos?token=%s", tokenSecreto)

	LogAttempt("20000000001", "GET", endpointConToken, "HTTP_401", 401, 150*time.Millisecond)

	logFile := filepath.Join(tempDir, "sunat_intentos.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Error al leer archivo de log de intentos: %v", err)
	}

	line := string(content)
	if !strings.Contains(line, "RUC=20000000001") {
		t.Errorf("El log no contiene el RUC esperado: %s", line)
	}
	if !strings.Contains(line, "ENDPOINT=GET") {
		t.Errorf("El log no contiene el método y endpoint: %s", line)
	}
	if !strings.Contains(line, "RESULTADO=HTTP_401") || !strings.Contains(line, "CODIGO=401") {
		t.Errorf("El log no contiene el código de resultado: %s", line)
	}
	if !strings.Contains(line, "150ms") {
		t.Errorf("El log no contiene el tiempo de respuesta: %s", line)
	}
	if strings.Contains(line, tokenSecreto) {
		t.Fatalf("¡VIOLACIÓN DE SEGURIDAD!: Se filtró el token secreto en sunat_intentos.log")
	}
}



