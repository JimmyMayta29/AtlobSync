package token

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/SwodLore/sire-api/internal/company"
	"github.com/SwodLore/sire-api/internal/crypto"
	"github.com/SwodLore/sire-api/internal/sunat"
	"github.com/go-resty/resty/v2"
)

var (
	scriptRegex     = regexp.MustCompile(`(?is)<script.*?</script>`)
	styleRegex      = regexp.MustCompile(`(?is)<style.*?</style>`)
	htmlTagRegex    = regexp.MustCompile(`<[^>]*>`)
	multiSpaceRegex = regexp.MustCompile(`\s+`)
)

func stripHTML(input string) string {
	clean := scriptRegex.ReplaceAllString(input, "")
	clean = styleRegex.ReplaceAllString(clean, "")
	clean = htmlTagRegex.ReplaceAllString(clean, " ")
	clean = html.UnescapeString(clean)
	clean = multiSpaceRegex.ReplaceAllString(clean, " ")
	return strings.TrimSpace(clean)
}

type CachedToken struct {
	AccessToken string
	ExpiresAt   time.Time
}

type CooldownInfo struct {
	Until  time.Time
	Reason string
}

type AuthError struct {
	StatusCode int
	ErrorCode  string
	Desc       string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("SUNAT rechazó las credenciales (HTTP %d - %s): %s", e.StatusCode, e.ErrorCode, e.Desc)
}

type TokenManager struct {
	companyRepo *company.Repository
	crypto      *crypto.CryptoService
	tokens      map[string]*CachedToken
	tokensMu    sync.RWMutex
	cooldowns   map[string]*CooldownInfo
	cooldownsMu sync.RWMutex
	rucMutexes  map[string]*sync.Mutex
	rucMapMu    sync.Mutex
	httpClient  *resty.Client
	authBaseURL string
}

func NewTokenManager(repo *company.Repository, cryptoSvc *crypto.CryptoService) *TokenManager {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := resty.New()
	client.SetTransport(transport)
	client.SetTimeout(60 * time.Second)

	return &TokenManager{
		companyRepo: repo,
		crypto:      cryptoSvc,
		tokens:      make(map[string]*CachedToken),
		cooldowns:   make(map[string]*CooldownInfo),
		rucMutexes:  make(map[string]*sync.Mutex),
		httpClient:  client,
		authBaseURL: "https://api-seguridad.sunat.gob.pe",
	}
}

// SetAuthURL permite configurar la URL base de autenticación (para pruebas unitarias con httptest)
func (tm *TokenManager) SetAuthURL(url string) {
	tm.authBaseURL = strings.TrimRight(url, "/")
}

func (tm *TokenManager) getRucMutex(ruc string) *sync.Mutex {
	tm.rucMapMu.Lock()
	defer tm.rucMapMu.Unlock()

	mu, exists := tm.rucMutexes[ruc]
	if !exists {
		mu = &sync.Mutex{}
		tm.rucMutexes[ruc] = mu
	}
	return mu
}

// IsCooldown verifica si el RUC se encuentra en periodo de enfriamiento (15 min) por credenciales inválidas
func (tm *TokenManager) IsCooldown(ruc string) (bool, time.Time, string) {
	tm.cooldownsMu.RLock()
	info, exists := tm.cooldowns[ruc]
	tm.cooldownsMu.RUnlock()

	now := time.Now()
	if exists && info != nil {
		if now.Before(info.Until) {
			return true, info.Until, info.Reason
		}
		// Ya expiró en memoria, limpiar
		tm.cooldownsMu.Lock()
		delete(tm.cooldowns, ruc)
		tm.cooldownsMu.Unlock()
	}

	// Verificar en base de datos si el servidor fue reiniciado durante el periodo de enfriamiento
	if tm.companyRepo != nil {
		emp, err := tm.companyRepo.GetByRUC(ruc)
		if err == nil && emp != nil && emp.BloqueadoHasta != nil {
			if now.Before(*emp.BloqueadoHasta) {
				tm.cooldownsMu.Lock()
				tm.cooldowns[ruc] = &CooldownInfo{
					Until:  *emp.BloqueadoHasta,
					Reason: emp.MensajeAuth,
				}
				tm.cooldownsMu.Unlock()
				return true, *emp.BloqueadoHasta, emp.MensajeAuth
			}
		}
	}

	return false, time.Time{}, ""
}

