package sire

import (
	"archive/zip"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// Constantes de seguridad, paginación y formato oficial SIRE (RVIE / RCE).
const (
	// ProposalTXTDelimiter es el separador oficial de los archivos planos TXT descargados de SUNAT SIRE.
	ProposalTXTDelimiter = "|"

	// DefaultMaxProposalTXTBytes es el tope por defecto (50 MB) de bytes descomprimidos realmente leídos en memoria.
	DefaultMaxProposalTXTBytes int64 = 50 * 1024 * 1024

	// DefaultProposalPageLimit y MaxProposalPageLimit controlan la paginación segura del endpoint.
	DefaultProposalPageLimit = 50
	MaxProposalPageLimit     = 500
	MaxProposalQueryLen      = 40

	// Parámetros de caché en memoria para análisis TXT (nunca se persiste en disco ni en BD).
	DefaultProposalCacheMaxEntries = 4
	DefaultProposalCacheTTL        = 5 * 60 // segundos (5 minutos)

	// Códigos de error estructurados del analizador (nunca exponen RUC, montos, razones sociales ni tokens).
	ErrCodeZipInesperado      = "ZIP_INESPERADO"
	ErrCodeZipDemasiadoGrande = "ZIP_DEMASIADO_GRANDE"
	ErrCodeFormatoInesperado  = "FORMATO_INESPERADO"
	ErrCodeNoEncontrado       = "DESCARGA_NO_ENCONTRADA"
	ErrCodeNoCompletado       = "DESCARGA_NO_COMPLETADA"
	ErrCodeRutaInvalida       = "RUTA_FUERA_DE_STORAGE"

	// Códigos de tipo de comprobante confirmados contra Tabla 10 SUNAT / SIRE:
	// - Notas de Crédito (RESTAN en totales):
	//     "07" = Nota de Crédito
	//     "87" = Nota de Crédito Especial
	//     "97" = Nota de Crédito - No Domiciliado
	// - Notas de Débito (SUMAN en totales):
	//     "08" = Nota de Débito
	//     "88" = Nota de Débito Especial
	//     "98" = Nota de Débito - No Domiciliado
	//
	// Fórmula explícita de totales (sobre registros activos, estado == "1"):
	//     Total (total_cp) = total_comprobantes + total_nd - total_nc
	// donde total_nc se expone en magnitud positiva para que la fórmula sea exacta al céntimo.
	// Los KPI de Base Imponible e IGV se conservan como los entrega SUNAT; los descuentos RVIE
	// (Dscto BI, Dscto IGV / IPM) se entregan como campos informativos aparte.
	//
	// Reglas estrictas de estado del comprobante (Est. Comp / Est. Comp.):
	// - "1": activo (incluido en totales).
	// - "2": anulado (excluido de totales en Ventas y Compras; devuelto aparte).
	// - Cualquier otro código: estado desconocido (excluido de totales, nunca sumado en silencio;
	//   contado en conteos.estado_desconocido y reportado en advertencias[]).
	TipoCPNotaCredito          = "07"
	TipoCPNotaCreditoEspecial  = "87"
	TipoCPNotaCreditoNoDomicil = "97"
	TipoCPNotaDebito           = "08"
	TipoCPNotaDebitoEspecial   = "88"
	TipoCPNotaDebitoNoDomicil  = "98"

	EstadoCompActivo  = "1"
	EstadoCompAnulado = "2"

	// Marcas normalizadas de estado por fila y valores válidos del filtro opcional ?estado=
	EstadoMarcaActivo      = "activo"
	EstadoMarcaAnulado     = "anulado"
	EstadoMarcaDesconocido = "desconocido"
	EstadoFiltroTodos      = "todos"
)

// Cabeceras oficiales RVIE (Ventas - libro 140000, Anexo 2 / Manual v31 Secc. 5.18): 40 columnas exactas.
var rvieExpectedHeaders = []string{
	"Ruc",
	"Razon Social",
	"Periodo",
	"CAR SUNAT",
	"Fecha de emisión",
	"Fecha Vcto/Pago",
	"Tipo CP/Doc.",
	"Serie del CDP",
	"Nro CP o Doc. Nro Inicial (Rango)",
	"Nro Final (Rango)",
	"Tipo Doc Identidad",
	"Nro Doc Identidad",
	"Apellidos Nombres/ Razón Social",
	"Valor Facturado Exportación",
	"BI Gravada",
	"Dscto BI",
	"IGV / IPM",
	"Dscto IGV / IPM",
	"Mto Exonerado",
	"Mto Inafecto",
	"ISC",
	"BI Grav IVAP",
	"IVAP",
	"ICBPER",
	"Otros Tributos",
	"Total CP",
	"Moneda",
	"Tipo Cambio",
	"Fecha Emisión Doc Modificado",
	"Tipo CP Modificado",
	"Serie CP Modificado",
	"Nro CP Modificado",
	"ID Proyecto Operadores Atribución",
	"Tipo de Nota",
	"Est. Comp",
	"Valor FOB Embarcado",
	"Valor OP Gratuitas",
	"Tipo Operación",
	"DAM / CP",
	"CLU",
}

// Cabeceras núcleo oficiales RCE (Compras - libro 080000, Anexo 8 / Manual v28 Secc. 5.34):
// columnas 00..40 obligatorias (validación estricta por nombre); CLU1..CLU39 (39 columnas) son opcionales con advertencia si cambia el conteo.
var rceBaseHeaders = []string{
	"RUC",
	"Apellidos y Nombres o Razón social",
	"Periodo",
	"CAR SUNAT",
	"Fecha de emisión",
	"Fecha Vcto/Pago",
	"Tipo CP/Doc.",
	"Serie del CDP",
	"Año",
	"Nro CP o Doc. Nro Inicial (Rango)",
	"Nro Final (Rango)",
	"Tipo Doc Identidad",
	"Nro Doc Identidad",
	"Apellidos Nombres/ Razón Social",
	"BI Gravado DG",
	"IGV / IPM DG",
	"BI Gravado DGNG",
	"IGV / IPM DGNG",
	"BI Gravado DNG",
	"IGV / IPM DNG",
	"Valor Adq. NG",
	"ISC",
	"ICBPER",
	"Otros Trib/ Cargos",
	"Total CP",
	"Moneda",
	"Tipo de Cambio",
	"Fecha Emisión Doc Modificado",
	"Tipo CP Modificado",
	"Serie CP Modificado",
	"COD. DAM O DSI",
	"Nro CP Modificado",
	"Clasif de Bss y Sss",
	"ID Proyecto Operadores",
	"PorcPart",
	"IMB",
	"CAR Orig/ Ind E o I",
	"Detracción",
	"Tipo de Nota",
	"Est. Comp.",
	"Incal",
}

const rceStandardCLUCount = 39

var rceExpectedHeaders = buildRCEExpectedHeaders()

func buildRCEExpectedHeaders() []string {
	out := make([]string, 0, len(rceBaseHeaders)+rceStandardCLUCount)
	out = append(out, rceBaseHeaders...)
	for i := 1; i <= rceStandardCLUCount; i++ {
		out = append(out, fmt.Sprintf("CLU%d", i))
	}
	return out
}

func isTipoNotaCredito(tipoCP string) bool {
	return tipoCP == TipoCPNotaCredito || tipoCP == TipoCPNotaCreditoEspecial || tipoCP == TipoCPNotaCreditoNoDomicil
}

func isTipoNotaDebito(tipoCP string) bool {
	return tipoCP == TipoCPNotaDebito || tipoCP == TipoCPNotaDebitoEspecial || tipoCP == TipoCPNotaDebitoNoDomicil
}

// HeaderMismatch describe una diferencia puntual entre la cabecera esperada y la encontrada (sin datos de filas).
type HeaderMismatch struct {
	Indice     int    `json:"indice"`
	Esperada   string `json:"esperada"`
	Encontrada string `json:"encontrada"`
}

// FormatMismatchDetail detalla incompatibilidades de layout sin exponer contenido de filas.
type FormatMismatchDetail struct {
	Libro               string           `json:"libro"`
	ColumnasEsperadas   int              `json:"columnas_esperadas"`
	ColumnasEncontradas int              `json:"columnas_encontradas"`
	Diferencias         []HeaderMismatch `json:"diferencias,omitempty"`
}

// ProposalParseError representa un error controlado y seguro del analizador de propuestas TXT.
type ProposalParseError struct {
	Code       string
	HTTPStatus int
	Message    string
	Detail     string
	FormatInfo *FormatMismatchDetail
}

func (e *ProposalParseError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ProposalQueryOptions configura límites de lectura, filtros opcionales y paginación para el análisis.
type ProposalQueryOptions struct {
	Limit                int
	Offset               int
	Estado               string // "", "activo" (defecto), "anulado", "todos"
	TipoCP               string // filtro opcional por código de tipo de comprobante (ej. "01", "07")
	Query                string // búsqueda literal opcional (máx. 40 caracteres) en serie, número o documento
	MaxUncompressedBytes int64
}

// ProposalRow representa una fila normalizada del archivo TXT oficial de propuesta SIRE.
// No se realizan conversiones de moneda: Moneda y TipoCambio se exponen tal como figuran en el TXT.
type ProposalRow struct {
	Linea              int     `json:"linea"`
	Periodo            string  `json:"periodo"`
	CAR                string  `json:"car_sunat"`
	FechaEmision       string  `json:"fecha_emision"`
	FechaVcto          string  `json:"fecha_vcto,omitempty"`
	TipoCP             string  `json:"tipo_cp"`
	Serie              string  `json:"serie"`
	Numero             string  `json:"numero"`
	NumeroFinal        string  `json:"numero_final,omitempty"`
	TipoDocIdentidad   string  `json:"tipo_doc_identidad"`
	NroDocIdentidad    string  `json:"nro_doc_identidad"`
	RazonSocial        string  `json:"razon_social"`
	ValorExportacion   float64 `json:"valor_exportacion,omitempty"`
	BaseImponible      float64 `json:"base_imponible"`
	BaseGravadaDG      float64 `json:"base_gravada_dg,omitempty"`
	BaseGravadaDGNG    float64 `json:"base_gravada_dgng,omitempty"`
	BaseGravadaDNG     float64 `json:"base_gravada_dng,omitempty"`
	DescuentoBI        float64 `json:"descuento_bi,omitempty"`
	IGV                float64 `json:"igv"`
	IGVDG              float64 `json:"igv_dg,omitempty"`
	IGVDGNG            float64 `json:"igv_dgng,omitempty"`
	IGVDNG             float64 `json:"igv_dng,omitempty"`
	DescuentoIGV       float64 `json:"descuento_igv,omitempty"`
	Exonerado          float64 `json:"exonerado,omitempty"`
	Inafecto           float64 `json:"inafecto,omitempty"`
	NoGravado          float64 `json:"no_gravado,omitempty"`
	ISC                float64 `json:"isc,omitempty"`
	BaseIVAP           float64 `json:"base_ivap,omitempty"`
	IVAP               float64 `json:"ivap,omitempty"`
	ICBPER             float64 `json:"icbper,omitempty"`
	OtrosTributos      float64 `json:"otros_tributos,omitempty"`
	TotalCP            float64 `json:"total_cp"`
	Moneda             string  `json:"moneda"`
	TipoCambio         string  `json:"tipo_cambio"`
	FechaDocModificado string  `json:"fecha_doc_modificado,omitempty"`
	TipoDocModificado  string  `json:"tipo_doc_modificado,omitempty"`
	SerieDocModificado string  `json:"serie_doc_modificado,omitempty"`
	NroDocModificado   string  `json:"nro_doc_modificado,omitempty"`
	TipoNota           string  `json:"tipo_nota,omitempty"`
	EstadoComp         string  `json:"estado_comp"`
	EstadoMarca        string  `json:"estado_marca"` // "activo", "anulado", "desconocido"
	Anulado            bool    `json:"anulado"`
	Detraccion         string  `json:"detraccion,omitempty"`
	TipoOperacion      string  `json:"tipo_operacion,omitempty"`
}

// ProposalCounts expone conteos explícitos y separados sobre el archivo completo.
type ProposalCounts struct {
	TotalFilas        int `json:"total_filas"`
	Activos           int `json:"activos"`            // Solo estado == "1"
	Comprobantes      int `json:"comprobantes"`       // Activos distintos de NC (07/87/97) y ND (08/88/98)
	NC                int `json:"nc"`                 // Notas de crédito activas (07/87/97, restan)
	ND                int `json:"nd"`                 // Notas de débito activas (08/88/98, suman)
	Anulados          int `json:"anulados"`           // Solo estado == "2" (excluidos de totales)
	EstadoDesconocido int `json:"estado_desconocido"` // Cualquier código distinto de "1" y "2" (excluidos de totales)
}

// ProposalTotals contiene los totales acumulados exclusivamente sobre registros activos (estado == "1") del archivo completo.
// Fórmula exacta: total_cp = total_comprobantes + total_nd - total_nc (donde total_nc >= 0 representa el monto descontado por NC).
// BaseImponible e IGV se conservan como SUNAT; DescuentoBI y DescuentoIGV son informativos aparte.
type ProposalTotals struct {
	BaseImponible     float64 `json:"base_imponible"`
	BaseGravadaDG     float64 `json:"base_gravada_dg,omitempty"`
	BaseGravadaDGNG   float64 `json:"base_gravada_dgng,omitempty"`
	BaseGravadaDNG    float64 `json:"base_gravada_dng,omitempty"`
	IGV               float64 `json:"igv"`
	IGVDG             float64 `json:"igv_dg,omitempty"`
	IGVDGNG           float64 `json:"igv_dgng,omitempty"`
	IGVDNG            float64 `json:"igv_dng,omitempty"`
	ValorExportacion  float64 `json:"valor_exportacion,omitempty"`
	DescuentoBI       float64 `json:"descuento_bi,omitempty"`
	DescuentoIGV      float64 `json:"descuento_igv,omitempty"`
	Exonerado         float64 `json:"exonerado,omitempty"`
	Inafecto          float64 `json:"inafecto,omitempty"`
	NoGravado         float64 `json:"no_gravado,omitempty"`
	ISC               float64 `json:"isc,omitempty"`
	BaseIVAP          float64 `json:"base_ivap,omitempty"`
	IVAP              float64 `json:"ivap,omitempty"`
	ICBPER            float64 `json:"icbper,omitempty"`
	OtrosTributos     float64 `json:"otros_tributos,omitempty"`
	TotalComprobantes float64 `json:"total_comprobantes"`
	TotalNC           float64 `json:"total_nc"`
	TotalND           float64 `json:"total_nd"`
	TotalCP           float64 `json:"total_cp"`
}

// ProposalTypeAgg entrega el subtotal por tipo de comprobante (Tabla 10 SUNAT) sobre registros activos.
type ProposalTypeAgg struct {
	TipoCP        string  `json:"tipo_cp"`
	Categoria     string  `json:"categoria"` // "comprobante", "nc", "nd"
	Cantidad      int     `json:"cantidad"`
	BaseImponible float64 `json:"base_imponible"`
	IGV           float64 `json:"igv"`
	TotalCP       float64 `json:"total_cp"`
}

// ProposalDayAgg agrupa comprobantes activos por fecha de emisión (dd/mm/aaaa).
type ProposalDayAgg struct {
	Fecha    string  `json:"fecha"`
	Cantidad int     `json:"cantidad"`
	TotalCP  float64 `json:"total_cp"`
}

// ProposalCounterpartyAgg resume las principales contrapartes activas del periodo.
type ProposalCounterpartyAgg struct {
	NroDoc      string  `json:"nro_doc"`
	RazonSocial string  `json:"razon_social"`
	Cantidad    int     `json:"cantidad"`
	TotalCP     float64 `json:"total_cp"`
}

// ProposalSummary concentra el resumen tributario calculado siempre sobre el archivo completo (no sobre la página).
type ProposalSummary struct {
	DescargaID               int64                     `json:"descarga_id,omitempty"`
	Libro                    string                    `json:"libro"`
	CodLibro                 string                    `json:"cod_libro"`
	Periodo                  string                    `json:"periodo"`
	NombreArchivoZIP         string                    `json:"nombre_archivo_zip,omitempty"`
	NombreArchivoTXT         string                    `json:"nombre_archivo_txt"`
	Ticket                   string                    `json:"ticket,omitempty"`
	Codificacion             string                    `json:"codificacion"`
	Delimitador              string                    `json:"delimitador"`
	Columnas                 int                       `json:"columnas"`
	IncluyeAnuladosEnTotales bool                      `json:"incluye_anulados_en_totales"`
	FormulaTotales           string                    `json:"formula_totales"`
	PoliticaTotales          string                    `json:"politica_totales"`
	Advertencias             []string                  `json:"advertencias"`
	Conteos                  ProposalCounts            `json:"conteos"`
	Totales                  ProposalTotals            `json:"totales"`
	FilasNoPEN               int                       `json:"filas_no_pen"`
	ContrapartesUnicas       int                       `json:"contrapartes_unicas"`
	PorTipo                  []ProposalTypeAgg         `json:"por_tipo"`
	PorDia                   []ProposalDayAgg          `json:"por_dia"`
	TopContrapartes          []ProposalCounterpartyAgg `json:"top_contrapartes"`
}

// ProposalPagination informa el tramo de filas devuelto al cliente según los filtros aplicados.
type ProposalPagination struct {
	Estado            string `json:"estado"`
	TipoCP            string `json:"tipo_cp,omitempty"`
	Query             string `json:"q,omitempty"`
	Limit             int    `json:"limit"`
	Offset            int    `json:"offset"`
	TotalFiltrado     int    `json:"total_filtrado"`
	TotalFiltradas    int    `json:"total_filtradas"`
	TotalFilasActivas int    `json:"total_filas_activas"`
	Devueltas         int    `json:"devueltas"`
	TieneMas          bool   `json:"tiene_mas"`
}

// ProposalAnalysisResponse es la respuesta completa del endpoint GET /api/descargas/:id/propuesta-txt.
type ProposalAnalysisResponse struct {
	Resumen       ProposalSummary    `json:"resumen"`
	TotalFiltrado int                `json:"total_filtrado"`
	Paginacion    ProposalPagination `json:"paginacion"`
	Filas         []ProposalRow      `json:"filas"`
	Anulados      []ProposalRow      `json:"anulados"`
}

// ParsedProposalTXT es el resultado completo en memoria antes de paginar.
type ParsedProposalTXT struct {
	Resumen      ProposalSummary
	Todas        []ProposalRow
	Activos      []ProposalRow
	Anulados     []ProposalRow
	Desconocidos []ProposalRow
}

// normalizeProposalBook unifica libro/codLibro a "VENTAS" ("140000") o "COMPRAS" ("080000").
func normalizeProposalBook(libro, codLibro string) (string, string, error) {
	l := strings.ToUpper(strings.TrimSpace(libro))
	c := strings.TrimSpace(codLibro)
	switch {
	case l == "VENTAS" || l == "RVIE" || l == "140000" || c == "140000":
		return "VENTAS", "140000", nil
	case l == "COMPRAS" || l == "RCE" || l == "080000" || c == "080000":
		return "COMPRAS", "080000", nil
	default:
		return "", "", &ProposalParseError{
			Code:       ErrCodeFormatoInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "libro SIRE no reconocido para análisis TXT",
			Detail:     "se esperaba VENTAS (140000) o COMPRAS (080000)",
		}
	}
}

// isSafeSingleTXTZIPEntry valida que el nombre de entrada del ZIP sea únicamente un archivo .txt plano sin rutas.
func isSafeSingleTXTZIPEntry(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") || strings.Contains(name, ":") {
		return false
	}
	if filepath.IsAbs(name) || filepath.Base(name) != name {
		return false
	}
	return strings.EqualFold(filepath.Ext(name), ".txt")
}

// ReadProposalTXTFromZIPFile abre un ZIP en disco y lee su única entrada .txt en memoria (sin extraer a disco),
// aplicando io.LimitReader sobre los bytes reales descomprimidos.
func ReadProposalTXTFromZIPFile(zipPath string, maxUncompressedBytes int64) (string, []byte, error) {
	f, err := os.Open(zipPath)
	if err != nil {
		return "", nil, &ProposalParseError{
			Code:       ErrCodeNoEncontrado,
			HTTPStatus: http.StatusNotFound,
			Message:    "el archivo físico no se encuentra disponible",
		}
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return "", nil, &ProposalParseError{
			Code:       ErrCodeNoEncontrado,
			HTTPStatus: http.StatusNotFound,
			Message:    "el archivo físico no es válido",
		}
	}

	return ReadProposalTXTFromZIPReader(f, info.Size(), maxUncompressedBytes)
}

// ReadProposalTXTFromZIPReader lee en memoria el único archivo .txt permitido dentro de un ZIP.
// Regla determinista: el ZIP debe contener exactamente 1 entrada, ser archivo regular, tener nombre plano sin rutas
// y extensión .txt, y no exceder maxUncompressedBytes de bytes realmente leídos.
func ReadProposalTXTFromZIPReader(r io.ReaderAt, size int64, maxUncompressedBytes int64) (string, []byte, error) {
	if maxUncompressedBytes <= 0 {
		maxUncompressedBytes = DefaultMaxProposalTXTBytes
	}

	zr, err := zip.NewReader(r, size)
	if err != nil {
		return "", nil, &ProposalParseError{
			Code:       ErrCodeZipInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "el archivo no es un ZIP válido",
		}
	}

	if len(zr.File) == 0 {
		return "", nil, &ProposalParseError{
			Code:       ErrCodeZipInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "el archivo ZIP está vacío",
		}
	}

	if len(zr.File) != 1 {
		return "", nil, &ProposalParseError{
			Code:       ErrCodeZipInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "el ZIP debe contener exactamente un archivo .txt",
			Detail:     fmt.Sprintf("se encontraron %d entradas en el ZIP", len(zr.File)),
		}
	}

	entry := zr.File[0]
	if entry.FileInfo().IsDir() || !isSafeSingleTXTZIPEntry(entry.Name) {
		return "", nil, &ProposalParseError{
			Code:       ErrCodeZipInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "entrada de ZIP rechazada: solo se admite un único archivo .txt sin rutas ni directorios",
		}
	}

	rc, err := entry.Open()
	if err != nil {
		return "", nil, &ProposalParseError{
			Code:       ErrCodeZipInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "no se pudo abrir el archivo .txt dentro del ZIP",
		}
	}
	defer rc.Close()

	// Protección contra ZIP bomba: leemos con io.LimitReader sobre los bytes REALES descomprimidos,
	// ignorando el tamaño declarado en la cabecera del ZIP.
	limited := io.LimitReader(rc, maxUncompressedBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return "", nil, &ProposalParseError{
			Code:       ErrCodeZipInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "error leyendo el flujo descomprimido del ZIP",
		}
	}
	if int64(len(raw)) > maxUncompressedBytes {
		return "", nil, &ProposalParseError{
			Code:       ErrCodeZipDemasiadoGrande,
			HTTPStatus: http.StatusRequestEntityTooLarge,
			Message:    "el archivo TXT descomprimido excede el límite máximo permitido",
			Detail:     fmt.Sprintf("límite máximo: %d bytes", maxUncompressedBytes),
		}
	}

	return entry.Name, raw, nil
}

// decodeProposalTXTBytes maneja explícitamente la codificación UTF-8 con BOM, UTF-8 sin BOM o Windows-1252.
func decodeProposalTXTBytes(raw []byte) (string, string, error) {
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		trimmed := raw[3:]
		if utf8.Valid(trimmed) {
			return string(trimmed), "UTF-8-BOM", nil
		}
	}
	if utf8.Valid(raw) {
		return string(raw), "UTF-8", nil
	}
	decoded, err := charmap.Windows1252.NewDecoder().Bytes(raw)
	if err != nil {
		return "", "", &ProposalParseError{
			Code:       ErrCodeFormatoInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "no se pudo decodificar el archivo TXT como UTF-8 ni Windows-1252",
		}
	}
	return string(decoded), "Windows-1252", nil
}

