package app

import (
	"testing"
	"time"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/xray"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	be := backend.New("http://localhost:0", nil)
	xm := xray.NewManager(nil)
	return New(nil, be, xm, "http://localhost:0", 0, 0)
}

func TestStatusModeAlwaysProxy(t *testing.T) {
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

// onXrayExit must not start a reconnect when not connected.
func TestOnXrayExitNoReconnectWhenDisconnected(t *testing.T) {
	a := newTestApp(t)
	a.onXrayExit()
	a.mu.Lock()
	reconnecting := a.reconnecting
	a.mu.Unlock()
	if reconnecting {
		t.Fatal("reconnecting should be false when not connected")
	}
}

// onXrayExit must not start a reconnect after a user disconnect
// (wantConnected=false), even if state still reads connected.
func TestOnXrayExitNoReconnectAfterUserDisconnect(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	a.state = StateConnected
	a.wantConnected = false
	a.mu.Unlock()

	a.onXrayExit()
	time.Sleep(50 * time.Millisecond)

	a.mu.Lock()
	reconnecting := a.reconnecting
	a.mu.Unlock()
	if reconnecting {
		t.Fatal("should not reconnect after user disconnect")
	}
}
