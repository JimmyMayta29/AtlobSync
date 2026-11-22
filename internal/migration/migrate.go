package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

// LegacySourceInfo describe los datos existentes detectados junto al instalador (p. ej. API-SIRE/).
type LegacySourceInfo struct {
	Found             bool   `json:"found"`
	CanMigrate        bool   `json:"can_migrate"`
	DestAlreadyExists bool   `json:"dest_already_exists"`
	SourceDir         string `json:"source_dir"`
	HasMasterKey      bool   `json:"has_master_key"`
	CompaniesCount    int    `json:"companies_count"`
	DownloadsFiles    int    `json:"downloads_files"`
	DBSizeBytes       int64  `json:"db_size_bytes"`
}

// MigrationReport resume el resultado de migrar los datos locales a la carpeta de la aplicación instalada.
type MigrationReport struct {
	CompaniesCount    int   `json:"companies_count"`
	NormalizedPaths   int   `json:"normalized_paths"`
	CopiedFiles       int   `json:"copied_files"`
	CopiedBytes       int64 `json:"copied_bytes"`
	MasterKeyMigrated bool  `json:"master_key_migrated"`
}

// DetectLegacySource inspecciona si junto al instalador existen datos previos de API-SIRE que convenga migrar.
func DetectLegacySource(installerDir, dstBaseDir string) *LegacySourceInfo {
	info := &LegacySourceInfo{}

	if dstDB := filepath.Join(dstBaseDir, "data", "sire.db"); fileExistsNonEmpty(dstDB) {
		info.DestAlreadyExists = true
	}

	candidates := []string{
		filepath.Join(installerDir, "API-SIRE"),
		installerDir,
	}

	for _, cand := range candidates {
		if strings.TrimSpace(cand) == "" {
			continue
		}
		absCand, err := filepath.Abs(cand)
		if err != nil {
			continue
		}
		absDst, _ := filepath.Abs(dstBaseDir)
		if strings.EqualFold(filepath.Clean(absCand), filepath.Clean(absDst)) {
			continue
		}

		srcDB := filepath.Join(absCand, "data", "sire.db")
		fi, err := os.Stat(srcDB)
		if err != nil || fi.IsDir() || fi.Size() == 0 {
			continue
		}

		info.Found = true
		info.SourceDir = absCand
		info.DBSizeBytes = fi.Size()

		if key, keyErr := ExtractMasterKey(absCand); keyErr == nil && len(key) >= 32 {
			info.HasMasterKey = true
		}

		info.CompaniesCount = countActiveCompaniesReadOnly(srcDB)
		info.DownloadsFiles = countFilesInDir(filepath.Join(absCand, "descargas"))
		info.CanMigrate = !info.DestAlreadyExists && info.HasMasterKey
		return info
	}

	return info
}

// ExtractMasterKey obtiene la llave maestra desde data/app.key o .env (APP_MASTER_KEY).
func ExtractMasterKey(legacyDir string) (string, error) {
	keyFile := filepath.Join(legacyDir, "data", "app.key")
	if b, err := os.ReadFile(keyFile); err == nil {
		k := strings.TrimSpace(string(b))
		if len(k) >= 32 {
			return k, nil
		}
	}

	envFile := filepath.Join(legacyDir, ".env")
	if envMap, err := godotenv.Read(envFile); err == nil {
		k := strings.TrimSpace(envMap["APP_MASTER_KEY"])
		if len(k) >= 32 {
			return k, nil
		}
	}

	return "", fmt.Errorf("no se encontró llave maestra válida (data/app.key o APP_MASTER_KEY en .env) en '%s'", legacyDir)
}

