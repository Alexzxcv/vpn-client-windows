//go:build windows

// Package sysproxy управляет системным прокси Windows (WinINET) — чтобы трафик
// приложений (браузеры и пр.) шёл через локальный xray.
package sysproxy

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const inetSettings = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// Set включает системный прокси: http/https → httpAddr, socks → socksAddr.
// Локальные адреса (<local>) идут напрямую.
func Set(httpAddr, socksAddr string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, inetSettings, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("sysproxy: open key: %w", err)
	}
	defer k.Close()

	proxy := fmt.Sprintf("http=%s;https=%s;socks=%s", httpAddr, httpAddr, socksAddr)
	if err := k.SetStringValue("ProxyServer", proxy); err != nil {
		return fmt.Errorf("sysproxy: set ProxyServer: %w", err)
	}
	if err := k.SetStringValue("ProxyOverride", "<local>"); err != nil {
		return fmt.Errorf("sysproxy: set ProxyOverride: %w", err)
	}
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("sysproxy: set ProxyEnable: %w", err)
	}
	refresh()
	return nil
}

// Clear выключает системный прокси.
func Clear() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, inetSettings, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("sysproxy: open key: %w", err)
	}
	defer k.Close()

	if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
		return fmt.Errorf("sysproxy: clear ProxyEnable: %w", err)
	}
	refresh()
	return nil
}

// refresh уведомляет WinINET о смене настроек (применяется без перезапуска).
func refresh() {
	const (
		internetOptionSettingsChanged = 39
		internetOptionRefresh         = 37
	)
	proc := windows.NewLazySystemDLL("wininet.dll").NewProc("InternetSetOptionW")
	// InternetSetOptionW(NULL, option, NULL, 0). Возвращаемую ошибку игнорируем:
	// syscall всегда отдаёт last-error даже при успехе.
	_, _, _ = proc.Call(0, internetOptionSettingsChanged, 0, 0)
	_, _, _ = proc.Call(0, internetOptionRefresh, 0, 0)
}
