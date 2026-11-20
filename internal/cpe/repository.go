package cpe

import (
	"database/sql"
	"fmt"
	"strings"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const cpeSelectColumns = `
	id, empresa_ruc, origen, COALESCE(periodo, ''), tipo_cp, serie, numero,
	COALESCE(fecha_emision, ''), COALESCE(ruc_contraparte, ''), COALESCE(razon_contraparte, ''),
	COALESCE(base_imponible, 0), COALESCE(igv, 0), COALESCE(monto_total, 0), COALESCE(moneda, 'PEN'),
	COALESCE(estado_sunat, 'DESCONOCIDO'), COALESCE(car_sunat, ''),
	COALESCE(concordancia_sire, 'PENDIENTE'), COALESCE(detalle_concordancia, ''),
	COALESCE(resumen_items, ''), COALESCE(vigente_sire, 1),
	COALESCE(ruta_xml, ''), COALESCE(ruta_cdr, ''), COALESCE(ruta_pdf, ''),
	COALESCE(hash_archivo, ''), created_at
`

// Upsert inserta o actualiza un comprobante preservando rutas de archivos (XML/CDR/PDF) ya adjuntadas
func (r *Repository) Upsert(c *ComprobanteCPE) (*ComprobanteCPE, error) {
	if c.Moneda == "" {
		c.Moneda = "PEN"
	}
	if c.EstadoSUNAT == "" {
		c.EstadoSUNAT = "EN_SIRE"
	}
	if c.Origen == "" {
		c.Origen = "VENTAS"
	}
	if c.ConcordanciaSIRE == "" {
		c.ConcordanciaSIRE = "PENDIENTE"
	}

	query := `
		INSERT INTO comprobantes_cpe (
			empresa_ruc, origen, periodo, tipo_cp, serie, numero,
			fecha_emision, ruc_contraparte, razon_contraparte,
			base_imponible, igv, monto_total, moneda,
			estado_sunat, car_sunat, concordancia_sire, detalle_concordancia, resumen_items, vigente_sire,
			ruta_xml, ruta_cdr, ruta_pdf, hash_archivo
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT(empresa_ruc, origen, ruc_contraparte, tipo_cp, serie, numero) DO UPDATE SET
			periodo = CASE WHEN excluded.periodo != '' THEN excluded.periodo ELSE comprobantes_cpe.periodo END,
			fecha_emision = CASE WHEN excluded.fecha_emision != '' THEN excluded.fecha_emision ELSE comprobantes_cpe.fecha_emision END,
			razon_contraparte = CASE WHEN excluded.razon_contraparte != '' THEN excluded.razon_contraparte ELSE comprobantes_cpe.razon_contraparte END,
			base_imponible = CASE WHEN excluded.base_imponible != 0 THEN excluded.base_imponible ELSE comprobantes_cpe.base_imponible END,
			igv = CASE WHEN excluded.igv != 0 THEN excluded.igv ELSE comprobantes_cpe.igv END,
			monto_total = CASE WHEN excluded.monto_total != 0 THEN excluded.monto_total ELSE comprobantes_cpe.monto_total END,
			moneda = CASE WHEN excluded.moneda != '' THEN excluded.moneda ELSE comprobantes_cpe.moneda END,
			estado_sunat = CASE WHEN excluded.estado_sunat != '' AND excluded.estado_sunat != 'DESCONOCIDO' THEN excluded.estado_sunat ELSE comprobantes_cpe.estado_sunat END,
			car_sunat = CASE WHEN excluded.car_sunat != '' THEN excluded.car_sunat ELSE comprobantes_cpe.car_sunat END,
			concordancia_sire = CASE WHEN excluded.concordancia_sire != '' AND excluded.concordancia_sire != 'PENDIENTE' THEN excluded.concordancia_sire ELSE comprobantes_cpe.concordancia_sire END,
			detalle_concordancia = CASE WHEN excluded.detalle_concordancia != '' THEN excluded.detalle_concordancia ELSE comprobantes_cpe.detalle_concordancia END,
			resumen_items = CASE WHEN excluded.resumen_items != '' THEN excluded.resumen_items ELSE comprobantes_cpe.resumen_items END,
			vigente_sire = 1,
			ruta_xml = CASE WHEN excluded.ruta_xml != '' THEN excluded.ruta_xml ELSE comprobantes_cpe.ruta_xml END,
			ruta_cdr = CASE WHEN excluded.ruta_cdr != '' THEN excluded.ruta_cdr ELSE comprobantes_cpe.ruta_cdr END,
			ruta_pdf = CASE WHEN excluded.ruta_pdf != '' THEN excluded.ruta_pdf ELSE comprobantes_cpe.ruta_pdf END,
			hash_archivo = CASE WHEN excluded.hash_archivo != '' THEN excluded.hash_archivo ELSE comprobantes_cpe.hash_archivo END
	`

	_, err := r.db.Exec(query,
		c.EmpresaRUC, c.Origen, c.Periodo, c.TipoCP, c.Serie, c.Numero,
		c.FechaEmision, c.RucContraparte, c.RazonContraparte,
		c.BaseImponible, c.IGV, c.MontoTotal, c.Moneda,
		c.EstadoSUNAT, c.CarSUNAT, c.ConcordanciaSIRE, c.DetalleConcordancia, c.ResumenItems,
		c.RutaXML, c.RutaCDR, c.RutaPDF, c.HashArchivo,
	)
	if err != nil {
		return nil, fmt.Errorf("error guardando comprobante CPE: %w", err)
	}

	return r.GetByKey(c.EmpresaRUC, c.Origen, c.RucContraparte, c.TipoCP, c.Serie, c.Numero)
}

// UpsertBatch inserta o actualiza múltiples comprobantes dentro de una sola transacción SQLite
func (r *Repository) UpsertBatch(items []ComprobanteCPE) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error iniciando transacción CPE: %w", err)
	}
	count, err := execUpsertBatchTx(tx, items)
	if err != nil {
		_ = tx.Rollback()
		return count, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("error confirmando transacción batch CPE: %w", err)
	}
	return count, nil
}

