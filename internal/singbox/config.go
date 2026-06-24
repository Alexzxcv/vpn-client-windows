// Package singbox manages sing-box as a subprocess for the full-tunnel (TUN)
// mode: it generates a sing-box config (TUN inbound + VLESS Reality outbound),
// discovers the binary, starts/stops the process and probes readiness.
//
// The TUN mode captures all device traffic via a virtual interface (wintun on
// Windows), unlike the proxy mode which only sets the system proxy. sing-box is
// the engine for TUN; xray-core remains the engine for proxy.
//
// Never log VLESS UUIDs, reality keys or generated configs.
package singbox

import (
	"encoding/json"
	"fmt"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
)

// TUN interface defaults. The address is a small /30 inside the CGNAT-ish
// 172.18.x range to avoid clashing with common LAN/Docker subnets.
const (
	tunInterfaceName = "sapn-tun"
	tunAddress       = "172.18.0.1/30"
	tunMTU           = 1500
)

// GenerateConfig builds a sing-box v1.11.x config JSON with:
//   - a tun inbound (auto_route + strict_route, system stack),
//   - a vless + reality outbound (tag "vless-out") plus a direct outbound,
//   - DNS resolving through the tunnel,
//   - a route rule that sends traffic to the VPN node's IP directly (avoids a
//     routing loop) and otherwise defaults to the vless outbound.
//
// Targets sing-box 1.11.x. The TUN address uses the merged `address` field
// (the legacy `inet4_address` is rejected by 1.11 unless an opt-in env var is
// set, and is removed entirely in 1.12).
func GenerateConfig(cfg backend.VLESSConfig) ([]byte, error) {
	if cfg.Host == "" || cfg.Port == 0 || cfg.UUID == "" {
		return nil, fmt.Errorf("incomplete vless config (host/port/uuid required)")
	}

	fingerprint := cfg.Fingerprint
	if fingerprint == "" {
		fingerprint = "chrome"
	}

	conf := map[string]any{
		"log": map[string]any{
			"level":     "warn",
			"timestamp": true,
		},
		"dns": map[string]any{
			"servers": []any{
				map[string]any{
					"tag":     "dns-remote",
					"address": "1.1.1.1",
					"detour":  "vless-out",
				},
				map[string]any{
					"tag":     "dns-direct",
					"address": "1.1.1.1",
					"detour":  "direct",
				},
			},
			// Резолв доменов идёт в туннель; для конфигурационных IP — напрямую.
			"final":    "dns-remote",
			"strategy": "prefer_ipv4",
		},
		"inbounds": []any{
			map[string]any{
				"type":           "tun",
				"tag":            "tun-in",
				"interface_name": tunInterfaceName,
				// address (merged inet4/inet6) — actual field name in 1.11.x.
				"address":      []string{tunAddress},
				"auto_route":   true,
				"strict_route": true,
				"stack":        "system",
				"mtu":          tunMTU,
				// Перехватываем DNS-запросы и заворачиваем их в dns-маршрутизацию.
				"sniff": true,
			},
		},
		"outbounds": []any{
			map[string]any{
				"type":        "vless",
				"tag":         "vless-out",
				"server":      cfg.Host,
				"server_port": cfg.Port,
				"uuid":        cfg.UUID,
				"flow":        cfg.Flow,
				"tls": map[string]any{
					"enabled":     true,
					"server_name": cfg.SNI,
					"utls": map[string]any{
						"enabled":     true,
						"fingerprint": fingerprint,
					},
					"reality": map[string]any{
						"enabled":    true,
						"public_key": cfg.PublicKey,
						"short_id":   cfg.ShortID,
					},
				},
			},
			map[string]any{
				"type": "direct",
				"tag":  "direct",
			},
		},
		"route": map[string]any{
			"rules": []any{
				// DNS-запросы — в dns-маршрутизатор (hijack).
				map[string]any{
					"protocol": "dns",
					"action":   "hijack-dns",
				},
				// Трафик ДО самого VPN-сервера обязан идти напрямую, иначе он
				// попадёт обратно в туннель и образует петлю. auto_route обычно
				// исключает соединение управляющего процесса, но правило надёжнее.
				map[string]any{
					"domain":   []string{cfg.Host},
					"outbound": "direct",
				},
			},
			"final":                 "vless-out",
			"auto_detect_interface": true,
		},
	}

	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sing-box config: %w", err)
	}
	return b, nil
}
