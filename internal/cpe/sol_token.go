package cpe

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	// URL oficial de inicio de sesión OAuth Menú SOL de SUNAT (obtenida desde MenuInternet.htm)
	defaultSunatSolLoginURL = "https://api-seguridad.sunat.gob.pe/v1/clientessol/4f3b88b3-d9d6-402a-b85d-6a0bc857746a/oauth2/authen?redirect_uri=https://e-menu.sunat.gob.pe/cl-ti-itmenu/AutenticaMenuInternet.htm&state=rO0ABXNyABFqYXZhLnV0aWwuSGFzaE1hcAUH2sHDFmDRAwACRgAKbG9hZEZhY3RvckkACXRocmVzaG9sZHhwP0AAAAAAAAx3CAAAABAAAAADdAADZXhlcHQABnBhcmFtc3QASyomKiYvY2wtdGktaXRtZW51L01lbnVJbnRlcm5ldC5odG0mYjY0ZDI2YThiNWFmMDkxOTIzYjIzYjY0MDdhMWMxZGI0MWU3MzNhNnQABGV4ZWNweA==&client_id=4f3b88b3-d9d6-402a-b85d-6a0bc857746a&response_type=code"
	// URL directa al módulo Consulta de Comprobantes de Pago dentro del Menú SOL (emite SUNAT.token)
	defaultSunatConsultaMenuURL = "https://e-menu.sunat.gob.pe/cl-ti-itmenu/MenuInternet.htm?action=execute&code=11.38.1.1.1&s=ww1"
	defaultSolUserAgent         = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

var (
	// ErrSolCaptchaRequired indica que SUNAT mostró un reto CAPTCHA en el formulario de acceso.
	ErrSolCaptchaRequired = errors.New("SUNAT solicitó un CAPTCHA en el acceso SOL (por intentos previos fallidos o control de seguridad); inicie sesión manualmente una vez en el portal SOL para restablecerlo")
	// ErrSolAuthRejected indica que SUNAT rechazó el RUC, Usuario SOL o Clave SOL.
	ErrSolAuthRejected = errors.New("SUNAT rechazó las credenciales SOL proporcionadas")
	// ErrSolBrowserMissing indica que no se encontró Edge ni Chrome instalado en el equipo.
	ErrSolBrowserMissing = errors.New("no se encontró Microsoft Edge ni Google Chrome en el equipo (puede definir la variable CHROME_PATH)")
	// ErrSolCooldown indica que el acceso SOL está en pausa temporal por seguridad tras un error de credenciales o CAPTCHA.
	ErrSolCooldown = errors.New("acceso SOL en pausa temporal por seguridad")
)

func maskRUCForLog(ruc string) string {
	r := strings.TrimSpace(ruc)
	if len(r) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(r)-4) + r[len(r)-4:]
}

// SolTokenProvider abstrae la obtención del token Bearer de sesión SOL (permite mocks en tests).
type SolTokenProvider interface {
	GetToken(ctx context.Context, ruc, usuarioSOL, claveSOL string, forceRefresh bool) (string, error)
	Invalidate(ruc string)
}

// SolSessionStatus describe el estado local de la sesión Portal SOL (api-cpe) para una empresa,
// sin exponer nunca el valor del token JWT ni credenciales.
type SolSessionStatus struct {
	CredencialesSOLConfiguradas bool   `json:"credenciales_sol_configuradas"`
	BrowserEncontrado           bool   `json:"browser_encontrado"`
	BrowserNombre               string `json:"browser_nombre,omitempty"`
	TokenVigente                bool   `json:"token_vigente"`
	ExpiraEn                    string `json:"expira_en,omitempty"`
	MinutosRestantes            int    `json:"minutos_restantes,omitempty"`
	EnCooldown                  bool   `json:"en_cooldown"`
	CooldownHasta               string `json:"cooldown_hasta,omitempty"`
	CooldownMotivo              string `json:"cooldown_motivo,omitempty"`
	Estado                      string `json:"estado"`
	Mensaje                     string `json:"mensaje"`
}

type cachedSolToken struct {
	Token     string
	ExpiresAt time.Time
}

type solCooldown struct {
	Until  time.Time
	Reason string
}

// SolTokenManager gestiona la autenticación SOL por RUC usando 1 sola sesión headless Edge/Chrome
// y cachea el JWT resultante (SUNAT.token) para realizar cientos de descargas vía HTTP directo.
type SolTokenManager struct {
	mu           sync.RWMutex
	tokens       map[string]*cachedSolToken
	cooldowns    map[string]*solCooldown
	rucLocks     map[string]*sync.Mutex
	loginURL     string
	menuURL      string
	loginTimeout time.Duration
	headless     bool
}

