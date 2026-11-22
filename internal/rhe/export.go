package rhe

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// BuildRHETXT genera el archivo TXT canónico delimitado por '|' respetando exactamente
// las 16 columnas oficiales del reporte Consulta Receptor de SUNAT.
func BuildRHETXT(recibos []ReciboHonorario) []byte {
	var b strings.Builder
	b.WriteString(strings.Join(OfficialRHETXTHeaders, "|"))
	b.WriteString("|\r\n")

	for _, r := range recibos {
		grat := r.Gratuito
		if grat == "" {
			grat = "-"
		}
		desc := r.Descripcion
		if desc == "" {
			desc = "-"
		}
		obs := r.Observacion
		if obs == "" {
			obs = "-"
		}
		cols := []string{
			r.FechaEmision,
			r.TipoDoc,
			r.NroDoc,
			r.Estado,
			r.TipoDocEmisor,
			r.NroDocEmisor,
			r.RazonEmisor,
			r.TipoRenta,
			grat,
			desc,
			obs,
			r.Moneda,
			fmt.Sprintf("%.2f", r.RentaBruta),
			fmt.Sprintf("%.2f", r.ImpuestoRenta),
			fmt.Sprintf("%.2f", r.RentaNeta),
			fmt.Sprintf("%.2f", r.MontoPendiente),
		}
		// Evitar que algún '|' dentro de un campo de texto rompa las columnas
		for i := range cols {
			cols[i] = strings.ReplaceAll(cols[i], "|", "/")
			cols[i] = strings.ReplaceAll(cols[i], "\r", " ")
			cols[i] = strings.ReplaceAll(cols[i], "\n", " ")
		}
		b.WriteString(strings.Join(cols, "|"))
		b.WriteString("|\r\n")
	}

	return []byte(b.String())
}

// BuildRHEExcelXLSX genera un libro Microsoft Excel (.xlsx) real y organizado con 2 hojas:
// - Hoja 1 "Detalle RHE Recibidos": columnas limpias y tipadas (RUC/Serie/Número como texto, montos #,##0.00),
//   cabecera congelada ATLOB Navy, filtro automático nativo y resaltado de recibos anulados/revertidos.
// - Hoja 2 "Resumen 4ta Categoría": KPIs oficiales (solo activos) y desglose por moneda.
func BuildRHEExcelXLSX(recibos []ReciboHonorario, resumen RHEResumenKPI, empresaRUC, razonSocial, periodo string, meta *ConsultaRHEMeta) ([]byte, error) {
	headers := []string{
		"Fecha Emisión",
		"Tipo Doc.",
		"Serie",
		"Número",
		"Nro. Comprobante",
		"Estado",
		"Tipo Doc. Emisor",
		"RUC / Doc. Emisor",
		"Apellidos y Nombres / Razón Social del Emisor",
		"Tipo Renta",
		"Gratuito",
		"Descripción del Servicio",
		"Observación",
		"Moneda",
		"Renta Bruta",
		"Impuesto a la Renta",
		"Renta Neta",
		"Monto Pendiente de Pago",
	}

	isCurrencyCol := func(idx int) bool {
		return idx >= 14 && idx <= 17
	}

	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = utf8.RuneCountInString(h) + 4
	}

	type detailRow struct {
		textVals  []string
		numVals   map[int]float64
		isAnulado bool
	}

	rows := make([]detailRow, 0, len(recibos))
	for _, r := range recibos {
		serie := r.Serie
		numero := r.Numero
		if serie == "" && numero == "" {
			serie, numero = splitSerieNumero(r.NroDoc)
		}
		vals := []string{
			r.FechaEmision,
			r.TipoDoc,
			serie,
			numero,
			r.NroDoc,
			r.Estado,
			r.TipoDocEmisor,
			r.NroDocEmisor,
			r.RazonEmisor,
			r.TipoRenta,
			r.Gratuito,
			r.Descripcion,
			r.Observacion,
			r.Moneda,
			fmt.Sprintf("%.2f", r.RentaBruta),
			fmt.Sprintf("%.2f", r.ImpuestoRenta),
			fmt.Sprintf("%.2f", r.RentaNeta),
			fmt.Sprintf("%.2f", r.MontoPendiente),
		}
		for i, v := range vals {
			if n := utf8.RuneCountInString(v) + 3; n > colWidths[i] {
				colWidths[i] = n
			}
		}
		rows = append(rows, detailRow{
			textVals: vals,
			numVals: map[int]float64{
				14: r.RentaBruta,
				15: r.ImpuestoRenta,
				16: r.RentaNeta,
				17: r.MontoPendiente,
			},
			isAnulado: r.EsAnulado || IsAnuladoORevertido(r.Estado),
		})
	}

	for i, w := range colWidths {
		if w < 12 {
			w = 12
		}
		if w > 46 {
			w = 46
		}
		colWidths[i] = w
	}

	// Construir Hoja 1: Detalle RHE Recibidos
	var s1 bytes.Buffer
	s1.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	s1.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)

	lastCol := colIndexToExcelName(len(headers) - 1)
	lastRow := len(rows) + 1
	fmt.Fprintf(&s1, `<dimension ref="A1:%s%d"/>`, lastCol, lastRow)
	s1.WriteString(`<sheetViews><sheetView tabSelected="1" workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`)
	s1.WriteString(`<sheetFormatPr defaultRowHeight="19"/>`)

	s1.WriteString(`<cols>`)
	for i, w := range colWidths {
		fmt.Fprintf(&s1, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, i+1, i+1, w)
	}
	s1.WriteString(`</cols>`)

	s1.WriteString(`<sheetData>`)
	s1.WriteString(`<row r="1" ht="24" customHeight="1">`)
	for i, h := range headers {
		writeInlineStringCell(&s1, colIndexToExcelName(i)+"1", 1, h)
	}
	s1.WriteString(`</row>`)

	for rIdx, row := range rows {
		rowNum := strconv.Itoa(rIdx + 2)
		fmt.Fprintf(&s1, `<row r="%s">`, rowNum)
		for cIdx := range headers {
			ref := colIndexToExcelName(cIdx) + rowNum
			if isCurrencyCol(cIdx) {
				style := 2
				if row.isAnulado {
					style = 8
				}
				writeNumberCell(&s1, ref, style, row.numVals[cIdx])
				continue
			}
			style := 0
			if row.isAnulado {
				style = 7
			}
			writeInlineStringCell(&s1, ref, style, row.textVals[cIdx])
		}
		s1.WriteString(`</row>`)
	}
	s1.WriteString(`</sheetData>`)
	fmt.Fprintf(&s1, `<autoFilter ref="A1:%s%d"/>`, lastCol, lastRow)
	s1.WriteString(`</worksheet>`)

	// Construir Hoja 2: Resumen 4ta Categoría
	sheet2XML := buildRHESummaryWorksheetXML(resumen, empresaRUC, razonSocial, periodo, meta)

	return packRHEOOXMLWorkbook(s1.Bytes(), sheet2XML)
}

