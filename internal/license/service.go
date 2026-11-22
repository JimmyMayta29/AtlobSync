package license

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SwodLore/sire-api/internal/config"
	"github.com/SwodLore/sire-api/internal/crypto"
)

const (
	defaultDemoOfflineGrace = 12 * time.Hour
	defaultFullOfflineGrace = 72 * time.Hour
	defaultRefreshInterval  = 4 * time.Hour
	clockRollbackTolerance  = 10 * time.Minute
)

// Config parametriza el motor comercial de licencias.
type Config struct {
	SupabaseURL      string
	SupabaseAnonKey  string
	Enforced         bool
	AppVersion       string
	DemoOfflineGrace time.Duration
	FullOfflineGrace time.Duration
	RefreshInterval  time.Duration
}

// LoadConfigFromEnv resuelve la configuración de licenciamiento priorizando variables inyectadas en compilación.
// Acepta tanto variables de Cloudflare Workers (LICENSE_API_URL / LICENSE_API_KEY) como de Supabase.
func LoadConfigFromEnv() Config {
	url := strings.TrimSpace(BuildSupabaseURL)
	if url == "" {
		url = strings.TrimSpace(os.Getenv("LICENSE_API_URL"))
	}
	if url == "" {
		url = strings.TrimSpace(os.Getenv("SUPABASE_URL"))
	}

	anonKey := strings.TrimSpace(BuildSupabaseAnonKey)
	if anonKey == "" {
		anonKey = strings.TrimSpace(os.Getenv("LICENSE_API_KEY"))
	}
	if anonKey == "" {
		anonKey = strings.TrimSpace(os.Getenv("SUPABASE_ANON_KEY"))
	}

	enforced := false
	if strings.EqualFold(strings.TrimSpace(BuildRequireLicense), "true") {
		enforced = true
	} else if url != "" && anonKey != "" && !isEnvTruthy(os.Getenv("ATLOB_LICENSE_BYPASS")) {
		enforced = true
	}

	return Config{
		SupabaseURL:      url,
		SupabaseAnonKey:  anonKey,
		Enforced:         enforced,
		AppVersion:       config.AppVersion,
		DemoOfflineGrace: defaultDemoOfflineGrace,
		FullOfflineGrace: defaultFullOfflineGrace,
		RefreshInterval:  defaultRefreshInterval,
	}
}

func isEnvTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "si", "on":
		return true
	default:
		return false
	}
}

// Service gestiona la activación, caché local cifrada AES-GCM, validación periódica y límites comerciales.
type Service struct {
	mu             sync.RWMutex
	db             *sql.DB
	cryptoSvc      *crypto.CryptoService
	cfg            Config
	client         *SupabaseClient
	companyCounter func() (int, error)
	nowFn          func() time.Time
	machineHash    string
	machineName    string
	cache          *cachedLicense
	offlineMode    bool
	refreshing     bool
}

func NewService(db *sql.DB, cryptoSvc *crypto.CryptoService, cfg Config, companyCounter func() (int, error)) *Service {
	if cfg.DemoOfflineGrace <= 0 {
		cfg.DemoOfflineGrace = defaultDemoOfflineGrace
	}
	if cfg.FullOfflineGrace <= 0 {
		cfg.FullOfflineGrace = defaultFullOfflineGrace
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = defaultRefreshInterval
	}
	if strings.TrimSpace(cfg.AppVersion) == "" {
		cfg.AppVersion = config.AppVersion
	}

	mHash, mName := GetMachineIdentity()
	var client *SupabaseClient
	if strings.TrimSpace(cfg.SupabaseURL) != "" && strings.TrimSpace(cfg.SupabaseAnonKey) != "" {
		client = NewSupabaseClient(cfg.SupabaseURL, cfg.SupabaseAnonKey)
	}

	return &Service{
		db:             db,
		cryptoSvc:      cryptoSvc,
		cfg:            cfg,
		client:         client,
		companyCounter: companyCounter,
		nowFn:          time.Now,
		machineHash:    mHash,
		machineName:    mName,
	}
}

