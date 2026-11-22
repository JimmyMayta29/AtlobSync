package web_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	comprasControllers "github.com/SwodLore/sire-api/COMPRAS/controllers"
	comprasRoutes "github.com/SwodLore/sire-api/COMPRAS/routes"
	ventasControllers "github.com/SwodLore/sire-api/VENTAS/controllers"
	ventasRoutes "github.com/SwodLore/sire-api/VENTAS/routes"
	"github.com/SwodLore/sire-api/internal/company"
	"github.com/SwodLore/sire-api/internal/config"
	"github.com/SwodLore/sire-api/internal/cpe"
	"github.com/SwodLore/sire-api/internal/crypto"
	"github.com/SwodLore/sire-api/internal/database"
	"github.com/SwodLore/sire-api/internal/license"
	"github.com/SwodLore/sire-api/internal/middleware"
	"github.com/SwodLore/sire-api/internal/rhe"
	"github.com/SwodLore/sire-api/internal/sire"
	"github.com/SwodLore/sire-api/internal/sunat"
	"github.com/SwodLore/sire-api/internal/system"
	"github.com/SwodLore/sire-api/internal/token"
	"github.com/SwodLore/sire-api/web"
	"github.com/gin-gonic/gin"
)

// setupTestRouter configura el motor de Gin con exactamente la misma estructura que main.go
func setupTestRouter(t *testing.T) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.HandleMethodNotAllowed = true

	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":     false,
				"error":       "Error interno del servidor",
				"detalle":     fmt.Sprintf("Panic recuperado: %v", recovered),
				"codigo_http": http.StatusInternalServerError,
			})
		} else {
			c.String(http.StatusInternalServerError, "500 Internal Server Error")
		}
		c.Abort()
	}))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{
				"success":     false,
				"error":       "Ruta no encontrada",
				"detalle":     fmt.Sprintf("El endpoint %s %s no existe en la API", c.Request.Method, path),
				"codigo_http": http.StatusNotFound,
			})
			return
		}
		c.Redirect(http.StatusFound, "/")
	})

	r.NoMethod(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusMethodNotAllowed, gin.H{
				"success":     false,
				"error":       "Método no permitido",
				"detalle":     fmt.Sprintf("El método %s no está permitido para la ruta %s", c.Request.Method, path),
				"codigo_http": http.StatusMethodNotAllowed,
			})
			return
		}
		c.String(http.StatusMethodNotAllowed, "Método no permitido")
	})

	sessionToken := middleware.InitSessionToken()
	r.Use(middleware.SecurityMiddleware())

	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Error inicializando DB en memoria: %v", err)
	}

	cryptoSvc, _ := crypto.NewCryptoService("12345678901234567890123456789012")
	companyRepo := company.NewRepository(db)
	tokenMgr := token.NewTokenManager(companyRepo, cryptoSvc)
	sunatClient := sunat.NewClient(tokenMgr)
	ventasControllers.Init(sunatClient)
	comprasControllers.Init(sunatClient)

	companySvc := company.NewService(companyRepo, cryptoSvc, tokenMgr)
	companyCtrl := company.NewController(companySvc)

	sireRepo := sire.NewRepository(db)
	sireClient := sire.NewSunatSireClient(sunatClient)
	sireWorker := sire.NewWorker(sireRepo, sireClient, tokenMgr, t.TempDir())
	sireSvc := sire.NewService(sireRepo, sireClient, sireWorker, t.TempDir())
	sireCtrl := sire.NewController(sireSvc)

	cpeRepo := cpe.NewRepository(db)
	cpeSvc := cpe.NewService(cpeRepo, companySvc, sireRepo, t.TempDir())
	cpeCtrl := cpe.NewController(cpeSvc)

	rheRepo := rhe.NewRepository(db)
	rheSvc := rhe.NewService(rheRepo, companySvc, t.TempDir())
	rheCtrl := rhe.NewController(rheSvc)

	cfg := &config.Config{Host: "127.0.0.1", Port: "8080", BaseDir: "."}
	sysCtrl := system.NewController(cfg, tokenMgr, companyRepo, sunatClient, sireWorker)
	licenseSvc := license.NewService(db, cryptoSvc, license.Config{Enforced: false}, nil)
	licenseCtrl := license.NewController(licenseSvc)

	web.RegisterRoutes(r)

	apiGroup := r.Group("/api")
	{
		licenseCtrl.RegisterRoutes(apiGroup)
		apiGroup.Use(license.RequireValidLicense(licenseSvc))
		sysCtrl.RegisterRoutes(apiGroup)
		companyCtrl.RegisterRoutes(apiGroup)

		empresaMiddleware := middleware.RequireActiveCompany(companyRepo)
		sireCtrl.RegisterRoutes(apiGroup, empresaMiddleware)
		cpeCtrl.RegisterRoutes(apiGroup, empresaMiddleware)
		rheCtrl.RegisterRoutes(apiGroup, empresaMiddleware)

		ventasGroup := apiGroup.Group("/empresas/:ruc/ventas", empresaMiddleware)
		ventasRoutes.RegisterRoutes(ventasGroup)

		comprasGroup := apiGroup.Group("/empresas/:ruc/compras", empresaMiddleware)
		comprasRoutes.RegisterRoutes(comprasGroup)
	}

	// Registrar endpoint de prueba para panic en /api
	apiGroup.GET("/test-panic", func(c *gin.Context) {
		panic("error inesperado de prueba")
	})

	return r, sessionToken
}