func buildRHESummaryWorksheetXML(res RHEResumenKPI, empresaRUC, razonSocial, periodo string, meta *ConsultaRHEMeta) []byte {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	buf.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	buf.WriteString(`<sheetViews><sheetView workbookViewId="0"/></sheetViews>`)
	buf.WriteString(`<sheetFormatPr defaultRowHeight="20"/>`)
	buf.WriteString(`<cols>`)
	buf.WriteString(`<col min="1" max="1" width="38" customWidth="1"/>`)
	buf.WriteString(`<col min="2" max="2" width="24" customWidth="1"/>`)
	buf.WriteString(`<col min="3" max="3" width="22" customWidth="1"/>`)
	buf.WriteString(`<col min="4" max="4" width="22" customWidth="1"/>`)
	buf.WriteString(`<col min="5" max="5" width="22" customWidth="1"/>`)
	buf.WriteString(`</cols>`)
	buf.WriteString(`<sheetData>`)

	rowNum := 0
	nextRow := func() string {
		rowNum++
		return strconv.Itoa(rowNum)
	}

	addTitle := func(title string) {
		r := nextRow()
		fmt.Fprintf(&buf, `<row r="%s" ht="24" customHeight="1">`, r)
		writeInlineStringCell(&buf, "A"+r, 1, title)
		writeInlineStringCell(&buf, "B"+r, 1, "")
		writeInlineStringCell(&buf, "C"+r, 1, "")
		writeInlineStringCell(&buf, "D"+r, 1, "")
		writeInlineStringCell(&buf, "E"+r, 1, "")
		buf.WriteString(`</row>`)
	}
	addSection := func(a, b, c, d, e string) {
		r := nextRow()
		fmt.Fprintf(&buf, `<row r="%s" ht="21" customHeight="1">`, r)
		writeInlineStringCell(&buf, "A"+r, 5, a)
		writeInlineStringCell(&buf, "B"+r, 5, b)
		writeInlineStringCell(&buf, "C"+r, 5, c)
		writeInlineStringCell(&buf, "D"+r, 5, d)
		writeInlineStringCell(&buf, "E"+r, 5, e)
		buf.WriteString(`</row>`)
	}
	addKVText := func(k, v string) {
		r := nextRow()
		fmt.Fprintf(&buf, `<row r="%s">`, r)
		writeInlineStringCell(&buf, "A"+r, 0, k)
		writeInlineStringCell(&buf, "B"+r, 0, v)
		buf.WriteString(`</row>`)
	}
	addKVInt := func(k string, v int, note string) {
		r := nextRow()
		fmt.Fprintf(&buf, `<row r="%s">`, r)
		writeInlineStringCell(&buf, "A"+r, 0, k)
		writeNumberCell(&buf, "B"+r, 4, float64(v))
		writeInlineStringCell(&buf, "C"+r, 0, note)
		buf.WriteString(`</row>`)
	}
	addKVCurrency := func(k string, v float64, note string, highlight bool) {
		r := nextRow()
		fmt.Fprintf(&buf, `<row r="%s">`, r)
		ls, vs := 0, 2
		if highlight {
			ls, vs = 5, 6
		}
		writeInlineStringCell(&buf, "A"+r, ls, k)
		writeNumberCell(&buf, "B"+r, vs, v)
		writeInlineStringCell(&buf, "C"+r, ls, note)
		buf.WriteString(`</row>`)
	}
	addBlank := func() {
		r := nextRow()
		fmt.Fprintf(&buf, `<row r="%s"/>`, r)
	}

	periodoLabel := periodo
	if periodoLabel == "" {
		periodoLabel = "Todos los periodos guardados"
	}

	addTitle("ATLOB Sync · SUNAT — Recibos por Honorarios Electrónicos (Receptor)")
	addKVText("RUC Empresa Receptora", empresaRUC)
	if razonSocial != "" {
		addKVText("Razón Social", razonSocial)
	}
	addKVText("Rol SUNAT", "Receptor (4.ª Categoría · Gasto / Costo)")
	addKVText("Periodo consultado", periodoLabel)
	if meta != nil {
		addKVText("Rango de consulta SUNAT", fmt.Sprintf("%s al %s", meta.FechaDesde, meta.FechaHasta))
		addKVText("Última sincronización SOL", meta.ConsultadoAt.Local().Format("2006-01-02 15:04:05"))
	}
	addKVText("Fecha de exportación", time.Now().Format("2006-01-02 15:04:05"))
	addBlank()

	addSection("Conteo de Comprobantes (4.ª Categoría)", "Cantidad", "Observación", "", "")
	addKVInt("Total registros exportados", res.TotalRegistros, "Activos + anulados/revertidos")
	addKVInt("Recibos activos (No anulados)", res.Activos, "Incluidos en los importes totales")
	addKVInt("  • Con retención de IR (8%)", res.ConRetencion, "Impuesto a la Renta > 0")
	addKVInt("  • Sin retención de IR", res.SinRetencion, "Impuesto a la Renta = 0")
	addKVInt("Anulados / Revertidos", res.AnuladosRevertidos, "Excluidos de totales")
	addBlank()

	addSection("Totales Consolidados (Solo Activos)", "Importe", "Criterio", "", "")
	addKVCurrency("Total Renta Bruta", res.TotalRentaBruta, "Suma de recibos activos", false)
	addKVCurrency("Total Impuesto a la Renta (Retención)", res.TotalImpuestoRenta, "Retención 4.ª categoría", false)
	addKVCurrency("TOTAL RENTA NETA", res.TotalRentaNeta, "Renta Bruta − Retención", true)
	addKVCurrency("Total Monto Pendiente de Pago", res.TotalMontoPendiente, "Saldo pendiente reportado", false)
	addBlank()

	if len(res.PorMoneda) > 0 {
		addSection("Desglose por Moneda (Solo Activos)", "Cantidad", "Renta Bruta", "Impuesto Renta", "Renta Neta")
		for _, m := range res.PorMoneda {
			r := nextRow()
			fmt.Fprintf(&buf, `<row r="%s">`, r)
			writeInlineStringCell(&buf, "A"+r, 0, m.Moneda)
			writeNumberCell(&buf, "B"+r, 4, float64(m.Cantidad))
			writeNumberCell(&buf, "C"+r, 2, m.RentaBruta)
			writeNumberCell(&buf, "D"+r, 2, m.ImpuestoRenta)
			writeNumberCell(&buf, "E"+r, 2, m.RentaNeta)
			buf.WriteString(`</row>`)
		}
	}

	buf.WriteString(`</sheetData></worksheet>`)
	return buf.Bytes()
}