// SetNowFn permite inyectar un reloj determinista en pruebas unitarias.
func (s *Service) SetNowFn(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fn != nil {
		s.nowFn = fn
	}
}

// SetMachineIdentity permite simular cambios de equipo en pruebas unitarias.
func (s *Service) SetMachineIdentity(hash, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.machineHash = hash
	s.machineName = name
}

func (s *Service) IsServerConfigured() bool {
	return s.client != nil && s.client.IsConfigured()
}

// Initialize carga la licencia cifrada local y la revalida online si corresponde.
func (s *Service) Initialize(ctx context.Context) error {
	if loaded, err := s.loadCachedLicense(); err == nil && loaded != nil {
		s.mu.Lock()
		s.cache = loaded
		s.mu.Unlock()
	}

	if !s.cfg.Enforced {
		return nil
	}

	s.mu.RLock()
	hasKey := s.cache != nil && strings.TrimSpace(s.cache.LicenseKey) != ""
	s.mu.RUnlock()

	if !hasKey || !s.IsServerConfigured() {
		return nil
	}

	_, err := s.Refresh(ctx)
	if errors.Is(err, ErrNetwork) {
		s.advanceClockWatermarkIfNeeded()
	}
	return err
}

// Activate normaliza y activa una clave de licencia contra Supabase, persistiendo el resultado cifrado en SQLite.
func (s *Service) Activate(ctx context.Context, rawKey string) (LicenseStatus, error) {
	key := strings.ToUpper(strings.TrimSpace(rawKey))
	if key == "" {
		st := s.GetStatus()
		st.Valid = false
		st.Code = CodeNotFound
		st.Message = "Ingresa un código de licencia válido."
		return st, errors.New(st.Message)
	}

	if !s.IsServerConfigured() {
		st := s.GetStatus()
		if !s.cfg.Enforced {
			return st, nil
		}
		st.Valid = false
		st.Code = CodeUnactivated
		st.Message = fmt.Sprintf("Esta compilación no tiene configurado el servidor de licencias. Contáctanos por WhatsApp al %s.", BuildSupportPhone)
		return st, errors.New(st.Message)
	}

	s.mu.RLock()
	mHash := s.machineHash
	mName := s.machineName
	appVer := s.cfg.AppVersion
	s.mu.RUnlock()

	// Importante: no abrir transacción SQLite ni sostener mutex mientras se consulta HTTP a Supabase.
	resp, err := s.client.Activate(ctx, key, mHash, mName, appVer)
	if err != nil {
		if errors.Is(err, ErrNetwork) {
			msg := "No se pudo conectar al servidor de licencias. Verifica tu conexión a Internet e intenta nuevamente."
			return s.buildStatusWithOverride(false, CodeOfflineLocked, msg), errors.New(msg)
		}
		return s.buildStatusWithOverride(false, CodeNotFound, err.Error()), err
	}

	now := s.nowFn().UTC()
	if resp.OK {
		entry := s.cacheFromRPC(key, mHash, resp, now, nil)
		if err := s.saveCachedLicense(entry); err != nil {
			return s.GetStatus(), fmt.Errorf("error guardando licencia local: %w", err)
		}
		s.mu.Lock()
		s.cache = entry
		s.offlineMode = false
		s.mu.Unlock()
		return s.GetStatus(), nil
	}

	code := strings.ToUpper(strings.TrimSpace(resp.Code))
	if code == "" {
		code = CodeNotFound
	}
	msg := strings.TrimSpace(resp.Message)
	if msg == "" {
		msg = "No se pudo activar el código de licencia."
	}

	// Si el código evaluado es la licencia actualmente guardada (o no había licencia previa y no es un error de tipeo NOT_FOUND),
	// reflejamos el estado actualizado en caché local para mantener consistencia tras reinicios.
	s.mu.RLock()
	prev := s.cache
	s.mu.RUnlock()
	sameKey := prev != nil && strings.EqualFold(prev.LicenseKey, key)

	if sameKey && (code == CodeExpired || code == CodeRevoked || code == CodeSuspended || code == CodeMachineMismatch) {
		entry := s.cacheFromRPC(key, mHash, resp, now, prev)
		entry.Status = strings.ToLower(code)
		if code == CodeMachineMismatch {
			entry.Status = "mismatch"
		}
		_ = s.saveCachedLicense(entry)
		s.mu.Lock()
		s.cache = entry
		s.offlineMode = false
		s.mu.Unlock()
		return s.GetStatus(), nil
	}

	out := s.buildFailureStatusFromRPC(key, code, msg, resp)
	return out, nil
}

