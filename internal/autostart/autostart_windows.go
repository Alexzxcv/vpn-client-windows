//go:build windows

// Package autostart manages "start with Windows" via the per-user Run registry
// key (HKCU\...\Run). Enabling adds the client (with --minimized so it launches
// straight to the tray); disabling removes the value.
package autostart

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	appName = "SAPN VPN"
)

// Set enables or disables launch-at-login for the given executable path.
func Set(enabled bool, exePath string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("autostart: open run key: %w", err)
	}
	defer k.Close()

	if enabled {
		val := fmt.Sprintf("\"%s\" --minimized", exePath)
		if err := k.SetStringValue(appName, val); err != nil {
			return fmt.Errorf("autostart: set value: %w", err)
		}
		return nil
	}
	if err := k.DeleteValue(appName); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("autostart: delete value: %w", err)
	}
	return nil
}

// Enabled reports whether launch-at-login is currently configured.
func Enabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(appName)
	return err == nil
}