func packRHEOOXMLWorkbook(sheet1XML, sheet2XML []byte) ([]byte, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)

	files := []struct {
		name string
		data []byte
	}{
		{"[Content_Types].xml", []byte(rheOOXMLContentTypes)},
		{"_rels/.rels", []byte(rheOOXMLRootRels)},
		{"xl/workbook.xml", []byte(rheOOXMLWorkbook)},
		{"xl/_rels/workbook.xml.rels", []byte(rheOOXMLWorkbookRels)},
		{"xl/styles.xml", []byte(rheOOXMLStyles)},
		{"xl/worksheets/sheet1.xml", sheet1XML},
		{"xl/worksheets/sheet2.xml", sheet2XML},
	}

	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("error creando entrada XLSX '%s': %w", f.name, err)
		}
		if _, err := w.Write(f.data); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("error escribiendo entrada XLSX '%s': %w", f.name, err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("error cerrando libro XLSX RHE: %w", err)
	}
	return out.Bytes(), nil
}

func colIndexToExcelName(idx int) string {
	idx++
	var chars []byte
	for idx > 0 {
		idx--
		chars = append(chars, byte('A'+(idx%26)))
		idx /= 26
	}
	for i, j := 0, len(chars)-1; i < j; i, j = i+1, j-1 {
		chars[i], chars[j] = chars[j], chars[i]
	}
	return string(chars)
}

