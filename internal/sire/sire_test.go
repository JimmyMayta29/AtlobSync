package sire

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SwodLore/sire-api/internal/sunat"
	_ "modernc.org/sqlite"
)

type mockTokenProvider struct {
	cooldownRUC string
}

func (m *mockTokenProvider) GetToken(ruc string) (string, error) {
	return "mock_token_sire", nil
}

func (m *mockTokenProvider) InvalidateToken(ruc string) {}

func (m *mockTokenProvider) IsCooldown(ruc string) (bool, time.Time, string) {
	if m.cooldownRUC == ruc {
		return true, time.Now().Add(15 * time.Minute), "Credenciales SOL inválidas"
	}
	return false, time.Time{}, ""
}

func (m *mockTokenProvider) ClearCooldown(ruc string) {
	if m.cooldownRUC == ruc {
		m.cooldownRUC = ""
	}
}

func setupTestDB(t *testing.T) (*sql.DB, string) {
	tempDir, err := os.MkdirTemp("", "sire_test_*")
	if err != nil {
		t.Fatalf("Error al crear temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Error al abrir db: %v", err)
	}

	ddl := `
	CREATE TABLE IF NOT EXISTS descargas_sire (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		empresa_ruc TEXT NOT NULL,
		cod_libro TEXT NOT NULL,
		libro TEXT NOT NULL,
		periodo TEXT NOT NULL,
		ticket TEXT NOT NULL,
		estado TEXT NOT NULL,
		mensaje_sunat TEXT DEFAULT '',
		nombre_archivo TEXT DEFAULT '',
		ruta_local TEXT DEFAULT '',
		tamano_bytes INTEGER DEFAULT 0,
		hash_archivo TEXT DEFAULT '',
		reintentos INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(empresa_ruc, ticket)
	);

	CREATE TABLE IF NOT EXISTS empresas (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ruc TEXT NOT NULL,
		razon_social TEXT NOT NULL,
		activo INTEGER DEFAULT 1,
		deleted_at DATETIME NULL
	);

	INSERT INTO empresas (ruc, razon_social, activo) VALUES ('20000000001', 'TEST SAC', 1);
	`
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("Error al crear tabla: %v", err)
	}

	return db, tempDir
}

func createDummyZip() []byte {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("LE20000000001-202410-140000.txt")
	_, _ = f.Write([]byte("20000000001|01|F001|00000001|2024-10-15|100.00|18.00|118.00\r\n"))
	_ = w.Close()
	return buf.Bytes()
}

