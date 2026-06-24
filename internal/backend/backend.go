// Package backend is an HTTP client to the vpn_service backend.
//
// It performs auth (login + refresh-on-401) and fetches VPN data
// (locations, per-device VLESS Reality config). Access/refresh tokens are kept
// in the client and are available for persistence by callers, but are never
// logged. VLESS UUIDs and reality keys are likewise never logged.
package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultAPIBase = "https://bot.niffty.ru/api"

// DefaultAPIBase resolves the backend base URL. Pass the value of the
// VPNCLIENT_API_BASE env var (or empty for the default).
func DefaultAPIBase(envVal string) string {
	if envVal = strings.TrimSpace(envVal); envVal != "" {
		return strings.TrimRight(envVal, "/")
	}
	return defaultAPIBase
}

// Location is a selectable VPN location returned by /vpn/locations.
type Location struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
	// LatencyMs is the backend-measured latency/health for this node in
	// milliseconds (0 if the backend did not provide it). Surfaced to the UI so
	// the user can compare nodes and so "Auto (best)" is explainable.
	LatencyMs int `json:"latency_ms,omitempty"`
}

// FreeDaily describes the free daily traffic allowance (when the account has no
// paid subscription, or in addition to it). All sizes are in bytes.
type FreeDaily struct {
	LimitBytes     int64     `json:"limit_bytes"`
	UsedTodayBytes int64     `json:"used_today_bytes"`
	ResetsAt       time.Time `json:"resets_at"`
}

// Subscription is the current account subscription / quota snapshot from
// GET /subscription. Byte counters are absolute totals for the current period.
type Subscription struct {
	TrafficUsedBytes  int64      `json:"traffic_used_bytes"`
	TrafficLimitBytes int64      `json:"traffic_limit_bytes"`
	FreeDaily         *FreeDaily `json:"free_daily,omitempty"`
}

// UsageSample is one point of the traffic-over-time series from
// GET /subscription/usage. Ts is the sample wall-clock time; UsedBytes is the
// cumulative bytes used at that moment, LimitBytes the limit then in force.
type UsageSample struct {
	Ts         time.Time `json:"ts"`
	UsedBytes  int64     `json:"used_bytes"`
	LimitBytes int64     `json:"limit_bytes"`
}

// Usage is the windowed traffic series from GET /subscription/usage.
type Usage struct {
	Samples []UsageSample `json:"samples"`
}

// VLESSConfig is the per-device VLESS Reality outbound config from /vpn/config.
type VLESSConfig struct {
	Host        string
	Port        int
	UUID        string
	Security    string
	Flow        string
	PublicKey   string
	ShortID     string
	SNI         string
	Fingerprint string
	// ExpiresAt is when this credential expires (zero if the backend did not
	// provide it). The app uses it to schedule a background refresh.
	ExpiresAt time.Time
}

// DeviceSigner produces the per-request device-identity headers for
// /vpn/config. It is implemented by internal/deviceid.Identity. Kept as an
// interface so backend does not import deviceid (avoids a cycle and keeps the
// crypto in one place).
type DeviceSigner interface {
	// SignedHeaders returns device_id, unix-second timestamp and the base64
	// Ed25519 signature over "<device_id>.<timestamp>".
	SignedHeaders(now time.Time) (deviceID, timestamp, signature string, err error)
}

// User is the authenticated account (GET /me).
type User struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
}

// Client talks to the vpn_service backend. It is safe for concurrent use.
type Client struct {
	base string
	http *http.Client

	// OnTokens, если задан, вызывается при каждом изменении токенов
	// (login/refresh/clear) — для персистентности на диске. Установить до
	// первого использования.
	OnTokens func(access, refresh string)

	// OnLogout, если задан, вызывается при ClearTokens (logout) ПОСЛЕ обнуления
	// токенов в памяти и OnTokens. Предназначен для гарантированной очистки
	// токенов с диска (tokenstore.Clear), чтобы не осталось утечки на диске
	// даже если OnTokens по какой-то причине не удалил файл. Идемпотентен.
	OnLogout func()

	// refreshMu serializes token refreshes (single-flight). Without it,
	// concurrent 401s on startup each call /auth/refresh with the SAME refresh
	// token; the backend rotates refresh tokens and treats the second use as
	// reuse -> revokes all sessions -> the user is logged out. Single-flight
	// guarantees one refresh; latecomers reuse the freshly minted access token.
	refreshMu sync.Mutex

	mu      sync.RWMutex
	access  string
	refresh string
}

