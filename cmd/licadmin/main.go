package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type licenseRow struct {
	ID              string     `json:"id"`
	LicenseKey      string     `json:"license_key"`
	PlanType        string     `json:"plan_type"`
	PlanLabel       string     `json:"plan_label"`
	ClientName      string     `json:"client_name"`
	ClientContact   string     `json:"client_contact"`
	ClientEmail     string     `json:"client_email"`
	ClientPhone     string     `json:"client_phone"`
	Status          string     `json:"status"`
	DurationDays    *int       `json:"duration_days"`
	MaxCompanies    int        `json:"max_companies"`
	MachineHash     *string    `json:"machine_hash"`
	MachineName     string     `json:"machine_name"`
	ActivatedAt     *time.Time `json:"activated_at"`
	ExpiresAt       *time.Time `json:"expires_at"`
	LastSeenAt      *time.Time `json:"last_seen_at"`
	CompaniesCount  int        `json:"companies_count"`
	ActivationCount int        `json:"activation_count"`
	HeartbeatCount  int        `json:"heartbeat_count"`
	Notes           string     `json:"notes"`
	CreatedAt       time.Time  `json:"created_at"`
}

type adminClient struct {
	baseURL    string
	serviceKey string
	httpClient *http.Client
}

func newAdminClient(baseURL, serviceKey string) *adminClient {
	return &adminClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		serviceKey: strings.TrimSpace(serviceKey),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *adminClient) doRequest(method, path string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", c.serviceKey)
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error de conexión con Supabase: %w", err)
	}
	defer resp.Body.Close()

	rawResp, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		var errBody struct {
			Message string `json:"message"`
			Details string `json:"details"`
			Hint    string `json:"hint"`
		}
		_ = json.Unmarshal(rawResp, &errBody)
		if errBody.Message != "" {
			return fmt.Errorf("Supabase (%d): %s %s", resp.StatusCode, errBody.Message, errBody.Hint)
		}
		return fmt.Errorf("Supabase respondió HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(rawResp)))
	}

	if out != nil && len(rawResp) > 0 {
		if err := json.Unmarshal(rawResp, out); err != nil {
			return fmt.Errorf("error decodificando respuesta: %w (%s)", err, string(rawResp))
		}
	}
	return nil
}

func (c *adminClient) callRPC(fn string, params map[string]interface{}, out interface{}) error {
	return c.doRequest(http.MethodPost, "/rest/v1/rpc/"+fn, params, out)
}

func (c *adminClient) listLicenses() ([]licenseRow, error) {
	var rows []licenseRow
	err := c.doRequest(http.MethodGet, "/rest/v1/licenses?select=*&order=created_at.desc&limit=100", nil, &rows)
	return rows, err
}

func loadAdminEnv() (string, string) {
	candidates := []string{".env.admin"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env.admin"))
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}
	url := strings.TrimSpace(os.Getenv("LICENSE_API_URL"))
	if url == "" {
		url = strings.TrimSpace(os.Getenv("SUPABASE_URL"))
	}
	key := strings.TrimSpace(os.Getenv("LICENSE_ADMIN_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY"))
	}
	return url, key
}

func main() {
	url, serviceKey := loadAdminEnv()
	if url == "" || serviceKey == "" {
		fmt.Println("================================================================")
		fmt.Println("  ATLOB Sync · Administrador de Licencias (Cloudflare / Supabase)")
		fmt.Println("================================================================")
		fmt.Println()
		fmt.Println("[Falta configurar .env.admin]")
		fmt.Println("1. Copia '.env.admin.example' como '.env.admin' en la carpeta API-SIRE.")
		fmt.Println("2. Pega tu LICENSE_API_URL y LICENSE_ADMIN_KEY (o variables Supabase).")
		fmt.Println("3. Consulta 'cloudflare\\GUIA_CLOUDFLARE.md' para mas detalles.")
		fmt.Println()
		waitEnter()
		os.Exit(1)
	}

	client := newAdminClient(url, serviceKey)
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println()
		fmt.Println("================================================================")
		fmt.Println("  ATLOB Sync · Gestor Comercial de Licencias (Cloud)")
		fmt.Println("================================================================")
		fmt.Println("  1) Crear DEMO de 7 días (para prospecto)")
		fmt.Println("  2) Crear Licencia COMERCIAL (Mensual / Anual / Permanente)")
		fmt.Println("  3) Ver licencias activas y actividad de clientes")
		fmt.Println("  4) Extender días a una licencia")
		fmt.Println("  5) Revocar licencia (bloquear acceso)")
		fmt.Println("  6) Liberar equipo (permitir activar en otra PC)")
		fmt.Println("  0) Salir")
		fmt.Println("----------------------------------------------------------------")

		choice := prompt(reader, "Elige una opción", "1")
		switch choice {
		case "1":
			handleCreateDemo(reader, client)
		case "2":
			handleCreateCommercial(reader, client)
		case "3":
			handleListLicenses(client)
		case "4":
			handleExtendLicense(reader, client)
		case "5":
			handleRevokeLicense(reader, client)
		case "6":
			handleResetMachine(reader, client)
		case "0", "q", "exit":
			fmt.Println("Hasta luego.")
			return
		default:
			fmt.Println("Opción no reconocida.")
		}
	}
}