// RegisterTrial crea automáticamente una cuenta demo de 7 días (máx 3 empresas) vinculada al equipo y correo del prospecto.
func (s *Service) RegisterTrial(ctx context.Context, clientName, clientEmail, clientPhone string) (LicenseStatus, error) {
	name := strings.TrimSpace(clientName)
	email := strings.ToLower(strings.TrimSpace(clientEmail))
	phone := strings.TrimSpace(clientPhone)

	if len([]rune(name)) < 2 {
		msg := "Ingresa tu nombre completo o estudio contable (mínimo 2 caracteres)."
		return s.buildStatusWithOverride(false, CodeUnactivated, msg), errors.New(msg)
	}
	atIdx := strings.IndexByte(email, '@')
	if atIdx <= 0 || strings.ContainsAny(email, " \t\r\n") || !strings.Contains(email[atIdx+1:], ".") {
		msg := "Ingresa un correo electrónico válido."
		return s.buildStatusWithOverride(false, CodeUnactivated, msg), errors.New(msg)
	}
	digitCount := 0
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digitCount++
		}
	}
	if digitCount < 7 {
		msg := "Ingresa un número de celular o WhatsApp válido (mínimo 7 dígitos)."
		return s.buildStatusWithOverride(false, CodeUnactivated, msg), errors.New(msg)
	}

	if !s.IsServerConfigured() {
		st := s.GetStatus()
		if !s.cfg.Enforced {
			return st, nil
		}
		st.Valid = false
		st.Code = CodeUnactivated
		st.Message = fmt.Sprintf("Esta compilación no tiene configurado el servidor de licencias. Contáctanos por WhatsApp al %s.", BuildSupportPhone)
		return st, errors.New(st.Message)
	}

	s.mu.RLock()
	mHash := s.machineHash
	mName := s.machineName
	appVer := s.cfg.AppVersion
	s.mu.RUnlock()

	resp, err := s.client.RegisterTrial(ctx, name, email, phone, mHash, mName, appVer)
	if err != nil {
		if errors.Is(err, ErrNetwork) {
			msg := "No se pudo conectar al servidor para crear tu prueba gratuita. Verifica tu conexión a Internet e intenta nuevamente."
			return s.buildStatusWithOverride(false, CodeOfflineLocked, msg), errors.New(msg)
		}
		return s.buildStatusWithOverride(false, CodeUnactivated, err.Error()), err
	}

	now := s.nowFn().UTC()
	resolvedKey := strings.ToUpper(strings.TrimSpace(resp.LicenseKey))
	if resp.OK {
		entry := s.cacheFromRPC(resolvedKey, mHash, resp, now, nil)
		if err := s.saveCachedLicense(entry); err != nil {
			return s.GetStatus(), fmt.Errorf("error guardando cuenta de prueba local: %w", err)
		}
		s.mu.Lock()
		s.cache = entry
		s.offlineMode = false
		s.mu.Unlock()
		return s.GetStatus(), nil
	}

	code := strings.ToUpper(strings.TrimSpace(resp.Code))
	if code == "" {
		code = CodeUnactivated
	}
	msg := strings.TrimSpace(resp.Message)
	if msg == "" {
		msg = "No se pudo registrar la prueba gratuita."
	}

	if resolvedKey != "" && (code == CodeExpired || code == CodeRevoked || code == CodeSuspended) {
		entry := s.cacheFromRPC(resolvedKey, mHash, resp, now, nil)
		entry.Status = strings.ToLower(code)
		_ = s.saveCachedLicense(entry)
		s.mu.Lock()
		s.cache = entry
		s.offlineMode = false
		s.mu.Unlock()
		return s.GetStatus(), nil
	}

	out := s.buildFailureStatusFromRPC(resolvedKey, code, msg, resp)
	return out, nil
}