// fireOnTokens уведомляет подписчика о текущих токенах (для сохранения).
func (c *Client) fireOnTokens() {
	if c.OnTokens == nil {
		return
	}
	c.mu.RLock()
	a, r := c.access, c.refresh
	c.mu.RUnlock()
	c.OnTokens(a, r)
}

// New creates a backend client. base is the backend root URL (no trailing slash
// required). If http is nil a client with a sane timeout is used.
func New(base string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		base: strings.TrimRight(base, "/"),
		http: hc,
	}
}

// Tokens returns the currently stored access and refresh tokens (for
// persistence by the caller). Never log these.
func (c *Client) Tokens() (access, refresh string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.access, c.refresh
}

// SetTokens injects previously persisted tokens.
func (c *Client) SetTokens(access, refresh string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.access = access
	c.refresh = refresh
}

// Authenticated reports whether an access token is present.
func (c *Client) Authenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.access != ""
}

// ClearTokens drops any stored tokens (logout) and runs the OnLogout cleanup
// hook (e.g. wiping the on-disk token file) so no tokens leak to disk.
func (c *Client) ClearTokens() {
	c.mu.Lock()
	c.access = ""
	c.refresh = ""
	c.mu.Unlock()
	c.fireOnTokens()
	if c.OnLogout != nil {
		c.OnLogout()
	}
}

type loginResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Login authenticates with email/password and stores the returned tokens.
func (c *Client) Login(ctx context.Context, login, password string) error {
	body := map[string]string{"login": login, "password": password}
	var out loginResp
	if err := c.doJSON(ctx, http.MethodPost, "/auth/login", body, &out, false); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if out.AccessToken == "" {
		return errors.New("login: empty access_token in response")
	}
	c.mu.Lock()
	c.access = out.AccessToken
	c.refresh = out.RefreshToken
	c.mu.Unlock()
	c.fireOnTokens()
	return nil
}

// refreshTokens calls POST /auth/refresh once to renew the access token.
// staleAccess is the access token that just got a 401; single-flight ensures we
// don't replay an already-rotated refresh token (which the backend treats as
// reuse and revokes all sessions).
func (c *Client) refreshTokens(ctx context.Context, staleAccess string) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	// If another goroutine already refreshed while we waited for the lock, the
	// access token changed — just use it instead of replaying the refresh token.
	c.mu.RLock()
	cur, rt := c.access, c.refresh
	c.mu.RUnlock()
	if cur != "" && cur != staleAccess {
		return nil
	}
	if rt == "" {
		return errors.New("no refresh token")
	}
	body := map[string]string{"refresh_token": rt}
	var out loginResp
	if err := c.doJSON(ctx, http.MethodPost, "/auth/refresh", body, &out, false); err != nil {
		return fmt.Errorf("refresh: %w", err)
	}
	if out.AccessToken == "" {
		return errors.New("refresh: empty access_token in response")
	}
	c.mu.Lock()
	c.access = out.AccessToken
	if out.RefreshToken != "" {
		c.refresh = out.RefreshToken
	}
	c.mu.Unlock()
	c.fireOnTokens()
	return nil
}

// Me returns the authenticated account.
func (c *Client) Me(ctx context.Context) (User, error) {
	var u User
	if err := c.doJSON(ctx, http.MethodGet, "/me", nil, &u, true); err != nil {
		return User{}, fmt.Errorf("me: %w", err)
	}
	return u, nil
}

// Locations returns available VPN locations.
func (c *Client) Locations(ctx context.Context) ([]Location, error) {
	var locs []Location
	if err := c.doJSON(ctx, http.MethodGet, "/vpn/locations", nil, &locs, true); err != nil {
		return nil, fmt.Errorf("locations: %w", err)
	}
	return locs, nil
}

// Subscription returns the current account subscription / quota snapshot.
func (c *Client) Subscription(ctx context.Context) (Subscription, error) {
	var s Subscription
	if err := c.doJSON(ctx, http.MethodGet, "/subscription", nil, &s, true); err != nil {
		return Subscription{}, fmt.Errorf("subscription: %w", err)
	}
	return s, nil
}

// Usage returns the traffic-over-time series for the last `hours` hours
// (used for the connect-page sparkline). hours defaults to 24 when <= 0.
func (c *Client) Usage(ctx context.Context, hours int) (Usage, error) {
	if hours <= 0 {
		hours = 24
	}
	var u Usage
	path := fmt.Sprintf("/subscription/usage?hours=%d", hours)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &u, true); err != nil {
		return Usage{}, fmt.Errorf("usage: %w", err)
	}
	return u, nil
}

