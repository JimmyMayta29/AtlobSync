package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

type targetSpec struct {
	OutPath          string
	ProductName      string
	FileDescription  string
	InternalName     string
	OriginalFilename string
}

func main() {
	genWinres := flag.Bool("winres", false, "Genera archivos .syso con ícono ATLOB, metadatos y manifiesto High-DPI")
	gzipIn := flag.String("gzip-in", "", "Archivo de entrada a comprimir con gzip")
	gzipOut := flag.String("gzip-out", "", "Archivo de salida .gz")
	flag.Parse()

	if *genWinres {
		if err := generateAllWinres(); err != nil {
			log.Fatalf("error generando recursos Windows (.syso): %v", err)
		}
		fmt.Println("[BUILDTOOL] Recursos Windows (.syso) generados correctamente.")
		return
	}

	if *gzipIn != "" && *gzipOut != "" {
		if err := compressFile(*gzipIn, *gzipOut); err != nil {
			log.Fatalf("error comprimiendo %s -> %s: %v", *gzipIn, *gzipOut, err)
		}
		return
	}

	flag.Usage()
	os.Exit(2)
}

func generateAllWinres() error {
	iconPath := filepath.Join("web", "assets", "atlob-icon.png")
	f, err := os.Open(iconPath)
	if err != nil {
		return fmt.Errorf("no se pudo abrir %s: %w", iconPath, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("no se pudo decodificar PNG %s: %w", iconPath, err)
	}

	icon, err := winres.NewIconFromResizedImage(img, nil)
	if err != nil {
		return fmt.Errorf("no se pudo construir ícono multi-resolución: %w", err)
	}

	targets := []targetSpec{
		{
			OutPath:          "rsrc_windows_amd64.syso",
			ProductName:      "ATLOB Sync · SUNAT",
			FileDescription:  "ATLOB Sync · SUNAT — Aplicación Local de Escritorio",
			InternalName:     "ATLOB_Sync",
			OriginalFilename: "ATLOB_Sync.exe",
		},
		{
			OutPath:          filepath.Join("cmd", "installer", "rsrc_windows_amd64.syso"),
			ProductName:      "ATLOB Sync · SUNAT",
			FileDescription:  "Instalador de ATLOB Sync · SUNAT",
			InternalName:     "ATLOB_Sync_Instalador",
			OriginalFilename: "ATLOB_Sync_Instalador.exe",
		},
		{
			OutPath:          filepath.Join("cmd", "uninstaller", "rsrc_windows_amd64.syso"),
			ProductName:      "ATLOB Sync · SUNAT",
			FileDescription:  "Desinstalador de ATLOB Sync · SUNAT",
			InternalName:     "Desinstalar_ATLOB_Sync",
			OriginalFilename: "Desinstalar_ATLOB_Sync.exe",
		},
	}

	for _, t := range targets {
		if err := os.MkdirAll(filepath.Dir(t.OutPath), 0755); err != nil {
			return err
		}

		rs := winres.ResourceSet{}
		if err := rs.SetIcon(winres.ID(1), icon); err != nil {
			return fmt.Errorf("error asignando ícono en %s: %w", t.OutPath, err)
		}

		vi := version.Info{
			FileVersion:    [4]uint16{2, 0, 0, 0},
			ProductVersion: [4]uint16{2, 0, 0, 0},
		}
		const langESPE = 0x280A // es-PE
		for _, lang := range []uint16{version.LangDefault, langESPE} {
			_ = vi.Set(lang, version.CompanyName, "ATLOB Consultores")
			_ = vi.Set(lang, version.ProductName, t.ProductName)
			_ = vi.Set(lang, version.FileDescription, t.FileDescription)
			_ = vi.Set(lang, version.InternalName, t.InternalName)
			_ = vi.Set(lang, version.OriginalFilename, t.OriginalFilename)
			_ = vi.Set(lang, version.FileVersion, "2.0.0.0")
			_ = vi.Set(lang, version.ProductVersion, "2.0.0.0")
			_ = vi.Set(lang, version.LegalCopyright, "© 2026 ATLOB Consultores")
		}
		rs.SetVersionInfo(vi)

		rs.SetManifest(winres.AppManifest{
			Description:         t.FileDescription,
			ExecutionLevel:      winres.AsInvoker,
			DPIAwareness:        winres.DPIPerMonitorV2,
			UseCommonControlsV6: true,
			LongPathAware:       true,
		})

		out, err := os.Create(t.OutPath)
		if err != nil {
			return fmt.Errorf("no se pudo crear %s: %w", t.OutPath, err)
		}
		if err := rs.WriteObject(out, winres.ArchAMD64); err != nil {
			_ = out.Close()
			return fmt.Errorf("error escribiendo COFF %s: %w", t.OutPath, err)
		}
		if err := out.Close(); err != nil {
			return err
		}
	}

	return nil
}

func compressFile(inPath, outPath string) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	in, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}

	inInfo, _ := in.Stat()
	outInfo, _ := out.Stat()
	if inInfo != nil && outInfo != nil {
		fmt.Printf("[BUILDTOOL] Comprimido %s: %.2f MB -> %.2f MB\n",
			filepath.Base(inPath),
			float64(inInfo.Size())/(1024*1024),
			float64(outInfo.Size())/(1024*1024),
		)
	}
	return nil
}
