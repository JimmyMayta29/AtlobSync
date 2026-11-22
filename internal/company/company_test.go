package company

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SwodLore/sire-api/internal/crypto"
	_ "modernc.org/sqlite"
)

type dummyTokenProvider struct{}

func (d *dummyTokenProvider) GetToken(ruc string) (string, error) {
	return "dummy_token_test", nil
}

func (d *dummyTokenProvider) InvalidateToken(ruc string) {}

func (d *dummyTokenProvider) IsCooldown(ruc string) (bool, time.Time, string) {
	return false, time.Time{}, ""
}

func (d *dummyTokenProvider) ClearCooldown(ruc string) {}

func setupTestDB(t *testing.T) (*sql.DB, *Repository, *Service) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Error abriendo SQLite en memoria: %v", err)
	}

	ddl := `
	CREATE TABLE empresas (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ruc TEXT NOT NULL,
		razon_social TEXT NOT NULL,
		usuario_sol TEXT NOT NULL,
		clave_sol_enc TEXT NOT NULL,
		client_id TEXT NOT NULL,
		client_secret_enc TEXT NOT NULL,
		client_id_cpe TEXT DEFAULT '',
		client_secret_cpe_enc TEXT DEFAULT '',
		estado_credenciales TEXT DEFAULT 'PENDIENTE',
		bloqueado_hasta DATETIME NULL,
		mensaje_auth TEXT DEFAULT '',
		activo INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		deleted_at DATETIME NULL
	);
	CREATE UNIQUE INDEX idx_empresas_ruc_active ON empresas(ruc) WHERE deleted_at IS NULL;
	`
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("Error creando tablas en memoria: %v", err)
	}

	cryptoSvc, _ := crypto.NewCryptoService("clave_test_32_bytes_super_secreta")
	repo := NewRepository(db)
	svc := NewService(repo, cryptoSvc, &dummyTokenProvider{})

	return db, repo, svc
}

func TestValidateRUC(t *testing.T) {
	validos := []string{"20000000001", "10000000001", "15123456789"}
	for _, ruc := range validos {
		if err := ValidateRUC(ruc); err != nil {
			t.Errorf("RUC %s debería ser válido: %v", ruc, err)
		}
	}

	invalidos := []string{"123", "2000000000", "200000000015", "30000000001", "abcdefghijk"}
	for _, ruc := range invalidos {
		if err := ValidateRUC(ruc); err == nil {
			t.Errorf("RUC %s debería ser inválido", ruc)
		}
	}
}

func TestEmpresaCRUD(t *testing.T) {
	db, _, svc := setupTestDB(t)
	defer db.Close()

	// 1. Crear empresa
	req := &CreateEmpresaRequest{
		RUC:          "20000000001",
		RazonSocial:  "EMPRESA DEMO S.A.C.",
		UsuarioSOL:   "DEMOUSER",
		ClaveSOL:     "ClaveSecreta123",
		ClientID:     "client-id-test",
		ClientSecret: "client-secret-test",
	}

	creada, err := svc.CreateEmpresa(req)
	if err != nil {
		t.Fatalf("Error al crear empresa: %v", err)
	}
	if creada.RUC != req.RUC {
		t.Fatalf("Se esperaba RUC %s, se obtuvo %s", req.RUC, creada.RUC)
	}
	if creada.ClaveSOL != "******" || creada.ClientSecret != "******" {
		t.Fatalf("Las credenciales deben estar enmascaradas en la respuesta")
	}

	// 2. Obtener empresa
	obtenida, err := svc.GetEmpresa("20000000001")
	if err != nil {
		t.Fatalf("Error al obtener empresa: %v", err)
	}
	if obtenida.RazonSocial != req.RazonSocial {
		t.Fatalf("Razón social incorrecta")
	}

	// 3. Listar empresas
	lista, err := svc.ListEmpresas()
	if err != nil || len(lista) != 1 {
		t.Fatalf("Error al listar empresas: %v, total: %d", err, len(lista))
	}

	// 4. Actualizar empresa
	updateReq := &UpdateEmpresaRequest{
		RazonSocial:  "EMPRESA DEMO MODIFICADA S.A.C.",
		UsuarioSOL:   "DEMOUSER",
		ClientID:     "client-id-test",
		ClaveSOL:     "NuevaClave456",
		ClientSecret: "",
	}
	actualizada, err := svc.UpdateEmpresa("20000000001", updateReq)
	if err != nil {
		t.Fatalf("Error al actualizar empresa: %v", err)
	}
	if actualizada.RazonSocial != "EMPRESA DEMO MODIFICADA S.A.C." {
		t.Fatalf("La razón social no se actualizó")
	}

	// 5. Eliminar empresa (soft delete)
	err = svc.DeleteEmpresa("20000000001")
	if err != nil {
		t.Fatalf("Error al eliminar empresa: %v", err)
	}

	// Verificar que ya no aparece en lista
	listaPost, _ := svc.ListEmpresas()
	if len(listaPost) != 0 {
		t.Fatalf("La empresa eliminada aún aparece en la lista")
	}
}

