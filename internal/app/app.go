// Package app is the orchestrator gluing backend, device id and xray together.
// It owns the connection state machine and is safe for concurrent use.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/device"
	"github.com/Alexzxcv/vpn-client-windows/internal/sysproxy"
	"github.com/Alexzxcv/vpn-client-windows/internal/xray"
)

// State is the connection state.
type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateError        State = "error"
)

// Default local proxy ports.
const (
	DefaultSocksPort = 10800
	DefaultHTTPPort  = 10801
)

// LocationInfo is the minimal current-location descriptor for Status.
type LocationInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Status is a snapshot of the orchestrator state.
type Status struct {
	Authenticated bool          `json:"authenticated"`
	Connected     bool          `json:"connected"`
	State         State         `json:"state"`
	Location      *LocationInfo `json:"location,omitempty"`
	Since         *time.Time    `json:"since,omitempty"`
	LastError     string        `json:"last_error,omitempty"`
}

// App orchestrates auth and the VPN connection lifecycle.
type App struct {
	log     *slog.Logger
	be      *backend.Client
	xm      *xray.Manager
	apiBase string

	socksPort int
	httpPort  int

	// manageProxy: ставить/снимать системный прокси Windows на connect/disconnect.
	// Отключается env VPNCLIENT_NO_SYSPROXY (например для headless-тестов).
	manageProxy bool

	mu        sync.Mutex
	state     State
	location  *LocationInfo
	since     *time.Time
	lastError string
	proxyOn   bool
}

// New builds an App. log may be nil. socksPort/httpPort default to
// DefaultSocksPort/DefaultHTTPPort when zero.
func New(log *slog.Logger, be *backend.Client, xm *xray.Manager, apiBase string, socksPort, httpPort int) *App {
	if log == nil {
		log = slog.Default()
	}
	if socksPort == 0 {
		socksPort = DefaultSocksPort
	}
	if httpPort == 0 {
		httpPort = DefaultHTTPPort
	}
	return &App{
		log:         log,
		be:          be,
		xm:          xm,
		apiBase:     apiBase,
		socksPort:   socksPort,
		httpPort:    httpPort,
		manageProxy: os.Getenv("VPNCLIENT_NO_SYSPROXY") == "",
		state:       StateDisconnected,
	}
}

// APIBase returns the backend base URL (for bootstrap).
func (a *App) APIBase() string { return a.apiBase }

// SocksPort / HTTPPort expose the configured local proxy ports.
func (a *App) SocksPort() int { return a.socksPort }
func (a *App) HTTPPort() int  { return a.httpPort }

// Login authenticates against the backend.
func (a *App) Login(ctx context.Context, email, password string) error {
	if err := a.be.Login(ctx, email, password); err != nil {
		return fmt.Errorf("app login: %w", err)
	}
	a.log.Info("login ok")
	return nil
}

// Logout disconnects (if needed) and clears backend tokens.
func (a *App) Logout(ctx context.Context) error {
	_ = a.Disconnect(ctx)
	a.be.ClearTokens()
	a.log.Info("logout")
	return nil
}

// Me returns the authenticated account.
func (a *App) Me(ctx context.Context) (backend.User, error) {
	return a.be.Me(ctx)
}

// Locations returns available locations.
func (a *App) Locations(ctx context.Context) ([]backend.Location, error) {
	return a.be.Locations(ctx)
}

// Status returns a snapshot of the current state.
func (a *App) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	var loc *LocationInfo
	if a.location != nil {
		l := *a.location
		loc = &l
	}
	var since *time.Time
	if a.since != nil {
		s := *a.since
		since = &s
	}
	return Status{
		Authenticated: a.be.Authenticated(),
		Connected:     a.state == StateConnected,
		State:         a.state,
		Location:      loc,
		Since:         since,
		LastError:     a.lastError,
	}
}

func (a *App) setState(s State, loc *LocationInfo, lastErr string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = s
	a.lastError = lastErr
	switch s {
	case StateConnected:
		now := time.Now()
		a.since = &now
		if loc != nil {
			a.location = loc
		}
	case StateDisconnected, StateError:
		a.since = nil
		a.location = nil
	case StateConnecting:
		if loc != nil {
			a.location = loc
		}
	}
}

// Connect fetches the VLESS config, generates the xray config, starts xray and
// waits for the SOCKS port to become ready. serverID may be nil.
func (a *App) Connect(ctx context.Context, serverID *string) (State, error) {
	a.setState(StateConnecting, nil, "")

	deviceID, err := device.MachineID()
	if err != nil {
		return a.fail(fmt.Errorf("device id: %w", err))
	}

	// Привязываем устройство (идемпотентно) — бэкенд требует этого до /vpn/config.
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "windows-client"
	}
	if err := a.be.RegisterDevice(ctx, deviceID, hostname, "windows"); err != nil {
		return a.fail(fmt.Errorf("register device: %w", err))
	}

	cfg, err := a.be.VPNConfig(ctx, deviceID, serverID)
	if err != nil {
		return a.fail(fmt.Errorf("fetch vpn config: %w", err))
	}

	confJSON, err := xray.GenerateConfig(cfg, a.socksPort, a.httpPort)
	if err != nil {
		return a.fail(fmt.Errorf("generate xray config: %w", err))
	}

	if err := a.xm.Start(ctx, confJSON); err != nil {
		return a.fail(fmt.Errorf("start xray: %w", err))
	}

	readyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := a.xm.WaitReady(readyCtx, a.socksPort); err != nil {
		_ = a.xm.Stop()
		return a.fail(fmt.Errorf("xray not ready: %w", err))
	}

	// Поднимаем системный прокси Windows на локальный xray (http+socks).
	if a.manageProxy {
		httpAddr := fmt.Sprintf("127.0.0.1:%d", a.httpPort)
		socksAddr := fmt.Sprintf("127.0.0.1:%d", a.socksPort)
		if err := sysproxy.Set(httpAddr, socksAddr); err != nil {
			// Туннель поднят, но прокси не выставился — не валим коннект, логируем.
			a.log.Error("set system proxy", slog.String("err", err.Error()))
		} else {
			a.mu.Lock()
			a.proxyOn = true
			a.mu.Unlock()
		}
	}

	var loc *LocationInfo
	if serverID != nil {
		loc = &LocationInfo{ID: *serverID}
	}
	a.setState(StateConnected, loc, "")
	a.log.Info("connected", slog.Int("socks", a.socksPort), slog.Int("http", a.httpPort))
	return StateConnected, nil
}

func (a *App) fail(err error) (State, error) {
	a.log.Error("connect failed", slog.String("err", err.Error()))
	a.setState(StateError, nil, err.Error())
	return StateError, err
}

// Disconnect stops xray, снимает системный прокси и сбрасывает состояние.
func (a *App) Disconnect(ctx context.Context) error {
	_ = ctx

	a.mu.Lock()
	wasProxy := a.proxyOn
	a.proxyOn = false
	a.mu.Unlock()
	if wasProxy {
		if err := sysproxy.Clear(); err != nil {
			a.log.Error("clear system proxy", slog.String("err", err.Error()))
		}
	}

	err := a.xm.Stop()
	a.setState(StateDisconnected, nil, "")
	a.log.Info("disconnected")
	if err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}
	return nil
}
