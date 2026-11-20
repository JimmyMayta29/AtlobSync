package sire

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SwodLore/sire-api/internal/database"
	"github.com/SwodLore/sire-api/internal/middleware"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/charmap"
)

func buildSyntheticZIPBytes(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("error creando entrada ZIP sintética %q: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("error escribiendo entrada ZIP sintética %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("error cerrando ZIP sintético: %v", err)
	}
	return buf.Bytes()
}

func makeRVIERow(tipo, serie, num, doc, razon, bi, igv, total, estado string) string {
	return makeRVIERowFull(tipo, serie, num, doc, razon, bi, "0.00", igv, "0.00", total, "PEN", "1.000", estado)
}

func makeRVIERowFull(tipo, serie, num, doc, razon, bi, dsctoBI, igv, dsctoIGV, total, moneda, tc, estado string) string {
	cols := make([]string, len(rvieExpectedHeaders))
	cols[0] = "20000000001"
	cols[1] = "EMPRESA DEMO S.A.C."
	cols[2] = "202610"
	cols[3] = "20000000001" + tipo + serie + num
	cols[4] = "10/10/2026"
	cols[5] = ""
	cols[6] = tipo
	cols[7] = serie
	cols[8] = num
	cols[10] = "6"
	cols[11] = doc
	cols[12] = razon
	cols[13] = "0.00"
	cols[14] = bi
	cols[15] = dsctoBI
	cols[16] = igv
	cols[17] = dsctoIGV
	cols[18] = "0.00"
	cols[19] = "0.00"
	cols[20] = "0.00"
	cols[21] = "0.00"
	cols[22] = "0.00"
	cols[23] = "0.00"
	cols[24] = "0.00"
	cols[25] = total
	cols[26] = moneda
	cols[27] = tc
	cols[34] = estado
	return strings.Join(cols, "|")
}

func makeRCERowCols(colCount int, tipo, serie, num, doc, razon, biDG, igvDG, noGrav, total, moneda, tc, estado string) string {
	cols := make([]string, colCount)
	cols[0] = "20000000001"
	cols[1] = "EMPRESA DEMO S.A.C."
	cols[2] = "202610"
	cols[3] = doc + tipo + serie + num
	cols[4] = "12/10/2026"
	cols[6] = tipo
	cols[7] = serie
	cols[9] = num
	cols[11] = "6"
	cols[12] = doc
	cols[13] = razon
	cols[14] = biDG
	cols[15] = igvDG
	cols[16] = "0.00"
	cols[17] = "0.00"
	cols[18] = "0.00"
	cols[19] = "0.00"
	cols[20] = noGrav
	cols[21] = "0.00"
	cols[22] = "0.00"
	cols[23] = "0.00"
	cols[24] = total
	cols[25] = moneda
	cols[26] = tc
	cols[39] = estado
	return strings.Join(cols, "|")
}

func makeRCERow(tipo, serie, num, doc, razon, biDG, igvDG, noGrav, total, estado string) string {
	return makeRCERowCols(len(rceExpectedHeaders), tipo, serie, num, doc, razon, biDG, igvDG, noGrav, total, "PEN", "1.000", estado)
}

