package sire

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ExportedProposalFile representa un archivo listo para descargar por el cliente (XLSX, TXT o ZIP).
type ExportedProposalFile struct {
	Data        []byte
	FileName    string
	ContentType string
}

// ExportarPropuesta genera la exportación de una descarga SIRE completada en formato "xlsx", "txt" o "zip".
func (s *Service) ExportarPropuesta(id int64, formato string) (*ExportedProposalFile, error) {
	fmtLower := strings.ToLower(strings.TrimSpace(formato))
	if fmtLower == "" {
		fmtLower = "xlsx"
	}
	if fmtLower != "xlsx" && fmtLower != "txt" && fmtLower != "zip" {
		return nil, &ProposalParseError{
			Code:       "FORMATO_EXPORTACION_INVALIDO",
			HTTPStatus: http.StatusBadRequest,
			Message:    "formato de exportación no soportado (use xlsx, txt o zip)",
		}
	}

	d, err := s.repo.GetByID(id)
	if err != nil {
		return nil, &ProposalParseError{
			Code:       ErrCodeNoEncontrado,
			HTTPStatus: http.StatusNotFound,
			Message:    "descarga no encontrada",
		}
	}

	if d.Estado != string(EstadoCompletado) {
		return nil, &ProposalParseError{
			Code:       ErrCodeNoCompletado,
			HTTPStatus: http.StatusConflict,
			Message:    "la descarga aún no está completada",
			Detail:     fmt.Sprintf("estado actual: %s", d.Estado),
		}
	}

	safePath, err := resolveSafeStoragePath(s.storageDir, d.RutaLocal)
	if err != nil {
		return nil, err
	}

	fi, statErr := os.Stat(safePath)
	if statErr != nil || fi.IsDir() {
		return nil, &ProposalParseError{
			Code:       ErrCodeNoEncontrado,
			HTTPStatus: http.StatusNotFound,
			Message:    "el archivo físico de la propuesta no fue encontrado",
		}
	}

	baseLabel := buildProposalExportBaseName(d)

	// 1. Formato ZIP original tal cual SUNAT
	if fmtLower == "zip" {
		zipBytes, readErr := os.ReadFile(safePath)
		if readErr != nil {
			return nil, fmt.Errorf("error leyendo archivo ZIP: %w", readErr)
		}
		fname := strings.TrimSpace(d.NombreArchivo)
		if fname == "" {
			fname = baseLabel + ".zip"
		}
		return &ExportedProposalFile{
			Data:        zipBytes,
			FileName:    filepath.Base(fname),
			ContentType: "application/zip",
		}, nil
	}

	// Extraer TXT interno de forma segura (con límite anti ZIP-bomb)
	maxBytes := s.maxProposalTXTBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxProposalTXTBytes
	}

	f, openErr := os.Open(safePath)
	if openErr != nil {
		return nil, fmt.Errorf("error abriendo archivo ZIP: %w", openErr)
	}
	txtName, rawTXT, extractErr := ReadProposalTXTFromZIPReader(f, fi.Size(), maxBytes)
	_ = f.Close()
	if extractErr != nil {
		return nil, extractErr
	}

	// 2. Formato TXT oficial extraído
	if fmtLower == "txt" {
		outName := filepath.Base(strings.TrimSpace(txtName))
		if outName == "" || outName == "." {
			outName = baseLabel + ".txt"
		}
		return &ExportedProposalFile{
			Data:        rawTXT,
			FileName:    outName,
			ContentType: "text/plain; charset=utf-8",
		}, nil
	}

	// 3. Formato Excel organizado (.xlsx)
	xlsxBytes, buildErr := BuildProposalExcelXLSX(rawTXT, txtName, d)
	if buildErr != nil {
		return nil, buildErr
	}

	return &ExportedProposalFile{
		Data:        xlsxBytes,
		FileName:    baseLabel + ".xlsx",
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}, nil
}

func buildProposalExportBaseName(d *DescargaSire) string {
	libro := strings.ToUpper(strings.TrimSpace(d.Libro))
	if libro == "" {
		if d.CodLibro == "080000" {
			libro = "COMPRAS"
		} else {
			libro = "VENTAS"
		}
	}
	ruc := strings.TrimSpace(d.EmpresaRUC)
	per := strings.TrimSpace(d.Periodo)
	parts := []string{"Propuesta_SIRE", libro}
	if ruc != "" {
		parts = append(parts, ruc)
	}
	if per != "" {
		parts = append(parts, per)
	}
	return strings.Join(parts, "_")
}