func normalizeHeaderToken(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(s, "\uFEFF"))
	fields := strings.Fields(s)
	return strings.ToLower(strings.Join(fields, " "))
}

// validateProposalHeader valida la cabecera oficial del libro:
// - VENTAS (RVIE): validación estricta de las 40 columnas exactas (00..39).
// - COMPRAS (RCE): validación estricta por nombre de las 41 columnas núcleo (00..40).
//   Las columnas opcionales CLU1..CLU39 (índices 41+) generan advertencia si su conteo o nombres cambian,
//   sin bloquear el análisis mientras el núcleo 00..40 sea válido.
func validateProposalHeader(libro string, foundCols []string) ([]string, *ProposalParseError) {
	if libro == "VENTAS" {
		expected := rvieExpectedHeaders
		var mismatches []HeaderMismatch
		maxLen := len(expected)
		if len(foundCols) > maxLen {
			maxLen = len(foundCols)
		}
		for i := 0; i < maxLen; i++ {
			exp := ""
			if i < len(expected) {
				exp = expected[i]
			}
			fnd := ""
			if i < len(foundCols) {
				fnd = strings.TrimSpace(foundCols[i])
			}
			if normalizeHeaderToken(exp) != normalizeHeaderToken(fnd) {
				mismatches = append(mismatches, HeaderMismatch{
					Indice:     i,
					Esperada:   exp,
					Encontrada: fnd,
				})
			}
		}
		if len(foundCols) != len(expected) || len(mismatches) > 0 {
			return nil, &ProposalParseError{
				Code:       ErrCodeFormatoInesperado,
				HTTPStatus: http.StatusUnprocessableEntity,
				Message:    "la cabecera del archivo TXT no coincide con el formato oficial de SUNAT SIRE",
				Detail:     fmt.Sprintf("libro %s: esperadas %d columnas, encontradas %d (%d diferencia(s) de cabecera)", libro, len(expected), len(foundCols), len(mismatches)),
				FormatInfo: &FormatMismatchDetail{
					Libro:               libro,
					ColumnasEsperadas:   len(expected),
					ColumnasEncontradas: len(foundCols),
					Diferencias:         mismatches,
				},
			}
		}
		return nil, nil
	}

	// COMPRAS (RCE): validación estricta de columnas núcleo 00..40 (41 columnas)
	coreExpected := rceBaseHeaders
	var coreMismatches []HeaderMismatch
	for i := 0; i < len(coreExpected); i++ {
		fnd := ""
		if i < len(foundCols) {
			fnd = strings.TrimSpace(foundCols[i])
		}
		if normalizeHeaderToken(coreExpected[i]) != normalizeHeaderToken(fnd) {
			coreMismatches = append(coreMismatches, HeaderMismatch{
				Indice:     i,
				Esperada:   coreExpected[i],
				Encontrada: fnd,
			})
		}
	}
	if len(foundCols) < len(coreExpected) || len(coreMismatches) > 0 {
		return nil, &ProposalParseError{
			Code:       ErrCodeFormatoInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "la cabecera núcleo (columnas 00..40) del archivo TXT RCE no coincide con el formato oficial de SUNAT SIRE",
			Detail:     fmt.Sprintf("libro COMPRAS: se requieren al menos %d columnas núcleo, encontradas %d (%d diferencia(s) en núcleo)", len(coreExpected), len(foundCols), len(coreMismatches)),
			FormatInfo: &FormatMismatchDetail{
				Libro:               "COMPRAS",
				ColumnasEsperadas:   len(coreExpected),
				ColumnasEncontradas: len(foundCols),
				Diferencias:         coreMismatches,
			},
		}
	}

	// Columnas opcionales CLU1..CLU39 (índices 41 en adelante)
	var warnings []string
	extraCols := foundCols[len(coreExpected):]
	cluMismatch := false
	for i, rawName := range extraCols {
		expectedCLU := fmt.Sprintf("CLU%d", i+1)
		if normalizeHeaderToken(expectedCLU) != normalizeHeaderToken(rawName) {
			cluMismatch = true
			break
		}
	}
	if len(extraCols) != rceStandardCLUCount {
		warnings = append(warnings, fmt.Sprintf("RCE: el archivo contiene %d columna(s) opcional(es) CLU tras las 41 columnas núcleo (estándar SUNAT: %d columnas CLU1..CLU39).", len(extraCols), rceStandardCLUCount))
	} else if cluMismatch {
		warnings = append(warnings, "RCE: los nombres de algunas columnas opcionales CLU (41..79) difieren del estándar CLU1..CLU39.")
	}

	return warnings, nil
}