// SetCooldown registra el periodo de enfriamiento en memoria y en la base de datos
func (tm *TokenManager) SetCooldown(ruc string, duration time.Duration, reason string) {
	until := time.Now().Add(duration)
	tm.cooldownsMu.Lock()
	tm.cooldowns[ruc] = &CooldownInfo{
		Until:  until,
		Reason: reason,
	}
	tm.cooldownsMu.Unlock()

	if tm.companyRepo != nil {
		_ = tm.companyRepo.UpdateAuthStatus(ruc, "INVALIDAS", reason, &until)
	}
	log.Printf("[TOKEN] [ENFRIAMIENTO] RUC %s en enfriamiento de seguridad hasta %s (%s). Se suspenden peticiones a SUNAT.",
		ruc, until.Format("15:04:05"), reason)
}

// ClearCooldown levanta el periodo de enfriamiento tras una actualización de credenciales vía PUT
func (tm *TokenManager) ClearCooldown(ruc string) {
	tm.cooldownsMu.Lock()
	delete(tm.cooldowns, ruc)
	tm.cooldownsMu.Unlock()

	if tm.companyRepo != nil {
		_ = tm.companyRepo.UpdateAuthStatus(ruc, "PENDIENTE", "", nil)
	}
	log.Printf("[TOKEN] [ENFRIAMIENTO] Enfriamiento levantado para RUC %s tras actualizar credenciales.", ruc)
}