// SyncProposalSnapshot sincroniza un periodo completo contra la propuesta SIRE vigente:
// marca previamente los registros del periodo como no vigentes (vigente_sire=0) y reactiva/actualiza
// únicamente los que figuran en la propuesta vigente (vigente_sire=1), preservando XML/PDF/CDR ya descargados.
func (r *Repository) SyncProposalSnapshot(ruc, origen, periodo string, items []ComprobanteCPE) (vigentes int, fueraPropuesta int, err error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("error iniciando transacción snapshot SIRE: %w", err)
	}

	if periodo != "" {
		_, err = tx.Exec(`
			UPDATE comprobantes_cpe
			SET vigente_sire = 0
			WHERE empresa_ruc = ? AND UPPER(origen) = UPPER(?) AND periodo = ? AND estado_sunat != 'IMPORTADO'
		`, ruc, origen, periodo)
		if err != nil {
			_ = tx.Rollback()
			return 0, 0, fmt.Errorf("error marcando vigencia previa SIRE: %w", err)
		}
	}

	vigentes, err = execUpsertBatchTx(tx, items)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}

	if periodo != "" {
		_ = tx.QueryRow(`
			SELECT COUNT(1) FROM comprobantes_cpe
			WHERE empresa_ruc = ? AND UPPER(origen) = UPPER(?) AND periodo = ? AND vigente_sire = 0
		`, ruc, origen, periodo).Scan(&fueraPropuesta)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("error confirmando snapshot SIRE: %w", err)
	}
	return vigentes, fueraPropuesta, nil
}

