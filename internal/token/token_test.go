package token

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SwodLore/sire-api/internal/company"
	"github.com/SwodLore/sire-api/internal/crypto"
	"github.com/SwodLore/sire-api/internal/database"
	_ "modernc.org/sqlite"
)

func setupTokenTestDB(t *testing.T) (*company.Repository, *crypto.CryptoService) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Error abriendo DB en memoria: %v", err)
	}

	cryptoSvc, _ := crypto.NewCryptoService("clave_test_32_bytes_super_secreta")
	repo := company.NewRepository(db)

	return repo, cryptoSvc
}

func TestTokenManagerCache(t *testing.T) {
	repo, cryptoSvc := setupTokenTestDB(t)
	tm := NewTokenManager(repo, cryptoSvc)

	ruc := "20000000001"
	token := "token_simulado_jwt"

	// Insertar manualmente en caché
	tm.tokensMu.Lock()
	tm.tokens[ruc] = &CachedToken{
		AccessToken: token,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	tm.tokensMu.Unlock()

	// Probar que retorna el token de caché sin llamar a SUNAT
	obtenido, err := tm.GetToken(ruc)
	if err != nil {
		t.Fatalf("Error al obtener token de caché: %v", err)
	}
	if obtenido != token {
		t.Fatalf("Se esperaba %s pero se obtuvo %s", token, obtenido)
	}

	// Probar invalidación
	tm.InvalidateToken(ruc)
	tm.tokensMu.RLock()
	_, exists := tm.tokens[ruc]
	tm.tokensMu.RUnlock()
	if exists {
		t.Fatalf("El token debería haberse eliminado tras InvalidateToken")
	}
}

func TestTokenManagerConcurrency(t *testing.T) {
	repo, cryptoSvc := setupTokenTestDB(t)
	tm := NewTokenManager(repo, cryptoSvc)
	ruc := "20000000001"

	// Cargar token en caché
	tm.tokensMu.Lock()
	tm.tokens[ruc] = &CachedToken{
		AccessToken: "token_concurrente_valido",
		ExpiresAt:   time.Now().Add(30 * time.Minute),
	}
	tm.tokensMu.Unlock()

	var wg sync.WaitGroup
	errores := make(chan error, 50)

	// Disparar 50 lecturas concurrentes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, err := tm.GetToken(ruc)
			if err != nil || tok != "token_concurrente_valido" {
				errores <- err
			}
		}()
	}

	wg.Wait()
	close(errores)

	for err := range errores {
		t.Fatalf("Fallo en concurrencia: %v", err)
	}
}