// Refresh revalida online la licencia activa con Supabase.
func (s *Service) Refresh(ctx context.Context) (LicenseStatus, error) {
	if !s.cfg.Enforced {
		return s.GetStatus(), nil
	}

	s.mu.RLock()
	prev := s.cache
	mHash := s.machineHash
	mName := s.machineName
	appVer := s.cfg.AppVersion
	s.mu.RUnlock()

	if prev == nil || strings.TrimSpace(prev.LicenseKey) == "" {
		return s.GetStatus(), nil
	}

	if !s.IsServerConfigured() {
		return s.GetStatus(), nil
	}

	companiesCount := s.safeCountCompanies()

	// Llamada HTTP fuera de mutex y fuera de transacciones SQLite.
	resp, err := s.client.Validate(ctx, prev.LicenseKey, mHash, mName, appVer, companiesCount)
	if err != nil {
		if errors.Is(err, ErrNetwork) {
			s.mu.Lock()
			s.offlineMode = true
			s.mu.Unlock()
			s.advanceClockWatermarkIfNeeded()
		}
		return s.GetStatus(), err
	}

	now := s.nowFn().UTC()
	if resp.OK {
		entry := s.cacheFromRPC(prev.LicenseKey, mHash, resp, now, prev)
		if err := s.saveCachedLicense(entry); err != nil {
			log.Printf("[LICENCIA] Aviso: no se pudo actualizar caché local cifrada: %v", err)
		}
		s.mu.Lock()
		s.cache = entry
		s.offlineMode = false
		s.mu.Unlock()
		return s.GetStatus(), nil
	}

	code := strings.ToUpper(strings.TrimSpace(resp.Code))
	entry := s.cacheFromRPC(prev.LicenseKey, mHash, resp, now, prev)
	switch code {
	case CodeExpired:
		entry.Status = "expired"
	case CodeRevoked, CodeNotFound:
		entry.Status = "revoked"
	case CodeSuspended:
		entry.Status = "suspended"
	case CodeMachineMismatch:
		entry.Status = "mismatch"
	case "NOT_ACTIVATED":
		entry.Status = "unactivated"
	default:
		if strings.TrimSpace(resp.Status) != "" {
			entry.Status = strings.ToLower(strings.TrimSpace(resp.Status))
		} else {
			entry.Status = "revoked"
		}
	}
	if err := s.saveCachedLicense(entry); err != nil {
		log.Printf("[LICENCIA] Aviso: no se pudo guardar estado bloqueado local: %v", err)
	}
	s.mu.Lock()
	s.cache = entry
	s.offlineMode = false
	s.mu.Unlock()
	return s.GetStatus(), nil
}

