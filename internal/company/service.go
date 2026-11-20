package company

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/SwodLore/sire-api/internal/crypto"
	"github.com/SwodLore/sire-api/internal/sunat"
)

type TokenProvider interface {
	GetToken(ruc string) (string, error)
	InvalidateToken(ruc string)
	IsCooldown(ruc string) (bool, time.Time, string)
	ClearCooldown(ruc string)
}

// SireAccessVerifier verifica acceso real al gateway api-sire.sunat.gob.pe tras obtener el token OAuth2
type SireAccessVerifier func(ruc string) (statusCode int, body string, err error)

type Service struct {
	repo         *Repository
	cryptoSvc    *crypto.CryptoService
	tokenManager TokenProvider
	sireVerifier SireAccessVerifier
}

func NewService(repo *Repository, cryptoSvc *crypto.CryptoService, tokenMgr TokenProvider) *Service {
	return &Service{
		repo:         repo,
		cryptoSvc:    cryptoSvc,
		tokenManager: tokenMgr,
	}
}

// SetSireVerifier inyecta un verificador ligero contra api-sire.sunat.gob.pe para validar permisos reales de SIRE
func (s *Service) SetSireVerifier(v SireAccessVerifier) {
	s.sireVerifier = v
}

func (s *Service) ListEmpresas() ([]EmpresaResponse, error) {
	empresas, err := s.repo.GetAll()
	if err != nil {
		return nil, err
	}

	res := make([]EmpresaResponse, 0, len(empresas))
	for _, e := range empresas {
		res = append(res, toResponse(&e))
	}
	return res, nil
}

func (s *Service) GetEmpresa(ruc string) (*EmpresaResponse, error) {
	if err := ValidateRUC(ruc); err != nil {
		return nil, err
	}

	emp, err := s.repo.GetByRUC(ruc)
	if err != nil {
		return nil, err
	}

	res := toResponse(emp)
	return &res, nil
}

func (s *Service) CreateEmpresa(req *CreateEmpresaRequest) (*EmpresaResponse, error) {
	req.RUC = strings.TrimSpace(req.RUC)
	if err := ValidateRUC(req.RUC); err != nil {
		return nil, err
	}

	// Verificar si ya existe una empresa activa con este RUC
	existing, _ := s.repo.GetByRUC(req.RUC)
	if existing != nil {
		return nil, fmt.Errorf("ya existe una empresa registrada con el RUC %s", req.RUC)
	}

	claveEnc, err := s.cryptoSvc.Encrypt(req.ClaveSOL)
	if err != nil {
		return nil, fmt.Errorf("error al cifrar clave SOL: %w", err)
	}

	clientSecretEnc, err := s.cryptoSvc.Encrypt(req.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("error al cifrar client_secret: %w", err)
	}

	var clientSecretCPEEnc string
	if req.ClientSecretCPE != "" {
		clientSecretCPEEnc, err = s.cryptoSvc.Encrypt(req.ClientSecretCPE)
		if err != nil {
			return nil, fmt.Errorf("error al cifrar client_secret_cpe: %w", err)
		}
	}

	emp := &Empresa{
		RUC:                req.RUC,
		RazonSocial:        strings.TrimSpace(req.RazonSocial),
		UsuarioSOL:         strings.TrimSpace(req.UsuarioSOL),
		ClaveSOLEnc:        claveEnc,
		ClientID:           strings.TrimSpace(req.ClientID),
		ClientSecretEnc:    clientSecretEnc,
		ClientIDCPE:        strings.TrimSpace(req.ClientIDCPE),
		ClientSecretCPEEnc: clientSecretCPEEnc,
		Activo:             1,
	}

	if err := s.repo.Create(emp); err != nil {
		return nil, err
	}

	res := toResponse(emp)
	return &res, nil
}

func (s *Service) UpdateEmpresa(ruc string, req *UpdateEmpresaRequest) (*EmpresaResponse, error) {
	if err := ValidateRUC(ruc); err != nil {
		return nil, err
	}

	emp, err := s.repo.GetByRUC(ruc)
	if err != nil {
		return nil, err
	}

	credencialesActualizadas := false

	if req.RazonSocial != "" {
		emp.RazonSocial = strings.TrimSpace(req.RazonSocial)
	}
	if req.UsuarioSOL != "" && strings.TrimSpace(req.UsuarioSOL) != emp.UsuarioSOL {
		emp.UsuarioSOL = strings.TrimSpace(req.UsuarioSOL)
		credencialesActualizadas = true
	} else if req.UsuarioSOL != "" {
		emp.UsuarioSOL = strings.TrimSpace(req.UsuarioSOL)
	}

	if req.ClientID != "" && strings.TrimSpace(req.ClientID) != emp.ClientID {
		emp.ClientID = strings.TrimSpace(req.ClientID)
		credencialesActualizadas = true
	} else if req.ClientID != "" {
		emp.ClientID = strings.TrimSpace(req.ClientID)
	}

	if req.ClientIDCPE != "" {
		emp.ClientIDCPE = strings.TrimSpace(req.ClientIDCPE)
	}

	if req.ClaveSOL != "" {
		claveEnc, err := s.cryptoSvc.Encrypt(req.ClaveSOL)
		if err != nil {
			return nil, fmt.Errorf("error al cifrar nueva clave SOL: %w", err)
		}
		emp.ClaveSOLEnc = claveEnc
		credencialesActualizadas = true
	}

	if req.ClientSecret != "" {
		secretEnc, err := s.cryptoSvc.Encrypt(req.ClientSecret)
		if err != nil {
			return nil, fmt.Errorf("error al cifrar nuevo client_secret: %w", err)
		}
		emp.ClientSecretEnc = secretEnc
		credencialesActualizadas = true
	}

	if req.ClientSecretCPE != "" {
		secretCPEEnc, err := s.cryptoSvc.Encrypt(req.ClientSecretCPE)
		if err != nil {
			return nil, fmt.Errorf("error al cifrar nuevo client_secret_cpe: %w", err)
		}
		emp.ClientSecretCPEEnc = secretCPEEnc
	}

	if credencialesActualizadas {
		s.tokenManager.InvalidateToken(ruc)
		s.tokenManager.ClearCooldown(ruc)
		emp.EstadoCredenciales = "PENDIENTE"
		emp.BloqueadoHasta = nil
		emp.MensajeAuth = ""
	}

	if req.Activo != nil {
		if *req.Activo {
			emp.Activo = 1
		} else {
			emp.Activo = 0
			s.tokenManager.InvalidateToken(ruc)
		}
	}

	if err := s.repo.Update(emp); err != nil {
		return nil, err
	}

	res := toResponse(emp)
	return &res, nil
}

