//go:build windows

package license

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// GetMachineIdentity obtiene una huella determinista del equipo Windows (SHA-256 del MachineGuid)
// y el nombre legible del equipo para auditoría comercial.
func GetMachineIdentity() (hash string, displayName string) {
	host, _ := os.Hostname()
	host = strings.TrimSpace(host)
	if len(host) > 64 {
		host = host[:64]
	}

	rawID := readWindowsMachineGUID()
	if rawID == "" {
		domain := strings.TrimSpace(os.Getenv("USERDOMAIN"))
		user := strings.TrimSpace(os.Getenv("USERNAME"))
		rawID = strings.ToLower(strings.Join([]string{host, domain, user}, "|"))
	}

	sum := sha256.Sum256([]byte("atlob-sync-v2-machine|" + strings.ToLower(strings.TrimSpace(rawID))))
	return hex.EncodeToString(sum[:]), host
}

func readWindowsMachineGUID() string {
	const regPath = `SOFTWARE\Microsoft\Cryptography`
	flags := []uint32{
		registry.QUERY_VALUE | registry.WOW64_64KEY,
		registry.QUERY_VALUE,
	}
	for _, access := range flags {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, regPath, access)
		if err != nil {
			continue
		}
		val, _, err := k.GetStringValue("MachineGuid")
		_ = k.Close()
		val = strings.TrimSpace(val)
		if err == nil && val != "" {
			return val
		}
	}
	return ""
}