// vpnConfigResp matches the wire shape of POST /vpn/config.
type vpnConfigResp struct {
	Server      string `json:"server"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	UUID        string `json:"uuid"`
	Security    string `json:"security"`
	Flow        string `json:"flow"`
	PublicKey   string `json:"public_key"`
	ShortID     string `json:"short_id"`
	SNI         string `json:"sni"`
	Fingerprint string `json:"fingerprint"`
	ExpiresAt   string `json:"expires_at"` // RFC3339
}

// VPNConfig fetches the VLESS Reality config for the optional server. The
// request is authenticated with the device-identity headers produced by signer
// (X-Device-Id / X-Device-Timestamp / X-Device-Signature). serverID may be nil
// to let the backend pick.
func (c *Client) VPNConfig(ctx context.Context, signer DeviceSigner, serverID *string) (VLESSConfig, error) {
	deviceID, ts, sig, err := signer.SignedHeaders(time.Now())
	if err != nil {
		return VLESSConfig{}, fmt.Errorf("vpn config: sign request: %w", err)
	}
	headers := map[string]string{
		"X-Device-Id":        deviceID,
		"X-Device-Timestamp": ts,
		"X-Device-Signature": sig,
	}

	body := map[string]any{}
	if serverID != nil {
		body["server_id"] = *serverID
	}
	var r vpnConfigResp
	if err := c.doJSONHeaders(ctx, http.MethodPost, "/vpn/config", body, &r, true, headers); err != nil {
		return VLESSConfig{}, fmt.Errorf("vpn config: %w", err)
	}
	host := r.Host
	if host == "" {
		host = r.Server // backend contract names this field "server"
	}
	var expires time.Time
	if r.ExpiresAt != "" {
		if t, perr := time.Parse(time.RFC3339, r.ExpiresAt); perr == nil {
			expires = t
		}
	}
	return VLESSConfig{
		Host:        host,
		Port:        r.Port,
		UUID:        r.UUID,
		Security:    r.Security,
		Flow:        r.Flow,
		PublicKey:   r.PublicKey,
		ShortID:     r.ShortID,
		SNI:         r.SNI,
		Fingerprint: r.Fingerprint,
		ExpiresAt:   expires,
	}, nil
}

// registerDeviceResp is the POST /devices response.
type registerDeviceResp struct {
	DeviceID string `json:"device_id"`
}

// RegisterDevice registers this device's public key (idempotent by public_key)
// and returns the backend-assigned device_id. mac is optional telemetry and may
// be empty. Required before /vpn/config.
func (c *Client) RegisterDevice(ctx context.Context, publicKeyB64, name, platform, mac string) (string, error) {
	body := map[string]string{
		"public_key": publicKeyB64,
		"name":       name,
		"platform":   platform,
	}
	if mac != "" {
		body["mac"] = mac
	}
	var out registerDeviceResp
	if err := c.doJSON(ctx, http.MethodPost, "/devices", body, &out, true); err != nil {
		return "", fmt.Errorf("register device: %w", err)
	}
	if out.DeviceID == "" {
		return "", errors.New("register device: empty device_id in response")
	}
	return out.DeviceID, nil
}

// doJSON performs an HTTP request, optionally attaching the bearer token and
// retrying once after a refresh on a 401.
func (c *Client) doJSON(ctx context.Context, method, path string, in, out any, auth bool) error {
	return c.doJSONHeaders(ctx, method, path, in, out, auth, nil)
}

// doJSONHeaders is doJSON with extra request headers (e.g. device-identity
// headers). The headers are re-sent on the post-refresh retry.
func (c *Client) doJSONHeaders(ctx context.Context, method, path string, in, out any, auth bool, headers map[string]string) error {
	c.mu.RLock()
	usedAccess := c.access
	c.mu.RUnlock()
	err := c.doOnce(ctx, method, path, in, out, auth, headers)
	if auth && errors.Is(err, errUnauthorized) {
		if rerr := c.refreshTokens(ctx, usedAccess); rerr != nil {
			return fmt.Errorf("after 401: %w", rerr)
		}
		return c.doOnce(ctx, method, path, in, out, auth, headers)
	}
	return err
}

var errUnauthorized = errors.New("unauthorized (401)")

func (c *Client) doOnce(ctx context.Context, method, path string, in, out any, auth bool, headers map[string]string) error {
	var bodyRdr io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyRdr = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bodyRdr)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if auth {
		c.mu.RLock()
		at := c.access
		c.mu.RUnlock()
		if at != "" {
			req.Header.Set("Authorization", "Bearer "+at)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		// drain to allow connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		return errUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	if out != nil {
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(out); err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("decode response: %w", err)
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	return nil
}
