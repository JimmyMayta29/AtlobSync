package rhe

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// SavePeriodoResults reemplaza de forma atómica los recibos vigentes de un (empresa_ruc, periodo)
// y registra los metadatos de la consulta en consultas_rhe.
func (r *Repository) SavePeriodoResults(empresaRUC, periodo, fechaDesde, fechaHasta, rutaTXT string, recibos []ReciboHonorario) error {
	empresaRUC = strings.TrimSpace(empresaRUC)
	periodo = strings.TrimSpace(periodo)

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("error iniciando transacción RHE: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Marcar registros previos del mismo periodo como no vigentes antes de insertar/actualizar la instantánea actual
	if _, err := tx.Exec(`
		UPDATE recibos_honorarios
		SET vigente = 0, updated_at = CURRENT_TIMESTAMP
		WHERE empresa_ruc = ? AND periodo = ?
	`, empresaRUC, periodo); err != nil {
		return fmt.Errorf("error limpiando vigencia previa RHE: %w", err)
	}

	upsertStmt, err := tx.Prepare(`
		INSERT INTO recibos_honorarios (
			empresa_ruc, periodo, fecha_emision, tipo_doc, nro_doc, serie, numero,
			estado, tipo_doc_emisor, nro_doc_emisor, razon_emisor,
			tipo_renta, gratuito, descripcion, observacion, moneda,
			renta_bruta, impuesto_renta, renta_neta, monto_pendiente,
			vigente, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?,
			1, CURRENT_TIMESTAMP
		)
		ON CONFLICT(empresa_ruc, nro_doc_emisor, tipo_doc, nro_doc) DO UPDATE SET
			periodo         = excluded.periodo,
			fecha_emision   = excluded.fecha_emision,
			serie           = excluded.serie,
			numero          = excluded.numero,
			estado          = excluded.estado,
			tipo_doc_emisor = excluded.tipo_doc_emisor,
			razon_emisor    = excluded.razon_emisor,
			tipo_renta      = excluded.tipo_renta,
			gratuito        = excluded.gratuito,
			descripcion     = excluded.descripcion,
			observacion     = excluded.observacion,
			moneda          = excluded.moneda,
			renta_bruta     = excluded.renta_bruta,
			impuesto_renta  = excluded.impuesto_renta,
			renta_neta      = excluded.renta_neta,
			monto_pendiente = excluded.monto_pendiente,
			vigente         = 1,
			updated_at      = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return fmt.Errorf("error preparando upsert RHE: %w", err)
	}
	defer upsertStmt.Close()

	for _, rec := range recibos {
		if _, err := upsertStmt.Exec(
			empresaRUC, periodo, rec.FechaEmision, rec.TipoDoc, rec.NroDoc, rec.Serie, rec.Numero,
			rec.Estado, rec.TipoDocEmisor, rec.NroDocEmisor, rec.RazonEmisor,
			rec.TipoRenta, rec.Gratuito, rec.Descripcion, rec.Observacion, rec.Moneda,
			rec.RentaBruta, rec.ImpuestoRenta, rec.RentaNeta, rec.MontoPendiente,
		); err != nil {
			return fmt.Errorf("error guardando RHE %s (%s): %w", rec.NroDoc, rec.NroDocEmisor, err)
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO consultas_rhe (empresa_ruc, periodo, fecha_desde, fecha_hasta, total_registros, ruta_txt, consultado_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(empresa_ruc, periodo) DO UPDATE SET
			fecha_desde     = excluded.fecha_desde,
			fecha_hasta     = excluded.fecha_hasta,
			total_registros = excluded.total_registros,
			ruta_txt        = excluded.ruta_txt,
			consultado_at   = CURRENT_TIMESTAMP
	`, empresaRUC, periodo, fechaDesde, fechaHasta, len(recibos), rutaTXT); err != nil {
		return fmt.Errorf("error guardando metadatos de consulta RHE: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error confirmando transacción RHE: %w", err)
	}
	tx = nil
	return nil
}

// List retorna los recibos vigentes almacenados en SQLite aplicando los filtros solicitados.
func (r *Repository) List(empresaRUC string, filter RHEListFilter) ([]ReciboHonorario, error) {
	where := []string{"empresa_ruc = ?", "vigente = 1"}
	args := []any{strings.TrimSpace(empresaRUC)}

	if p := strings.TrimSpace(filter.Periodo); p != "" {
		where = append(where, "periodo = ?")
		args = append(args, p)
	}

	if mon := strings.ToUpper(strings.TrimSpace(filter.Moneda)); mon != "" && mon != "TODOS" {
		where = append(where, "UPPER(moneda) = ?")
		args = append(args, mon)
	}

	if est := strings.ToUpper(strings.TrimSpace(filter.Estado)); est != "" && est != "TODOS" {
		switch est {
		case "ACTIVO", "NO_ANULADO", "NO ANULADO":
			where = append(where, "(UPPER(estado) LIKE '%NO ANULADO%' OR (UPPER(estado) NOT LIKE '%ANULADO%' AND UPPER(estado) NOT LIKE '%REVERTIDO%' AND UPPER(estado) NOT LIKE '%BAJA%'))")
		case "ANULADO", "REVERTIDO", "ANULADO_REVERTIDO":
			where = append(where, "(UPPER(estado) NOT LIKE '%NO ANULADO%' AND (UPPER(estado) LIKE '%ANULADO%' OR UPPER(estado) LIKE '%REVERTIDO%' OR UPPER(estado) LIKE '%BAJA%'))")
		}
	}

	if q := strings.TrimSpace(filter.Query); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		where = append(where, "(LOWER(nro_doc_emisor) LIKE ? OR LOWER(razon_emisor) LIKE ? OR LOWER(nro_doc) LIKE ? OR LOWER(descripcion) LIKE ? OR LOWER(observacion) LIKE ?)")
		args = append(args, like, like, like, like, like)
	}

	query := fmt.Sprintf(`
		SELECT
			id, empresa_ruc, periodo, fecha_emision, tipo_doc, nro_doc, serie, numero,
			estado, tipo_doc_emisor, nro_doc_emisor, razon_emisor,
			tipo_renta, gratuito, descripcion, observacion, moneda,
			renta_bruta, impuesto_renta, renta_neta, monto_pendiente,
			vigente, created_at, updated_at
		FROM recibos_honorarios
		WHERE %s
		ORDER BY
			substr(fecha_emision, 7, 4) || '-' || substr(fecha_emision, 4, 2) || '-' || substr(fecha_emision, 1, 2) DESC,
			id DESC
	`, strings.Join(where, " AND "))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error listando RHE: %w", err)
	}
	defer rows.Close()

	list := []ReciboHonorario{}
	for rows.Next() {
		var rec ReciboHonorario
		var vig int
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(
			&rec.ID, &rec.EmpresaRUC, &rec.Periodo, &rec.FechaEmision, &rec.TipoDoc, &rec.NroDoc, &rec.Serie, &rec.Numero,
			&rec.Estado, &rec.TipoDocEmisor, &rec.NroDocEmisor, &rec.RazonEmisor,
			&rec.TipoRenta, &rec.Gratuito, &rec.Descripcion, &rec.Observacion, &rec.Moneda,
			&rec.RentaBruta, &rec.ImpuestoRenta, &rec.RentaNeta, &rec.MontoPendiente,
			&vig, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("error escaneando fila RHE: %w", err)
		}
		rec.Vigente = vig == 1
		rec.EsAnulado = IsAnuladoORevertido(rec.Estado)
		if createdAt.Valid {
			rec.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			rec.UpdatedAt = updatedAt.Time
		}
		list = append(list, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// GetConsultaMeta retorna los metadatos de la última consulta SUNAT para empresa + periodo (o nil si nunca se consultó).
func (r *Repository) GetConsultaMeta(empresaRUC, periodo string) (*ConsultaRHEMeta, error) {
	empresaRUC = strings.TrimSpace(empresaRUC)
	periodo = strings.TrimSpace(periodo)
	if periodo == "" {
		return nil, nil
	}

	var meta ConsultaRHEMeta
	var consultadoAt sql.NullTime
	err := r.db.QueryRow(`
		SELECT empresa_ruc, periodo, fecha_desde, fecha_hasta, total_registros, ruta_txt, consultado_at
		FROM consultas_rhe
		WHERE empresa_ruc = ? AND periodo = ?
	`, empresaRUC, periodo).Scan(
		&meta.EmpresaRUC, &meta.Periodo, &meta.FechaDesde, &meta.FechaHasta,
		&meta.TotalRegistros, &meta.RutaTXT, &consultadoAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error consultando metadatos RHE: %w", err)
	}
	if consultadoAt.Valid {
		meta.ConsultadoAt = consultadoAt.Time
	} else {
		meta.ConsultadoAt = time.Now()
	}
	return &meta, nil
}

// CountByEmpresa cuenta el total de RHE vigentes guardados para la empresa (badge de navegación).
func (r *Repository) CountByEmpresa(empresaRUC string) (int, error) {
	var total int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM recibos_honorarios WHERE empresa_ruc = ? AND vigente = 1
	`, strings.TrimSpace(empresaRUC)).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}