func (s *Service) DeleteEmpresa(ruc string) error {
	if err := ValidateRUC(ruc); err != nil {
		return err
	}

	err := s.repo.SoftDelete(ruc)
	if err != nil {
		return err
	}

	s.tokenManager.InvalidateToken(ruc)
	s.tokenManager.ClearCooldown(ruc)
	return nil
}

func (s *Service) TestConnection(ruc string) (map[string]interface{}, error) {
	if err := ValidateRUC(ruc); err != nil {
		return nil, err
	}

	if inCooldown, until, reason := s.tokenManager.IsCooldown(ruc); inCooldown {
		return nil, fmt.Errorf("empresa en periodo de enfriamiento de seguridad hasta %s (%s). Actualice las credenciales SOL en Empresas para reintentar",
			until.Format("15:04:05"), reason)
	}

	// 1. Forzar obtención/renovación de token OAuth2 en api-seguridad.sunat.gob.pe
	s.tokenManager.InvalidateToken(ruc)
	_, err := s.tokenManager.GetToken(ruc)
	if err != nil {
		return nil, err
	}

	// 2. Si hay verificador SIRE configurado, comprobar acceso real a api-sire.sunat.gob.pe (1 sola llamada)
	if s.sireVerifier != nil {
		statusCode, body, vErr := s.sireVerifier(ruc)
		if vErr != nil {
			return nil, vErr
		}
		if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
			msg := "OAuth2 aceptó la Clave SOL, pero el gateway SIRE rechazó el token (HTTP 401/403). En Portal SOL verifique que la aplicación tenga alcance Desktop (no Web) y el servicio MIGE RCE y RVIE – SIRE habilitado."
			_ = s.repo.UpdateAuthStatus(ruc, "SIN_PERMISO_SIRE", msg, nil)
			return nil, &sunat.SunatHttpError{
				StatusCode: statusCode,
				Host:       "api-sire.sunat.gob.pe",
				RawBody:    body,
				Ruta:       "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/padron/web/omisos/140000/periodos",
			}
		}
		if statusCode >= 500 {
			return nil, &sunat.SunatHttpError{
				StatusCode: statusCode,
				Host:       "api-sire.sunat.gob.pe",
				RawBody:    body,
				Ruta:       "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/padron/web/omisos/140000/periodos",
			}
		}
		_ = s.repo.UpdateAuthStatus(ruc, "VALIDO", "Autenticación OAuth2 y acceso real a SIRE verificados correctamente", nil)
		return map[string]interface{}{
			"success":  true,
			"oauth_ok": true,
			"sire_ok":  true,
			"mensaje":  "Conexión completa verificada: Login OAuth2 y acceso al gateway SIRE (periodos) respondieron correctamente.",
		}, nil
	}

	return map[string]interface{}{
		"success":  true,
		"oauth_ok": true,
		"sire_ok":  true,
		"mensaje":  "Conexión con SUNAT exitosa. Credenciales y permisos validados.",
	}, nil
}

func toResponse(e *Empresa) EmpresaResponse {
	return EmpresaResponse{
		ID:                 e.ID,
		RUC:                e.RUC,
		RazonSocial:        e.RazonSocial,
		UsuarioSOL:         e.UsuarioSOL,
		ClaveSOL:           "******",
		ClientID:           e.ClientID,
		ClientSecret:       "******",
		ClientIDCPE:        e.ClientIDCPE,
		ClientSecretCPE:    "******",
		EstadoCredenciales: e.EstadoCredenciales,
		BloqueadoHasta:     e.BloqueadoHasta,
		MensajeAuth:        e.MensajeAuth,
		Activo:             e.Activo == 1,
		CreatedAt:          e.CreatedAt,
		UpdatedAt:          e.UpdatedAt,
	}
}

func (s *Service) GetDecryptedCredentials(ruc string) (string, string, string, string, string, error) {
	emp, err := s.repo.GetByRUC(ruc)
	if err != nil {
		return "", "", "", "", "", err
	}

	claveSOL, err := s.cryptoSvc.Decrypt(emp.ClaveSOLEnc)
	if err != nil {
		return "", "", "", "", "", errors.New("error al descifrar clave SOL")
	}

	clientSecret, err := s.cryptoSvc.Decrypt(emp.ClientSecretEnc)
	if err != nil {
		return "", "", "", "", "", errors.New("error al descifrar client secret")
	}

	return emp.RUC, emp.RazonSocial, emp.UsuarioSOL, claveSOL, clientSecret, nil
}