func sanitizeXMLString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == 0x09 || r == 0x0A || r == 0x0D || (r >= 0x20 && r != 0xFFFE && r != 0xFFFF) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func writeInlineStringCell(buf *bytes.Buffer, cellRef string, styleID int, value string) {
	if value == "" {
		fmt.Fprintf(buf, `<c r="%s" s="%d"/>`, cellRef, styleID)
		return
	}
	fmt.Fprintf(buf, `<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">`, cellRef, styleID)
	_ = xml.EscapeText(buf, []byte(sanitizeXMLString(value)))
	buf.WriteString(`</t></is></c>`)
}

func writeNumberCell(buf *bytes.Buffer, cellRef string, styleID int, val float64) {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		val = 0
	}
	fmt.Fprintf(buf, `<c r="%s" s="%d" t="n"><v>%s</v></c>`, cellRef, styleID, strconv.FormatFloat(val, 'f', -1, 64))
}

const rheOOXMLContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`

const rheOOXMLRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const rheOOXMLWorkbook = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Detalle RHE Recibidos" sheetId="1" r:id="rId1"/>
    <sheet name="Resumen 4ta Categoria" sheetId="2" r:id="rId2"/>
  </sheets>
</workbook>`

const rheOOXMLWorkbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const rheOOXMLStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <numFmts count="3">
    <numFmt numFmtId="164" formatCode="#,##0.00"/>
    <numFmt numFmtId="165" formatCode="0.000"/>
    <numFmt numFmtId="166" formatCode="#,##0"/>
  </numFmts>
  <fonts count="4">
    <font><sz val="10"/><color rgb="FF0B2545"/><name val="Segoe UI"/></font>
    <font><b/><sz val="10"/><color rgb="FFFFFFFF"/><name val="Segoe UI"/></font>
    <font><b/><sz val="10"/><color rgb="FF0B2545"/><name val="Segoe UI"/></font>
    <font><i/><sz val="10"/><color rgb="FF991B1B"/><name val="Segoe UI"/></font>
  </fonts>
  <fills count="6">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FF0B2545"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFE9F2F9"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFFEF6DF"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFFEF2F2"/><bgColor indexed="64"/></patternFill></fill>
  </fills>
  <borders count="2">
    <border><left/><right/><top/><bottom/><diagonal/></border>
    <border>
      <left style="thin"><color rgb="FFD8E3EF"/></left>
      <right style="thin"><color rgb="FFD8E3EF"/></right>
      <top style="thin"><color rgb="FFD8E3EF"/></top>
      <bottom style="thin"><color rgb="FFD8E3EF"/></bottom>
      <diagonal/>
    </border>
  </borders>
  <cellStyleXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>
  </cellStyleXfs>
  <cellXfs count="9">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1"/>
    <xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment vertical="center" wrapText="1"/></xf>
    <xf numFmtId="164" fontId="0" fillId="0" borderId="1" xfId="0" applyNumberFormat="1" applyBorder="1" applyAlignment="1"><alignment horizontal="right"/></xf>
    <xf numFmtId="165" fontId="0" fillId="0" borderId="1" xfId="0" applyNumberFormat="1" applyBorder="1" applyAlignment="1"><alignment horizontal="right"/></xf>
    <xf numFmtId="166" fontId="0" fillId="0" borderId="1" xfId="0" applyNumberFormat="1" applyBorder="1" applyAlignment="1"><alignment horizontal="right"/></xf>
    <xf numFmtId="0" fontId="2" fillId="3" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment vertical="center"/></xf>
    <xf numFmtId="164" fontId="2" fillId="4" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="right"/></xf>
    <xf numFmtId="0" fontId="3" fillId="5" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"/>
    <xf numFmtId="164" fontId="3" fillId="5" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="right"/></xf>
  </cellXfs>
  <cellStyles count="1">
    <cellStyle name="Normal" xfId="0" builtinId="0"/>
  </cellStyles>
</styleSheet>`