// GetStatus evalúa en memoria (thread-safe) la vigencia actual de la licencia, incluyendo expiración y reloj.
func (s *Service) GetStatus() LicenseStatus {
	companiesCount := s.safeCountCompanies()

	s.mu.RLock()
	cfg := s.cfg
	cache := s.cache
	mHash := s.machineHash
	offline := s.offlineMode
	refreshing := s.refreshing
	now := s.nowFn().UTC()
	serverCfg := s.IsServerConfigured()
	s.mu.RUnlock()

	machineShort := ShortMachineID(mHash)

	if !cfg.Enforced {
		st := LicenseStatus{
			Enforced:         false,
			Valid:            true,
			Code:             CodeDevBypass,
			Status:           "dev",
			PlanType:         "dev",
			PlanLabel:        "Modo Desarrollo Local",
			MaxCompanies:     0,
			CurrentCompanies: companiesCount,
			DaysRemaining:    -1,
			HoursRemaining:   -1,
			MachineID:        machineShort,
			Message:          "Modo desarrollo local (sin validación comercial obligatoria).",
			SupportPhone:     BuildSupportPhone,
			SupportWhatsApp:  BuildSupportWhatsApp,
			ServerConfigured: serverCfg,
		}
		st.SupportWhatsAppURL = BuildWhatsAppURL(BuildSupportWhatsApp, "Hola ATLOB Consultores, deseo información sobre ATLOB Sync.")
		return st
	}

	if cache == nil || strings.TrimSpace(cache.LicenseKey) == "" || strings.EqualFold(cache.Status, "unactivated") {
		st := LicenseStatus{
			Enforced:         true,
			Valid:            false,
			Code:             CodeUnactivated,
			Status:           "unactivated",
			CurrentCompanies: companiesCount,
			DaysRemaining:    0,
			HoursRemaining:   0,
			MachineID:        machineShort,
			Message:          "Ingresa tu código de prueba o licencia comercial para comenzar.",
			SupportPhone:     BuildSupportPhone,
			SupportWhatsApp:  BuildSupportWhatsApp,
			ServerConfigured: serverCfg,
		}
		st.SupportWhatsAppURL = BuildWhatsAppURL(BuildSupportWhatsApp, fmt.Sprintf("Hola, deseo solicitar un código demo de 7 días para ATLOB Sync (ID equipo: %s).", machineShort))
		return st
	}

	// Disparar refresco asíncrono si venció el intervalo (4h) y no hay otro en vuelo.
	if serverCfg && !refreshing && cfg.RefreshInterval > 0 && !cache.LastVerifiedUTC.IsZero() && now.Sub(cache.LastVerifiedUTC) >= cfg.RefreshInterval {
		s.triggerAsyncRefresh()
	}

	st := LicenseStatus{
		Enforced:         true,
		LicenseKeyMasked: MaskLicenseKey(cache.LicenseKey),
		PlanType:         cache.PlanType,
		PlanLabel:        cache.PlanLabel,
		ClientName:       cache.ClientName,
		Status:           cache.Status,
		MaxCompanies:     cache.MaxCompanies,
		CurrentCompanies: companiesCount,
		ActivatedAt:      cache.ActivatedAt,
		ExpiresAt:        cache.ExpiresAt,
		OfflineMode:      offline,
		MachineID:        machineShort,
		SupportPhone:     BuildSupportPhone,
		SupportWhatsApp:  BuildSupportWhatsApp,
		ServerConfigured: serverCfg,
	}
	if !cache.LastVerifiedUTC.IsZero() {
		lv := cache.LastVerifiedUTC
		st.LastVerifiedAt = &lv
	}

	setWA := func(customMsg string) {
		st.SupportWhatsAppURL = BuildWhatsAppURL(BuildSupportWhatsApp, customMsg)
	}

	if cache.MachineHash != "" && cache.MachineHash != mHash {
		st.Valid = false
		st.Code = CodeMachineMismatch
		st.Status = "mismatch"
		st.Message = fmt.Sprintf("Esta licencia está vinculada a otro equipo. Si cambiaste de PC contáctanos al %s (ID: %s).", BuildSupportPhone, machineShort)
		setWA(fmt.Sprintf("Hola, necesito desvincular mi licencia %s porque cambié de PC (nuevo ID: %s).", st.LicenseKeyMasked, machineShort))
		return st
	}

	switch strings.ToLower(strings.TrimSpace(cache.Status)) {
	case "revoked":
		st.Valid = false
		st.Code = CodeRevoked
		st.Status = "revoked"
		st.Message = nonEmpty(cache.Message, fmt.Sprintf("Tu licencia fue revocada. Contáctanos al %s para soporte o renovación.", BuildSupportPhone))
		setWA(fmt.Sprintf("Hola, mi licencia %s figura como revocada (%s). Quisiera revisarlo.", st.LicenseKeyMasked, st.ClientName))
		return st
	case "suspended":
		st.Valid = false
		st.Code = CodeSuspended
		st.Status = "suspended"
		st.Message = nonEmpty(cache.Message, fmt.Sprintf("Tu licencia se encuentra suspendida temporalmente. Contáctanos al %s.", BuildSupportPhone))
		setWA(fmt.Sprintf("Hola, mi licencia %s figura suspendida (%s). ¿Me ayudas a reactivarla?", st.LicenseKeyMasked, st.ClientName))
		return st
	case "expired":
		st.Valid = false
		st.Code = CodeExpired
		st.Status = "expired"
		st.DaysRemaining = 0
		st.HoursRemaining = 0
		st.Message = nonEmpty(cache.Message, fmt.Sprintf("Tu periodo de prueba o licencia ha finalizado. Tus datos siguen guardados en tu PC. Escríbenos al %s para activar un plan.", BuildSupportPhone))
		setWA(fmt.Sprintf("Hola, ya venció mi acceso %s (%s) y quiero contratar/renovar ATLOB Sync.", st.LicenseKeyMasked, st.PlanLabel))
		return st
	case "mismatch":
		st.Valid = false
		st.Code = CodeMachineMismatch
		st.Status = "mismatch"
		st.Message = nonEmpty(cache.Message, fmt.Sprintf("Esta licencia corresponde a otro equipo. Contáctanos al %s.", BuildSupportPhone))
		setWA(fmt.Sprintf("Hola, mi licencia %s indica otro equipo (ID actual: %s).", st.LicenseKeyMasked, machineShort))
		return st
	}

	// Verificar expiración por fecha
	if cache.ExpiresAt != nil {
		if !now.Before(*cache.ExpiresAt) {
			st.Valid = false
			st.Code = CodeExpired
			st.Status = "expired"
			st.DaysRemaining = 0
			st.HoursRemaining = 0
			st.Message = fmt.Sprintf("Tu periodo de prueba o licencia ha finalizado. Tus datos siguen seguros en este equipo. Escríbenos al %s para contratar o renovar.", BuildSupportPhone)
			setWA(fmt.Sprintf("Hola, finalizó mi %s (%s) en ATLOB Sync y quiero contratar un plan.", st.PlanLabel, st.LicenseKeyMasked))
			return st
		}
		remaining := cache.ExpiresAt.Sub(now)
		st.DaysRemaining = int(math.Ceil(remaining.Hours() / 24.0))
		st.HoursRemaining = int(math.Ceil(remaining.Hours()))
		if st.DaysRemaining < 0 {
			st.DaysRemaining = 0
		}
		if st.HoursRemaining < 0 {
			st.HoursRemaining = 0
		}
	} else {
		st.DaysRemaining = -1
		st.HoursRemaining = -1
	}

	// Protección anti-manipulación de reloj de Windows
	if !cache.MaxSeenUTC.IsZero() && now.Before(cache.MaxSeenUTC.Add(-clockRollbackTolerance)) {
		st.Valid = false
		st.Code = CodeOfflineLocked
		st.OfflineMode = true
		st.Message = "Se detectó un cambio en la fecha u hora del sistema. Verifica la hora de Windows y conéctate a Internet para revalidar tu licencia."
		setWA(fmt.Sprintf("Hola, tengo un aviso de validación de hora en ATLOB Sync (licencia %s).", st.LicenseKeyMasked))
		return st
	}

	// Ventana de gracia offline diferenciada (demo 12h vs comercial 72h)
	grace := cfg.FullOfflineGrace
	if strings.EqualFold(cache.PlanType, "demo") {
		grace = cfg.DemoOfflineGrace
	}
	if !cache.LastVerifiedUTC.IsZero() && now.Sub(cache.LastVerifiedUTC) > grace {
		st.Valid = false
		st.Code = CodeOfflineLocked
		st.OfflineMode = true
		st.Message = "Se requiere conexión a Internet para verificar la vigencia de tu licencia. Conéctate y presiona Revalidar."
		setWA(fmt.Sprintf("Hola, necesito ayuda para validar mi licencia %s en ATLOB Sync.", st.LicenseKeyMasked))
		return st
	}

	st.Valid = true
	st.Status = "active"
	if strings.EqualFold(cache.PlanType, "demo") && st.DaysRemaining >= 0 && st.DaysRemaining <= 2 {
		st.Code = CodeExpiringSoon
	} else {
		st.Code = CodeActive
	}
	st.Message = nonEmpty(cache.Message, "Licencia vigente.")
	setWA(fmt.Sprintf("Hola, quiero contratar un plan comercial para ATLOB Sync (cliente: %s, licencia: %s).", st.ClientName, st.LicenseKeyMasked))
	return st
}

