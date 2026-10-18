package sunat

import (
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	scriptRegex     = regexp.MustCompile(`(?is)<script.*?</script>`)
	styleRegex      = regexp.MustCompile(`(?is)<style.*?</style>`)
	htmlTagRegex    = regexp.MustCompile(`<[^>]*>`)
	multiSpaceRegex = regexp.MustCompile(`\s+`)
	tokenMaskRegex  = regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9_\-\.]{10,}|(ey[A-Za-z0-9_\-]{20,}\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+)`)
)

// Constantes de tipos de error según especificación
const (
	TipoRed                 = "red"
	TipoCredenciales        = "credenciales"
	TipoGatewaySire         = "gateway_sire"
	TipoSunat5xx            = "sunat_5xx"
	TipoRespuestaInesperada = "respuesta_inesperada"
)

// StripHTML elimina tags HTML, scripts, estilos y entidades de cualquier texto devuelto por SUNAT
func StripHTML(input string) string {
	clean := scriptRegex.ReplaceAllString(input, "")
	clean = styleRegex.ReplaceAllString(clean, "")
	clean = htmlTagRegex.ReplaceAllString(clean, " ")
	clean = html.UnescapeString(clean)
	clean = multiSpaceRegex.ReplaceAllString(clean, " ")
	return strings.TrimSpace(clean)
}

// MaskTokens reemplaza tokens Bearer o JWT que puedan aparecer en mensajes de error por <TOKEN>
func MaskTokens(input string) string {
	return tokenMaskRegex.ReplaceAllString(input, "$1<TOKEN>")
}

// SunatHttpError transporta metadatos completos de una llamada HTTP fallida a SUNAT
type SunatHttpError struct {
	StatusCode int
	Host       string
	Duration   time.Duration
	RawBody    string
	Err        error
	Headers    map[string]string
	Ruta       string
}

func (e *SunatHttpError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("HTTP %d desde %s: %s", e.StatusCode, e.Host, e.RawBody)
}

// ClassifiedError representa la estructura clasificada de error
type ClassifiedError struct {
	StatusCode int               `json:"codigo_http"`
	Tipo       string            `json:"tipo"`
	UserMsg    string            `json:"error"`
	TechDetail string            `json:"detalle"`
	Host       string            `json:"host,omitempty"`
	DurationMS int64             `json:"tiempo_ms,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Ruta       string            `json:"ruta,omitempty"`
}

// isNetworkError detecta si un error proviene de red (DNS, timeout, TLS, proxy)
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if ok := strings.Contains(err.Error(), "no such host") ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "handshake failure") ||
		strings.Contains(err.Error(), "tls:") ||
		strings.Contains(err.Error(), "proxy") ||
		strings.Contains(err.Error(), "dial tcp") ||
		strings.Contains(err.Error(), "deadline exceeded") ||
		strings.Contains(err.Error(), "Client.Timeout") ||
		strings.Contains(err.Error(), "EOF"); ok {
		return true
	}
	if ok := (&net.OpError{}); ok != nil && (strings.Contains(err.Error(), "dial") || strings.Contains(err.Error(), "lookup")) {
		return true
	}
	var urlErr *url.Error
	if errorsAs(err, &urlErr) {
		return true
	}
	if errorsAs(err, &netErr) {
		return true
	}
	return false
}

func errorsAs(err error, target interface{}) bool {
	switch t := target.(type) {
	case **url.Error:
		ue, ok := err.(*url.Error)
		if ok {
			*t = ue
			return true
		}
	case *net.Error:
		ne, ok := err.(net.Error)
		if ok {
			*t = ne
			return true
		}
	}
	return false
}

