package rhe

import (
	"context"
	"strings"
	"time"
)

// ReciboHonorario representa un Recibo por Honorarios Electrónico (o Nota de Crédito / Otro Ingreso de 4.ª Categoría)
// consultado en el Portal SOL de SUNAT bajo el rol de Receptor (empresa que recibe el servicio).
type ReciboHonorario struct {
	ID             int64     `json:"id"`
	EmpresaRUC     string    `json:"empresa_ruc"`
	Periodo        string    `json:"periodo"`
	FechaEmision   string    `json:"fecha_emision"`
	TipoDoc        string    `json:"tipo_doc"`
	NroDoc         string    `json:"nro_doc"`
	Serie          string    `json:"serie"`
	Numero         string    `json:"numero"`
	Estado         string    `json:"estado"`
	TipoDocEmisor  string    `json:"tipo_doc_emisor"`
	NroDocEmisor   string    `json:"nro_doc_emisor"`
	RazonEmisor    string    `json:"razon_emisor"`
	TipoRenta      string    `json:"tipo_renta"`
	Gratuito       string    `json:"gratuito"`
	Descripcion    string    `json:"descripcion"`
	Observacion    string    `json:"observacion"`
	Moneda         string    `json:"moneda"`
	RentaBruta     float64   `json:"renta_bruta"`
	ImpuestoRenta  float64   `json:"impuesto_renta"`
	RentaNeta      float64   `json:"renta_neta"`
	MontoPendiente float64   `json:"monto_pendiente"`
	Vigente        bool      `json:"vigente"`
	EsAnulado      bool      `json:"es_anulado"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// IsAnuladoORevertido evalúa si el estado reportado por SUNAT corresponde a un recibo anulado o revertido.
func IsAnuladoORevertido(estado string) bool {
	up := strings.ToUpper(strings.TrimSpace(estado))
	if up == "" || strings.Contains(up, "NO ANULADO") {
		return false
	}
	return strings.Contains(up, "ANULADO") || strings.Contains(up, "REVERTIDO") || strings.Contains(up, "BAJA")
}

// ConsultaRHEMeta guarda los metadatos de la última consulta RHE ejecutada contra SUNAT SOL para una empresa y periodo.
type ConsultaRHEMeta struct {
	EmpresaRUC     string    `json:"empresa_ruc"`
	Periodo        string    `json:"periodo"`
	FechaDesde     string    `json:"fecha_desde"`
	FechaHasta     string    `json:"fecha_hasta"`
	TotalRegistros int       `json:"total_registros"`
	RutaTXT        string    `json:"ruta_txt,omitempty"`
	ConsultadoAt   time.Time `json:"consultado_at"`
}

// RHEMonedaResumen desglosa totales activos por moneda de emisión (ej. SOLES, DOLARES).
type RHEMonedaResumen struct {
	Moneda         string  `json:"moneda"`
	Cantidad       int     `json:"cantidad"`
	RentaBruta     float64 `json:"renta_bruta"`
	ImpuestoRenta  float64 `json:"impuesto_renta"`
	RentaNeta      float64 `json:"renta_neta"`
	MontoPendiente float64 `json:"monto_pendiente"`
}

// RHEResumenKPI consolida conteos y montos oficiales (excluyendo anulados/revertidos).
type RHEResumenKPI struct {
	TotalRegistros      int                `json:"total_registros"`
	Activos             int                `json:"activos"`
	AnuladosRevertidos  int                `json:"anulados_revertidos"`
	ConRetencion        int                `json:"con_retencion"`
	SinRetencion        int                `json:"sin_retencion"`
	TotalRentaBruta     float64            `json:"total_renta_bruta"`
	TotalImpuestoRenta  float64            `json:"total_impuesto_renta"`
	TotalRentaNeta      float64            `json:"total_renta_neta"`
	TotalMontoPendiente float64            `json:"total_monto_pendiente"`
	PorMoneda           []RHEMonedaResumen `json:"por_moneda"`
}

// RHEListFilter agrupa los criterios de filtrado local sobre SQLite.
type RHEListFilter struct {
	Periodo string
	Estado  string // TODOS | ACTIVO | ANULADO
	Moneda  string
	Query   string
}

// SolRHEStatus describe el estado local del cliente SOL RHE (sin contactar a SUNAT).
type SolRHEStatus struct {
	BrowserEncontrado bool   `json:"browser_encontrado"`
	BrowserNombre     string `json:"browser_nombre,omitempty"`
	EnCooldown        bool   `json:"en_cooldown"`
	CooldownHasta     string `json:"cooldown_hasta,omitempty"`
	CooldownMotivo    string `json:"cooldown_motivo,omitempty"`
	Estado            string `json:"estado"`
	Mensaje           string `json:"mensaje"`
}

// ReceptorQueryResult contiene la respuesta cruda obtenida desde SUNAT SOL (Consulta Receptor 11.5.1.1.14).
type ReceptorQueryResult struct {
	RawTXT     []byte
	FileName   string
	FechaDesde string
	FechaHasta string
	Empty      bool
	Notice     string
}

// ReceptorProvider abstrae la consulta de Recibos por Honorarios como Receptor (implementado en modo local vía Portal SOL).
type ReceptorProvider interface {
	ConsultarPeriodo(ctx context.Context, ruc, usuarioSOL, claveSOL, periodo string) (*ReceptorQueryResult, error)
	Status(ruc string) SolRHEStatus
}

// CredentialsProvider obtiene las credenciales SOL descifradas de la empresa (implementado por *company.Service).
type CredentialsProvider interface {
	GetDecryptedCredentials(ruc string) (rucOut, razonSocial, usuarioSOL, claveSOL, clientSecret string, err error)
}

// RHEListaResponse es la respuesta estructurada para GET /api/empresas/:ruc/rhe.
type RHEListaResponse struct {
	EmpresaRUC     string            `json:"empresa_ruc"`
	Periodo        string            `json:"periodo"`
	Rol            string            `json:"rol"`
	Modo           string            `json:"modo"`
	TotalEmpresa   int               `json:"total_empresa"`
	Total          int               `json:"total"`
	Recibos        []ReciboHonorario `json:"recibos"`
	Resumen        RHEResumenKPI     `json:"resumen"`
	UltimaConsulta *ConsultaRHEMeta  `json:"ultima_consulta,omitempty"`
	EstadoSOL      SolRHEStatus      `json:"estado_sol"`
}

// RHEConsultaResponse es la respuesta estructurada para POST /api/empresas/:ruc/rhe/consultar.
type RHEConsultaResponse struct {
	EmpresaRUC     string            `json:"empresa_ruc"`
	Periodo        string            `json:"periodo"`
	FechaDesde     string            `json:"fecha_desde"`
	FechaHasta     string            `json:"fecha_hasta"`
	Rol            string            `json:"rol"`
	Modo           string            `json:"modo"`
	Mensaje        string            `json:"mensaje"`
	Total          int               `json:"total"`
	Recibos        []ReciboHonorario `json:"recibos"`
	Resumen        RHEResumenKPI     `json:"resumen"`
	UltimaConsulta *ConsultaRHEMeta  `json:"ultima_consulta,omitempty"`
}

// ExportedRHEFile encapsula un archivo exportado listo para descarga HTTP (.xlsx o .txt).
type ExportedRHEFile struct {
	Data        []byte
	FileName    string
	ContentType string
}