// CheckCanCreateCompany valida si la licencia activa permite registrar otra empresa adicional.
func (s *Service) CheckCanCreateCompany(currentCount int) error {
	if !s.cfg.Enforced {
		return nil
	}
	st := s.GetStatus()
	if !st.Valid {
		return fmt.Errorf("licencia no activa: %s", st.Message)
	}
	if st.MaxCompanies > 0 && currentCount >= st.MaxCompanies {
		return fmt.Errorf("tu plan actual (%s) permite hasta %d empresa(s). Para registrar más empresas, contáctanos por WhatsApp al %s",
			nonEmpty(st.PlanLabel, st.PlanType), st.MaxCompanies, st.SupportPhone)
	}
	return nil
}

// StartBackgroundRefresh mantiene revalidación periódica en segundo plano mientras la aplicación esté abierta.
func (s *Service) StartBackgroundRefresh(ctx context.Context) {
	interval := s.cfg.RefreshInterval
	if interval <= 0 {
		interval = defaultRefreshInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.cfg.Enforced || !s.IsServerConfigured() {
				continue
			}
			s.mu.RLock()
			hasKey := s.cache != nil && strings.TrimSpace(s.cache.LicenseKey) != ""
			s.mu.RUnlock()
			if !hasKey {
				continue
			}
			reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
			_, _ = s.Refresh(reqCtx)
			cancel()
		}
	}
}

