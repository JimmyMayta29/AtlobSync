package sire

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SwodLore/sire-api/internal/database"
)

func TestColIndexToExcelName(t *testing.T) {
	cases := []struct {
		idx  int
		want string
	}{
		{0, "A"},
		{25, "Z"},
		{26, "AA"},
		{39, "AN"},
		{40, "AO"},
		{79, "CB"},
	}
	for _, tc := range cases {
		if got := colIndexToExcelName(tc.idx); got != tc.want {
			t.Errorf("colIndexToExcelName(%d) = %q, esperado %q", tc.idx, got, tc.want)
		}
	}
}

func makeSampleRVIEProposalTXT() []byte {
	header := strings.Join(rvieExpectedHeaders, "|")
	row1 := make([]string, len(rvieExpectedHeaders))
	row1[0] = "20612628034"
	row1[1] = "EMPRESA DEMO S.A.C."
	row1[2] = "202610"
	row1[3] = "2061262803401F0010000000123"
	row1[4] = "15/10/2026"
	row1[6] = "01"
	row1[7] = "F001"
	row1[8] = "0000123"
	row1[10] = "6"
	row1[11] = "20100047218"
	row1[12] = "CLIENTE UNO & CIA <PERU>"
	row1[14] = "1000.00"
	row1[16] = "180.00"
	row1[25] = "1180.00"
	row1[26] = "PEN"
	row1[27] = "1.000"
	row1[34] = "1"

	row2 := make([]string, len(rvieExpectedHeaders))
	row2[0] = "20612628034"
	row2[1] = "EMPRESA DEMO S.A.C."
	row2[2] = "202610"
	row2[4] = "16/10/2026"
	row2[6] = "01"
	row2[7] = "F001"
	row2[8] = "0000124"
	row2[12] = "FACTURA ANULADA"
	row2[14] = "500.00"
	row2[16] = "90.00"
	row2[25] = "590.00"
	row2[26] = "PEN"
	row2[34] = "2"

	return []byte(header + "\n" + strings.Join(row1, "|") + "\n" + strings.Join(row2, "|") + "\n")
}

func makeSampleRCEProposalTXT() []byte {
	header := strings.Join(rceExpectedHeaders, "|")
	row := make([]string, len(rceExpectedHeaders))
	row[0] = "20612628034"
	row[1] = "EMPRESA DEMO S.A.C."
	row[2] = "202610"
	row[4] = "10/10/2026"
	row[6] = "01"
	row[7] = "E001"
	row[9] = "000045"
	row[11] = "6"
	row[12] = "20538712249"
	row[13] = "PROVEEDOR S.A."
	row[14] = "200.00"
	row[15] = "36.00"
	row[24] = "236.00"
	row[25] = "PEN"
	row[26] = "1.000"
	row[39] = "1"
	// Solo llenar CLU2 para verificar que las 38 CLU vacías se omiten y CLU2 se conserva
	row[42] = "DATO_CLU2"
	return []byte(header + "\n" + strings.Join(row, "|") + "\n")
}

func verifyValidXLSXArchive(t *testing.T, xlsxData []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(xlsxData), int64(len(xlsxData)))
	if err != nil {
		t.Fatalf("XLSX no es un archivo ZIP válido: %v", err)
	}
	files := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("error abriendo entrada %s: %v", f.Name, err)
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rc)
		_ = rc.Close()

		// Validar que cada XML interno sea bien formado
		dec := xml.NewDecoder(bytes.NewReader(buf.Bytes()))
		for {
			if _, err := dec.Token(); err != nil {
				if err.Error() == "EOF" {
					break
				}
				t.Fatalf("XML mal formado en %s: %v", f.Name, err)
			}
		}
		files[f.Name] = buf.String()
	}

	required := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"xl/workbook.xml",
		"xl/_rels/workbook.xml.rels",
		"xl/styles.xml",
		"xl/worksheets/sheet1.xml",
		"xl/worksheets/sheet2.xml",
	}
	for _, req := range required {
		if _, ok := files[req]; !ok {
			t.Fatalf("falta entrada obligatoria OOXML %s", req)
		}
	}
	return files
}

