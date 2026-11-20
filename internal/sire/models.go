package sire

import "time"

// EstadoDescarga define los estados posibles de un ticket/descarga en SIRE
type EstadoDescarga string

const (
	EstadoPendiente   EstadoDescarga = "PENDIENTE"
	EstadoEnProceso   EstadoDescarga = "EN_PROCESO"
	EstadoCompletado  EstadoDescarga = "COMPLETADO"
	EstadoError       EstadoDescarga = "ERROR"
)

// DescargaSire representa el registro persistido en SQLite para auditoría y descarga segura
type DescargaSire struct {
	ID            int64      `json:"id"`
	EmpresaRUC    string     `json:"empresa_ruc"`
	CodLibro      string     `json:"cod_libro"` // 140000 (RVIE) o 080000 (RCE)
	Libro         string     `json:"libro"`     // VENTAS o COMPRAS
	Periodo       string     `json:"periodo"`   // YYYYMM (ej. 202410)
	Ticket        string     `json:"ticket"`
	Estado        string     `json:"estado"` // PENDIENTE, EN_PROCESO, COMPLETADO, ERROR
	MensajeSUNAT  string     `json:"mensaje_sunat"`
	NombreArchivo string     `json:"nombre_archivo"`
	RutaLocal     string     `json:"ruta_local,omitempty"` // Ruta en disco (no expuesta al cliente por seguridad)
	TamanoBytes   int64      `json:"tamano_bytes"`
	HashArchivo   string     `json:"hash_archivo,omitempty"`
	Reintentos    int        `json:"reintentos"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// FiltrosExportacion representa los parámetros opcionales al solicitar exportación
type FiltrosExportacion struct {
	CodTipoArchivo        string `json:"cod_tipo_archivo"` // Anexo IV: 0 = TXT, 1 = Excel, 2 = CSV
	CodOrigenEnvio        string `json:"cod_origen_envio"` // Compras 5.34: 2 = Servicio API (Obligatorio)
	MtoTotalDesde         string `json:"mto_total_desde,omitempty"`
	MtoTotalHasta         string `json:"mto_total_hasta,omitempty"`
	FecDocumentoDesde     string `json:"fec_documento_desde,omitempty"`
	FecDocumentoHasta     string `json:"fec_documento_hasta,omitempty"`
	NumRucAdquiriente     string `json:"num_ruc_adquiriente,omitempty"`
	NumCarSunat           string `json:"num_car_sunat,omitempty"`
	CodTipoCDP            string `json:"cod_tipo_cdp,omitempty"`
	CodTipoInconsistencia string `json:"cod_tipo_inconsistencia,omitempty"`
}

// FiltrosPropuestaDetalle parámetros para consulta paginada de la propuesta
type FiltrosPropuestaDetalle struct {
	Page            string `form:"page"`
	PerPage         string `form:"perPage"`
	CodTipoCDP      string `form:"codTipoCDP"`
	NumSerieCDP     string `form:"numSerieCDP"`
	NumCDP          string `form:"numCDP"`
	FecEmisionIni   string `form:"fecEmisionIni"`
	FecEmisionFin   string `form:"fecEmisionFin"`
	NumDocIdentidad string `form:"numDocIdentidad"`
}

// TicketStatusSUNAT representa la respuesta de consulta de estado de ticket en SUNAT
type TicketStatusSUNAT struct {
	NumTicket            string `json:"numTicket"`
	PerTributario        string `json:"perTributario"`
	CodProceso           string `json:"codProceso"`
	DesProceso           string `json:"desProceso"`
	FecCargaImportacion  string `json:"fecCargaImportacion"`
	HoraCargaImportacion string `json:"horaCargaImportacion"`
	CodEstadoProceso     string `json:"codEstadoProceso"` // "06" = Terminado
	DesEstadoProceso     string `json:"desEstadoProceso"`
	ArchivoReporte       []struct {
		NomArchivoReporte     string `json:"nomArchivoReporte"`
		CodTipoArchivoReporte string `json:"codTipoArchivoReporte"`
		CodTipoAchivoReporte  string `json:"codTipoAchivoReporte"` // Typo real en API oficial SUNAT (sin 'r')
		NomArchivoContenido   string `json:"nomArchivoContenido"`
	} `json:"archivoReporte"`
}

// ConsultaEstadoTicketsResponse respuesta del endpoint de tickets masivos de SUNAT
type ConsultaEstadoTicketsResponse struct {
	Paginacion struct {
		Page           int `json:"page"`
		PerPage        int `json:"perPage"`
		TotalRegistros int `json:"totalRegistros"`
	} `json:"paginacion"`
	Registros []TicketStatusSUNAT `json:"registros"`
}
