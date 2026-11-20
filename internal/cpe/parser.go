package cpe

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

var (
	reFileStandard = regexp.MustCompile(`(?i)^(?:R-)?(\d{11})-(\d{2})-([A-Z0-9]{1,8})-(\d{1,12})\.[a-z0-9]+$`)
	reDigitsOnly   = regexp.MustCompile(`^\d+$`)
)

// isZipBytes detecta ZIP estándar (PK\x03\x04) y el formato ZIP segmentado que entrega SUNAT SIRE (PK\x07\x08PK\x03\x04)
func isZipBytes(raw []byte) bool {
	return bytes.HasPrefix(raw, []byte("PK\x03\x04")) || bytes.HasPrefix(raw, []byte("PK\x07\x08PK\x03\x04"))
}

// ParseSIREProposalArchive extrae comprobantes individuales de un archivo ZIP, TXT o CSV descargado de SUNAT SIRE
func ParseSIREProposalArchive(filePath, empresaRUC, origen, periodo string) ([]ComprobanteCPE, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el archivo '%s': %w", filePath, err)
	}

	var all []ComprobanteCPE

	// Si es ZIP (estándar o encabezado extendido SUNAT PK\x07\x08PK\x03\x04)
	if isZipBytes(raw) {
		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return nil, fmt.Errorf("archivo ZIP inválido '%s': %w", filePath, err)
		}
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(f.Name))
			if ext != ".txt" && ext != ".csv" {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(rc)
			_ = rc.Close()

			parsed := ParseSIRETextContent(data, empresaRUC, origen, periodo)
			all = append(all, parsed...)
		}
		return all, nil
	}

	// Si es TXT o CSV directo
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".txt" || ext == ".csv" || len(raw) > 0 {
		all = ParseSIRETextContent(raw, empresaRUC, origen, periodo)
	}
	return all, nil
}

// ParseSIRETextContent parsea líneas TXT (separadas por '|') o CSV de propuestas RVIE/RCE de SUNAT
func ParseSIRETextContent(content []byte, empresaRUC, origen, periodo string) []ComprobanteCPE {
	// Quitar BOM UTF-8 si existe
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	origenUpper := strings.ToUpper(strings.TrimSpace(origen))
	if origenUpper != "COMPRAS" {
		origenUpper = "VENTAS"
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	buf := make([]byte, 0, 128*1024)
	scanner.Buffer(buf, 2*1024*1024)

	var results []ComprobanteCPE
	var headerMap map[string]int
	firstLine := true
	usePipe := false

	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\uFEFF"))
		if line == "" {
			continue
		}

		if firstLine {
			usePipe = strings.Count(line, "|") >= 5
		}

		var cols []string
		if usePipe {
			cols = strings.Split(line, "|")
		} else {
			r := csv.NewReader(strings.NewReader(line))
			r.FieldsPerRecord = -1
			r.LazyQuotes = true
			if rec, err := r.Read(); err == nil {
				cols = rec
			} else {
				cols = strings.Split(line, ",")
			}
		}

		for i := range cols {
			cols[i] = strings.TrimSpace(cols[i])
		}

		if firstLine {
			firstLine = false
			if isHeaderRow(cols) {
				headerMap = buildHeaderMap(cols)
				continue
			}
		}

		cpe, ok := parseSIRERow(cols, headerMap, empresaRUC, origenUpper, periodo)
		if ok {
			results = append(results, cpe)
		}
	}

	return results
}

func isHeaderRow(cols []string) bool {
	if len(cols) < 5 {
		return false
	}
	joined := strings.ToLower(strings.Join(cols[:minInt(len(cols), 12)], " "))
	return strings.Contains(joined, "periodo") ||
		strings.Contains(joined, "tipo cp") ||
		strings.Contains(joined, "serie del cdp") ||
		strings.Contains(joined, "razon social") ||
		strings.Contains(joined, "razón social") ||
		strings.Contains(joined, "car sunat")
}