func TestProposalTXTParser_VentasRVIE_NC_ND_Anulados_DescuentosYFormula(t *testing.T) {
	lines := []string{
		strings.Join(rvieExpectedHeaders, "|"),
		// Factura 01 activa con descuento informativo: BI=1000, DsctoBI=-10, IGV=180, DsctoIGV=-1.80, Total=1180
		makeRVIERowFull("01", "F001", "00000001", "20000000002", "CLIENTE DEMO UNO S.A.C.", "1000.00", "-10.00", "180.00", "-1.80", "1180.00", "PEN", "1.000", "1"),
		// Boleta 03 activa: +200 / +36 / +236
		makeRVIERow("03", "B001", "00000001", "10000001", "PERSONA DEMO", "200.00", "36.00", "236.00", "1"),
		// NC 07 con signo negativo (como SUNAT): -100 / -18 / -118
		makeRVIERow("07", "FC01", "00000001", "20000000002", "CLIENTE DEMO UNO S.A.C.", "-100.00", "-18.00", "-118.00", "1"),
		// NC 07 con signo positivo: debe normalizarse a RESTA (-50 / -9 / -59)
		makeRVIERow("07", "FC01", "00000002", "20000000002", "CLIENTE DEMO UNO S.A.C.", "50.00", "9.00", "59.00", "1"),
		// ND 08 activa: SUMA (+20 / +3.60 / +23.60)
		makeRVIERow("08", "FD01", "00000001", "20000000002", "CLIENTE DEMO UNO S.A.C.", "20.00", "3.60", "23.60", "1"),
		// Anulado estado 2 con importes no nulos: EXCLUIDO de totales y devuelto aparte
		makeRVIERow("01", "F001", "00000002", "20000000003", "CLIENTE ANULADO S.A.C.", "500.00", "90.00", "590.00", "2"),
	}
	rawTXT := append([]byte{0xEF, 0xBB, 0xBF}, []byte(strings.Join(lines, "\r\n")+"\r\n")...)
	zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{
		"LE200000000012026100014040001EXP2.txt": rawTXT,
	})

	parsed, err := ParseProposalZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), "VENTAS", "140000", DefaultMaxProposalTXTBytes)
	if err != nil {
		t.Fatalf("error inesperado parseando RVIE sintético: %v", err)
	}

	if parsed.Resumen.Codificacion != "UTF-8-BOM" {
		t.Fatalf("se esperaba codificación UTF-8-BOM, se obtuvo %q", parsed.Resumen.Codificacion)
	}
	if parsed.Resumen.IncluyeAnuladosEnTotales {
		t.Fatalf("los totales de Ventas deben excluir anulados (incluye_anulados_en_totales=false)")
	}
	if parsed.Resumen.FormulaTotales != "Total = total_comprobantes + total_nd - total_nc" {
		t.Fatalf("fórmula de totales inesperada: %q", parsed.Resumen.FormulaTotales)
	}

	c := parsed.Resumen.Conteos
	if c.TotalFilas != 6 || c.Activos != 5 || c.Comprobantes != 2 || c.NC != 2 || c.ND != 1 || c.Anulados != 1 || c.EstadoDesconocido != 0 {
		t.Fatalf("conteos inesperados: %+v", c)
	}
	if len(parsed.Activos) != 5 || len(parsed.Anulados) != 1 || parsed.Anulados[0].EstadoMarca != EstadoMarcaAnulado {
		t.Fatalf("filas separadas inesperadas: activos=%d anulados=%d", len(parsed.Activos), len(parsed.Anulados))
	}

	tot := parsed.Resumen.Totales
	if tot.BaseImponible != 1070.00 || tot.IGV != 192.60 || tot.DescuentoBI != -10.00 || tot.DescuentoIGV != -1.80 {
		t.Fatalf("BI/IGV/Descuentos inesperados: %+v", tot)
	}
	if tot.TotalComprobantes != 1416.00 || tot.TotalNC != 177.00 || tot.TotalND != 23.60 || tot.TotalCP != 1262.60 {
		t.Fatalf("desglose totales inesperado: %+v", tot)
	}
	// Verificar fórmula exacta: Total = total_comprobantes + total_nd - total_nc
	if math.Abs((tot.TotalComprobantes+tot.TotalND-tot.TotalNC)-tot.TotalCP) > 0.0001 {
		t.Fatalf("no se cumple fórmula Total = total_comprobantes + total_nd - total_nc: %.2f + %.2f - %.2f != %.2f",
			tot.TotalComprobantes, tot.TotalND, tot.TotalNC, tot.TotalCP)
	}
}

func TestProposalTXTParser_TiposEspeciales_87_88_97_98_YSubtotalesPorTipo(t *testing.T) {
	lines := []string{
		strings.Join(rceExpectedHeaders, "|"),
		// Factura 01: +1000
		makeRCERow("01", "F001", "000001", "20000000002", "PROVEEDOR DEMO", "847.46", "152.54", "0.00", "1000.00", "1"),
		// NC 07: resta 100
		makeRCERow("07", "FC01", "000001", "20000000002", "PROVEEDOR DEMO", "-84.75", "-15.25", "0.00", "-100.00", "1"),
		// NC Especial 87 (enviada positiva): debe restar 50
		makeRCERow("87", "E001", "000001", "20000000002", "PROVEEDOR DEMO", "42.37", "7.63", "0.00", "50.00", "1"),
		// NC No Domiciliado 97: resta 25
		makeRCERow("97", "ND01", "000001", "20000000003", "EXTERIOR DEMO", "0.00", "0.00", "-25.00", "-25.00", "1"),
		// ND 08: suma 40
		makeRCERow("08", "FD01", "000001", "20000000002", "PROVEEDOR DEMO", "33.90", "6.10", "0.00", "40.00", "1"),
		// ND Especial 88: suma 30
		makeRCERow("88", "E002", "000001", "20000000002", "PROVEEDOR DEMO", "25.42", "4.58", "0.00", "30.00", "1"),
		// ND No Domiciliado 98: suma 15
		makeRCERow("98", "ND02", "000001", "20000000003", "EXTERIOR DEMO", "0.00", "0.00", "15.00", "15.00", "1"),
	}
	zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{
		"rce_tipos_especiales.txt": []byte(strings.Join(lines, "\n") + "\n"),
	})

	parsed, err := ParseProposalZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), "COMPRAS", "080000", DefaultMaxProposalTXTBytes)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	c := parsed.Resumen.Conteos
	if c.Comprobantes != 1 || c.NC != 3 || c.ND != 3 || c.Activos != 7 || c.Anulados != 0 {
		t.Fatalf("conteos esperados (1 comp, 3 NC, 3 ND), obtenidos: %+v", c)
	}

	tot := parsed.Resumen.Totales
	// Total = 1000 + (40+30+15) - (100+50+25) = 1000 + 85 - 175 = 910.00
	if tot.TotalComprobantes != 1000.00 || tot.TotalND != 85.00 || tot.TotalNC != 175.00 || tot.TotalCP != 910.00 {
		t.Fatalf("totales inesperados: %+v", tot)
	}

	subByTipo := make(map[string]ProposalTypeAgg)
	for _, st := range parsed.Resumen.PorTipo {
		subByTipo[st.TipoCP] = st
	}
	expectedSubs := map[string]struct {
		cat   string
		total float64
	}{
		"01": {"comprobante", 1000.00},
		"07": {"nc", -100.00},
		"87": {"nc", -50.00},
		"97": {"nc", -25.00},
		"08": {"nd", 40.00},
		"88": {"nd", 30.00},
		"98": {"nd", 15.00},
	}
	for tipo, exp := range expectedSubs {
		got, ok := subByTipo[tipo]
		if !ok {
			t.Fatalf("falta subtotal por tipo %s en resumen.por_tipo", tipo)
		}
		if got.Categoria != exp.cat || got.TotalCP != exp.total {
			t.Fatalf("subtotal tipo %s inesperado: %+v (esperado cat=%s total=%.2f)", tipo, got, exp.cat, exp.total)
		}
	}
}

