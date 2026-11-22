package main

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/SwodLore/sire-api/internal/desktop"
	"github.com/SwodLore/sire-api/internal/migration"
	"github.com/SwodLore/sire-api/internal/wininstall"
	"github.com/SwodLore/sire-api/web"
	"github.com/jchv/go-webview2"
)

//go:embed ui.html
var installerHTML string

type installerInitState struct {
	AppName    string                      `json:"app_name"`
	Version    string                      `json:"version"`
	InstallDir string                      `json:"install_dir"`
	DataDir    string                      `json:"data_dir"`
	IsUpdate   bool                        `json:"is_update"`
	Legacy     *migration.LegacySourceInfo `json:"legacy"`
}

type installOptions struct {
	DesktopShortcut   bool `json:"desktop_shortcut"`
	StartMenuShortcut bool `json:"start_menu_shortcut"`
	MigrateLegacy     bool `json:"migrate_legacy"`
	LaunchAfter       bool `json:"launch_after"`
	RegisterUninstall bool `json:"-"`
}

type installResult struct {
	Success    bool   `json:"success"`
	InstallDir string `json:"install_dir"`
	DataDir    string `json:"data_dir"`
	Summary    string `json:"summary"`
}

type cliFlags struct {
	Silent      bool
	InstallDir  string
	DataDir     string
	NoShortcuts bool
	NoRegistry  bool
	NoMigrate   bool
	NoLaunch    bool
}

func parseFlags() cliFlags {
	var f cliFlags
	// Soportar también bandera estilo Windows /S
	cleanedArgs := make([]string, 0, len(os.Args)-1)
	for _, a := range os.Args[1:] {
		if a == "/S" || a == "/s" {
			f.Silent = true
			continue
		}
		cleanedArgs = append(cleanedArgs, a)
	}

	fs := flag.NewFlagSet("ATLOB_Sync_Instalador", flag.ContinueOnError)
	fs.BoolVar(&f.Silent, "silent", f.Silent, "Instalación silenciosa sin interfaz")
	fs.StringVar(&f.InstallDir, "install-dir", "", "Directorio de instalación personalizado")
	fs.StringVar(&f.DataDir, "data-dir", "", "Directorio de datos personalizado")
	fs.BoolVar(&f.NoShortcuts, "no-shortcuts", false, "No crear accesos directos")
	fs.BoolVar(&f.NoRegistry, "no-registry", false, "No registrar entrada de desinstalación en Windows")
	fs.BoolVar(&f.NoMigrate, "no-migrate", false, "No migrar datos locales heredados")
	fs.BoolVar(&f.NoLaunch, "no-launch", false, "No abrir la aplicación al terminar")
	_ = fs.Parse(cleanedArgs)

	if strings.TrimSpace(f.InstallDir) == "" {
		f.InstallDir = wininstall.DefaultInstallDir()
	}
	if strings.TrimSpace(f.DataDir) == "" {
		f.DataDir = wininstall.DefaultDataDir()
	}
	return f
}

func getInstallerDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	wd, _ := os.Getwd()
	return wd
}

