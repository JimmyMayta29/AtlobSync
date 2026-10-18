//go:build ignore

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/SwodLore/sire-api/internal/crypto"
	"github.com/go-resty/resty/v2"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

// Script de diagnóstico de una sola llamada hacia SUNAT (GET periodos/140000).
// Diseñado para ser ejecutado únicamente cuando el usuario escriba "reanudar".
// Reutiliza el token vigente en caché (cifrado con AES-GCM) sin solicitar nuevos tokens.
// Imprime todos los headers de respuesta (sin cookies) y el cuerpo sanitizado para comparar con Postman.
func main() {
	_ = godotenv.Load()

	masterKey := os.Getenv("APP_MASTER_KEY")
	if masterKey == "" {
		fmt.Println("[DIAGNOSTICO] Error: Variable APP_MASTER_KEY no configurada.")
		os.Exit(1)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/sire.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Printf("[DIAGNOSTICO] Error al conectar a la base de datos SQLite: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	cryptoSvc, err := crypto.NewCryptoService(masterKey)
	if err != nil {
		fmt.Printf("[DIAGNOSTICO] Error al inicializar servicio criptográfico: %v\n", err)
		os.Exit(1)
	}

	ruc := os.Getenv("DEFAULT_RUC")
	if ruc == "" {
		ruc = "20000000001"
	}

	// 1. Obtener token cifrado de SQLite
	var tokenEnc, expStr string
	err = db.QueryRow("SELECT token_enc, expires_at FROM tokens_cache WHERE ruc = ?", ruc).Scan(&tokenEnc, &expStr)
	if err != nil {
		fmt.Printf("[DIAGNOSTICO] No existe token en caché para el RUC %s. (No se solicitan nuevos tokens a SUNAT por directiva de seguridad).\n", ruc)
		os.Exit(0)
	}

	// 2. Verificar vigencia del token
	exp, err := time.Parse(time.RFC3339, expStr)
	if err != nil {
		exp, err = time.ParseInLocation("2006-01-02 15:04:05", expStr, time.Local)
	}
	if err != nil || time.Now().After(exp) {
		fmt.Printf("[DIAGNOSTICO] El token en caché para RUC %s ha expirado (Registrado: %s). Por directiva de seguridad no se solicita nuevo token.\n", ruc, expStr)
		os.Exit(0)
	}

	// 3. Descifrar el token usando AES-256-GCM
	cachedToken, err := cryptoSvc.Decrypt(tokenEnc)
	if err != nil {
		fmt.Printf("[DIAGNOSTICO] Error al descifrar token de caché: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[DIAGNOSTICO] Token en caché válido (vigente hasta %s). Ejecutando única llamada de diagnóstico...\n", exp.Format("15:04:05"))

	// 4. Ejecutar UNA SOLA llamada exacta (GET periodos/140000)
	// Solo salen los headers originales: Accept, Content-Type, Authorization Bearer
	client := resty.New()
	client.SetTimeout(25 * time.Second)

	url := "https://api-sire.sunat.gob.pe/v1/contribuyente/migeigv/libros/rvierce/padron/web/omisos/140000/periodos"
	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+cachedToken).
		Get(url)

	if err != nil {
		fmt.Printf("[DIAGNOSTICO] Error de conexión hacia SUNAT: %v\n", err)
		os.Exit(1)
	}

	// 5. Salida sanitizada completa para contrastar con Postman
	fmt.Println("==================================================")
	fmt.Println("         RESULTADO DEL DIAGNÓSTICO SUNAT          ")
	fmt.Println("==================================================")
	fmt.Printf("Código HTTP: %d %s\n", resp.StatusCode(), http.StatusText(resp.StatusCode()))

	fmt.Println("\n--- Headers de Respuesta (sin cookies) ---")
	headerKeys := make([]string, 0, len(resp.Header()))
	for k := range resp.Header() {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)

	for _, k := range headerKeys {
		lowerK := strings.ToLower(k)
		// Excluir de forma estricta cualquier header de cookies o sesión sensible
		if strings.Contains(lowerK, "cookie") {
			continue
		}
		for _, val := range resp.Header()[k] {
			fmt.Printf("  %s: %s\n", k, val)
		}
	}

	fmt.Println("\n--- Cuerpo de Respuesta Sanitizado ---")
	bodyStr := resp.String()
	if strings.Contains(bodyStr, "<html") || strings.Contains(bodyStr, "<!DOCTYPE") {
		cleanBody := strings.ReplaceAll(bodyStr, "\n", " ")
		cleanBody = strings.ReplaceAll(cleanBody, "\r", " ")
		for strings.Contains(cleanBody, "  ") {
			cleanBody = strings.ReplaceAll(cleanBody, "  ", " ")
		}
		if len(cleanBody) > 300 {
			cleanBody = cleanBody[:300] + "..."
		}
		fmt.Printf("  [HTML Sanitizado]: %s\n", strings.TrimSpace(cleanBody))
	} else {
		var prettyJSON map[string]interface{}
		if err := json.Unmarshal([]byte(bodyStr), &prettyJSON); err == nil {
			formatted, _ := json.MarshalIndent(prettyJSON, "  ", "  ")
			fmt.Printf("  %s\n", string(formatted))
		} else {
			fmt.Printf("  %s\n", strings.TrimSpace(bodyStr))
		}
	}
	fmt.Println("==================================================")
}