func TestProposalTXTParser_EstadoDesconocido_ExcluidoYConAdvertencia(t *testing.T) {
	lines := []string{
		strings.Join(rvieExpectedHeaders, "|"),
		// Solo estado "1" suma (+118)
		makeRVIERow("01", "F001", "00000001", "20000000002", "CLIENTE ACTIVO", "100.00", "18.00", "118.00", "1"),
		// Estado "2" es anulado (no suma)
		makeRVIERow("01", "F001", "00000002", "20000000002", "CLIENTE ANULADO", "200.00", "36.00", "236.00", "2"),
		// Estado "0" desconocido (no debe sumarse en silencio)
		makeRVIERow("01", "F001", "00000003", "20000000002", "CLIENTE ESTADO CERO", "300.00", "54.00", "354.00", "0"),
		// Estado "9" desconocido (no debe sumarse en silencio)
		makeRVIERow("01", "F001", "00000004", "20000000002", "CLIENTE ESTADO NUEVE", "400.00", "72.00", "472.00", "9"),
	}
	zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{
		"estados_desconocidos.txt": []byte(strings.Join(lines, "\n") + "\n"),
	})

	parsed, err := ParseProposalZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), "VENTAS", "140000", DefaultMaxProposalTXTBytes)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	c := parsed.Resumen.Conteos
	if c.TotalFilas != 4 || c.Activos != 1 || c.Anulados != 1 || c.EstadoDesconocido != 2 {
		t.Fatalf("conteos inesperados: %+v", c)
	}
	if parsed.Resumen.Totales.TotalCP != 118.00 {
		t.Fatalf("estados desconocidos no deben sumarse en totales: esperado 118.00, obtenido %.2f", parsed.Resumen.Totales.TotalCP)
	}
	if len(parsed.Resumen.Advertencias) == 0 || !strings.Contains(parsed.Resumen.Advertencias[0], "[0, 9]") {
		t.Fatalf("se esperaba advertencia explícita con códigos [0, 9], obtenido: %v", parsed.Resumen.Advertencias)
	}
	if len(parsed.Desconocidos) != 2 || parsed.Desconocidos[0].EstadoMarca != EstadoMarcaDesconocido {
		t.Fatalf("filas con estado desconocido no marcadas correctamente: %+v", parsed.Desconocidos)
	}
}

func TestProposalTXTParser_MonedaNoPEN_SinConversion(t *testing.T) {
	lines := []string{
		strings.Join(rceExpectedHeaders, "|"),
		makeRCERowCols(len(rceExpectedHeaders), "01", "F001", "000001", "20000000002", "PROVEEDOR USD", "100.00", "18.00", "0.00", "118.00", "USD", "3.750", "1"),
		makeRCERowCols(len(rceExpectedHeaders), "01", "F001", "000002", "20000000003", "PROVEEDOR PEN", "200.00", "36.00", "0.00", "236.00", "PEN", "1.000", "1"),
	}
	zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{
		"moneda_usd.txt": []byte(strings.Join(lines, "\n") + "\n"),
	})

	parsed, err := ParseProposalZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), "COMPRAS", "080000", DefaultMaxProposalTXTBytes)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if parsed.Resumen.FilasNoPEN != 1 {
		t.Fatalf("se esperaba filas_no_pen=1, obtenido %d", parsed.Resumen.FilasNoPEN)
	}
	// Verifica que NO se multiplicó 118.00 por 3.750 (sin conversión)
	if parsed.Activos[0].Moneda != "USD" || parsed.Activos[0].TipoCambio != "3.750" || parsed.Activos[0].TotalCP != 118.00 {
		t.Fatalf("la fila USD no debe convertirse y debe exponer Moneda y TipoCambio: %+v", parsed.Activos[0])
	}
}

