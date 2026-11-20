package cpe

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// GenerateComprobantePDF genera un PDF 1.4 válido en Go puro (sin dependencias externas)
// con la ficha / representación estructurada del comprobante registrado en SIRE/CPE (sin XML).
func GenerateComprobantePDF(c *ComprobanteCPE, razonEmpresa string) []byte {
	var stream bytes.Buffer

	// Encabezado ATLOB Deep Navy + franja dorada
	stream.WriteString("0.024 0.137 0.243 rg\n")
	stream.WriteString("36 740 523 72 re f\n")
	stream.WriteString("0.878 0.655 0.094 rg\n")
	stream.WriteString("36 737 523 3 re f\n")

	// Título blanco en cabecera
	stream.WriteString("1 1 1 rg\n")
	writePDFText(&stream, "F2", 15, 52, 785, "COMPROBANTE ELECTRONICO - REGISTRO SIRE / SUNAT")
	writePDFText(&stream, "F1", 10, 52, 764, fmt.Sprintf("Empresa: %s  |  RUC: %s", truncateStr(razonEmpresa, 48), c.EmpresaRUC))
	writePDFText(&stream, "F1", 9, 52, 749, fmt.Sprintf("Origen: %s   |   Periodo Tributario: %s", c.Origen, valueOrDash(c.Periodo)))

	// Recuadro principal de identificación del documento
	stream.WriteString("0.94 0.96 0.99 rg\n")
	stream.WriteString("36 650 523 76 re f\n")
	stream.WriteString("0.80 0.85 0.92 RG 1 w\n")
	stream.WriteString("36 650 523 76 re S\n")

	stream.WriteString("0.06 0.09 0.16 rg\n")
	writePDFText(&stream, "F2", 13, 52, 704, fmt.Sprintf("%s  %s-%s", tipoCPNombre(c.TipoCP), c.Serie, c.Numero))
	writePDFText(&stream, "F1", 10, 52, 684, fmt.Sprintf("Fecha de Emision: %s", valueOrDash(c.FechaEmision)))
	writePDFText(&stream, "F1", 10, 290, 684, fmt.Sprintf("Estado SUNAT: %s", valueOrDash(c.EstadoSUNAT)))
	writePDFText(&stream, "F1", 9, 52, 666, fmt.Sprintf("CAR SUNAT: %s", valueOrDash(c.CarSUNAT)))

	// Datos de la contraparte
	labelContraparte := "Cliente / Adquiriente"
	if strings.ToUpper(c.Origen) == "COMPRAS" {
		labelContraparte = "Proveedor / Emisor"
	}
	writePDFText(&stream, "F2", 11, 42, 622, "DATOS DE LA CONTRAPARTE ("+strings.ToUpper(labelContraparte)+")")
	stream.WriteString("0.85 0.88 0.92 RG 0.7 w\n")
	stream.WriteString("36 614 m 559 614 l S\n")

	writePDFText(&stream, "F1", 10, 42, 594, fmt.Sprintf("Documento / RUC: %s", valueOrDash(c.RucContraparte)))
	writePDFText(&stream, "F1", 10, 42, 576, fmt.Sprintf("Razon Social / Nombre: %s", truncateStr(valueOrDash(c.RazonContraparte), 68)))

	// Resumen de importes
	writePDFText(&stream, "F2", 11, 42, 536, "RESUMEN DE IMPORTES")
	stream.WriteString("36 528 m 559 528 l S\n")

	stream.WriteString("0.97 0.98 0.99 rg\n")
	stream.WriteString("36 436 523 82 re f\n")
	stream.WriteString("0.85 0.88 0.92 RG 0.7 w\n")
	stream.WriteString("36 436 523 82 re S\n")

	stream.WriteString("0.06 0.09 0.16 rg\n")
	moneda := valueOrDash(c.Moneda)
	writePDFText(&stream, "F1", 10, 52, 496, fmt.Sprintf("Moneda: %s", moneda))
	writePDFText(&stream, "F1", 10, 52, 476, fmt.Sprintf("Base Imponible: %s %.2f", moneda, c.BaseImponible))
	writePDFText(&stream, "F1", 10, 290, 476, fmt.Sprintf("IGV / IPM: %s %.2f", moneda, c.IGV))
	writePDFText(&stream, "F2", 12, 52, 450, fmt.Sprintf("IMPORTE TOTAL: %s %.2f", moneda, c.MontoTotal))

	// Estado de archivos adjuntos
	writePDFText(&stream, "F2", 11, 42, 400, "ARCHIVOS DIGITALES ASOCIADOS EN REPOSITORIO LOCAL")
	stream.WriteString("36 392 m 559 392 l S\n")
	xmlStatus := "No adjunto (use Descarga Masiva SUNAT o importe el XML)"
	if c.TieneXML {
		xmlStatus = "Disponible en repositorio local"
	}
	cdrStatus := "No descargado (use 'Consultar CDR en SUNAT')"
	if c.TieneCDR {
		cdrStatus = "Constancia CDR disponible (.zip)"
	}
	pdfStatus := "Ficha generada desde registro SIRE"
	if c.TienePDF {
		pdfStatus = "PDF original adjunto disponible"
	}
	writePDFText(&stream, "F1", 9, 42, 372, "XML UBL 2.1: "+xmlStatus)
	writePDFText(&stream, "F1", 9, 42, 356, "CDR SUNAT:   "+cdrStatus)
	writePDFText(&stream, "F1", 9, 42, 340, "PDF:         "+pdfStatus)

	// Pie de página informativo
	stream.WriteString("0.40 0.45 0.52 rg\n")
	stream.WriteString("36 88 m 559 88 l S\n")
	writePDFText(&stream, "F1", 8, 42, 72, "Nota: Representacion impresa generada automaticamente a partir de los datos oficiales del registro SIRE - SUNAT.")
	writePDFText(&stream, "F1", 8, 42, 58, "Descargue el XML desde SUNAT SOL para obtener el detalle completo de items, firma hash y codigo QR oficial.")
	writePDFText(&stream, "F1", 8, 42, 44, fmt.Sprintf("Generado por ATLOB Sync · SUNAT (ATLOB Consultores) el %s", time.Now().Format("2006-01-02 15:04:05")))

	return assembleMultiPagePDF([][]byte{stream.Bytes()})
}

