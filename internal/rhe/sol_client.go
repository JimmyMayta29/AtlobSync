package rhe

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

	"github.com/chromedp/chromedp"
)

const (
	defaultSunatSolLoginURL  = "https://api-seguridad.sunat.gob.pe/v1/clientessol/4f3b88b3-d9d6-402a-b85d-6a0bc857746a/oauth2/authen?redirect_uri=https://e-menu.sunat.gob.pe/cl-ti-itmenu/AutenticaMenuInternet.htm&state=rO0ABXNyABFqYXZhLnV0aWwuSGFzaE1hcAUH2sHDFmDRAwACRgAKbG9hZEZhY3RvckkACXRocmVzaG9sZHhwP0AAAAAAAAx3CAAAABAAAAADdAADZXhlcHQABnBhcmFtc3QASyomKiYvY2wtdGktaXRtZW51L01lbnVJbnRlcm5ldC5odG0mYjY0ZDI2YThiNWFmMDkxOTIzYjIzYjY0MDdhMWMxZGI0MWU3MzNhNnQABGV4ZWNweA==&client_id=4f3b88b3-d9d6-402a-b85d-6a0bc857746a&response_type=code"
	defaultRHEReceptorURL    = "https://e-menu.sunat.gob.pe/cl-ti-itmenu/MenuInternet.htm?action=execute&code=11.5.1.1.14&s=ww1"
	defaultRHEServletURL     = "https://ww1.sunat.gob.pe/ol-ti-itreciboelectronico/cpelec001Alias"
	defaultSolUserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	rheConsultaTimeout       = 95 * time.Second
	rheCooldownOnAuthFailure = 10 * time.Minute
)

var (
	// ErrSolCaptchaRequired indica que SUNAT mostró un reto CAPTCHA en el acceso SOL.
	ErrSolCaptchaRequired = errors.New("SUNAT solicitó un CAPTCHA en el acceso SOL; inicie sesión manualmente una vez en el portal SOL para restablecerlo")
	// ErrSolAuthRejected indica que SUNAT rechazó las credenciales SOL.
	ErrSolAuthRejected = errors.New("SUNAT rechazó las credenciales SOL proporcionadas")
	// ErrSolBrowserMissing indica que no se encontró Microsoft Edge ni Google Chrome instalado.
	ErrSolBrowserMissing = errors.New("no se encontró Microsoft Edge ni Google Chrome en el equipo (puede definir la variable CHROME_PATH)")
	// ErrSolCooldown indica que el acceso SOL está temporalmente en pausa tras un error previo.
	ErrSolCooldown = errors.New("acceso SOL RHE en pausa temporal por seguridad")
)

var peruTimezone = time.FixedZone("PET", -5*3600)

type solCooldown struct {
	Until  time.Time
	Reason string
}

// LocalSOLReceptorClient implementa ReceptorProvider en modo 100% local (costo S/ 0)
// conectándose directamente al módulo oficial Consulta Receptor (11.5.1.1.14) del Portal SOL SUNAT.
type LocalSOLReceptorClient struct {
	mu           sync.RWMutex
	cooldowns    map[string]*solCooldown
	rucLocks     map[string]*sync.Mutex
	loginURL     string
	receptorURL  string
	servletURL   string
	queryTimeout time.Duration
	headless     bool
	nowFn        func() time.Time
}

func NewLocalSOLReceptorClient() *LocalSOLReceptorClient {
	headless := true
	if strings.EqualFold(strings.TrimSpace(os.Getenv("SUNAT_SOL_HEADLESS")), "false") {
		headless = false
	}
	return &LocalSOLReceptorClient{
		cooldowns:    make(map[string]*solCooldown),
		rucLocks:     make(map[string]*sync.Mutex),
		loginURL:     defaultSunatSolLoginURL,
		receptorURL:  defaultRHEReceptorURL,
		servletURL:   defaultRHEServletURL,
		queryTimeout: rheConsultaTimeout,
		headless:     headless,
		nowFn:        time.Now,
	}
}