func TestProposalTXTParser_RCE_CoreEstricto_CLUOpcionalesConAdvertencia(t *testing.T) {
	// Caso 1: RCE con solo las 41 columnas núcleo (0 CLU) -> válido con advertencia
	t.Run("rce_41_columnas_sin_clu_advertencia", func(t *testing.T) {
		lines := []string{
			strings.Join(rceBaseHeaders, "|"),
			makeRCERowCols(len(rceBaseHeaders), "01", "F001", "000001", "20000000002", "PROVEEDOR CORE", "100.00", "18.00", "0.00", "118.00", "PEN", "1.000", "1"),
		}
		zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{"rce_core.txt": []byte(strings.Join(lines, "\n") + "\n")})
		parsed, err := ParseProposalZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), "COMPRAS", "080000", DefaultMaxProposalTXTBytes)
		if err != nil {
			t.Fatalf("RCE con 41 columnas núcleo válidas debe pasar con advertencia, falló con: %v", err)
		}
		if parsed.Resumen.Columnas != 41 || parsed.Resumen.Totales.TotalCP != 118.00 {
			t.Fatalf("resumen inesperado: columnas=%d total=%.2f", parsed.Resumen.Columnas, parsed.Resumen.Totales.TotalCP)
		}
		if len(parsed.Resumen.Advertencias) == 0 {
			t.Fatalf("se esperaba advertencia por ausencia de columnas CLU1..CLU39")
		}
	})

	// Caso 2: RCE con 45 columnas (41 núcleo + CLU1..CLU4) -> válido con advertencia
	t.Run("rce_45_columnas_clu_parcial_advertencia", func(t *testing.T) {
		hdr := append(append([]string(nil), rceBaseHeaders...), "CLU1", "CLU2", "CLU3", "CLU4")
		lines := []string{
			strings.Join(hdr, "|"),
			makeRCERowCols(45, "01", "F001", "000001", "20000000002", "PROVEEDOR CLU4", "100.00", "18.00", "0.00", "118.00", "PEN", "1.000", "1"),
		}
		zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{"rce_clu4.txt": []byte(strings.Join(lines, "\n") + "\n")})
		parsed, err := ParseProposalZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), "COMPRAS", "080000", DefaultMaxProposalTXTBytes)
		if err != nil {
			t.Fatalf("RCE con 4 CLU debe pasar con advertencia, falló con: %v", err)
		}
		if len(parsed.Resumen.Advertencias) == 0 || !strings.Contains(parsed.Resumen.Advertencias[0], "4 columna(s)") {
			t.Fatalf("se esperaba advertencia indicando 4 columnas CLU, obtenido: %v", parsed.Resumen.Advertencias)
		}
	})

	// Caso 3: Falla en una columna núcleo (00..40) -> FORMATO_INESPERADO
	t.Run("rce_nucleo_alterado_formato_inesperado", func(t *testing.T) {
		hdr := append([]string(nil), rceExpectedHeaders...)
		hdr[14] = "Base Alterada"
		zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{"rce_bad_core.txt": []byte(strings.Join(hdr, "|") + "\n")})
		_, err := ParseProposalZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), "COMPRAS", "080000", DefaultMaxProposalTXTBytes)
		var pErr *ProposalParseError
		if !errors.As(err, &pErr) || pErr.Code != ErrCodeFormatoInesperado {
			t.Fatalf("se esperaba FORMATO_INESPERADO al alterar columna núcleo, se obtuvo: %v", err)
		}
	})

	// Caso 4: Menos de 41 columnas núcleo -> FORMATO_INESPERADO
	t.Run("rce_menos_de_41_columnas_formato_inesperado", func(t *testing.T) {
		hdr := rceBaseHeaders[:40]
		zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{"rce_short.txt": []byte(strings.Join(hdr, "|") + "\n")})
		_, err := ParseProposalZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), "COMPRAS", "080000", DefaultMaxProposalTXTBytes)
		var pErr *ProposalParseError
		if !errors.As(err, &pErr) || pErr.Code != ErrCodeFormatoInesperado {
			t.Fatalf("se esperaba FORMATO_INESPERADO con 40 columnas en RCE, se obtuvo: %v", err)
		}
	})
}

func TestProposalTXTParser_ComprasRCE_NC_ND_Anulados_Windows1252(t *testing.T) {
	// Simula doble espacio real de SUNAT en cabecera col 13 ("Razón  Social") y codificación Windows-1252.
	hdr := append([]string(nil), rceExpectedHeaders...)
	hdr[13] = "Apellidos Nombres/ Razón  Social"

	lines := []string{
		strings.Join(hdr, "|"),
		makeRCERow("01", "F001", "000010", "20000000004", "PROVEEDOR PEÑA S.A.C.", "500.00", "90.00", "100.00", "690.00", "1"),
		makeRCERow("07", "FC01", "000001", "20000000004", "PROVEEDOR PEÑA S.A.C.", "-100.00", "-18.00", "0.00", "-118.00", "1"),
		makeRCERow("08", "FD01", "000001", "20000000004", "PROVEEDOR PEÑA S.A.C.", "50.00", "9.00", "0.00", "59.00", "1"),
		makeRCERow("01", "F001", "000011", "20000000005", "PROVEEDOR ANULADO S.A.C.", "300.00", "54.00", "0.00", "354.00", "2"),
	}
	win1252Bytes, err := charmap.Windows1252.NewEncoder().Bytes([]byte(strings.Join(lines, "\n") + "\n"))
	if err != nil {
		t.Fatalf("error codificando Windows-1252: %v", err)
	}
	zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{
		"20000000001-20261020-propuesta.txt": win1252Bytes,
	})

	parsed, err := ParseProposalZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), "COMPRAS", "080000", DefaultMaxProposalTXTBytes)
	if err != nil {
		t.Fatalf("error inesperado parseando RCE sintético: %v", err)
	}

	if parsed.Resumen.Codificacion != "Windows-1252" {
		t.Fatalf("se esperaba codificación Windows-1252, se obtuvo %q", parsed.Resumen.Codificacion)
	}
	if parsed.Activos[0].RazonSocial != "PROVEEDOR PEÑA S.A.C." {
		t.Fatalf("decodificación Windows-1252 incorrecta: %q", parsed.Activos[0].RazonSocial)
	}
	if parsed.Resumen.Conteos.Comprobantes != 1 || parsed.Resumen.Conteos.NC != 1 || parsed.Resumen.Conteos.ND != 1 || parsed.Resumen.Conteos.Anulados != 1 {
		t.Fatalf("conteos RCE inesperados: %+v", parsed.Resumen.Conteos)
	}
	tot := parsed.Resumen.Totales
	if tot.BaseImponible != 450.00 || tot.IGV != 81.00 || tot.NoGravado != 100.00 || tot.TotalComprobantes != 690.00 || tot.TotalNC != 118.00 || tot.TotalND != 59.00 || tot.TotalCP != 631.00 {
		t.Fatalf("totales RCE inesperados: %+v", tot)
	}
}

