// Package xray manages xray-core as a subprocess: config generation, binary
// discovery, process start/stop and a SOCKS readiness probe.
//
// Never log VLESS UUIDs, reality keys or generated configs.
package xray

import (
	"encoding/json"
	"fmt"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/routing"
)

// GenerateConfig builds an xray config with no split-tunnel rules.
func GenerateConfig(cfg backend.VLESSConfig, socksPort, httpPort int) ([]byte, error) {
	return GenerateConfigWith(cfg, socksPort, httpPort, routing.Options{})
}

// GenerateConfigWith builds an xray config JSON with:
//   - a SOCKS inbound on 127.0.0.1:<socksPort> (UDP enabled),
//   - an HTTP inbound on 127.0.0.1:<httpPort>,
//   - a VLESS + Reality outbound to the given server,
//   - optional split-tunnel "direct" routing rules from opts.
func GenerateConfigWith(cfg backend.VLESSConfig, socksPort, httpPort int, opts routing.Options) ([]byte, error) {
	if cfg.Host == "" || cfg.Port == 0 || cfg.UUID == "" {
		return nil, fmt.Errorf("incomplete vless config (host/port/uuid required)")
	}

	// Reality требует валидный uTLS-fingerprint. Если backend/кастомный конфиг не
	// задал его, подставляем "chrome" (как делает sing-box). Без этого xray НЕ
	// стартует с пустым fingerprint — SOCKS-порт не открывается и connect падает
	// с "context deadline exceeded", хотя TUN (sing-box) на том же узле работает.
	fingerprint := cfg.Fingerprint
	if fingerprint == "" {
		fingerprint = "chrome"
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
						"fingerprint": fingerprint,
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

	if rules := xrayRoutingRules(opts); len(rules) > 0 {
		conf["routing"] = map[string]any{
			"domainStrategy": "IPIfNonMatch",
			"rules":          rules,
		}
	}

	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal xray config: %w", err)
	}
	return b, nil
}

// xrayRoutingRules builds xray routing rules that send split-tunnel traffic to
// the "direct" outbound. Order: explicit user domains/IPs first, then the
// optional geo (geosite:ru / geoip:ru) rule.
func xrayRoutingRules(opts routing.Options) []any {
	var rules []any

	if len(opts.Domains) > 0 {
		// xray domain matching: a leading "." in our list means "suffix", which
		// xray expresses as "domain:<suffix>" (without the dot).
		domains := make([]string, 0, len(opts.Domains))
		for _, d := range opts.Domains {
			if len(d) > 1 && d[0] == '.' {
				domains = append(domains, "domain:"+d[1:])
			} else {
				domains = append(domains, d)
			}
		}
		rules = append(rules, map[string]any{
			"type":        "field",
			"domain":      domains,
			"outboundTag": "direct",
		})
	}

	if len(opts.IPCIDRs) > 0 {
		rules = append(rules, map[string]any{
			"type":        "field",
			"ip":          opts.IPCIDRs,
			"outboundTag": "direct",
		})
	}

	if opts.RussiaDirect {
		rules = append(rules,
			map[string]any{
				"type":        "field",
				"domain":      []string{"geosite:category-ru", "geosite:ru"},
				"outboundTag": "direct",
			},
			map[string]any{
				"type":        "field",
				"ip":          []string{"geoip:ru"},
				"outboundTag": "direct",
			},
		)
	}

	return rules
}
