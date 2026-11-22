package rhe

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// OfficialRHETXTHeaders son las 16 columnas exactas que entrega SUNAT SOL en descargaConsultaReceptor (delimitado por '|').
var OfficialRHETXTHeaders = []string{
	"Fecha de Emisión",
	"Tipo Doc. Emitido",
	"Nro. Doc. Emitido",
	"Estado Doc. Emitido",
	"Tipo de Doc. Emisor",
	"Nro. Doc. Emisor",
	"Apellidos y Nombres, Denominación o Razón Social del Emisor",
	"Tipo de Renta",
	"Gratuito",
	"Descripción",
	"Observación",
	"Moneda de Operación",
	"Renta Bruta",
	"Impuesto a la Renta",
	"Renta Neta",
	"Monto Neto Pendiente de Pago",
}

var rePeriodoYYYYMM = regexp.MustCompile(`^(20\d{2})(0[1-9]|1[0-2])$`)

// ValidatePeriodo verifica que el periodo tenga formato estricto YYYYMM.
func ValidatePeriodo(periodo string) (int, int, error) {
	p := strings.TrimSpace(periodo)
	m := rePeriodoYYYYMM.FindStringSubmatch(p)
	if len(m) != 3 {
		return 0, 0, fmt.Errorf("periodo inválido '%s' (se esperaba formato YYYYMM, ej. 202610)", periodo)
	}
	year, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	return year, month, nil
}

// ParseReceptorTXT convierte el reporte oficial TXT de SUNAT (descargaConsultaReceptor, separado por '|')
// en registros estructurados de ReciboHonorario. Soporta UTF-8 (con o sin BOM) y Windows-1252/ISO-8859-1.
func ParseReceptorTXT(raw []byte, empresaRUC, periodo string) ([]ReciboHonorario, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return []ReciboHonorario{}, nil
	}

	text := decodeRHETXTBytes(raw)
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var colMap map[string]int
	var recibos []ReciboHonorario
	lineNum := 0

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lineNum++
		cols := strings.Split(line, "|")

		// Detectar si la primera línea es la cabecera oficial de SUNAT
		if lineNum == 1 && looksLikeRHEHeader(cols) {
			colMap = mapRHEHeaderColumns(cols)
			continue
		}

		// Si el archivo no tenía cabecera estándar, usar el mapa posicional oficial de SUNAT (16 columnas)
		if colMap == nil {
			colMap = defaultRHEColumnMap()
		}

		get := func(key string) string {
			idx, ok := colMap[key]
			if !ok || idx < 0 || idx >= len(cols) {
				return ""
			}
			return strings.TrimSpace(cols[idx])
		}

		fecha := get("fecha_emision")
		tipoDoc := strings.ToUpper(get("tipo_doc"))
		nroDoc := strings.ToUpper(get("nro_doc"))
		nroEmisor := get("nro_doc_emisor")

		// Saltar posibles líneas basura o filas vacías sin identificador mínimo
		if nroDoc == "" && nroEmisor == "" {
			continue
		}
		if tipoDoc == "" {
			tipoDoc = "RH"
		}

		serie, numero := splitSerieNumero(nroDoc)
		estado := normalizeEstado(get("estado"))
		tipoDocEmisor := strings.ToUpper(get("tipo_doc_emisor"))
		if tipoDocEmisor == "" {
			tipoDocEmisor = "RUC"
		}
		moneda := normalizeMoneda(get("moneda"))

		rec := ReciboHonorario{
			EmpresaRUC:     strings.TrimSpace(empresaRUC),
			Periodo:        strings.TrimSpace(periodo),
			FechaEmision:   fecha,
			TipoDoc:        tipoDoc,
			NroDoc:         nroDoc,
			Serie:          serie,
			Numero:         numero,
			Estado:         estado,
			TipoDocEmisor:  tipoDocEmisor,
			NroDocEmisor:   nroEmisor,
			RazonEmisor:    get("razon_emisor"),
			TipoRenta:      strings.ToUpper(get("tipo_renta")),
			Gratuito:       cleanDashOrText(get("gratuito")),
			Descripcion:    cleanDashOrText(get("descripcion")),
			Observacion:    cleanDashOrText(get("observacion")),
			Moneda:         moneda,
			RentaBruta:     parseRHEAmount(get("renta_bruta")),
			ImpuestoRenta:  parseRHEAmount(get("impuesto_renta")),
			RentaNeta:      parseRHEAmount(get("renta_neta")),
			MontoPendiente: parseRHEAmount(get("monto_pendiente")),
			Vigente:        true,
			EsAnulado:      IsAnuladoORevertido(estado),
		}
		recibos = append(recibos, rec)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error leyendo líneas del reporte TXT RHE: %w", err)
	}
	if recibos == nil {
		recibos = []ReciboHonorario{}
	}
	return recibos, nil
}

