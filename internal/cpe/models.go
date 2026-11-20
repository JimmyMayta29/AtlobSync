package cpe

import "time"

// ComprobanteCPE representa un comprobante electrónico individual vinculado a la empresa
type ComprobanteCPE struct {
	ID               int64     `json:"id"`
	EmpresaRUC       string    `json:"empresa_ruc"`
	Origen           string    `json:"origen"` // VENTAS o COMPRAS
	Periodo          string    `json:"periodo"`
	TipoCP           string    `json:"tipo_cp"` // 01, 03, 07, 08, etc.
	Serie            string    `json:"serie"`
	Numero           string    `json:"numero"`
	FechaEmision     string    `json:"fecha_emision"`
	RucContraparte   string    `json:"ruc_contraparte"`
	RazonContraparte string    `json:"razon_contraparte"`
	BaseImponible    float64   `json:"base_imponible"`
	IGV              float64   `json:"igv"`
	MontoTotal       float64   `json:"monto_total"`
	Moneda           string    `json:"moneda"`
	EstadoSUNAT         string    `json:"estado_sunat"`
	CarSUNAT            string    `json:"car_sunat"`
	ConcordanciaSIRE    string    `json:"concordancia_sire"`              // PENDIENTE, CONCORDANTE, DIFERENCIA, NO_EN_SUNAT
	DetalleConcordancia string    `json:"detalle_concordancia,omitempty"` // Detalle de validación XML vs SIRE
	ResumenItems        string    `json:"resumen_items,omitempty"`        // Vista rápida de ítems reales extraídos del XML
	VigenteSIRE         bool      `json:"vigente_sire"`                   // true si figura en la propuesta SIRE vigente
	RutaXML             string    `json:"-"`
	RutaCDR             string    `json:"-"`
	RutaPDF             string    `json:"-"`
	TieneXML            bool      `json:"tiene_xml"`
	TieneCDR            bool      `json:"tiene_cdr"`
	TienePDF            bool      `json:"tiene_pdf"`
	HashArchivo         string    `json:"hash_archivo,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

// ManualCPERequest permite registrar un comprobante individual manualmente (y opcionalmente consultar su CDR)
type ManualCPERequest struct {
	Origen           string  `json:"origen"` // VENTAS o COMPRAS
	Periodo          string  `json:"periodo"`
	RucContraparte   string  `json:"ruc_contraparte"`
	RazonContraparte string  `json:"razon_contraparte"`
	TipoCP           string  `json:"tipo_cp" binding:"required"`
	Serie            string  `json:"serie" binding:"required"`
	Numero           string  `json:"numero" binding:"required"`
	FechaEmision     string  `json:"fecha_emision"`
	Moneda           string  `json:"moneda"`
	BaseImponible    float64 `json:"base_imponible"`
	IGV              float64 `json:"igv"`
	MontoTotal       float64 `json:"monto_total"`
	ConsultarCDR     bool    `json:"consultar_cdr"`
}

// SyncResult resume una sincronización desde propuestas SIRE descargadas
type SyncResult struct {
	ArchivosProcesados int      `json:"archivos_procesados"`
	ComprobantesNuevos int      `json:"comprobantes_importados"`
	Advertencias       []string `json:"advertencias,omitempty"`
}

// ImportResult resume una carga/importación de archivos XML, PDF o CDR
type ImportResult struct {
	ArchivosGuardados int              `json:"archivos_guardados"`
	Comprobantes      []ComprobanteCPE `json:"comprobantes"`
	Advertencias      []string         `json:"advertencias,omitempty"`
}

// CdrConsultaResult resultado de la consulta SOAP getStatusCdr a SUNAT
type CdrConsultaResult struct {
	Comprobante   *ComprobanteCPE `json:"comprobante"`
	StatusCode    string          `json:"status_code"`
	StatusMessage string          `json:"status_message"`
	CdrDescargado bool            `json:"cdr_descargado"`
}

// SunatSingleDownloadResult resume la descarga directa de un comprobante desde SUNAT SOL (api-cpe)
type SunatSingleDownloadResult struct {
	Comprobante     *ComprobanteCPE `json:"comprobante"`
	XmlDescargado   bool            `json:"xml_descargado"`
	PdfDescargado   bool            `json:"pdf_descargado"`
	PdfGeneradoXML  bool            `json:"pdf_generado_xml"`
	CdrDescargado   bool            `json:"cdr_descargado"`
	Advertencias    []string        `json:"advertencias,omitempty"`
}

// BulkDownloadRequest define los parámetros de una descarga masiva de comprobantes desde SUNAT SOL
type BulkDownloadRequest struct {
	Origen             string   `json:"origen"`                // COMPRAS, VENTAS o TODOS (vacío = TODOS)
	Periodo            string   `json:"periodo"`               // YYYYMM opcional
	TiposCP            []string `json:"tipos_cp,omitempty"`    // Filtro opcional ej. ["01","07","08"] (vacío = todos)
	SoloVigentesSIRE   *bool    `json:"solo_vigentes_sire"`    // true por defecto: solo los de la propuesta SIRE vigente
	IDs                []int64  `json:"ids,omitempty"`         // Lista específica de IDs (opcional)
	Formatos           []string `json:"formatos"`              // ["xml","pdf","cdr","json"]
	SoloPendientes     *bool    `json:"solo_pendientes"`       // true por defecto: omite los ya descargados
	GenerarPDFDesdeXML *bool    `json:"generar_pdf_desde_xml"` // true por defecto: genera PDF+QR si SUNAT no entrega PDF
	Workers            int      `json:"workers"`               // 1..5 (default 3)
}

// BulkDownloadItemResult detalle del resultado de un comprobante dentro del lote
type BulkDownloadItemResult struct {
	ID          int64  `json:"id"`
	Comprobante string `json:"comprobante"`
	RucEmisor   string `json:"ruc_emisor"`
	XML         string `json:"xml,omitempty"` // ok, omitido, no_disponible, error
	PDF         string `json:"pdf,omitempty"` // ok, generado_xml, omitido, no_disponible, error
	CDR         string `json:"cdr,omitempty"` // ok, omitido, no_disponible, error
	Error       string `json:"error,omitempty"`
}

// BulkDownloadJob estado en vivo y resultado de un lote de descarga masiva
type BulkDownloadJob struct {
	ID             string                   `json:"id"`
	EmpresaRUC     string                   `json:"empresa_ruc"`
	Origen         string                   `json:"origen"`
	Periodo        string                   `json:"periodo"`
	Estado         string                   `json:"estado"` // PENDIENTE, AUTENTICANDO, EN_PROCESO, EMPAQUETANDO, COMPLETADO, ERROR, CANCELADO
	Mensaje        string                   `json:"mensaje"`
	Formatos       []string                 `json:"formatos"`
	Total          int                      `json:"total"`
	Procesados     int                      `json:"procesados"`
	Exitosos       int                      `json:"exitosos"`
	XmlDescargados int                      `json:"xml_descargados"`
	PdfDescargados int                      `json:"pdf_descargados"`
	PdfGenerados   int                      `json:"pdf_generados_xml"`
	CdrDescargados int                      `json:"cdr_descargados"`
	NoDisponibles  int                      `json:"no_disponibles"`
	Errores        int                      `json:"errores"`
	NombreZIP      string                   `json:"nombre_zip,omitempty"`
	RutaZIP        string                   `json:"-"`
	TieneZIP       bool                     `json:"tiene_zip"`
	IDsFallidos    []int64                  `json:"ids_fallidos,omitempty"`
	DetalleErrores []BulkDownloadItemResult `json:"detalle_errores,omitempty"`
	IniciadoEn     time.Time                `json:"iniciado_en"`
	ActualizadoEn  time.Time                `json:"actualizado_en"`
	FinalizadoEn   *time.Time               `json:"finalizado_en,omitempty"`
}

// ComprobanteJSON representación JSON estructurada de un comprobante (compatible con ERPs y contabilidad)
type ComprobanteJSON struct {
	EmpresaRUC       string       `json:"empresa_ruc"`
	Origen           string       `json:"origen"`
	Periodo          string       `json:"periodo"`
	TipoCP           string       `json:"tipo_cp"`
	TipoNombre       string       `json:"tipo_nombre"`
	Serie            string       `json:"serie"`
	Numero           string       `json:"numero"`
	FechaEmision     string       `json:"fecha_emision"`
	HoraEmision      string       `json:"hora_emision,omitempty"`
	FechaVencimiento string       `json:"fecha_vencimiento,omitempty"`
	Moneda           string       `json:"moneda"`
	RucEmisor        string       `json:"ruc_emisor"`
	RazonEmisor      string       `json:"razon_emisor"`
	DireccionEmisor  string       `json:"direccion_emisor,omitempty"`
	TipoDocCliente   string       `json:"tipo_doc_cliente,omitempty"`
	NumDocCliente    string       `json:"num_doc_cliente"`
	RazonCliente     string       `json:"razon_cliente"`
	DireccionCliente string       `json:"direccion_cliente,omitempty"`
	BaseImponible    float64      `json:"base_imponible"`
	OpGravada        float64      `json:"op_gravada,omitempty"`
	OpExonerada      float64      `json:"op_exonerada,omitempty"`
	OpInafecta       float64      `json:"op_inafecta,omitempty"`
	OpGratuita       float64      `json:"op_gratuita,omitempty"`
	IGV              float64      `json:"igv"`
	MontoTotal       float64      `json:"monto_total"`
	FormaPago        string       `json:"forma_pago,omitempty"`
	Cuotas           []UBLCuota   `json:"cuotas,omitempty"`
	Leyendas         []string     `json:"leyendas,omitempty"`
	DocModificadoRef string       `json:"doc_modificado_ref,omitempty"`
	MotivoNota       string       `json:"motivo_nota,omitempty"`
	EstadoSUNAT      string       `json:"estado_sunat"`
	CarSUNAT         string       `json:"car_sunat,omitempty"`
	HashXML          string       `json:"hash_xml,omitempty"`
	CodigoQR         string       `json:"codigo_qr,omitempty"`
	FuenteDatos      string       `json:"fuente_datos"` // "xml-ubl" o "sire-resumen"
	Items            []UBLItem    `json:"items,omitempty"`
}
