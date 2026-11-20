package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	AUTH "github.com/SwodLore/sire-api/AUTH"
	comprasControllers "github.com/SwodLore/sire-api/COMPRAS/controllers"
	comprasRoutes "github.com/SwodLore/sire-api/COMPRAS/routes"
	ventasControllers "github.com/SwodLore/sire-api/VENTAS/controllers"
	ventasRoutes "github.com/SwodLore/sire-api/VENTAS/routes"
	"github.com/SwodLore/sire-api/internal/company"
	"github.com/SwodLore/sire-api/internal/config"
	"github.com/SwodLore/sire-api/internal/cpe"
	"github.com/SwodLore/sire-api/internal/crypto"
	"github.com/SwodLore/sire-api/internal/database"
	"github.com/SwodLore/sire-api/internal/middleware"
	"github.com/SwodLore/sire-api/internal/sire"
	"github.com/SwodLore/sire-api/internal/sunat"
	"github.com/SwodLore/sire-api/internal/system"
	"github.com/SwodLore/sire-api/internal/token"
	"github.com/SwodLore/sire-api/web"
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Cargar configuración global y preparar carpetas
	cfg := config.LoadConfig()

	// 2. Comprobar disponibilidad de puerto y detectar si ya existe instancia activa
	port, isRunning, err := config.ProbeAvailablePort(cfg.Host, cfg.Port)
	if err != nil {
		config.ShowWindowsAlert("ATLOB Sync - Error de Red", fmt.Sprintf("No se pudo enlazar ningún puerto disponible: %v", err))
		log.Fatalf("Error crítico al buscar puerto: %v", err)
	}

	if isRunning {
		log.Printf("[INICIO] Se detectó una instancia de ATLOB Sync · SUNAT ya en ejecución en el puerto %s.", port)
		config.OpenBrowser(fmt.Sprintf("http://127.0.0.1:%s", port))
		return // Salida limpia sin abrir segundo proceso
	}
	cfg.Port = port

	// 3. Inicializar token de sesión local para protección de la API
	middleware.InitSessionToken()

	// Configurar modo de Gin
	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 4. Inicializar Base de Datos SQLite (WAL y migraciones)
	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		config.ShowWindowsAlert("ATLOB Sync - Error de Base de Datos", fmt.Sprintf("Error abriendo base de datos: %v", err))
		log.Fatalf("Error crítico al inicializar base de datos SQLite: %v", err)
	}
	defer db.Close()

	// 5. Inicializar Servicio Criptográfico AES-256-GCM
	cryptoSvc, err := crypto.NewCryptoService(cfg.AppMasterKey)
	if err != nil {
		config.ShowWindowsAlert("ATLOB Sync - Error Criptográfico", fmt.Sprintf("Error en servicio criptográfico: %v", err))
		log.Fatalf("Error crítico al inicializar servicio criptográfico: %v", err)
	}

	// 6. Inicializar Repositorio y Gestor de Tokens por RUC
	companyRepo := company.NewRepository(db)
	tokenMgr := token.NewTokenManager(companyRepo, cryptoSvc)

	// 7. Inicializar Cliente HTTP SUNAT Multiempresa
	sunatClient := sunat.NewClient(tokenMgr)
	ventasControllers.Init(sunatClient)
	comprasControllers.Init(sunatClient)

	// 8. Inicializar Servicio y Controlador de Empresas
	companySvc := company.NewService(companyRepo, cryptoSvc, tokenMgr)
	companyCtrl := company.NewController(companySvc)

	// 9. Inicializar Módulo Asíncrono SIRE (Worker en segundo plano y descargas)
	sireRepo := sire.NewRepository(db)
	sireClient := sire.NewSunatSireClient(sunatClient)

	// Conectar verificador real de acceso a api-sire.sunat.gob.pe para "Probar Conexión"
	companySvc.SetSireVerifier(sireClient.VerificarAccesoSIRE)

	// 9.1 Inicializar Módulo de Comprobantes Individuales (PDF / XML / CDR)
	cpeRepo := cpe.NewRepository(db)
	cpeSvc := cpe.NewService(cpeRepo, companySvc, sireRepo, cfg.StorageDir)
	cpeCtrl := cpe.NewController(cpeSvc)

	sireWorker := sire.NewWorker(sireRepo, sireClient, tokenMgr, cfg.StorageDir)
	sireWorker.OnDownloadCompleted = func(d *sire.DescargaSire) {
		_, _ = cpeSvc.SyncFromDescarga(d)
	}
	sireWorker.Start()
	defer sireWorker.Stop()

	sireSvc := sire.NewService(sireRepo, sireClient, sireWorker, cfg.StorageDir)
	sireCtrl := sire.NewController(sireSvc)

	// Sembrar empresa inicial desde .env si la BD está vacía
	company.SeedInitialCompanyFromEnv(companySvc)

	// 10. Controlador del Sistema (Diagnósticos, Apertura de Carpetas e Info)
	sysCtrl := system.NewController(cfg, tokenMgr, companyRepo, sunatClient, sireWorker)

	// 11. Configurar Servidor Gin y Middlewares de Seguridad Local
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.HandleMethodNotAllowed = true

	r.Use(middleware.SanitizedLoggerMiddleware())
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

	r.Use(middleware.SecurityMiddleware())

	// 12. Registrar Interfaz Web Embebida (//go:embed)
	web.RegisterRoutes(r)

	// 13. Registrar Rutas de la API Multiempresa
	apiGroup := r.Group("/api")
	{
		// Endpoints de utilidades del sistema
		sysCtrl.RegisterRoutes(apiGroup)

		// CRUD y utilidades de Empresas (/api/empresas)
		companyCtrl.RegisterRoutes(apiGroup)

		// Endpoints multiempresa protegidos por validación de RUC activo
		empresaMiddleware := middleware.RequireActiveCompany(companyRepo)

		// Rutas SIRE Asíncronas (Exportar, Tickets, Descargas seguras, Detalle paginado, Constancia CDR)
		sireCtrl.RegisterRoutes(apiGroup, empresaMiddleware)

		// Rutas de Comprobantes Individuales (PDF / XML / CDR)
		cpeCtrl.RegisterRoutes(apiGroup, empresaMiddleware)

		ventasGroup := apiGroup.Group("/empresas/:ruc/ventas", empresaMiddleware)
		ventasRoutes.RegisterRoutes(ventasGroup)

		comprasGroup := apiGroup.Group("/empresas/:ruc/compras", empresaMiddleware)
		comprasRoutes.RegisterRoutes(comprasGroup)
	}

	// 14. Rutas heredadas (para compatibilidad con scripts existentes)
	AUTH.RegisterAuthRoutes(r)

	legacyMiddleware := middleware.RequireDefaultCompany(companyRepo, cfg.DefaultRUC)

	legacyVentas := r.Group("/api/sunat/ventas", legacyMiddleware)
	ventasRoutes.RegisterRoutes(legacyVentas)

	legacyCompras := r.Group("/api/sunat/compras", legacyMiddleware)
	comprasRoutes.RegisterRoutes(legacyCompras)

	// 15. Iniciar Servidor en 127.0.0.1 y abrir navegador automáticamente
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	appURL := fmt.Sprintf("http://127.0.0.1:%s", cfg.Port)

	log.Printf("========================================================")
	log.Printf("  ATLOB Sync · SUNAT (ATLOB Consultores) v2.0 Iniciado  ")
	log.Printf("  URL de la aplicación: %s                              ", appURL)
	log.Printf("  Carpeta de datos:     %s                              ", cfg.DBPath)
	log.Printf("  Carpeta de descargas: %s                              ", cfg.StorageDir)
	log.Printf("========================================================")

	config.OpenBrowser(appURL)

	if err := r.Run(addr); err != nil {
		config.ShowWindowsAlert("ATLOB Sync - Error de Ejecución", fmt.Sprintf("Error iniciando servidor en %s: %v", addr, err))
		log.Fatalf("Error al iniciar servidor HTTP: %v", err)
	}
}
