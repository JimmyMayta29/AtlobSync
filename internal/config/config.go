package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/joho/godotenv"
)

type Config struct {
	Host           string `json:"host"`
	Port           string `json:"port"`
	GinMode        string `json:"gin_mode"`
	AppMasterKey   string `json:"-"`
	KeySource      string `json:"key_source"`
	BaseDir        string `json:"base_dir"`
	DBPath         string `json:"db_path"`
	StorageDir     string `json:"storage_dir"`
	LogsDir        string `json:"logs_dir"`
	WebViewDataDir string `json:"webview_data_dir"`
	DefaultRUC     string `json:"default_ruc"`
}

var AppConfig *Config

// DefaultAppDataDir retorna la carpeta estándar de datos locales del usuario (%LocalAppData%\ATLOB Sync en Windows).
func DefaultAppDataDir() string {
	if runtime.GOOS == "windows" {
		if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
			return filepath.Join(localAppData, "ATLOB Sync")
		}
	}
	if cfgDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(cfgDir) != "" {
		return filepath.Join(cfgDir, "ATLOB Sync")
	}
	return ""
}

// isInstalledProgramPath detecta si el ejecutable reside en un directorio típico de instalación de programas.
func isInstalledProgramPath(execDir string) bool {
	lower := strings.ToLower(filepath.Clean(execDir))
	return strings.Contains(lower, `\programs\atlob sync`) ||
		strings.Contains(lower, `\program files`) ||
		strings.Contains(lower, `/programs/atlob sync`)
}

// hasPortableMarker verifica si el directorio del ejecutable contiene archivos de datos portables.
func hasPortableMarker(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "sire.db")); err == nil {
		return true
	}
	return false
}

// isGoRunOrTestBinary detecta si el binario se está ejecutando desde `go run` o `go test`.
func isGoRunOrTestBinary(execPath string) bool {
	lower := strings.ToLower(execPath)
	return strings.Contains(lower, "go-build") ||
		strings.HasSuffix(lower, ".test.exe") ||
		strings.HasSuffix(lower, ".test")
}

// resolveBaseDir determina el directorio base de datos locales a partir de la ruta del ejecutable y variables de entorno.
func resolveBaseDir(execPath string, execErr error) string {
	if customDir := strings.TrimSpace(os.Getenv("ATLOB_DATA_DIR")); customDir != "" {
		if abs, err := filepath.Abs(customDir); err == nil {
			return abs
		}
		return customDir
	}

	if execErr == nil && strings.TrimSpace(execPath) != "" && !isGoRunOrTestBinary(execPath) {
		execDir := filepath.Dir(execPath)
		// Modo portable explícito o carpeta de proyecto/portable con .env o data/sire.db junto al .exe
		if strings.TrimSpace(os.Getenv("ATLOB_PORTABLE")) == "1" {
			return execDir
		}
		if !isInstalledProgramPath(execDir) && hasPortableMarker(execDir) {
			return execDir
		}
		// Modo instalado de escritorio local: usar %LocalAppData%\ATLOB Sync
		if appDataDir := DefaultAppDataDir(); appDataDir != "" {
			return appDataDir
		}
		return execDir
	}

	// Fallback para ejecución desde go run / go test
	dir, err := os.Getwd()
	if err == nil {
		return dir
	}
	return "."
}

// GetBaseDir obtiene la ruta base de datos locales de la aplicación.
func GetBaseDir() string {
	execPath, err := os.Executable()
	return resolveBaseDir(execPath, err)
}

// SetupDirectoriesAndLogging asegura la existencia de carpetas requeridas y configura logging en archivo
func SetupDirectoriesAndLogging(baseDir string) error {
	dataDir := filepath.Join(baseDir, "data")
	logsDir := filepath.Join(dataDir, "logs")
	storageDir := filepath.Join(baseDir, "descargas")
	webviewDir := filepath.Join(baseDir, "webview2")

	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("error al crear directorio de logs: %w", err)
	}
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return fmt.Errorf("error al crear directorio de descargas: %w", err)
	}
	if err := os.MkdirAll(webviewDir, 0755); err != nil {
		return fmt.Errorf("error al crear directorio de perfil local: %w", err)
	}

	logFilePath := filepath.Join(logsDir, "sire_api.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	}
	return nil
}

