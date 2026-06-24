package app

import (
	"testing"
	"time"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/settings"
	"github.com/Alexzxcv/vpn-client-windows/internal/singbox"
	"github.com/Alexzxcv/vpn-client-windows/internal/xray"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	be := backend.New("http://localhost:0", nil)
	xm := xray.NewManager(nil)
	sbm := singbox.NewManager(nil)
	set := settings.Load()
	return New(nil, be, xm, sbm, nil, set, "http://localhost:0")
}

func TestStatusModeDefaultsToProxy(t *testing.T) {
	a := newTestApp(t)
	st := a.Status()
	if st.Mode != ModeProxy {
		t.Fatalf("mode = %q, want %q", st.Mode, ModeProxy)
	}
	if st.State != StateDisconnected {
		t.Fatalf("state = %q, want disconnected", st.State)
	}
	// No proxy active -> no proxy_address.
	if st.ProxyAddress != "" {
		t.Fatalf("proxy_address = %q, want empty when not connected", st.ProxyAddress)
	}
}

func TestStatusProxyAddressWhenProxyOn(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	a.proxyOn = true
	a.mu.Unlock()
	st := a.Status()
	want := "127.0.0.1:10800"
	if st.ProxyAddress != want {
		t.Fatalf("proxy_address = %q, want %q", st.ProxyAddress, want)
	}
}

func TestParseMode(t *testing.T) {
	cases := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"", ModeProxy, false},
		{"proxy", ModeProxy, false},
		{"tun", ModeTUN, false},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := ParseMode(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("ParseMode(%q) expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseMode(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("ParseMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStatusReportsActualMode(t *testing.T) {
	a := newTestApp(t)
	a.setMode(ModeTUN)
	if got := a.Status().Mode; got != ModeTUN {
		t.Fatalf("Status mode = %q, want tun", got)
	}
}

// onEngineExit must not start a reconnect when not connected.
func TestOnEngineExitNoReconnectWhenDisconnected(t *testing.T) {
	a := newTestApp(t)
	a.onEngineExit()
	a.mu.Lock()
	reconnecting := a.reconnecting
	a.mu.Unlock()
	if reconnecting {
		t.Fatal("reconnecting should be false when not connected")
	}
}

// onEngineExit must not start a reconnect after a user disconnect
// (wantConnected=false), even if state still reads connected.
func TestOnEngineExitNoReconnectAfterUserDisconnect(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	a.state = StateConnected
	a.wantConnected = false
	a.mu.Unlock()

	a.onEngineExit()
	time.Sleep(50 * time.Millisecond)

	a.mu.Lock()
	reconnecting := a.reconnecting
	a.mu.Unlock()
	if reconnecting {
		t.Fatal("should not reconnect after user disconnect")
	}
}