// GenerateComprobantePDFFromUBL genera una Representación Impresa comercial A4 en PDF 1.4
// directamente desde el XML UBL 2.1 oficial de SUNAT: incluye emisor, receptor, tabla paginada
// de ítems/descripciones, cuotas de crédito, leyendas, Hash DigestValue y Código QR oficial SUNAT en vectores.
func GenerateComprobantePDFFromUBL(c *ComprobanteCPE, meta *UBLMetadata, razonEmpresa string) []byte {
	if meta == nil {
		return GenerateComprobantePDF(c, razonEmpresa)
	}

	// Resolver campos consolidados (prioridad XML UBL, respaldo registro SIRE)
	rucEmisor := meta.RucEmisor
	razonEmisor := meta.RazonEmisor
	rucCliente := meta.RucCliente
	razonCliente := meta.RazonCliente
	tipoCP := padTipoCP(meta.TipoCP)
	serie := meta.Serie
	numero := meta.Numero
	fecha := meta.FechaEmision
	moneda := meta.Moneda
	baseImp := meta.BaseImponible
	igv := meta.IGV
	total := meta.MontoTotal

	if c != nil {
		if tipoCP == "" {
			tipoCP = padTipoCP(c.TipoCP)
		}
		if serie == "" {
			serie = c.Serie
		}
		if numero == "" {
			numero = c.Numero
		}
		if fecha == "" {
			fecha = c.FechaEmision
		}
		if moneda == "" {
			moneda = c.Moneda
		}
		if baseImp == 0 {
			baseImp = c.BaseImponible
		}
		if igv == 0 {
			igv = c.IGV
		}
		if total == 0 {
			total = c.MontoTotal
		}
		if strings.ToUpper(c.Origen) == "COMPRAS" {
			if rucEmisor == "" {
				rucEmisor = c.RucContraparte
			}
			if razonEmisor == "" {
				razonEmisor = c.RazonContraparte
			}
			if rucCliente == "" {
				rucCliente = c.EmpresaRUC
			}
			if razonCliente == "" {
				razonCliente = razonEmpresa
			}
		} else {
			if rucEmisor == "" {
				rucEmisor = c.EmpresaRUC
			}
			if razonEmisor == "" {
				razonEmisor = razonEmpresa
			}
			if rucCliente == "" {
				rucCliente = c.RucContraparte
			}
			if razonCliente == "" {
				razonCliente = c.RazonContraparte
			}
		}
	}
	if moneda == "" {
		moneda = "PEN"
	}
	monedaSym := monedaSimbolo(moneda)

	var pages [][]byte
	var cur bytes.Buffer

	drawTableHeader := func(buf *bytes.Buffer, yTop int) int {
		buf.WriteString("0.024 0.137 0.243 rg\n")
		buf.WriteString(fmt.Sprintf("36 %d 523 20 re f\n", yTop-20))
		buf.WriteString("0.878 0.655 0.094 rg\n")
		buf.WriteString(fmt.Sprintf("36 %d 523 2 re f\n", yTop-22))
		buf.WriteString("1 1 1 rg\n")
		writePDFText(buf, "F2", 8, 42, yTop-13, "#")
		writePDFText(buf, "F2", 8, 62, yTop-13, "CANT.")
		writePDFText(buf, "F2", 8, 102, yTop-13, "UND")
		writePDFText(buf, "F2", 8, 138, yTop-13, "DESCRIPCION")
		writePDFText(buf, "F2", 8, 434, yTop-13, "P. UNIT")
		writePDFText(buf, "F2", 8, 498, yTop-13, "IMPORTE")
		buf.WriteString("0.024 0.137 0.243 rg\n")
		return yTop - 24
	}

	// ---- CABECERA PÁGINA 1 ----
	// Columna izquierda: Emisor
	cur.WriteString("0.06 0.09 0.16 rg\n")
	emisorLines := wrapText(valueOrDash(razonEmisor), 42, 2)
	yEmisor := 796
	for _, ln := range emisorLines {
		writePDFText(&cur, "F2", 12, 38, yEmisor, ln)
		yEmisor -= 15
	}
	if meta.DireccionEmisor != "" {
		cur.WriteString("0.30 0.35 0.42 rg\n")
		for _, ln := range wrapText(meta.DireccionEmisor, 54, 2) {
			writePDFText(&cur, "F1", 8, 38, yEmisor, ln)
			yEmisor -= 11
		}
	}

	// Columna derecha: Recuadro oficial RUC / Tipo / Serie-Número
	cur.WriteString("0.96 0.97 0.99 rg\n")
	cur.WriteString("362 738 197 72 re f\n")
	cur.WriteString("0.06 0.09 0.16 RG 1.2 w\n")
	cur.WriteString("362 738 197 72 re S\n")

	cur.WriteString("0.06 0.09 0.16 rg\n")
	writePDFText(&cur, "F2", 10, 398, 792, fmt.Sprintf("R.U.C. %s", valueOrDash(rucEmisor)))
	// Banda azul con el tipo de comprobante
	cur.WriteString("0.06 0.09 0.16 rg\n")
	cur.WriteString("362 764 197 20 re f\n")
	cur.WriteString("1 1 1 rg\n")
	tipoCorto := tipoCPNombreCorto(tipoCP)
	xTipo := 362 + (197-len(tipoCorto)*5)/2
	if xTipo < 368 {
		xTipo = 368
	}
	writePDFText(&cur, "F2", 9, xTipo, 771, tipoCorto)
	cur.WriteString("0.06 0.09 0.16 rg\n")
	docNum := fmt.Sprintf("%s - %s", serie, numero)
	xNum := 362 + (197-len(docNum)*6)/2
	if xNum < 370 {
		xNum = 370
	}
	writePDFText(&cur, "F2", 11, xNum, 746, docNum)

	// ---- PANEL DE DATOS DEL CLIENTE Y DOCUMENTO ----
	panelTop := 726
	panelH := 76
	if meta.DocModificadoRef != "" {
		panelH = 92
	}
	cur.WriteString("0.98 0.99 1.00 rg\n")
	cur.WriteString(fmt.Sprintf("36 %d 523 %d re f\n", panelTop-panelH, panelH))
	cur.WriteString("0.82 0.86 0.92 RG 0.7 w\n")
	cur.WriteString(fmt.Sprintf("36 %d 523 %d re S\n", panelTop-panelH, panelH))

	cur.WriteString("0.06 0.09 0.16 rg\n")
	yInfo := panelTop - 15
	writePDFText(&cur, "F2", 8, 44, yInfo, "ADQUIRIENTE / CLIENTE:")
	writePDFText(&cur, "F1", 8, 152, yInfo, truncateStr(valueOrDash(razonCliente), 42))
	writePDFText(&cur, "F2", 8, 392, yInfo, "FECHA EMISION:")
	fechaDisp := valueOrDash(fecha)
	if meta.HoraEmision != "" {
		fechaDisp += " " + truncateStr(meta.HoraEmision, 8)
	}
	writePDFText(&cur, "F1", 8, 470, yInfo, truncateStr(fechaDisp, 19))

	yInfo -= 15
	writePDFText(&cur, "F2", 8, 44, yInfo, "DOC. IDENTIDAD:")
	writePDFText(&cur, "F1", 8, 152, yInfo, fmt.Sprintf("%s (%s)", valueOrDash(rucCliente), tipoDocClienteNombre(meta.TipoDocCliente)))
	writePDFText(&cur, "F2", 8, 392, yInfo, "MONEDA:")
	writePDFText(&cur, "F1", 8, 470, yInfo, monedaNombre(moneda))

	yInfo -= 15
	writePDFText(&cur, "F2", 8, 44, yInfo, "DIRECCION:")
	writePDFText(&cur, "F1", 8, 152, yInfo, truncateStr(valueOrDash(meta.DireccionCliente), 44))
	writePDFText(&cur, "F2", 8, 392, yInfo, "FORMA PAGO:")
	formaPago := meta.FormaPago
	if formaPago == "" {
		formaPago = "Contado"
	}
	writePDFText(&cur, "F1", 8, 470, yInfo, formaPago)

	yInfo -= 15
	if meta.FechaVencimiento != "" {
		writePDFText(&cur, "F2", 8, 44, yInfo, "FECHA VENCIMIENTO:")
		writePDFText(&cur, "F1", 8, 152, yInfo, meta.FechaVencimiento)
	} else if c != nil && c.CarSUNAT != "" {
		writePDFText(&cur, "F2", 8, 44, yInfo, "CAR SUNAT:")
		writePDFText(&cur, "F1", 8, 152, yInfo, truncateStr(c.CarSUNAT, 34))
	}
	if c != nil && c.Periodo != "" {
		writePDFText(&cur, "F2", 8, 392, yInfo, "PERIODO SIRE:")
		writePDFText(&cur, "F1", 8, 470, yInfo, c.Periodo)
	}

	if meta.DocModificadoRef != "" {
		yInfo -= 15
		writePDFText(&cur, "F2", 8, 44, yInfo, "DOC. MODIFICADO:")
		writePDFText(&cur, "F1", 8, 152, yInfo, fmt.Sprintf("%s  |  Motivo: %s", meta.DocModificadoRef, truncateStr(meta.MotivoNota, 46)))
	}

	// ---- TABLA DE ÍTEMS ----
	y := panelTop - panelH - 12
	y = drawTableHeader(&cur, y)

	if len(meta.Items) == 0 {
		cur.WriteString("0.40 0.45 0.52 rg\n")
		writePDFText(&cur, "F1", 8, 44, y-12, "(El XML no contiene lineas de detalle parseables; consulte los totales abajo.)")
		y -= 24
	} else {
		for idx, it := range meta.Items {
			desc := strings.TrimSpace(it.Descripcion)
			if it.Codigo != "" {
				desc = "[" + it.Codigo + "] " + desc
			}
			descLines := wrapText(valueOrDash(desc), 54, 4)
			rowH := len(descLines)*11 + 7

			// Salto de página si no entra la fila
			if y-rowH < 90 {
				writePageFooter(&cur, len(pages)+1)
				pages = append(pages, cur.Bytes())
				cur.Reset()
				// Encabezado compacto en página siguiente
				cur.WriteString("0.06 0.09 0.16 rg\n")
				writePDFText(&cur, "F2", 10, 38, 804, fmt.Sprintf("%s  |  %s %s-%s (Continuacion)", truncateStr(razonEmisor, 45), tipoCorto, serie, numero))
				y = drawTableHeader(&cur, 790)
			}

			// Fondo alterno sutil
			if idx%2 == 1 {
				cur.WriteString("0.97 0.98 0.99 rg\n")
				cur.WriteString(fmt.Sprintf("36 %d 523 %d re f\n", y-rowH, rowH))
			}

			cur.WriteString("0.06 0.09 0.16 rg\n")
			rowBaseY := y - 12
			itemID := it.ID
			if itemID == "" {
				itemID = fmt.Sprintf("%d", idx+1)
			}
			writePDFText(&cur, "F1", 8, 42, rowBaseY, truncateStr(itemID, 4))
			writePDFText(&cur, "F1", 8, 62, rowBaseY, formatCantidad(it.Cantidad))
			writePDFText(&cur, "F1", 8, 102, rowBaseY, truncateStr(it.Unidad, 5))

			for li, dl := range descLines {
				writePDFText(&cur, "F1", 8, 138, rowBaseY-li*11, dl)
			}

			unitVal := it.PrecioUnitario
			if unitVal == 0 {
				unitVal = it.ValorUnitario
			}
			lineTotal := it.ValorVenta
			if it.IGV > 0 && it.PrecioUnitario > 0 && it.ValorUnitario > 0 && it.PrecioUnitario > it.ValorUnitario {
				// Mostrar total de línea con IGV si P.Unit incluye IGV
				lineTotal = it.ValorVenta + it.IGV
			}
			writePDFText(&cur, "F1", 8, 434, rowBaseY, fmt.Sprintf("%.2f", unitVal))
			writePDFText(&cur, "F1", 8, 498, rowBaseY, fmt.Sprintf("%.2f", lineTotal))

			// Línea separadora inferior
			cur.WriteString("0.88 0.91 0.95 RG 0.4 w\n")
			cur.WriteString(fmt.Sprintf("36 %d m 559 %d l S\n", y-rowH, y-rowH))
			y -= rowH
		}
	}

	// ---- BLOQUE FINAL (CUOTAS + TOTALES + LEYENDAS + CÓDIGO QR VECTORIAL + HASH) ----
	neededBottom := 185
	if len(meta.Cuotas) > 0 {
		neededBottom += 24 + len(meta.Cuotas)*12
	}
	if y < neededBottom {
		writePageFooter(&cur, len(pages)+1)
		pages = append(pages, cur.Bytes())
		cur.Reset()
		cur.WriteString("0.06 0.09 0.16 rg\n")
		writePDFText(&cur, "F2", 10, 38, 804, fmt.Sprintf("%s  |  %s %s-%s (Resumen)", truncateStr(razonEmisor, 45), tipoCorto, serie, numero))
		y = 780
	}

	// Cuotas (si es al Crédito)
	if len(meta.Cuotas) > 0 {
		y -= 14
		cur.WriteString("0.06 0.09 0.16 rg\n")
		writePDFText(&cur, "F2", 8, 40, y, "INFORMACION DEL CREDITO / CUOTAS:")
		y -= 12
		for _, q := range meta.Cuotas {
			writePDFText(&cur, "F1", 8, 48, y, fmt.Sprintf("- %s   |   Vencimiento: %s   |   Monto: %s %.2f", q.ID, valueOrDash(q.FechaVencimiento), monedaSym, q.Monto))
			y -= 11
		}
	}

	// Bloque de Totales (derecha) y Leyendas + QR (izquierda)
	y -= 14
	summaryTop := y

	// Totales (derecha, x=362..559)
	type totalRow struct {
		label string
		val   float64
		bold  bool
	}
	rowsTot := make([]totalRow, 0, 6)
	opGrav := meta.OpGravada
	if opGrav == 0 && baseImp > 0 {
		opGrav = baseImp
	}
	rowsTot = append(rowsTot, totalRow{label: "OP. GRAVADA", val: opGrav})
	if meta.OpExonerada > 0 {
		rowsTot = append(rowsTot, totalRow{label: "OP. EXONERADA", val: meta.OpExonerada})
	}
	if meta.OpInafecta > 0 {
		rowsTot = append(rowsTot, totalRow{label: "OP. INAFECTA", val: meta.OpInafecta})
	}
	if meta.OpGratuita > 0 {
		rowsTot = append(rowsTot, totalRow{label: "OP. GRATUITA", val: meta.OpGratuita})
	}
	rowsTot = append(rowsTot, totalRow{label: "I.G.V. (18%)", val: igv})
	rowsTot = append(rowsTot, totalRow{label: "IMPORTE TOTAL", val: total, bold: true})

	totBoxH := len(rowsTot)*17 + 12
	cur.WriteString("0.97 0.98 0.99 rg\n")
	cur.WriteString(fmt.Sprintf("362 %d 197 %d re f\n", summaryTop-totBoxH, totBoxH))
	cur.WriteString("0.80 0.85 0.92 RG 0.8 w\n")
	cur.WriteString(fmt.Sprintf("362 %d 197 %d re S\n", summaryTop-totBoxH, totBoxH))

	yt := summaryTop - 16
	for _, rt := range rowsTot {
		if rt.bold {
			cur.WriteString("0.06 0.09 0.16 rg\n")
			cur.WriteString(fmt.Sprintf("362 %d 197 20 re f\n", yt-5))
			cur.WriteString("1 1 1 rg\n")
			writePDFText(&cur, "F2", 9, 370, yt+1, rt.label)
			writePDFText(&cur, "F2", 9, 476, yt+1, fmt.Sprintf("%s %.2f", monedaSym, rt.val))
			cur.WriteString("0.06 0.09 0.16 rg\n")
		} else {
			cur.WriteString("0.06 0.09 0.16 rg\n")
			writePDFText(&cur, "F1", 8, 370, yt, rt.label)
			writePDFText(&cur, "F1", 8, 478, yt, fmt.Sprintf("%s %.2f", monedaSym, rt.val))
		}
		yt -= 17
	}

	// Izquierda: Leyendas (SON: ..., detracción)
	yLeft := summaryTop - 10
	if len(meta.Leyendas) > 0 {
		cur.WriteString("0.06 0.09 0.16 rg\n")
		for i, ley := range meta.Leyendas {
			if i >= 3 {
				break
			}
			for _, ln := range wrapText(ley, 56, 2) {
				writePDFText(&cur, "F2", 7, 38, yLeft, ln)
				yLeft -= 10
			}
		}
	}

	// Código QR oficial SUNAT en vectores + Hash DigestValue
	qrString := BuildSunatQRString(meta, c)
	qrSize := 82.0
	qrX := 38.0
	qrY := float64(yLeft) - qrSize - 6
	if qrY < 58 {
		qrY = 58
	}
	drawVectorQR(&cur, qrString, qrX, qrY, qrSize)

	hashTextX := int(qrX+qrSize) + 12
	hashTextY := int(qrY+qrSize) - 14
	cur.WriteString("0.06 0.09 0.16 rg\n")
	writePDFText(&cur, "F2", 8, hashTextX, hashTextY, "REPRESENTACION IMPRESA DEL CPE")
	cur.WriteString("0.30 0.35 0.42 rg\n")
	writePDFText(&cur, "F1", 7, hashTextX, hashTextY-12, "Generado localmente desde el XML UBL 2.1 oficial.")
	if meta.DigestValue != "" {
		writePDFText(&cur, "F2", 7, hashTextX, hashTextY-25, "Resumen / Hash (DigestValue):")
		writePDFText(&cur, "F3", 7, hashTextX, hashTextY-35, truncateStr(meta.DigestValue, 36))
	}
	writePDFText(&cur, "F1", 7, hashTextX, hashTextY-49, "Consulte validez en: www.sunat.gob.pe")
	writePDFText(&cur, "F1", 7, hashTextX, hashTextY-60, fmt.Sprintf("Emitido PDF: %s", time.Now().Format("2006-01-02 15:04")))

	writePageFooter(&cur, len(pages)+1)
	pages = append(pages, cur.Bytes())

	return assembleMultiPagePDF(pages)
}