func handleCreateDemo(reader *bufio.Reader, client *adminClient) {
	fmt.Println("\n--- NUEVO DEMO DE PRUEBA ---")
	cliente := prompt(reader, "Nombre del cliente / estudio", "")
	if cliente == "" {
		fmt.Println("Cancelado: el nombre del cliente es obligatorio.")
		return
	}
	contacto := prompt(reader, "Teléfono o correo del prospecto (opcional)", "")
	days := promptInt(reader, "Días de prueba", 7)
	maxEmp := promptInt(reader, "Máximo de empresas/RUCs (0 = ilimitado)", 3)
	notas := prompt(reader, "Nota interna (ej. Video promo)", "")

	var row licenseRow
	err := client.callRPC("create_demo_license", map[string]interface{}{
		"p_client_name":    cliente,
		"p_client_contact": contacto,
		"p_days":           days,
		"p_max_companies":  maxEmp,
		"p_notes":          notas,
	}, &row)
	if err != nil {
		fmt.Printf("\n[ERROR] %v\n", err)
		return
	}

	limitTxt := fmt.Sprintf("hasta %d empresas (RUCs)", row.MaxCompanies)
	if row.MaxCompanies == 0 {
		limitTxt = "empresas ilimitadas"
	}

	fmt.Println("\n[EXITO] Licencia DEMO generada:")
	fmt.Printf("  Código:   %s\n", row.LicenseKey)
	fmt.Printf("  Cliente:  %s\n", row.ClientName)
	fmt.Printf("  Duración: %d días (inician recién cuando el cliente la active)\n", days)
	fmt.Printf("  Límite:   %s\n", limitTxt)
	fmt.Println("\n--- MENSAJE LISTO PARA ENVIAR POR WHATSAPP ---")
	fmt.Printf("Hola %s, aquí tienes tu acceso de prueba funcional de ATLOB Sync · SUNAT por %d días (%s).\n\n", row.ClientName, days, limitTxt)
	fmt.Printf("1. Descarga e instala ATLOB_Sync_Instalador.exe\n")
	fmt.Printf("   (Si Windows muestra 'Windows protegió su PC', clic en 'Más información' > 'Ejecutar de todas formas').\n")
	fmt.Printf("2. Al abrir la aplicación, ingresa este código de activación:\n")
	fmt.Printf("   %s\n\n", row.LicenseKey)
	fmt.Printf("Tus %d días empiezan a correr recién cuando lo actives. Si necesitas ayuda escríbeme por aquí (940 781 298).\n", days)
	fmt.Println("----------------------------------------------")
}

func handleCreateCommercial(reader *bufio.Reader, client *adminClient) {
	fmt.Println("\n--- NUEVA LICENCIA COMERCIAL ---")
	fmt.Println("  1) Plan Mensual (30 días)")
	fmt.Println("  2) Plan Anual (365 días)")
	fmt.Println("  3) Licencia Permanente")
	tipoOpt := prompt(reader, "Tipo de plan", "2")

	plan := "anual"
	defaultDays := 365
	switch tipoOpt {
	case "1":
		plan = "mensual"
		defaultDays = 30
	case "3":
		plan = "permanente"
		defaultDays = 0
	}

	cliente := prompt(reader, "Nombre del cliente / empresa", "")
	if cliente == "" {
		fmt.Println("Cancelado: el nombre del cliente es obligatorio.")
		return
	}
	contacto := prompt(reader, "Teléfono o correo del cliente (opcional)", "")

	var daysArg interface{} = nil
	if plan != "permanente" {
		daysArg = promptInt(reader, "Días de vigencia", defaultDays)
	}
	maxEmp := promptInt(reader, "Máximo de empresas/RUCs (0 = ilimitado)", 0)
	notas := prompt(reader, "Nota interna (ej. Comprobante pago)", "")

	var row licenseRow
	err := client.callRPC("create_commercial_license", map[string]interface{}{
		"p_client_name":    cliente,
		"p_plan_type":      plan,
		"p_days":           daysArg,
		"p_max_companies":  maxEmp,
		"p_client_contact": contacto,
		"p_notes":          notas,
	}, &row)
	if err != nil {
		fmt.Printf("\n[ERROR] %v\n", err)
		return
	}

	fmt.Println("\n[EXITO] Licencia COMERCIAL generada:")
	fmt.Printf("  Código:  %s\n", row.LicenseKey)
	fmt.Printf("  Plan:    %s\n", row.PlanLabel)
	fmt.Printf("  Cliente: %s\n", row.ClientName)
	fmt.Println("\n--- MENSAJE LISTO PARA ENVIAR AL CLIENTE ---")
	fmt.Printf("Hola %s, ¡gracias por confiar en ATLOB Sync · SUNAT!\n", row.ClientName)
	fmt.Printf("Tu código de licencia (%s) es:\n%s\n\n", row.PlanLabel, row.LicenseKey)
	fmt.Printf("Puedes ingresarlo directamente en la app sin perder tus empresas ni descargas previas.\n")
	fmt.Println("---------------------------------------------")
}

