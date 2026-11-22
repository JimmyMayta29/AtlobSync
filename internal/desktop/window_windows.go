//go:build windows

package desktop

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows/registry"
)

const (
	// MainWindowTitle es el título oficial de la ventana nativa de escritorio.
	MainWindowTitle = "ATLOB Sync · SUNAT — ATLOB Consultores"

	// MainWindowIconID coincide con winres.ID(1) incrustado en el ejecutable.
	MainWindowIconID = 1

	webview2ClientGUID = "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procGetWindowTextLenW   = user32.NewProc("GetWindowTextLengthW")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procIsIconic            = user32.NewProc("IsIconic")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop    = user32.NewProc("BringWindowToTop")
)

const (
	smCxScreen = 0
	smCyScreen = 1
	swShow     = 5
	swRestore  = 9
)

// WindowConfig parametriza la ventana nativa de escritorio.
type WindowConfig struct {
	Title    string
	URL      string
	DataPath string
	Width    uint
	Height   uint
	Debug    bool
}

// IsWebView2Available comprueba vía registro oficial de Windows si Edge WebView2 Runtime está instalado.
func IsWebView2Available() bool {
	checks := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\` + webview2ClientGUID},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\EdgeUpdate\Clients\` + webview2ClientGUID},
		{registry.CURRENT_USER, `Software\Microsoft\EdgeUpdate\Clients\` + webview2ClientGUID},
	}

	for _, chk := range checks {
		k, err := registry.OpenKey(chk.root, chk.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		pv, _, err := k.GetStringValue("pv")
		_ = k.Close()
		pv = strings.TrimSpace(pv)
		if err == nil && pv != "" && pv != "0.0.0.0" {
			return true
		}
	}
	return false
}

// ComputeSafeWindowSize acota el tamaño deseado a las dimensiones reales de pantalla para evitar desbordamientos.
func ComputeSafeWindowSize(desiredW, desiredH uint) (uint, uint) {
	if desiredW == 0 {
		desiredW = 1320
	}
	if desiredH == 0 {
		desiredH = 840
	}

	screenW, _, _ := procGetSystemMetrics.Call(smCxScreen)
	screenH, _, _ := procGetSystemMetrics.Call(smCyScreen)

	sw := uint(screenW)
	sh := uint(screenH)

	if sw > 160 && desiredW > sw-60 {
		desiredW = sw - 60
	}
	if sh > 160 && desiredH > sh-80 {
		desiredH = sh - 80
	}

	if desiredW < 640 && sw >= 640 {
		desiredW = 640
	}
	if desiredH < 480 && sh >= 480 {
		desiredH = 480
	}
	return desiredW, desiredH
}

// BringExistingWindowToFront busca una ventana abierta de ATLOB Sync y la trae al primer plano.
func BringExistingWindowToFront() bool {
	return bringWindowByPrefix("ATLOB Sync")
}

func bringWindowByPrefix(prefix string) bool {
	var found uintptr
	prefixLower := strings.ToLower(prefix)

	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		length, _, _ := procGetWindowTextLenW.Call(hwnd)
		if length == 0 {
			return 1
		}
		buf := make([]uint16, length+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
		title := syscall.UTF16ToString(buf)
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(title)), prefixLower) {
			found = hwnd
			return 0 // detener enumeración
		}
		return 1
	})

	_, _, _ = procEnumWindows.Call(cb, 0)
	if found == 0 {
		return false
	}

	iconic, _, _ := procIsIconic.Call(found)
	if iconic != 0 {
		_, _, _ = procShowWindow.Call(found, swRestore)
	} else {
		_, _, _ = procShowWindow.Call(found, swShow)
	}
	_, _, _ = procBringWindowToTop.Call(found)
	_, _, _ = procSetForegroundWindow.Call(found)
	return true
}

// RunWindow abre la interfaz local en una ventana de escritorio nativa (WebView2) y bloquea hasta su cierre.
func RunWindow(opts WindowConfig) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = MainWindowTitle
	}
	if opts.DataPath != "" {
		_ = os.MkdirAll(opts.DataPath, 0755)
	}

	wWidth, wHeight := ComputeSafeWindowSize(opts.Width, opts.Height)

	if IsWebView2Available() {
		w := webview2.NewWithOptions(webview2.WebViewOptions{
			Debug:     opts.Debug,
			AutoFocus: true,
			DataPath:  opts.DataPath,
			WindowOptions: webview2.WindowOptions{
				Title:  title,
				Width:  wWidth,
				Height: wHeight,
				IconId: MainWindowIconID,
				Center: true,
			},
		})
		if w != nil {
			minW, minH := 920, 600
			if int(wWidth) < minW {
				minW = int(wWidth)
			}
			if int(wHeight) < minH {
				minH = int(wHeight)
			}
			w.SetSize(minW, minH, webview2.HintMin)
			w.Navigate(opts.URL)
			w.Run()
			w.Destroy()
			return nil
		}
		log.Printf("[DESKTOP] No se pudo inicializar WebView2; intentando modo ventana de aplicación local.")
	} else {
		log.Printf("[DESKTOP] Edge WebView2 Runtime no detectado; intentando modo ventana de aplicación local.")
	}

	return runStandaloneAppWindowFallback(opts.URL, opts.DataPath, wWidth, wHeight)
}

func runStandaloneAppWindowFallback(targetURL, dataPath string, width, height uint) error {
	browserPath := findLocalAppBrowser()
	if browserPath == "" {
		return errors.New("no se encontró Microsoft Edge WebView2 Runtime ni Edge/Chrome instalado en este equipo; instale Microsoft Edge WebView2 Runtime para abrir ATLOB Sync en escritorio")
	}

	profileDir := filepath.Join(dataPath, "app_window_profile")
	_ = os.MkdirAll(profileDir, 0755)

	args := []string{
		"--app=" + targetURL,
		"--user-data-dir=" + profileDir,
		fmt.Sprintf("--window-size=%d,%d", width, height),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
	}

	cmd := exec.Command(browserPath, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error iniciando ventana local de aplicación: %w", err)
	}
	_ = cmd.Wait()
	return nil
}

func findLocalAppBrowser() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, p := range candidates {
		if p != "" {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return p
			}
		}
	}
	for _, bin := range []string{"msedge.exe", "chrome.exe"} {
		if found, err := exec.LookPath(bin); err == nil {
			return found
		}
	}
	return ""
}