func NewSolTokenManager() *SolTokenManager {
	headless := true
	if strings.EqualFold(strings.TrimSpace(os.Getenv("SUNAT_SOL_HEADLESS")), "false") {
		headless = false
	}
	return &SolTokenManager{
		tokens:       make(map[string]*cachedSolToken),
		cooldowns:    make(map[string]*solCooldown),
		rucLocks:     make(map[string]*sync.Mutex),
		loginURL:     defaultSunatSolLoginURL,
		menuURL:      defaultSunatConsultaMenuURL,
		loginTimeout: 75 * time.Second,
		headless:     headless,
	}
}

func (m *SolTokenManager) getRucLock(ruc string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lk, ok := m.rucLocks[ruc]
	if !ok {
		lk = &sync.Mutex{}
		m.rucLocks[ruc] = lk
	}
	return lk
}

func (m *SolTokenManager) Invalidate(ruc string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, strings.TrimSpace(ruc))
}

// ClearCooldown limpia la pausa temporal SOL de un RUC (usado en pruebas explícitas iniciadas por el usuario).
func (m *SolTokenManager) ClearCooldown(ruc string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cooldowns, strings.TrimSpace(ruc))
}

// Status inspecciona localmente el estado de navegador, token en caché y pausa SOL sin contactar a SUNAT.
func (m *SolTokenManager) Status(ruc string) SolSessionStatus {
	ruc = strings.TrimSpace(ruc)
	st := SolSessionStatus{
		CredencialesSOLConfiguradas: true,
	}

	if execPath := findChromiumExecutable(); execPath != "" {
		st.BrowserEncontrado = true
		st.BrowserNombre = filepath.Base(execPath)
	}

	now := time.Now()
	m.mu.RLock()
	if cd, ok := m.cooldowns[ruc]; ok && now.Before(cd.Until) {
		st.EnCooldown = true
		st.CooldownHasta = cd.Until.Format("15:04:05")
		st.CooldownMotivo = cd.Reason
	}
	if cached, ok := m.tokens[ruc]; ok && cached.Token != "" && now.Before(cached.ExpiresAt) {
		st.TokenVigente = true
		st.ExpiraEn = cached.ExpiresAt.Format("15:04:05")
		mins := int(time.Until(cached.ExpiresAt).Minutes())
		if mins < 1 {
			mins = 1
		}
		st.MinutosRestantes = mins
	}
	m.mu.RUnlock()

	switch {
	case !st.BrowserEncontrado:
		st.Estado = "SIN_NAVEGADOR"
		st.Mensaje = "No se encontró Microsoft Edge ni Google Chrome instalado en el equipo (puede definir CHROME_PATH)."
	case st.EnCooldown:
		st.Estado = "COOLDOWN"
		st.Mensaje = fmt.Sprintf("Acceso SOL en pausa temporal hasta %s por seguridad.", st.CooldownHasta)
	case st.TokenVigente:
		st.Estado = "SESION_ACTIVA"
		st.Mensaje = fmt.Sprintf("Sesión Portal SOL activa (válida hasta %s, ~%d min restantes).", st.ExpiraEn, st.MinutosRestantes)
	default:
		st.Estado = "LISTO"
		st.Mensaje = "Navegador local detectado. La sesión SOL se abrirá automáticamente al descargar un comprobante o al probar aquí."
	}
	return st
}

