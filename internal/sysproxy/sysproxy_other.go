//go:build !windows

// Package sysproxy на не-Windows платформах — заглушка (клиент целевой под Windows).
package sysproxy

// Set ничего не делает вне Windows.
func Set(httpAddr, socksAddr string) error { _, _ = httpAddr, socksAddr; return nil }

// Clear ничего не делает вне Windows.
func Clear() error { return nil }