func execUpsertBatchTx(tx *sql.Tx, items []ComprobanteCPE) (int, error) {
	stmt, err := tx.Prepare(`
		INSERT INTO comprobantes_cpe (
			empresa_ruc, origen, periodo, tipo_cp, serie, numero,
			fecha_emision, ruc_contraparte, razon_contraparte,
			base_imponible, igv, monto_total, moneda,
			estado_sunat, car_sunat, vigente_sire, ruta_xml, ruta_cdr, ruta_pdf, hash_archivo
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT(empresa_ruc, origen, ruc_contraparte, tipo_cp, serie, numero) DO UPDATE SET
			periodo = CASE WHEN excluded.periodo != '' THEN excluded.periodo ELSE comprobantes_cpe.periodo END,
			fecha_emision = CASE WHEN excluded.fecha_emision != '' THEN excluded.fecha_emision ELSE comprobantes_cpe.fecha_emision END,
			razon_contraparte = CASE WHEN excluded.razon_contraparte != '' THEN excluded.razon_contraparte ELSE comprobantes_cpe.razon_contraparte END,
			base_imponible = CASE WHEN excluded.base_imponible != 0 THEN excluded.base_imponible ELSE comprobantes_cpe.base_imponible END,
			igv = CASE WHEN excluded.igv != 0 THEN excluded.igv ELSE comprobantes_cpe.igv END,
			monto_total = CASE WHEN excluded.monto_total != 0 THEN excluded.monto_total ELSE comprobantes_cpe.monto_total END,
			moneda = CASE WHEN excluded.moneda != '' THEN excluded.moneda ELSE comprobantes_cpe.moneda END,
			estado_sunat = CASE WHEN excluded.estado_sunat != '' AND excluded.estado_sunat != 'DESCONOCIDO' THEN excluded.estado_sunat ELSE comprobantes_cpe.estado_sunat END,
			car_sunat = CASE WHEN excluded.car_sunat != '' THEN excluded.car_sunat ELSE comprobantes_cpe.car_sunat END,
			vigente_sire = 1,
			ruta_xml = CASE WHEN excluded.ruta_xml != '' THEN excluded.ruta_xml ELSE comprobantes_cpe.ruta_xml END,
			ruta_cdr = CASE WHEN excluded.ruta_cdr != '' THEN excluded.ruta_cdr ELSE comprobantes_cpe.ruta_cdr END,
			ruta_pdf = CASE WHEN excluded.ruta_pdf != '' THEN excluded.ruta_pdf ELSE comprobantes_cpe.ruta_pdf END,
			hash_archivo = CASE WHEN excluded.hash_archivo != '' THEN excluded.hash_archivo ELSE comprobantes_cpe.hash_archivo END
	`)
	if err != nil {
		return 0, fmt.Errorf("error preparando sentencia batch CPE: %w", err)
	}
	defer stmt.Close()

	count := 0
	for i := range items {
		c := items[i]
		if c.Moneda == "" {
			c.Moneda = "PEN"
		}
		if c.EstadoSUNAT == "" {
			c.EstadoSUNAT = "EN_SIRE"
		}
		if c.Origen == "" {
			c.Origen = "VENTAS"
		}
		if _, execErr := stmt.Exec(
			c.EmpresaRUC, c.Origen, c.Periodo, c.TipoCP, c.Serie, c.Numero,
			c.FechaEmision, c.RucContraparte, c.RazonContraparte,
			c.BaseImponible, c.IGV, c.MontoTotal, c.Moneda,
			c.EstadoSUNAT, c.CarSUNAT, c.RutaXML, c.RutaCDR, c.RutaPDF, c.HashArchivo,
		); execErr != nil {
			return count, fmt.Errorf("error guardando comprobante %s-%s-%s en batch: %w", c.TipoCP, c.Serie, c.Numero, execErr)
		}
		count++
	}
	return count, nil
}

func (r *Repository) GetByKey(ruc, origen, rucContraparte, tipoCP, serie, numero string) (*ComprobanteCPE, error) {
	query := "SELECT " + cpeSelectColumns + `
		FROM comprobantes_cpe
		WHERE empresa_ruc = ? AND origen = ? AND COALESCE(ruc_contraparte, '') = ? AND tipo_cp = ? AND serie = ? AND numero = ?
	`
	return scanCPE(r.db.QueryRow(query, ruc, origen, rucContraparte, tipoCP, serie, numero))
}

func (r *Repository) GetByID(id int64, ruc string) (*ComprobanteCPE, error) {
	query := "SELECT " + cpeSelectColumns + `
		FROM comprobantes_cpe
		WHERE id = ? AND empresa_ruc = ?
	`
	return scanCPE(r.db.QueryRow(query, id, ruc))
}

func (r *Repository) List(ruc, origen, periodo, search string) ([]ComprobanteCPE, error) {
	return r.ListFiltered(ruc, origen, periodo, "", search, true)
}