// GetToken retorna un JWT Bearer válido para api-cpe.sunat.gob.pe (reutilizando caché si está vigente).
func (m *SolTokenManager) GetToken(ctx context.Context, ruc, usuarioSOL, claveSOL string, forceRefresh bool) (string, error) {
	ruc = strings.TrimSpace(ruc)
	usuarioSOL = strings.TrimSpace(usuarioSOL)
	claveSOL = strings.TrimSpace(claveSOL)
	if ruc == "" || usuarioSOL == "" || claveSOL == "" {
		return "", errors.New("credenciales SOL incompletas (RUC, Usuario SOL y Clave SOL son obligatorios)")
	}

	// Lectura rápida desde caché
	if !forceRefresh {
		m.mu.RLock()
		if cached, ok := m.tokens[ruc]; ok && cached.Token != "" && time.Now().Before(cached.ExpiresAt) {
			tok := cached.Token
			m.mu.RUnlock()
			return tok, nil
		}
		m.mu.RUnlock()
	}

	// Serializar logins por RUC para evitar sesiones concurrentes
	lk := m.getRucLock(ruc)
	lk.Lock()
	defer lk.Unlock()

	// Double-check tras adquirir el lock
	if !forceRefresh {
		m.mu.RLock()
		if cached, ok := m.tokens[ruc]; ok && cached.Token != "" && time.Now().Before(cached.ExpiresAt) {
			tok := cached.Token
			m.mu.RUnlock()
			return tok, nil
		}
		m.mu.RUnlock()
	}

	// Verificar cooldown activo (credenciales inválidas o CAPTCHA reciente)
	m.mu.RLock()
	if cd, ok := m.cooldowns[ruc]; ok && time.Now().Before(cd.Until) {
		remaining := time.Until(cd.Until).Round(time.Second)
		reason := cd.Reason
		m.mu.RUnlock()
		return "", fmt.Errorf("%w (%s restantes): %s", ErrSolCooldown, remaining, reason)
	}
	m.mu.RUnlock()

	tok, err := m.loginAndCaptureToken(ctx, ruc, usuarioSOL, claveSOL)
	if err != nil {
		if errors.Is(err, ErrSolCaptchaRequired) || errors.Is(err, ErrSolAuthRejected) {
			m.mu.Lock()
			m.cooldowns[ruc] = &solCooldown{
				Until:  time.Now().Add(10 * time.Minute),
				Reason: err.Error(),
			}
			m.mu.Unlock()
		}
		return "", err
	}

	// Limpiar cooldown si el login tuvo éxito
	exp := parseJWTExpiry(tok)
	if exp.IsZero() || time.Until(exp) < 2*time.Minute {
		exp = time.Now().Add(25 * time.Minute)
	} else {
		exp = exp.Add(-90 * time.Second)
	}

	m.mu.Lock()
	delete(m.cooldowns, ruc)
	m.tokens[ruc] = &cachedSolToken{
		Token:     tok,
		ExpiresAt: exp,
	}
	m.mu.Unlock()

	log.Printf("[CPE-SOL-AUTH] Token SUNAT.token obtenido para RUC %s (válido hasta %s)", maskRUCForLog(ruc), exp.Format("15:04:05"))
	return tok, nil
}