// TestTokenCooldown_MaxOneAttemptAndRecoveryOnPUT valida:
// 1. Ante invalid_grant, aplica enfriamiento de 15 min y marca en BD como INVALIDAS.
// 2. Máximo 1 intento de petición de red hacia SUNAT durante el periodo de enfriamiento.
// 3. Recuperación tras un PUT con nuevas credenciales (limpia enfriamiento y recupera conectividad).
func TestTokenCooldown_MaxOneAttemptAndRecoveryOnPUT(t *testing.T) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Error abriendo DB: %v", err)
	}
	defer db.Close()

	cryptoSvc, _ := crypto.NewCryptoService("12345678901234567890123456789012")
	repo := company.NewRepository(db)
	tm := NewTokenManager(repo, cryptoSvc)
	companySvc := company.NewService(repo, cryptoSvc, tm)

	var tokenRequests atomic.Int64
	var returnValidToken atomic.Bool

	// Servidor HTTP simulado que emula el endpoint de autenticación OAuth2 de SUNAT
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if !returnValidToken.Load() {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":             "invalid_grant",
				"error_description": "Error en la autenticacion del usuario",
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "token_valido_despues_de_put_123",
			"expires_in":   3600,
		})
	}))
	defer mockServer.Close()

	tm.SetAuthURL(mockServer.URL)

	ruc := "20123456789"
	_, err = companySvc.CreateEmpresa(&company.CreateEmpresaRequest{
		RUC:          ruc,
		RazonSocial:  "TEST COOLDOWN S.A.C.",
		UsuarioSOL:   "USUARIO1",
		ClaveSOL:     "CLAVE_ERRONEA_INICIAL",
		ClientID:     "client-id-test",
		ClientSecret: "client-secret-test",
	})
	if err != nil {
		t.Fatalf("Error al registrar empresa: %v", err)
	}

	// 1. Primer intento: Debe fallar en SUNAT y activar enfriamiento preventivo de 15 minutos
	tok, err := tm.GetToken(ruc)
	if err == nil {
		t.Fatalf("Se esperaba fallo por credenciales incorrectas, pero se obtuvo token: %s", tok)
	}
	if tokenRequests.Load() != 1 {
		t.Fatalf("Se esperaba exactamente 1 intento HTTP a SUNAT, pero se registraron %d", tokenRequests.Load())
	}

	// Verificar registro en SQLite
	emp, err := repo.GetByRUC(ruc)
	if err != nil || emp.EstadoCredenciales != "INVALIDAS" || emp.BloqueadoHasta == nil {
		t.Fatalf("La base de datos debió marcarse como INVALIDAS con fecha BloqueadoHasta. Estado actual: %+v", emp)
	}

	// 2. Segundo intento inmediato: En periodo de enfriamiento, debe abortar localmente SIN peticiones de red
	tok2, err2 := tm.GetToken(ruc)
	if err2 == nil {
		t.Fatalf("Se esperaba rechazo por enfriamiento pero se obtuvo token: %s", tok2)
	}
	if !strings.Contains(err2.Error(), "enfriamiento") {
		t.Fatalf("El error devuelto debe indicar enfriamiento de seguridad: %v", err2)
	}
	if tokenRequests.Load() != 1 {
		t.Fatalf("Durante el enfriamiento NO debe haber nuevas peticiones a SUNAT. Contador: %d", tokenRequests.Load())
	}

	// 3. Tercer intento: Sigue bloqueado, contador de red permanece inmutable en 1
	_, _ = tm.GetToken(ruc)
	if tokenRequests.Load() != 1 {
		t.Fatalf("El tercer intento no debió disparar peticiones de red. Contador: %d", tokenRequests.Load())
	}

	// 4. Actualización de credenciales vía PUT (simulado con UpdateEmpresa)
	returnValidToken.Store(true) // En el portal SUNAT ya se actualizaron las credenciales
	nuevaClave := "CLAVE_NUEVA_CORRECTA"
	_, err = companySvc.UpdateEmpresa(ruc, &company.UpdateEmpresaRequest{
		RazonSocial:  "TEST COOLDOWN S.A.C.",
		UsuarioSOL:   "USUARIO1",
		ClaveSOL:     nuevaClave,
		ClientID:     "client-id-test",
		ClientSecret: "client-secret-test",
	})
	if err != nil {
		t.Fatalf("Error al actualizar empresa con PUT: %v", err)
	}

	// Confirmar que el enfriamiento fue levantado tras el PUT en memoria y BD
	inCooldown, _, _ := tm.IsCooldown(ruc)
	if inCooldown {
		t.Fatalf("El enfriamiento en memoria debió levantarse tras el PUT")
	}

	empActualizada, _ := repo.GetByRUC(ruc)
	if empActualizada.BloqueadoHasta != nil || empActualizada.EstadoCredenciales != "PENDIENTE" {
		t.Fatalf("El estado en BD debió restaurarse a PENDIENTE y BloqueadoHasta=nil tras PUT: %+v", empActualizada)
	}

	// 5. Cuarto intento: Tras el PUT, se permite una nueva llamada HTTP a SUNAT (contador sube a 2) y es exitosa
	tokExito, errExito := tm.GetToken(ruc)
	if errExito != nil {
		t.Fatalf("Se esperaba éxito tras actualizar credenciales pero falló: %v", errExito)
	}
	if tokExito != "token_valido_despues_de_put_123" {
		t.Fatalf("Token obtenido incorrecto: %s", tokExito)
	}
	if tokenRequests.Load() != 2 {
		t.Fatalf("Se esperaban exactamente 2 peticiones a SUNAT (1 inicial fallida + 1 exitosa tras PUT), pero ocurrieron %d", tokenRequests.Load())
	}

	// 6. Quinto intento: Token obtenido de la memoria caché, sin llamadas HTTP adicionales (contador permanece en 2)
	tokCache, _ := tm.GetToken(ruc)
	if tokCache != "token_valido_despues_de_put_123" {
		t.Fatalf("Token en caché incorrecto: %s", tokCache)
	}
	if tokenRequests.Load() != 2 {
		t.Fatalf("El token en caché no debió generar peticiones HTTP a SUNAT. Contador: %d", tokenRequests.Load())
	}
}

func TestTokenManager_SQLiteEncryptedPersistence(t *testing.T) {
	repo, cryptoSvc := setupTokenTestDB(t)
	ruc := "20000000001"
	rawToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.token_secreto_simulado"

	// Guardar directamente simulando obtención de token
	expiresAt := time.Now().Add(1 * time.Hour)
	tokenEnc, err := cryptoSvc.Encrypt(rawToken)
	if err != nil {
		t.Fatalf("Error al cifrar token con AES-GCM: %v", err)
	}

	_, err = repo.DB().Exec(
		"INSERT INTO tokens_cache (ruc, token_enc, expires_at) VALUES (?, ?, ?)",
		ruc, tokenEnc, expiresAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("Error al insertar token cifrado en BD: %v", err)
	}

	// Verificar que el registro en SQLite NO contenga el token en claro ni "eyJ"
	var dbTokenEnc, dbExpStr string
	err = repo.DB().QueryRow("SELECT token_enc, expires_at FROM tokens_cache WHERE ruc = ?", ruc).Scan(&dbTokenEnc, &dbExpStr)
	if err != nil {
		t.Fatalf("Error al leer token_enc de SQLite: %v", err)
	}

	if dbTokenEnc == rawToken || strings.Contains(dbTokenEnc, "eyJ") {
		t.Fatalf("El token en SQLite no debe estar en texto plano ni contener 'eyJ': %s", dbTokenEnc)
	}

	// Instanciar un segundo TokenManager (simulando reinicio de la aplicación con memoria vacía)
	tm2 := NewTokenManager(repo, cryptoSvc)
	tokenRecuperado, err := tm2.GetToken(ruc)
	if err != nil {
		t.Fatalf("tm2 debió recuperar el token de SQLite cifrado: %v", err)
	}

	if tokenRecuperado != rawToken {
		t.Fatalf("Se esperaba %s pero se descifró %s", rawToken, tokenRecuperado)
	}

	// Verificar invalidación
	tm2.InvalidateToken(ruc)

	var count int
	_ = repo.DB().QueryRow("SELECT COUNT(*) FROM tokens_cache WHERE ruc = ?", ruc).Scan(&count)
	if count != 0 {
		t.Fatalf("Tras InvalidateToken, el registro en SQLite debió eliminarse, count = %d", count)
	}
}