// ListFiltered lista comprobantes con filtro de origen, periodo, tipo de comprobante, búsqueda y vigencia SIRE.
func (r *Repository) ListFiltered(ruc, origen, periodo, tipoCP, search string, soloVigentes bool) ([]ComprobanteCPE, error) {
	query := "SELECT " + cpeSelectColumns + " FROM comprobantes_cpe WHERE empresa_ruc = ?"
	args := []interface{}{ruc}

	if soloVigentes {
		query += " AND COALESCE(vigente_sire, 1) = 1"
	}
	if o := strings.ToUpper(strings.TrimSpace(origen)); o != "" && o != "TODOS" {
		query += " AND UPPER(origen) = ?"
		args = append(args, o)
	}
	if p := strings.TrimSpace(periodo); p != "" {
		query += " AND periodo = ?"
		args = append(args, p)
	}
	if t := strings.ToUpper(strings.TrimSpace(tipoCP)); t != "" && t != "TODOS" {
		if t == "FACTURAS_NOTAS" {
			query += " AND tipo_cp IN ('01','07','08')"
		} else {
			query += " AND tipo_cp = ?"
			args = append(args, padTipoCP(t))
		}
	}
	if q := strings.TrimSpace(search); q != "" {
		like := "%" + q + "%"
		query += " AND (serie LIKE ? OR numero LIKE ? OR ruc_contraparte LIKE ? OR razon_contraparte LIKE ? OR car_sunat LIKE ? OR resumen_items LIKE ?)"
		args = append(args, like, like, like, like, like, like)
	}

	// Priorizar Facturas/Notas y archivos ya descargados para facilitar el trabajo diario
	query += " ORDER BY CASE WHEN tipo_cp IN ('01','07','08') THEN 0 ELSE 1 END, id DESC LIMIT 1000"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error al listar comprobantes: %w", err)
	}
	defer rows.Close()

	list := make([]ComprobanteCPE, 0)
	for rows.Next() {
		item, err := scanCPE(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, nil
}

// ListForBulk obtiene todos los comprobantes de un periodo/origen/tipos o una lista explícita de IDs (sin límite de filas)
func (r *Repository) ListForBulk(ruc, origen, periodo string, tiposCP []string, soloVigentes bool, ids []int64) ([]ComprobanteCPE, error) {
	query := "SELECT " + cpeSelectColumns + " FROM comprobantes_cpe WHERE empresa_ruc = ?"
	args := []interface{}{ruc}

	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		for i, id := range ids {
			placeholders[i] = "?"
			args = append(args, id)
		}
		query += " AND id IN (" + strings.Join(placeholders, ",") + ")"
	} else {
		if soloVigentes {
			query += " AND COALESCE(vigente_sire, 1) = 1"
		}
		if o := strings.ToUpper(strings.TrimSpace(origen)); o != "" && o != "TODOS" {
			query += " AND UPPER(origen) = ?"
			args = append(args, o)
		}
		if p := strings.TrimSpace(periodo); p != "" {
			query += " AND periodo = ?"
			args = append(args, p)
		}
		if len(tiposCP) > 0 {
			ph := make([]string, 0, len(tiposCP))
			for _, t := range tiposCP {
				t = padTipoCP(t)
				if t != "" {
					ph = append(ph, "?")
					args = append(args, t)
				}
			}
			if len(ph) > 0 {
				query += " AND tipo_cp IN (" + strings.Join(ph, ",") + ")"
			}
		}
	}

	query += " ORDER BY CASE WHEN tipo_cp IN ('01','07','08') THEN 0 ELSE 1 END, id ASC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error listando comprobantes para descarga masiva: %w", err)
	}
	defer rows.Close()

	list := make([]ComprobanteCPE, 0)
	for rows.Next() {
		item, err := scanCPE(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, nil
}

func (r *Repository) UpdateFilePath(id int64, ruc, tipoArchivo, path, estadoSunat, hashHex string) error {
	var col string
	switch strings.ToLower(tipoArchivo) {
	case "xml":
		col = "ruta_xml"
	case "cdr":
		col = "ruta_cdr"
	case "pdf":
		col = "ruta_pdf"
	default:
		return fmt.Errorf("tipo de archivo no soportado: %s", tipoArchivo)
	}

	query := fmt.Sprintf("UPDATE comprobantes_cpe SET %s = ?", col)
	args := []interface{}{path}
	if estadoSunat != "" {
		query += ", estado_sunat = ?"
		args = append(args, estadoSunat)
	}
	if hashHex != "" {
		query += ", hash_archivo = ?"
		args = append(args, hashHex)
	}
	query += " WHERE id = ? AND empresa_ruc = ?"
	args = append(args, id, ruc)

	_, err := r.db.Exec(query, args...)
	return err
}

// UpdateConcordancia guarda el resultado de comparar el XML original de SUNAT contra la propuesta SIRE
// junto con el resumen de los ítems/descripciones reales del comprobante.
func (r *Repository) UpdateConcordancia(id int64, ruc, concordancia, detalle, resumenItems string) error {
	_, err := r.db.Exec(`
		UPDATE comprobantes_cpe
		SET concordancia_sire = ?,
		    detalle_concordancia = ?,
		    resumen_items = CASE WHEN ? != '' THEN ? ELSE resumen_items END
		WHERE id = ? AND empresa_ruc = ?
	`, concordancia, detalle, resumenItems, resumenItems, id, ruc)
	return err
}

func scanCPE(scanner interface {
	Scan(dest ...interface{}) error
}) (*ComprobanteCPE, error) {
	var c ComprobanteCPE
	var vigenteInt int
	err := scanner.Scan(
		&c.ID, &c.EmpresaRUC, &c.Origen, &c.Periodo, &c.TipoCP, &c.Serie, &c.Numero,
		&c.FechaEmision, &c.RucContraparte, &c.RazonContraparte,
		&c.BaseImponible, &c.IGV, &c.MontoTotal, &c.Moneda,
		&c.EstadoSUNAT, &c.CarSUNAT,
		&c.ConcordanciaSIRE, &c.DetalleConcordancia, &c.ResumenItems, &vigenteInt,
		&c.RutaXML, &c.RutaCDR, &c.RutaPDF,
		&c.HashArchivo, &c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.VigenteSIRE = vigenteInt != 0
	c.TieneXML = strings.TrimSpace(c.RutaXML) != ""
	c.TieneCDR = strings.TrimSpace(c.RutaCDR) != ""
	c.TienePDF = strings.TrimSpace(c.RutaPDF) != ""
	return &c, nil
}