// BuildProposalExcelXLSX convierte el contenido TXT oficial de SUNAT SIRE (delimitado por '|')
// en un libro Microsoft Excel (.xlsx) real y organizado con 2 hojas:
// - Hoja 1 "Detalle Propuesta": columnas separadas limpiamente, cabecera congelada, filtros automáticos,
//   RUC/series/números preservados como texto (sin pérdida de ceros) y montos como número (#,##0.00).
// - Hoja 2 "Resumen KPIs": conteos, totales oficiales (excluyendo anulados) y desglose por tipo CP.
func BuildProposalExcelXLSX(rawTXT []byte, txtName string, meta *DescargaSire) ([]byte, error) {
	libro := ""
	codLibro := ""
	if meta != nil {
		libro = meta.Libro
		codLibro = meta.CodLibro
	}

	parsed, err := ParseProposalTXTBytes(rawTXT, txtName, libro, codLibro)
	if err != nil {
		return nil, err
	}

	text, _, err := decodeProposalTXTBytes(rawTXT)
	if err != nil {
		return nil, err
	}

	normLibro := parsed.Resumen.Libro
	headers, rows, err := splitProposalTXTRows(text)
	if err != nil {
		return nil, err
	}

	// En COMPRAS (RCE), conservar siempre las 41 columnas núcleo (0..40) y añadir solo columnas CLU (41+)
	// que realmente tengan al menos un valor no vacío para evitar 39 columnas vacías innecesarias.
	keepCols := selectExportColumns(normLibro, headers, rows)

	estadoIdx := 34 // RVIE Est. Comp
	if normLibro == "COMPRAS" {
		estadoIdx = 39 // RCE Est. Comp.
	}

	// Construir matriz final con una columna final "Estado Legible"
	finalHeaders := make([]string, 0, len(keepCols)+1)
	colKinds := make([]excelColKind, 0, len(keepCols)+1)
	for _, idx := range keepCols {
		finalHeaders = append(finalHeaders, strings.TrimSpace(headers[idx]))
		colKinds = append(colKinds, classifyProposalExcelColumn(normLibro, idx))
	}
	finalHeaders = append(finalHeaders, "Estado Legible")
	colKinds = append(colKinds, colKindText)

	colWidths := make([]int, len(finalHeaders))
	for i, h := range finalHeaders {
		colWidths[i] = utf8.RuneCountInString(h) + 4
	}

	var detailRows []excelDetailRow
	for _, rawRow := range rows {
		estadoCode := ""
		if estadoIdx >= 0 && estadoIdx < len(rawRow) {
			estadoCode = strings.TrimSpace(rawRow[estadoIdx])
		}
		isAnulado := estadoCode == EstadoCompAnulado
		estadoLabel := "Desconocido"
		switch estadoCode {
		case EstadoCompActivo:
			estadoLabel = "Activo"
		case EstadoCompAnulado:
			estadoLabel = "Anulado"
		}

		vals := make([]string, 0, len(keepCols)+1)
		for pos, idx := range keepCols {
			val := ""
			if idx < len(rawRow) {
				val = strings.TrimSpace(rawRow[idx])
			}
			vals = append(vals, val)
			if n := utf8.RuneCountInString(val) + 2; n > colWidths[pos] {
				colWidths[pos] = n
			}
		}
		vals = append(vals, estadoLabel)
		if n := utf8.RuneCountInString(estadoLabel) + 3; n > colWidths[len(colWidths)-1] {
			colWidths[len(colWidths)-1] = n
		}

		detailRows = append(detailRows, excelDetailRow{
			Values:    vals,
			IsAnulado: isAnulado,
		})
	}

	// Acotar anchos para legibilidad óptima en Excel
	for i, w := range colWidths {
		if w < 11 {
			w = 11
		}
		if w > 42 {
			w = 42
		}
		colWidths[i] = w
	}

	sheet1XML := buildDetailWorksheetXML(finalHeaders, colKinds, colWidths, detailRows)
	sheet2XML := buildSummaryWorksheetXML(parsed, meta)

	return packOOXMLWorkbook(sheet1XML, sheet2XML)
}

type excelColKind int

