package license

import (
	"net/url"
	"strings"
	"time"
)

const (
	CodeActive          = "ACTIVE"
	CodeExpiringSoon    = "EXPIRING_SOON"
	CodeUnactivated     = "UNACTIVATED"
	CodeExpired         = "EXPIRED"
	CodeRevoked         = "REVOKED"
	CodeSuspended       = "SUSPENDED"
	CodeMachineMismatch = "MACHINE_MISMATCH"
	CodeOfflineLocked   = "OFFLINE_LOCKED"
	CodeDevBypass       = "DEV_BYPASS"
	CodeNotFound        = "NOT_FOUND"
)

// LicenseStatus representa el estado comercial actual de la licencia expuesto a la interfaz web.
type LicenseStatus struct {
	Enforced           bool       `json:"enforced"`
	Valid              bool       `json:"valid"`
	Code               string     `json:"code"`
	Status             string     `json:"status"`
	LicenseKeyMasked   string     `json:"license_key_masked"`
	PlanType           string     `json:"plan_type"`
	PlanLabel          string     `json:"plan_label"`
	ClientName         string     `json:"client_name"`
	MaxCompanies       int        `json:"max_companies"`
	CurrentCompanies   int        `json:"current_companies"`
	ActivatedAt        *time.Time `json:"activated_at,omitempty"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	DaysRemaining      int        `json:"days_remaining"`
	HoursRemaining     int        `json:"hours_remaining"`
	LastVerifiedAt     *time.Time `json:"last_verified_at,omitempty"`
	OfflineMode        bool       `json:"offline_mode"`
	MachineID          string     `json:"machine_id"`
	Message            string     `json:"message"`
	SupportPhone       string     `json:"support_phone"`
	SupportWhatsApp    string     `json:"support_whatsapp"`
	SupportWhatsAppURL string     `json:"support_whatsapp_url"`
	ServerConfigured   bool       `json:"server_configured"`
}

// cachedLicense se almacena cifrado con AES-256-GCM en la tabla licencia_local de SQLite.
type cachedLicense struct {
	LicenseKey      string     `json:"license_key"`
	MachineHash     string     `json:"machine_hash"`
	PlanType        string     `json:"plan_type"`
	PlanLabel       string     `json:"plan_label"`
	ClientName      string     `json:"client_name"`
	Status          string     `json:"status"`
	MaxCompanies    int        `json:"max_companies"`
	ActivatedAt     *time.Time `json:"activated_at,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	LastVerifiedUTC time.Time  `json:"last_verified_utc"`
	MaxSeenUTC      time.Time  `json:"max_seen_utc"`
	Message         string     `json:"message"`
}

// ShortMachineID devuelve un identificador corto legible para soporte técnico (ej. A1B2-C3D4-E5F6).
func ShortMachineID(hash string) string {
	clean := strings.ToUpper(strings.TrimSpace(hash))
	if len(clean) >= 12 {
		return clean[0:4] + "-" + clean[4:8] + "-" + clean[8:12]
	}
	if clean == "" {
		return "LOCAL-PC"
	}
	return clean
}

// MaskLicenseKey oculta los segmentos internos de la clave para mostrarla en UI sin exponerla completa.
func MaskLicenseKey(key string) string {
	clean := strings.ToUpper(strings.TrimSpace(key))
	if clean == "" {
		return ""
	}
	parts := strings.Split(clean, "-")
	if len(parts) >= 4 {
		masked := make([]string, len(parts))
		masked[0] = parts[0]
		masked[1] = parts[1]
		for i := 2; i < len(parts)-1; i++ {
			masked[i] = strings.Repeat("*", len(parts[i]))
		}
		masked[len(parts)-1] = parts[len(parts)-1]
		return strings.Join(masked, "-")
	}
	if len(clean) <= 6 {
		return strings.Repeat("*", len(clean))
	}
	return clean[:2] + strings.Repeat("*", len(clean)-6) + clean[len(clean)-4:]
}

// BuildWhatsAppURL construye el enlace oficial https://wa.me/... con mensaje prellenado.
func BuildWhatsAppURL(whatsappDigits, message string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, whatsappDigits)
	if digits == "" {
		digits = "51940781298"
	}
	u := "https://wa.me/" + digits
	if strings.TrimSpace(message) != "" {
		u += "?text=" + url.QueryEscape(strings.TrimSpace(message))
	}
	return u
}
