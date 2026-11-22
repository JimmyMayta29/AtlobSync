package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

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
	"github.com/SwodLore/sire-api/internal/desktop"
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

func isHeadlessRequested() bool {
	if strings.TrimSpace(os.Getenv("ATLOB_HEADLESS")) == "1" {
		return true
	}
	for _, arg := range os.Args[1:] {
		if arg == "--headless" || arg == "--no-gui" {
			return true
		}
	}
	return false
}

func main() {
	runtime.LockOSThread()

	// 1. Cargar configuración global y preparar carpetas locales
	cfg := config.LoadConfig()

	// 2. Comprobar disponibilidad de puerto y detectar si ya existe instancia activa
	port, isRunning, err := config.ProbeAvailablePort(cfg.Host, cfg.Port)
	if err != nil {
		config.ShowWindowsAlert("ATLOB Sync - Error de Red", fmt.Sprintf("No se pudo enlazar ningún puerto local disponible: %v", err))
		log.Fatalf("Error crítico al buscar puerto: %v", err)
	}

	if isRunning {
		log.Printf("[INICIO] Se detectó una instancia de ATLOB Sync · SUNAT ya en ejecución en el puerto local %s.", port)
		if !desktop.BringExistingWindowToFront() && !isHeadlessRequested() {
			config.ShowWindowsInfo("ATLOB Sync · SUNAT", "ATLOB Sync ya se encuentra abierto en este equipo.")
		}
		return // Salida limpia sin abrir segundo proceso ni navegador web
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

	// 6. Inicializar Repositorio, Gestor de Tokens por RUC y Motor de Licencias Comerciales
	companyRepo := company.NewRepository(db)
	licenseSvc := license.NewService(db, cryptoSvc, license.LoadConfigFromEnv(), func() (int, error) {
		empresas, err := companyRepo.GetAll()
		if err != nil {
			return 0, err
		}
		return len(empresas), nil
	})
	initLicCtx, cancelLicInit := context.WithTimeout(context.Background(), 8*time.Second)
	if licInitErr := licenseSvc.Initialize(initLicCtx); licInitErr != nil {
		log.Printf("[LICENCIA] Aviso en validación inicial: %v", licInitErr)
	}
	cancelLicInit()

	bgLicCtx, cancelBgLic := context.WithCancel(context.Background())
	defer cancelBgLic()
	go licenseSvc.StartBackgroundRefresh(bgLicCtx)

	if licStatus := licenseSvc.GetStatus(); licStatus.Enforced {
		log.Printf("[LICENCIA] Estado=%s | Plan=%s | Vigente=%v | Días restantes=%d | Equipo=%s",
			licStatus.Code, licStatus.PlanLabel, licStatus.Valid, licStatus.DaysRemaining, licStatus.MachineID)
	} else {
		log.Printf("[LICENCIA] Modo desarrollo local (validación comercial desactivada).")
	}

	tokenMgr := token.NewTokenManager(companyRepo, cryptoSvc)

	// 7. Inicializar Cliente HTTP SUNAT Multiempresa
	sunatClient := sunat.NewClient(tokenMgr)
	ventasControllers.Init(sunatClient)
	comprasControllers.Init(sunatClient)

	// 8. Inicializar Servicio y Controlador de Empresas
	companySvc := company.NewService(companyRepo, cryptoSvc, tokenMgr)
	companySvc.SetCompanyLimitChecker(licenseSvc.CheckCanCreateCompany)
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

	// 9.2 Inicializar Módulo de Recibos por Honorarios Electrónicos (RHE · Receptor · Modo Local SOL)
	rheRepo := rhe.NewRepository(db)
	rheSvc := rhe.NewService(rheRepo, companySvc, cfg.StorageDir)
	rheCtrl := rhe.NewController(rheSvc)

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

	licenseCtrl := license.NewController(licenseSvc)
	licenseMiddleware := license.RequireValidLicense(licenseSvc)

	// 13. Registrar Rutas de la API Multiempresa
	apiGroup := r.Group("/api")
	{
		// Rutas de licencia (activación, estado y revalidación)
		licenseCtrl.RegisterRoutes(apiGroup)

		// Protección comercial para el resto de la API
		apiGroup.Use(licenseMiddleware)

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

		// Rutas de Recibos por Honorarios Electrónicos (RHE · Receptor)
		rheCtrl.RegisterRoutes(apiGroup, empresaMiddleware)

		ventasGroup := apiGroup.Group("/empresas/:ruc/ventas", empresaMiddleware)
		ventasRoutes.RegisterRoutes(ventasGroup)

		comprasGroup := apiGroup.Group("/empresas/:ruc/compras", empresaMiddleware)
		comprasRoutes.RegisterRoutes(comprasGroup)
	}

	// 14. Rutas heredadas (para compatibilidad con scripts existentes)
	AUTH.RegisterAuthRoutes(r)

	legacyMiddleware := middleware.RequireDefaultCompany(companyRepo, cfg.DefaultRUC)

	legacyVentas := r.Group("/api/sunat/ventas", licenseMiddleware, legacyMiddleware)
	ventasRoutes.RegisterRoutes(legacyVentas)

	legacyCompras := r.Group("/api/sunat/compras", licenseMiddleware, legacyMiddleware)
	comprasRoutes.RegisterRoutes(legacyCompras)

	// 15. Iniciar motor local en 127.0.0.1 y abrir ventana nativa de escritorio (sin navegador web)
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	appURL := fmt.Sprintf("http://127.0.0.1:%s", cfg.Port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		config.ShowWindowsAlert("ATLOB Sync - Error de Ejecución", fmt.Sprintf("No se pudo iniciar el servicio local en %s: %v", addr, err))
		log.Fatalf("Error al enlazar listener local en %s: %v", addr, err)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	srvErrCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErrCh <- err
		}
		close(srvErrCh)
	}()

	log.Printf("========================================================")
	log.Printf("  ATLOB Sync · SUNAT (ATLOB Consultores) v2.0 Escritorio")
	log.Printf("  Motor local:          %s", appURL)
	log.Printf("  Base de datos:        %s", cfg.DBPath)
	log.Printf("  Carpeta de descargas: %s", cfg.StorageDir)
	log.Printf("========================================================")

	if isHeadlessRequested() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		select {
		case <-sigCh:
			log.Printf("[CIERRE] Señal de terminación recibida en modo headless.")
		case srvErr := <-srvErrCh:
			if srvErr != nil {
				log.Printf("[ERROR] Motor HTTP local detenido: %v", srvErr)
			}
		}
	} else {
		winErr := desktop.RunWindow(desktop.WindowConfig{
			Title:    desktop.MainWindowTitle,
			URL:      appURL,
			DataPath: cfg.WebViewDataDir,
			Width:    1320,
			Height:   840,
			Debug:    cfg.GinMode != "release",
		})
		if winErr != nil {
			config.ShowWindowsAlert("ATLOB Sync - Error de Interfaz Local", winErr.Error())
			log.Printf("[ERROR] Ventana de escritorio finalizada con error: %v", winErr)
		}
	}

	// 16. Cierre limpio del motor local, worker y base de datos SQLite
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[CIERRE] Error en apagado ordenado del motor local: %v", err)
	}
	log.Printf("[CIERRE] ATLOB Sync · SUNAT cerrado correctamente.")
}