func (s *Service) triggerAsyncRefresh() {
	s.mu.Lock()
	if s.refreshing {
		s.mu.Unlock()
		return
	}
	s.refreshing = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.refreshing = false
			s.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = s.Refresh(ctx)
	}()
}

func (s *Service) advanceClockWatermarkIfNeeded() {
	s.mu.Lock()
	if s.cache == nil {
		s.mu.Unlock()
		return
	}
	now := s.nowFn().UTC()
	if !now.After(s.cache.MaxSeenUTC.Add(5 * time.Minute)) {
		s.mu.Unlock()
		return
	}
	// Clonar para guardar fuera del mutex
	clone := *s.cache
	clone.MaxSeenUTC = now
	s.cache = &clone
	s.mu.Unlock()

	_ = s.saveCachedLicense(&clone)
}

func (s *Service) safeCountCompanies() int {
	if s.companyCounter == nil {
		return 0
	}
	cnt, err := s.companyCounter()
	if err != nil || cnt < 0 {
		return 0
	}
	return cnt
}

func (s *Service) cacheFromRPC(key, machineHash string, resp *rpcResponse, now time.Time, prev *cachedLicense) *cachedLicense {
	maxSeen := now
	if resp.ServerTime != nil && resp.ServerTime.UTC().After(maxSeen) {
		maxSeen = resp.ServerTime.UTC()
	}
	if prev != nil && prev.MaxSeenUTC.After(maxSeen) {
		maxSeen = prev.MaxSeenUTC
	}

	planType := strings.ToLower(strings.TrimSpace(resp.PlanType))
	if planType == "" && prev != nil {
		planType = prev.PlanType
	}
	planLabel := strings.TrimSpace(resp.PlanLabel)
	if planLabel == "" && prev != nil {
		planLabel = prev.PlanLabel
	}
	clientName := strings.TrimSpace(resp.ClientName)
	if clientName == "" && prev != nil {
		clientName = prev.ClientName
	}
	status := strings.ToLower(strings.TrimSpace(resp.Status))
	if status == "" {
		if resp.OK {
			status = "active"
		} else {
			status = strings.ToLower(strings.TrimSpace(resp.Code))
		}
	}

	maxCompanies := resp.MaxCompanies
	if maxCompanies == 0 && prev != nil && prev.MaxCompanies > 0 && planType == "demo" && !resp.OK {
		maxCompanies = prev.MaxCompanies
	}

	activatedAt := resp.ActivatedAt
	if activatedAt == nil && prev != nil {
		activatedAt = prev.ActivatedAt
	}
	expiresAt := resp.ExpiresAt
	if expiresAt == nil && prev != nil && !resp.OK {
		expiresAt = prev.ExpiresAt
	}

	return &cachedLicense{
		LicenseKey:      key,
		MachineHash:     machineHash,
		PlanType:        planType,
		PlanLabel:       planLabel,
		ClientName:      clientName,
		Status:          status,
		MaxCompanies:    maxCompanies,
		ActivatedAt:     activatedAt,
		ExpiresAt:       expiresAt,
		LastVerifiedUTC: now,
		MaxSeenUTC:      maxSeen,
		Message:         strings.TrimSpace(resp.Message),
	}
}

