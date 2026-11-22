//go:build !windows

package desktop

import "errors"

const (
	MainWindowTitle  = "ATLOB Sync · SUNAT — ATLOB Consultores"
	MainWindowIconID = 1
)

type WindowConfig struct {
	Title    string
	URL      string
	DataPath string
	Width    uint
	Height   uint
	Debug    bool
}

func IsWebView2Available() bool {
	return false
}

func BringExistingWindowToFront() bool {
	return false
}

func RunWindow(opts WindowConfig) error {
	return errors.New("la ventana nativa de escritorio WebView2 solo está disponible en Windows")
}