func main() {
	runtime.LockOSThread()

	cfg := parseFlags()
	installerDir := getInstallerDir()
	legacy := migration.DetectLegacySource(installerDir, cfg.DataDir)

	// Modo silencioso (/S o --silent)
	if cfg.Silent {
		opts := installOptions{
			DesktopShortcut:   !cfg.NoShortcuts,
			StartMenuShortcut: !cfg.NoShortcuts,
			MigrateLegacy:     !cfg.NoMigrate && legacy.CanMigrate,
			LaunchAfter:       !cfg.NoLaunch,
			RegisterUninstall: !cfg.NoRegistry,
		}
		res, err := executeInstall(cfg.InstallDir, cfg.DataDir, legacy, opts, func(step string, pct int) {
			fmt.Printf("[%3d%%] %s\n", pct, step)
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[OK] %s\n", res.Summary)
		if opts.LaunchAfter {
			launchInstalledApp(cfg.InstallDir, cfg.DataDir)
		}
		return
	}

	// Modo interactivo GUI nativo (WebView2 con respaldo automático a MessageBox nativo)
	if desktop.IsWebView2Available() {
		if runInstallerWebView(cfg, legacy) {
			return
		}
	}

	runFallbackMessageBoxInstaller(cfg, legacy)
}

func runInstallerWebView(cfg cliFlags, legacy *migration.LegacySourceInfo) bool {
	tmpWV2Dir, err := os.MkdirTemp("", "atlob_sync_installer_wv2_*")
	if err != nil {
		tmpWV2Dir = filepath.Join(os.TempDir(), "atlob_sync_installer_wv2")
	}
	defer os.RemoveAll(tmpWV2Dir)

	winW, winH := desktop.ComputeSafeWindowSize(590, 540)
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  tmpWV2Dir,
		WindowOptions: webview2.WindowOptions{
			Title:  "Instalador — " + wininstall.AppDisplayName,
			Width:  winW,
			Height: winH,
			IconId: desktop.MainWindowIconID,
			Center: true,
		},
	})
	if w == nil {
		return false
	}
	defer w.Destroy()
	w.SetSize(int(winW), int(winH), webview2.HintFixed)

	isUpdate := fileExists(filepath.Join(cfg.InstallDir, wininstall.AppExeName))
	var installMu sync.Mutex
	installing := false

	_ = w.Bind("installerInit", func() installerInitState {
		return installerInitState{
			AppName:    wininstall.AppDisplayName,
			Version:    wininstall.AppVersion,
			InstallDir: cfg.InstallDir,
			DataDir:    cfg.DataDir,
			IsUpdate:   isUpdate,
			Legacy:     legacy,
		}
	})

	_ = w.Bind("installerStart", func(opts installOptions) error {
		installMu.Lock()
		if installing {
			installMu.Unlock()
			return errors.New("la instalación ya está en progreso")
		}
		installing = true
		installMu.Unlock()

		opts.RegisterUninstall = !cfg.NoRegistry
		if cfg.NoShortcuts {
			opts.DesktopShortcut = false
			opts.StartMenuShortcut = false
		}
		if cfg.NoMigrate {
			opts.MigrateLegacy = false
		}

		go func() {
			defer func() {
				installMu.Lock()
				installing = false
				installMu.Unlock()
			}()

			res, err := executeInstall(cfg.InstallDir, cfg.DataDir, legacy, opts, func(step string, pct int) {
				stepJSON, _ := json.Marshal(step)
				w.Dispatch(func() {
					w.Eval(fmt.Sprintf("window.__onInstallProgress(%s, %d);", string(stepJSON), pct))
				})
			})
			if err != nil {
				errJSON, _ := json.Marshal(err.Error())
				w.Dispatch(func() {
					w.Eval(fmt.Sprintf("window.__onInstallError(%s);", string(errJSON)))
				})
				return
			}
			resJSON, _ := json.Marshal(res)
			w.Dispatch(func() {
				w.Eval(fmt.Sprintf("window.__onInstallDone(%s);", string(resJSON)))
			})
		}()

		return nil
	})

	_ = w.Bind("installerFinish", func(launchApp bool) {
		if launchApp {
			launchInstalledApp(cfg.InstallDir, cfg.DataDir)
		}
		w.Dispatch(func() {
			w.Terminate()
		})
	})

	logoURI := buildLogoDataURI()
	renderedHTML := strings.ReplaceAll(installerHTML, "{{LOGO_DATA_URI}}", logoURI)
	w.SetHtml(renderedHTML)
	w.Run()
	return true
}

func runFallbackMessageBoxInstaller(cfg cliFlags, legacy *migration.LegacySourceInfo) {
	msg := fmt.Sprintf("¿Desea instalar %s v%s en este equipo?\n\nCarpeta de aplicación:\n%s\n\nCarpeta de datos:\n%s",
		wininstall.AppDisplayName, wininstall.AppVersion, cfg.InstallDir, cfg.DataDir)
	if legacy.CanMigrate {
		msg += fmt.Sprintf("\n\nSe migrarán automáticamente sus datos existentes desde API-SIRE (%d empresa(s) y %d archivo(s)).",
			legacy.CompaniesCount, legacy.DownloadsFiles)
	}

	ans := wininstall.MessageBox("Instalador — "+wininstall.AppDisplayName, msg, wininstall.MBYesNo|wininstall.MBIconQuestion)
	if ans != wininstall.IDYes {
		return
	}

	opts := installOptions{
		DesktopShortcut:   !cfg.NoShortcuts,
		StartMenuShortcut: !cfg.NoShortcuts,
		MigrateLegacy:     !cfg.NoMigrate && legacy.CanMigrate,
		LaunchAfter:       !cfg.NoLaunch,
		RegisterUninstall: !cfg.NoRegistry,
	}
	res, err := executeInstall(cfg.InstallDir, cfg.DataDir, legacy, opts, nil)
	if err != nil {
		wininstall.MessageBox("Error de instalación", "No se pudo completar la instalación:\n\n"+err.Error(), wininstall.MBOK|wininstall.MBIconError)
		os.Exit(1)
	}

	wininstall.MessageBox("Instalación completada", res.Summary, wininstall.MBOK|wininstall.MBIconInformation)
	if opts.LaunchAfter {
		launchInstalledApp(cfg.InstallDir, cfg.DataDir)
	}
}

