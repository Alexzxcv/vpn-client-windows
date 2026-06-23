// Package tokenstore persists the backend access/refresh tokens to disk so the
// user stays logged in across restarts (tray "Connect" works immediately).
//
// TODO(security): на Windows зашифровать через DPAPI (CryptProtectData);
// сейчас файл хранится открыто в каталоге пользователя (0600).
package tokenstore

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type tokens struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
}

func filePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sapn-vpn", "tokens.json"), nil
}

// Load возвращает сохранённые access/refresh (пустые, если нет/ошибка).
func Load() (access, refresh string) {
	p, err := filePath()
	if err != nil {
		return "", ""
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", ""
	}
	var t tokens
	if json.Unmarshal(data, &t) != nil {
		return "", ""
	}
	return t.Access, t.Refresh
}

// Save сохраняет токены (0600). Пустой refresh — удаляет файл (logout).
func Save(access, refresh string) {
	p, err := filePath()
	if err != nil {
		return
	}
	if refresh == "" {
		_ = os.Remove(p)
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	data, err := json.Marshal(tokens{Access: access, Refresh: refresh})
	if err != nil {
		return
	}
	_ = os.WriteFile(p, data, 0o600)
}
