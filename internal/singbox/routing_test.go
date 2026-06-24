package singbox

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Alexzxcv/vpn-client-windows/internal/routing"
)

func TestGenerateConfigLeakProtection(t *testing.T) {
	raw, err := GenerateConfig(sampleConfig())
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// DNS default must resolve through the tunnel (anti-leak) and be IPv4-only.
	dns := parsed["dns"].(map[string]any)
	if dns["final"] != "dns-remote" {
		t.Fatalf("dns final = %v, want dns-remote", dns["final"])
	}
	if dns["strategy"] != "ipv4_only" {
		t.Fatalf("dns strategy = %v, want ipv4_only", dns["strategy"])
	}

	// A block outbound must exist for dropping naked IPv6.
	var hasBlock bool
	for _, o := range parsed["outbounds"].([]any) {
		if o.(map[string]any)["type"] == "block" {
			hasBlock = true
		}
	}
	if !hasBlock {
		t.Fatal("expected a block outbound for IPv6 leak protection")
	}

	// A route rule must block ip_version 6.
	s := string(raw)
	if !strings.Contains(s, `"ip_version": 6`) || !strings.Contains(s, `"outbound": "block"`) {
		t.Fatalf("expected an IPv6->block route rule\n%s", s)
	}
}

func TestGenerateConfigWithSplitTunnel(t *testing.T) {
	opts := routing.Options{
		Domains:      []string{".ru", "example.org"},
		IPCIDRs:      []string{"192.168.0.0/16"},
		RussiaDirect: true,
	}
	raw, err := GenerateConfigWith(sampleConfig(), opts)
	if err != nil {
		t.Fatalf("GenerateConfigWith: %v", err)
	}
	s := string(raw)
	for _, want := range []string{
		"domain_suffix", "ru", "example.org",
		"ip_cidr", "192.168.0.0/16",
		"geosite-ru", "geoip-ru", "rule_set",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("split-tunnel config missing %q\n%s", want, s)
		}
	}
}
