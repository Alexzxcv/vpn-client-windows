package xray

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/routing"
)

func sampleVLESS() backend.VLESSConfig {
	return backend.VLESSConfig{
		Host: "example.com", Port: 443,
		UUID: "11111111-2222-3333-4444-555555555555",
		SNI:  "www.microsoft.com", Fingerprint: "chrome",
	}
}

func TestGenerateConfigNoRoutingByDefault(t *testing.T) {
	raw, err := GenerateConfig(sampleVLESS(), 10800, 10801)
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := parsed["routing"]; ok {
		t.Fatal("expected no routing block when options empty")
	}
}

func TestGenerateConfigWithRouting(t *testing.T) {
	opts := routing.Options{
		Domains:      []string{".ru", "example.org"},
		IPCIDRs:      []string{"10.0.0.0/8"},
		RussiaDirect: true,
	}
	raw, err := GenerateConfigWith(sampleVLESS(), 10800, 10801, opts)
	if err != nil {
		t.Fatalf("GenerateConfigWith: %v", err)
	}
	s := string(raw)
	for _, want := range []string{
		"routing", "outboundTag", "direct",
		"domain:ru", "example.org", "10.0.0.0/8",
		"geoip:ru", "geosite:ru",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("routing config missing %q\n%s", want, s)
		}
	}
}