// NormalizeStoragePath convierte rutas absolutas heredadas (...\descargas\<rel>) en rutas relativas portables (<rel>).
func NormalizeStoragePath(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}

	slashed := strings.ReplaceAll(trimmed, `\`, `/`)
	lower := strings.ToLower(slashed)

	const marker = "/descargas/"
	if idx := strings.LastIndex(lower, marker); idx >= 0 {
		rel := strings.TrimLeft(slashed[idx+len(marker):], "/")
		rel = path.Clean(rel)
		if rel != "" && rel != "." && rel != ".." && !strings.HasPrefix(rel, "../") && !strings.HasPrefix(rel, "/") {
			return rel, rel != trimmed
		}
	}

	return trimmed, false
}

// MigrateLegacyData copia de forma segura la BD SQLite (consolidando WAL), la llave maestra y las descargas
// hacia dstBaseDir sin modificar ni borrar los archivos originales en srcDir.
func MigrateLegacyData(srcDir, dstBaseDir string, onProgress func(msg string)) (*MigrationReport, error) {
	report := &MigrationReport{}

	notify := func(msg string) {
		if onProgress != nil {
			onProgress(msg)
		}
	}

	srcDB := filepath.Join(srcDir, "data", "sire.db")
	if !fileExistsNonEmpty(srcDB) {
		return nil, fmt.Errorf("no se encontró base de datos origen en '%s'", srcDB)
	}

	dstDataDir := filepath.Join(dstBaseDir, "data")
	dstLogsDir := filepath.Join(dstDataDir, "logs")
	dstStorageDir := filepath.Join(dstBaseDir, "descargas")
	dstDB := filepath.Join(dstDataDir, "sire.db")

	if fileExistsNonEmpty(dstDB) {
		return nil, errors.New("ya existe una base de datos en el destino; se omitió la migración para evitar sobrescribir datos")
	}

	// 1. Verificar llave maestra antes de copiar cualquier archivo
	masterKey, err := ExtractMasterKey(srcDir)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dstLogsDir, 0755); err != nil {
		return nil, fmt.Errorf("error creando carpetas de datos destino: %w", err)
	}
	if err := os.MkdirAll(dstStorageDir, 0755); err != nil {
		return nil, fmt.Errorf("error creando carpeta de descargas destino: %w", err)
	}

	// 2. Guardar llave maestra en data/app.key
	notify("Migrando llave criptográfica local...")
	dstKeyPath := filepath.Join(dstDataDir, "app.key")
	if err := os.WriteFile(dstKeyPath, []byte(masterKey), 0600); err != nil {
		return nil, fmt.Errorf("error guardando llave maestra en '%s': %w", dstKeyPath, err)
	}
	report.MasterKeyMigrated = true

	// 3. Crear snapshot consistente de sire.db (incluyendo WAL) mediante VACUUM INTO
	notify("Copiando base de datos local (empresas, propuestas y comprobantes)...")
	tmpDB := dstDB + ".migrating"
	_ = os.Remove(tmpDB)

	if err := snapshotSQLiteDB(srcDB, tmpDB); err != nil {
		_ = os.Remove(tmpDB)
		return nil, fmt.Errorf("error consolidando base de datos SQLite: %w", err)
	}

	// 4. Normalizar rutas absolutas heredadas a rutas relativas portables dentro de la copia migrada
	notify("Normalizando rutas de archivos a formato portable...")
	normCount, companiesCount, err := normalizeMigratedDB(tmpDB)
	if err != nil {
		_ = os.Remove(tmpDB)
		return nil, fmt.Errorf("error normalizando rutas en base de datos migrada: %w", err)
	}
	report.NormalizedPaths = normCount
	report.CompaniesCount = companiesCount

	if err := os.Rename(tmpDB, dstDB); err != nil {
		_ = os.Remove(tmpDB)
		return nil, fmt.Errorf("error finalizando base de datos migrada: %w", err)
	}

	// 5. Copiar carpeta de descargas (ZIPs SIRE y comprobantes CPE)
	srcDownloads := filepath.Join(srcDir, "descargas")
	if fi, err := os.Stat(srcDownloads); err == nil && fi.IsDir() {
		notify("Copiando propuestas y comprobantes descargados...")
		copiedFiles, copiedBytes, err := copyDirContents(srcDownloads, dstStorageDir)
		if err != nil {
			return report, fmt.Errorf("error copiando carpeta de descargas: %w", err)
		}
		report.CopiedFiles = copiedFiles
		report.CopiedBytes = copiedBytes
	}

	return report, nil
}

func snapshotSQLiteDB(srcDB, dstDB string) error {
	db, err := sql.Open("sqlite", srcDB)
	if err != nil {
		return err
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")

	escapedDst := strings.ReplaceAll(filepath.ToSlash(dstDB), "'", "''")
	if _, err := db.Exec(fmt.Sprintf("VACUUM INTO '%s';", escapedDst)); err != nil {
		return err
	}
	return nil
}

func normalizeMigratedDB(dbPath string) (normalized int, companies int, err error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, 0, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	_ = db.QueryRow("SELECT COUNT(*) FROM empresas WHERE deleted_at IS NULL").Scan(&companies)

	tx, err := db.Begin()
	if err != nil {
		return 0, companies, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 1. descargas_sire.ruta_local
	type descRow struct {
		id   int64
		ruta string
	}
	var descUpdates []descRow
	if rows, qErr := tx.Query("SELECT id, COALESCE(ruta_local, '') FROM descargas_sire WHERE length(COALESCE(ruta_local, '')) > 0"); qErr == nil {
		for rows.Next() {
			var r descRow
			if scanErr := rows.Scan(&r.id, &r.ruta); scanErr == nil {
				if rel, changed := NormalizeStoragePath(r.ruta); changed {
					descUpdates = append(descUpdates, descRow{id: r.id, ruta: rel})
				}
			}
		}
		_ = rows.Close()
	}
	for _, u := range descUpdates {
		if _, execErr := tx.Exec("UPDATE descargas_sire SET ruta_local = ? WHERE id = ?", u.ruta, u.id); execErr != nil {
			err = execErr
			return 0, companies, err
		}
		normalized++
	}

	// 2. comprobantes_cpe (ruta_xml, ruta_pdf, ruta_cdr)
	type cpeRow struct {
		id  int64
		xml string
		pdf string
		cdr string
	}
	var cpeUpdates []cpeRow
	if rows, qErr := tx.Query("SELECT id, COALESCE(ruta_xml, ''), COALESCE(ruta_pdf, ''), COALESCE(ruta_cdr, '') FROM comprobantes_cpe WHERE length(COALESCE(ruta_xml, '')) > 0 OR length(COALESCE(ruta_pdf, '')) > 0 OR length(COALESCE(ruta_cdr, '')) > 0"); qErr == nil {
		for rows.Next() {
			var r cpeRow
			if scanErr := rows.Scan(&r.id, &r.xml, &r.pdf, &r.cdr); scanErr == nil {
				newXML, ch1 := NormalizeStoragePath(r.xml)
				newPDF, ch2 := NormalizeStoragePath(r.pdf)
				newCDR, ch3 := NormalizeStoragePath(r.cdr)
				if ch1 || ch2 || ch3 {
					cpeUpdates = append(cpeUpdates, cpeRow{id: r.id, xml: newXML, pdf: newPDF, cdr: newCDR})
				}
			}
		}
		_ = rows.Close()
	}
	for _, u := range cpeUpdates {
		if _, execErr := tx.Exec("UPDATE comprobantes_cpe SET ruta_xml = ?, ruta_pdf = ?, ruta_cdr = ? WHERE id = ?", u.xml, u.pdf, u.cdr, u.id); execErr != nil {
			err = execErr
			return 0, companies, err
		}
		normalized++
	}

	if err = tx.Commit(); err != nil {
		return 0, companies, err
	}
	return normalized, companies, nil
}

func copyDirContents(srcRoot, dstRoot string) (int, int64, error) {
	var files int
	var totalBytes int64

	err := filepath.WalkDir(srcRoot, func(srcPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcRoot, srcPath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dstPath := filepath.Join(dstRoot, rel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		srcInfo, err := d.Info()
		if err != nil {
			return err
		}

		// Omitir si ya existe un archivo idéntico en tamaño en destino
		if dstInfo, statErr := os.Stat(dstPath); statErr == nil && !dstInfo.IsDir() && dstInfo.Size() == srcInfo.Size() {
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}

		written, err := copySingleFile(srcPath, dstPath)
		if err != nil {
			return err
		}
		_ = os.Chtimes(dstPath, srcInfo.ModTime(), srcInfo.ModTime())
		files++
		totalBytes += written
		return nil
	})
	return files, totalBytes, err
}

func copySingleFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	return io.Copy(out, in)
}

func fileExistsNonEmpty(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Size() > 0
}

func countActiveCompaniesReadOnly(dbPath string) int {
	uri := "file:" + filepath.ToSlash(dbPath) + "?mode=ro&_pragma=busy_timeout(2000)"
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return 0
	}
	defer db.Close()
	var n int
	_ = db.QueryRow("SELECT COUNT(*) FROM empresas WHERE deleted_at IS NULL").Scan(&n)
	return n
}

func countFilesInDir(root string) int {
	var count int
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}