func TestBuildProposalExcelXLSX_RVIEAndRCE(t *testing.T) {
	t.Run("RVIE Ventas genera XLSX válido con texto preservado, montos numéricos y filtros", func(t *testing.T) {
		rawTXT := makeSampleRVIEProposalTXT()
		meta := &DescargaSire{
			EmpresaRUC: "20612628034",
			Libro:      "VENTAS",
			CodLibro:   "140000",
			Periodo:    "202610",
			Ticket:     "20260300000099",
		}
		xlsxBytes, err := BuildProposalExcelXLSX(rawTXT, "LE20612628034202610.txt", meta)
		if err != nil {
			t.Fatalf("BuildProposalExcelXLSX falló: %v", err)
		}

		files := verifyValidXLSXArchive(t, xlsxBytes)
		sheet1 := files["xl/worksheets/sheet1.xml"]
		sheet2 := files["xl/worksheets/sheet2.xml"]

		// Verificar congelado de cabecera y autofiltro
		if !strings.Contains(sheet1, `state="frozen"`) || !strings.Contains(sheet1, `<autoFilter ref="A1:AO3"/>`) {
			t.Fatalf("sheet1 debe tener fila 1 congelada y autoFilter A1:AO3")
		}
		// Verificar preservación de ceros a la izquierda (Tipo 01, Número 0000123) y escape XML (&, <, >)
		if !strings.Contains(sheet1, `t="inlineStr"><is><t xml:space="preserve">0000123</t>`) {
			t.Fatalf("el número 0000123 debe preservar ceros como inlineStr")
		}
		if !strings.Contains(sheet1, `CLIENTE UNO &amp; CIA &lt;PERU&gt;`) {
			t.Fatalf("caracteres especiales XML no escapados correctamente")
		}
		// Verificar monto Total CP como numérico t="n"
		if !strings.Contains(sheet1, `t="n"><v>1180</v>`) {
			t.Fatalf("Total CP 1180.00 debe escribirse como celda numérica")
		}
		// Verificar columna Estado Legible con Activo y Anulado
		if !strings.Contains(sheet1, `>Activo<`) || !strings.Contains(sheet1, `>Anulado<`) {
			t.Fatalf("sheet1 debe incluir Estado Legible Activo y Anulado")
		}
		// Verificar hoja 2 Resumen KPIs (total neto excluye anulado 590 -> 1180)
		if !strings.Contains(sheet2, `TOTAL NETO PROPUESTA`) || !strings.Contains(sheet2, `<v>1180</v>`) {
			t.Fatalf("sheet2 debe incluir TOTAL NETO PROPUESTA = 1180")
		}
	})

	t.Run("RCE Compras omite CLUs vacíos pero conserva CLU con datos", func(t *testing.T) {
		rawTXT := makeSampleRCEProposalTXT()
		meta := &DescargaSire{
			EmpresaRUC: "20612628034",
			Libro:      "COMPRAS",
			CodLibro:   "080000",
			Periodo:    "202610",
		}
		xlsxBytes, err := BuildProposalExcelXLSX(rawTXT, "RCE202610.txt", meta)
		if err != nil {
			t.Fatalf("BuildProposalExcelXLSX RCE falló: %v", err)
		}
		files := verifyValidXLSXArchive(t, xlsxBytes)
		sheet1 := files["xl/worksheets/sheet1.xml"]

		if !strings.Contains(sheet1, `>CLU2<`) || !strings.Contains(sheet1, `>DATO_CLU2<`) {
			t.Fatalf("RCE debe conservar CLU2 con datos")
		}
		if strings.Contains(sheet1, `>CLU39<`) {
			t.Fatalf("RCE no debe inflar el Excel con columnas CLU totalmente vacías")
		}
	})
}

func TestService_ExportarPropuesta_XLSX_TXT_ZIP(t *testing.T) {
	storageDir := t.TempDir()
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Crear ZIP real en storageDir
	relPath := "20612628034/VENTAS/202610/LE20612628034202610.zip"
	absZIP := filepath.Join(storageDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(absZIP), 0755); err != nil {
		t.Fatal(err)
	}
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	w, _ := zw.Create("LE20612628034202610.txt")
	rawTXT := makeSampleRVIEProposalTXT()
	_, _ = w.Write(rawTXT)
	_ = zw.Close()
	if err := os.WriteFile(absZIP, zbuf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)
	d := &DescargaSire{
		EmpresaRUC:    "20612628034",
		CodLibro:      "140000",
		Libro:         "VENTAS",
		Periodo:       "202610",
		Ticket:        "20260300000099",
		Estado:        string(EstadoCompletado),
		NombreArchivo: "LE20612628034202610.zip",
		RutaLocal:     relPath,
	}
	if err := repo.Create(d); err != nil {
		t.Fatal(err)
	}

	svc := NewService(repo, nil, nil, storageDir)

	// 1. XLSX
	expXLSX, err := svc.ExportarPropuesta(d.ID, "xlsx")
	if err != nil {
		t.Fatalf("ExportarPropuesta xlsx falló: %v", err)
	}
	if !strings.HasSuffix(expXLSX.FileName, ".xlsx") || !strings.Contains(expXLSX.ContentType, "spreadsheetml") {
		t.Fatalf("metadatos xlsx inesperados: %s | %s", expXLSX.FileName, expXLSX.ContentType)
	}
	verifyValidXLSXArchive(t, expXLSX.Data)

	// 2. TXT
	expTXT, err := svc.ExportarPropuesta(d.ID, "txt")
	if err != nil {
		t.Fatalf("ExportarPropuesta txt falló: %v", err)
	}
	if expTXT.FileName != "LE20612628034202610.txt" || !bytes.Equal(expTXT.Data, rawTXT) {
		t.Fatalf("exportación txt no coincide con el TXT original dentro del ZIP")
	}

	// 3. ZIP
	expZIP, err := svc.ExportarPropuesta(d.ID, "zip")
	if err != nil {
		t.Fatalf("ExportarPropuesta zip falló: %v", err)
	}
	if expZIP.FileName != "LE20612628034202610.zip" || !bytes.Equal(expZIP.Data, zbuf.Bytes()) {
		t.Fatalf("exportación zip no coincide con el ZIP original")
	}

	// 4. Formato inválido
	if _, err := svc.ExportarPropuesta(d.ID, "pdf"); err == nil {
		t.Fatalf("formato no soportado debió devolver error")
	}
}
