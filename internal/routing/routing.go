// Package routing holds the engine-agnostic split-tunnel options applied to both
// the xray (proxy) and sing-box (TUN) config generators.
package routing

import "strings"

// Options describes which traffic should bypass the tunnel (go direct).
type Options struct {
	// Domains are domain suffixes/keywords to route directly, e.g. ".ru",
	// "example.com". A leading dot is treated as a suffix match.
	Domains []string
	// IPCIDRs are IP/CIDR ranges to route directly, e.g. "10.0.0.0/8",
	// "1.2.3.4".
	IPCIDRs []string
	// RussiaDirect enables the geosite:ru / geoip:ru direct rule (xray) and the
	// corresponding geoip/geosite direct rule (sing-box).
	RussiaDirect bool
}

// Empty reports whether no direct routing is configured.
func (o Options) Empty() bool {
	return len(o.Domains) == 0 && len(o.IPCIDRs) == 0 && !o.RussiaDirect
}

// SplitList splits a user "direct list" (mixed domains + IP CIDRs) into the two
// buckets. An entry is treated as an IP/CIDR if it parses as one; otherwise it
// is a domain. Entries are trimmed and empties dropped.
func SplitList(entries []string) (domains, ipcidrs []string) {
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if looksLikeIPOrCIDR(e) {
			ipcidrs = append(ipcidrs, e)
		} else {
			domains = append(domains, strings.ToLower(e))
		}
	}
	return domains, ipcidrs
}

// looksLikeIPOrCIDR is a cheap heuristic: contains "/" (CIDR) or only digits,
// dots and colons (IPv4/IPv6 literal). Domains contain letters.
func looksLikeIPOrCIDR(s string) bool {
	if strings.Contains(s, "/") {
		return true
	}
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == ':' {
			continue
		}
		return false
	}
	return true
}
