package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBaseDir_Priority(t *testing.T) {
	t.Run("ATLOB_DATA_DIR tiene prioridad máxima", func(t *testing.T) {
		customDir := t.TempDir()
		t.Setenv("ATLOB_DATA_DIR", customDir)
		t.Setenv("ATLOB_PORTABLE", "")

		got := resolveBaseDir(`C:\Users\demo\AppData\Local\Programs\ATLOB Sync\ATLOB_Sync.exe`, nil)
		if filepath.Clean(got) != filepath.Clean(customDir) {
			t.Fatalf("esperado %q, obtenido %q", customDir, got)
		}
	})

	t.Run("Ejecutable instalado usa LOCALAPPDATA/ATLOB Sync", func(t *testing.T) {
		fakeLocalAppData := t.TempDir()
		t.Setenv("ATLOB_DATA_DIR", "")
		t.Setenv("ATLOB_PORTABLE", "")
		t.Setenv("LOCALAPPDATA", fakeLocalAppData)

		installedExe := filepath.Join(fakeLocalAppData, "Programs", "ATLOB Sync", "ATLOB_Sync.exe")
		got := resolveBaseDir(installedExe, nil)
		expected := filepath.Join(fakeLocalAppData, "ATLOB Sync")
		if filepath.Clean(got) != filepath.Clean(expected) {
			t.Fatalf("esperado %q, obtenido %q", expected, got)
		}
	})

	t.Run("Ejecutable portable con marcador .env usa su propia carpeta", func(t *testing.T) {
		portableDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(portableDir, ".env"), []byte("PORT=8080\n"), 0600); err != nil {
			t.Fatal(err)
		}
		fakeLocalAppData := t.TempDir()
		t.Setenv("ATLOB_DATA_DIR", "")
		t.Setenv("ATLOB_PORTABLE", "")
		t.Setenv("LOCALAPPDATA", fakeLocalAppData)

		exePath := filepath.Join(portableDir, "ATLOB_Sync.exe")
		got := resolveBaseDir(exePath, nil)
		if filepath.Clean(got) != filepath.Clean(portableDir) {
			t.Fatalf("esperado %q, obtenido %q", portableDir, got)
		}
	})

	t.Run("Ejecutable sin marcadores portables usa LOCALAPPDATA/ATLOB Sync", func(t *testing.T) {
		cleanDir := t.TempDir()
		fakeLocalAppData := t.TempDir()
		t.Setenv("ATLOB_DATA_DIR", "")
		t.Setenv("ATLOB_PORTABLE", "")
		t.Setenv("LOCALAPPDATA", fakeLocalAppData)

		exePath := filepath.Join(cleanDir, "ATLOB_Sync.exe")
		got := resolveBaseDir(exePath, nil)
		expected := filepath.Join(fakeLocalAppData, "ATLOB Sync")
		if filepath.Clean(got) != filepath.Clean(expected) {
			t.Fatalf("esperado %q, obtenido %q", expected, got)
		}
	})
}

func TestResolveMasterKey_GeneratesAndProtectsExistingDB(t *testing.T) {
	t.Setenv("APP_MASTER_KEY", "")

	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "data"), 0755); err != nil {
		t.Fatal(err)
	}

	// Primer arranque limpio genera data/app.key
	key1, src1, err := ResolveMasterKey(baseDir)
	if err != nil {
		t.Fatalf("error inesperado en primer arranque: %v", err)
	}
	if len(key1) != 64 || src1 == "" {
		t.Fatalf("llave generada inesperada: len=%d src=%q", len(key1), src1)
	}

	// Segundo arranque reutiliza data/app.key
	key2, src2, err := ResolveMasterKey(baseDir)
	if err != nil {
		t.Fatalf("error inesperado en segundo arranque: %v", err)
	}
	if key2 != key1 || src2 != "data/app.key" {
		t.Fatalf("se esperaba reutilizar llave existente: got=%q src=%q", key2, src2)
	}

	// Si se borra app.key pero existe sire.db, debe rechazar generar llave nueva
	_ = os.Remove(filepath.Join(baseDir, "data", "app.key"))
	if err := os.WriteFile(filepath.Join(baseDir, "data", "sire.db"), []byte("SQLite format 3\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolveMasterKey(baseDir); err == nil {
		t.Fatalf("se esperaba error al faltar app.key con sire.db existente")
	}
}
