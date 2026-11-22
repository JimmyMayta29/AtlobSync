package migration

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/SwodLore/sire-api/internal/database"
)

func TestNormalizeStoragePath(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		changed bool
	}{
		{
			in:      `C:\PROYECTOS SUNAT\SIRE - SUNAT\API-SIRE\descargas\20612628034\VENTAS\202610\LE20612628034.zip`,
			want:    "20612628034/VENTAS/202610/LE20612628034.zip",
			changed: true,
		},
		{
			in:      `C:\PROYECTOS SUNAT\Conexion Sunat\API-SIRE\descargas\20612628034\cpe\ventas\F001-1.xml`,
			want:    "20612628034/cpe/ventas/F001-1.xml",
			changed: true,
		},
		{
			in:      "20612628034/VENTAS/202610/LE20612628034.zip",
			want:    "20612628034/VENTAS/202610/LE20612628034.zip",
			changed: false,
		},
		{
			in:      "",
			want:    "",
			changed: false,
		},
	}

	for _, tc := range cases {
		got, ch := NormalizeStoragePath(tc.in)
		if got != tc.want || ch != tc.changed {
			t.Errorf("NormalizeStoragePath(%q) = (%q, %v), esperado (%q, %v)", tc.in, got, ch, tc.want, tc.changed)
		}
	}
}

func TestMigrateLegacyData_EndToEnd(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, "API-SIRE")
	dstDir := filepath.Join(root, "InstalledData")

	// Crear BD origen en modo WAL con datos de prueba y rutas heredadas absolutas
	srcDBPath := filepath.Join(legacyDir, "data", "sire.db")
	db, err := database.InitDB(srcDBPath)
	if err != nil {
		t.Fatalf("error iniciando DB origen: %v", err)
	}

	_, err = db.Exec(`INSERT INTO empresas (ruc, razon_social, usuario_sol, clave_sol_enc, client_id, client_secret_enc, estado_credenciales)
		VALUES ('20612628034', 'EMPRESA DEMO S.A.C.', 'MODDATOS', 'enc1', 'cid1', 'enc2', 'VALIDO')`)
	if err != nil {
		t.Fatalf("error insertando empresa: %v", err)
	}

	_, err = db.Exec(`INSERT INTO descargas_sire (empresa_ruc, cod_libro, libro, periodo, ticket, estado, nombre_archivo, ruta_local)
		VALUES ('20612628034', '140000', 'VENTAS', '202610', 'T001', 'COMPLETADO', 'prop.zip', 'C:\PROYECTOS SUNAT\SIRE - SUNAT\API-SIRE\descargas\20612628034\VENTAS\202610\prop.zip')`)
	if err != nil {
		t.Fatalf("error insertando descarga: %v", err)
	}

	_, err = db.Exec(`INSERT INTO comprobantes_cpe (empresa_ruc, origen, periodo, tipo_cp, serie, numero, ruta_xml, ruta_pdf, ruta_cdr)
		VALUES ('20612628034', 'VENTAS', '202610', '01', 'F001', '1',
		'C:\PROYECTOS SUNAT\Conexion Sunat\API-SIRE\descargas\20612628034\cpe\ventas\F001-1.xml',
		'C:\PROYECTOS SUNAT\Conexion Sunat\API-SIRE\descargas\20612628034\cpe\ventas\F001-1.pdf',
		'20612628034/cpe/ventas/R-F001-1.zip')`)
	if err != nil {
		t.Fatalf("error insertando cpe: %v", err)
	}
	_ = db.Close()

	// Crear .env con APP_MASTER_KEY
	const testKey = "clave_maestra_de_prueba_con_mas_de_treinta_y_dos_caracteres_1234"
	if err := os.WriteFile(filepath.Join(legacyDir, ".env"), []byte("APP_MASTER_KEY="+testKey+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Crear archivos en descargas/
	zipFile := filepath.Join(legacyDir, "descargas", "20612628034", "VENTAS", "202610", "prop.zip")
	if err := os.MkdirAll(filepath.Dir(zipFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zipFile, []byte("PK-demo-zip"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Detectar fuente legacy
	info := DetectLegacySource(root, dstDir)
	if !info.Found || !info.CanMigrate || !info.HasMasterKey || info.CompaniesCount != 1 || info.DownloadsFiles != 1 {
		t.Fatalf("detección inesperada: %+v", info)
	}

	// 2. Ejecutar migración
	rep, err := MigrateLegacyData(legacyDir, dstDir, nil)
	if err != nil {
		t.Fatalf("MigrateLegacyData falló: %v", err)
	}
	if !rep.MasterKeyMigrated || rep.CompaniesCount != 1 || rep.NormalizedPaths != 2 || rep.CopiedFiles != 1 {
		t.Fatalf("reporte de migración inesperado: %+v", rep)
	}

	// 3. Verificar llave maestra en destino
	keyBytes, err := os.ReadFile(filepath.Join(dstDir, "data", "app.key"))
	if err != nil || string(keyBytes) != testKey {
		t.Fatalf("app.key en destino no coincide: err=%v val=%q", err, string(keyBytes))
	}

	// 4. Verificar rutas normalizadas en BD destino
	dstDB, err := sql.Open("sqlite", filepath.Join(dstDir, "data", "sire.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dstDB.Close()

	var rutaDesc string
	if err := dstDB.QueryRow("SELECT ruta_local FROM descargas_sire WHERE ticket = 'T001'").Scan(&rutaDesc); err != nil {
		t.Fatal(err)
	}
	if rutaDesc != "20612628034/VENTAS/202610/prop.zip" {
		t.Fatalf("ruta_local no normalizada: %q", rutaDesc)
	}

	var rXML, rPDF, rCDR string
	if err := dstDB.QueryRow("SELECT ruta_xml, ruta_pdf, ruta_cdr FROM comprobantes_cpe WHERE serie = 'F001'").Scan(&rXML, &rPDF, &rCDR); err != nil {
		t.Fatal(err)
	}
	if rXML != "20612628034/cpe/ventas/F001-1.xml" || rPDF != "20612628034/cpe/ventas/F001-1.pdf" || rCDR != "20612628034/cpe/ventas/R-F001-1.zip" {
		t.Fatalf("rutas CPE no normalizadas: xml=%q pdf=%q cdr=%q", rXML, rPDF, rCDR)
	}

	// 5. Verificar archivo copiado en descargas destino
	copiedZip, err := os.ReadFile(filepath.Join(dstDir, "descargas", "20612628034", "VENTAS", "202610", "prop.zip"))
	if err != nil || string(copiedZip) != "PK-demo-zip" {
		t.Fatalf("archivo descargado no copiado correctamente: err=%v", err)
	}

	// 6. Verificar protección contra sobrescritura en segundo intento
	info2 := DetectLegacySource(root, dstDir)
	if !info2.DestAlreadyExists || info2.CanMigrate {
		t.Fatalf("segundo DetectLegacySource debió marcar DestAlreadyExists=true y CanMigrate=false: %+v", info2)
	}
	if _, err := MigrateLegacyData(legacyDir, dstDir, nil); err == nil {
		t.Fatalf("segundo MigrateLegacyData debió fallar para proteger datos existentes")
	}
}