func buildHeaderMap(cols []string) map[string]int {
	m := make(map[string]int)
	setIfMissing := func(key string, idx int) {
		if _, exists := m[key]; !exists {
			m[key] = idx
		}
	}

	for i, raw := range cols {
		h := normalizeHeader(raw)
		if h == "" {
			continue
		}
		// Nunca mapear columnas de documento modificado (ej. "Fecha Emisión Doc Modificado", "Tipo CP Modificado", etc.)
		if strings.Contains(h, "modificado") {
			continue
		}

		if strings.Contains(h, "periodo") {
			setIfMissing("periodo", i)
		} else if strings.Contains(h, "car sunat") || h == "car" {
			setIfMissing("car", i)
		} else if strings.Contains(h, "fecha de emision") || strings.Contains(h, "fecha emision") {
			setIfMissing("fecha", i)
		} else if strings.Contains(h, "tipo cp") || strings.Contains(h, "tipo de comprobante") || h == "tipo cp doc" || h == "tipo_cp" {
			setIfMissing("tipo", i)
		} else if strings.Contains(h, "serie") {
			setIfMissing("serie", i)
		} else if strings.Contains(h, "nro cp") || strings.Contains(h, "nro comprobante") || strings.Contains(h, "numero") {
			setIfMissing("numero", i)
		} else if strings.Contains(h, "nro doc identidad") || strings.Contains(h, "numero documento") || strings.Contains(h, "ruc proveedor") || strings.Contains(h, "ruc cliente") {
			setIfMissing("ruc_cp", i)
		} else if i > 2 && (strings.Contains(h, "apellidos") || strings.Contains(h, "razon social")) {
			// i > 2 evita tomar la col 1 ("Apellidos y Nombres o Razón social" de la propia empresa en RCE Compras)
			setIfMissing("razon_cp", i)
		} else if strings.Contains(h, "bi gravad") || strings.Contains(h, "base imponible") {
			setIfMissing("base", i)
		} else if strings.Contains(h, "igv") {
			setIfMissing("igv", i)
		} else if strings.Contains(h, "total cp") || strings.Contains(h, "importe total") || strings.Contains(h, "monto total") {
			setIfMissing("total", i)
		} else if strings.Contains(h, "moneda") {
			setIfMissing("moneda", i)
		} else if strings.Contains(h, "est comp") || h == "estado" {
			setIfMissing("estado", i)
		}
	}
	return m
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n",
		".", "", "/", " ", "-", " ", "_", " ",
	)
	return strings.Join(strings.Fields(repl.Replace(s)), " ")
}