func TestSireFullAsyncFlowAndWorker(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer db.Close()
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "descargas")

	var mu sync.Mutex
	ticketPollCount := 0
	dummyZip := createDummyZip()

	// Mock SUNAT Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		w.Header().Set("Content-Type", "application/json")

		// 1. Exportar Propuesta
		if strings.Contains(path, "exportapropuesta") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"numTicket": "20240212345678",
			})
			return
		}

		// 2. Consulta Estado Tickets
		if strings.Contains(path, "consultaestadotickets") {
			mu.Lock()
			ticketPollCount++
			currentPoll := ticketPollCount
			mu.Unlock()

			// Validar parámetros obligatorios según Ventas v31 5.16 y Compras v28 5.31
			if r.URL.Query().Get("codOrigenEnvio") != "2" {
				t.Errorf("consultaestadotickets debe recibir codOrigenEnvio=2, recibido: '%s'", r.URL.Query().Get("codOrigenEnvio"))
			}
			if r.URL.Query().Get("codLibro") != "140000" {
				t.Errorf("consultaestadotickets debe recibir codLibro=140000 para Ventas, recibido: '%s'", r.URL.Query().Get("codLibro"))
			}

			if currentPoll == 1 {
				// Primer intento: SUNAT responde "02" (En proceso)
				resp := map[string]interface{}{
					"registros": []map[string]interface{}{
						{
							"numTicket":        "20240212345678",
							"codEstadoProceso": "02",
							"desEstadoProceso": "En Proceso",
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// Segundo intento: SUNAT responde "06" (Terminado)
			resp := map[string]interface{}{
				"registros": []map[string]interface{}{
					{
						"numTicket":        "20240212345678",
						"codEstadoProceso": "06",
						"desEstadoProceso": "Terminado",
						"perTributario":    "202410",
						"codProceso":       "1",
						"archivoReporte": []map[string]string{
							{
								"nomArchivoReporte":     "LE20000000001-202410-140000.zip",
								"codTipoArchivoReporte": "00",
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// 3. Descarga Archivo Reporte
		if strings.Contains(path, "archivoreporte") {
			// Validar los 6 parámetros exigidos por Ventas v31 5.17
			q := r.URL.Query()
			if q.Get("nomArchivoReporte") != "LE20000000001-202410-140000.zip" {
				t.Errorf("nomArchivoReporte incorrecto: %s", q.Get("nomArchivoReporte"))
			}
			if q.Get("codTipoArchivoReporte") != "00" {
				t.Errorf("codTipoArchivoReporte incorrecto: %s", q.Get("codTipoArchivoReporte"))
			}
			if q.Get("perTributario") != "202410" {
				t.Errorf("perTributario incorrecto: %s", q.Get("perTributario"))
			}
			if q.Get("codProceso") != "1" {
				t.Errorf("codProceso incorrecto: %s", q.Get("codProceso"))
			}
			if q.Get("numTicket") != "20240212345678" {
				t.Errorf("numTicket incorrecto: %s", q.Get("numTicket"))
			}
			if q.Get("codLibro") != "140000" {
				t.Errorf("codLibro incorrecto: %s", q.Get("codLibro"))
			}
			if r.Header.Get("Accept") != "application/json" {
				t.Errorf("Header Accept debe ser application/json: %s", r.Header.Get("Accept"))
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Header Content-Type debe ser application/json: %s", r.Header.Get("Content-Type"))
			}

			w.Header().Set("Content-Type", "application/zip")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(dummyZip)
			return
		}

		// 4. Detalle de Propuesta Paginado
		if strings.Contains(path, "comprobantes") {
			resp := map[string]interface{}{
				"paginacion": map[string]interface{}{
					"page":           1,
					"perPage":        20,
					"totalRegistros": 1,
				},
				"registros": []map[string]interface{}{
					{
						"numSerie":   "F001",
						"numCDP":     "00000001",
						"mtoTotal":   118.00,
						"codTipoCDP": "01",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Configurar clientes
	mockToken := &mockTokenProvider{}
	sunatClient := sunat.NewClient(mockToken)
	sunatClient.SetBaseURL(server.URL)

	sireClient := NewSunatSireClient(sunatClient)
	repo := NewRepository(db)
	worker := NewWorker(repo, sireClient, mockToken, storageDir)
	service := NewService(repo, sireClient, worker, storageDir)

	worker.Start()
	defer worker.Stop()

	// Paso 1: Solicitar Exportación
	const testRUC = "20000000001"
	const testPeriodo = "202410"

	descarga, err := service.SolicitarExportacion(testRUC, "VENTAS", testPeriodo, FiltrosExportacion{CodTipoArchivo: "0"})
	if err != nil {
		t.Fatalf("Error al solicitar exportación: %v", err)
	}

	if descarga.Ticket != "20240212345678" {
		t.Fatalf("Ticket esperado '20240212345678', obtenido: %s", descarga.Ticket)
	}

	// Paso 2: Esperar a que el worker en segundo plano termine el polling y guarde el archivo
	var finalDescarga *DescargaSire
	for i := 0; i < 30; i++ {
		time.Sleep(200 * time.Millisecond)
		d, err := repo.GetByID(descarga.ID)
		if err == nil && d.Estado == string(EstadoCompletado) {
			finalDescarga = d
			break
		}
	}

	if finalDescarga == nil {
		t.Fatalf("El worker no completó la descarga dentro del tiempo límite")
	}

	// Paso 3: Verificar que la ruta guardada en BD sea relativa al storageDir, exista en disco y el hash sea válido
	if filepath.IsAbs(finalDescarga.RutaLocal) {
		t.Fatalf("Se esperaba ruta relativa portable al storageDir, se obtuvo absoluta: %s", finalDescarga.RutaLocal)
	}
	expectedDiskPath := filepath.Join(storageDir, filepath.FromSlash(finalDescarga.RutaLocal))
	if _, err := os.Stat(expectedDiskPath); os.IsNotExist(err) {
		t.Fatalf("El archivo físico no existe en disco: %s", expectedDiskPath)
	}

	if finalDescarga.TamanoBytes != int64(len(dummyZip)) {
		t.Fatalf("Tamaño esperado %d, obtenido %d", len(dummyZip), finalDescarga.TamanoBytes)
	}

	if len(finalDescarga.HashArchivo) != 64 {
		t.Fatalf("Hash SHA256 inválido: %s", finalDescarga.HashArchivo)
	}

	// Paso 4: Probar descarga segura de archivo por ID (resuelve ruta relativa al storageDir)
	dRetrieved, absPath, err := service.ObtenerArchivoParaDescarga(descarga.ID, testRUC)
	if err != nil {
		t.Fatalf("Error al obtener archivo para descarga: %v", err)
	}
	evalExpected, _ := filepath.EvalSymlinks(expectedDiskPath)
	if evalExpected == "" {
		evalExpected = expectedDiskPath
	}
	if dRetrieved.ID != descarga.ID || !strings.EqualFold(filepath.Clean(absPath), filepath.Clean(evalExpected)) {
		t.Fatalf("Ruta o descarga incorrecta: got=%s want=%s", absPath, evalExpected)
	}

	// Paso 5: Consultar detalle paginado de la propuesta
	detalle, err := service.ConsultarDetallePropuesta(testRUC, "VENTAS", testPeriodo, FiltrosPropuestaDetalle{
		Page:    "1",
		PerPage: "20",
	})
	if err != nil {
		t.Fatalf("Error al consultar detalle paginado: %v", err)
	}
	if detalle == nil {
		t.Fatalf("Se esperaba respuesta con datos de propuesta")
	}
}

func TestSireWorkerStartupRecovery(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer db.Close()
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "descargas")
	dummyZip := createDummyZip()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "consultaestadotickets") {
			if r.URL.Query().Get("codLibro") != "080000" {
				t.Errorf("Se esperaba codLibro=080000 en consulta de compras, recibido: %s", r.URL.Query().Get("codLibro"))
			}
			if r.URL.Query().Get("codOrigenEnvio") != "2" {
				t.Errorf("Se esperaba codOrigenEnvio=2, recibido: %s", r.URL.Query().Get("codOrigenEnvio"))
			}
			resp := map[string]interface{}{
				"registros": []map[string]interface{}{
					{
						"numTicket":        "20240299999999",
						"codEstadoProceso": "06",
						"desEstadoProceso": "Terminado",
						"perTributario":    "202411",
						"codProceso":       "1",
						"archivoReporte": []map[string]string{
							{
								"nomArchivoReporte":     "LE20000000001-202411-080000.zip",
								"codTipoArchivoReporte": "00",
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if strings.Contains(r.URL.Path, "archivoreporte") {
			w.Header().Set("Content-Type", "application/zip")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(dummyZip)
			return
		}
	}))
	defer server.Close()

	mockToken := &mockTokenProvider{}
	sunatClient := sunat.NewClient(mockToken)
	sunatClient.SetBaseURL(server.URL)

	sireClient := NewSunatSireClient(sunatClient)
	repo := NewRepository(db)

	// Simular un ticket que quedó en "EN_PROCESO" antes de que el servidor se apague
	pendiente := &DescargaSire{
		EmpresaRUC: "20000000001",
		CodLibro:   "080000",
		Libro:      "COMPRAS",
		Periodo:    "202411",
		Ticket:     "20240299999999",
		Estado:     string(EstadoEnProceso),
	}
	if err := repo.Create(pendiente); err != nil {
		t.Fatalf("Error al insertar ticket pendiente: %v", err)
	}

	// Al iniciar un nuevo worker, debe recuperar automáticamente el ticket pendiente
	worker := NewWorker(repo, sireClient, mockToken, storageDir)
	worker.Start()
	defer worker.Stop()

	var recovered *DescargaSire
	for i := 0; i < 30; i++ {
		time.Sleep(200 * time.Millisecond)
		d, err := repo.GetByID(pendiente.ID)
		if err == nil && d.Estado == string(EstadoCompletado) {
			recovered = d
			break
		}
	}

	if recovered == nil {
		t.Fatalf("El worker no recuperó ni completó el ticket pendiente tras reinicio")
	}

	if filepath.IsAbs(recovered.RutaLocal) {
		t.Fatalf("Se esperaba ruta relativa portable tras recuperación, se obtuvo: %s", recovered.RutaLocal)
	}
	recoveredDiskPath := filepath.Join(storageDir, filepath.FromSlash(recovered.RutaLocal))
	if _, err := os.Stat(recoveredDiskPath); os.IsNotExist(err) {
		t.Fatalf("El archivo recuperado no fue guardado en disco: %s", recoveredDiskPath)
	}
}

func TestSireWorkerMaxRetriesError(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer db.Close()
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "descargas")

	// SUNAT siempre responde "02" (En proceso)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"registros": []map[string]interface{}{
				{
					"numTicket":        "20240277777777",
					"codEstadoProceso": "02",
					"desEstadoProceso": "En Proceso",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mockToken := &mockTokenProvider{}
	sunatClient := sunat.NewClient(mockToken)
	sunatClient.SetBaseURL(server.URL)

	sireClient := NewSunatSireClient(sunatClient)
	repo := NewRepository(db)

	// Ticket que ya tiene 40 reintentos
	ticket40 := &DescargaSire{
		EmpresaRUC: "20000000001",
		CodLibro:   "140000",
		Libro:      "VENTAS",
		Periodo:    "202410",
		Ticket:     "20240277777777",
		Estado:     string(EstadoEnProceso),
		Reintentos: 40,
	}
	if err := repo.Create(ticket40); err != nil {
		t.Fatalf("Error al crear ticket: %v", err)
	}

	worker := NewWorker(repo, sireClient, mockToken, storageDir)
	worker.processJob(ticket40.ID)

	updated, err := repo.GetByID(ticket40.ID)
	if err != nil {
		t.Fatalf("Error al obtener ticket: %v", err)
	}

	if updated.Estado != string(EstadoError) {
		t.Fatalf("Se esperaba estado ERROR tras 40 intentos, se obtuvo: %s", updated.Estado)
	}

	if !strings.Contains(updated.MensajeSUNAT, "40 intentos") {
		t.Fatalf("Mensaje de error debe mencionar los 40 intentos agotados: %s", updated.MensajeSUNAT)
	}
}

func TestSireWorkerDeactivatedCompanyIgnored(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer db.Close()
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "descargas")

	// Desactivar empresa (activo = 0)
	_, err := db.Exec("UPDATE empresas SET activo = 0 WHERE ruc = '20000000001'")
	if err != nil {
		t.Fatalf("Error al desactivar empresa: %v", err)
	}

	var sunatCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sunatCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mockToken := &mockTokenProvider{}
	sunatClient := sunat.NewClient(mockToken)
	sunatClient.SetBaseURL(server.URL)

	sireClient := NewSunatSireClient(sunatClient)
	repo := NewRepository(db)

	ticketInactivo := &DescargaSire{
		EmpresaRUC: "20000000001",
		CodLibro:   "140000",
		Libro:      "VENTAS",
		Periodo:    "202410",
		Ticket:     "20240255555555",
		Estado:     string(EstadoEnProceso),
	}
	if err := repo.Create(ticketInactivo); err != nil {
		t.Fatalf("Error al crear ticket: %v", err)
	}

	worker := NewWorker(repo, sireClient, mockToken, storageDir)
	worker.processJob(ticketInactivo.ID)

	// Debe haber ignorado el ticket sin llamar a SUNAT
	if sunatCalled {
		t.Fatalf("El worker NO debió llamar a SUNAT para una empresa desactivada")
	}

	// Y GetPendingTickets no debe incluir a empresas inactivas
	pending, err := repo.GetPendingTickets()
	if err != nil {
		t.Fatalf("Error al obtener pending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("GetPendingTickets debió ignorar el ticket de la empresa inactiva, obtuvo: %d", len(pending))
	}
}

func TestSireWorker_CooldownIgnoresTicketsWithoutConsumingRetries(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer db.Close()
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "descargas")

	var sunatCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sunatCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Proveedor con RUC en enfriamiento
	mockToken := &mockTokenProvider{
		cooldownRUC: "20000000001",
	}

	sunatClient := sunat.NewClient(mockToken)
	sunatClient.SetBaseURL(server.URL)

	sireClient := NewSunatSireClient(sunatClient)
	repo := NewRepository(db)

	ticketCooldown := &DescargaSire{
		EmpresaRUC: "20000000001",
		CodLibro:   "140000",
		Libro:      "VENTAS",
		Periodo:    "202410",
		Ticket:     "20240288888888",
		Estado:     string(EstadoEnProceso),
		Reintentos: 0,
	}
	if err := repo.Create(ticketCooldown); err != nil {
		t.Fatalf("Error al crear ticket: %v", err)
	}

	worker := NewWorker(repo, sireClient, mockToken, storageDir)
	worker.processJob(ticketCooldown.ID)

	// Verificaciones:
	// 1. NO debió llamar a SUNAT
	if sunatCalled {
		t.Fatalf("El worker NO debió llamar a SUNAT mientras la empresa está en enfriamiento")
	}

	// 2. El ticket debe permanecer en espera SIN consumir reintentos
	updated, err := repo.GetByID(ticketCooldown.ID)
	if err != nil {
		t.Fatalf("Error al consultar ticket: %v", err)
	}
	if updated.Reintentos != 0 {
		t.Fatalf("Los reintentos no debieron incrementarse durante el enfriamiento. Reintentos actuales: %d", updated.Reintentos)
	}
	if updated.Estado != string(EstadoEnProceso) {
		t.Fatalf("El ticket debe permanecer en su estado actual, no fallar definitivamente. Estado: %s", updated.Estado)
	}
}

func TestDescargarArchivoReporte_VentasV31_y_ComprasV28(t *testing.T) {
	var lastAcceptHeader string
	var lastContentType string
	var lastCodTipo string
	var lastCodLibro string
	var lastNomArchivo string
	var lastPerTributario string
	var lastCodProceso string
	var lastNumTicket string
	dummyZip := createDummyZip()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastAcceptHeader = r.Header.Get("Accept")
		lastContentType = r.Header.Get("Content-Type")
		lastCodTipo = r.URL.Query().Get("codTipoArchivoReporte")
		lastCodLibro = r.URL.Query().Get("codLibro")
		lastNomArchivo = r.URL.Query().Get("nomArchivoReporte")
		lastPerTributario = r.URL.Query().Get("perTributario")
		lastCodProceso = r.URL.Query().Get("codProceso")
		lastNumTicket = r.URL.Query().Get("numTicket")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(dummyZip)
	}))
	defer server.Close()

	sunatClient := sunat.NewClient(&mockTokenProvider{})
	sunatClient.BaseURL = server.URL
	sireClient := NewSunatSireClient(sunatClient)

	// Caso A: RVIE (140000) - Manual Ventas v31, Secc. 5.17 (Pág. 47-48)
	// Exige: nomArchivoReporte, codTipoArchivoReporte (o null), perTributario, codProceso, numTicket, codLibro=140000
	bytesRVIE, err := sireClient.DescargarArchivoReporte("20000000001", "archivo_ventas.zip", "01", "202410", "1", "20240212345678", "140000")
	if err != nil {
		t.Fatalf("Error en RVIE: %v", err)
	}
	if len(bytesRVIE) == 0 {
		t.Errorf("Esperaba contenido binario")
	}
	if lastAcceptHeader != "application/json" {
		t.Errorf("Header Accept esperado 'application/json', recibido: '%s'", lastAcceptHeader)
	}
	if lastContentType != "application/json" {
		t.Errorf("Header Content-Type esperado 'application/json', recibido: '%s'", lastContentType)
	}
	if lastCodTipo != "01" {
		t.Errorf("Query param codTipoArchivoReporte esperado '01', recibido: '%s'", lastCodTipo)
	}
	if lastCodLibro != "140000" {
		t.Errorf("RVIE debe enviar codLibro='140000', recibido: '%s'", lastCodLibro)
	}
	if lastNomArchivo != "archivo_ventas.zip" {
		t.Errorf("nomArchivoReporte incorrecto: %s", lastNomArchivo)
	}
	if lastPerTributario != "202410" {
		t.Errorf("perTributario incorrecto: %s", lastPerTributario)
	}
	if lastCodProceso != "1" {
		t.Errorf("codProceso incorrecto: %s", lastCodProceso)
	}
	if lastNumTicket != "20240212345678" {
		t.Errorf("numTicket incorrecto: %s", lastNumTicket)
	}

	// Caso B: RCE (080000) - Manual Compras v28, Secc. 5.32 (Pág. 86-87)
	// Exige: nomArchivoReporte, codTipoArchivoReporte (o null), perTributario, codProceso, numTicket, codLibro=080000
	bytesRCE, err := sireClient.DescargarArchivoReporte("20000000001", "archivo_compras.zip", "", "202410", "1", "20240299999999", "080000")
	if err != nil {
		t.Fatalf("Error en RCE: %v", err)
	}
	if len(bytesRCE) == 0 {
		t.Errorf("Esperaba contenido binario")
	}
	if lastCodTipo != "null" {
		t.Errorf("Query param codTipoArchivoReporte esperado 'null' cuando es vacío, recibido: '%s'", lastCodTipo)
	}
	if lastCodLibro != "080000" {
		t.Errorf("RCE (Compras 5.32 v28) debe enviar codLibro='080000', recibido: '%s'", lastCodLibro)
	}
	if lastPerTributario != "202410" {
		t.Errorf("perTributario incorrecto: %s", lastPerTributario)
	}
	if lastCodProceso != "1" {
		t.Errorf("codProceso incorrecto: %s", lastCodProceso)
	}
	if lastNumTicket != "20240299999999" {
		t.Errorf("numTicket incorrecto: %s", lastNumTicket)
	}
	if lastAcceptHeader != "application/json" {
		t.Errorf("Header Accept esperado 'application/json', recibido: '%s'", lastAcceptHeader)
	}
	if lastContentType != "application/json" {
		t.Errorf("Header Content-Type esperado 'application/json', recibido: '%s'", lastContentType)
	}
}

func TestExportarPropuestaVentas_ManualV31(t *testing.T) {
	var lastCodOrigenEnvio string
	var lastCodTipoArchivo string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastCodOrigenEnvio = r.URL.Query().Get("codOrigenEnvio")
		lastCodTipoArchivo = r.URL.Query().Get("codTipoArchivo")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"numTicket": "20240211113333",
		})
	}))
	defer server.Close()

	sunatClient := sunat.NewClient(&mockTokenProvider{})
	sunatClient.BaseURL = server.URL
	sireClient := NewSunatSireClient(sunatClient)

	// Caso 1: Ventas 5.18 NO debe enviar codOrigenEnvio
	_, err := sireClient.ExportarPropuestaVentas("20000000001", "202410", FiltrosExportacion{
		CodTipoArchivo: "0",
	})
	if err != nil {
		t.Fatalf("Error al exportar ventas: %v", err)
	}
	if lastCodOrigenEnvio != "" {
		t.Errorf("Ventas v31 Secc. 5.18 NO incluye codOrigenEnvio, pero se envió: '%s'", lastCodOrigenEnvio)
	}
	if lastCodTipoArchivo != "0" {
		t.Errorf("codTipoArchivo esperado '0' (txt), recibido: '%s'", lastCodTipoArchivo)
	}

	// Caso 2: Ventas 5.18 solo admite 0 (txt) y 1 (xls)
	_, err = sireClient.ExportarPropuestaVentas("20000000001", "202410", FiltrosExportacion{
		CodTipoArchivo: "1",
	})
	if err != nil {
		t.Fatalf("Error al exportar ventas: %v", err)
	}
	if lastCodTipoArchivo != "1" {
		t.Errorf("codTipoArchivo esperado '1' (xls), recibido: '%s'", lastCodTipoArchivo)
	}

	// Caso 3: Si se pasa un valor no soportado (ej. "2"), debe normalizarse a "0"
	_, err = sireClient.ExportarPropuestaVentas("20000000001", "202410", FiltrosExportacion{
		CodTipoArchivo: "2",
	})
	if err != nil {
		t.Fatalf("Error al exportar ventas: %v", err)
	}
	if lastCodTipoArchivo != "0" {
		t.Errorf("codTipoArchivo no permitido debe defaulting a '0', recibido: '%s'", lastCodTipoArchivo)
	}
}

func TestExportarPropuestaCompras_ManualV28(t *testing.T) {
	var lastCodOrigenEnvio string
	var lastCodTipoArchivo string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastCodOrigenEnvio = r.URL.Query().Get("codOrigenEnvio")
		lastCodTipoArchivo = r.URL.Query().Get("codTipoArchivo")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"numTicket": "20240211112222",
		})
	}))
	defer server.Close()

	sunatClient := sunat.NewClient(&mockTokenProvider{})
	sunatClient.BaseURL = server.URL
	sireClient := NewSunatSireClient(sunatClient)

	// Caso 1: Compras 5.34 (Pág. 89) exige codOrigenEnvio=2 siempre
	_, err := sireClient.ExportarPropuestaCompras("20000000001", "202410", FiltrosExportacion{
		CodTipoArchivo: "0",
		CodOrigenEnvio: "",
	})
	if err != nil {
		t.Fatalf("Error al exportar compras: %v", err)
	}
	if lastCodOrigenEnvio != "2" {
		t.Errorf("Compras debe enviar codOrigenEnvio='2' por defecto, recibido: '%s'", lastCodOrigenEnvio)
	}
	if lastCodTipoArchivo != "0" {
		t.Errorf("codTipoArchivo esperado '0' (txt), recibido: '%s'", lastCodTipoArchivo)
	}

	// Caso 2: Compras 5.34 admite 1 (csv)
	_, err = sireClient.ExportarPropuestaCompras("20000000001", "202410", FiltrosExportacion{
		CodTipoArchivo: "1",
		CodOrigenEnvio: "1", // Intento de pasar 1 debe ser forzado a 2
	})
	if err != nil {
		t.Fatalf("Error al exportar compras: %v", err)
	}
	if lastCodOrigenEnvio != "2" {
		t.Errorf("Compras debe forzar codOrigenEnvio='2' siempre, recibido: '%s'", lastCodOrigenEnvio)
	}
	if lastCodTipoArchivo != "1" {
		t.Errorf("codTipoArchivo esperado '1' (csv), recibido: '%s'", lastCodTipoArchivo)
	}
}