func (c *LocalSOLReceptorClient) getRucLock(ruc string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	lk, ok := c.rucLocks[ruc]
	if !ok {
		lk = &sync.Mutex{}
		c.rucLocks[ruc] = lk
	}
	return lk
}

// ClearCooldown limpia la pausa temporal del RUC (usado cuando el usuario corrige credenciales o reintenta).
func (c *LocalSOLReceptorClient) ClearCooldown(ruc string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cooldowns, strings.TrimSpace(ruc))
}

// Status reporta el estado local de disponibilidad (navegador + cooldown) sin realizar ninguna conexión a SUNAT.
func (c *LocalSOLReceptorClient) Status(ruc string) SolRHEStatus {
	ruc = strings.TrimSpace(ruc)
	st := SolRHEStatus{}

	if execPath := findChromiumExecutable(); execPath != "" {
		st.BrowserEncontrado = true
		st.BrowserNombre = filepath.Base(execPath)
	}

	now := c.nowFn()
	c.mu.RLock()
	if cd, ok := c.cooldowns[ruc]; ok && now.Before(cd.Until) {
		st.EnCooldown = true
		st.CooldownHasta = cd.Until.Format("15:04:05")
		st.CooldownMotivo = cd.Reason
	}
	c.mu.RUnlock()

	switch {
	case !st.BrowserEncontrado:
		st.Estado = "SIN_NAVEGADOR"
		st.Mensaje = "No se encontró Microsoft Edge ni Google Chrome en el equipo (puede definir CHROME_PATH)."
	case st.EnCooldown:
		st.Estado = "COOLDOWN"
		st.Mensaje = fmt.Sprintf("Consulta SOL en pausa temporal hasta %s por seguridad.", st.CooldownHasta)
	default:
		st.Estado = "LISTO"
		st.Mensaje = "Modo Local listo (Portal SOL · Consulta Receptor · Costo S/ 0)."
	}
	return st
}

// ComputePeriodoDateRange calcula el rango DD/MM/YYYY para un periodo YYYYMM respetando el límite
// de SUNAT (máximo 1 mes / no fechas futuras respecto a hoy en hora Perú UTC-5).
func ComputePeriodoDateRange(periodo string, now time.Time) (string, string, error) {
	year, month, err := ValidatePeriodo(periodo)
	if err != nil {
		return "", "", err
	}

	nowPeru := now.In(peruTimezone)
	today := time.Date(nowPeru.Year(), nowPeru.Month(), nowPeru.Day(), 0, 0, 0, 0, peruTimezone)
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, peruTimezone)
	lastDay := firstDay.AddDate(0, 1, -1)

	if firstDay.After(today) {
		return "", "", fmt.Errorf("el periodo %s aún no ha iniciado (fecha actual en Perú: %s)", periodo, today.Format("02/01/2006"))
	}
	if lastDay.After(today) {
		lastDay = today
	}

	return firstDay.Format("02/01/2006"), lastDay.Format("02/01/2006"), nil
}

