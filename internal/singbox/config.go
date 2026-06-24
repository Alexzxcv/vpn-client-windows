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
	"os"
	"path/filepath"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/routing"
)

// RuleSetPath resolves a bundled sing-box rule-set (.srs) file by name. It looks
// next to the running executable (dir/, dir/geo/) where the installer places the
// geo databases. The path is returned even if the file is absent so callers get
// a deterministic location; sing-box reports a clear error if it is missing.
func RuleSetPath(name string) string {
	if env := os.Getenv("VPNCLIENT_GEO_DIR"); env != "" {
		return filepath.Join(env, name)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, cand := range []string{
			filepath.Join(dir, "geo", name),
			filepath.Join(dir, name),
		} {
			if st, serr := os.Stat(cand); serr == nil && !st.IsDir() {
				return cand
			}
		}
		// default to dir/geo/<name>
		return filepath.Join(dir, "geo", name)
	}
	return name
}

// TUN interface defaults. The address is a small /30 inside the CGNAT-ish
// 172.18.x range to avoid clashing with common LAN/Docker subnets.
const (
	tunInterfaceName = "sapn-tun"
	tunAddress       = "172.18.0.1/30"
	tunMTU           = 1500
)

// TUNInterfaceName returns the TUN interface name used in the generated config
// (so the kill-switch can allow traffic over it).
func TUNInterfaceName() string { return tunInterfaceName }

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
	return GenerateConfigWith(cfg, routing.Options{})
}

// GenerateConfigWith is GenerateConfig with split-tunnel direct routing and
// DNS/IPv6 leak protection:
//   - DNS is forced through the tunnel (dns-remote via vless-out); direct DNS is
//     only used for the geo/split-tunnel direct path, never as the default.
//   - DNS queries are hijacked off the TUN so the OS resolver cannot leak.
//   - "naked" IPv6 is dropped (block outbound) so traffic cannot escape over an
//     untunnelled v6 path; the strategy prefers IPv4 inside the tunnel.
func GenerateConfigWith(cfg backend.VLESSConfig, opts routing.Options) ([]byte, error) {
	if cfg.Host == "" || cfg.Port == 0 || cfg.UUID == "" {
		return nil, fmt.Errorf("incomplete vless config (host/port/uuid required)")
	}

	fingerprint := cfg.Fingerprint
	if fingerprint == "" {
		fingerprint = "chrome"
	}

	dnsServers := []any{
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
	}
	dnsRules := []any{}
	// Russian / split-tunnel domains resolve via direct DNS so the geo/IP-based
	// direct routing actually lands on a domestic IP. Everything else resolves
	// through the tunnel (no leak).
	if opts.RussiaDirect {
		dnsRules = append(dnsRules, map[string]any{
			"rule_set": []string{"geosite-ru"},
			"server":   "dns-direct",
		})
	}
	if len(opts.Domains) > 0 {
		dnsRules = append(dnsRules, map[string]any{
			"domain_suffix": dnsSuffixes(opts.Domains),
			"server":        "dns-direct",
		})
	}

	dns := map[string]any{
		"servers":  dnsServers,
		"final":    "dns-remote", // default resolver is the tunnelled one (anti-leak)
		"strategy": "ipv4_only",  // do not hand out AAAA: prevents IPv6 DNS leak
	}
	if len(dnsRules) > 0 {
		dns["rules"] = dnsRules
	}

	outbounds := []any{
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
		// block outbound: used to drop naked IPv6 (anti-leak).
		map[string]any{
			"type": "block",
			"tag":  "block",
		},
	}

	routeRules := []any{
		// DNS-запросы — в dns-маршрутизатор (hijack), чтобы ОС не резолвила мимо.
		map[string]any{
			"protocol": "dns",
			"action":   "hijack-dns",
		},
		// Трафик ДО самого VPN-сервера обязан идти напрямую, иначе он попадёт
		// обратно в туннель и образует петлю.
		map[string]any{
			"domain":   []string{cfg.Host},
			"outbound": "direct",
		},
	}

	// Split-tunnel: user domains/IPs go direct.
	if len(opts.Domains) > 0 {
		routeRules = append(routeRules, map[string]any{
			"domain_suffix": dnsSuffixes(opts.Domains),
			"outbound":      "direct",
		})
	}
	if len(opts.IPCIDRs) > 0 {
		routeRules = append(routeRules, map[string]any{
			"ip_cidr":  opts.IPCIDRs,
			"outbound": "direct",
		})
	}
	if opts.RussiaDirect {
		routeRules = append(routeRules,
			map[string]any{
				"rule_set": []string{"geosite-ru", "geoip-ru"},
				"outbound": "direct",
			},
		)
	}

	// Anti-leak: drop any IPv6 destination that would otherwise escape the
	// tunnel. We tunnel IPv4 only; v6 is blocked rather than sent in the clear.
	routeRules = append(routeRules, map[string]any{
		"ip_version": 6,
		"outbound":   "block",
	})

	route := map[string]any{
		"rules":                 routeRules,
		"final":                 "vless-out",
		"auto_detect_interface": true,
	}
	if opts.RussiaDirect {
		route["rule_set"] = ruRuleSets()
	}

	conf := map[string]any{
		"log": map[string]any{
			"level":     "warn",
			"timestamp": true,
		},
		"dns": dns,
		"inbounds": []any{
			map[string]any{
				"type":           "tun",
				"tag":            "tun-in",
				"interface_name": tunInterfaceName,
				// address (merged inet4/inet6) — actual field name in 1.11.x.
				// IPv4-only address: no v6 on the TUN (anti-leak).
				"address":      []string{tunAddress},
				"auto_route":   true,
				"strict_route": true,
				"stack":        "system",
				"mtu":          tunMTU,
				// Перехватываем DNS-запросы и заворачиваем их в dns-маршрутизацию.
				"sniff": true,
			},
		},
		"outbounds": outbounds,
		"route":     route,
	}

	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sing-box config: %w", err)
	}
	return b, nil
}

// dnsSuffixes normalises domain entries to sing-box domain_suffix form: a
// leading "." is stripped (sing-box matches the suffix without it).
func dnsSuffixes(domains []string) []string {
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		if len(d) > 1 && d[0] == '.' {
			d = d[1:]
		}
		out = append(out, d)
	}
	return out
}

// ruRuleSets returns the rule_set definitions for geosite:ru and geoip:ru,
// loaded from local .srs files bundled next to the binary (see RuleSetDir). They
// are referenced by tag from the route/DNS rules.
func ruRuleSets() []any {
	return []any{
		map[string]any{
			"tag":    "geosite-ru",
			"type":   "local",
			"format": "binary",
			"path":   RuleSetPath("geosite-ru.srs"),
		},
		map[string]any{
			"tag":    "geoip-ru",
			"type":   "local",
			"format": "binary",
			"path":   RuleSetPath("geoip-ru.srs"),
		},
	}
}