func TestProposalTXTParser_HeaderAltered_ReturnsFormatoInesperadoWithoutRowLeak(t *testing.T) {
	altered := append([]string(nil), rvieExpectedHeaders...)
	altered[14] = "Columna Inventada SUNAT"
	secretRUC := "20999888777"
	secretRazon := "RAZON SECRETA CONFIDENCIAL SAC"
	row := makeRVIERow("01", "F001", "00000001", secretRUC, secretRazon, "9876.54", "1777.78", "11654.32", "1")
	raw := []byte(strings.Join(altered, "|") + "\n" + row + "\n")
	zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{"propuesta.txt": raw})

	_, err := ParseProposalZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), "VENTAS", "140000", DefaultMaxProposalTXTBytes)
	var pErr *ProposalParseError
	if !errors.As(err, &pErr) || pErr.Code != ErrCodeFormatoInesperado {
		t.Fatalf("se esperaba FORMATO_INESPERADO, se obtuvo: %v", err)
	}
	if pErr.FormatInfo == nil || len(pErr.FormatInfo.Diferencias) != 1 || pErr.FormatInfo.Diferencias[0].Indice != 14 {
		t.Fatalf("detalle de cabecera esperado con diferencia en índice 14, obtenido: %+v", pErr.FormatInfo)
	}

	serialized, _ := json.Marshal(pErr)
	msg := pErr.Error() + " " + string(serialized)
	for _, forbidden := range []string{secretRUC, secretRazon, "9876.54", "11654.32"} {
		if strings.Contains(msg, forbidden) {
			t.Fatalf("el error de formato filtró datos de fila (%q): %s", forbidden, msg)
		}
	}
}

func TestProposalTXTParser_ZIPValidationRules(t *testing.T) {
	validTXT := []byte(strings.Join(rvieExpectedHeaders, "|") + "\n")

	cases := []struct {
		name    string
		entries map[string][]byte
	}{
		{"zip vacio", map[string][]byte{}},
		{"zip con 2 txt", map[string][]byte{"uno.txt": validTXT, "dos.txt": validTXT}},
		{"zip con entrada no-txt (csv)", map[string][]byte{"propuesta.csv": validTXT}},
		{"zip con txt y archivo extra no-txt", map[string][]byte{"propuesta.txt": validTXT, "extra.pdf": []byte("%PDF-1.4")}},
		{"zip con traversal ../", map[string][]byte{"../escape.txt": validTXT}},
		{"zip con subdirectorio /", map[string][]byte{"subdir/propuesta.txt": validTXT}},
		{"zip con separador windows backslash", map[string][]byte{`subdir\propuesta.txt`: validTXT}},
		{"zip con ruta absoluta", map[string][]byte{"/etc/passwd.txt": validTXT}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			zipBytes := buildSyntheticZIPBytes(t, tc.entries)
			_, _, err := ReadProposalTXTFromZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), DefaultMaxProposalTXTBytes)
			var pErr *ProposalParseError
			if !errors.As(err, &pErr) || pErr.Code != ErrCodeZipInesperado {
				t.Fatalf("caso %q: se esperaba %s, se obtuvo: %v", tc.name, ErrCodeZipInesperado, err)
			}
		})
	}
}

func TestProposalTXTParser_ZIPBomb_SpoofedSmallHeader(t *testing.T) {
	payload := bytes.Repeat([]byte("A"), 64*1024)
	zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{"bomba.txt": payload})

	t.Run("bomba_comprimida_lfh_cero_real_grande", func(t *testing.T) {
		if len(zipBytes) >= 4096 {
			t.Fatalf("el ZIP comprimido debería ser mucho menor a 4096 bytes, fue %d", len(zipBytes))
		}
		lfhIdx := bytes.Index(zipBytes, []byte("PK\x03\x04"))
		if lfhIdx < 0 {
			t.Fatalf("no se encontró cabecera local ZIP")
		}
		if binary.LittleEndian.Uint32(zipBytes[lfhIdx+22:lfhIdx+26]) != 0 {
			t.Fatalf("se esperaba tamaño LFH declarado=0 (data descriptor)")
		}

		_, _, err := ReadProposalTXTFromZIPReader(bytes.NewReader(zipBytes), int64(len(zipBytes)), 4096)
		var pErr *ProposalParseError
		if !errors.As(err, &pErr) || pErr.Code != ErrCodeZipDemasiadoGrande || pErr.HTTPStatus != http.StatusRequestEntityTooLarge {
			t.Fatalf("se esperaba %s (413), se obtuvo: %v", ErrCodeZipDemasiadoGrande, err)
		}
	})

	t.Run("bomba_cdh_falsificado_10_bytes", func(t *testing.T) {
		patched := append([]byte(nil), zipBytes...)
		lfhIdx := bytes.Index(patched, []byte("PK\x03\x04"))
		cdhIdx := bytes.Index(patched, []byte("PK\x01\x02"))
		binary.LittleEndian.PutUint32(patched[lfhIdx+22:lfhIdx+26], 10)
		binary.LittleEndian.PutUint32(patched[cdhIdx+24:cdhIdx+28], 10)

		zr, err := zip.NewReader(bytes.NewReader(patched), int64(len(patched)))
		if err != nil {
			t.Fatalf("ZIP parcheado no pudo abrirse: %v", err)
		}
		if zr.File[0].UncompressedSize64 != 10 {
			t.Fatalf("se esperaba tamaño CDH falsificado=10, se obtuvo %d", zr.File[0].UncompressedSize64)
		}

		_, _, err = ReadProposalTXTFromZIPReader(bytes.NewReader(patched), int64(len(patched)), 4096)
		var pErr *ProposalParseError
		if !errors.As(err, &pErr) || (pErr.Code != ErrCodeZipDemasiadoGrande && pErr.Code != ErrCodeZipInesperado) {
			t.Fatalf("se esperaba rechazo controlado, se obtuvo: %v", err)
		}
	})
}