func (s *Service) buildStatusWithOverride(valid bool, code, msg string) LicenseStatus {
	st := s.GetStatus()
	st.Valid = valid
	st.Code = code
	st.Message = msg
	return st
}

func (s *Service) buildFailureStatusFromRPC(key, code, msg string, resp *rpcResponse) LicenseStatus {
	st := s.GetStatus()
	st.Valid = false
	st.Code = code
	st.Message = msg
	st.LicenseKeyMasked = MaskLicenseKey(key)
	if resp != nil {
		if resp.PlanType != "" {
			st.PlanType = resp.PlanType
		}
		if resp.PlanLabel != "" {
			st.PlanLabel = resp.PlanLabel
		}
		if resp.ClientName != "" {
			st.ClientName = resp.ClientName
		}
		if resp.ExpiresAt != nil {
			st.ExpiresAt = resp.ExpiresAt
		}
		if resp.Status != "" {
			st.Status = strings.ToLower(resp.Status)
		} else {
			st.Status = strings.ToLower(code)
		}
	}
	st.SupportWhatsAppURL = BuildWhatsAppURL(BuildSupportWhatsApp, fmt.Sprintf("Hola, intenté activar la clave %s en ATLOB Sync y apareció: %s (%s).", st.LicenseKeyMasked, code, st.MachineID))
	return st
}

func (s *Service) loadCachedLicense() (*cachedLicense, error) {
	if s.db == nil || s.cryptoSvc == nil {
		return nil, errors.New("dependencias locales no inicializadas")
	}
	var payloadEnc string
	err := s.db.QueryRow("SELECT payload_enc FROM licencia_local WHERE id = 1").Scan(&payloadEnc)
	if err != nil {
		return nil, err
	}
	plain, err := s.cryptoSvc.Decrypt(payloadEnc)
	if err != nil {
		return nil, err
	}
	var cached cachedLicense
	if err := json.Unmarshal([]byte(plain), &cached); err != nil {
		return nil, err
	}
	return &cached, nil
}

func (s *Service) saveCachedLicense(cached *cachedLicense) error {
	if s.db == nil || s.cryptoSvc == nil || cached == nil {
		return nil
	}
	raw, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	enc, err := s.cryptoSvc.Encrypt(string(raw))
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO licencia_local (id, payload_enc, updated_at)
		VALUES (1, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			payload_enc = excluded.payload_enc,
			updated_at = CURRENT_TIMESTAMP
	`, enc)
	return err
}

func nonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	return fallback
}