// TestRoutesContract_AllFrontendRoutesExistInBackend extrae todas las llamadas /api/... de web/index.html
// y valida contra las rutas registradas en el Router de Gin
func TestRoutesContract_AllFrontendRoutesExistInBackend(t *testing.T) {
	r, _ := setupTestRouter(t)

	// Obtener lista de rutas registradas en Gin
	registeredRoutes := make(map[string]bool)
	for _, routeInfo := range r.Routes() {
		registeredRoutes[routeInfo.Method+" "+routeInfo.Path] = true
	}

	// Extraer todas las cadenas que empiezan con /api/ de index.html
	htmlContent := string(web.GetEmbeddedHTML())
	re := regexp.MustCompile("['\"`](/api/[^'\"`?#\\s]+)")
	matches := re.FindAllStringSubmatch(htmlContent, -1)

	if len(matches) == 0 {
		t.Fatalf("No se encontraron rutas /api/ en el archivo index.html")
	}

	// Mapeo de rutas de index.html normalizadas (reemplazando ${...} con comodines :param)
	normalizedRoutes := make(map[string]bool)
	rePlaceholder := regexp.MustCompile(`\$\{[^}]+\}`)

	for _, m := range matches {
		rawPath := m[1]

		// Ignorar placeholders puros o rutas de documentación
		if strings.Contains(rawPath, "ejemplo") {
			continue
		}

		// Reemplazar ${...} por comodines :param o valores de prueba
		cleanPath := rePlaceholder.ReplaceAllString(rawPath, ":param")
		normalizedRoutes[cleanPath] = true
	}

	// Lista de rutas esperadas extraídas del frontend y su mapeo al Gin router
	expectedRoutes := []struct {
		desc        string
		method      string
		patternPath string
	}{
		{"Listar empresas", "GET", "/api/empresas"},
		{"Registrar empresa", "POST", "/api/empresas"},
		{"Editar empresa", "PUT", "/api/empresas/:ruc"},
		{"Eliminar empresa", "DELETE", "/api/empresas/:ruc"},
		{"Probar conexión", "POST", "/api/empresas/:ruc/probar-conexion"},
		{"Periodos Ventas", "GET", "/api/empresas/:ruc/ventas/periodos/:codLibro"},
		{"Periodos Compras", "GET", "/api/empresas/:ruc/compras/periodos/:codLibro"},
		{"Detalle Propuesta SIRE Global", "GET", "/api/sire/propuesta/detalle"},
		{"Exportar Propuesta SIRE Global", "POST", "/api/sire/propuesta/exportar"},
		{"Consultar Ticket SIRE", "GET", "/api/sire/tickets/:ruc/:ticket"},
		{"Listar Descargas Global", "GET", "/api/descargas"},
		{"Descargar Archivo por ID", "GET", "/api/descargas/:id/archivo"},
		{"Exportar Propuesta (XLSX/TXT/ZIP) por ID", "GET", "/api/descargas/:id/exportar"},
		{"Analizar Propuesta TXT por ID", "GET", "/api/descargas/:id/propuesta-txt"},
		{"Info del Sistema", "GET", "/api/sistema/info"},
		{"Abrir Carpeta", "POST", "/api/sistema/abrir-carpeta"},
		{"Abrir URL Externa", "POST", "/api/sistema/abrir-url"},
		{"Estado Licencia", "GET", "/api/licencia/estado"},
		{"Activar Licencia", "POST", "/api/licencia/activar"},
		{"Validar Licencia", "POST", "/api/licencia/validar"},
		{"Registrar Prueba 7 días", "POST", "/api/licencia/registrar-prueba"},
		{"Estado Worker", "GET", "/api/sistema/estado-worker"},
		{"Diagnóstico Periodos", "POST", "/api/sistema/diagnostico/:ruc"},
		{"Salud de la App", "GET", "/api/salud"},
		{"Constancia Recepción SIRE", "GET", "/api/empresas/:ruc/sire/constancia"},
		{"Listar Comprobantes CPE", "GET", "/api/empresas/:ruc/cpe"},
		{"Estado Sesión SOL CPE", "GET", "/api/empresas/:ruc/cpe/estado-sol"},
		{"Probar Sesión SOL CPE", "POST", "/api/empresas/:ruc/cpe/probar-sol"},
		{"Sincronizar CPE desde SIRE", "POST", "/api/empresas/:ruc/cpe/sincronizar-sire"},
		{"Registrar CPE Manual", "POST", "/api/empresas/:ruc/cpe/manual"},
		{"Importar XML/PDF/CDR", "POST", "/api/empresas/:ruc/cpe/importar"},
		{"Consultar CDR SOAP SUNAT", "POST", "/api/empresas/:ruc/cpe/:id/consultar-cdr"},
		{"Descargar Individual SUNAT SOL", "POST", "/api/empresas/:ruc/cpe/:id/descargar-sunat"},
		{"Ver Detalle Real CPE", "GET", "/api/empresas/:ruc/cpe/:id/detalle"},
		{"Descargar PDF CPE", "GET", "/api/empresas/:ruc/cpe/:id/pdf"},
		{"Descargar XML CPE", "GET", "/api/empresas/:ruc/cpe/:id/xml"},
		{"Descargar CDR CPE", "GET", "/api/empresas/:ruc/cpe/:id/cdr"},
		{"Descargar JSON CPE", "GET", "/api/empresas/:ruc/cpe/:id/json"},
		{"Iniciar Descarga Masiva CPE", "POST", "/api/empresas/:ruc/cpe/descarga-masiva"},
		{"Listar Lotes Descarga Masiva CPE", "GET", "/api/empresas/:ruc/cpe/descarga-masiva"},
		{"Estado Lote Descarga Masiva CPE", "GET", "/api/empresas/:ruc/cpe/descarga-masiva/:job_id"},
		{"Cancelar Lote Descarga Masiva CPE", "DELETE", "/api/empresas/:ruc/cpe/descarga-masiva/:job_id"},
		{"Descargar ZIP Lote Descarga Masiva CPE", "GET", "/api/empresas/:ruc/cpe/descarga-masiva/:job_id/zip"},
		{"Exportar Items CSV CPE", "GET", "/api/empresas/:ruc/cpe/exportar-items"},
		{"Listar RHE Recibidos", "GET", "/api/empresas/:ruc/rhe"},
		{"Consultar RHE en SUNAT SOL", "POST", "/api/empresas/:ruc/rhe/consultar"},
		{"Exportar RHE (XLSX/TXT)", "GET", "/api/empresas/:ruc/rhe/exportar"},
	}

	for _, er := range expectedRoutes {
		key := er.method + " " + er.patternPath
		if !registeredRoutes[key] {
			t.Errorf("[CONTRATO DE RUTAS] La ruta requerida por la UI '%s' NO está registrada en el router de Gin", key)
		}
	}
}