// ClassifyError clasifica un error en una de las 5 categorías requeridas:
// 1. red: (DNS, timeout, TLS, proxy, con el mensaje de Go)
// 2. credenciales: (invalid_grant en OAuth)
// 3. gateway_sire: (401 del gateway con token vigente; un 401 con HTML de nginx NO es servidor no disponible)
// 4. sunat_5xx: (500/502/503/504 reales)
// 5. respuesta_inesperada: (código y primeros 200 caracteres sin HTML)
func ClassifyError(err error) ClassifiedError {
	if err == nil {
		return ClassifiedError{
			StatusCode: http.StatusOK,
			Tipo:       "",
			UserMsg:    "",
			TechDetail: "",
		}
	}

	var statusCode int
	var host = "api-sire.sunat.gob.pe"
	var duration time.Duration
	var rawBody string
	var headers map[string]string
	var ruta string

	if httpErr, ok := err.(*SunatHttpError); ok {
		statusCode = httpErr.StatusCode
		if httpErr.Host != "" {
			host = httpErr.Host
		}
		duration = httpErr.Duration
		rawBody = httpErr.RawBody
		headers = httpErr.Headers
		ruta = httpErr.Ruta
		if httpErr.Err != nil && rawBody == "" {
			rawBody = httpErr.Err.Error()
		}
	} else {
		rawBody = err.Error()
	}

	cleanRaw := MaskTokens(StripHTML(rawBody))
	lower := strings.ToLower(rawBody)

	// Regla 1: credenciales (invalid_grant en OAuth de api-seguridad o autenticación de usuario)
	if strings.Contains(lower, "invalid_grant") ||
		strings.Contains(lower, "autenticacion del usuario") ||
		strings.Contains(lower, "error en la autenticacion") ||
		strings.Contains(lower, "enfriamiento") {

		if statusCode == 0 {
			statusCode = http.StatusUnauthorized
		}
		userMsg := "Credenciales SOL inválidas o vencidas; actualícelas en Empresas."
		snippet := cleanRaw
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		techDetail := fmt.Sprintf("Host: %s | Código HTTP: %d | Tiempo: %dms | Tipo: %s | Detalle: %s",
			host, statusCode, duration.Milliseconds(), TipoCredenciales, snippet)

		return ClassifiedError{
			StatusCode: statusCode,
			Tipo:       TipoCredenciales,
			UserMsg:    userMsg,
			TechDetail: techDetail,
			Host:       host,
			DurationMS: duration.Milliseconds(),
			Headers:    headers,
			Ruta:       ruta,
		}
	}

	// Regla 2: gateway_sire (401 del gateway con token vigente)
	// ¡CRÍTICO!: Un 401 con HTML de nginx (ej. "401 Authorization Required") NO se clasifica como servidor no disponible.
	if statusCode == 401 ||
		strings.Contains(lower, "401") ||
		strings.Contains(lower, "authorization required") ||
		strings.Contains(lower, "unauthorized") {

		statusCode = http.StatusUnauthorized
		userMsg := "SUNAT aceptó el inicio de sesión pero rechazó el token en SIRE. Puede faltar una habilitación de la cuenta o de la aplicación. Contacte a SUNAT con los datos de abajo."
		snippet := cleanRaw
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		techDetail := fmt.Sprintf("Host: %s | Código HTTP: %d | Tiempo: %dms | Tipo: %s | Detalle: %s",
			host, statusCode, duration.Milliseconds(), TipoGatewaySire, snippet)

		return ClassifiedError{
			StatusCode: statusCode,
			Tipo:       TipoGatewaySire,
			UserMsg:    userMsg,
			TechDetail: techDetail,
			Host:       host,
			DurationMS: duration.Milliseconds(),
			Headers:    headers,
			Ruta:       ruta,
		}
	}

	// Regla 3: red (DNS, timeout, TLS, proxy, con el mensaje de Go)
	if isNetworkError(err) || (statusCode == 0 && (strings.Contains(lower, "dial") || strings.Contains(lower, "lookup") || strings.Contains(lower, "timeout") || strings.Contains(lower, "connection refused") || strings.Contains(lower, "proxy"))) {
		if statusCode == 0 {
			statusCode = http.StatusBadGateway
		}
		userMsg := "Error de conexión de red al comunicarse con SUNAT."
		techDetail := fmt.Sprintf("Host: %s | Código HTTP: %d | Tiempo: %dms | Tipo: %s | Mensaje Go: %s",
			host, statusCode, duration.Milliseconds(), TipoRed, cleanRaw)

		return ClassifiedError{
			StatusCode: statusCode,
			Tipo:       TipoRed,
			UserMsg:    userMsg,
			TechDetail: techDetail,
			Host:       host,
			DurationMS: duration.Milliseconds(),
			Headers:    headers,
			Ruta:       ruta,
		}
	}

	// Regla 4: sunat_5xx (500/502/503/504 reales de SUNAT)
	if (statusCode >= 500 && statusCode <= 504) ||
		(statusCode == 0 && (strings.Contains(lower, "502") || strings.Contains(lower, "503") || strings.Contains(lower, "504") || strings.Contains(lower, "500") || strings.Contains(lower, "bad gateway"))) {

		if statusCode == 0 {
			statusCode = http.StatusBadGateway
		}
		userMsg := "Los servidores de SUNAT no están disponibles o devolvieron un error interno (5xx)."
		snippet := cleanRaw
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		techDetail := fmt.Sprintf("Host: %s | Código HTTP: %d | Tiempo: %dms | Tipo: %s | Detalle: %s",
			host, statusCode, duration.Milliseconds(), TipoSunat5xx, snippet)

		return ClassifiedError{
			StatusCode: statusCode,
			Tipo:       TipoSunat5xx,
			UserMsg:    userMsg,
			TechDetail: techDetail,
			Host:       host,
			DurationMS: duration.Milliseconds(),
			Headers:    headers,
			Ruta:       ruta,
		}
	}

	// Regla 5: respuesta_inesperada (código y primeros 200 caracteres sin HTML)
	if statusCode == 0 {
		statusCode = http.StatusBadRequest
	}
	userMsg := "SUNAT devolvió una respuesta con código o formato no esperado."
	snippet := cleanRaw
	if len(snippet) > 200 {
		snippet = snippet[:200] + "..."
	}
	techDetail := fmt.Sprintf("Host: %s | Código HTTP: %d | Tiempo: %dms | Tipo: %s | Detalle: %s",
		host, statusCode, duration.Milliseconds(), TipoRespuestaInesperada, snippet)

	return ClassifiedError{
		StatusCode: statusCode,
		Tipo:       TipoRespuestaInesperada,
		UserMsg:    userMsg,
		TechDetail: techDetail,
		Host:       host,
		DurationMS: duration.Milliseconds(),
		Headers:    headers,
		Ruta:       ruta,
	}
}

