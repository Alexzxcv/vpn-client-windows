//go:build windows

package device

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// MachineID returns a stable per-machine identifier: the hex-encoded SHA-256
// of the Windows MachineGuid (HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid).
// The raw MachineGuid is never returned or logged.
func MachineID() (string, error) {
	k, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Cryptography`,
		registry.QUERY_VALUE|registry.WOW64_64KEY,
	)
	if err != nil {
		return "", fmt.Errorf("open cryptography registry key: %w", err)
	}
	defer k.Close()

	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return "", fmt.Errorf("read MachineGuid: %w", err)
	}
	if guid == "" {
		return "", fmt.Errorf("MachineGuid is empty")
	}

	sum := sha256.Sum256([]byte(guid))
	return hex.EncodeToString(sum[:]), nil
}
