package sunat

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

type TokenProvider interface {
	GetToken(ruc string) (string, error)
	InvalidateToken(ruc string)
	IsCooldown(ruc string) (bool, time.Time, string)
	ClearCooldown(ruc string)
}

type Client struct {
	tokenMgr   TokenProvider
	httpClient *resty.Client
	BaseURL    string
}

type clientCtxKey string

const (
	tenantRucCtxKey clientCtxKey = "tenant_ruc"
)

func NewClient(tokenMgr TokenProvider) *Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	r := resty.New()
	r.SetTransport(transport)
	r.SetTimeout(60 * time.Second)
	r.SetRetryCount(1)

	client := &Client{
		tokenMgr:   tokenMgr,
		httpClient: r,
		BaseURL:    "https://api-sire.sunat.gob.pe",
	}

	// Logging de cada intento saliente en data/logs/sunat_intentos.log (Task 5)
	r.OnAfterResponse(func(cl *resty.Client, resp *resty.Response) error {
		var ruc string
		if val := resp.Request.Context().Value(tenantRucCtxKey); val != nil {
			ruc, _ = val.(string)
		}
		method := resp.Request.Method
		endpoint := resp.Request.URL
		statusCode := resp.StatusCode()
		duration := resp.Time()
		resultado := "EXITO"
		if statusCode >= 400 {
			resultado = fmt.Sprintf("HTTP_%d", statusCode)
		}
		LogAttempt(ruc, method, endpoint, resultado, statusCode, duration)
		return nil
	})

	r.OnError(func(req *resty.Request, err error) {
		var ruc string
		if val := req.Context().Value(tenantRucCtxKey); val != nil {
			ruc, _ = val.(string)
		}
		method := req.Method
		endpoint := req.URL
		LogAttempt(ruc, method, endpoint, "ERROR_RED", 0, 0)
	})

	// Interceptor de seguridad de Resty: NUNCA enviar peticiones hacia SUNAT sin un Bearer token válido
	r.OnBeforeRequest(func(cl *resty.Client, req *resty.Request) error {
		auth := req.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			var rucStr string
			if val := req.Context().Value(tenantRucCtxKey); val != nil {
				rucStr = fmt.Sprintf(" para el RUC %v", val)
			}
			return fmt.Errorf("operación abortada: no se cuenta con token Bearer válido%s", rucStr)
		}
		return nil
	})

	// Middleware de reintento automático ante HTTP 401 (estrictamente una sola vez)
	r.AddRetryCondition(func(resp *resty.Response, err error) bool {
		if resp != nil && resp.StatusCode() == http.StatusUnauthorized {
			if resp.Request.Attempt > 1 {
				return false // Ya se intentó renovar una vez, no volver a disparar peticiones
			}
			var ruc string
			if val := resp.Request.Context().Value(tenantRucCtxKey); val != nil {
				ruc, _ = val.(string)
			}
			if ruc != "" {
				if inCooldown, _, _ := tokenMgr.IsCooldown(ruc); inCooldown {
					log.Printf("[SUNAT] RUC %s en enfriamiento de seguridad. No se reintenta tras 401.", ruc)
					return false
				}
				log.Printf("[SUNAT] Recibido HTTP 401 para RUC %s. Reintentando renovación de sesión una sola vez...", ruc)
				tokenMgr.InvalidateToken(ruc)
				freshToken, err := tokenMgr.GetToken(ruc)
				if err == nil {
					resp.Request.SetHeader("Authorization", "Bearer "+freshToken)
					return true
				}
			}
		}
		return false
	})

	return client
}

// TokenManager devuelve el proveedor de tokens asociado
func (c *Client) TokenManager() TokenProvider {
	return c.tokenMgr
}

// SetBaseURL permite sobreescribir la URL base (útil para pruebas unitarias con httptest)
func (c *Client) SetBaseURL(url string) {
	c.BaseURL = strings.TrimRight(url, "/")
}

// R genera una nueva petición Resty configurada con el token y el RUC para reintento automático
func (c *Client) R(ruc string) (*resty.Request, error) {
	token, err := c.tokenMgr.GetToken(ruc)
	if err != nil {
		return nil, fmt.Errorf("error de autenticación SUNAT para RUC %s: %w", ruc, err)
	}

	ctx := context.WithValue(context.Background(), tenantRucCtxKey, ruc)
	req := c.httpClient.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+token)

	return req, nil
}

// NewRequest genera una nueva petición Resty lista para usar con el RUC especificado.
// Si la obtención de token falla, la petición se cancela antes de tocar la red hacia SUNAT.
func (c *Client) NewRequest(ruc string) *resty.Request {
	req, err := c.R(ruc)
	if err != nil {
		log.Printf("[SUNAT] [PETICION ABORTADA] Error de token para RUC %s: %v. Cancelando envío a SUNAT.", ruc, err)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return c.httpClient.R().
			SetContext(ctx)
	}
	return req
}

// Do ejecuta una función builder inyectando el token Bearer del RUC
func (c *Client) Do(ruc string, reqBuilder func(r *resty.Request) (*resty.Response, error)) (*resty.Response, error) {
	req, err := c.R(ruc)
	if err != nil {
		return nil, err
	}
	return reqBuilder(req)
}

// NewSunatHttpError construye un SunatHttpError completo a partir de una respuesta de Resty
func NewSunatHttpError(resp *resty.Response) *SunatHttpError {
	if resp == nil {
		return &SunatHttpError{
			StatusCode: 500,
			RawBody:    "no se recibió respuesta de los servidores de SUNAT",
		}
	}
	var host string
	if resp.RawResponse != nil && resp.RawResponse.Request != nil && resp.RawResponse.Request.URL != nil {
		host = resp.RawResponse.Request.URL.Host
	} else if resp.Request != nil {
		host = "api-sire.sunat.gob.pe"
	}
	var ruta string
	if resp.Request != nil {
		ruta = resp.Request.URL
	}
	return &SunatHttpError{
		StatusCode: resp.StatusCode(),
		Host:       host,
		Duration:   resp.Time(),
		RawBody:    resp.String(),
		Headers:    FilterResponseHeaders(resp.Header()),
		Ruta:       ruta,
	}
}

// FormatSunatResponse formatea la respuesta de SUNAT para devolver un error estructurado, amigable y clasificado
func FormatSunatResponse(c *gin.Context, resp *resty.Response) {
	if resp == nil {
		RespondWithError(c, fmt.Errorf("no se recibió respuesta de los servidores de SUNAT"))
		return
	}
	RespondWithError(c, NewSunatHttpError(resp))
}