func handleListLicenses(client *adminClient) {
	rows, err := client.listLicenses()
	if err != nil {
		fmt.Printf("\n[ERROR] %v\n", err)
		return
	}
	if len(rows) == 0 {
		fmt.Println("\nNo hay licencias registradas todavía.")
		return
	}

	fmt.Printf("\n%-20s | %-20s | %-28s | %-10s | %-9s | %-10s | %-7s | %-14s | %-14s\n",
		"CLAVE", "CLIENTE", "CONTACTO", "PLAN", "ESTADO", "VIGENCIA", "EMPR.", "EQUIPO", "ULTIMA CONEXION")
	fmt.Println(strings.Repeat("-", 150))

	now := time.Now()
	for _, r := range rows {
		vig := "Pend. act."
		if r.PlanType == "permanente" && r.ActivatedAt != nil {
			vig = "Permanente"
		} else if r.ExpiresAt != nil {
			days := int(r.ExpiresAt.Sub(now).Hours()/24.0 + 0.999)
			if days < 0 {
				days = 0
			}
			vig = fmt.Sprintf("%dd (%s)", days, r.ExpiresAt.Local().Format("02/01"))
		}

		maxStr := "∞"
		if r.MaxCompanies > 0 {
			maxStr = strconv.Itoa(r.MaxCompanies)
		}
		empStr := fmt.Sprintf("%d/%s", r.CompaniesCount, maxStr)

		contacto := strings.TrimSpace(r.ClientContact)
		if contacto == "" {
			contacto = strings.TrimSpace(strings.Trim(r.ClientEmail+" | "+r.ClientPhone, " |"))
		}
		if contacto == "" {
			contacto = "-"
		}

		equipo := "-"
		if r.MachineName != "" {
			equipo = truncate(r.MachineName, 14)
		} else if r.MachineHash != nil && *r.MachineHash != "" {
			equipo = "Vinculado"
		}

		lastSeen := "Sin uso aún"
		if r.LastSeenAt != nil {
			lastSeen = r.LastSeenAt.Local().Format("02/01/06 15:04")
		}

		fmt.Printf("%-20s | %-20s | %-28s | %-10s | %-9s | %-10s | %-7s | %-14s | %-14s\n",
			truncate(r.LicenseKey, 20),
			truncate(r.ClientName, 20),
			truncate(contacto, 28),
			truncate(r.PlanType, 10),
			truncate(r.Status, 9),
			truncate(vig, 10),
			empStr,
			equipo,
			lastSeen,
		)
	}
}

func handleExtendLicense(reader *bufio.Reader, client *adminClient) {
	key := strings.ToUpper(prompt(reader, "Código de licencia a extender", ""))
	if key == "" {
		return
	}
	days := promptInt(reader, "Días adicionales a sumar", 7)
	var row licenseRow
	err := client.callRPC("extend_license", map[string]interface{}{
		"p_license_key": key,
		"p_extra_days":  days,
	}, &row)
	if err != nil {
		fmt.Printf("\n[ERROR] %v\n", err)
		return
	}
	expStr := "-"
	if row.ExpiresAt != nil {
		expStr = row.ExpiresAt.Local().Format("02/01/2006 15:04")
	}
	fmt.Printf("\n[EXITO] Licencia %s extendida +%d días. Nuevo vencimiento: %s\n", row.LicenseKey, days, expStr)
}

func handleRevokeLicense(reader *bufio.Reader, client *adminClient) {
	key := strings.ToUpper(prompt(reader, "Código de licencia a revocar", ""))
	if key == "" {
		return
	}
	reason := prompt(reader, "Motivo (opcional)", "Revocada por administrador")
	var row licenseRow
	err := client.callRPC("revoke_license", map[string]interface{}{
		"p_license_key": key,
		"p_reason":      reason,
	}, &row)
	if err != nil {
		fmt.Printf("\n[ERROR] %v\n", err)
		return
	}
	fmt.Printf("\n[EXITO] Licencia %s revocada. En la próxima verificación la app bloqueará el acceso.\n", row.LicenseKey)
}

func handleResetMachine(reader *bufio.Reader, client *adminClient) {
	key := strings.ToUpper(prompt(reader, "Código de licencia a desvincular de su PC", ""))
	if key == "" {
		return
	}
	var row licenseRow
	err := client.callRPC("reset_license_machine", map[string]interface{}{
		"p_license_key": key,
	}, &row)
	if err != nil {
		fmt.Printf("\n[ERROR] %v\n", err)
		return
	}
	fmt.Printf("\n[EXITO] Equipo desvinculado de %s. El cliente ya puede activarla en su nueva PC.\n", row.LicenseKey)
}

func prompt(reader *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptInt(reader *bufio.Reader, label string, def int) int {
	for {
		raw := prompt(reader, label, strconv.Itoa(def))
		n, err := strconv.Atoi(raw)
		if err == nil && n >= 0 {
			return n
		}
		fmt.Println("Ingresa un número entero válido (0 o mayor).")
	}
}

func truncate(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

func waitEnter() {
	fmt.Print("Presiona Enter para continuar...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