const (
	colKindText excelColKind = iota
	colKindCurrency
	colKindExchangeRate
)

type excelDetailRow struct {
	Values    []string
	IsAnulado bool
}

func splitProposalTXTRows(text string) ([]string, [][]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var headers []string
	var rows [][]string

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		cols := strings.Split(line, ProposalTXTDelimiter)
		if headers == nil {
			headers = cols
			continue
		}
		rows = append(rows, cols)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	if len(headers) == 0 {
		return nil, nil, &ProposalParseError{
			Code:       ErrCodeFormatoInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "el archivo TXT de la propuesta está vacío",
		}
	}
	return headers, rows, nil
}

func selectExportColumns(normLibro string, headers []string, rows [][]string) []int {
	if normLibro != "COMPRAS" || len(headers) <= len(rceBaseHeaders) {
		cols := make([]int, len(headers))
		for i := range headers {
			cols[i] = i
		}
		return cols
	}

	cols := make([]int, 0, len(rceBaseHeaders)+4)
	for i := 0; i < len(rceBaseHeaders); i++ {
		cols = append(cols, i)
	}
	// Añadir columnas CLU adicionales solo si tienen contenido real
	for idx := len(rceBaseHeaders); idx < len(headers); idx++ {
		hasValue := false
		for _, r := range rows {
			if idx < len(r) && strings.TrimSpace(r[idx]) != "" {
				hasValue = true
				break
			}
		}
		if hasValue {
			cols = append(cols, idx)
		}
	}
	return cols
}

func classifyProposalExcelColumn(normLibro string, colIdx int) excelColKind {
	if normLibro == "COMPRAS" {
		if colIdx >= 14 && colIdx <= 24 {
			return colKindCurrency
		}
		if colIdx == 26 {
			return colKindExchangeRate
		}
		return colKindText
	}
	// VENTAS (RVIE)
	if (colIdx >= 13 && colIdx <= 25) || colIdx == 35 || colIdx == 36 {
		return colKindCurrency
	}
	if colIdx == 27 {
		return colKindExchangeRate
	}
	return colKindText
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

func writeXMLText(buf *bytes.Buffer, s string) {
	clean := sanitizeXMLString(s)
	_ = xml.EscapeText(buf, []byte(clean))
}

func writeInlineStringCell(buf *bytes.Buffer, cellRef string, styleID int, value string) {
	if value == "" {
		fmt.Fprintf(buf, `<c r="%s" s="%d"/>`, cellRef, styleID)
		return
	}
	fmt.Fprintf(buf, `<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">`, cellRef, styleID)
	writeXMLText(buf, value)
	buf.WriteString(`</t></is></c>`)
}

func writeNumberCell(buf *bytes.Buffer, cellRef string, styleID int, val float64) {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		val = 0
	}
	fmt.Fprintf(buf, `<c r="%s" s="%d" t="n"><v>%s</v></c>`, cellRef, styleID, strconv.FormatFloat(val, 'f', -1, 64))
}

