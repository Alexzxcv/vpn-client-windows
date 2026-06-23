// Package xray manages xray-core as a subprocess: config generation, binary
// discovery, process start/stop and a SOCKS readiness probe.
//
// Never log VLESS UUIDs, reality keys or generated configs.
package xray

import (
	"encoding/json"
	"fmt"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
)

// GenerateConfig builds an xray config JSON with:
//   - a SOCKS inbound on 127.0.0.1:<socksPort> (UDP enabled),
//   - an HTTP inbound on 127.0.0.1:<httpPort>,
//   - a VLESS + Reality outbound to the given server.
func GenerateConfig(cfg backend.VLESSConfig, socksPort, httpPort int) ([]byte, error) {
	if cfg.Host == "" || cfg.Port == 0 || cfg.UUID == "" {
		return nil, fmt.Errorf("incomplete vless config (host/port/uuid required)")
	}

	conf := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": []any{
			map[string]any{
				"tag":      "socks-in",
				"listen":   "127.0.0.1",
				"port":     socksPort,
				"protocol": "socks",
				"settings": map[string]any{
					"udp":  true,
					"auth": "noauth",
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []string{"http", "tls"},
				},
			},
			map[string]any{
				"tag":      "http-in",
				"listen":   "127.0.0.1",
				"port":     httpPort,
				"protocol": "http",
				"settings": map[string]any{},
			},
		},
		"outbounds": []any{
			map[string]any{
				"tag":      "proxy",
				"protocol": "vless",
				"settings": map[string]any{
					"vnext": []any{
						map[string]any{
							"address": cfg.Host,
							"port":    cfg.Port,
							"users": []any{
								map[string]any{
									"id":         cfg.UUID,
									"encryption": "none",
									"flow":       cfg.Flow,
								},
							},
						},
					},
				},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "reality",
					"realitySettings": map[string]any{
						"serverName":  cfg.SNI,
						"fingerprint": cfg.Fingerprint,
						"publicKey":   cfg.PublicKey,
						"shortId":     cfg.ShortID,
					},
				},
			},
			map[string]any{
				"tag":      "direct",
				"protocol": "freedom",
				"settings": map[string]any{},
			},
		},
	}

	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal xray config: %w", err)
	}
	return b, nil
}
