//go:build !windows

package autostart

// Set is a no-op on non-Windows platforms.
func Set(enabled bool, exePath string) error { return nil }

// Enabled always reports false on non-Windows platforms.
func Enabled() bool { return false }
