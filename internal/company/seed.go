package company

import (
	"log"
	"os"
	"strings"
)

// SeedInitialCompanyFromEnv migra automáticamente las credenciales existentes en .env si la base de datos está vacía
func SeedInitialCompanyFromEnv(service *Service) {
	empresas, err := service.ListEmpresas()
	if err != nil {
		log.Printf("[SEED] Error verificando empresas existentes: %v", err)
		return
	}
	if len(empresas) > 0 {
		return // Ya existen empresas registradas
	}

	ruc := strings.TrimSpace(os.Getenv("RUC"))
	usuario := strings.TrimSpace(os.Getenv("USUARIO"))
	clave := strings.TrimSpace(os.Getenv("CLAVE_SOL"))
	clientID := strings.TrimSpace(os.Getenv("CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("CLIENT_SECRET"))

	if ruc != "" && usuario != "" && clave != "" && clientID != "" && clientSecret != "" {
		log.Printf("[SEED] Detectadas credenciales en .env para RUC %s. Migrando automáticamente a SQLite cifrado...", ruc)
		req := &CreateEmpresaRequest{
			RUC:          ruc,
			RazonSocial:  "EMPRESA RUC " + ruc,
			UsuarioSOL:   usuario,
			ClaveSOL:     clave,
			ClientID:     clientID,
			ClientSecret: clientSecret,
		}
		_, err := service.CreateEmpresa(req)
		if err != nil {
			log.Printf("[SEED] Error al migrar empresa inicial: %v", err)
		} else {
			log.Printf("[SEED] Empresa con RUC %s registrada y cifrada exitosamente en SQLite.", ruc)
		}
	}
}