func parseCents(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, errors.New("valor numérico inválido")
	}
	return int64(math.Round(v * 100)), nil
}

func centsToFloat(c int64) float64 {
	return float64(c) / 100.0
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// applySignRule aplica la regla tributaria explícita por tipo de comprobante (Tabla 10 SUNAT):
// - Notas de Crédito (07, 87, 97): siempre restan (-abs).
// - Notas de Débito (08, 88, 98): siempre suman (+abs).
// - Otros comprobantes: conserva su valor numérico del TXT.
func applySignRule(tipoCP string, cents int64) int64 {
	switch {
	case isTipoNotaCredito(tipoCP):
		return -absInt64(cents)
	case isTipoNotaDebito(tipoCP):
		return absInt64(cents)
	default:
		return cents
	}
}

func classifyTipoCategoria(tipoCP string) string {
	switch {
	case isTipoNotaCredito(tipoCP):
		return "nc"
	case isTipoNotaDebito(tipoCP):
		return "nd"
	default:
		return "comprobante"
	}
}

// ParseProposalTXTBytes valida cabecera, codificación, delimitador y parsea un TXT oficial RVIE/RCE en memoria.
func ParseProposalTXTBytes(raw []byte, txtName, libro, codLibro string) (*ParsedProposalTXT, error) {
	normLibro, normCodLibro, err := normalizeProposalBook(libro, codLibro)
	if err != nil {
		return nil, err
	}

	text, encodingName, err := decodeProposalTXTBytes(raw)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	headerRead := false
	headerColsCount := 0
	defaultExpectedCols := len(rvieExpectedHeaders)
	if normLibro == "COMPRAS" {
		defaultExpectedCols = len(rceBaseHeaders)
	}

	advertencias := make([]string, 0)
	todas := make([]ProposalRow, 0)
	activos := make([]ProposalRow, 0)
	anulados := make([]ProposalRow, 0)
	desconocidos := make([]ProposalRow, 0)
	unknownEstadoCodes := make(map[string]struct{})

	type aggCents struct {
		count int
		bi    int64
		igv   int64
		total int64
	}
	type cpAggCents struct {
		razon string
		count int
		total int64
	}

	byTipo := make(map[string]*aggCents)
	byDia := make(map[string]*aggCents)
	byCP := make(map[string]*cpAggCents)
	distinctCP := make(map[string]struct{})

	var (
		counts                                    ProposalCounts
		sumBI, sumBIDG, sumBIDGNG, sumBIDNG       int64
		sumIGV, sumIGVDG, sumIGVDGNG, sumIGVDNG   int64
		sumExp, sumDsctoBI, sumDsctoIGV           int64
		sumExon, sumInaf, sumNoGrav, sumISC       int64
		sumBIIVAP, sumIVAP, sumICBPER, sumOtros   int64
		sumTotalComp, sumTotalNCAbs, sumTotalND   int64
		sumTotal                                  int64
		filasNoPEN                                int
		detectedPeriodo                           string
		lineNum                                   int
	)

	for scanner.Scan() {
		lineNum++
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}

		if !headerRead {
			if !strings.Contains(line, ProposalTXTDelimiter) {
				return nil, &ProposalParseError{
					Code:       ErrCodeFormatoInesperado,
					HTTPStatus: http.StatusUnprocessableEntity,
					Message:    "el archivo TXT no utiliza el delimitador oficial '|' (pipe)",
					FormatInfo: &FormatMismatchDetail{
						Libro:               normLibro,
						ColumnasEsperadas:   defaultExpectedCols,
						ColumnasEncontradas: 1,
					},
				}
			}
			headerCols := strings.Split(line, ProposalTXTDelimiter)
			hdrWarnings, hdrErr := validateProposalHeader(normLibro, headerCols)
			if hdrErr != nil {
				return nil, hdrErr
			}
			advertencias = append(advertencias, hdrWarnings...)
			headerColsCount = len(headerCols)
			headerRead = true
			continue
		}

		cols := strings.Split(line, ProposalTXTDelimiter)
		if len(cols) != headerColsCount {
			return nil, &ProposalParseError{
				Code:       ErrCodeFormatoInesperado,
				HTTPStatus: http.StatusUnprocessableEntity,
				Message:    "una fila de datos no coincide con el número de columnas de la cabecera",
				Detail:     fmt.Sprintf("línea %d: esperadas %d columnas, encontradas %d", lineNum, headerColsCount, len(cols)),
				FormatInfo: &FormatMismatchDetail{
					Libro:               normLibro,
					ColumnasEsperadas:   headerColsCount,
					ColumnasEncontradas: len(cols),
				},
			}
		}

		row, rowCents, parseErr := parseProposalRow(normLibro, lineNum, cols)
		if parseErr != nil {
			return nil, parseErr
		}

		if detectedPeriodo == "" && row.Periodo != "" {
			detectedPeriodo = row.Periodo
		}

		counts.TotalFilas++
		if mon := strings.ToUpper(strings.TrimSpace(row.Moneda)); mon != "" && mon != "PEN" {
			filasNoPEN++
		}
		todas = append(todas, row)

		// Regla estricta de estados:
		// - Solo "1" es activo (suma en totales).
		// - Solo "2" es anulado (excluido de totales, devuelto aparte).
		// - Cualquier otro código es estado desconocido (excluido de totales, nunca sumado en silencio, advertencia explícita).
		switch row.EstadoMarca {
		case EstadoMarcaAnulado:
			counts.Anulados++
			anulados = append(anulados, row)
			continue
		case EstadoMarcaDesconocido:
			counts.EstadoDesconocido++
			codeLbl := row.EstadoComp
			if codeLbl == "" {
				codeLbl = "(vacío)"
			}
			unknownEstadoCodes[codeLbl] = struct{}{}
			desconocidos = append(desconocidos, row)
			continue
		}

		counts.Activos++
		switch {
		case isTipoNotaCredito(row.TipoCP):
			counts.NC++
			// total_nc en magnitud positiva para que: total_cp = total_comprobantes + total_nd - total_nc
			sumTotalNCAbs += absInt64(rowCents.total)
		case isTipoNotaDebito(row.TipoCP):
			counts.ND++
			sumTotalND += rowCents.total
		default:
			counts.Comprobantes++
			sumTotalComp += rowCents.total
		}

		sumBI += rowCents.bi
		sumBIDG += rowCents.biDG
		sumBIDGNG += rowCents.biDGNG
		sumBIDNG += rowCents.biDNG
		sumIGV += rowCents.igv
		sumIGVDG += rowCents.igvDG
		sumIGVDGNG += rowCents.igvDGNG
		sumIGVDNG += rowCents.igvDNG
		sumExp += rowCents.valorExp
		sumDsctoBI += rowCents.dsctoBI
		sumDsctoIGV += rowCents.dsctoIGV
		sumExon += rowCents.exonerado
		sumInaf += rowCents.inafecto
		sumNoGrav += rowCents.noGravado
		sumISC += rowCents.isc
		sumBIIVAP += rowCents.biIVAP
		sumIVAP += rowCents.ivap
		sumICBPER += rowCents.icbper
		sumOtros += rowCents.otros
		// Equivalente exacto a: sumTotalComp + sumTotalND - sumTotalNCAbs
		sumTotal += rowCents.total

		// Subtotal por tipo de comprobante (solo activos)
		tipoKey := row.TipoCP
		if tipoKey == "" {
			tipoKey = "SIN_TIPO"
		}
		if _, ok := byTipo[tipoKey]; !ok {
			byTipo[tipoKey] = &aggCents{}
		}
		byTipo[tipoKey].count++
		byTipo[tipoKey].bi += rowCents.bi
		byTipo[tipoKey].igv += rowCents.igv
		byTipo[tipoKey].total += rowCents.total

		// Agregados por día (solo activos)
		if row.FechaEmision != "" {
			if _, ok := byDia[row.FechaEmision]; !ok {
				byDia[row.FechaEmision] = &aggCents{}
			}
			byDia[row.FechaEmision].count++
			byDia[row.FechaEmision].total += rowCents.total
		}

		// Contrapartes únicas y Top 5 (solo activos)
		cpDoc := strings.TrimSpace(row.NroDocIdentidad)
		if cpDoc != "" {
			distinctCP[cpDoc] = struct{}{}
			if _, ok := byCP[cpDoc]; !ok {
				byCP[cpDoc] = &cpAggCents{razon: row.RazonSocial}
			}
			if byCP[cpDoc].razon == "" && row.RazonSocial != "" {
				byCP[cpDoc].razon = row.RazonSocial
			}
			byCP[cpDoc].count++
			byCP[cpDoc].total += rowCents.total
		}

		activos = append(activos, row)
	}

	if err := scanner.Err(); err != nil {
		return nil, &ProposalParseError{
			Code:       ErrCodeFormatoInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "error leyendo líneas del archivo TXT",
		}
	}

	if !headerRead {
		return nil, &ProposalParseError{
			Code:       ErrCodeFormatoInesperado,
			HTTPStatus: http.StatusUnprocessableEntity,
			Message:    "el archivo TXT está vacío y no contiene cabecera",
			FormatInfo: &FormatMismatchDetail{
				Libro:               normLibro,
				ColumnasEsperadas:   defaultExpectedCols,
				ColumnasEncontradas: 0,
			},
		}
	}

	if counts.EstadoDesconocido > 0 {
		codes := make([]string, 0, len(unknownEstadoCodes))
		for code := range unknownEstadoCodes {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		advertencias = append(advertencias, fmt.Sprintf("Se excluyeron %d registro(s) de los totales por tener código de estado desconocido [%s] (solo '1' es activo y '2' anulado).", counts.EstadoDesconocido, strings.Join(codes, ", ")))
	}

	porTipo := make([]ProposalTypeAgg, 0, len(byTipo))
	for k, v := range byTipo {
		porTipo = append(porTipo, ProposalTypeAgg{
			TipoCP:        k,
			Categoria:     classifyTipoCategoria(k),
			Cantidad:      v.count,
			BaseImponible: centsToFloat(v.bi),
			IGV:           centsToFloat(v.igv),
			TotalCP:       centsToFloat(v.total),
		})
	}
	sort.Slice(porTipo, func(i, j int) bool { return porTipo[i].TipoCP < porTipo[j].TipoCP })

	porDia := make([]ProposalDayAgg, 0, len(byDia))
	for k, v := range byDia {
		porDia = append(porDia, ProposalDayAgg{
			Fecha:    k,
			Cantidad: v.count,
			TotalCP:  centsToFloat(v.total),
		})
	}
	sort.Slice(porDia, func(i, j int) bool { return sortableDateKey(porDia[i].Fecha) < sortableDateKey(porDia[j].Fecha) })

	topCP := make([]ProposalCounterpartyAgg, 0, len(byCP))
	for k, v := range byCP {
		topCP = append(topCP, ProposalCounterpartyAgg{
			NroDoc:      k,
			RazonSocial: v.razon,
			Cantidad:    v.count,
			TotalCP:     centsToFloat(v.total),
		})
	}
	sort.Slice(topCP, func(i, j int) bool {
		if topCP[i].TotalCP != topCP[j].TotalCP {
			return topCP[i].TotalCP > topCP[j].TotalCP
		}
		return topCP[i].NroDoc < topCP[j].NroDoc
	})
	if len(topCP) > 5 {
		topCP = topCP[:5]
	}

	summary := ProposalSummary{
		Libro:                    normLibro,
		CodLibro:                 normCodLibro,
		Periodo:                  detectedPeriodo,
		NombreArchivoTXT:         txtName,
		Codificacion:             encodingName,
		Delimitador:              ProposalTXTDelimiter,
		Columnas:                 headerColsCount,
		IncluyeAnuladosEnTotales: false,
		FormulaTotales:           "Total = total_comprobantes + total_nd - total_nc",
		PoliticaTotales:          "Solo estado '1' suma; estado '2' (anulado) y cualquier estado distinto se excluyen de totales. NC (07, 87, 97) restan; ND (08, 88, 98) suman. Base e IGV tal como SUNAT; descuentos RVIE informativos aparte. Sin conversión de moneda.",
		Advertencias:             advertencias,
		Conteos:                  counts,
		Totales: ProposalTotals{
			BaseImponible:     centsToFloat(sumBI),
			BaseGravadaDG:     centsToFloat(sumBIDG),
			BaseGravadaDGNG:   centsToFloat(sumBIDGNG),
			BaseGravadaDNG:    centsToFloat(sumBIDNG),
			IGV:               centsToFloat(sumIGV),
			IGVDG:             centsToFloat(sumIGVDG),
			IGVDGNG:           centsToFloat(sumIGVDGNG),
			IGVDNG:            centsToFloat(sumIGVDNG),
			ValorExportacion:  centsToFloat(sumExp),
			DescuentoBI:       centsToFloat(sumDsctoBI),
			DescuentoIGV:      centsToFloat(sumDsctoIGV),
			Exonerado:         centsToFloat(sumExon),
			Inafecto:          centsToFloat(sumInaf),
			NoGravado:         centsToFloat(sumNoGrav),
			ISC:               centsToFloat(sumISC),
			BaseIVAP:          centsToFloat(sumBIIVAP),
			IVAP:              centsToFloat(sumIVAP),
			ICBPER:            centsToFloat(sumICBPER),
			OtrosTributos:     centsToFloat(sumOtros),
			TotalComprobantes: centsToFloat(sumTotalComp),
			TotalNC:           centsToFloat(sumTotalNCAbs),
			TotalND:           centsToFloat(sumTotalND),
			TotalCP:           centsToFloat(sumTotal),
		},
		FilasNoPEN:         filasNoPEN,
		ContrapartesUnicas: len(distinctCP),
		PorTipo:            porTipo,
		PorDia:             porDia,
		TopContrapartes:    topCP,
	}

	return &ParsedProposalTXT{
		Resumen:      summary,
		Todas:        todas,
		Activos:      activos,
		Anulados:     anulados,
		Desconocidos: desconocidos,
	}, nil
}

type rowAmountCents struct {
	bi, biDG, biDGNG, biDNG     int64
	igv, igvDG, igvDGNG, igvDNG int64
	valorExp, dsctoBI, dsctoIGV int64
	exonerado, inafecto         int64
	noGravado, isc              int64
	biIVAP, ivap, icbper, otros int64
	total                       int64
}

func parseProposalRow(libro string, lineNum int, cols []string) (ProposalRow, rowAmountCents, *ProposalParseError) {
	col := func(i int) string {
		if i < 0 || i >= len(cols) {
			return ""
		}
		return strings.TrimSpace(cols[i])
	}
	num := func(i int, fieldName string) (int64, *ProposalParseError) {
		c, err := parseCents(col(i))
		if err != nil {
			return 0, &ProposalParseError{
				Code:       ErrCodeFormatoInesperado,
				HTTPStatus: http.StatusUnprocessableEntity,
				Message:    "valor numérico inválido en archivo TXT",
				Detail:     fmt.Sprintf("línea %d, columna %d (%s)", lineNum, i, fieldName),
			}
		}
		return c, nil
	}

	var (
		row ProposalRow
		amt rowAmountCents
		err *ProposalParseError
	)

	row.Linea = lineNum
	row.Periodo = col(2)
	row.CAR = col(3)
	row.FechaEmision = col(4)
	row.FechaVcto = col(5)
	row.TipoCP = col(6)
	row.Serie = col(7)

	if libro == "VENTAS" {
		row.Numero = col(8)
		row.NumeroFinal = col(9)
		row.TipoDocIdentidad = col(10)
		row.NroDocIdentidad = col(11)
		row.RazonSocial = col(12)

		if amt.valorExp, err = num(13, "Valor Facturado Exportación"); err != nil {
			return row, amt, err
		}
		if amt.bi, err = num(14, "BI Gravada"); err != nil {
			return row, amt, err
		}
		if amt.dsctoBI, err = num(15, "Dscto BI"); err != nil {
			return row, amt, err
		}
		if amt.igv, err = num(16, "IGV / IPM"); err != nil {
			return row, amt, err
		}
		if amt.dsctoIGV, err = num(17, "Dscto IGV / IPM"); err != nil {
			return row, amt, err
		}
		if amt.exonerado, err = num(18, "Mto Exonerado"); err != nil {
			return row, amt, err
		}
		if amt.inafecto, err = num(19, "Mto Inafecto"); err != nil {
			return row, amt, err
		}
		if amt.isc, err = num(20, "ISC"); err != nil {
			return row, amt, err
		}
		if amt.biIVAP, err = num(21, "BI Grav IVAP"); err != nil {
			return row, amt, err
		}
		if amt.ivap, err = num(22, "IVAP"); err != nil {
			return row, amt, err
		}
		if amt.icbper, err = num(23, "ICBPER"); err != nil {
			return row, amt, err
		}
		if amt.otros, err = num(24, "Otros Tributos"); err != nil {
			return row, amt, err
		}
		if amt.total, err = num(25, "Total CP"); err != nil {
			return row, amt, err
		}

		row.Moneda = col(26)
		row.TipoCambio = col(27)
		row.FechaDocModificado = col(28)
		row.TipoDocModificado = col(29)
		row.SerieDocModificado = col(30)
		row.NroDocModificado = col(31)
		row.TipoNota = col(33)
		row.EstadoComp = col(34)
		row.TipoOperacion = col(37)
	} else {
		// COMPRAS (RCE: 41 columnas núcleo 00..40 + opcionales CLU1..CLU39)
		row.Numero = col(9)
		row.NumeroFinal = col(10)
		row.TipoDocIdentidad = col(11)
		row.NroDocIdentidad = col(12)
		row.RazonSocial = col(13)

		if amt.biDG, err = num(14, "BI Gravado DG"); err != nil {
			return row, amt, err
		}
		if amt.igvDG, err = num(15, "IGV / IPM DG"); err != nil {
			return row, amt, err
		}
		if amt.biDGNG, err = num(16, "BI Gravado DGNG"); err != nil {
			return row, amt, err
		}
		if amt.igvDGNG, err = num(17, "IGV / IPM DGNG"); err != nil {
			return row, amt, err
		}
		if amt.biDNG, err = num(18, "BI Gravado DNG"); err != nil {
			return row, amt, err
		}
		if amt.igvDNG, err = num(19, "IGV / IPM DNG"); err != nil {
			return row, amt, err
		}
		if amt.noGravado, err = num(20, "Valor Adq. NG"); err != nil {
			return row, amt, err
		}
		if amt.isc, err = num(21, "ISC"); err != nil {
			return row, amt, err
		}
		if amt.icbper, err = num(22, "ICBPER"); err != nil {
			return row, amt, err
		}
		if amt.otros, err = num(23, "Otros Trib/ Cargos"); err != nil {
			return row, amt, err
		}
		if amt.total, err = num(24, "Total CP"); err != nil {
			return row, amt, err
		}

		amt.bi = amt.biDG + amt.biDGNG + amt.biDNG
		amt.igv = amt.igvDG + amt.igvDGNG + amt.igvDNG

		row.Moneda = col(25)
		row.TipoCambio = col(26)
		row.FechaDocModificado = col(27)
		row.TipoDocModificado = col(28)
		row.SerieDocModificado = col(29)
		row.NroDocModificado = col(31)
		row.Detraccion = col(37)
		row.TipoNota = col(38)
		row.EstadoComp = col(39)
	}

	switch row.EstadoComp {
	case EstadoCompActivo:
		row.EstadoMarca = EstadoMarcaActivo
		row.Anulado = false
	case EstadoCompAnulado:
		row.EstadoMarca = EstadoMarcaAnulado
		row.Anulado = true
	default:
		row.EstadoMarca = EstadoMarcaDesconocido
		row.Anulado = false
	}

	// Aplicar signo explícito para NC (07, 87, 97 = resta) y ND (08, 88, 98 = suma) en comprobantes activos.
	if row.EstadoMarca == EstadoMarcaActivo && (isTipoNotaCredito(row.TipoCP) || isTipoNotaDebito(row.TipoCP)) {
		amt.bi = applySignRule(row.TipoCP, amt.bi)
		amt.biDG = applySignRule(row.TipoCP, amt.biDG)
		amt.biDGNG = applySignRule(row.TipoCP, amt.biDGNG)
		amt.biDNG = applySignRule(row.TipoCP, amt.biDNG)
		amt.igv = applySignRule(row.TipoCP, amt.igv)
		amt.igvDG = applySignRule(row.TipoCP, amt.igvDG)
		amt.igvDGNG = applySignRule(row.TipoCP, amt.igvDGNG)
		amt.igvDNG = applySignRule(row.TipoCP, amt.igvDNG)
		amt.valorExp = applySignRule(row.TipoCP, amt.valorExp)
		amt.dsctoBI = applySignRule(row.TipoCP, amt.dsctoBI)
		amt.dsctoIGV = applySignRule(row.TipoCP, amt.dsctoIGV)
		amt.exonerado = applySignRule(row.TipoCP, amt.exonerado)
		amt.inafecto = applySignRule(row.TipoCP, amt.inafecto)
		amt.noGravado = applySignRule(row.TipoCP, amt.noGravado)
		amt.isc = applySignRule(row.TipoCP, amt.isc)
		amt.biIVAP = applySignRule(row.TipoCP, amt.biIVAP)
		amt.ivap = applySignRule(row.TipoCP, amt.ivap)
		amt.icbper = applySignRule(row.TipoCP, amt.icbper)
		amt.otros = applySignRule(row.TipoCP, amt.otros)
		amt.total = applySignRule(row.TipoCP, amt.total)
	}

	row.ValorExportacion = centsToFloat(amt.valorExp)
	row.BaseImponible = centsToFloat(amt.bi)
	row.BaseGravadaDG = centsToFloat(amt.biDG)
	row.BaseGravadaDGNG = centsToFloat(amt.biDGNG)
	row.BaseGravadaDNG = centsToFloat(amt.biDNG)
	row.DescuentoBI = centsToFloat(amt.dsctoBI)
	row.IGV = centsToFloat(amt.igv)
	row.IGVDG = centsToFloat(amt.igvDG)
	row.IGVDGNG = centsToFloat(amt.igvDGNG)
	row.IGVDNG = centsToFloat(amt.igvDNG)
	row.DescuentoIGV = centsToFloat(amt.dsctoIGV)
	row.Exonerado = centsToFloat(amt.exonerado)
	row.Inafecto = centsToFloat(amt.inafecto)
	row.NoGravado = centsToFloat(amt.noGravado)
	row.ISC = centsToFloat(amt.isc)
	row.BaseIVAP = centsToFloat(amt.biIVAP)
	row.IVAP = centsToFloat(amt.ivap)
	row.ICBPER = centsToFloat(amt.icbper)
	row.OtrosTributos = centsToFloat(amt.otros)
	row.TotalCP = centsToFloat(amt.total)

	return row, amt, nil
}

// ParseProposalZIPFile lee y analiza un ZIP de propuesta SIRE directamente en memoria.
func ParseProposalZIPFile(zipPath, libro, codLibro string, maxUncompressedBytes int64) (*ParsedProposalTXT, error) {
	txtName, raw, err := ReadProposalTXTFromZIPFile(zipPath, maxUncompressedBytes)
	if err != nil {
		return nil, err
	}
	return ParseProposalTXTBytes(raw, txtName, libro, codLibro)
}

// ParseProposalZIPReader lee y analiza un ZIP desde io.ReaderAt directamente en memoria.
func ParseProposalZIPReader(r io.ReaderAt, size int64, libro, codLibro string, maxUncompressedBytes int64) (*ParsedProposalTXT, error) {
	txtName, raw, err := ReadProposalTXTFromZIPReader(r, size, maxUncompressedBytes)
	if err != nil {
		return nil, err
	}
	return ParseProposalTXTBytes(raw, txtName, libro, codLibro)
}

func normalizeEstadoFilter(raw string) (string, bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", EstadoMarcaActivo:
		return EstadoMarcaActivo, true
	case EstadoMarcaAnulado:
		return EstadoMarcaAnulado, true
	case EstadoFiltroTodos:
		return EstadoFiltroTodos, true
	default:
		return "", false
	}
}

