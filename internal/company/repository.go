package company

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *sql.DB {
	return r.db
}

func (r *Repository) GetAll() ([]Empresa, error) {
	query := `
		SELECT id, ruc, razon_social, usuario_sol, clave_sol_enc, client_id, client_secret_enc,
		       coalesce(client_id_cpe, ''), coalesce(client_secret_cpe_enc, ''),
		       coalesce(estado_credenciales, 'PENDIENTE'), bloqueado_hasta, coalesce(mensaje_auth, ''),
		       activo, created_at, updated_at
		FROM empresas
		WHERE deleted_at IS NULL
		ORDER BY razon_social ASC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error al listar empresas: %w", err)
	}
	defer rows.Close()

	var lista []Empresa
	for rows.Next() {
		var e Empresa
		var createdAt, updatedAt string
		var bloqueadoHasta sql.NullString
		err := rows.Scan(
			&e.ID, &e.RUC, &e.RazonSocial, &e.UsuarioSOL, &e.ClaveSOLEnc, &e.ClientID, &e.ClientSecretEnc,
			&e.ClientIDCPE, &e.ClientSecretCPEEnc, &e.EstadoCredenciales, &bloqueadoHasta, &e.MensajeAuth,
			&e.Activo, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error al leer fila de empresa: %w", err)
		}
		if bloqueadoHasta.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", bloqueadoHasta.String)
			e.BloqueadoHasta = &t
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		e.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		lista = append(lista, e)
	}

	return lista, nil
}

func (r *Repository) GetByRUC(ruc string) (*Empresa, error) {
	query := `
		SELECT id, ruc, razon_social, usuario_sol, clave_sol_enc, client_id, client_secret_enc,
		       coalesce(client_id_cpe, ''), coalesce(client_secret_cpe_enc, ''),
		       coalesce(estado_credenciales, 'PENDIENTE'), bloqueado_hasta, coalesce(mensaje_auth, ''),
		       activo, created_at, updated_at
		FROM empresas
		WHERE ruc = ? AND deleted_at IS NULL
		LIMIT 1
	`
	var e Empresa
	var createdAt, updatedAt string
	var bloqueadoHasta sql.NullString
	err := r.db.QueryRow(query, ruc).Scan(
		&e.ID, &e.RUC, &e.RazonSocial, &e.UsuarioSOL, &e.ClaveSOLEnc, &e.ClientID, &e.ClientSecretEnc,
		&e.ClientIDCPE, &e.ClientSecretCPEEnc, &e.EstadoCredenciales, &bloqueadoHasta, &e.MensajeAuth,
		&e.Activo, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("empresa no encontrada")
		}
		return nil, fmt.Errorf("error al buscar empresa por RUC: %w", err)
	}
	if bloqueadoHasta.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", bloqueadoHasta.String)
		e.BloqueadoHasta = &t
	}
	e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	e.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &e, nil
}

func (r *Repository) Create(e *Empresa) error {
	query := `
		INSERT INTO empresas (
			ruc, razon_social, usuario_sol, clave_sol_enc, client_id, client_secret_enc,
			client_id_cpe, client_secret_cpe_enc, estado_credenciales, activo, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'PENDIENTE', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	res, err := r.db.Exec(query,
		e.RUC, e.RazonSocial, e.UsuarioSOL, e.ClaveSOLEnc, e.ClientID, e.ClientSecretEnc,
		e.ClientIDCPE, e.ClientSecretCPEEnc,
	)
	if err != nil {
		return fmt.Errorf("error al insertar empresa: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		e.ID = id
	}
	return nil
}

func (r *Repository) Update(e *Empresa) error {
	query := `
		UPDATE empresas
		SET razon_social = ?, usuario_sol = ?, clave_sol_enc = ?, client_id = ?, client_secret_enc = ?,
		    client_id_cpe = ?, client_secret_cpe_enc = ?,
		    estado_credenciales = 'PENDIENTE', bloqueado_hasta = NULL, mensaje_auth = '',
		    activo = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ruc = ? AND deleted_at IS NULL
	`
	_, err := r.db.Exec(query,
		e.RazonSocial, e.UsuarioSOL, e.ClaveSOLEnc, e.ClientID, e.ClientSecretEnc,
		e.ClientIDCPE, e.ClientSecretCPEEnc, e.Activo, e.RUC,
	)
	if err != nil {
		return fmt.Errorf("error al actualizar empresa: %w", err)
	}
	return nil
}

func (r *Repository) UpdateAuthStatus(ruc, estado, mensaje string, bloqueadoHasta *time.Time) error {
	query := `
		UPDATE empresas
		SET estado_credenciales = ?, mensaje_auth = ?, bloqueado_hasta = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ruc = ? AND deleted_at IS NULL
	`
	var bloqueadoStr *string
	if bloqueadoHasta != nil {
		s := bloqueadoHasta.Format("2006-01-02 15:04:05")
		bloqueadoStr = &s
	}

	_, err := r.db.Exec(query, estado, mensaje, bloqueadoStr, ruc)
	return err
}

func (r *Repository) SoftDelete(ruc string) error {
	query := `
		UPDATE empresas
		SET deleted_at = CURRENT_TIMESTAMP, activo = 0
		WHERE ruc = ? AND deleted_at IS NULL
	`
	res, err := r.db.Exec(query, ruc)
	if err != nil {
		return fmt.Errorf("error al eliminar empresa: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("empresa no encontrada o ya eliminada")
	}
	return nil
}
