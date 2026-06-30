package xray

import (
	"encoding/json"
	"testing"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
)

func TestGenerateConfig(t *testing.T) {
	cfg := backend.VLESSConfig{
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

	raw, err := GenerateConfig(cfg, 10808, 10809)
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	inbounds, ok := parsed["inbounds"].([]any)
	if !ok || len(inbounds) != 2 {
		t.Fatalf("expected 2 inbounds, got %v", parsed["inbounds"])
	}

	socks := inbounds[0].(map[string]any)
	if socks["protocol"] != "socks" {
		t.Fatalf("first inbound should be socks, got %v", socks["protocol"])
	}
	if got := int(socks["port"].(float64)); got != 10808 {
		t.Fatalf("socks port = %d, want 10808", got)
	}
	if socks["listen"] != "127.0.0.1" {
		t.Fatalf("socks listen = %v, want 127.0.0.1", socks["listen"])
	}
	if udp := socks["settings"].(map[string]any)["udp"]; udp != true {
		t.Fatalf("socks udp should be true, got %v", udp)
	}

	httpIn := inbounds[1].(map[string]any)
	if httpIn["protocol"] != "http" {
		t.Fatalf("second inbound should be http, got %v", httpIn["protocol"])
	}
	if got := int(httpIn["port"].(float64)); got != 10809 {
		t.Fatalf("http port = %d, want 10809", got)
	}

	outbounds := parsed["outbounds"].([]any)
	out0 := outbounds[0].(map[string]any)
	if out0["protocol"] != "vless" {
		t.Fatalf("first outbound should be vless, got %v", out0["protocol"])
	}

	vnext := out0["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)
	if vnext["address"] != "example.com" {
		t.Fatalf("address = %v, want example.com", vnext["address"])
	}
	if got := int(vnext["port"].(float64)); got != 443 {
		t.Fatalf("port = %d, want 443", got)
	}
	user := vnext["users"].([]any)[0].(map[string]any)
	if user["id"] != cfg.UUID {
		t.Fatalf("user id mismatch")
	}
	if user["encryption"] != "none" {
		t.Fatalf("encryption = %v, want none", user["encryption"])
	}
	if user["flow"] != cfg.Flow {
		t.Fatalf("flow = %v, want %v", user["flow"], cfg.Flow)
	}

	ss := out0["streamSettings"].(map[string]any)
	if ss["network"] != "tcp" || ss["security"] != "reality" {
		t.Fatalf("streamSettings network/security wrong: %v", ss)
	}
	rs := ss["realitySettings"].(map[string]any)
	if rs["serverName"] != cfg.SNI {
		t.Fatalf("serverName = %v, want %v", rs["serverName"], cfg.SNI)
	}
	if rs["fingerprint"] != cfg.Fingerprint {
		t.Fatalf("fingerprint mismatch")
	}
	if rs["publicKey"] != cfg.PublicKey {
		t.Fatalf("publicKey mismatch")
	}
	if rs["shortId"] != cfg.ShortID {
		t.Fatalf("shortId mismatch")
	}
}

// httpPort == 0 (multi-proxy instances) must produce a SOCKS-only config: a
// single socks-in inbound on the given port, no http-in.
func TestGenerateConfig_SocksOnly(t *testing.T) {
	cfg := backend.VLESSConfig{
		Host: "example.com", Port: 443,
		UUID: "11111111-2222-3333-4444-555555555555",
		SNI:  "www.microsoft.com", PublicKey: "PUBKEY", ShortID: "abcd",
	}
	raw, err := GenerateConfig(cfg, 10810, 0)
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	inbounds := parsed["inbounds"].([]any)
	if len(inbounds) != 1 {
		t.Fatalf("want 1 inbound (socks-only), got %d", len(inbounds))
	}
	in0 := inbounds[0].(map[string]any)
	if in0["protocol"] != "socks" {
		t.Fatalf("inbound protocol = %v, want socks", in0["protocol"])
	}
	if int(in0["port"].(float64)) != 10810 {
		t.Fatalf("inbound port = %v, want 10810", in0["port"])
	}
}

func TestGenerateConfigValidation(t *testing.T) {
	_, err := GenerateConfig(backend.VLESSConfig{}, 1, 2)
	if err == nil {
		t.Fatal("expected error for incomplete config")
	}
}

// An empty fingerprint must default to "chrome": xray's Reality rejects an empty
// uTLS fingerprint and would not start (SOCKS never opens), even though sing-box
// tolerates it. Regression test for the proxy "context deadline exceeded" bug.
func TestGenerateConfig_DefaultsEmptyFingerprint(t *testing.T) {
	cfg := backend.VLESSConfig{
		Host: "example.com", Port: 443,
		UUID: "11111111-2222-3333-4444-555555555555",
		SNI:  "www.microsoft.com", PublicKey: "PUBKEY", ShortID: "abcd",
		// Fingerprint intentionally empty.
	}
	raw, err := GenerateConfig(cfg, 10808, 10809)
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	out0 := parsed["outbounds"].([]any)[0].(map[string]any)
	rs := out0["streamSettings"].(map[string]any)["realitySettings"].(map[string]any)
	if rs["fingerprint"] != "chrome" {
		t.Fatalf("empty fingerprint = %v, want default %q", rs["fingerprint"], "chrome")
	}
}
