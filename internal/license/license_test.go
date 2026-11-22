package license

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SwodLore/sire-api/internal/crypto"
	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

type mockLicenseRecord struct {
	Key          string
	PlanType     string
	PlanLabel    string
	ClientName   string
	ClientEmail  string
	Status       string
	MaxCompanies int
	DurationDays int
	MachineHash  string
	ActivatedAt  *time.Time
	ExpiresAt    *time.Time
}

type mockSupabaseServer struct {
	mu      sync.Mutex
	server  *httptest.Server
	now     time.Time
	records map[string]*mockLicenseRecord
	down    bool
}

func newMockSupabaseServer(baseTime time.Time) *mockSupabaseServer {
	m := &mockSupabaseServer{
		now:     baseTime,
		records: make(map[string]*mockLicenseRecord),
	}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()

		if m.down {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"message":"service unavailable"}`))
			return
		}

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mHash, _ := body["p_machine_hash"].(string)

		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/register_trial") {
			cName, _ := body["p_client_name"].(string)
			cEmail, _ := body["p_client_email"].(string)
			cEmail = strings.ToLower(strings.TrimSpace(cEmail))
			for _, rec := range m.records {
				if rec.MachineHash == mHash {
					if rec.ExpiresAt != nil && !m.now.Before(*rec.ExpiresAt) {
						rec.Status = "expired"
						_ = json.NewEncoder(w).Encode(map[string]interface{}{
							"ok":            false,
							"code":          "EXPIRED",
							"status":        "expired",
							"license_key":   rec.Key,
							"plan_type":     rec.PlanType,
							"plan_label":    rec.PlanLabel,
							"client_name":   rec.ClientName,
							"max_companies": rec.MaxCompanies,
							"expires_at":    rec.ExpiresAt,
							"server_time":   m.now,
							"message":       "Este equipo ya utilizó su prueba gratuita de 7 días.",
						})
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"ok":            true,
						"code":          "ACTIVE",
						"status":        "active",
						"license_key":   rec.Key,
						"plan_type":     rec.PlanType,
						"plan_label":    rec.PlanLabel,
						"client_name":   rec.ClientName,
						"max_companies": rec.MaxCompanies,
						"activated_at":  rec.ActivatedAt,
						"expires_at":    rec.ExpiresAt,
						"server_time":   m.now,
						"message":       "Recuperamos tu prueba gratuita.",
					})
					return
				}
				if rec.ClientEmail == cEmail && rec.PlanType == "demo" && rec.MachineHash != mHash {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"ok":            false,
						"code":          "MACHINE_MISMATCH",
						"status":        rec.Status,
						"license_key":   rec.Key,
						"plan_type":     rec.PlanType,
						"plan_label":    rec.PlanLabel,
						"client_name":   rec.ClientName,
						"max_companies": rec.MaxCompanies,
						"expires_at":    rec.ExpiresAt,
						"server_time":   m.now,
						"message":       "Este correo ya cuenta con una prueba gratuita registrada en otro equipo.",
					})
					return
				}
			}
			exp := m.now.Add(7 * 24 * time.Hour)
			act := m.now
			newKey := "ATLOB-DEMO-AUTO-0001"
			rec := &mockLicenseRecord{
				Key:          newKey,
				PlanType:     "demo",
				PlanLabel:    "Prueba Gratis 7 días",
				ClientName:   cName,
				ClientEmail:  cEmail,
				Status:       "active",
				MaxCompanies: 3,
				DurationDays: 7,
				MachineHash:  mHash,
				ActivatedAt:  &act,
				ExpiresAt:    &exp,
			}
			m.records[newKey] = rec
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":            true,
				"code":          "ACTIVE",
				"status":        "active",
				"license_key":   rec.Key,
				"plan_type":     rec.PlanType,
				"plan_label":    rec.PlanLabel,
				"client_name":   rec.ClientName,
				"max_companies": rec.MaxCompanies,
				"activated_at":  rec.ActivatedAt,
				"expires_at":    rec.ExpiresAt,
				"server_time":   m.now,
				"message":       "¡Cuenta creada! Tienes 7 días de prueba gratuita (hasta 3 empresas).",
			})
			return
		}

		key, _ := body["p_license_key"].(string)
		key = strings.ToUpper(strings.TrimSpace(key))
		rec, ok := m.records[key]
		if !ok {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":          false,
				"code":        "NOT_FOUND",
				"status":      "",
				"license_key": key,
				"server_time": m.now,
				"message":     "El código de licencia no existe.",
			})
			return
		}

		if rec.Status == "revoked" || rec.Status == "suspended" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":            false,
				"code":          strings.ToUpper(rec.Status),
				"status":        rec.Status,
				"license_key":   rec.Key,
				"plan_type":     rec.PlanType,
				"plan_label":    rec.PlanLabel,
				"client_name":   rec.ClientName,
				"max_companies": rec.MaxCompanies,
				"expires_at":    rec.ExpiresAt,
				"server_time":   m.now,
				"message":       "Licencia " + rec.Status,
			})
			return
		}

		if rec.ExpiresAt != nil && !m.now.Before(*rec.ExpiresAt) {
			rec.Status = "expired"
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":            false,
				"code":          "EXPIRED",
				"status":        "expired",
				"license_key":   rec.Key,
				"plan_type":     rec.PlanType,
				"plan_label":    rec.PlanLabel,
				"client_name":   rec.ClientName,
				"max_companies": rec.MaxCompanies,
				"expires_at":    rec.ExpiresAt,
				"server_time":   m.now,
				"message":       "Tu prueba de 7 días finalizó.",
			})
			return
		}

		if strings.HasSuffix(r.URL.Path, "/activate_license") {
			if rec.MachineHash == "" {
				rec.MachineHash = mHash
				act := m.now
				rec.ActivatedAt = &act
				if rec.DurationDays > 0 && rec.ExpiresAt == nil {
					exp := m.now.Add(time.Duration(rec.DurationDays) * 24 * time.Hour)
					rec.ExpiresAt = &exp
				}
			} else if rec.MachineHash != mHash {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":            false,
					"code":          "MACHINE_MISMATCH",
					"status":        rec.Status,
					"license_key":   rec.Key,
					"plan_type":     rec.PlanType,
					"plan_label":    rec.PlanLabel,
					"client_name":   rec.ClientName,
					"max_companies": rec.MaxCompanies,
					"expires_at":    rec.ExpiresAt,
					"server_time":   m.now,
					"message":       "Licencia vinculada a otro equipo.",
				})
				return
			}
		} else if strings.HasSuffix(r.URL.Path, "/validate_license") {
			if rec.MachineHash == "" {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":          false,
					"code":        "NOT_ACTIVATED",
					"status":      "active",
					"license_key": rec.Key,
					"server_time": m.now,
					"message":     "Licencia no activada.",
				})
				return
			}
			if rec.MachineHash != mHash {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":            false,
					"code":          "MACHINE_MISMATCH",
					"status":        rec.Status,
					"license_key":   rec.Key,
					"plan_type":     rec.PlanType,
					"plan_label":    rec.PlanLabel,
					"client_name":   rec.ClientName,
					"max_companies": rec.MaxCompanies,
					"expires_at":    rec.ExpiresAt,
					"server_time":   m.now,
					"message":       "Esta licencia corresponde a otro equipo.",
				})
				return
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":            true,
			"code":          "ACTIVE",
			"status":        "active",
			"license_key":   rec.Key,
			"plan_type":     rec.PlanType,
			"plan_label":    rec.PlanLabel,
			"client_name":   rec.ClientName,
			"max_companies": rec.MaxCompanies,
			"activated_at":  rec.ActivatedAt,
			"expires_at":    rec.ExpiresAt,
			"server_time":   m.now,
			"message":       "Licencia vigente.",
		})
	}))
	return m
}

func setupLicenseTestDB(t *testing.T) (*sql.DB, *crypto.CryptoService) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("error abriendo sqlite en memoria: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE licencia_local (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			payload_enc TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("error creando tabla licencia_local: %v", err)
	}
	cryptoSvc, err := crypto.NewCryptoService("clave_prueba_32_bytes_licencias_2026")
	if err != nil {
		t.Fatalf("error inicializando crypto: %v", err)
	}
	return db, cryptoSvc
}

func TestLicenseActivationPersistenceAndReopen(t *testing.T) {
	baseNow := time.Date(2026, 11, 21, 19, 0, 0, 0, time.UTC)
	mock := newMockSupabaseServer(baseNow)
	defer mock.server.Close()

	mock.records["ATLOB-DEMO-7K9P-4X2M"] = &mockLicenseRecord{
		Key:          "ATLOB-DEMO-7K9P-4X2M",
		PlanType:     "demo",
		PlanLabel:    "Demo 7 días",
		ClientName:   "Estudio Contable Prueba",
		Status:       "active",
		MaxCompanies: 3,
		DurationDays: 7,
	}

	db, cryptoSvc := setupLicenseTestDB(t)
	defer db.Close()

	cfg := Config{
		SupabaseURL:     mock.server.URL,
		SupabaseAnonKey: "anon-test-key",
		Enforced:        true,
		AppVersion:      "2.0.0",
	}

	svc := NewService(db, cryptoSvc, cfg, func() (int, error) { return 1, nil })
	svc.SetNowFn(func() time.Time { return baseNow })
	svc.SetMachineIdentity("machine-hash-abc-001", "PC-CONTA-01")

	st := svc.GetStatus()
	if st.Valid || st.Code != CodeUnactivated {
		t.Fatalf("se esperaba licencia sin activar, obtenido: %+v", st)
	}

	st, err := svc.Activate(context.Background(), " atlob-demo-7k9p-4x2m ")
	if err != nil {
		t.Fatalf("error activando demo: %v", err)
	}
	if !st.Valid || st.DaysRemaining != 7 || st.MaxCompanies != 3 || st.CurrentCompanies != 1 {
		t.Fatalf("estado tras activación inesperado: %+v", st)
	}
	if !strings.Contains(st.LicenseKeyMasked, "****") {
		t.Fatalf("la clave debe devolverse enmascarada, obtenido: %q", st.LicenseKeyMasked)
	}

	// Reabrir servicio desde SQLite simulando reinicio de la app
	svc2 := NewService(db, cryptoSvc, cfg, func() (int, error) { return 2, nil })
	svc2.SetNowFn(func() time.Time { return baseNow.Add(2 * time.Hour) })
	svc2.SetMachineIdentity("machine-hash-abc-001", "PC-CONTA-01")
	if err := svc2.Initialize(context.Background()); err != nil {
		t.Fatalf("error inicializando servicio reabierto: %v", err)
	}
	st2 := svc2.GetStatus()
	if !st2.Valid || st2.ClientName != "Estudio Contable Prueba" || st2.CurrentCompanies != 2 {
		t.Fatalf("licencia persistida inválida tras reinicio: %+v", st2)
	}
}

func TestLicenseExpiredMiddlewareBlocksAPIWith402(t *testing.T) {
	baseNow := time.Date(2026, 11, 21, 19, 0, 0, 0, time.UTC)
	mock := newMockSupabaseServer(baseNow)
	defer mock.server.Close()

	mock.records["ATLOB-DEMO-EXP1-0001"] = &mockLicenseRecord{
		Key:          "ATLOB-DEMO-EXP1-0001",
		PlanType:     "demo",
		PlanLabel:    "Demo 7 días",
		ClientName:   "Cliente Expirado",
		Status:       "active",
		MaxCompanies: 3,
		DurationDays: 7,
	}

	db, cryptoSvc := setupLicenseTestDB(t)
	defer db.Close()

	cfg := Config{
		SupabaseURL:     mock.server.URL,
		SupabaseAnonKey: "anon-test-key",
		Enforced:        true,
	}
	svc := NewService(db, cryptoSvc, cfg, func() (int, error) { return 1, nil })
	svc.SetNowFn(func() time.Time { return baseNow })
	svc.SetMachineIdentity("machine-hash-exp-01", "PC-01")

	if _, err := svc.Activate(context.Background(), "ATLOB-DEMO-EXP1-0001"); err != nil {
		t.Fatalf("error activando: %v", err)
	}

	// Avanzar 8 días en servidor y reloj local
	expiredNow := baseNow.Add(8 * 24 * time.Hour)
	mock.mu.Lock()
	mock.now = expiredNow
	mock.mu.Unlock()
	svc.SetNowFn(func() time.Time { return expiredNow })

	if _, err := svc.Refresh(context.Background()); err != nil {
		t.Fatalf("error en refresh expirado: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	NewController(svc).RegisterRoutes(api)
	api.Use(RequireValidLicense(svc))
	api.GET("/empresas", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	api.GET("/salud", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Ruta de negocio debe devolver 402
	reqEmp, _ := http.NewRequest(http.MethodGet, "/api/empresas", nil)
	wEmp := httptest.NewRecorder()
	r.ServeHTTP(wEmp, reqEmp)
	if wEmp.Code != http.StatusPaymentRequired {
		t.Fatalf("se esperaba HTTP 402 en /api/empresas vencida, obtenido %d (%s)", wEmp.Code, wEmp.Body.String())
	}

	// Ruta /api/licencia/estado y /api/salud deben seguir accesibles
	for _, openPath := range []string{"/api/licencia/estado", "/api/salud"} {
		req, _ := http.NewRequest(http.MethodGet, openPath, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("ruta exenta %s devolvió %d, se esperaba 200", openPath, w.Code)
		}
	}
}

func TestLicenseMachineMismatchRevokedAndCompanyLimit(t *testing.T) {
	baseNow := time.Date(2026, 11, 21, 19, 0, 0, 0, time.UTC)
	mock := newMockSupabaseServer(baseNow)
	defer mock.server.Close()

	mock.records["ATLOB-DEMO-LIM3-9999"] = &mockLicenseRecord{
		Key:          "ATLOB-DEMO-LIM3-9999",
		PlanType:     "demo",
		PlanLabel:    "Demo 7 días",
		ClientName:   "Estudio Limite",
		Status:       "active",
		MaxCompanies: 3,
		DurationDays: 7,
	}

	db, cryptoSvc := setupLicenseTestDB(t)
	defer db.Close()

	cfg := Config{SupabaseURL: mock.server.URL, SupabaseAnonKey: "anon", Enforced: true}

	// Equipo 1 activa correctamente
	svcPC1 := NewService(db, cryptoSvc, cfg, func() (int, error) { return 3, nil })
	svcPC1.SetNowFn(func() time.Time { return baseNow })
	svcPC1.SetMachineIdentity("hash-pc-1", "PC-1")

	st1, err := svcPC1.Activate(context.Background(), "ATLOB-DEMO-LIM3-9999")
	if err != nil || !st1.Valid {
		t.Fatalf("error activando en PC1: %v %+v", err, st1)
	}

	// Límite de 3 empresas: con 2 pasa, con 3 bloquea la 4ta
	if err := svcPC1.CheckCanCreateCompany(2); err != nil {
		t.Fatalf("con 2 empresas debía permitir crear la 3ra: %v", err)
	}
	if err := svcPC1.CheckCanCreateCompany(3); err == nil || !strings.Contains(err.Error(), "940 781 298") {
		t.Fatalf("con 3 empresas debía bloquear la 4ta indicando contacto WhatsApp, obtenido: %v", err)
	}

	// Equipo 2 intenta activar la misma clave -> MACHINE_MISMATCH
	db2, cryptoSvc2 := setupLicenseTestDB(t)
	defer db2.Close()
	svcPC2 := NewService(db2, cryptoSvc2, cfg, func() (int, error) { return 0, nil })
	svcPC2.SetNowFn(func() time.Time { return baseNow })
	svcPC2.SetMachineIdentity("hash-pc-2", "PC-2")

	st2, err := svcPC2.Activate(context.Background(), "ATLOB-DEMO-LIM3-9999")
	if err != nil {
		t.Fatalf("error inesperado de red: %v", err)
	}
	if st2.Valid || st2.Code != CodeMachineMismatch {
		t.Fatalf("se esperaba MACHINE_MISMATCH en PC2, obtenido: %+v", st2)
	}

	// Revocación remota
	mock.mu.Lock()
	mock.records["ATLOB-DEMO-LIM3-9999"].Status = "revoked"
	mock.mu.Unlock()

	stRev, err := svcPC1.Refresh(context.Background())
	if err != nil {
		t.Fatalf("error en refresh revocado: %v", err)
	}
	if stRev.Valid || stRev.Code != CodeRevoked {
		t.Fatalf("se esperaba REVOKED tras revocación remota, obtenido: %+v", stRev)
	}
}

func TestLicenseOfflineGraceAndClockRollbackProtection(t *testing.T) {
	baseNow := time.Date(2026, 11, 21, 19, 0, 0, 0, time.UTC)
	mock := newMockSupabaseServer(baseNow)
	defer mock.server.Close()

	mock.records["ATLOB-DEMO-OFFL-1111"] = &mockLicenseRecord{
		Key:          "ATLOB-DEMO-OFFL-1111",
		PlanType:     "demo",
		PlanLabel:    "Demo 7 días",
		ClientName:   "Cliente Offline",
		Status:       "active",
		MaxCompanies: 3,
		DurationDays: 7,
	}

	db, cryptoSvc := setupLicenseTestDB(t)
	defer db.Close()

	cfg := Config{
		SupabaseURL:      mock.server.URL,
		SupabaseAnonKey:  "anon",
		Enforced:         true,
		DemoOfflineGrace: 12 * time.Hour,
		FullOfflineGrace: 72 * time.Hour,
		RefreshInterval:  4 * time.Hour,
	}

	svc := NewService(db, cryptoSvc, cfg, func() (int, error) { return 0, nil })
	svc.SetNowFn(func() time.Time { return baseNow })
	svc.SetMachineIdentity("hash-offline-1", "PC-OFFLINE")

	if _, err := svc.Activate(context.Background(), "ATLOB-DEMO-OFFL-1111"); err != nil {
		t.Fatalf("error activando: %v", err)
	}

	// Simular caída de Internet
	mock.mu.Lock()
	mock.down = true
	mock.mu.Unlock()

	// Dentro de 6h offline (<12h): permitido con OfflineMode=true tras refresh fallido
	svc.SetNowFn(func() time.Time { return baseNow.Add(6 * time.Hour) })
	st6h, _ := svc.Refresh(context.Background())
	if !st6h.Valid || !st6h.OfflineMode {
		t.Fatalf("dentro de 12h offline debía seguir válido en modo offline: %+v", st6h)
	}

	// Retroceder el reloj 2 horas respecto a MaxSeenUTC (+6h -> +3h): bloqueado por manipulación de reloj
	svc.SetNowFn(func() time.Time { return baseNow.Add(3 * time.Hour) })
	stRollback := svc.GetStatus()
	if stRollback.Valid || stRollback.Code != CodeOfflineLocked {
		t.Fatalf("retroceder reloj debía bloquear con OFFLINE_LOCKED, obtenido: %+v", stRollback)
	}

	// Superar ventana offline de 12h (+13h): bloqueado
	svc.SetNowFn(func() time.Time { return baseNow.Add(13 * time.Hour) })
	st13h := svc.GetStatus()
	if st13h.Valid || st13h.Code != CodeOfflineLocked {
		t.Fatalf("superar 12h offline en demo debía bloquear con OFFLINE_LOCKED, obtenido: %+v", st13h)
	}
}

func TestLicenseDevModeAllowsRequestsWithoutSupabaseConfig(t *testing.T) {
	db, cryptoSvc := setupLicenseTestDB(t)
	defer db.Close()

	svc := NewService(db, cryptoSvc, Config{Enforced: false}, nil)
	st := svc.GetStatus()
	if !st.Valid || st.Enforced || st.Code != CodeDevBypass {
		t.Fatalf("modo dev debía permitir paso sin restricciones: %+v", st)
	}
	if err := svc.CheckCanCreateCompany(50); err != nil {
		t.Fatalf("modo dev no debe limitar empresas: %v", err)
	}
}

func TestLicenseRegisterTrialAutoProvisionsAndBlocksRepeat(t *testing.T) {
	baseNow := time.Date(2026, 11, 21, 20, 0, 0, 0, time.UTC)
	mock := newMockSupabaseServer(baseNow)
	defer mock.server.Close()

	db, cryptoSvc := setupLicenseTestDB(t)
	defer db.Close()

	cfg := Config{SupabaseURL: mock.server.URL, SupabaseAnonKey: "anon", Enforced: true}
	svc := NewService(db, cryptoSvc, cfg, func() (int, error) { return 0, nil })
	svc.SetNowFn(func() time.Time { return baseNow })
	svc.SetMachineIdentity("hash-trial-pc1", "PC-PROSPECTO")

	// Validación local de campos vacíos / inválidos
	if _, err := svc.RegisterTrial(context.Background(), "A", "mal", "123"); err == nil {
		t.Fatalf("debía rechazar datos inválidos antes de consultar al servidor")
	}

	// Registro válido: aprovisiona 7 días y máximo 3 empresas automáticamente
	st, err := svc.RegisterTrial(context.Background(), "Estudio Contable Lima", "contacto@estudio.pe", "940781298")
	if err != nil {
		t.Fatalf("error en registro automático de prueba: %v", err)
	}
	if !st.Valid || st.DaysRemaining != 7 || st.MaxCompanies != 3 || st.ClientName != "Estudio Contable Lima" {
		t.Fatalf("estado inesperado tras auto-registro: %+v", st)
	}

	// Otro equipo intentando registrar con el mismo correo -> MACHINE_MISMATCH
	db2, cryptoSvc2 := setupLicenseTestDB(t)
	defer db2.Close()
	svc2 := NewService(db2, cryptoSvc2, cfg, nil)
	svc2.SetNowFn(func() time.Time { return baseNow })
	svc2.SetMachineIdentity("hash-trial-pc2", "PC-OTRO")
	stDupEmail, err := svc2.RegisterTrial(context.Background(), "Otro Usuario", "contacto@estudio.pe", "999888777")
	if err != nil || stDupEmail.Valid || stDupEmail.Code != CodeMachineMismatch {
		t.Fatalf("mismo correo en otro equipo debía bloquearse con MACHINE_MISMATCH, obtenido: err=%v st=%+v", err, stDupEmail)
	}

	// Tras vencer 8 días, intentar re-registrar en la misma PC debe bloquearse como EXPIRED
	expiredNow := baseNow.Add(8 * 24 * time.Hour)
	mock.mu.Lock()
	mock.now = expiredNow
	mock.mu.Unlock()
	svc.SetNowFn(func() time.Time { return expiredNow })

	stExpired, err := svc.RegisterTrial(context.Background(), "Estudio Contable Lima", "otro@correo.pe", "940781298")
	if err != nil || stExpired.Valid || stExpired.Code != CodeExpired {
		t.Fatalf("tras vencer los 7 días debía bloquearse como EXPIRED, obtenido: err=%v st=%+v", err, stExpired)
	}
}