func buildDetailWorksheetXML(headers []string, kinds []excelColKind, widths []int, rows []excelDetailRow) []byte {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	buf.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)

	lastCol := colIndexToExcelName(len(headers) - 1)
	lastRow := len(rows) + 1
	fmt.Fprintf(&buf, `<dimension ref="A1:%s%d"/>`, lastCol, lastRow)

	// Congelar fila 1 (cabeceras)
	buf.WriteString(`<sheetViews><sheetView tabSelected="1" workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`)
	buf.WriteString(`<sheetFormatPr defaultRowHeight="19"/>`)

	// Anchos de columna autoajustados
	buf.WriteString(`<cols>`)
	for i, w := range widths {
		fmt.Fprintf(&buf, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, i+1, i+1, w)
	}
	buf.WriteString(`</cols>`)

	buf.WriteString(`<sheetData>`)
	// Fila 1: Cabeceras con estilo ATLOB Navy
	buf.WriteString(`<row r="1" ht="24" customHeight="1">`)
	for cIdx, h := range headers {
		ref := colIndexToExcelName(cIdx) + "1"
		writeInlineStringCell(&buf, ref, 1, h)
	}
	buf.WriteString(`</row>`)

	// Filas de datos
	for rIdx, row := range rows {
		rowNum := rIdx + 2
		rowNumStr := strconv.Itoa(rowNum)
		fmt.Fprintf(&buf, `<row r="%s">`, rowNumStr)
		for cIdx, val := range row.Values {
			ref := colIndexToExcelName(cIdx) + rowNumStr
			kind := colKindText
			if cIdx < len(kinds) {
				kind = kinds[cIdx]
			}
			if kind == colKindCurrency || kind == colKindExchangeRate {
				if val == "" {
					style := 2
					if kind == colKindExchangeRate {
						style = 3
					}
					if row.IsAnulado {
						style = 8
					}
					fmt.Fprintf(&buf, `<c r="%s" s="%d"/>`, ref, style)
					continue
				}
				if num, err := strconv.ParseFloat(val, 64); err == nil && !math.IsNaN(num) && !math.IsInf(num, 0) {
					style := 2
					if kind == colKindExchangeRate {
						style = 3
					}
					if row.IsAnulado {
						style = 8
					}
					writeNumberCell(&buf, ref, style, num)
					continue
				}
			}
			style := 0
			if row.IsAnulado {
				style = 7
			}
			writeInlineStringCell(&buf, ref, style, val)
		}
		buf.WriteString(`</row>`)
	}
	buf.WriteString(`</sheetData>`)

	// Filtro automático nativo de Excel en toda la tabla
	fmt.Fprintf(&buf, `<autoFilter ref="A1:%s%d"/>`, lastCol, lastRow)
	buf.WriteString(`</worksheet>`)
	return buf.Bytes()
}

