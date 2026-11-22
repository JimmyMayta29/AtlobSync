//go:build !windows

package wininstall

import "errors"

const (
	AppDisplayName      = "ATLOB Sync · SUNAT"
	AppShortName        = "ATLOB Sync"
	AppPublisher        = "ATLOB Consultores"
	AppVersion          = "2.0.0"
	AppExeName          = "ATLOB_Sync.exe"
	UninstallerExeName  = "Desinstalar_ATLOB_Sync.exe"
	UninstallRegKeyPath = ""

	MBOK              = 0
	MBYesNo           = 4
	MBIconError       = 16
	MBIconQuestion    = 32
	MBIconWarning     = 48
	MBIconInformation = 64
	MBDefButton2      = 256
	IDYes             = 6
	IDNo              = 7
)

func DefaultInstallDir() string                                     { return "" }
func DefaultDataDir() string                                        { return "" }
func DesktopDir() string                                            { return "" }
func StartMenuProgramsDir() string                                  { return "" }
func MessageBox(title, message string, flags uint32) int            { return IDNo }
func StopRunningInstances()                                         {}
func CreateAppShortcuts(installDir string, desktop, startMenu bool) error {
	return errors.New("solo disponible en Windows")
}
func RemoveShortcuts() {}
func RegisterUninstallEntry(installDir string, installedBytes int64) error {
	return errors.New("solo disponible en Windows")
}
func UnregisterUninstallEntry()          {}
func ScheduleSelfCleanup(installDir string) {}