func TestEndpointsNeverReturnSecrets(t *testing.T) {
	_, _, svc := setupTestDB(t)

	claveSecreta := "MiClaveSuperSecreta123"
	secretPrivado := "MiSecretSuperSecreto456"

	// 1. Crear empresa
	creada, err := svc.CreateEmpresa(&CreateEmpresaRequest{
		RUC:          "20000000001",
		RazonSocial:  "EMPRESA SEGURA S.A.C.",
		UsuarioSOL:   "USUARIO1",
		ClaveSOL:     claveSecreta,
		ClientID:     "client-id-seguro",
		ClientSecret: secretPrivado,
	})
	if err != nil {
		t.Fatalf("Error al crear empresa: %v", err)
	}

	// Serializar a JSON y verificar que nunca contenga los secretos en claro
	jsonBytes, _ := json.Marshal(creada)
	jsonStr := string(jsonBytes)

	if strings.Contains(jsonStr, claveSecreta) {
		t.Fatalf("La respuesta de CreateEmpresa contiene la clave SOL en texto plano: %s", jsonStr)
	}
	if strings.Contains(jsonStr, secretPrivado) {
		t.Fatalf("La respuesta de CreateEmpresa contiene el client secret en texto plano: %s", jsonStr)
	}
	if creada.ClaveSOL != "******" || creada.ClientSecret != "******" {
		t.Fatalf("Los campos clave_sol y client_secret deben devolverse siempre enmascarados con '******'")
	}

	// 2. Consultar por RUC
	obtenida, err := svc.GetEmpresa("20000000001")
	if err != nil {
		t.Fatalf("Error al obtener empresa: %v", err)
	}
	jsonBytes, _ = json.Marshal(obtenida)
	jsonStr = string(jsonBytes)

	if strings.Contains(jsonStr, claveSecreta) || strings.Contains(jsonStr, secretPrivado) {
		t.Fatalf("GetEmpresa filtró secretos en claro: %s", jsonStr)
	}

	// 3. Listar empresas
	lista, err := svc.ListEmpresas()
	if err != nil {
		t.Fatalf("Error al listar empresas: %v", err)
	}
	jsonBytes, _ = json.Marshal(lista)
	jsonStr = string(jsonBytes)

	if strings.Contains(jsonStr, claveSecreta) || strings.Contains(jsonStr, secretPrivado) {
		t.Fatalf("ListEmpresas filtró secretos en claro: %s", jsonStr)
	}

	// 4. Actualizar empresa
	nuevaClave := "NuevaClaveUltraSecreta789"
	actualizada, err := svc.UpdateEmpresa("20000000001", &UpdateEmpresaRequest{
		RazonSocial: "EMPRESA SEGURA MODIFICADA",
		ClaveSOL:    nuevaClave,
	})
	if err != nil {
		t.Fatalf("Error al actualizar empresa: %v", err)
	}
	jsonBytes, _ = json.Marshal(actualizada)
	jsonStr = string(jsonBytes)

	if strings.Contains(jsonStr, nuevaClave) || strings.Contains(jsonStr, secretPrivado) {
		t.Fatalf("UpdateEmpresa filtró secretos en claro: %s", jsonStr)
	}
}