// RespondWithError envía una respuesta JSON estructurada con tipo, detalle y metadatos
func RespondWithError(c *gin.Context, err error) {
	ce := ClassifyError(err)
	resp := gin.H{
		"success":     false,
		"error":       ce.UserMsg,
		"tipo":        ce.Tipo,
		"detalle":     ce.TechDetail,
		"codigo_http": ce.StatusCode,
		"host":        ce.Host,
		"tiempo_ms":   ce.DurationMS,
	}
	if len(ce.Headers) > 0 {
		resp["headers"] = ce.Headers
	}
	if ce.Ruta != "" {
		resp["ruta"] = ce.Ruta
	}
	c.JSON(ce.StatusCode, resp)
}

// SanitizeError mantiene compatibilidad con firmas previas
func SanitizeError(err error) (statusCode int, userMessage string, techDetail string) {
	ce := ClassifyError(err)
	return ce.StatusCode, ce.UserMsg, ce.TechDetail
}

// Sistema de registro de intentos (Task 5): data/logs/sunat_intentos.log
var (
	attemptLogMu   sync.Mutex
	attemptLogPath string
)

func SetAttemptLogDir(dir string) {
	attemptLogMu.Lock()
	defer attemptLogMu.Unlock()
	_ = os.MkdirAll(dir, 0755)
	attemptLogPath = filepath.Join(dir, "sunat_intentos.log")
}

// LogAttempt registra una línea por intento con hora, RUC, endpoint, tipo de resultado y código, sin secretos
func LogAttempt(ruc, method, endpoint, resultado string, statusCode int, duration time.Duration) {
	attemptLogMu.Lock()
	defer attemptLogMu.Unlock()

	if attemptLogPath == "" {
		attemptLogPath = filepath.Join("data", "logs", "sunat_intentos.log")
	}
	_ = os.MkdirAll(filepath.Dir(attemptLogPath), 0755)

	f, err := os.OpenFile(attemptLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	now := time.Now().Format("2006-01-02 15:04:05")
	cleanEndpoint := MaskTokens(endpoint)
	line := fmt.Sprintf("[%s] RUC=%s | ENDPOINT=%s %s | RESULTADO=%s | CODIGO=%d | TIEMPO=%dms\n",
		now, ruc, method, cleanEndpoint, resultado, statusCode, duration.Milliseconds())
	_, _ = f.WriteString(line)
}

// FilterResponseHeaders limpia headers sensibles (cookies) para soporte técnico
func FilterResponseHeaders(h http.Header) map[string]string {
	res := make(map[string]string)
	for k := range h {
		lowerK := strings.ToLower(k)
		if strings.Contains(lowerK, "cookie") {
			continue
		}
		res[k] = h.Get(k)
	}
	return res
}