// GetToken obtiene un token válido para el RUC especificado (desde caché o solicitándolo a SUNAT)
func (tm *TokenManager) GetToken(ruc string) (string, error) {
	// 0. Protección contra bloqueo: Si está en enfriamiento de 15 minutos, rechazar inmediatamente SIN llamar a SUNAT
	if inCooldown, until, reason := tm.IsCooldown(ruc); inCooldown {
		return "", fmt.Errorf("empresa con RUC %s en enfriamiento de seguridad hasta %s (%s). Actualice las credenciales SOL en Empresas para reintentar",
			ruc, until.Format("15:04:05"), reason)
	}

	// 1. Revisar caché en memoria con lock de lectura
	tm.tokensMu.RLock()
	cached, exists := tm.tokens[ruc]
	if exists && cached != nil {
		// Renovar si quedan menos de 5 minutos de vigencia
		if time.Now().Add(5 * time.Minute).Before(cached.ExpiresAt) {
			token := cached.AccessToken
			tm.tokensMu.RUnlock()
			return token, nil
		}
	}
	tm.tokensMu.RUnlock()

	// 1.1 Revisar caché persistente en SQLite (cifrado en reposo con AES-GCM)
	if tm.companyRepo != nil && tm.companyRepo.DB() != nil && tm.crypto != nil {
		var tokenEnc, expiresAtStr string
		err := tm.companyRepo.DB().QueryRow("SELECT token_enc, expires_at FROM tokens_cache WHERE ruc = ?", ruc).Scan(&tokenEnc, &expiresAtStr)
		if err == nil {
			exp, errParse := parseExpiresAt(expiresAtStr)
			if errParse == nil && time.Now().Add(5*time.Minute).Before(exp) {
				tokenDec, errDec := tm.crypto.Decrypt(tokenEnc)
				if errDec == nil {
					tm.tokensMu.Lock()
					tm.tokens[ruc] = &CachedToken{
						AccessToken: tokenDec,
						ExpiresAt:   exp,
					}
					tm.tokensMu.Unlock()
					return tokenDec, nil
				}
			}
		}
	}

	// 2. Adquirir lock exclusivo para este RUC para evitar peticiones concurrentes duplicadas
	rucMu := tm.getRucMutex(ruc)
	rucMu.Lock()
	defer rucMu.Unlock()

	// Doble chequeo de enfriamiento tras obtener el lock
	if inCooldown, until, reason := tm.IsCooldown(ruc); inCooldown {
		return "", fmt.Errorf("empresa con RUC %s en enfriamiento de seguridad hasta %s (%s). Actualice las credenciales SOL en Empresas para reintentar",
			ruc, until.Format("15:04:05"), reason)
	}

	// Doble chequeo de caché tras obtener el lock
	tm.tokensMu.RLock()
	cached, exists = tm.tokens[ruc]
	if exists && cached != nil && time.Now().Add(5*time.Minute).Before(cached.ExpiresAt) {
		token := cached.AccessToken
		tm.tokensMu.RUnlock()
		return token, nil
	}
	tm.tokensMu.RUnlock()

	// Doble chequeo en SQLite tras obtener el lock (cifrado con AES-GCM)
	if tm.companyRepo != nil && tm.companyRepo.DB() != nil && tm.crypto != nil {
		var tokenEnc, expiresAtStr string
		err := tm.companyRepo.DB().QueryRow("SELECT token_enc, expires_at FROM tokens_cache WHERE ruc = ?", ruc).Scan(&tokenEnc, &expiresAtStr)
		if err == nil {
			exp, errParse := parseExpiresAt(expiresAtStr)
			if errParse == nil && time.Now().Add(5*time.Minute).Before(exp) {
				tokenDec, errDec := tm.crypto.Decrypt(tokenEnc)
				if errDec == nil {
					tm.tokensMu.Lock()
					tm.tokens[ruc] = &CachedToken{
						AccessToken: tokenDec,
						ExpiresAt:   exp,
					}
					tm.tokensMu.Unlock()
					return tokenDec, nil
				}
			}
		}
	}

	// 3. Obtener empresa y descifrar credenciales
	emp, err := tm.companyRepo.GetByRUC(ruc)
	if err != nil {
		return "", fmt.Errorf("no se encontró la empresa con RUC %s: %w", ruc, err)
	}

	if emp.Activo == 0 {
		return "", fmt.Errorf("la empresa con RUC %s está inactiva", ruc)
	}

	claveSOL, err := tm.crypto.Decrypt(emp.ClaveSOLEnc)
	if err != nil {
		return "", fmt.Errorf("error al descifrar clave SOL de la empresa: %w", err)
	}

	clientSecret, err := tm.crypto.Decrypt(emp.ClientSecretEnc)
	if err != nil {
		return "", fmt.Errorf("error al descifrar client_secret de la empresa: %w", err)
	}

	// 4. Solicitar token a SUNAT
	token, expiresIn, err := tm.solicitarTokenSUNAT(emp.ClientID, clientSecret, emp.RUC, emp.UsuarioSOL, claveSOL)
	if err != nil {
		var authErr *AuthError
		if errors.As(err, &authErr) || strings.Contains(strings.ToLower(err.Error()), "invalid_grant") || strings.Contains(strings.ToLower(err.Error()), "autenticacion") {
			// Bloqueo preventivo por 15 minutos en memoria y SQLite
			tm.SetCooldown(ruc, 15*time.Minute, "Credenciales SOL inválidas o vencidas; actualícelas en Empresas")
			return "", fmt.Errorf("credenciales SOL inválidas o vencidas; actualícelas en Empresas (SUNAT: %s)", stripHTML(err.Error()))
		}
		return "", err
	}

	// Si autenticó exitosamente, actualizar estado a VALIDO si no lo estaba
	if emp.EstadoCredenciales != "VALIDO" {
		_ = tm.companyRepo.UpdateAuthStatus(ruc, "VALIDO", "Credenciales autenticadas exitosamente", nil)
	}

	// 5. Guardar en caché (en memoria y en SQLite)
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	tm.tokensMu.Lock()
	tm.tokens[ruc] = &CachedToken{
		AccessToken: token,
		ExpiresAt:   expiresAt,
	}
	tm.tokensMu.Unlock()

	if tm.companyRepo != nil && tm.companyRepo.DB() != nil && tm.crypto != nil {
		tokenEnc, errEnc := tm.crypto.Encrypt(token)
		if errEnc == nil {
			_, _ = tm.companyRepo.DB().Exec(
				"INSERT INTO tokens_cache (ruc, token_enc, expires_at, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(ruc) DO UPDATE SET token_enc = excluded.token_enc, expires_at = excluded.expires_at, updated_at = CURRENT_TIMESTAMP",
				ruc, tokenEnc, expiresAt.UTC().Format(time.RFC3339),
			)
		}
	}

	log.Printf("[TOKEN] Sesión autenticada en SUNAT para RUC %s (Vigencia: %d seg)", ruc, expiresIn)
	return token, nil
}

