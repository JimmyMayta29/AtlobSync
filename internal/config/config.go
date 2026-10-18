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
	Host         string `json:"host"`
	Port         string `json:"port"`
	GinMode      string `json:"gin_mode"`
	AppMasterKey string `json:"-"`
	KeySource    string `json:"key_source"`
	BaseDir      string `json:"base_dir"`
	DBPath       string `json:"db_path"`
	StorageDir   string `json:"storage_dir"`
	LogsDir      string `json:"logs_dir"`
	DefaultRUC   string `json:"default_ruc"`
}

var AppConfig *Config

// GetBaseDir obtiene la ruta base del ejecutable o del proyecto
func GetBaseDir() string {
	execPath, err := os.Executable()
	if err == nil && !strings.Contains(execPath, "go-build") && !strings.Contains(execPath, "Temp") {
		return filepath.Dir(execPath)
	}
	// Fallback para ejecución desde go run / go test
	dir, err := os.Getwd()
	if err == nil {
		return dir
	}
	return "."
}

// SetupDirectoriesAndLogging asegura la existencia de carpetas requeridas y configura logging en archivo
func SetupDirectoriesAndLogging(baseDir string) error {
	dataDir := filepath.Join(baseDir, "data")
	logsDir := filepath.Join(dataDir, "logs")
	storageDir := filepath.Join(baseDir, "descargas")

	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("error al crear directorio de logs: %w", err)
	}
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return fmt.Errorf("error al crear directorio de descargas: %w", err)
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
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var info struct {
					Status  string `json:"status"`
					App     string `json:"app"`
					Version string `json:"version"`
					PID     int    `json:"pid"`
				}
				if json.NewDecoder(resp.Body).Decode(&info) == nil && info.App == "sire-api" {
					if info.Version == AppVersion || info.Version == "" {
						return currentPort, true, nil
					}
					return "", false, fmt.Errorf("hay otra instancia de sire_api.exe en ejecución con una versión distinta (versión detectada: %s, versión actual: %s) en el puerto %s; ciérrela desde el Administrador de tareas", info.Version, AppVersion, currentPort)
				}
			}
		}
	}

	return "", false, errors.New("no se encontró ningún puerto disponible entre 8080 y 8095; cierre los procesos en conflicto")
}

// OpenBrowser abre la URL en el navegador predeterminado del sistema operativo
func OpenBrowser(targetURL string) {
	go func() {
		time.Sleep(350 * time.Millisecond)
		if runtime.GOOS == "windows" {
			_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL).Start()
		} else if runtime.GOOS == "darwin" {
			_ = exec.Command("open", targetURL).Start()
		} else {
			_ = exec.Command("xdg-open", targetURL).Start()
		}
	}()
}

// OpenFolder abre una carpeta en el explorador de archivos de Windows
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
		ShowWindowsAlert("SIRE API - Error de Seguridad", err.Error())
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

	defaultRUC := os.Getenv("DEFAULT_RUC")
	if defaultRUC == "" {
		defaultRUC = os.Getenv("RUC")
	}

	AppConfig = &Config{
		Host:         host,
		Port:         port,
		GinMode:      ginMode,
		AppMasterKey: masterKey,
		KeySource:    keySource,
		BaseDir:      baseDir,
		DBPath:       dbPath,
		StorageDir:   storageDir,
		LogsDir:      logsDir,
		DefaultRUC:   defaultRUC,
	}

	return AppConfig
}