// ResolveMasterKey obtiene la llave maestra garantizando que nunca se sobreescriba si hay datos cifrados
func ResolveMasterKey(baseDir string) (string, string, error) {
	// 1. Variable de entorno o archivo .env
	envKey := strings.TrimSpace(os.Getenv("APP_MASTER_KEY"))
	if len(envKey) >= 32 {
		return envKey, "ENV / .env", nil
	}

	// 2. Archivo local seguro data/app.key
	keyPath := filepath.Join(baseDir, "data", "app.key")
	if data, err := os.ReadFile(keyPath); err == nil {
		storedKey := strings.TrimSpace(string(data))
		if len(storedKey) >= 32 {
			return storedKey, "data/app.key", nil
		}
	}

	// 3. Protección contra pérdida de datos: si existe base de datos previa, NO generar llave nueva
	dbPath := filepath.Join(baseDir, "data", "sire.db")
	if fi, err := os.Stat(dbPath); err == nil && fi.Size() > 0 {
		return "", "", fmt.Errorf("se detectó una base de datos existente en '%s' pero no se encontró la llave maestra (APP_MASTER_KEY).\nPor seguridad criptográfica no se puede generar una llave nueva porque las credenciales registradas quedarían inaccesibles.\nRestaure el archivo 'data/app.key' o configure la variable APP_MASTER_KEY.", dbPath)
	}

	// 4. Primer arranque limpio: generar nueva llave segura de 32 bytes (64 caracteres hexadecimales)
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		return "", "", fmt.Errorf("error generando entropía criptográfica: %w", err)
	}
	newKey := hex.EncodeToString(randBytes)

	if err := os.WriteFile(keyPath, []byte(newKey), 0600); err != nil {
		return "", "", fmt.Errorf("error guardando llave maestra en '%s': %w", keyPath, err)
	}

	log.Printf("[CONFIG] Primer arranque: Llave maestra generada y almacenada en '%s'", keyPath)
	return newKey, "data/app.key (Generada)", nil
}

const AppVersion = "2.0.0"

// ProbeAvailablePort busca un puerto libre o detecta si ya hay una instancia de sire-api en ejecución
func ProbeAvailablePort(host, startPort string) (selectedPort string, isExistingInstance bool, err error) {
	portNum, err := strconv.Atoi(startPort)
	if err != nil {
		portNum = 8080
	}

	for i := 0; i < 15; i++ {
		currentPort := strconv.Itoa(portNum + i)
		addr := net.JoinHostPort(host, currentPort)

		ln, listenErr := net.Listen("tcp", addr)
		if listenErr == nil {
			_ = ln.Close()
			return currentPort, false, nil
		}

		// El puerto está en uso: consultar si es una instancia activa de sire-api vía /api/salud
		client := &http.Client{Timeout: 800 * time.Millisecond}
		saludURL := fmt.Sprintf("http://127.0.0.1:%s/api/salud", currentPort)
		resp, pingErr := client.Get(saludURL)
		if pingErr != nil || resp == nil {
			// Fallback a /api/sistema/ping para compatibilidad
			resp, pingErr = client.Get(fmt.Sprintf("http://127.0.0.1:%s/api/sistema/ping", currentPort))
		}

		if pingErr == nil && resp != nil {
			var info struct {
				Status  string `json:"status"`
				App     string `json:"app"`
				Version string `json:"version"`
				PID     int    `json:"pid"`
			}
			decodeErr := json.NewDecoder(resp.Body).Decode(&info)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && decodeErr == nil && info.App == "sire-api" {
				if info.Version == AppVersion || info.Version == "" {
					return currentPort, true, nil
				}
				return "", false, fmt.Errorf("hay otra instancia de ATLOB Sync en ejecución con una versión distinta (versión detectada: %s, versión actual: %s) en el puerto %s; ciérrela desde el Administrador de tareas", info.Version, AppVersion, currentPort)
			}
		}
	}

	return "", false, errors.New("no se encontró ningún puerto disponible entre 8080 y 8095; cierre los procesos en conflicto")
}

// OpenFolder abre una carpeta en el explorador de archivos del sistema operativo
func OpenFolder(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	if runtime.GOOS == "windows" {
		return exec.Command("explorer.exe", absPath).Start()
	} else if runtime.GOOS == "darwin" {
		return exec.Command("open", absPath).Start()
	}
	return exec.Command("xdg-open", absPath).Start()
}

