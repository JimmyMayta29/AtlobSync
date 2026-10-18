package company

import (
	"errors"
	"regexp"
	"time"
)

type Empresa struct {
	ID                  int64      `json:"id"`
	RUC                 string     `json:"ruc"`
	RazonSocial         string     `json:"razon_social"`
	UsuarioSOL          string     `json:"usuario_sol"`
	ClaveSOLEnc         string     `json:"-"`
	ClientID            string     `json:"client_id"`
	ClientSecretEnc     string     `json:"-"`
	ClientIDCPE         string     `json:"client_id_cpe"`
	ClientSecretCPEEnc  string     `json:"-"`
	EstadoCredenciales  string     `json:"estado_credenciales"`
	BloqueadoHasta      *time.Time `json:"bloqueado_hasta,omitempty"`
	MensajeAuth         string     `json:"mensaje_auth,omitempty"`
	Activo              int        `json:"activo"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty"`
}

type EmpresaResponse struct {
	ID                 int64      `json:"id"`
	RUC                string     `json:"ruc"`
	RazonSocial        string     `json:"razon_social"`
	UsuarioSOL         string     `json:"usuario_sol"`
	ClaveSOL           string     `json:"clave_sol"`        // Enmascarado siempre como ******
	ClientID           string     `json:"client_id"`
	ClientSecret       string     `json:"client_secret"`    // Enmascarado siempre como ******
	ClientIDCPE        string     `json:"client_id_cpe,omitempty"`
	ClientSecretCPE    string     `json:"client_secret_cpe,omitempty"`
	EstadoCredenciales string     `json:"estado_credenciales"`
	BloqueadoHasta     *time.Time `json:"bloqueado_hasta,omitempty"`
	MensajeAuth        string     `json:"mensaje_auth,omitempty"`
	Activo             bool       `json:"activo"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type CreateEmpresaRequest struct {
	RUC             string `json:"ruc" binding:"required"`
	RazonSocial     string `json:"razon_social" binding:"required"`
	UsuarioSOL      string `json:"usuario_sol" binding:"required"`
	ClaveSOL        string `json:"clave_sol" binding:"required"`
	ClientID        string `json:"client_id" binding:"required"`
	ClientSecret    string `json:"client_secret" binding:"required"`
	ClientIDCPE     string `json:"client_id_cpe"`
	ClientSecretCPE string `json:"client_secret_cpe"`
}

type UpdateEmpresaRequest struct {
	RazonSocial     string `json:"razon_social"`     // Opcional: si viene vacío, conserva la actual
	UsuarioSOL      string `json:"usuario_sol"`      // Opcional: si viene vacío, conserva la actual
	ClaveSOL        string `json:"clave_sol"`        // Opcional: si viene vacío, conserva la actual
	ClientID        string `json:"client_id"`        // Opcional: si viene vacío, conserva la actual
	ClientSecret    string `json:"client_secret"`    // Opcional: si viene vacío, conserva la actual
	ClientIDCPE     string `json:"client_id_cpe"`
	ClientSecretCPE string `json:"client_secret_cpe"`
	Activo          *bool  `json:"activo"`
}

var rucRegex = regexp.MustCompile(`^(10|20|15|17|16)\d{9}$`)

// ValidateRUC verifica que el RUC peruano tenga 11 dígitos y empiece con prefijo tributario válido
func ValidateRUC(ruc string) error {
	if !rucRegex.MatchString(ruc) {
		return errors.New("el RUC debe tener 11 dígitos numéricos y comenzar con 10, 20, 15, 17 o 16")
	}
	return nil
}