// InvalidateToken elimina el token en memoria y en SQLite para forzar una renovación en el siguiente intento
func (tm *TokenManager) InvalidateToken(ruc string) {
	tm.tokensMu.Lock()
	delete(tm.tokens, ruc)
	tm.tokensMu.Unlock()

	if tm.companyRepo != nil && tm.companyRepo.DB() != nil {
		_, _ = tm.companyRepo.DB().Exec("DELETE FROM tokens_cache WHERE ruc = ?", ruc)
	}
	log.Printf("[TOKEN] Caché de sesión invalidada para RUC %s", ruc)
}

func (tm *TokenManager) solicitarTokenSUNAT(clientID, clientSecret, ruc, usuarioSOL, claveSOL string) (string, int64, error) {
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	ruc = strings.TrimSpace(ruc)
	usuarioSOL = strings.TrimSpace(usuarioSOL)
	claveSOL = strings.TrimSpace(claveSOL)

	url := fmt.Sprintf("%s/v1/clientessol/%s/oauth2/token/", tm.authBaseURL, clientID)

	formData := map[string]string{
		"grant_type":    "password",
		"scope":         "https://api-sire.sunat.gob.pe",
		"client_id":     clientID,
		"client_secret": clientSecret,
		"username":      fmt.Sprintf("%s%s", ruc, usuarioSOL),
		"password":      claveSOL,
	}

	start := time.Now()
	resp, err := tm.httpClient.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(formData).
		Post(url)
	dur := time.Since(start)

	var statusCode int
	var resultado string
	if err != nil {
		resultado = "ERROR_RED"
	} else {
		statusCode = resp.StatusCode()
		if statusCode == http.StatusOK {
			resultado = "EXITO"
		} else {
			resultado = fmt.Sprintf("HTTP_%d", statusCode)
		}
	}
	sunat.LogAttempt(ruc, "POST", url, resultado, statusCode, dur)

	if err != nil {
		return "", 0, fmt.Errorf("error de red al conectar con SUNAT: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		var errorBody map[string]interface{}
		_ = json.Unmarshal(resp.Body(), &errorBody)
		var desc string
		var errCode string
		if d, ok := errorBody["error_description"].(string); ok {
			desc = d
		} else if e, ok := errorBody["error"].(string); ok {
			desc = e
		} else {
			desc = string(resp.Body())
		}
		if e, ok := errorBody["error"].(string); ok {
			errCode = e
		}

		cleanDesc := stripHTML(desc)
		lower := strings.ToLower(cleanDesc + " " + errCode)

		if strings.Contains(lower, "invalid_grant") ||
			strings.Contains(lower, "autenticacion") ||
			strings.Contains(lower, "usuario") ||
			resp.StatusCode() == http.StatusBadRequest ||
			resp.StatusCode() == http.StatusUnauthorized {
			return "", 0, &AuthError{
				StatusCode: resp.StatusCode(),
				ErrorCode:  errCode,
				Desc:       cleanDesc,
			}
		}

		return "", 0, fmt.Errorf("SUNAT rechazó la autenticación (HTTP %d): %s", resp.StatusCode(), cleanDesc)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &res); err != nil {
		return "", 0, fmt.Errorf("respuesta de SUNAT con formato JSON inválido: %w", err)
	}

	token, ok := res["access_token"].(string)
	if !ok || token == "" {
		return "", 0, errors.New("la respuesta de SUNAT no contiene 'access_token'")
	}

	expiresIn := int64(3600) // Default 1 hora
	if exp, ok := res["expires_in"].(float64); ok && exp > 0 {
		expiresIn = int64(exp)
	}

	return token, expiresIn, nil
}

func parseExpiresAt(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05-07:00", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("formato de fecha no reconocido: %s", s)
}