func writePageFooter(buf *bytes.Buffer, pageNum int) {
	buf.WriteString("0.82 0.86 0.92 RG 0.5 w\n")
	buf.WriteString("36 42 m 559 42 l S\n")
	buf.WriteString("0.35 0.45 0.55 rg\n")
	writePDFText(buf, "F1", 7, 38, 30, "ATLOB Sync · SUNAT (ATLOB Consultores)  |  Representacion impresa generada desde XML UBL 2.1 oficial")
	writePDFText(buf, "F1", 7, 514, 30, fmt.Sprintf("Pagina %d", pageNum))
}

func drawVectorQR(buf *bytes.Buffer, content string, x, y, boxSize float64) {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return
	}
	qr.DisableBorder = false
	bm := qr.Bitmap()
	n := len(bm)
	if n == 0 {
		return
	}

	// Marco blanco con borde sutil
	buf.WriteString("1 1 1 rg\n")
	buf.WriteString(fmt.Sprintf("%.2f %.2f %.2f %.2f re f\n", x, y, boxSize, boxSize))
	buf.WriteString("0.80 0.85 0.92 RG 0.6 w\n")
	buf.WriteString(fmt.Sprintf("%.2f %.2f %.2f %.2f re S\n", x, y, boxSize, boxSize))

	mod := boxSize / float64(n)
	buf.WriteString("0 0 0 rg\n")
	for row := 0; row < n; row++ {
		for col := 0; col < n; col++ {
			if bm[row][col] {
				rx := x + float64(col)*mod
				// PDF tiene origen abajo-izquierda; invertir fila
				ry := y + float64(n-1-row)*mod
				buf.WriteString(fmt.Sprintf("%.2f %.2f %.2f %.2f re f\n", rx, ry, mod+0.05, mod+0.05))
			}
		}
	}
}

