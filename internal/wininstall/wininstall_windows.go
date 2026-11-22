//go:build windows

package wininstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

const (
	AppDisplayName      = "ATLOB Sync · SUNAT"
	AppShortName        = "ATLOB Sync"
	AppPublisher        = "ATLOB Consultores"
	AppVersion          = "2.0.0"
	AppExeName          = "ATLOB_Sync.exe"
	UninstallerExeName  = "Desinstalar_ATLOB_Sync.exe"
	UninstallRegKeyPath = `Software\Microsoft\Windows\CurrentVersion\Uninstall\ATLOBSync`

	MBOK              = 0x00000000
	MBYesNo           = 0x00000004
	MBIconError       = 0x00000010
	MBIconQuestion    = 0x00000020
	MBIconWarning     = 0x00000030
	MBIconInformation = 0x00000040
	MBDefButton2      = 0x00000100

	IDYes = 6
	IDNo  = 7

	csidlPrograms         = 0x0002
	csidlDesktopDirectory = 0x0010
	createNoWindow        = 0x08000000
)

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	procSHGetFolderPathW = shell32.NewProc("SHGetFolderPathW")
	user32               = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW      = user32.NewProc("MessageBoxW")
)

// DefaultInstallDir retorna la carpeta de instalación sin privilegios de administrador (%LocalAppData%\Programs\ATLOB Sync).
func DefaultInstallDir() string {
	if base := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); base != "" {
		return filepath.Join(base, "Programs", AppShortName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local", "Programs", AppShortName)
}

// DefaultDataDir retorna la carpeta de datos locales del usuario (%LocalAppData%\ATLOB Sync).
func DefaultDataDir() string {
	if base := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); base != "" {
		return filepath.Join(base, AppShortName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local", AppShortName)
}

func getShellFolderPath(csidl uintptr) string {
	buf := make([]uint16, 260)
	hr, _, _ := procSHGetFolderPathW.Call(0, csidl, 0, 0, uintptr(unsafe.Pointer(&buf[0])))
	if hr == 0 {
		if p := strings.TrimSpace(syscall.UTF16ToString(buf)); p != "" {
			return p
		}
	}
	return ""
}

// DesktopDir obtiene la ruta real del Escritorio del usuario (compatible con redirección de OneDrive).
func DesktopDir() string {
	if p := getShellFolderPath(csidlDesktopDirectory); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}

// StartMenuProgramsDir obtiene la carpeta Programas del Menú Inicio del usuario actual.
func StartMenuProgramsDir() string {
	if p := getShellFolderPath(csidlPrograms); p != "" {
		return p
	}
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs")
}

// MessageBox muestra un cuadro de diálogo nativo de Windows.
func MessageBox(title, message string, flags uint32) int {
	tPtr, _ := syscall.UTF16PtrFromString(title)
	mPtr, _ := syscall.UTF16PtrFromString(message)
	ret, _, _ := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), uintptr(flags))
	return int(ret)
}

// StopRunningInstances detiene instancias activas de ATLOB_Sync.exe o sire_api.exe antes de instalar/desinstalar.
func StopRunningInstances() {
	names := []string{AppExeName, "sire_api.exe"}
	for _, img := range names {
		cmd := exec.Command("taskkill.exe", "/IM", img, "/T", "/F")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
		_ = cmd.Run()
	}
	time.Sleep(450 * time.Millisecond)
}

