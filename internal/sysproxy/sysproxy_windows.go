//go:build windows

// Package sysproxy управляет системным прокси Windows (WinINET) — чтобы трафик
// приложений (браузеры и пр.) шёл через локальный xray.
package sysproxy

import (
	"fmt"
	"strings"

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

// ClearIfOurs снимает системный прокси ТОЛЬКО если он сейчас включён и указывает
// на один из наших локальных портов (ports, напр. 10800/10801). Это нужно при
// СТАРТЕ ядра: если прошлый запуск упал в connected-состоянии и не снял прокси,
// мы чистим его — но не трогаем прокси, который пользователь выставил сам.
// Возвращает true, если прокси был снят.
func ClearIfOurs(ports ...int) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, inetSettings, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, fmt.Errorf("sysproxy: open key: %w", err)
	}
	defer k.Close()

	enabled, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil {
		// Значения нет — прокси не настроен, чистить нечего.
		return false, nil
	}
	if enabled == 0 {
		return false, nil
	}

	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil || server == "" {
		return false, nil
	}
	if !mentionsAnyPort(server, ports) {
		// Прокси чужой (не наш) — не трогаем.
		return false, nil
	}

	if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
		return false, fmt.Errorf("sysproxy: clear ProxyEnable: %w", err)
	}
	refresh()
	return true, nil
}

// mentionsAnyPort сообщает, упоминает ли строка ProxyServer любой из портов в
// виде ":<port>" (как `http=127.0.0.1:10801;...`).
func mentionsAnyPort(proxyServer string, ports []int) bool {
	for _, p := range ports {
		if p == 0 {
			continue
		}
		if strings.Contains(proxyServer, fmt.Sprintf(":%d", p)) {
			return true
		}
	}
	return false
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