func parseSIRERow(cols []string, hm map[string]int, empresaRUC, origen, defaultPeriodo string) (ComprobanteCPE, bool) {
	getCol := func(idx int) string {
		if idx >= 0 && idx < len(cols) {
			return strings.TrimSpace(cols[idx])
		}
		return ""
	}
	getByName := func(key string, fallback int) string {
		if hm != nil {
			if idx, ok := hm[key]; ok {
				return getCol(idx)
			}
		}
		return getCol(fallback)
	}

	var tipo, serie, numero, fecha, periodo, car, rucCP, razonCP, moneda, estadoRaw string
	var baseVal, igvVal, totalVal float64

	// Caso 1: Formato simplificado de 8 columnas (ej. RUC|TIPO|SERIE|NUMERO|FECHA|BASE|IGV|TOTAL)
	if hm == nil && len(cols) >= 8 && len(cols) <= 10 && len(cols[0]) == 11 && len(cols[1]) == 2 {
		tipo = cols[1]
		serie = cols[2]
		numero = cols[3]
		fecha = cols[4]
		baseVal = parseFloat(cols[5])
		igvVal = parseFloat(cols[6])
		totalVal = parseFloat(cols[7])
		periodo = defaultPeriodo
		moneda = "PEN"
	} else if origen == "COMPRAS" {
		// Formato oficial SUNAT Anexo 8 (RCE)
		periodo = getByName("periodo", 2)
		car = getByName("car", 3)
		fecha = getByName("fecha", 4)
		tipo = getByName("tipo", 6)
		serie = getByName("serie", 7)
		numero = getByName("numero", 9)
		rucCP = getByName("ruc_cp", 12)
		razonCP = getByName("razon_cp", 13)
		baseVal = parseFloat(getByName("base", 14))
		igvVal = parseFloat(getByName("igv", 15))
		totalVal = parseFloat(getByName("total", 24))
		moneda = getByName("moneda", 25)
		estadoRaw = getByName("estado", 39)
	} else {
		// Formato oficial SUNAT Anexo 2 (RVIE Ventas)
		periodo = getByName("periodo", 2)
		car = getByName("car", 3)
		fecha = getByName("fecha", 4)
		tipo = getByName("tipo", 6)
		serie = getByName("serie", 7)
		numero = getByName("numero", 8)
		rucCP = getByName("ruc_cp", 11)
		razonCP = getByName("razon_cp", 12)
		baseVal = parseFloat(getByName("base", 14))
		igvVal = parseFloat(getByName("igv", 16))
		totalVal = parseFloat(getByName("total", 25))
		moneda = getByName("moneda", 26)
		estadoRaw = getByName("estado", 34)
	}

	if periodo == "" {
		periodo = defaultPeriodo
	}
	if moneda == "" {
		moneda = "PEN"
	}

	estado := "EN_SIRE"
	switch strings.TrimSpace(estadoRaw) {
	case "1":
		estado = "ACTIVO"
	case "2":
		estado = "ANULADO"
	}

	tipo = padTipoCP(tipo)
	serie = strings.ToUpper(strings.TrimSpace(serie))
	numero = strings.TrimSpace(numero)

	if tipo == "" || serie == "" || numero == "" {
		return ComprobanteCPE{}, false
	}

	return ComprobanteCPE{
		EmpresaRUC:       empresaRUC,
		Origen:           origen,
		Periodo:          periodo,
		TipoCP:           tipo,
		Serie:            serie,
		Numero:           numero,
		FechaEmision:     fecha,
		RucContraparte:   rucCP,
		RazonContraparte: razonCP,
		BaseImponible:    baseVal,
		IGV:              igvVal,
		MontoTotal:       totalVal,
		Moneda:           moneda,
		EstadoSUNAT:      estado,
		CarSUNAT:         car,
	}, true
}

// UBLItem representa una línea / ítem de un comprobante electrónico UBL 2.1
type UBLItem struct {
	ID             string  `json:"id"`
	Cantidad       float64 `json:"cantidad"`
	Unidad         string  `json:"unidad"`
	Codigo         string  `json:"codigo,omitempty"`
	Descripcion    string  `json:"descripcion"`
	ValorUnitario  float64 `json:"valor_unitario"`
	PrecioUnitario float64 `json:"precio_unitario"`
	IGV            float64 `json:"igv"`
	ValorVenta     float64 `json:"valor_venta"`
}

// UBLCuota representa una cuota de pago al crédito en un XML UBL 2.1 SUNAT
type UBLCuota struct {
	ID               string  `json:"id"`
	Monto            float64 `json:"monto"`
	FechaVencimiento string  `json:"fecha_vencimiento"`
}

// UBLMetadata metadatos completos extraídos de un XML UBL 2.1 de SUNAT
type UBLMetadata struct {
	RucEmisor        string
	RazonEmisor      string
	DireccionEmisor  string
	TipoDocCliente   string
	RucCliente       string
	RazonCliente     string
	DireccionCliente string
	TipoCP           string
	Serie            string
	Numero           string
	FechaEmision     string
	HoraEmision      string
	FechaVencimiento string
	Moneda           string
	MontoTotal       float64
	IGV              float64
	BaseImponible    float64
	OpGravada        float64
	OpExonerada      float64
	OpInafecta       float64
	OpGratuita       float64
	FormaPago        string
	Cuotas           []UBLCuota
	Leyendas         []string
	DigestValue      string
	DocModificadoRef string
	MotivoNota       string
	Items            []UBLItem
	EsCDR            bool
	CdrResponseCode  string
}

func xmlCharsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "iso-8859-1", "latin1", "latin-1":
		return charmap.ISO8859_1.NewDecoder().Reader(input), nil
	case "windows-1252", "cp1252":
		return charmap.Windows1252.NewDecoder().Reader(input), nil
	default:
		return input, nil
	}
}

// ParseUBLXML extrae datos completos de un archivo XML UBL 2.1 (Factura, Boleta, Nota o ApplicationResponse CDR)
func ParseUBLXML(xmlBytes []byte) (*UBLMetadata, error) {
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	dec.CharsetReader = xmlCharsetReader
	meta := &UBLMetadata{Moneda: "PEN"}

	var path []string
	var rootLocal string

	// Estado de ítem en curso
	var curItem *UBLItem
	var curPriceType string

	// Estado de PaymentTerms en curso
	var inPaymentTerms bool
	var ptID, ptMeansID, ptDueDate string
	var ptAmount float64

	// Estado de TaxSubtotal en raíz (para separar gravada, exonerada, inafecta, gratuita)
	var inRootTaxSubtotal bool
	var curTaxableAmount float64
	var curTaxSchemeID string

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("XML inválido: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if rootLocal == "" {
				rootLocal = t.Name.Local
				switch rootLocal {
				case "ApplicationResponse":
					meta.EsCDR = true
				case "CreditNote":
					meta.TipoCP = "07"
				case "DebitNote":
					meta.TipoCP = "08"
				}
			}
			path = append(path, t.Name.Local)
			joined := strings.Join(path, "/")

			// Capturar atributo schemeID del cliente
			if strings.Contains(joined, "AccountingCustomerParty") && t.Name.Local == "ID" && meta.TipoDocCliente == "" {
				for _, attr := range t.Attr {
					if attr.Name.Local == "schemeID" && strings.TrimSpace(attr.Value) != "" {
						meta.TipoDocCliente = strings.TrimSpace(attr.Value)
					}
				}
			}

			// Inicio de línea / ítem (Factura, NC, ND)
			if len(path) == 2 && (t.Name.Local == "InvoiceLine" || t.Name.Local == "CreditNoteLine" || t.Name.Local == "DebitNoteLine") {
				curItem = &UBLItem{Unidad: "NIU"}
				curPriceType = ""
			}

			// Atributo unitCode en InvoicedQuantity / CreditedQuantity / DebitedQuantity
			if curItem != nil && (t.Name.Local == "InvoicedQuantity" || t.Name.Local == "CreditedQuantity" || t.Name.Local == "DebitedQuantity") {
				for _, attr := range t.Attr {
					if attr.Name.Local == "unitCode" && strings.TrimSpace(attr.Value) != "" {
						curItem.Unidad = strings.TrimSpace(attr.Value)
					}
				}
			}

			// Inicio de PaymentTerms (nivel raíz)
			if len(path) == 2 && t.Name.Local == "PaymentTerms" {
				inPaymentTerms = true
				ptID, ptMeansID, ptDueDate = "", "", ""
				ptAmount = 0
			}

			// Inicio de TaxSubtotal a nivel de documento raíz (profundidad 3: Root/TaxTotal/TaxSubtotal)
			if len(path) == 3 && t.Name.Local == "TaxSubtotal" && path[1] == "TaxTotal" {
				inRootTaxSubtotal = true
				curTaxableAmount = 0
				curTaxSchemeID = ""
			}

		case xml.EndElement:
			if len(path) == 2 && curItem != nil && (t.Name.Local == "InvoiceLine" || t.Name.Local == "CreditNoteLine" || t.Name.Local == "DebitNoteLine") {
				if curItem.ValorVenta == 0 && curItem.Cantidad > 0 && curItem.ValorUnitario > 0 {
					curItem.ValorVenta = curItem.Cantidad * curItem.ValorUnitario
				}
				if curItem.PrecioUnitario == 0 && curItem.ValorUnitario > 0 {
					curItem.PrecioUnitario = curItem.ValorUnitario
				}
				meta.Items = append(meta.Items, *curItem)
				curItem = nil
				curPriceType = ""
			}

			if len(path) == 2 && inPaymentTerms && t.Name.Local == "PaymentTerms" {
				if strings.EqualFold(ptID, "FormaPago") {
					if strings.EqualFold(ptMeansID, "Contado") || strings.EqualFold(ptMeansID, "Credito") {
						meta.FormaPago = ptMeansID
					} else if strings.HasPrefix(strings.ToLower(ptMeansID), "cuota") {
						meta.Cuotas = append(meta.Cuotas, UBLCuota{
							ID:               ptMeansID,
							Monto:            ptAmount,
							FechaVencimiento: ptDueDate,
						})
					}
				}
				inPaymentTerms = false
			}

			if len(path) == 3 && inRootTaxSubtotal && t.Name.Local == "TaxSubtotal" {
				switch curTaxSchemeID {
				case "1000":
					meta.OpGravada += curTaxableAmount
				case "9997":
					meta.OpExonerada += curTaxableAmount
				case "9998":
					meta.OpInafecta += curTaxableAmount
				case "9996":
					meta.OpGratuita += curTaxableAmount
				}
				inRootTaxSubtotal = false
			}

			if len(path) > 0 {
				path = path[:len(path)-1]
			}

		case xml.CharData:
			val := strings.TrimSpace(string(t))
			if val == "" || len(path) == 0 {
				continue
			}
			curr := path[len(path)-1]
			joined := strings.Join(path, "/")

			if meta.EsCDR {
				if curr == "ResponseCode" && meta.CdrResponseCode == "" {
					meta.CdrResponseCode = val
				}
				if curr == "ReferenceID" && strings.Contains(val, "-") && meta.Serie == "" {
					parts := strings.SplitN(val, "-", 2)
					meta.Serie = strings.ToUpper(strings.TrimSpace(parts[0]))
					meta.Numero = strings.TrimSpace(parts[1])
				}
				continue
			}

			// Hash de firma digital (ds:DigestValue)
			if curr == "DigestValue" && meta.DigestValue == "" {
				meta.DigestValue = val
			}

			// Documento principal (profundidad 2: Invoice/ID, Invoice/IssueDate, etc.)
			if len(path) == 2 {
				switch curr {
				case "ID":
					if strings.Contains(val, "-") && meta.Serie == "" {
						parts := strings.SplitN(val, "-", 2)
						meta.Serie = strings.ToUpper(strings.TrimSpace(parts[0]))
						meta.Numero = strings.TrimSpace(parts[1])
					}
				case "IssueDate":
					if meta.FechaEmision == "" {
						meta.FechaEmision = val
					}
				case "IssueTime":
					if meta.HoraEmision == "" {
						meta.HoraEmision = val
					}
				case "DueDate":
					if meta.FechaVencimiento == "" {
						meta.FechaVencimiento = val
					}
				case "InvoiceTypeCode":
					meta.TipoCP = padTipoCP(val)
				case "DocumentCurrencyCode":
					meta.Moneda = val
				case "Note":
					meta.Leyendas = append(meta.Leyendas, val)
				}
			}

			// Notas de Crédito / Débito: referencia y motivo
			if strings.Contains(joined, "BillingReference/InvoiceDocumentReference/ID") && meta.DocModificadoRef == "" {
				meta.DocModificadoRef = val
			}
			if strings.Contains(joined, "DiscrepancyResponse/Description") && meta.MotivoNota == "" {
				meta.MotivoNota = val
			}

			// PaymentTerms (Contado / Crédito / Cuotas)
			if inPaymentTerms && len(path) == 3 {
				switch curr {
				case "ID":
					ptID = val
				case "PaymentMeansID":
					ptMeansID = val
				case "Amount":
					ptAmount = parseFloat(val)
				case "PaymentDueDate":
					ptDueDate = val
				}
			}

			// TaxSubtotal raíz
			if inRootTaxSubtotal {
				if curr == "TaxableAmount" {
					curTaxableAmount = parseFloat(val)
				} else if strings.HasSuffix(joined, "TaxScheme/ID") {
					curTaxSchemeID = val
				}
			}

			// Ítem actual (InvoiceLine / CreditNoteLine / DebitNoteLine)
			if curItem != nil {
				switch {
				case len(path) == 3 && curr == "ID" && curItem.ID == "":
					curItem.ID = val
				case len(path) == 3 && (curr == "InvoicedQuantity" || curr == "CreditedQuantity" || curr == "DebitedQuantity"):
					curItem.Cantidad = parseFloat(val)
				case len(path) == 3 && curr == "LineExtensionAmount":
					curItem.ValorVenta = parseFloat(val)
				case strings.HasSuffix(joined, "Item/Description"):
					if curItem.Descripcion != "" {
						curItem.Descripcion += " " + val
					} else {
						curItem.Descripcion = val
					}
				case strings.HasSuffix(joined, "SellersItemIdentification/ID") && curItem.Codigo == "":
					curItem.Codigo = val
				case strings.HasSuffix(joined, "Price/PriceAmount") && !strings.Contains(joined, "PricingReference"):
					curItem.ValorUnitario = parseFloat(val)
				case strings.HasSuffix(joined, "AlternativeConditionPrice/PriceTypeCode"):
					curPriceType = val
				case strings.HasSuffix(joined, "AlternativeConditionPrice/PriceAmount"):
					if curPriceType == "" || curPriceType == "01" {
						curItem.PrecioUnitario = parseFloat(val)
					}
				case strings.HasSuffix(joined, "TaxTotal/TaxAmount") && len(path) == 4:
					curItem.IGV = parseFloat(val)
				}
				continue
			}

			// Emisor (AccountingSupplierParty)
			if strings.Contains(joined, "AccountingSupplierParty") {
				if curr == "ID" && len(val) == 11 && reDigitsOnly.MatchString(val) && meta.RucEmisor == "" {
					meta.RucEmisor = val
				} else if curr == "RegistrationName" && meta.RazonEmisor == "" {
					meta.RazonEmisor = val
				} else if (curr == "Line" || curr == "StreetName") && meta.DireccionEmisor == "" {
					meta.DireccionEmisor = val
				}
			}

			// Cliente (AccountingCustomerParty)
			if strings.Contains(joined, "AccountingCustomerParty") {
				if curr == "ID" && reDigitsOnly.MatchString(val) && meta.RucCliente == "" {
					meta.RucCliente = val
				} else if curr == "RegistrationName" && meta.RazonCliente == "" {
					meta.RazonCliente = val
				} else if (curr == "Line" || curr == "StreetName") && meta.DireccionCliente == "" {
					meta.DireccionCliente = val
				}
			}

			// Totales monetarios
			if strings.Contains(joined, "LegalMonetaryTotal") {
				if curr == "PayableAmount" && meta.MontoTotal == 0 {
					meta.MontoTotal = parseFloat(val)
				} else if curr == "LineExtensionAmount" && meta.BaseImponible == 0 {
					meta.BaseImponible = parseFloat(val)
				}
			}
			if strings.HasSuffix(joined, "TaxTotal/TaxAmount") && len(path) == 3 && meta.IGV == 0 {
				meta.IGV = parseFloat(val)
			}
		}
	}

	if meta.TipoDocCliente == "" && meta.RucCliente != "" {
		switch len(meta.RucCliente) {
		case 11:
			meta.TipoDocCliente = "6"
		case 8:
			meta.TipoDocCliente = "1"
		default:
			meta.TipoDocCliente = "0"
		}
	}

	if meta.BaseImponible == 0 && meta.OpGravada > 0 {
		meta.BaseImponible = meta.OpGravada
	}
	if meta.BaseImponible == 0 && meta.MontoTotal > 0 && meta.IGV >= 0 && meta.MontoTotal >= meta.IGV {
		meta.BaseImponible = meta.MontoTotal - meta.IGV
	}

	return meta, nil
}