// TestBackend_NoRoute_ReturnsJSON prueba que cualquier ruta inexistente bajo /api responda JSON con código 404
func TestBackend_NoRoute_ReturnsJSON(t *testing.T) {
	r, sessionToken := setupTestRouter(t)

	req, _ := http.NewRequest(http.MethodGet, "/api/ruta-totalmente-inexistente-12345", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Local-Token", sessionToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("Se esperaba 404 para ruta inexistente, se obtuvo %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type debe ser application/json, se obtuvo %s", contentType)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("El cuerpo devuelto no es JSON válido: %v. Cuerpo: %s", err, w.Body.String())
	}

	if res["success"] != false || res["error"] == nil || res["detalle"] == nil || res["codigo_http"] != float64(404) {
		t.Fatalf("La respuesta 404 no cumple la estructura JSON {success:false, error, detalle, codigo_http}. Obtenido: %v", res)
	}
}

// TestBackend_NoMethod_ReturnsJSON prueba que un método no permitido bajo /api responda JSON 405
func TestBackend_NoMethod_ReturnsJSON(t *testing.T) {
	r, sessionToken := setupTestRouter(t)

	// DELETE no está permitido en /api/salud (solo GET)
	req, _ := http.NewRequest(http.MethodDelete, "/api/salud", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Local-Token", sessionToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Se esperaba 405 para método no permitido, se obtuvo %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type debe ser application/json, se obtuvo %s", contentType)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("El cuerpo devuelto no es JSON válido: %v. Cuerpo: %s", err, w.Body.String())
	}

	if res["success"] != false || res["codigo_http"] != float64(405) {
		t.Fatalf("La respuesta 405 no cumple la estructura JSON. Obtenido: %v", res)
	}
}