func TestTestConnection_VerifiesSireGateway(t *testing.T) {
	db, repo, svc := setupTestDB(t)
	defer db.Close()

	_, err := svc.CreateEmpresa(&CreateEmpresaRequest{
		RUC:          "20000000001",
		RazonSocial:  "EMPRESA DEMO S.A.C.",
		UsuarioSOL:   "DEMOUSER",
		ClaveSOL:     "Clave123",
		ClientID:     "cid-test",
		ClientSecret: "csec-test",
	})
	if err != nil {
		t.Fatalf("error creando empresa: %v", err)
	}

	// Caso 1: OAuth2 OK pero SIRE devuelve 401 (ej. credenciales Web en vez de Desktop)
	svc.SetSireVerifier(func(ruc string) (int, string, error) {
		return 401, "<html>401 Authorization Required</html>", nil
	})
	if _, err := svc.TestConnection("20000000001"); err == nil {
		t.Fatalf("se esperaba error cuando SIRE devuelve 401")
	}
	emp, _ := repo.GetByRUC("20000000001")
	if emp.EstadoCredenciales != "SIN_PERMISO_SIRE" {
		t.Fatalf("se esperaba estado SIN_PERMISO_SIRE, se obtuvo %s", emp.EstadoCredenciales)
	}

	// Caso 2: OAuth2 OK y SIRE devuelve 200
	svc.SetSireVerifier(func(ruc string) (int, string, error) {
		return 200, `[{"numEjercicio":"2026"}]`, nil
	})
	res, err := svc.TestConnection("20000000001")
	if err != nil {
		t.Fatalf("no se esperaba error con SIRE 200: %v", err)
	}
	if res["sire_ok"] != true || res["oauth_ok"] != true {
		t.Fatalf("respuesta inesperada: %v", res)
	}
	emp, _ = repo.GetByRUC("20000000001")
	if emp.EstadoCredenciales != "VALIDO" {
		t.Fatalf("se esperaba estado VALIDO, se obtuvo %s", emp.EstadoCredenciales)
	}
}

func TestCompanyLimitCheckerBlocksWhenPlanReached(t *testing.T) {
	db, _, svc := setupTestDB(t)
	defer db.Close()

	svc.SetCompanyLimitChecker(func(currentCount int) error {
		if currentCount >= 1 {
			return errors.New("tu plan actual permite hasta 1 empresa(s)")
		}
		return nil
	})

	_, err := svc.CreateEmpresa(&CreateEmpresaRequest{
		RUC:          "20000000001",
		RazonSocial:  "EMPRESA UNO S.A.C.",
		UsuarioSOL:   "USER1",
		ClaveSOL:     "Clave123",
		ClientID:     "cid-1",
		ClientSecret: "csec-1",
	})
	if err != nil {
		t.Fatalf("la primera empresa debía crearse sin error: %v", err)
	}

	_, err = svc.CreateEmpresa(&CreateEmpresaRequest{
		RUC:          "20000000002",
		RazonSocial:  "EMPRESA DOS S.A.C.",
		UsuarioSOL:   "USER2",
		ClaveSOL:     "Clave123",
		ClientID:     "cid-2",
		ClientSecret: "csec-2",
	})
	if err == nil || !strings.Contains(err.Error(), "1 empresa") {
		t.Fatalf("la segunda empresa debía bloquearse por límite de plan, obtenido: %v", err)
	}
}