func (m *SolTokenManager) loginAndCaptureToken(parentCtx context.Context, ruc, usuarioSOL, claveSOL string) (string, error) {
	execPath := findChromiumExecutable()
	if execPath == "" {
		return "", ErrSolBrowserMissing
	}

	tmpProfile, err := os.MkdirTemp("", "sire-sol-profile-*")
	if err != nil {
		return "", fmt.Errorf("error creando perfil temporal de navegador: %w", err)
	}
	defer os.RemoveAll(tmpProfile)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(execPath),
		chromedp.UserDataDir(tmpProfile),
		chromedp.UserAgent(defaultSolUserAgent),
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.WindowSize(1280, 860),
	)
	if !m.headless {
		opts = append(opts, chromedp.VisibleWindow)
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parentCtx, opts...)
	defer cancelAlloc()

	runCtx, cancelRun := chromedp.NewContext(allocCtx)
	defer cancelRun()

	timeoutCtx, cancelTimeout := context.WithTimeout(runCtx, m.loginTimeout)
	defer cancelTimeout()

	// Capturar Authorization: Bearer de tráfico de red dirigido a api-cpe / e-factura
	var capturedMu sync.Mutex
	var capturedToken string
	setCaptured := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" {
			return
		}
		capturedMu.Lock()
		if capturedToken == "" {
			capturedToken = t
		}
		capturedMu.Unlock()
	}
	getCaptured := func() string {
		capturedMu.Lock()
		defer capturedMu.Unlock()
		return capturedToken
	}

	if err := chromedp.Listen(timeoutCtx, func(ev any) {
		if reqEv, ok := ev.(*network.EventRequestWillBeSent); ok && reqEv.Request != nil {
			reqURL := strings.ToLower(reqEv.Request.URL)
			if !strings.Contains(reqURL, "sunat.gob.pe") {
				return
			}
			for k, v := range reqEv.Request.Headers {
				if strings.EqualFold(k, "Authorization") {
					if authStr, ok := v.(string); ok && strings.HasPrefix(authStr, "Bearer ") {
						tok := strings.TrimSpace(strings.TrimPrefix(authStr, "Bearer "))
						if strings.Count(tok, ".") == 2 {
							setCaptured(tok)
						}
					}
				}
			}
		}
	}); err != nil {
		return "", fmt.Errorf("no se pudo inicializar el navegador headless (%s): %w", filepath.Base(execPath), err)
	}

	log.Printf("[CPE-SOL-AUTH] Iniciando sesión SOL para RUC %s (navegador: %s)", maskRUCForLog(ruc), filepath.Base(execPath))

	// 1. Habilitar red, abrir formulario de login SOL y esperar campos visibles
	if _, err := chromedp.Call(timeoutCtx, network.Enable, network.EnableParams{}); err != nil {
		return "", fmt.Errorf("no se pudo habilitar monitoreo de red del navegador: %w", err)
	}

	err = chromedp.Do(timeoutCtx,
		chromedp.Navigate(m.loginURL),
		chromedp.WaitVisible(chromedp.ID("txtRuc")),
		chromedp.WaitVisible(chromedp.ID("txtUsuario")),
		chromedp.WaitVisible(chromedp.ID("txtContrasena")),
		chromedp.WaitVisible(chromedp.ID("btnAceptar")),
	)
	if err != nil {
		return "", fmt.Errorf("no se pudo cargar el formulario de login SOL de SUNAT: %w", err)
	}

	// Esperar a que jQuery vincule el evento click al botón #btnAceptar y enviar credenciales
	readyDeadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(readyDeadline) {
		ready, _ := chromedp.Run(timeoutCtx, chromedp.Evaluate[bool](`(() => {
			const btn = document.getElementById('btnAceptar');
			if (!btn || !window.jQuery) return false;
			const ev = jQuery._data && jQuery._data(btn, 'events');
			return Boolean(ev && ev.click);
		})()`))
		if ready {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	err = chromedp.Do(timeoutCtx,
		chromedp.SetValue(chromedp.ID("txtRuc"), ruc),
		chromedp.SetValue(chromedp.ID("txtUsuario"), usuarioSOL),
		chromedp.SetValue(chromedp.ID("txtContrasena"), claveSOL),
	)
	if err != nil {
		return "", fmt.Errorf("error completando credenciales en el formulario SOL: %w", err)
	}

	_, _ = chromedp.Run(timeoutCtx, chromedp.Evaluate[any](`(() => {
		for (const id of ['txtRuc', 'txtUsuario', 'txtContrasena']) {
			const el = document.getElementById(id);
			if (el) {
				el.dispatchEvent(new Event('input', { bubbles: true }));
				el.dispatchEvent(new Event('change', { bubbles: true }));
			}
		}
		return true;
	})()`))

	if err := chromedp.Do(timeoutCtx, chromedp.Click(chromedp.ID("btnAceptar"))); err != nil {
		return "", fmt.Errorf("error enviando formulario SOL: %w", err)
	}

	// 2. Esperar resultado del login: menú principal, error visible o CAPTCHA
	loginDeadline := time.Now().Add(28 * time.Second)
	loggedIn := false
	for time.Now().Before(loginDeadline) {
		if timeoutCtx.Err() != nil {
			return "", fmt.Errorf("tiempo de espera agotado durante login SOL: %w", timeoutCtx.Err())
		}

		curURL, _ := chromedp.Run(timeoutCtx, chromedp.Location())
		captchaVisible, _ := chromedp.Run(timeoutCtx, chromedp.Evaluate[bool](`(() => {
			const el = document.getElementById('txtCaptcha');
			if (!el) return false;
			const s = window.getComputedStyle(el);
			return Boolean(s && s.display !== 'none' && s.visibility !== 'hidden' && el.offsetParent !== null);
		})()`))
		errMsg, _ := chromedp.Run(timeoutCtx, chromedp.Evaluate[string](`(() => {
			for (const sel of ['#spanMensajeError', '#divMensajeError', '#lblMensaje', '.mensajeError']) {
				const el = document.querySelector(sel);
				if (el && el.offsetParent !== null) {
					const t = (el.innerText || el.textContent || '').trim();
					if (t) return t;
				}
			}
			return '';
		})()`))

		if isSunatMenuURL(curURL) {
			loggedIn = true
			break
		}
		if captchaVisible {
			return "", ErrSolCaptchaRequired
		}
		if errMsg != "" {
			return "", fmt.Errorf("%w: %s", ErrSolAuthRejected, errMsg)
		}

		time.Sleep(500 * time.Millisecond)
	}

	if !loggedIn {
		return "", errors.New("no se alcanzó el Menú SOL tras iniciar sesión (verifique RUC, Usuario SOL, Clave SOL o disponibilidad de SUNAT)")
	}

	// 3. Cerrar posibles modales informativos de campaña y navegar al módulo Consulta de Comprobantes (11.38.1.1.1)
	_ = chromedp.Do(timeoutCtx, chromedp.Sleep(1200*time.Millisecond))
	_, _ = chromedp.Run(timeoutCtx, chromedp.Evaluate[any](`(() => {
		for (const id of ['divModalCampana', 'divModalCampanaBak']) {
			const el = document.getElementById(id);
			if (el) { el.style.display = 'none'; el.classList.remove('in'); }
		}
		document.querySelectorAll('.modal-backdrop').forEach(e => e.remove());
		return true;
	})()`))
	_ = chromedp.Do(timeoutCtx, chromedp.Navigate(m.menuURL))

	// 4. Esperar a capturar SUNAT.token (desde storage o interceptado en red)
	tokenDeadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(tokenDeadline) {
		if timeoutCtx.Err() != nil {
			break
		}
		if tok := getCaptured(); tok != "" {
			return tok, nil
		}

		storageToken, _ := chromedp.Run(timeoutCtx, chromedp.Evaluate[string](`(() => {
			const keys = ['SUNAT.token', 'token', 'access_token'];
			for (const k of keys) {
				const v = sessionStorage.getItem(k) || localStorage.getItem(k);
				if (v && v.split('.').length === 3) return v;
			}
			const ifr = document.getElementById('iframeApplication');
			if (ifr && ifr.contentWindow) {
				try {
					for (const k of keys) {
						const v = ifr.contentWindow.sessionStorage.getItem(k) || ifr.contentWindow.localStorage.getItem(k);
						if (v && v.split('.').length === 3) return v;
					}
				} catch (e) {}
			}
			return '';
		})()`))

		storageToken = strings.TrimSpace(strings.TrimPrefix(storageToken, "Bearer "))
		if strings.Count(storageToken, ".") == 2 {
			return storageToken, nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	if tok := getCaptured(); tok != "" {
		return tok, nil
	}

	return "", errors.New("se inició sesión en SUNAT SOL pero no se pudo capturar SUNAT.token desde el módulo de Consulta de Comprobantes (11.38.1.1.1); verifique que el usuario SOL tenga permisos para Comprobantes de Pago")
}

func isSunatMenuURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(u.Path)
	return strings.HasPrefix(host, "e-menu") &&
		strings.HasSuffix(host, "sunat.gob.pe") &&
		strings.Contains(path, "menu") &&
		!strings.Contains(path, "autentica")
}

func parseJWTExpiry(jwt string) time.Time {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return time.Time{}
	}
	payload := parts[1]
	if mod := len(payload) % 4; mod != 0 {
		payload += strings.Repeat("=", 4-mod)
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}

// ExtractRUCFromJWT extrae el RUC autenticado del payload del JWT SUNAT.token (si está presente).
func ExtractRUCFromJWT(jwt string) string {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return ""
	}
	payload := parts[1]
	if mod := len(payload) % 4; mod != 0 {
		payload += strings.Repeat("=", 4-mod)
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}
	var claims struct {
		Sub      string `json:"sub"`
		UserData struct {
			NumRUC string `json:"numRUC"`
		} `json:"userdata"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}
	if len(claims.UserData.NumRUC) == 11 {
		return claims.UserData.NumRUC
	}
	if len(claims.Sub) == 11 {
		return claims.Sub
	}
	return ""
}

// findChromiumExecutable localiza Chrome o Microsoft Edge instalado en el sistema (0 descargas externas).
func findChromiumExecutable() string {
	for _, envKey := range []string{"CHROME_PATH", "EDGE_PATH", "BROWSER_PATH"} {
		if p := strings.TrimSpace(os.Getenv(envKey)); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	var candidates []string
	switch runtime.GOOS {
	case "windows":
		pf := os.Getenv("ProgramFiles")
		pf86 := os.Getenv("ProgramFiles(x86)")
		localApp := os.Getenv("LOCALAPPDATA")
		candidates = []string{
			filepath.Join(pf, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(pf86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localApp, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(pf86, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(pf, "Microsoft", "Edge", "Application", "msedge.exe"),
		}
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	default:
		for _, bin := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge"} {
			if p, err := exec.LookPath(bin); err == nil {
				return p
			}
		}
	}

	for _, c := range candidates {
		if c != "" {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	return ""
}
