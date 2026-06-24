package deviceid

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"
)

func newTestIdentity(t *testing.T) *Identity {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return &Identity{priv: priv, pub: pub}
}

func TestPublicKeyB64Is32Bytes(t *testing.T) {
	id := newTestIdentity(t)
	raw, err := base64.StdEncoding.DecodeString(id.PublicKeyB64())
	if err != nil {
		t.Fatalf("public key not valid base64: %v", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		t.Fatalf("public key = %d bytes, want %d", len(raw), ed25519.PublicKeySize)
	}
}

func TestSignedHeadersRequiresDeviceID(t *testing.T) {
	id := newTestIdentity(t)
	if _, _, _, err := id.SignedHeaders(time.Now()); err == nil {
		t.Fatal("expected error when device not registered")
	}
}

func TestSignedHeadersVerifies(t *testing.T) {
	id := newTestIdentity(t)
	id.SetDeviceID("dev-123")
	now := time.Unix(1700000000, 0)

	deviceID, ts, sigB64, err := id.SignedHeaders(now)
	if err != nil {
		t.Fatalf("SignedHeaders: %v", err)
	}
	if deviceID != "dev-123" {
		t.Fatalf("device id = %q", deviceID)
	}
	if ts != "1700000000" {
		t.Fatalf("timestamp = %q, want 1700000000", ts)
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatalf("signature not base64: %v", err)
	}
	// Verify against the public key: signed message is "<id>.<ts>".
	msg := []byte(deviceID + "." + ts)
	if !ed25519.Verify(id.pub, msg, sig) {
		t.Fatal("signature does not verify against public key")
	}
}

func TestSealUnsealRoundtrip(t *testing.T) {
	// On non-windows seal/unseal are identity; on windows they exercise DPAPI.
	plain := []byte("the-secret-private-key-material-32b!!")
	sealed, err := seal(plain)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	got, err := unseal(sealed)
	if err != nil {
		t.Fatalf("unseal: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("roundtrip mismatch: %q != %q", got, plain)
	}
}
