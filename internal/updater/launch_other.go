//go:build !windows

package updater

import "fmt"

// LaunchInstaller is unsupported off Windows (the client ships for Windows). The
// stub keeps the package buildable for vet/tests on dev boxes.
func LaunchInstaller(string) error {
	return fmt.Errorf("updater: installer launch is only supported on Windows")
}