// assembleMultiPagePDF ensambla un documento PDF 1.4 válido con 1 o más páginas y tabla xref exacta
func assembleMultiPagePDF(pageStreams [][]byte) []byte {
	if len(pageStreams) == 0 {
		pageStreams = [][]byte{[]byte("")}
	}

	// Objetos fijos:
	// 1 = Catalog
	// 2 = Pages
	// 3 = Font F1 (Helvetica)
	// 4 = Font F2 (Helvetica-Bold)
	// 5 = Font F3 (Courier)
	// Luego por cada página i (0..N-1):
	//   pageObjID = 6 + i*2
	//   contentObjID = 7 + i*2
	totalObjs := 5 + len(pageStreams)*2
	offsets := make([]int, totalObjs+1)

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")

	offsets[1] = pdf.Len()
	pdf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	var kids []string
	for i := range pageStreams {
		kids = append(kids, fmt.Sprintf("%d 0 R", 6+i*2))
	}
	offsets[2] = pdf.Len()
	pdf.WriteString(fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", strings.Join(kids, " "), len(pageStreams)))

	offsets[3] = pdf.Len()
	pdf.WriteString("3 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>\nendobj\n")

	offsets[4] = pdf.Len()
	pdf.WriteString("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>\nendobj\n")

	offsets[5] = pdf.Len()
	pdf.WriteString("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>\nendobj\n")

	for i, content := range pageStreams {
		pageID := 6 + i*2
		contentID := 7 + i*2

		offsets[pageID] = pdf.Len()
		pdf.WriteString(fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R >> >> /Contents %d 0 R >>\nendobj\n", pageID, contentID))

		offsets[contentID] = pdf.Len()
		pdf.WriteString(fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n", contentID, len(content)))
		pdf.Write(content)
		pdf.WriteString("\nendstream\nendobj\n")
	}

	xrefOffset := pdf.Len()
	pdf.WriteString(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", totalObjs+1))
	for i := 1; i <= totalObjs; i++ {
		pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	pdf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", totalObjs+1, xrefOffset))

	return pdf.Bytes()
}

func writePDFText(buf *bytes.Buffer, font string, size int, x, y int, text string) {
	buf.WriteString(fmt.Sprintf("BT /%s %d Tf %d %d Td (%s) Tj ET\n", font, size, x, y, pdfEscapeWinAnsi(text)))
}

func pdfEscapeWinAnsi(s string) string {
	var out bytes.Buffer
	for _, r := range s {
		var b byte
		switch {
		case r == '\\' || r == '(' || r == ')':
			out.WriteByte('\\')
			out.WriteByte(byte(r))
			continue
		case r >= 32 && r <= 126:
			b = byte(r)
		case r >= 160 && r <= 255:
			// Latin-1 / WinAnsi coincide para á é í ó ú ñ Á É Í Ó Ú Ñ ° etc.
			b = byte(r)
		default:
			b = '?'
		}
		out.WriteByte(b)
	}
	return out.String()
}

func wrapText(s string, maxChars, maxLines int) []string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if s == "" {
		return []string{"-"}
	}
	words := strings.Split(s, " ")
	var lines []string
	cur := ""
	for _, w := range words {
		if len([]rune(w)) > maxChars {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			runes := []rune(w)
			for len(runes) > maxChars {
				lines = append(lines, string(runes[:maxChars]))
				runes = runes[maxChars:]
			}
			cur = string(runes)
			continue
		}
		candidate := w
		if cur != "" {
			candidate = cur + " " + w
		}
		if len([]rune(candidate)) <= maxChars {
			cur = candidate
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = truncateStr(lines[maxLines-1], maxChars)
	}
	return lines
}

func formatCantidad(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func monedaSimbolo(m string) string {
	switch strings.ToUpper(strings.TrimSpace(m)) {
	case "USD":
		return "US$"
	case "EUR":
		return "EUR"
	default:
		return "S/"
	}
}

func monedaNombre(m string) string {
	switch strings.ToUpper(strings.TrimSpace(m)) {
	case "USD":
		return "DOLARES (USD)"
	case "EUR":
		return "EUROS (EUR)"
	default:
		return "SOLES (PEN)"
	}
}

func tipoDocClienteNombre(code string) string {
	switch strings.TrimSpace(code) {
	case "6":
		return "RUC"
	case "1":
		return "DNI"
	case "4":
		return "C.E."
	case "7":
		return "PASAPORTE"
	default:
		return "DOC"
	}
}

func tipoCPNombre(code string) string {
	switch padTipoCP(code) {
	case "01":
		return "FACTURA ELECTRONICA (01)"
	case "03":
		return "BOLETA DE VENTA ELECTRONICA (03)"
	case "07":
		return "NOTA DE CREDITO ELECTRONICA (07)"
	case "08":
		return "NOTA DE DEBITO ELECTRONICA (08)"
	case "09":
		return "GUIA DE REMISION REMITENTE (09)"
	case "14":
		return "RECIBO SERVICIOS PUBLICOS (14)"
	default:
		return fmt.Sprintf("COMPROBANTE TIPO %s", code)
	}
}

func tipoCPNombreCorto(code string) string {
	switch padTipoCP(code) {
	case "01":
		return "FACTURA ELECTRONICA"
	case "03":
		return "BOLETA DE VENTA ELECTRONICA"
	case "07":
		return "NOTA DE CREDITO ELECTRONICA"
	case "08":
		return "NOTA DE DEBITO ELECTRONICA"
	case "09":
		return "GUIA DE REMISION ELECTRONICA"
	case "14":
		return "RECIBO DE SERVICIOS PUBLICOS"
	default:
		return fmt.Sprintf("COMPROBANTE ELECTRONICO (%s)", code)
	}
}

func valueOrDash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return s
}

func truncateStr(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}
