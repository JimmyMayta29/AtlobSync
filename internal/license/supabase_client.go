package license

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNetwork indica un fallo de conectividad o disponibilidad temporal con el servidor de licencias.
var ErrNetwork = errors.New("error de red al contactar el servidor de licencias")

type rpcResponse struct {
	OK             bool       `json:"ok"`
	Code           string     `json:"code"`
	Status         string     `json:"status"`
	LicenseKey     string     `json:"license_key"`
	PlanType       string     `json:"plan_type"`
	PlanLabel      string     `json:"plan_label"`
	ClientName     string     `json:"client_name"`
	MaxCompanies   int        `json:"max_companies"`
	DurationDays   *int       `json:"duration_days"`
	ActivatedAt    *time.Time `json:"activated_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	DaysRemaining  int        `json:"days_remaining"`
	HoursRemaining int        `json:"hours_remaining"`
	ServerTime     *time.Time `json:"server_time"`
	Message        string     `json:"message"`
}

type supabaseErrorBody struct {
	Message string `json:"message"`
	Hint    string `json:"hint"`
	Details string `json:"details"`
	Error   string `json:"error"`
}

// SupabaseClient realiza llamadas RPC seguras contra PostgREST de Supabase usando la anon key.
type SupabaseClient struct {
	baseURL    string
	anonKey    string
	httpClient *http.Client
}

func NewSupabaseClient(baseURL, anonKey string) *SupabaseClient {
	return &SupabaseClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		anonKey: strings.TrimSpace(anonKey),
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (c *SupabaseClient) IsConfigured() bool {
	return c != nil && c.baseURL != "" && c.anonKey != ""
}

func (c *SupabaseClient) Activate(ctx context.Context, key, machineHash, machineName, appVersion string) (*rpcResponse, error) {
	payload := map[string]interface{}{
		"p_license_key":  key,
		"p_machine_hash": machineHash,
		"p_machine_name": machineName,
		"p_app_version":  appVersion,
	}
	return c.callRPC(ctx, "activate_license", payload)
}

func (c *SupabaseClient) Validate(ctx context.Context, key, machineHash, machineName, appVersion string, companiesCount int) (*rpcResponse, error) {
	payload := map[string]interface{}{
		"p_license_key":     key,
		"p_machine_hash":    machineHash,
		"p_machine_name":    machineName,
		"p_app_version":     appVersion,
		"p_companies_count": companiesCount,
	}
	return c.callRPC(ctx, "validate_license", payload)
}

func (c *SupabaseClient) RegisterTrial(ctx context.Context, clientName, clientEmail, clientPhone, machineHash, machineName, appVersion string) (*rpcResponse, error) {
	payload := map[string]interface{}{
		"p_client_name":  clientName,
		"p_client_email": clientEmail,
		"p_client_phone": clientPhone,
		"p_machine_hash": machineHash,
		"p_machine_name": machineName,
		"p_app_version":  appVersion,
	}
	return c.callRPC(ctx, "register_trial", payload)
}

func (c *SupabaseClient) callRPC(ctx context.Context, fnName string, payload map[string]interface{}) (*rpcResponse, error) {
	if !c.IsConfigured() {
		return nil, errors.New("servidor de licencias Supabase no configurado")
	}

	rawBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/rest/v1/rpc/%s", c.baseURL, fnName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetwork, err)
	}

	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Authorization", "Bearer "+c.anonKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetwork, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: error leyendo respuesta: %v", ErrNetwork, err)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: servidor respondió HTTP %d", ErrNetwork, resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		var sbErr supabaseErrorBody
		_ = json.Unmarshal(bodyBytes, &sbErr)
		msg := strings.TrimSpace(sbErr.Message)
		if msg == "" {
			msg = strings.TrimSpace(sbErr.Error)
		}
		if msg == "" {
			msg = fmt.Sprintf("error HTTP %d en servicio de licencias", resp.StatusCode)
		}
		return nil, errors.New(msg)
	}

	var out rpcResponse
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return nil, fmt.Errorf("respuesta inválida del servidor de licencias: %w", err)
	}
	return &out, nil
}
