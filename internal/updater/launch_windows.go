//go:build windows

package updater

import (
	"fmt"
	"os/exec"
	"strings"
)

// LaunchInstaller starts the downloaded installer/zip and returns immediately so
// the caller can quit the running client (an .exe installer typically needs the
// old binary closed to replace it). For a .exe we run it directly; for anything
// else we ask the shell to open it (Explorer), letting the user finish manually.
//
// The process is detached: we do not wait for it. Any error starting it is
// returned so the UI can fall back to opening the release page.
func LaunchInstaller(path string) error {
	if path == "" {
		return fmt.Errorf("updater: empty installer path")
	}
	if strings.HasSuffix(strings.ToLower(path), ".exe") {
		cmd := exec.Command(path)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("updater: launch installer: %w", err)
		}
		_ = cmd.Process.Release()
		return nil
	}
	// Non-exe (e.g. a zip): open with the default shell handler.
	cmd := exec.Command("cmd", "/C", "start", "", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("updater: open download: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}