func setupProposalEndpointEnv(t *testing.T) (*gin.Engine, string, *Repository, *Service, string, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("error inicializando SQLite en memoria: %v", err)
	}
	_, err = db.Exec(`INSERT INTO empresas (ruc, razon_social, usuario_sol, clave_sol_enc, client_id, client_secret_enc, activo) VALUES ('20000000001', 'EMPRESA DEMO S.A.C.', 'MODDATOS', 'enc_dummy', 'client_demo', 'secret_dummy', 1)`)
	if err != nil {
		t.Fatalf("error sembrando empresa sintética: %v", err)
	}

	storageDir := filepath.Join(t.TempDir(), "descargas")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		t.Fatalf("error creando storageDir temporal: %v", err)
	}

	repo := NewRepository(db)
	svc := NewService(repo, nil, nil, storageDir)
	ctrl := NewController(svc)

	token := middleware.InitSessionToken()
	r := gin.New()
	r.Use(middleware.SecurityMiddleware())
	apiGroup := r.Group("/api")
	ctrl.RegisterRoutes(apiGroup, func(c *gin.Context) { c.Next() })

	cleanup := func() { _ = db.Close() }
	return r, token, repo, svc, storageDir, cleanup
}

func TestProposalEndpoint_Auth_PathSecurity_States_Pagination_EstadoFilter_AndNoCPESync(t *testing.T) {
	r, token, repo, svc, storageDir, cleanup := setupProposalEndpointEnv(t)
	defer cleanup()

	// Preparar ZIP sintético válido dentro de storageDir con 4 activos + 1 anulado + 1 desconocido
	lines := []string{
		strings.Join(rvieExpectedHeaders, "|"),
		makeRVIERow("01", "F001", "00000001", "20000000002", "CLIENTE DEMO 1", "100.00", "18.00", "118.00", "1"),
		makeRVIERow("01", "F001", "00000002", "20000000002", "CLIENTE DEMO 1", "200.00", "36.00", "236.00", "1"),
		makeRVIERow("07", "FC01", "00000001", "20000000002", "CLIENTE DEMO 1", "-50.00", "-9.00", "-59.00", "1"),
		makeRVIERow("08", "FD01", "00000001", "20000000002", "CLIENTE DEMO 1", "10.00", "1.80", "11.80", "1"),
		makeRVIERow("01", "F001", "00000003", "20000000003", "CLIENTE ANULADO", "999.00", "179.82", "1178.82", "2"),
		makeRVIERow("01", "F001", "00000004", "20000000004", "CLIENTE DESCONOCIDO", "500.00", "90.00", "590.00", "9"),
	}
	zipBytes := buildSyntheticZIPBytes(t, map[string][]byte{
		"LE200000000012026100014040001EXP2.txt": []byte(strings.Join(lines, "\n") + "\n"),
	})

	insideDir := filepath.Join(storageDir, "20000000001", "VENTAS", "202610")
	if err := os.MkdirAll(insideDir, 0755); err != nil {
		t.Fatalf("error creando subcarpeta: %v", err)
	}
	insideZIP := filepath.Join(insideDir, "propuesta_demo.zip")
	if err := os.WriteFile(insideZIP, zipBytes, 0644); err != nil {
		t.Fatalf("error escribiendo ZIP sintético: %v", err)
	}

	dOK := &DescargaSire{
		EmpresaRUC: "20000000001", CodLibro: "140000", Libro: "VENTAS", Periodo: "202610",
		Ticket: "TKT-OK-001", Estado: string(EstadoCompletado), NombreArchivo: "propuesta_demo.zip", RutaLocal: insideZIP,
	}
	if err := repo.Create(dOK); err != nil {
		t.Fatalf("error creando descarga completada: %v", err)
	}

	dProc := &DescargaSire{
		EmpresaRUC: "20000000001", CodLibro: "140000", Libro: "VENTAS", Periodo: "202610",
		Ticket: "TKT-PROC-002", Estado: string(EstadoEnProceso),
	}
	if err := repo.Create(dProc); err != nil {
		t.Fatalf("error creando descarga en proceso: %v", err)
	}

	outsideDir := storageDir + "_evil"
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("error creando directorio externo: %v", err)
	}
	outsideZIP := filepath.Join(outsideDir, "escapado.zip")
	if err := os.WriteFile(outsideZIP, zipBytes, 0644); err != nil {
		t.Fatalf("error escribiendo ZIP externo: %v", err)
	}
	dOutside := &DescargaSire{
		EmpresaRUC: "20000000001", CodLibro: "140000", Libro: "VENTAS", Periodo: "202610",
		Ticket: "TKT-OUT-003", Estado: string(EstadoCompletado), NombreArchivo: "escapado.zip", RutaLocal: outsideZIP,
	}
	if err := repo.Create(dOutside); err != nil {
		t.Fatalf("error creando descarga externa: %v", err)
	}

	doReq := func(url string, withToken bool) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Host = "127.0.0.1:8080"
		if withToken {
			req.Header.Set("X-Local-Token", token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	if w := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt", dOK.ID), false); w.Code != http.StatusForbidden {
		t.Fatalf("sin X-Local-Token se esperaba 403, se obtuvo %d", w.Code)
	}
	if w := doReq("/api/descargas/99999/propuesta-txt", true); w.Code != http.StatusNotFound {
		t.Fatalf("ID inexistente debe devolver 404, se obtuvo %d", w.Code)
	}
	if w := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt", dProc.ID), true); w.Code != http.StatusConflict {
		t.Fatalf("descarga no completada debe devolver 409, se obtuvo %d", w.Code)
	}
	if w := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt", dOutside.ID), true); w.Code != http.StatusForbidden {
		t.Fatalf("ruta fuera de storageDir debe devolver 403, se obtuvo %d", w.Code)
	}

	symlinkPath := filepath.Join(insideDir, "link_afuera.zip")
	if symErr := os.Symlink(outsideZIP, symlinkPath); symErr == nil {
		dSym := &DescargaSire{
			EmpresaRUC: "20000000001", CodLibro: "140000", Libro: "VENTAS", Periodo: "202610",
			Ticket: "TKT-SYM-004", Estado: string(EstadoCompletado), NombreArchivo: "link_afuera.zip", RutaLocal: symlinkPath,
		}
		if err := repo.Create(dSym); err == nil {
			if wSym := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt", dSym.ID), true); wSym.Code != http.StatusForbidden {
				t.Fatalf("symlink apuntando fuera de storageDir debe devolver 403, se obtuvo %d", wSym.Code)
			}
		}
	}

	var cpeBefore int
	if err := repo.db.QueryRow(`SELECT COUNT(*) FROM comprobantes_cpe`).Scan(&cpeBefore); err != nil {
		t.Fatalf("error consultando comprobantes_cpe antes: %v", err)
	}

	type apiResp struct {
		Success    bool               `json:"success"`
		Resumen    ProposalSummary    `json:"resumen"`
		Paginacion ProposalPagination `json:"paginacion"`
		Filas      []ProposalRow      `json:"filas"`
		Anulados   []ProposalRow      `json:"anulados"`
	}

	// Página 1 (default estado=activo): resumen completo (306.80), 2 filas devueltas con marca "activo"
	wPage1 := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt?limit=2&offset=0", dOK.ID), true)
	if wPage1.Code != http.StatusOK {
		t.Fatalf("página 1 esperada 200, obtenida %d: %s", wPage1.Code, wPage1.Body.String())
	}
	var resp1 apiResp
	_ = json.Unmarshal(wPage1.Body.Bytes(), &resp1)
	if !resp1.Success || resp1.Paginacion.Estado != EstadoMarcaActivo || len(resp1.Filas) != 2 || !resp1.Paginacion.TieneMas || resp1.Paginacion.TotalFiltradas != 4 {
		t.Fatalf("paginación página 1 incorrecta: %+v filas=%d", resp1.Paginacion, len(resp1.Filas))
	}
	if resp1.Resumen.Totales.TotalCP != 306.80 || resp1.Filas[0].EstadoMarca != EstadoMarcaActivo {
		t.Fatalf("resumen completo o marca de estado incorrectos: total=%.2f marca=%q", resp1.Resumen.Totales.TotalCP, resp1.Filas[0].EstadoMarca)
	}

	// Filtro estado=anulado: devuelve solo anulados, pero el resumen sigue siendo del archivo completo (306.80)
	wAnul := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt?estado=anulado&limit=10&offset=0", dOK.ID), true)
	if wAnul.Code != http.StatusOK {
		t.Fatalf("estado=anulado esperado 200, obtenido %d", wAnul.Code)
	}
	var respAnul apiResp
	_ = json.Unmarshal(wAnul.Body.Bytes(), &respAnul)
	if respAnul.Paginacion.Estado != EstadoMarcaAnulado || len(respAnul.Filas) != 1 || respAnul.Filas[0].EstadoMarca != EstadoMarcaAnulado {
		t.Fatalf("filtro estado=anulado incorrecto: %+v filas=%+v", respAnul.Paginacion, respAnul.Filas)
	}
	if respAnul.Resumen.Totales.TotalCP != 306.80 {
		t.Fatalf("el resumen debe calcularse sobre el archivo completo incluso con estado=anulado: %.2f", respAnul.Resumen.Totales.TotalCP)
	}

	// Filtro estado=todos: devuelve las 6 filas con sus marcas ("activo", "anulado", "desconocido")
	wTodos := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt?estado=todos&limit=10&offset=0", dOK.ID), true)
	if wTodos.Code != http.StatusOK {
		t.Fatalf("estado=todos esperado 200, obtenido %d", wTodos.Code)
	}
	var respTodos apiResp
	_ = json.Unmarshal(wTodos.Body.Bytes(), &respTodos)
	if respTodos.Paginacion.Estado != EstadoFiltroTodos || len(respTodos.Filas) != 6 || respTodos.Paginacion.TotalFiltradas != 6 {
		t.Fatalf("filtro estado=todos incorrecto: %+v filas=%d", respTodos.Paginacion, len(respTodos.Filas))
	}
	if respTodos.Filas[4].EstadoMarca != EstadoMarcaAnulado || respTodos.Filas[5].EstadoMarca != EstadoMarcaDesconocido {
		t.Fatalf("marcas de estado inesperadas en estado=todos: [4]=%q [5]=%q", respTodos.Filas[4].EstadoMarca, respTodos.Filas[5].EstadoMarca)
	}

	// Parámetros inválidos -> 400
	if w := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt?estado=invalido", dOK.ID), true); w.Code != http.StatusBadRequest {
		t.Fatalf("estado inválido debe devolver 400, se obtuvo %d", w.Code)
	}
	if w := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt?limit=-5", dOK.ID), true); w.Code != http.StatusBadRequest {
		t.Fatalf("limit negativo debe devolver 400, se obtuvo %d", w.Code)
	}
	if w := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt?offset=-1", dOK.ID), true); w.Code != http.StatusBadRequest {
		t.Fatalf("offset negativo debe devolver 400, se obtuvo %d", w.Code)
	}

	// Verificar regla B5: comprobantes_cpe NO cambió
	var cpeAfter int
	if err := repo.db.QueryRow(`SELECT COUNT(*) FROM comprobantes_cpe`).Scan(&cpeAfter); err != nil {
		t.Fatalf("error consultando comprobantes_cpe después: %v", err)
	}
	if cpeBefore != cpeAfter || cpeAfter != 0 {
		t.Fatalf("el análisis TXT modificó comprobantes_cpe (antes=%d, después=%d)", cpeBefore, cpeAfter)
	}

	// Verificar que no se extrajo ningún .txt a disco
	entries, err := os.ReadDir(insideDir)
	if err != nil {
		t.Fatalf("error leyendo insideDir: %v", err)
	}
	for _, e := range entries {
		if strings.EqualFold(filepath.Ext(e.Name()), ".txt") {
			t.Fatalf("el analizador extrajo un .txt a disco (%s); debe leer solo en memoria", e.Name())
		}
	}

	// Verificar tope de tamaño configurable
	svc.maxProposalTXTBytes = 64
	if wSmall := doReq(fmt.Sprintf("/api/descargas/%d/propuesta-txt", dOK.ID), true); wSmall.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("con tope 64 bytes se esperaba 413, se obtuvo %d", wSmall.Code)
	}
}

