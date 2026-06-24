// Package deviceid implements the cryptographic device-identity contract shared
// with the backend.
//
// On first run it generates an Ed25519 keypair. The PRIVATE key is persisted
// next to the auth tokens but never in cleartext: on Windows it is sealed with
// DPAPI (CryptProtectData, user scope) before hitting disk. The PUBLIC key is
// registered with the backend (POST /devices) and the backend returns a
// device_id. Subsequent /vpn/config requests are authenticated by signing
// "<device_id>.<timestamp>" with the private key.
//
// Security: the private key, the device_id and the signature are NEVER logged.
package deviceid

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// keyFileName is the on-disk file holding the sealed private key, next to
// tokens.json in %APPDATA%/sapn-vpn.
const keyFileName = "device_key"

// Identity owns the device keypair and the backend-assigned device_id. It is
// safe for concurrent use.
type Identity struct {
	mu       sync.RWMutex
	priv     ed25519.PrivateKey
	pub      ed25519.PublicKey
	deviceID string // assigned by the backend after registration; empty until then
}

// dir resolves the config directory (same as tokenstore: %APPDATA%/sapn-vpn).
func dir() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("deviceid: user config dir: %w", err)
	}
	return filepath.Join(d, "sapn-vpn"), nil
}

func keyPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, keyFileName), nil
}

// Load loads the persisted keypair, generating and persisting a new one on
// first run. The private key on disk is DPAPI-sealed (Windows).
func Load() (*Identity, error) {
	p, err := keyPath()
	if err != nil {
		return nil, err
	}

	sealed, err := os.ReadFile(p)
	switch {
	case err == nil:
		raw, uerr := unseal(sealed)
		if uerr != nil {
			return nil, fmt.Errorf("deviceid: unseal private key: %w", uerr)
		}
		if len(raw) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("deviceid: stored key has wrong size %d", len(raw))
		}
		priv := ed25519.PrivateKey(raw)
		return &Identity{
			priv: priv,
			pub:  priv.Public().(ed25519.PublicKey),
		}, nil
	case errors.Is(err, os.ErrNotExist):
		return generateAndPersist(p)
	default:
		return nil, fmt.Errorf("deviceid: read key file: %w", err)
	}
}

func generateAndPersist(p string) (*Identity, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("deviceid: generate key: %w", err)
	}
	sealed, err := seal(priv)
	if err != nil {
		return nil, fmt.Errorf("deviceid: seal private key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, fmt.Errorf("deviceid: mkdir: %w", err)
	}
	if err := os.WriteFile(p, sealed, 0o600); err != nil {
		return nil, fmt.Errorf("deviceid: write key file: %w", err)
	}
	return &Identity{priv: priv, pub: pub}, nil
}

// PublicKeyB64 returns the base64 (std) encoding of the 32-byte public key,
// as expected by POST /devices.
func (id *Identity) PublicKeyB64() string {
	id.mu.RLock()
	defer id.mu.RUnlock()
	return base64.StdEncoding.EncodeToString(id.pub)
}

// DeviceID returns the backend-assigned device_id (empty until SetDeviceID).
func (id *Identity) DeviceID() string {
	id.mu.RLock()
	defer id.mu.RUnlock()
	return id.deviceID
}

// SetDeviceID records the device_id returned by the backend on registration.
func (id *Identity) SetDeviceID(deviceID string) {
	id.mu.Lock()
	id.deviceID = deviceID
	id.mu.Unlock()
}

// SignedHeaders returns the per-request auth tuple for POST /vpn/config:
// the device_id, the unix-second timestamp string and the base64 (std)
// Ed25519 signature over "<device_id>.<timestamp>". It fails if no device_id
// has been recorded yet.
func (id *Identity) SignedHeaders(now time.Time) (deviceID, timestamp, signature string, err error) {
	id.mu.RLock()
	defer id.mu.RUnlock()
	if id.deviceID == "" {
		return "", "", "", errors.New("deviceid: device not registered yet")
	}
	ts := strconv.FormatInt(now.Unix(), 10)
	msg := id.deviceID + "." + ts
	sig := ed25519.Sign(id.priv, []byte(msg))
	return id.deviceID, ts, base64.StdEncoding.EncodeToString(sig), nil
}