func matchesProposalFilters(row ProposalRow, tipoFilter, qLower string) bool {
	if tipoFilter != "" && !strings.EqualFold(strings.TrimSpace(row.TipoCP), tipoFilter) {
		return false
	}
	if qLower != "" {
		serieLower := strings.ToLower(row.Serie)
		numLower := strings.ToLower(row.Numero)
		docLower := strings.ToLower(row.NroDocIdentidad)
		compLower := serieLower + "-" + numLower
		if !strings.Contains(serieLower, qLower) &&
			!strings.Contains(numLower, qLower) &&
			!strings.Contains(compLower, qLower) &&
			!strings.Contains(docLower, qLower) {
			return false
		}
	}
	return true
}

// PaginateProposalResult construye la respuesta paginada aplicando filtros combinados (estado, tipo_cp y q literal).
// El resumen siempre proviene del archivo completo (parsed.Resumen), nunca de la página ni de los filtros.
func PaginateProposalResult(parsed *ParsedProposalTXT, opts ProposalQueryOptions) *ProposalAnalysisResponse {
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultProposalPageLimit
	}
	if limit > MaxProposalPageLimit {
		limit = MaxProposalPageLimit
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	normEstado, ok := normalizeEstadoFilter(opts.Estado)
	if !ok {
		normEstado = EstadoMarcaActivo
	}

	var base []ProposalRow
	switch normEstado {
	case EstadoMarcaAnulado:
		base = parsed.Anulados
	case EstadoFiltroTodos:
		base = parsed.Todas
	default:
		base = parsed.Activos
	}

	tipoFilter := strings.TrimSpace(opts.TipoCP)
	qTrim := strings.TrimSpace(opts.Query)
	qLower := strings.ToLower(qTrim)

	var filtered []ProposalRow
	if tipoFilter == "" && qLower == "" {
		filtered = base
	} else {
		filtered = make([]ProposalRow, 0)
		for _, row := range base {
			if matchesProposalFilters(row, tipoFilter, qLower) {
				filtered = append(filtered, row)
			}
		}
	}

	totalFiltrado := len(filtered)
	start := offset
	if start > totalFiltrado {
		start = totalFiltrado
	}
	end := start + limit
	if end > totalFiltrado {
		end = totalFiltrado
	}

	pageRows := make([]ProposalRow, end-start)
	copy(pageRows, filtered[start:end])

	anuladosCopy := make([]ProposalRow, len(parsed.Anulados))
	copy(anuladosCopy, parsed.Anulados)

	return &ProposalAnalysisResponse{
		Resumen:       parsed.Resumen,
		TotalFiltrado: totalFiltrado,
		Paginacion: ProposalPagination{
			Estado:            normEstado,
			TipoCP:            tipoFilter,
			Query:             qTrim,
			Limit:             limit,
			Offset:            offset,
			TotalFiltrado:     totalFiltrado,
			TotalFiltradas:    totalFiltrado,
			TotalFilasActivas: len(parsed.Activos),
			Devueltas:         len(pageRows),
			TieneMas:          end < totalFiltrado,
		},
		Filas:    pageRows,
		Anulados: anuladosCopy,
	}
}

