package customserver

import (
	"encoding/base64"
	"testing"
)

func TestParseVLESS_Full(t *testing.T) {
	link := "vless://11111111-2222-3333-4444-555555555555@example.com:443" +
		"?security=reality&pbk=PUBKEY&sid=ab12&sni=www.microsoft.com&fp=firefox&flow=xtls-rprx-vision#My%20Node"
	s, err := ParseVLESS(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.UUID != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("uuid = %q", s.UUID)
	}
	if s.Host != "example.com" || s.Port != 443 {
		t.Errorf("host:port = %s:%d", s.Host, s.Port)
	}
	if s.PublicKey != "PUBKEY" || s.ShortID != "ab12" {
		t.Errorf("reality = %q / %q", s.PublicKey, s.ShortID)
	}
	if s.SNI != "www.microsoft.com" || s.Fingerprint != "firefox" {
		t.Errorf("tls = %q / %q", s.SNI, s.Fingerprint)
	}
	if s.Flow != "xtls-rprx-vision" {
		t.Errorf("flow = %q", s.Flow)
	}
	if s.Name != "My Node" {
		t.Errorf("name = %q (want decoded fragment)", s.Name)
	}
	if s.Security != "reality" {
		t.Errorf("security = %q", s.Security)
	}
	if s.ID == "" {
		t.Error("id not generated")
	}
}

func TestParseVLESS_Defaults(t *testing.T) {
	// Без security/sni/fp — должны подставиться дефолты, имя = host.
	s, err := ParseVLESS("vless://uuid-xyz@1.2.3.4:8443")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Security != "reality" {
		t.Errorf("security default = %q", s.Security)
	}
	if s.SNI != "1.2.3.4" {
		t.Errorf("sni fallback = %q (want host)", s.SNI)
	}
	if s.Fingerprint != "chrome" {
		t.Errorf("fp default = %q", s.Fingerprint)
	}
	if s.Name != "1.2.3.4" {
		t.Errorf("name fallback = %q (want host)", s.Name)
	}
}

func TestParseVLESS_Errors(t *testing.T) {
	cases := []string{
		"http://example.com",       // не vless
		"vless://@example.com:443", // нет uuid
		"vless://uuid@:443",        // нет хоста
		"vless://uuid@example.com", // нет порта
	}
	for _, c := range cases {
		if _, err := ParseVLESS(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestParseSubscription_Base64(t *testing.T) {
	raw := "vless://a@h1:443?pbk=k1#n1\n" +
		"# комментарий, не ссылка\n" +
		"vless://b@h2:443?pbk=k2#n2\n"
	enc := base64.StdEncoding.EncodeToString([]byte(raw))
	servers := ParseSubscription(enc)
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
	if servers[0].Host != "h1" || servers[1].Host != "h2" {
		t.Errorf("hosts = %s, %s", servers[0].Host, servers[1].Host)
	}
}

func TestParseSubscription_PlainText(t *testing.T) {
	raw := "vless://a@h1:443#n1\nvless://b@h2:443#n2"
	servers := ParseSubscription(raw)
	if len(servers) != 2 {
		t.Fatalf("got %d, want 2", len(servers))
	}
}

func TestServer_VLESSConfig(t *testing.T) {
	s := Server{Host: "h", Port: 443, UUID: "u", Flow: "f", PublicKey: "p", ShortID: "s", SNI: "sn", Fingerprint: "chrome"}
	c := s.VLESSConfig()
	if c.Host != "h" || c.Port != 443 || c.UUID != "u" || c.PublicKey != "p" || c.SNI != "sn" {
		t.Errorf("conversion mismatch: %+v", c)
	}
	if !c.ExpiresAt.IsZero() {
		t.Error("custom server config must have zero ExpiresAt")
	}
}
