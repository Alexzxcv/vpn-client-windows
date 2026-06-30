// Package settings persists user-editable local client settings to
// %APPDATA%/sapn-vpn/settings.json: the local SOCKS/HTTP proxy ports, the
// kill-switch toggle, the manual split-tunnel direct list and the "Russian
// sites direct" geo toggle.
//
// It is safe for concurrent use. Unknown/zero fields fall back to defaults so
// an old or partial file keeps working.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Defaults mirror the historical hard-coded values.
const (
	DefaultSocksPort = 10800
	DefaultHTTPPort  = 10801
)

// Settings is the on-disk, user-editable config.
type Settings struct {
	// SocksPort / HTTPPort are the local proxy listen ports for the proxy mode.
	SocksPort int `json:"socks_port"`
	HTTPPort  int `json:"http_port"`

	// KillSwitch blocks all egress while connected if the tunnel drops. It
	// defaults to on for TUN; see EffectiveKillSwitch.
	KillSwitch *bool `json:"kill_switch,omitempty"`

	// DirectList is the manual split-tunnel list: domains (".ru", "example.com")
	// and/or IP CIDRs ("10.0.0.0/8") that must bypass the tunnel (go direct).
	DirectList []string `json:"direct_list,omitempty"`

	// RussiaDirect routes Russian sites/IPs directly via geosite:ru / geoip:ru.
	RussiaDirect bool `json:"russia_direct,omitempty"`

	// Autostart launches the client at Windows login (registry Run key). The
	// registry is the source of truth; this mirrors it for the UI.
	Autostart bool `json:"autostart,omitempty"`

	// MultiProxyEnabled reveals the multi-proxy feature (several local SOCKS5
	// proxies, each on its own port tunnelling to its own server). Off by default.
	MultiProxyEnabled bool `json:"multi_proxy_enabled,omitempty"`

	// ProxyMappings are the configured multi-proxy entries (persisted config only;
	// running state lives in the app). Empty when the feature is unused.
	ProxyMappings []ProxyMapping `json:"proxy_mappings,omitempty"`
}

// ProxyMapping is one configured multi-proxy entry: a local SOCKS5 port bound to
// a specific server. ServerID is "auto", a backend location id, or "custom:<id>".
// Exactly one mapping may be Main (its port receives the Windows system proxy).
type ProxyMapping struct {
	ID       string `json:"id"`
	Port     int    `json:"port"`
	ServerID string `json:"server_id"`
	Main     bool   `json:"main,omitempty"`
}

// Default returns a Settings populated with defaults.
func Default() Settings {
	on := true
	return Settings{
		SocksPort:  DefaultSocksPort,
		HTTPPort:   DefaultHTTPPort,
		KillSwitch: &on,
	}
}

// normalise fills zero/invalid fields with defaults.
func (s *Settings) normalise() {
	if s.SocksPort <= 0 || s.SocksPort > 65535 {
		s.SocksPort = DefaultSocksPort
	}
	if s.HTTPPort <= 0 || s.HTTPPort > 65535 {
		s.HTTPPort = DefaultHTTPPort
	}
	if s.KillSwitch == nil {
		on := true
		s.KillSwitch = &on
	}
	// Multi-proxy mappings: drop entries with no id or an invalid port; keep at
	// most one Main.
	if len(s.ProxyMappings) > 0 {
		seenMain := false
		out := make([]ProxyMapping, 0, len(s.ProxyMappings))
		for _, m := range s.ProxyMappings {
			if m.ID == "" || m.Port <= 0 || m.Port > 65535 {
				continue
			}
			if m.Main {
				if seenMain {
					m.Main = false
				} else {
					seenMain = true
				}
			}
			out = append(out, m)
		}
		s.ProxyMappings = out
	}
}

func filePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("settings: user config dir: %w", err)
	}
	return filepath.Join(dir, "sapn-vpn", "settings.json"), nil
}

// Store is a thread-safe, file-backed settings holder.
type Store struct {
	mu   sync.RWMutex
	cur  Settings
	path string
}

// Load reads settings.json (creating defaults if absent or unreadable).
func Load() *Store {
	p, _ := filePath()
	st := &Store{path: p, cur: Default()}
	if p == "" {
		return st
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return st // defaults
	}
	var s Settings
	if json.Unmarshal(data, &s) == nil {
		s.normalise()
		st.cur = s
	}
	return st
}

// Get returns a copy of the current settings.
func (st *Store) Get() Settings {
	st.mu.RLock()
	defer st.mu.RUnlock()
	s := st.cur
	// copy slices so callers can't mutate the stored ones
	if s.DirectList != nil {
		cp := make([]string, len(s.DirectList))
		copy(cp, s.DirectList)
		s.DirectList = cp
	}
	if s.ProxyMappings != nil {
		cp := make([]ProxyMapping, len(s.ProxyMappings))
		copy(cp, s.ProxyMappings)
		s.ProxyMappings = cp
	}
	return s
}

// Save validates, persists and replaces the current settings (0600 file).
func (st *Store) Save(s Settings) error {
	s.normalise()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("settings: marshal: %w", err)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.path != "" {
		if err := os.MkdirAll(filepath.Dir(st.path), 0o700); err != nil {
			return fmt.Errorf("settings: mkdir: %w", err)
		}
		if err := os.WriteFile(st.path, data, 0o600); err != nil {
			return fmt.Errorf("settings: write: %w", err)
		}
	}
	st.cur = s
	return nil
}