// resolveSafeStoragePath valida que targetPath exista, no sea directorio y quede estrictamente dentro de storageDir
// tras resolver filepath.Clean, filepath.Abs y filepath.EvalSymlinks.
func resolveSafeStoragePath(storageDir, targetPath string) (string, error) {
	if strings.TrimSpace(targetPath) == "" {
		return "", &ProposalParseError{
			Code:       ErrCodeNoEncontrado,
			HTTPStatus: http.StatusNotFound,
			Message:    "no existe ruta registrada para el archivo físico",
		}
	}

	rawAbsStorage, err := filepath.Abs(filepath.Clean(storageDir))
	if err != nil {
		return "", &ProposalParseError{
			Code:       ErrCodeRutaInvalida,
			HTTPStatus: http.StatusForbidden,
			Message:    "no se pudo resolver el directorio de descargas",
		}
	}
	evalStorage := rawAbsStorage
	if resolved, err := filepath.EvalSymlinks(rawAbsStorage); err == nil {
		evalStorage = resolved
	}

	trimmedTarget := strings.TrimSpace(targetPath)
	cleanedTarget := filepath.Clean(trimmedTarget)
	var candidatePath string
	if filepath.IsAbs(cleanedTarget) {
		candidatePath = cleanedTarget
	} else {
		if strings.HasPrefix(trimmedTarget, "/") || strings.HasPrefix(trimmedTarget, "\\") {
			return "", &ProposalParseError{
				Code:       ErrCodeRutaInvalida,
				HTTPStatus: http.StatusForbidden,
				Message:    "violación de seguridad: la ruta del archivo queda fuera del área permitida",
			}
		}
		candidatePath = filepath.Join(rawAbsStorage, filepath.FromSlash(cleanedTarget))
	}

	absTarget, err := filepath.Abs(filepath.Clean(candidatePath))
	if err != nil {
		return "", &ProposalParseError{
			Code:       ErrCodeRutaInvalida,
			HTTPStatus: http.StatusForbidden,
			Message:    "ruta de archivo inválida",
		}
	}

	if !isPathWithinBase(rawAbsStorage, absTarget) && !isPathWithinBase(evalStorage, absTarget) {
		return "", &ProposalParseError{
			Code:       ErrCodeRutaInvalida,
			HTTPStatus: http.StatusForbidden,
			Message:    "violación de seguridad: la ruta del archivo queda fuera del área permitida",
		}
	}

	evalTarget, err := filepath.EvalSymlinks(absTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &ProposalParseError{
				Code:       ErrCodeNoEncontrado,
				HTTPStatus: http.StatusNotFound,
				Message:    "el archivo físico no fue encontrado en el servidor",
			}
		}
		return "", &ProposalParseError{
			Code:       ErrCodeRutaInvalida,
			HTTPStatus: http.StatusForbidden,
			Message:    "no se pudo validar la ruta real del archivo",
		}
	}

	if !isPathWithinBase(evalStorage, evalTarget) {
		return "", &ProposalParseError{
			Code:       ErrCodeRutaInvalida,
			HTTPStatus: http.StatusForbidden,
			Message:    "violación de seguridad: la ruta resuelta queda fuera del área permitida",
		}
	}

	info, err := os.Stat(evalTarget)
	if err != nil || info.IsDir() {
		return "", &ProposalParseError{
			Code:       ErrCodeNoEncontrado,
			HTTPStatus: http.StatusNotFound,
			Message:    "el archivo físico no fue encontrado o no es un archivo regular",
		}
	}

	return evalTarget, nil
}

func isPathWithinBase(baseDir, targetPath string) bool {
	baseClean := filepath.Clean(baseDir)
	targetClean := filepath.Clean(targetPath)
	if filepath.Separator == '\\' {
		baseClean = strings.ToLower(baseClean)
		targetClean = strings.ToLower(targetClean)
	}
	rel, err := filepath.Rel(baseClean, targetClean)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	return true
}

func sortableDateKey(ddmmyyyy string) string {
	parts := strings.Split(ddmmyyyy, "/")
	if len(parts) == 3 {
		return parts[2] + "-" + parts[1] + "-" + parts[0]
	}
	return ddmmyyyy
}