// TestProposalTXTParser_RealFixtureManual ejecuta verificación manual local de cuadre contra un ZIP real
// ÚNICAMENTE cuando existe la variable de entorno SIRE_REAL_FIXTURE.
// Imprime SOLO libro, periodo, totales y conteos (nunca filas, RUC ni razones sociales).
func TestProposalTXTParser_RealFixtureManual(t *testing.T) {
	fixturePath := strings.TrimSpace(os.Getenv("SIRE_REAL_FIXTURE"))
	if fixturePath == "" {
		t.Skip("SIRE_REAL_FIXTURE no definida; verificación manual local omitida")
	}

	book := strings.ToUpper(strings.TrimSpace(os.Getenv("SIRE_REAL_BOOK")))
	if book == "" {
		upperPath := strings.ToUpper(fixturePath)
		if strings.Contains(upperPath, "VENTAS") || strings.Contains(upperPath, "140400") || strings.Contains(upperPath, "140000") {
			book = "VENTAS"
		} else {
			book = "COMPRAS"
		}
	}

	parsed, err := ParseProposalZIPFile(fixturePath, book, "", DefaultMaxProposalTXTBytes)
	if err != nil {
		t.Fatalf("error parseando fixture real: %v", err)
	}

	s := parsed.Resumen
	t.Logf(
		"CUADRE_MANUAL | libro=%s (%s) | periodo=%s | codificacion=%s | columnas=%d | incluye_anulados=%v | filas_no_pen=%d | advertencias=%d | conteos={total:%d activos:%d comprobantes:%d nc:%d nd:%d anulados:%d estado_desconocido:%d} | totales={bi:%.2f igv:%.2f no_gravado:%.2f total_comprobantes:%.2f total_nc:%.2f total_nd:%.2f total_cp:%.2f}",
		s.Libro, s.CodLibro, s.Periodo, s.Codificacion, s.Columnas, s.IncluyeAnuladosEnTotales,
		s.FilasNoPEN, len(s.Advertencias),
		s.Conteos.TotalFilas, s.Conteos.Activos, s.Conteos.Comprobantes, s.Conteos.NC, s.Conteos.ND, s.Conteos.Anulados, s.Conteos.EstadoDesconocido,
		s.Totales.BaseImponible, s.Totales.IGV, s.Totales.NoGravado, s.Totales.TotalComprobantes, s.Totales.TotalNC, s.Totales.TotalND, s.Totales.TotalCP,
	)
}