// ComputeResumen calcula KPIs exactos en centavos enteros sobre los recibos listados,
// excluyendo recibos ANULADOS o REVERTIDOS de los importes totales.
func ComputeResumen(recibos []ReciboHonorario) RHEResumenKPI {
	res := RHEResumenKPI{
		TotalRegistros: len(recibos),
		PorMoneda:      []RHEMonedaResumen{},
	}

	type acc struct {
		cantidad  int
		bruta     int64
		impuesto  int64
		neta      int64
		pendiente int64
	}
	byCurrency := make(map[string]*acc)
	var totalBruta, totalImpuesto, totalNeta, totalPendiente int64

	for _, r := range recibos {
		if r.EsAnulado || IsAnuladoORevertido(r.Estado) {
			res.AnuladosRevertidos++
			continue
		}
		res.Activos++

		sign := int64(1)
		// En Notas de Crédito de 4ta categoría (NC), si SUNAT reporta valores positivos, restan al neto del periodo.
		if strings.EqualFold(strings.TrimSpace(r.TipoDoc), "NC") && r.RentaBruta > 0 {
			sign = -1
		}

		brutaCents := sign * toCents(r.RentaBruta)
		impCents := sign * toCents(r.ImpuestoRenta)
		netaCents := sign * toCents(r.RentaNeta)
		pendCents := sign * toCents(r.MontoPendiente)

		if math.Abs(r.ImpuestoRenta) > 0.0001 {
			res.ConRetencion++
		} else {
			res.SinRetencion++
		}

		totalBruta += brutaCents
		totalImpuesto += impCents
		totalNeta += netaCents
		totalPendiente += pendCents

		mon := normalizeMoneda(r.Moneda)
		cur, ok := byCurrency[mon]
		if !ok {
			cur = &acc{}
			byCurrency[mon] = cur
		}
		cur.cantidad++
		cur.bruta += brutaCents
		cur.impuesto += impCents
		cur.neta += netaCents
		cur.pendiente += pendCents
	}

	res.TotalRentaBruta = fromCents(totalBruta)
	res.TotalImpuestoRenta = fromCents(totalImpuesto)
	res.TotalRentaNeta = fromCents(totalNeta)
	res.TotalMontoPendiente = fromCents(totalPendiente)

	monedas := make([]string, 0, len(byCurrency))
	for m := range byCurrency {
		monedas = append(monedas, m)
	}
	sort.Slice(monedas, func(i, j int) bool {
		if monedas[i] == "SOLES" {
			return true
		}
		if monedas[j] == "SOLES" {
			return false
		}
		return monedas[i] < monedas[j]
	})

	for _, m := range monedas {
		c := byCurrency[m]
		res.PorMoneda = append(res.PorMoneda, RHEMonedaResumen{
			Moneda:         m,
			Cantidad:       c.cantidad,
			RentaBruta:     fromCents(c.bruta),
			ImpuestoRenta:  fromCents(c.impuesto),
			RentaNeta:      fromCents(c.neta),
			MontoPendiente: fromCents(c.pendiente),
		})
	}

	return res
}

func toCents(v float64) int64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return int64(math.Round(v * 100.0))
}

func fromCents(c int64) float64 {
	return float64(c) / 100.0
}

func splitSerieNumero(nroDoc string) (string, string) {
	s := strings.TrimSpace(nroDoc)
	if idx := strings.Index(s, "-"); idx >= 0 {
		return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+1:])
	}
	return "", s
}

func normalizeEstado(estado string) string {
	up := strings.ToUpper(strings.TrimSpace(estado))
	if up == "" {
		return "NO ANULADO"
	}
	return up
}

func normalizeMoneda(moneda string) string {
	up := strings.ToUpper(strings.TrimSpace(moneda))
	switch up {
	case "", "PEN", "S/", "NUEVOS SOLES", "SOL":
		return "SOLES"
	case "USD", "US$", "DÓLARES", "DOLARES AMERICANOS":
		return "DOLARES"
	default:
		return up
	}
}

func cleanDashOrText(s string) string {
	t := strings.TrimSpace(s)
	if t == "-" {
		return ""
	}
	return t
}