// ConsultarPeriodo consulta en SUNAT SOL (Consulta Receptor 11.5.1.1.14) todos los Recibos por Honorarios
// recibidos por la empresa durante el periodo YYYYMM y descarga el reporte TXT oficial completo.
func (c *LocalSOLReceptorClient) ConsultarPeriodo(ctx context.Context, ruc, usuarioSOL, claveSOL, periodo string) (*ReceptorQueryResult, error) {
	ruc = strings.TrimSpace(ruc)
	usuarioSOL = strings.TrimSpace(usuarioSOL)
	claveSOL = strings.TrimSpace(claveSOL)

	if ruc == "" || usuarioSOL == "" || claveSOL == "" {
		return nil, errors.New("credenciales SOL incompletas (RUC, Usuario SOL y Clave SOL son obligatorios)")
	}

	fechaDesde, fechaHasta, err := ComputePeriodoDateRange(periodo, c.nowFn())
	if err != nil {
		return nil, err
	}

	lk := c.getRucLock(ruc)
	lk.Lock()
	defer lk.Unlock()

	c.mu.RLock()
	if cd, ok := c.cooldowns[ruc]; ok && c.nowFn().Before(cd.Until) {
		remaining := time.Until(cd.Until).Round(time.Second)
		reason := cd.Reason
		c.mu.RUnlock()
		return nil, fmt.Errorf("%w (%s restantes): %s", ErrSolCooldown, remaining, reason)
	}
	c.mu.RUnlock()

	res, err := c.runSOLQuery(ctx, ruc, usuarioSOL, claveSOL, fechaDesde, fechaHasta)
	if err != nil {
		if errors.Is(err, ErrSolCaptchaRequired) || errors.Is(err, ErrSolAuthRejected) {
			c.mu.Lock()
			c.cooldowns[ruc] = &solCooldown{
				Until:  c.nowFn().Add(rheCooldownOnAuthFailure),
				Reason: err.Error(),
			}
			c.mu.Unlock()
		}
		return nil, err
	}

	c.ClearCooldown(ruc)
	return res, nil
}

