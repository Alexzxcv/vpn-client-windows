package control

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Alexzxcv/vpn-client-windows/internal/app"
	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/settings"
	"github.com/Alexzxcv/vpn-client-windows/internal/singbox"
	"github.com/Alexzxcv/vpn-client-windows/internal/xray"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	be := backend.New("http://localhost:8080", nil)
	xm := xray.NewManager(nil)
	sbm := singbox.NewManager(nil)
	application := app.New(nil, be, xm, sbm, nil, settings.Load(), "http://localhost:8080")
	s, err := New(nil, application, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = s.httpSrv.Close() })
	return s
}

func TestBootstrapPublicAndStatusNeedsToken(t *testing.T) {
	s := newTestServer(t)
	base := s.URL()

	// bootstrap is public
	resp, err := http.Get(base + "api/bootstrap")
	if err != nil {
		t.Fatalf("get bootstrap: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap status = %d", resp.StatusCode)
	}
	var boot struct {
		SessionToken string `json:"session_token"`
		APIBase      string `json:"api_base"`
		Version      string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&boot); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if boot.SessionToken != s.Token() {
		t.Fatalf("bootstrap token mismatch")
	}
	if boot.Version == "" || boot.APIBase == "" {
		t.Fatalf("bootstrap missing fields: %+v", boot)
	}

	// status without token -> 401
	resp2, err := http.Get(base + "api/status")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status without token = %d, want 401", resp2.StatusCode)
	}

	// status with token -> 200
	req, _ := http.NewRequest(http.MethodGet, base+"api/status", nil)
	req.Header.Set("Authorization", "Bearer "+s.Token())
	resp3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get status w/token: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("status w/token = %d, want 200", resp3.StatusCode)
	}
	body, _ := io.ReadAll(resp3.Body)
	if !strings.Contains(string(body), "disconnected") {
		t.Fatalf("status body unexpected: %s", body)
	}
}

func TestPlaceholderUIServed(t *testing.T) {
	s := newTestServer(t)
	resp, err := http.Get(s.URL())
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "UI not built") {
		t.Fatalf("expected placeholder, got: %s", body)
	}
}
