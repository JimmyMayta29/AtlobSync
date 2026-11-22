//go:build !windows

package license

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

// GetMachineIdentity obtiene una huella determinista del equipo y su hostname.
func GetMachineIdentity() (hash string, displayName string) {
	host, _ := os.Hostname()
	host = strings.TrimSpace(host)
	if len(host) > 64 {
		host = host[:64]
	}

	rawID := ""
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if b, err := os.ReadFile(p); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				rawID = s
				break
			}
		}
	}
	if rawID == "" {
		rawID = host + "|" + strings.TrimSpace(os.Getenv("USER"))
	}

	sum := sha256.Sum256([]byte("atlob-sync-v2-machine|" + strings.ToLower(strings.TrimSpace(rawID))))
	return hex.EncodeToString(sum[:]), host
}
