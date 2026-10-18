package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB(dbPath string) (*sql.DB, error) {
	// Asegurar que el directorio contenedor exista
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio para BD en '%s': %w", dir, err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir base de datos SQLite en '%s': %w", dbPath, err)
	}

	// Configurar pragmas obligatorios para rendimiento e integridad referencial
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("error al ejecutar pragma '%s': %w", p, err)
		}
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("error al ejecutar migraciones: %w", err)
	}

	DB = db
	log.Printf("[DATABASE] Base de datos SQLite inicializada exitosamente en: %s", dbPath)
	return DB, nil
}

func runMigrations(db *sql.DB) error {
	ddl := `
	-- 1. Tabla de Empresas con soft delete y secrets cifrados
	CREATE TABLE IF NOT EXISTS empresas (
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

	-- Índice único en RUC sólo para registros activos (soft delete)
	CREATE UNIQUE INDEX IF NOT EXISTS idx_empresas_ruc_active ON empresas(ruc) WHERE deleted_at IS NULL;

	-- 2. Historial de Descargas y Tickets SIRE
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

	-- Índice para búsquedas rápidas en descargas
	CREATE INDEX IF NOT EXISTS idx_descargas_ruc_periodo ON descargas_sire(empresa_ruc, periodo, cod_libro);

	-- 3. Repositorio de Comprobantes Electrónicos
	CREATE TABLE IF NOT EXISTS comprobantes_cpe (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		empresa_ruc TEXT NOT NULL,
		origen TEXT NOT NULL,
		tipo_cp TEXT NOT NULL,
		serie TEXT NOT NULL,
		numero TEXT NOT NULL,
		fecha_emision TEXT,
		monto_total REAL DEFAULT 0.0,
		moneda TEXT DEFAULT 'PEN',
		estado_sunat TEXT DEFAULT 'DESCONOCIDO',
		ruta_xml TEXT DEFAULT '',
		ruta_cdr TEXT DEFAULT '',
		ruta_pdf TEXT DEFAULT '',
		hash_archivo TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(empresa_ruc, tipo_cp, serie, numero)
	);

	CREATE INDEX IF NOT EXISTS idx_cpe_ruc_fecha ON comprobantes_cpe(empresa_ruc, fecha_emision);

	-- 4. Caché persistente de tokens OAuth2 cifrado con AES-GCM (cifrado en reposo)
	CREATE TABLE IF NOT EXISTS tokens_cache (
		ruc TEXT PRIMARY KEY,
		token_enc TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(ddl); err != nil {
		return err
	}

	// Migraciones idempotentes para columnas nuevas en tablas existentes
	_, _ = db.Exec("ALTER TABLE descargas_sire ADD COLUMN hash_archivo TEXT DEFAULT '';")
	_, _ = db.Exec("ALTER TABLE descargas_sire ADD COLUMN reintentos INTEGER DEFAULT 0;")
	_, _ = db.Exec("ALTER TABLE empresas ADD COLUMN estado_credenciales TEXT DEFAULT 'PENDIENTE';")
	_, _ = db.Exec("ALTER TABLE empresas ADD COLUMN bloqueado_hasta DATETIME NULL;")
	_, _ = db.Exec("ALTER TABLE empresas ADD COLUMN mensaje_auth TEXT DEFAULT '';")

	// Purgar cualquier tabla heredada que contenga la columna en texto plano 'access_token'
	var legacyCol string
	_ = db.QueryRow("SELECT name FROM pragma_table_info('tokens_cache') WHERE name = 'access_token'").Scan(&legacyCol)
	if legacyCol == "access_token" {
		_, _ = db.Exec("DROP TABLE IF EXISTS tokens_cache;")
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS tokens_cache (
			ruc TEXT PRIMARY KEY,
			token_enc TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`)
		_, _ = db.Exec("VACUUM;")
	}

	return nil
}