// BuildSunatQRString construye la cadena oficial del Código QR según normativa SUNAT:
// RUC_EMISOR|TIPO_DOC|SERIE|NUMERO|IGV|TOTAL|FECHA_YYYY-MM-DD|TIPO_DOC_CLI|NUM_DOC_CLI|HASH|
func BuildSunatQRString(meta *UBLMetadata, c *ComprobanteCPE) string {
	var rucEmisor, tipoCP, serie, numero, fecha, tipoDocCli, numDocCli, hash string
	var igv, total float64

	if c != nil {
		tipoCP = padTipoCP(c.TipoCP)
		serie = strings.ToUpper(strings.TrimSpace(c.Serie))
		numero = strings.TrimSpace(c.Numero)
		fecha = normalizeFechaISO(c.FechaEmision)
		igv = c.IGV
		total = c.MontoTotal
		if strings.ToUpper(c.Origen) == "COMPRAS" {
			rucEmisor = c.RucContraparte
			numDocCli = c.EmpresaRUC
			tipoDocCli = "6"
		} else {
			rucEmisor = c.EmpresaRUC
			numDocCli = c.RucContraparte
			if len(numDocCli) == 11 {
				tipoDocCli = "6"
			} else if len(numDocCli) == 8 {
				tipoDocCli = "1"
			} else {
				tipoDocCli = "0"
			}
		}
	}

	if meta != nil {
		if meta.RucEmisor != "" {
			rucEmisor = meta.RucEmisor
		}
		if meta.TipoCP != "" {
			tipoCP = padTipoCP(meta.TipoCP)
		}
		if meta.Serie != "" {
			serie = meta.Serie
		}
		if meta.Numero != "" {
			numero = meta.Numero
		}
		if meta.FechaEmision != "" {
			fecha = normalizeFechaISO(meta.FechaEmision)
		}
		if meta.TipoDocCliente != "" {
			tipoDocCli = meta.TipoDocCliente
		}
		if meta.RucCliente != "" {
			numDocCli = meta.RucCliente
		}
		if meta.IGV != 0 {
			igv = meta.IGV
		}
		if meta.MontoTotal != 0 {
			total = meta.MontoTotal
		}
		hash = meta.DigestValue
	}

	return fmt.Sprintf("%s|%s|%s|%s|%.2f|%.2f|%s|%s|%s|%s|",
		rucEmisor, tipoCP, serie, numero, igv, total, fecha, tipoDocCli, numDocCli, hash)
}

func normalizeFechaISO(f string) string {
	f = strings.TrimSpace(f)
	// Convertir DD/MM/YYYY -> YYYY-MM-DD
	if len(f) == 10 && f[2] == '/' && f[5] == '/' {
		return f[6:10] + "-" + f[3:5] + "-" + f[0:2]
	}
	return f
}

// ParseFilenameCPE intenta extraer RUC, TipoCP, Serie y Número del formato estándar SUNAT (ej. 20000000001-01-F001-00000123.pdf)
func ParseFilenameCPE(filename string) (ruc, tipoCP, serie, numero string, isCDR bool, ok bool) {
	base := filepath.Base(filename)
	m := reFileStandard.FindStringSubmatch(base)
	if len(m) != 5 {
		return "", "", "", "", false, false
	}
	isCDR = strings.HasPrefix(strings.ToUpper(base), "R-")
	return m[1], padTipoCP(m[2]), strings.ToUpper(m[3]), m[4], isCDR, true
}

func padTipoCP(t string) string {
	t = strings.TrimSpace(t)
	if len(t) == 1 && reDigitsOnly.MatchString(t) {
		return "0" + t
	}
	return t
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
