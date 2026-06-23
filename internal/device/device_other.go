//go:build !windows

package device

import "fmt"

// MachineID is only implemented on Windows. This stub allows the rest of the
// codebase to compile on non-Windows platforms (e.g. CI linters).
func MachineID() (string, error) {
	return "", fmt.Errorf("device.MachineID: only supported on windows")
}