func TestConsultarEstadoTicket_ManualV31_y_V28(t *testing.T) {
	var queryParams map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryParams = make(map[string]string)
		for k, v := range r.URL.Query() {
			if len(v) > 0 {
				queryParams[k] = v[0]
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"registros": []map[string]interface{}{
				{
					"numTicket":        "20240299990001",
					"codEstadoProceso": "06",
					"desEstadoProceso": "Terminado",
					"perTributario":    "202410",
					"codProceso":       "1",
				},
			},
		})
	}))
	defer server.Close()

	sunatClient := sunat.NewClient(&mockTokenProvider{})
	sunatClient.BaseURL = server.URL
	sireClient := NewSunatSireClient(sunatClient)

	// Consulta de ticket para RVIE (140000)
	status, err := sireClient.ConsultarEstadoTicket("20000000001", "202410", "202410", "20240299990001", "140000")
	if err != nil {
		t.Fatalf("Error al consultar ticket: %v", err)
	}
	if status.NumTicket != "20240299990001" {
		t.Errorf("numTicket incorrecto: %s", status.NumTicket)
	}
	if queryParams["codOrigenEnvio"] != "2" {
		t.Errorf("Se esperaba codOrigenEnvio='2', recibido: '%s'", queryParams["codOrigenEnvio"])
	}
	if queryParams["codLibro"] != "140000" {
		t.Errorf("Se esperaba codLibro='140000', recibido: '%s'", queryParams["codLibro"])
	}
	if queryParams["perIni"] != "202410" || queryParams["perFin"] != "202410" {
		t.Errorf("perIni/perFin incorrectos")
	}
	if queryParams["page"] != "1" || queryParams["perPage"] != "20" {
		t.Errorf("Paginación por defecto incorrecta")
	}

	// Consulta de ticket para RCE (080000)
	_, err = sireClient.ConsultarEstadoTicket("20000000001", "202410", "202410", "20240299990001", "080000")
	if err != nil {
		t.Fatalf("Error al consultar ticket RCE: %v", err)
	}
	if queryParams["codLibro"] != "080000" {
		t.Errorf("Se esperaba codLibro='080000' para compras, recibido: '%s'", queryParams["codLibro"])
	}
}

