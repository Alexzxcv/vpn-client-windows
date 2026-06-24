package singbox

import (
	"encoding/json"
	"testing"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
)

func sampleConfig() backend.VLESSConfig {
	return backend.VLESSConfig{
		Host:        "example.com",
		Port:        443,
		UUID:        "11111111-2222-3333-4444-555555555555",
		Security:    "reality",
		Flow:        "xtls-rprx-vision",
		PublicKey:   "PUBKEY",
		ShortID:     "abcd",
		SNI:         "www.microsoft.com",
		Fingerprint: "chrome",
	}
}

func TestGenerateConfig(t *testing.T) {
	raw, err := GenerateConfig(sampleConfig())
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// --- inbound: tun ---
	inbounds, ok := parsed["inbounds"].([]any)
	if !ok || len(inbounds) != 1 {
		t.Fatalf("expected 1 inbound, got %v", parsed["inbounds"])
	}
	tun := inbounds[0].(map[string]any)
	if tun["type"] != "tun" {
		t.Fatalf("inbound type = %v, want tun", tun["type"])
	}
	if tun["interface_name"] != tunInterfaceName {
		t.Fatalf("interface_name = %v, want %v", tun["interface_name"], tunInterfaceName)
	}
	addrs, ok := tun["address"].([]any)
	if !ok || len(addrs) != 1 || addrs[0] != tunAddress {
		t.Fatalf("address = %v, want [%v]", tun["address"], tunAddress)
	}
	if tun["auto_route"] != true {
		t.Fatalf("auto_route = %v, want true", tun["auto_route"])
	}
	if tun["strict_route"] != true {
		t.Fatalf("strict_route = %v, want true", tun["strict_route"])
	}
	if tun["stack"] != "system" {
		t.Fatalf("stack = %v, want system", tun["stack"])
	}
	if got := int(tun["mtu"].(float64)); got != tunMTU {
		t.Fatalf("mtu = %d, want %d", got, tunMTU)
	}

	// --- outbounds: vless + direct ---
	outbounds := parsed["outbounds"].([]any)
	if len(outbounds) != 2 {
		t.Fatalf("expected 2 outbounds, got %d", len(outbounds))
	}
	vless := outbounds[0].(map[string]any)
	if vless["type"] != "vless" {
		t.Fatalf("first outbound type = %v, want vless", vless["type"])
	}
	if vless["server"] != "example.com" {
		t.Fatalf("server = %v, want example.com", vless["server"])
	}
	if got := int(vless["server_port"].(float64)); got != 443 {
		t.Fatalf("server_port = %d, want 443", got)
	}
	if vless["uuid"] != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("uuid mismatch")
	}
	if vless["flow"] != "xtls-rprx-vision" {
		t.Fatalf("flow mismatch")
	}

	tls := vless["tls"].(map[string]any)
	if tls["enabled"] != true || tls["server_name"] != "www.microsoft.com" {
		t.Fatalf("tls block wrong: %v", tls)
	}
	utls := tls["utls"].(map[string]any)
	if utls["enabled"] != true || utls["fingerprint"] != "chrome" {
		t.Fatalf("utls block wrong: %v", utls)
	}
	reality := tls["reality"].(map[string]any)
	if reality["enabled"] != true || reality["public_key"] != "PUBKEY" || reality["short_id"] != "abcd" {
		t.Fatalf("reality block wrong: %v", reality)
	}

	direct := outbounds[1].(map[string]any)
	if direct["type"] != "direct" || direct["tag"] != "direct" {
		t.Fatalf("second outbound should be direct, got %v", direct)
	}

	// --- route ---
	route := parsed["route"].(map[string]any)
	if route["final"] != "vless-out" {
		t.Fatalf("route final = %v, want vless-out", route["final"])
	}
	if _, ok := route["rules"].([]any); !ok {
		t.Fatalf("route rules missing")
	}
}

func TestGenerateConfigFingerprintDefault(t *testing.T) {
	cfg := sampleConfig()
	cfg.Fingerprint = ""
	raw, err := GenerateConfig(cfg)
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	utls := parsed["outbounds"].([]any)[0].(map[string]any)["tls"].(map[string]any)["utls"].(map[string]any)
	if utls["fingerprint"] != "chrome" {
		t.Fatalf("default fingerprint = %v, want chrome", utls["fingerprint"])
	}
}

func TestGenerateConfigValidation(t *testing.T) {
	if _, err := GenerateConfig(backend.VLESSConfig{}); err == nil {
		t.Fatal("expected error for incomplete config")
	}
}