func (c *LocalSOLReceptorClient) runSOLQuery(parentCtx context.Context, ruc, usuarioSOL, claveSOL, fechaDesde, fechaHasta string) (*ReceptorQueryResult, error) {
	execPath := findChromiumExecutable()
	if execPath == "" {
		return nil, ErrSolBrowserMissing
	}

	tmpProfile, err := os.MkdirTemp("", "atlob-rhe-sol-*")
	if err != nil {
		return nil, fmt.Errorf("error creando perfil temporal de navegador: %w", err)
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
		chromedp.WindowSize(1280, 900),
	)
	if !c.headless {
		opts = append(opts, chromedp.VisibleWindow)
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parentCtx, opts...)
	defer cancelAlloc()

	runCtx, cancelRun := chromedp.NewContext(allocCtx)
	defer cancelRun()

	timeoutCtx, cancelTimeout := context.WithTimeout(runCtx, c.queryTimeout)
	defer cancelTimeout()

	log.Printf("[RHE-SOL] Iniciando consulta local Receptor para RUC %s (%s al %s · navegador: %s)",
		maskRUCForLog(ruc), fechaDesde, fechaHasta, filepath.Base(execPath))

	// 1. Abrir formulario de login OAuth2 SOL
	if err := chromedp.Do(timeoutCtx,
		chromedp.Navigate(c.loginURL),
		chromedp.WaitVisible(chromedp.ID("txtRuc")),
		chromedp.WaitVisible(chromedp.ID("txtUsuario")),
		chromedp.WaitVisible(chromedp.ID("txtContrasena")),
		chromedp.WaitVisible(chromedp.ID("btnAceptar")),
	); err != nil {
		return nil, fmt.Errorf("no se pudo cargar el formulario de login SOL de SUNAT: %w", err)
	}

	// Esperar enlace jQuery del botón Aceptar
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

	if err := chromedp.Do(timeoutCtx,
		chromedp.SetValue(chromedp.ID("txtRuc"), ruc),
		chromedp.SetValue(chromedp.ID("txtUsuario"), usuarioSOL),
		chromedp.SetValue(chromedp.ID("txtContrasena"), claveSOL),
	); err != nil {
		return nil, fmt.Errorf("error completando credenciales SOL: %w", err)
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
		return nil, fmt.Errorf("error enviando formulario SOL: %w", err)
	}

	// 2. Esperar llegada a Menú SOL, detección de CAPTCHA o rechazo de credenciales
	loginDeadline := time.Now().Add(28 * time.Second)
	loggedIn := false
	for time.Now().Before(loginDeadline) {
		if timeoutCtx.Err() != nil {
			return nil, fmt.Errorf("tiempo de espera agotado durante login SOL: %w", timeoutCtx.Err())
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
			return nil, ErrSolCaptchaRequired
		}
		if errMsg != "" {
			return nil, fmt.Errorf("%w: %s", ErrSolAuthRejected, errMsg)
		}

		time.Sleep(500 * time.Millisecond)
	}
	if !loggedIn {
		return nil, errors.New("no se alcanzó el Menú SOL tras iniciar sesión (verifique RUC, Usuario SOL, Clave SOL o disponibilidad de SUNAT)")
	}

	// 3. Esperar que Menú SOL concluya su carga inicial antes de abrir Consulta Receptor (11.5.1.1.14)
	menuDeadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(menuDeadline) {
		ready, _ := chromedp.Run(timeoutCtx, chromedp.Evaluate[bool](`(() => {
			return document.readyState === 'complete' && Boolean(document.getElementById('nivel4_11_5_1_1_14') || document.getElementById('btnSalir'));
		})()`))
		if ready {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	time.Sleep(1500 * time.Millisecond)

	// 4. Navegar directo al módulo oficial Consulta Receptor (11.5.1.1.14) -> ol-ti-itreciboelectronico/cpelec001Alias
	if err := chromedp.Do(timeoutCtx, chromedp.Navigate(c.receptorURL)); err != nil {
		return nil, fmt.Errorf("no se pudo abrir el módulo Consulta Receptor RHE (11.5.1.1.14): %w", err)
	}

	formReady := false
	formDeadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(formDeadline) {
		if timeoutCtx.Err() != nil {
			break
		}
		curURL, _ := chromedp.Run(timeoutCtx, chromedp.Location())
		if strings.Contains(strings.ToLower(curURL), "ol-ti-itreciboelectronico") {
			hasForm, _ := chromedp.Run(timeoutCtx, chromedp.Evaluate[bool](`(() => {
				const f = document.forms['lista'] || document.forms[0];
				return Boolean(f && f.elements['fec_desde'] && f.elements['fec_hasta']);
			})()`))
			if hasForm {
				formReady = true
				break
			}
		}
		time.Sleep(450 * time.Millisecond)
	}
	if !formReady {
		return nil, errors.New("no se cargó el formulario Consulta Receptor RHE en SUNAT (verifique que el usuario SOL tenga acceso a Comprobantes de Pago / SEE - SOL / Recibo por Honorarios Electrónicos)")
	}

	// 5. Llenar criterios de búsqueda (rango de fechas, TODOS los estados, todos los tipos 4ta) y enviar
	submitScript := fmt.Sprintf(`(() => {
		window.__atlobAlertMsg = '';
		window.alert = function(m) { window.__atlobAlertMsg = String(m || ''); };
		const f = document.forms['lista'] || document.forms[0];
		if (!f) return 'NO_FORM';
		if (f.elements['ruc_emisor']) f.elements['ruc_emisor'].value = '';
		if (f.elements['num_serie']) f.elements['num_serie'].value = '';
		if (f.elements['num_comprob']) f.elements['num_comprob'].value = '';
		if (f.elements['fec_desde']) {
			f.elements['fec_desde'].value = %q;
			f.elements['fec_desde'].dispatchEvent(new Event('change', { bubbles: true }));
		}
		if (f.elements['fec_hasta']) {
			f.elements['fec_hasta'].value = %q;
			f.elements['fec_hasta'].dispatchEvent(new Event('change', { bubbles: true }));
		}
		if (f.elements['tipoestado']) {
			f.elements['tipoestado'].value = '00';
		}
		const checks = f.querySelectorAll('input[type=checkbox][name=tipocomprobante1]');
		checks.forEach(ch => { ch.checked = true; });
		if (typeof opcBuscar === 'function') {
			opcBuscar();
			return 'OK_OPC';
		}
		const btn = f.elements['wacepta'] || Array.from(f.querySelectorAll('input[type=button], input[type=submit], button')).find(b => /buscar/i.test(b.value || b.innerText || ''));
		if (btn) {
			btn.click();
			return 'OK_BTN';
		}
		f.submit();
		return 'OK_SUBMIT';
	})()`, fechaDesde, fechaHasta)

	submitStatus, err := chromedp.Run(timeoutCtx, chromedp.Evaluate[string](submitScript))
	if err != nil || submitStatus == "NO_FORM" {
		return nil, fmt.Errorf("error enviando criterios de consulta RHE en SUNAT: %v (%s)", err, submitStatus)
	}

	// 6. Esperar respuesta de SUNAT: lista4 (resultados + descarga oficial), sin registros, o alerta de validación
	type pageProbe struct {
		HasSearchForm bool   `json:"hasSearchForm"`
		HasLista4     bool   `json:"hasLista4"`
		HasNoDataMsg  bool   `json:"hasNoDataMsg"`
		AlertMsg      string `json:"alertMsg"`
		BodySnippet   string `json:"bodySnippet"`
	}

	var finalProbe pageProbe
	resultDeadline := time.Now().Add(25 * time.Second)
	// Dar un breve margen para que el POST inicie la navegación
	time.Sleep(1200 * time.Millisecond)

	for time.Now().Before(resultDeadline) {
		if timeoutCtx.Err() != nil {
			break
		}
		rawProbe, evalErr := chromedp.Run(timeoutCtx, chromedp.Evaluate[string](`(() => {
			const hasSearchForm = Boolean(document.forms && document.forms['lista'] && document.forms['lista'].elements && document.forms['lista'].elements['fec_desde']);
			const hasLista4 = Boolean(document.forms && document.forms['lista4']);
			const alertMsg = String(window.__atlobAlertMsg || '').trim();
			const bodyText = (document.body && document.body.innerText ? document.body.innerText : '').replace(/\s+/g, ' ').trim();
			const hasNoDataMsg = !hasSearchForm && !hasLista4 && /no\s+(existen|se\s+encontr|registra|hay\s+datos|posee)|ning[uú]n\s+registro|sin\s+resultados|0\s+de\s+0/i.test(bodyText);
			return JSON.stringify({
				hasSearchForm,
				hasLista4,
				hasNoDataMsg,
				alertMsg,
				bodySnippet: bodyText.slice(0, 320)
			});
		})()`))
		if evalErr == nil && rawProbe != "" {
			var p pageProbe
			if json.Unmarshal([]byte(rawProbe), &p) == nil {
				finalProbe = p
				if p.AlertMsg != "" || p.HasLista4 || p.HasNoDataMsg {
					break
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	if finalProbe.AlertMsg != "" {
		return nil, fmt.Errorf("SUNAT rechazó el criterio de consulta RHE: %s", finalProbe.AlertMsg)
	}

	if finalProbe.HasNoDataMsg {
		log.Printf("[RHE-SOL] Consulta completada para RUC %s (%s al %s): sin recibos en el periodo.",
			maskRUCForLog(ruc), fechaDesde, fechaHasta)
		return &ReceptorQueryResult{
			RawTXT:     BuildRHETXT(nil),
			FileName:   fmt.Sprintf("RHE_Receptor_%s_%s_%s.txt", ruc, strings.ReplaceAll(fechaDesde, "/", ""), strings.ReplaceAll(fechaHasta, "/", "")),
			FechaDesde: fechaDesde,
			FechaHasta: fechaHasta,
			Empty:      true,
			Notice:     "SUNAT no reportó Recibos por Honorarios recibidos en este periodo.",
		}, nil
	}

	if !finalProbe.HasLista4 {
		// Verificar si la página mostró un mensaje vacío sin patrón estándar (sin formulario de búsqueda ni lista4)
		if !finalProbe.HasSearchForm && strings.Contains(strings.ToLower(finalProbe.BodySnippet), "retornar") {
			return &ReceptorQueryResult{
				RawTXT:     BuildRHETXT(nil),
				FileName:   fmt.Sprintf("RHE_Receptor_%s.txt", ruc),
				FechaDesde: fechaDesde,
				FechaHasta: fechaHasta,
				Empty:      true,
				Notice:     "No se encontraron Recibos por Honorarios recibidos en el periodo consultado.",
			}, nil
		}
		return nil, fmt.Errorf("no se obtuvo la tabla de resultados RHE desde SUNAT (verifique disponibilidad del portal SOL o permisos del usuario)")
	}

	// 7. Descargar el reporte TXT oficial completo (POST accion=descargaConsultaReceptor) sin perder bytes ni paginación
	dlScript := fmt.Sprintf(`(() => {
		try {
			const xhr = new XMLHttpRequest();
			xhr.open('POST', %q, false);
			xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
			xhr.overrideMimeType('text/plain; charset=x-user-defined');
			xhr.send('accion=descargaConsultaReceptor');
			const rawStr = xhr.responseText || '';
			let bin = '';
			for (let i = 0; i < rawStr.length; i++) {
				bin += String.fromCharCode(rawStr.charCodeAt(i) & 0xff);
			}
			return JSON.stringify({
				status: xhr.status,
				contentType: xhr.getResponseHeader('content-type') || '',
				disposition: xhr.getResponseHeader('content-disposition') || '',
				b64: btoa(bin)
			});
		} catch (e) {
			return JSON.stringify({ error: String(e) });
		}
	})()`, c.servletURL)

	rawDownloadJSON, err := chromedp.Run(timeoutCtx, chromedp.Evaluate[string](dlScript))
	if err != nil {
		return nil, fmt.Errorf("error descargando reporte TXT oficial RHE desde SUNAT: %w", err)
	}

	var dlResp struct {
		Status      int    `json:"status"`
		ContentType string `json:"contentType"`
		Disposition string `json:"disposition"`
		B64         string `json:"b64"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal([]byte(rawDownloadJSON), &dlResp); err != nil {
		return nil, fmt.Errorf("error interpretando respuesta de descarga RHE: %w", err)
	}
	if dlResp.Error != "" {
		return nil, fmt.Errorf("error interno descargando reporte RHE: %s", dlResp.Error)
	}
	if dlResp.Status != 200 {
		return nil, fmt.Errorf("SUNAT respondió HTTP %d al descargar el reporte oficial RHE", dlResp.Status)
	}

	rawBytes, err := base64.StdEncoding.DecodeString(dlResp.B64)
	if err != nil {
		return nil, fmt.Errorf("error decodificando bytes del reporte TXT RHE: %w", err)
	}

	// Validar que SUNAT devolvió texto TXT y no una página HTML de sesión expirada
	trimmedSample := strings.ToLower(strings.TrimSpace(string(rawBytes[:minInt(len(rawBytes), 200)])))
	if strings.HasPrefix(trimmedSample, "<html") || strings.HasPrefix(trimmedSample, "<!doctype") {
		return nil, errors.New("SUNAT devolvió HTML en lugar del reporte TXT oficial RHE (la sesión SOL pudo haber expirado; intente nuevamente)")
	}

	fileName := extractFilenameFromDisposition(dlResp.Disposition)
	if fileName == "" {
		fileName = "archivo.txt"
	}

	log.Printf("[RHE-SOL] Reporte oficial TXT RHE descargado para RUC %s (%s al %s · %d bytes)",
		maskRUCForLog(ruc), fechaDesde, fechaHasta, len(rawBytes))

	return &ReceptorQueryResult{
		RawTXT:     rawBytes,
		FileName:   fileName,
		FechaDesde: fechaDesde,
		FechaHasta: fechaHasta,
		Empty:      len(strings.TrimSpace(string(rawBytes))) == 0,
	}, nil
}

func extractFilenameFromDisposition(disp string) string {
	lower := strings.ToLower(disp)
	idx := strings.Index(lower, "filename=")
	if idx < 0 {
		return ""
	}
	val := strings.TrimSpace(disp[idx+len("filename="):])
	if semi := strings.Index(val, ";"); semi >= 0 {
		val = val[:semi]
	}
	val = strings.Trim(val, `"' `)
	return filepath.Base(val)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maskRUCForLog(ruc string) string {
	r := strings.TrimSpace(ruc)
	if len(r) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(r)-4) + r[len(r)-4:]
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