func buildSummaryWorksheetXML(parsed *ParsedProposalTXT, meta *DescargaSire) []byte {
	res := parsed.Resumen
	tot := res.Totales
	cnt := res.Conteos

	libroLabel := "Ventas (RVIE · 140000)"
	if res.Libro == "COMPRAS" {
		libroLabel = "Compras (RCE · 080000)"
	}

	ruc := ""
	ticket := res.Ticket
	if meta != nil {
		ruc = meta.EmpresaRUC
		if ticket == "" {
			ticket = meta.Ticket
		}
	}

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	buf.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	buf.WriteString(`<sheetViews><sheetView workbookViewId="0"/></sheetViews>`)
	buf.WriteString(`<sheetFormatPr defaultRowHeight="20"/>`)
	buf.WriteString(`<cols>`)
	buf.WriteString(`<col min="1" max="1" width="38" customWidth="1"/>`)
	buf.WriteString(`<col min="2" max="2" width="24" customWidth="1"/>`)
	buf.WriteString(`<col min="3" max="3" width="22" customWidth="1"/>`)
	buf.WriteString(`<col min="4" max="4" width="32" customWidth="1"/>`)
	buf.WriteString(`</cols>`)
	buf.WriteString(`<sheetData>`)

	rowNum := 0
	nextRow := func() string {
		rowNum++
		return strconv.Itoa(rowNum)
	}

	addTitleRow := func(title string) {
		r := nextRow()
		fmt.Fprintf(&buf, `<row r="%s" ht="24" customHeight="1">`, r)
		writeInlineStringCell(&buf, "A"+r, 1, title)
		writeInlineStringCell(&buf, "B"+r, 1, "")
		writeInlineStringCell(&buf, "C"+r, 1, "")
		writeInlineStringCell(&buf, "D"+r, 1, "")
		buf.WriteString(`</row>`)
	}
	addSectionRow := func(label, colB, colC, colD string) {
		r := nextRow()
		fmt.Fprintf(&buf, `<row r="%s" ht="21" customHeight="1">`, r)
		writeInlineStringCell(&buf, "A"+r, 5, label)
		writeInlineStringCell(&buf, "B"+r, 5, colB)
		writeInlineStringCell(&buf, "C"+r, 5, colC)
		writeInlineStringCell(&buf, "D"+r, 5, colD)
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
		lblStyle := 0
		valStyle := 2
		if highlight {
			lblStyle = 5
			valStyle = 6
		}
		writeInlineStringCell(&buf, "A"+r, lblStyle, k)
		writeNumberCell(&buf, "B"+r, valStyle, v)
		writeInlineStringCell(&buf, "C"+r, lblStyle, note)
		buf.WriteString(`</row>`)
	}
	addBlank := func() {
		r := nextRow()
		fmt.Fprintf(&buf, `<row r="%s"/>`, r)
	}

	addTitleRow("ATLOB Sync · SUNAT — Resumen Oficial Propuesta SIRE")
	addKVText("Libro electrónico", libroLabel)
	addKVText("RUC Contribuyente", ruc)
	addKVText("Periodo tributario", res.Periodo)
	addKVText("Ticket SUNAT", ticket)
	addKVText("Archivo TXT origen", res.NombreArchivoTXT)
	addKVText("Codificación / Columnas", fmt.Sprintf("%s · %d columnas", res.Codificacion, res.Columnas))
	addKVText("Fecha de exportación", time.Now().Format("2006-01-02 15:04:05"))
	addBlank()

	addSectionRow("Conteos de Registros", "Cantidad", "Observación", "")
	addKVInt("Total de filas en TXT", cnt.TotalFilas, "Incluye activos y anulados")
	addKVInt("Comprobantes activos (Estado 1)", cnt.Activos, "Incluidos en totales")
	addKVInt("  • Comprobantes regulares", cnt.Comprobantes, "Suman")
	addKVInt("  • Notas de Débito (08/88/98)", cnt.ND, "Suman")
	addKVInt("  • Notas de Crédito (07/87/97)", cnt.NC, "Restan")
	addKVInt("Anulados (Estado 2)", cnt.Anulados, "Excluidos de totales")
	if cnt.EstadoDesconocido > 0 {
		addKVInt("Estado desconocido", cnt.EstadoDesconocido, "Excluidos de totales")
	}
	addBlank()

	addSectionRow("Totales Oficiales (Solo Activos)", "Importe", "Criterio SUNAT", "")
	addKVCurrency("Base Imponible", tot.BaseImponible, "Gravada según SUNAT", false)
	addKVCurrency("IGV / IPM", tot.IGV, "Impuesto según SUNAT", false)
	noGrav := tot.NoGravado
	if res.Libro == "VENTAS" {
		noGrav = tot.NoGravado + tot.Exonerado + tot.Inafecto
	}
	addKVCurrency("No Gravado / Exonerado / Inafecto", noGrav, "Operaciones no gravadas", false)
	addKVCurrency("Comprobantes regulares (+)", tot.TotalComprobantes, "", false)
	addKVCurrency("Notas de Débito (+)", tot.TotalND, "", false)
	addKVCurrency("Notas de Crédito (−)", tot.TotalNC, "Magnitud que resta", false)
	addKVCurrency("TOTAL NETO PROPUESTA", tot.TotalCP, "Total = Comprobantes + ND − NC", true)
	addBlank()

	if len(res.PorTipo) > 0 {
		addSectionRow("Subtotales por Tipo de Comprobante (Activos)", "Cantidad", "Total", "Categoría")
		for _, item := range res.PorTipo {
			r := nextRow()
			fmt.Fprintf(&buf, `<row r="%s">`, r)
			writeInlineStringCell(&buf, "A"+r, 0, fmt.Sprintf("Tipo %s", item.TipoCP))
			writeNumberCell(&buf, "B"+r, 4, float64(item.Cantidad))
			writeNumberCell(&buf, "C"+r, 2, item.TotalCP)
			writeInlineStringCell(&buf, "D"+r, 0, strings.ToUpper(item.Categoria))
			buf.WriteString(`</row>`)
		}
	}

	buf.WriteString(`</sheetData></worksheet>`)
	return buf.Bytes()
}

func packOOXMLWorkbook(sheet1XML, sheet2XML []byte) ([]byte, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)

	files := []struct {
		name string
		data []byte
	}{
		{"[Content_Types].xml", []byte(ooxmlContentTypes)},
		{"_rels/.rels", []byte(ooxmlRootRels)},
		{"xl/workbook.xml", []byte(ooxmlWorkbook)},
		{"xl/_rels/workbook.xml.rels", []byte(ooxmlWorkbookRels)},
		{"xl/styles.xml", []byte(ooxmlStyles)},
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
		return nil, fmt.Errorf("error cerrando archivo XLSX: %w", err)
	}
	return out.Bytes(), nil
}

const ooxmlContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`

const ooxmlRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const ooxmlWorkbook = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Detalle Propuesta" sheetId="1" r:id="rId1"/>
    <sheet name="Resumen KPIs" sheetId="2" r:id="rId2"/>
  </sheets>
</workbook>`

const ooxmlWorkbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const ooxmlStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
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
