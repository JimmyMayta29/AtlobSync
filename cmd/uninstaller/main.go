package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SwodLore/sire-api/internal/wininstall"
)

func main() {
	silent := false
	deleteData := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--silent", "/S", "/s":
			silent = true
		case "--purge-data":
			deleteData = true
		}
	}

	installDir := wininstall.DefaultInstallDir()
	if execPath, err := os.Executable(); err == nil {
		dir := filepath.Dir(execPath)
		if filepath.Base(dir) == wininstall.AppShortName {
			installDir = dir
		}
	}
	dataDir := wininstall.DefaultDataDir()

	if !silent {
		ans := wininstall.MessageBox(
			"Desinstalar "+wininstall.AppDisplayName,
			fmt.Sprintf("¿Desea desinstalar %s de este equipo?\n\nCarpeta del programa:\n%s", wininstall.AppDisplayName, installDir),
			wininstall.MBYesNo|wininstall.MBIconQuestion,
		)
		if ans != wininstall.IDYes {
			return
		}

		// Preguntar explícitamente (por defecto NO) si desea eliminar datos locales
		if _, err := os.Stat(dataDir); err == nil {
			ansData := wininstall.MessageBox(
				"Conservar datos locales",
				fmt.Sprintf("¿Desea ELIMINAR también sus datos locales (empresas registradas, llave criptográfica y descargas)?\n\nUbicación de datos:\n%s\n\nSeleccione «No» para conservarlos de forma segura.", dataDir),
				wininstall.MBYesNo|wininstall.MBIconWarning|wininstall.MBDefButton2,
			)
			if ansData == wininstall.IDYes {
				deleteData = true
			}
		}
	}

	// 1. Cerrar aplicación si está abierta
	wininstall.StopRunningInstances()

	// 2. Eliminar accesos directos y registro en Windows
	wininstall.RemoveShortcuts()
	wininstall.UnregisterUninstallEntry()

	// 3. Eliminar ejecutable principal
	_ = os.Remove(filepath.Join(installDir, wininstall.AppExeName))

	// 4. Eliminar datos solo si el usuario confirmó explícitamente
	if deleteData && dataDir != "" {
		_ = os.RemoveAll(dataDir)
	}

	// 5. Programar limpieza de la carpeta del desinstalador al cerrar este proceso
	wininstall.ScheduleSelfCleanup(installDir)

	if !silent {
		msg := wininstall.AppDisplayName + " se desinstaló correctamente."
		if !deleteData {
			msg += fmt.Sprintf("\n\nSus datos locales se conservaron en:\n%s", dataDir)
		}
		wininstall.MessageBox("Desinstalación completada", msg, wininstall.MBOK|wininstall.MBIconInformation)
	}
}