// TestBackend_Recovery_ReturnsJSON prueba que un panic en un handler de /api devuelva JSON 500
func TestBackend_Recovery_ReturnsJSON(t *testing.T) {
	r, sessionToken := setupTestRouter(t)

	req, _ := http.NewRequest(http.MethodGet, "/api/test-panic", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Local-Token", sessionToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Se esperaba 500 tras panic, se obtuvo %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type debe ser application/json, se obtuvo %s", contentType)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("El cuerpo devuelto tras panic no es JSON válido: %v. Cuerpo: %s", err, w.Body.String())
	}

	if res["success"] != false || res["codigo_http"] != float64(500) {
		t.Fatalf("La respuesta de recuperación de panic no cumple el contrato JSON. Obtenido: %v", res)
	}
}

// TestBackend_SaludEndpoint_PublicAndReturnsInfo prueba que /api/salud funcione sin token y devuelva app y version
func TestBackend_SaludEndpoint_PublicAndReturnsInfo(t *testing.T) {
	r, _ := setupTestRouter(t)

	// Petición SIN X-Local-Token
	req, _ := http.NewRequest(http.MethodGet, "/api/salud", nil)
	req.Host = "127.0.0.1:8080"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Se esperaba 200 en /api/salud sin token, se obtuvo %d", w.Code)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("No devolvió JSON válido: %v", err)
	}

	if res["status"] != "ok" || res["app"] != "sire-api" || res["version"] != "2.0.0" {
		t.Fatalf("Datos de /api/salud inesperados: %v", res)
	}
	if res["pid"] == nil || res["pid"].(float64) <= 0 {
		t.Fatalf("pid inválido en /api/salud: %v", res["pid"])
	}
}

// TestBackend_SecurityMiddleware_RejectionsAreJSON prueba que rechazos por token ausente sean JSON estructurado
func TestBackend_SecurityMiddleware_RejectionsAreJSON(t *testing.T) {
	r, _ := setupTestRouter(t)

	// Petición a ruta protegida SIN token de sesión
	req, _ := http.NewRequest(http.MethodGet, "/api/empresas", nil)
	req.Host = "127.0.0.1:8080"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Se esperaba 403 por falta de token de sesión local, se obtuvo %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type debe ser application/json en rechazo de middleware, se obtuvo %s", contentType)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("El rechazo no es JSON válido: %v", err)
	}

	if res["success"] != false || res["codigo_http"] != float64(403) {
		t.Fatalf("Estructura de error de middleware inesperada: %v", res)
	}
}
