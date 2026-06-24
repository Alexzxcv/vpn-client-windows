//go:build !windows

package app

import (
	"os/exec"
	"runtime"
)

// openURL opens raw in the default browser on non-Windows platforms (used by
// tests / dev builds on macOS / Linux).
func openURL(raw string) error {
	cmd := "xdg-open"
	if runtime.GOOS == "darwin" {
		cmd = "open"
	}
	return exec.Command(cmd, raw).Start()
}