func parseRHEAmount(raw string) float64 {
	s := strings.TrimSpace(raw)
	if s == "" || s == "-" {
		return 0
	}
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func decodeRHETXTBytes(raw []byte) string {
	// Quitar UTF-8 BOM si existiera
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(raw) {
		return string(raw)
	}
	// Fallback Windows-1252 / ISO-8859-1 -> UTF-8
	var b strings.Builder
	b.Grow(len(raw) + len(raw)/8)
	for _, c := range raw {
		b.WriteRune(win1252ByteToRune(c))
	}
	return b.String()
}

func win1252ByteToRune(b byte) rune {
	switch b {
	case 0x80:
		return '€'
	case 0x82:
		return '‚'
	case 0x83:
		return 'ƒ'
	case 0x84:
		return '„'
	case 0x85:
		return '…'
	case 0x86:
		return '†'
	case 0x87:
		return '‡'
	case 0x88:
		return 'ˆ'
	case 0x89:
		return '‰'
	case 0x8A:
		return 'Š'
	case 0x8B:
		return '‹'
	case 0x8C:
		return 'Œ'
	case 0x8E:
		return 'Ž'
	case 0x91:
		return '‘'
	case 0x92:
		return '’'
	case 0x93:
		return '“'
	case 0x94:
		return '”'
	case 0x95:
		return '•'
	case 0x96:
		return '–'
	case 0x97:
		return '—'
	case 0x98:
		return '˜'
	case 0x99:
		return '™'
	case 0x9A:
		return 'š'
	case 0x9B:
		return '›'
	case 0x9C:
		return 'œ'
	case 0x9E:
		return 'ž'
	case 0x9F:
		return 'Ÿ'
	default:
		return rune(b)
	}
}

func normalizeHeaderKey(h string) string {
	s := strings.ToLower(strings.TrimSpace(h))
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n",
		".", "", ",", "", ":", "", "-", " ", "_", " ",
	)
	s = replacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func looksLikeRHEHeader(cols []string) bool {
	if len(cols) < 6 {
		return false
	}
	first := normalizeHeaderKey(cols[0])
	return strings.Contains(first, "fecha") && strings.Contains(first, "emision")
}

func defaultRHEColumnMap() map[string]int {
	return map[string]int{
		"fecha_emision":   0,
		"tipo_doc":        1,
		"nro_doc":         2,
		"estado":          3,
		"tipo_doc_emisor": 4,
		"nro_doc_emisor":  5,
		"razon_emisor":    6,
		"tipo_renta":      7,
		"gratuito":        8,
		"descripcion":     9,
		"observacion":     10,
		"moneda":          11,
		"renta_bruta":     12,
		"impuesto_renta":  13,
		"renta_neta":      14,
		"monto_pendiente": 15,
	}
}

func mapRHEHeaderColumns(cols []string) map[string]int {
	m := defaultRHEColumnMap()
	detected := make(map[string]int)
	for i, c := range cols {
		k := normalizeHeaderKey(c)
		switch {
		case strings.Contains(k, "fecha") && strings.Contains(k, "emision"):
			detected["fecha_emision"] = i
		case strings.Contains(k, "tipo") && strings.Contains(k, "emitido"):
			detected["tipo_doc"] = i
		case strings.Contains(k, "nro") && strings.Contains(k, "emitido"):
			detected["nro_doc"] = i
		case strings.Contains(k, "estado"):
			detected["estado"] = i
		case strings.Contains(k, "tipo") && strings.Contains(k, "emisor"):
			detected["tipo_doc_emisor"] = i
		case strings.Contains(k, "nro") && strings.Contains(k, "emisor"):
			detected["nro_doc_emisor"] = i
		case strings.Contains(k, "apellidos") || strings.Contains(k, "razon social"):
			detected["razon_emisor"] = i
		case strings.Contains(k, "tipo") && strings.Contains(k, "renta"):
			detected["tipo_renta"] = i
		case strings.Contains(k, "gratuito"):
			detected["gratuito"] = i
		case strings.Contains(k, "descripcion"):
			detected["descripcion"] = i
		case strings.Contains(k, "observacion"):
			detected["observacion"] = i
		case strings.Contains(k, "moneda"):
			detected["moneda"] = i
		case strings.Contains(k, "renta bruta"):
			detected["renta_bruta"] = i
		case strings.Contains(k, "impuesto") && strings.Contains(k, "renta"):
			detected["impuesto_renta"] = i
		case strings.Contains(k, "renta neta"):
			detected["renta_neta"] = i
		case strings.Contains(k, "pendiente"):
			detected["monto_pendiente"] = i
		}
	}
	for k, v := range detected {
		m[k] = v
	}
	return m
}