func TestWorker_FileExtensionPreservation(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer db.Close()
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "descargas")

	// Simular archivo plano de texto devuelto directamente por SUNAT sin zip
	txtContent := []byte("20000000001|01|F001|00000001|2024-10-15|100.00|18.00|118.00\r\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "consultaestadotickets") {
			resp := map[string]interface{}{
				"registros": []map[string]interface{}{
					{
						"numTicket":        "20240299990002",
						"codEstadoProceso": "06",
						"desEstadoProceso": "Terminado",
						"perTributario":    "202410",
						"codProceso":       "1",
						"archivoReporte": []map[string]string{
							{
								"nomArchivoReporte":     "LE20000000001-202410-140000.xls",
								"codTipoArchivoReporte": "01",
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if strings.Contains(r.URL.Path, "archivoreporte") {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(txtContent)
			return
		}
	}))
	defer server.Close()

	mockToken := &mockTokenProvider{}
	sunatClient := sunat.NewClient(mockToken)
	sunatClient.SetBaseURL(server.URL)

	sireClient := NewSunatSireClient(sunatClient)
	repo := NewRepository(db)

	ticketXls := &DescargaSire{
		EmpresaRUC: "20000000001",
		CodLibro:   "140000",
		Libro:      "VENTAS",
		Periodo:    "202410",
		Ticket:     "20240299990002",
		Estado:     string(EstadoEnProceso),
	}
	if err := repo.Create(ticketXls); err != nil {
		t.Fatalf("Error al crear ticket: %v", err)
	}

	worker := NewWorker(repo, sireClient, mockToken, storageDir)
	worker.processJob(ticketXls.ID)

	updated, err := repo.GetByID(ticketXls.ID)
	if err != nil {
		t.Fatalf("Error al obtener ticket: %v", err)
	}

	if updated.Estado != string(EstadoCompletado) {
		t.Fatalf("Ticket debió completarse, estado: %s, mensaje: %s", updated.Estado, updated.MensajeSUNAT)
	}

	// Verificar que el nombre conservó la extensión .xls entregada por SUNAT y no se forzó .xlsx
	if !strings.HasSuffix(updated.NombreArchivo, ".xls") {
		t.Fatalf("Se esperaba extensión .xls respetando el nombre de SUNAT, obtenido: %s", updated.NombreArchivo)
	}
	if strings.HasSuffix(updated.NombreArchivo, ".xlsx") {
		t.Fatalf("No debe forzarse la extensión .xlsx cuando el archivo es .xls")
	}
}

func TestWorker_MissingCodProcesoFailsWithError(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer db.Close()
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "descargas")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "consultaestadotickets") {
			// SUNAT responde 06 Terminado pero con codProceso vacío
			resp := map[string]interface{}{
				"registros": []map[string]interface{}{
					{
						"numTicket":        "20240299990003",
						"codEstadoProceso": "06",
						"desEstadoProceso": "Terminado",
						"perTributario":    "202410",
						"codProceso":       "", // VACÍO
						"archivoReporte": []map[string]string{
							{
								"nomArchivoReporte":     "LE20000000001-202410-140000.zip",
								"codTipoArchivoReporte": "00",
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer server.Close()

	mockToken := &mockTokenProvider{}
	sunatClient := sunat.NewClient(mockToken)
	sunatClient.SetBaseURL(server.URL)

	sireClient := NewSunatSireClient(sunatClient)
	repo := NewRepository(db)

	ticketSinProceso := &DescargaSire{
		EmpresaRUC: "20000000001",
		CodLibro:   "140000",
		Libro:      "VENTAS",
		Periodo:    "202410",
		Ticket:     "20240299990003",
		Estado:     string(EstadoEnProceso),
	}
	if err := repo.Create(ticketSinProceso); err != nil {
		t.Fatalf("Error al crear ticket: %v", err)
	}

	worker := NewWorker(repo, sireClient, mockToken, storageDir)
	worker.processJob(ticketSinProceso.ID)

	updated, err := repo.GetByID(ticketSinProceso.ID)
	if err != nil {
		t.Fatalf("Error al obtener ticket: %v", err)
	}

	if updated.Estado != string(EstadoError) {
		t.Fatalf("El ticket debía quedar en ERROR al faltar codProceso, obtenido: %s", updated.Estado)
	}
	if !strings.Contains(updated.MensajeSUNAT, "codProceso") {
		t.Fatalf("Mensaje debe señalar que falta codProceso: %s", updated.MensajeSUNAT)
	}
}

func TestSireWorker_Estado05EnProceso_TypoCodTipoAchivo_SpannedZip_y_RCEBusqueda(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer db.Close()
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "descargas")

	// Simular ZIP con prefijo PK\x07\x08PK\x03\x04 como entrega SUNAT en producción
	spannedZip := append([]byte("PK\x07\x08"), createDummyZip()...)

	var mu sync.Mutex
	pollCount := 0
	var capturedCodTipo string
	var capturedRcePath string
	var capturedCodTipoOpe string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "consultaestadotickets") {
			mu.Lock()
			pollCount++
			c := pollCount
			mu.Unlock()

			if c == 1 {
				// Respuesta real de SUNAT producción justo después de exportar: código 05 "En proceso"
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"registros": []map[string]interface{}{
						{
							"numTicket":        "20260300000080",
							"codEstadoProceso": "05",
							"desEstadoProceso": "En proceso",
							"perTributario":    "202610",
							"codProceso":       "10",
						},
					},
				})
				return
			}

			// Segundo intento: SUNAT responde 06 con su typo real "codTipoAchivoReporte"
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"registros": []map[string]interface{}{
					{
						"numTicket":        "20260300000080",
						"codEstadoProceso": "06",
						"desEstadoProceso": "Terminado",
						"perTributario":    "202610",
						"codProceso":       "10",
						"archivoReporte": []map[string]string{
							{
								"codTipoAchivoReporte": "00",
								"nomArchivoReporte":    "20000000001-20261119-120142-propuesta.zip",
								"nomArchivoContenido":  "20000000001-20261119-1201-propuesta.txt",
							},
						},
					},
				},
			})
			return
		}

		if strings.Contains(r.URL.Path, "archivoreporte") {
			capturedCodTipo = r.URL.Query().Get("codTipoArchivoReporte")
			w.Header().Set("Content-Type", "application/zip")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(spannedZip)
			return
		}

		if strings.Contains(r.URL.Path, "/rce/propuesta/web/propuesta/") {
			capturedRcePath = r.URL.Path
			capturedCodTipoOpe = r.URL.Query().Get("codTipoOpe")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"paginacion": map[string]interface{}{"page": 1, "perPage": 20, "totalRegistros": 1},
				"registros":  []map[string]interface{}{{"codTipoCDP": "01", "numSerieCDP": "E001", "numCDP": "482"}},
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mockToken := &mockTokenProvider{}
	sunatClient := sunat.NewClient(mockToken)
	sunatClient.SetBaseURL(server.URL)

	sireClient := NewSunatSireClient(sunatClient)
	repo := NewRepository(db)
	worker := NewWorker(repo, sireClient, mockToken, storageDir)
	service := NewService(repo, sireClient, worker, storageDir)

	worker.Start()
	defer worker.Stop()

	// Crear ticket fallido y probar ReintentarDescarga + transición 05 -> 06
	tErr := &DescargaSire{
		EmpresaRUC:   "20000000001",
		CodLibro:     "080000",
		Libro:        "COMPRAS",
		Periodo:      "202610",
		Ticket:       "20260300000080",
		Estado:       string(EstadoError),
		MensajeSUNAT: "Error previo simulado",
	}
	if err := repo.Create(tErr); err != nil {
		t.Fatalf("Error creando ticket: %v", err)
	}

	if _, err := service.ReintentarDescarga(tErr.ID); err != nil {
		t.Fatalf("Error en ReintentarDescarga: %v", err)
	}

	var done *DescargaSire
	for i := 0; i < 30; i++ {
		time.Sleep(200 * time.Millisecond)
		d, err := repo.GetByID(tErr.ID)
		if err == nil && d.Estado == string(EstadoCompletado) {
			done = d
			break
		}
	}
	if done == nil {
		d, _ := repo.GetByID(tErr.ID)
		t.Fatalf("Ticket con estado 05 -> 06 no se completó. Estado=%v Msg=%v", d.Estado, d.MensajeSUNAT)
	}
	if capturedCodTipo != "00" {
		t.Fatalf("Se esperaba codTipoArchivoReporte='00' leído desde codTipoAchivoReporte, obtenido: '%s'", capturedCodTipo)
	}
	if !strings.HasSuffix(done.NombreArchivo, ".zip") {
		t.Fatalf("Se esperaba extensión .zip, obtenido: %s", done.NombreArchivo)
	}

	// Verificar que ConsultarDetallePropuesta para COMPRAS usa /busqueda?codTipoOpe=1
	if _, err := service.ConsultarDetallePropuesta("20000000001", "COMPRAS", "202610", FiltrosPropuestaDetalle{Page: "1", PerPage: "20"}); err != nil {
		t.Fatalf("Error consultando detalle RCE Compras: %v", err)
	}
	if !strings.HasSuffix(capturedRcePath, "/202610/busqueda") || capturedCodTipoOpe != "1" {
		t.Fatalf("RCE Compras debe invocar /busqueda con codTipoOpe=1, recibido path=%s codTipoOpe=%s", capturedRcePath, capturedCodTipoOpe)
	}
}