var allowedExternalHosts = map[string]bool{
	"wa.me":                    true,
	"api.whatsapp.com":         true,
	"web.whatsapp.com":         true,
	"e-menu.sunat.gob.pe":      true,
	"api-seguridad.sunat.gob.pe": true,
	"ww1.sunat.gob.pe":         true,
	"www.sunat.gob.pe":         true,
}

// ValidateExternalURL verifica que la URL externa use HTTPS y pertenezca a un dominio oficial autorizado (WhatsApp o SUNAT).
func ValidateExternalURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, errors.New("la URL no puede estar vacía")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || !parsed.IsAbs() {
		return nil, errors.New("URL externa inválida")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return nil, errors.New("solo se permiten enlaces HTTPS seguros")
	}
	if parsed.User != nil || parsed.Port() != "" {
		return nil, errors.New("la URL externa contiene parámetros de autoridad no permitidos")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if !allowedExternalHosts[host] {
		return nil, fmt.Errorf("dominio externo no autorizado: %s", host)
	}
	return parsed, nil
}

// OpenExternalURL valida y abre un enlace oficial autorizado en el navegador predeterminado del equipo.
func OpenExternalURL(rawURL string) error {
	parsed, err := ValidateExternalURL(rawURL)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", parsed.String()).Start()
	} else if runtime.GOOS == "darwin" {
		return exec.Command("open", parsed.String()).Start()
	}
	return exec.Command("xdg-open", parsed.String()).Start()
}

// ShowWindowsAlert muestra una alerta modal nativa de Windows en caso de fallo crítico en modo GUI
func ShowWindowsAlert(title, message string) {
	if runtime.GOOS == "windows" {
		user32 := syscall.NewLazyDLL("user32.dll")
		msgBox := user32.NewProc("MessageBoxW")
		tPtr, _ := syscall.UTF16PtrFromString(title)
		mPtr, _ := syscall.UTF16PtrFromString(message)
		msgBox.Call(0, uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), 0x00000010 /* MB_ICONERROR */)
	} else {
		log.Printf("[%s] %s", title, message)
	}
}

// ShowWindowsInfo muestra un diálogo informativo nativo de Windows
func ShowWindowsInfo(title, message string) {
	if runtime.GOOS == "windows" {
		user32 := syscall.NewLazyDLL("user32.dll")
		msgBox := user32.NewProc("MessageBoxW")
		tPtr, _ := syscall.UTF16PtrFromString(title)
		mPtr, _ := syscall.UTF16PtrFromString(message)
		msgBox.Call(0, uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), 0x00000040 /* MB_ICONINFORMATION */)
	} else {
		log.Printf("[%s] %s", title, message)
	}
}

// LoadConfig inicializa la configuración general del sistema
func LoadConfig() *Config {
	baseDir := GetBaseDir()
	_ = godotenv.Load(filepath.Join(baseDir, ".env"))

	if err := SetupDirectoriesAndLogging(baseDir); err != nil {
		log.Printf("[AVISO] No se pudo configurar directorio de logs: %v", err)
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1" // Seguridad: bind a 127.0.0.1 por defecto
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = "release"
	}

	masterKey, keySource, err := ResolveMasterKey(baseDir)
	if err != nil {
		ShowWindowsAlert("ATLOB Sync - Error de Seguridad", err.Error())
		log.Fatalf("\n===================================================================================\n"+
			"[ERROR FATAL DE CONFIGURACIÓN]\n%s\n"+
			"===================================================================================\n", err.Error())
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(baseDir, "data", "sire.db")
	} else if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(baseDir, dbPath)
	}

	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = filepath.Join(baseDir, "descargas")
	} else if !filepath.IsAbs(storageDir) {
		storageDir = filepath.Join(baseDir, storageDir)
	}

	logsDir := filepath.Join(baseDir, "data", "logs")
	webviewDataDir := filepath.Join(baseDir, "webview2")

	defaultRUC := os.Getenv("DEFAULT_RUC")
	if defaultRUC == "" {
		defaultRUC = os.Getenv("RUC")
	}

	AppConfig = &Config{
		Host:           host,
		Port:           port,
		GinMode:        ginMode,
		AppMasterKey:   masterKey,
		KeySource:      keySource,
		BaseDir:        baseDir,
		DBPath:         dbPath,
		StorageDir:     storageDir,
		LogsDir:        logsDir,
		WebViewDataDir: webviewDataDir,
		DefaultRUC:     defaultRUC,
	}

	return AppConfig
}