// CreateShortcut crea un acceso directo .lnk de Windows usando WScript.Shell con parámetros pasados por entorno.
func CreateShortcut(lnkPath, targetPath, workingDir, iconLocation, description string) error {
	if err := os.MkdirAll(filepath.Dir(lnkPath), 0755); err != nil {
		return fmt.Errorf("error creando carpeta de acceso directo: %w", err)
	}

	psScript := `$ws = New-Object -ComObject WScript.Shell; ` +
		`$sc = $ws.CreateShortcut($env:ATLOB_LNK_PATH); ` +
		`$sc.TargetPath = $env:ATLOB_LNK_TARGET; ` +
		`$sc.WorkingDirectory = $env:ATLOB_LNK_WORKDIR; ` +
		`$sc.IconLocation = $env:ATLOB_LNK_ICON; ` +
		`$sc.Description = $env:ATLOB_LNK_DESC; ` +
		`$sc.Save()`

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	cmd.Env = append(os.Environ(),
		"ATLOB_LNK_PATH="+lnkPath,
		"ATLOB_LNK_TARGET="+targetPath,
		"ATLOB_LNK_WORKDIR="+workingDir,
		"ATLOB_LNK_ICON="+iconLocation,
		"ATLOB_LNK_DESC="+description,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error creando acceso directo '%s': %w (%s)", lnkPath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CreateAppShortcuts crea los accesos directos solicitados en Escritorio y/o Menú Inicio.
func CreateAppShortcuts(installDir string, desktop, startMenu bool) error {
	appExe := filepath.Join(installDir, AppExeName)
	uninstExe := filepath.Join(installDir, UninstallerExeName)
	iconLoc := appExe + ",0"

	if desktop {
		lnk := filepath.Join(DesktopDir(), AppShortName+".lnk")
		if err := CreateShortcut(lnk, appExe, installDir, iconLoc, AppDisplayName+" — Aplicación Local"); err != nil {
			return err
		}
	}

	if startMenu {
		menuDir := filepath.Join(StartMenuProgramsDir(), AppShortName)
		appLnk := filepath.Join(menuDir, AppShortName+".lnk")
		if err := CreateShortcut(appLnk, appExe, installDir, iconLoc, AppDisplayName+" — Aplicación Local"); err != nil {
			return err
		}
		uninstLnk := filepath.Join(menuDir, "Desinstalar "+AppShortName+".lnk")
		if err := CreateShortcut(uninstLnk, uninstExe, installDir, uninstExe+",0", "Desinstalar "+AppDisplayName); err != nil {
			return err
		}
	}

	return nil
}

// RemoveShortcuts elimina los accesos directos de Escritorio y Menú Inicio.
func RemoveShortcuts() {
	_ = os.Remove(filepath.Join(DesktopDir(), AppShortName+".lnk"))
	_ = os.RemoveAll(filepath.Join(StartMenuProgramsDir(), AppShortName))
}

// RegisterUninstallEntry registra ATLOB Sync en Aplicaciones instaladas de Windows (HKCU, sin requerir admin).
func RegisterUninstallEntry(installDir string, installedBytes int64) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, UninstallRegKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("error abriendo clave de registro de desinstalación: %w", err)
	}
	defer key.Close()

	appExe := filepath.Join(installDir, AppExeName)
	uninstExe := filepath.Join(installDir, UninstallerExeName)

	estimatedKB := uint32(installedBytes / 1024)
	if estimatedKB < 1024 {
		estimatedKB = 30720
	}

	strVals := map[string]string{
		"DisplayName":     AppDisplayName,
		"DisplayVersion":  AppVersion,
		"Publisher":       AppPublisher,
		"DisplayIcon":     appExe + ",0",
		"InstallLocation": installDir,
		"UninstallString": fmt.Sprintf(`"%s"`, uninstExe),
		"QuietUninstallString": fmt.Sprintf(`"%s" --silent`, uninstExe),
		"InstallDate":     time.Now().Format("20060102"),
	}
	for k, v := range strVals {
		if err := key.SetStringValue(k, v); err != nil {
			return err
		}
	}

	_ = key.SetDWordValue("NoModify", 1)
	_ = key.SetDWordValue("NoRepair", 1)
	_ = key.SetDWordValue("EstimatedSize", estimatedKB)
	return nil
}

// UnregisterUninstallEntry elimina el registro de desinstalación en HKCU.
func UnregisterUninstallEntry() {
	_ = registry.DeleteKey(registry.CURRENT_USER, UninstallRegKeyPath)
}

// ScheduleSelfCleanup elimina la carpeta de instalación unos segundos después de cerrar el desinstalador.
func ScheduleSelfCleanup(installDir string) {
	cleaned := filepath.Clean(installDir)
	if cleaned == "" || cleaned == "." || cleaned == `\` || len(cleaned) < 10 {
		return
	}
	arg := fmt.Sprintf(`timeout /t 2 /nobreak >NUL & rd /S /Q "%s"`, cleaned)
	cmd := exec.Command("cmd.exe", "/C", arg)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	_ = cmd.Start()
}