func executeInstall(
	installDir string,
	dataDir string,
	legacy *migration.LegacySourceInfo,
	opts installOptions,
	progress func(step string, pct int),
) (*installResult, error) {
	emit := func(step string, pct int) {
		if progress != nil {
			progress(step, pct)
		}
	}

	if len(appPayloadGz) == 0 || len(uninstallerPayloadGz) == 0 {
		return nil, errors.New("el instalador fue compilado sin los binarios embebidos (-tags installer_bundle)")
	}

	emit("Verificando instancias activas de ATLOB Sync...", 10)
	wininstall.StopRunningInstances()

	emit("Preparando carpetas locales de instalación...", 20)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, fmt.Errorf("no se pudo crear carpeta de instalación '%s': %w", installDir, err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "data", "logs"), 0755); err != nil {
		return nil, fmt.Errorf("no se pudo crear carpeta de datos '%s': %w", dataDir, err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "descargas"), 0755); err != nil {
		return nil, fmt.Errorf("no se pudo crear carpeta de descargas '%s': %w", dataDir, err)
	}

	emit("Instalando aplicación de escritorio ATLOB_Sync.exe...", 40)
	appDst := filepath.Join(installDir, wininstall.AppExeName)
	appBytes, err := extractGzipAtomic(appPayloadGz, appDst)
	if err != nil {
		return nil, fmt.Errorf("error instalando %s: %w", wininstall.AppExeName, err)
	}

	emit("Instalando desinstalador de Windows...", 55)
	uninstDst := filepath.Join(installDir, wininstall.UninstallerExeName)
	uninstBytes, err := extractGzipAtomic(uninstallerPayloadGz, uninstDst)
	if err != nil {
		return nil, fmt.Errorf("error instalando %s: %w", wininstall.UninstallerExeName, err)
	}

	var migSummary string
	if opts.MigrateLegacy && legacy != nil && legacy.CanMigrate {
		emit("Migrando empresas, llave criptográfica y descargas existentes...", 70)
		rep, migErr := migration.MigrateLegacyData(legacy.SourceDir, dataDir, func(msg string) {
			emit(msg, 76)
		})
		if migErr != nil {
			return nil, fmt.Errorf("error migrando datos existentes: %w", migErr)
		}
		migSummary = fmt.Sprintf(" Datos migrados: %d empresa(s), %d archivo(s) y %d ruta(s) normalizadas.",
			rep.CompaniesCount, rep.CopiedFiles, rep.NormalizedPaths)
	} else {
		emit("Verificando datos locales existentes...", 75)
	}

	if opts.DesktopShortcut || opts.StartMenuShortcut {
		emit("Creando accesos directos con ícono ATLOB...", 85)
		if err := wininstall.CreateAppShortcuts(installDir, opts.DesktopShortcut, opts.StartMenuShortcut); err != nil {
			return nil, err
		}
	}

	if opts.RegisterUninstall {
		emit("Registrando aplicación en Windows...", 94)
		if err := wininstall.RegisterUninstallEntry(installDir, appBytes+uninstBytes); err != nil {
			return nil, err
		}
	}

	emit("Finalizando instalación...", 100)
	summary := fmt.Sprintf("ATLOB Sync · SUNAT v%s instalado en %s.%s", wininstall.AppVersion, installDir, migSummary)
	return &installResult{
		Success:    true,
		InstallDir: installDir,
		DataDir:    dataDir,
		Summary:    summary,
	}, nil
}

func extractGzipAtomic(gzBytes []byte, dstPath string) (int64, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gzBytes))
	if err != nil {
		return 0, err
	}
	defer zr.Close()

	tmpPath := dstPath + ".tmp"
	_ = os.Remove(tmpPath)

	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return 0, err
	}

	n, copyErr := io.Copy(out, zr)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return 0, closeErr
	}
	if n == 0 {
		_ = os.Remove(tmpPath)
		return 0, errors.New("el archivo extraído está vacío")
	}

	_ = os.Remove(dstPath)
	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, err
	}
	return n, nil
}

func launchInstalledApp(installDir, dataDir string) {
	appExe := filepath.Join(installDir, wininstall.AppExeName)
	cmd := exec.Command(appExe)
	cmd.Dir = installDir
	// Solo inyectar ATLOB_DATA_DIR si difiere del estándar
	if !strings.EqualFold(filepath.Clean(dataDir), filepath.Clean(wininstall.DefaultDataDir())) {
		cmd.Env = append(os.Environ(), "ATLOB_DATA_DIR="+dataDir)
	}
	_ = cmd.Start()
}

func buildLogoDataURI() string {
	data, err := web.GetBrandAsset("atlob-logo-horizontal-dark.png")
	if err != nil || len(data) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
