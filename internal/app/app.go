// Package app is the orchestrator gluing backend, device id and xray together.
// It owns the connection state machine and is safe for concurrent use.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/device"
	"github.com/Alexzxcv/vpn-client-windows/internal/elevation"
	"github.com/Alexzxcv/vpn-client-windows/internal/singbox"
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

// Mode is the tunnelling mode. Only proxy is implemented today; tun (full
// WinTUN tunnel) is a future phase but the enum is reserved here so the status
// contract is stable.
type Mode string

const (
	ModeProxy Mode = "proxy"
	ModeTUN   Mode = "tun"
)

// Default local proxy ports.
const (
	DefaultSocksPort = 10800
	DefaultHTTPPort  = 10801
)

// Auto-reconnect tuning: how many attempts and the backoff schedule.
const (
	reconnectMaxAttempts = 5
	reconnectBaseBackoff = 1 * time.Second
	reconnectMaxBackoff  = 30 * time.Second
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
	Mode          Mode          `json:"mode"`
	ProxyAddress  string        `json:"proxy_address,omitempty"`
	Location      *LocationInfo `json:"location,omitempty"`
	Since         *time.Time    `json:"since,omitempty"`
	LastError     string        `json:"last_error,omitempty"`
}

// App orchestrates auth and the VPN connection lifecycle.
type App struct {
	log     *slog.Logger
	be      *backend.Client
	xm      *xray.Manager
	sbm     *singbox.Manager
	apiBase string

	socksPort int
	httpPort  int

	// manageProxy: ставить/снимать системный прокси Windows на connect/disconnect.
	// Отключается env VPNCLIENT_NO_SYSPROXY (например для headless-тестов).
	manageProxy bool

	mu            sync.Mutex
	state         State
	mode          Mode // фактический активный режим (proxy|tun)
	location      *LocationInfo
	since         *time.Time
	lastError     string
	proxyOn       bool
	lastServerID  *string // последний выбранный сервер (nil = пусть бэкенд выберёт)
	lastMode      Mode    // режим последнего connect (для авто-реконнекта)
	wantConnected bool    // пользователь хочет быть подключённым (для авто-реконнекта)
	reconnecting  bool    // идёт ли сейчас попытка авто-переподключения
}

// New builds an App. log may be nil. socksPort/httpPort default to
// DefaultSocksPort/DefaultHTTPPort when zero. sbm may be nil if the build does
// not ship the TUN engine (proxy-only); TUN connects then return an error.
func New(log *slog.Logger, be *backend.Client, xm *xray.Manager, sbm *singbox.Manager, apiBase string, socksPort, httpPort int) *App {
	if log == nil {
		log = slog.Default()
	}
	if socksPort == 0 {
		socksPort = DefaultSocksPort
	}
	if httpPort == 0 {
		httpPort = DefaultHTTPPort
	}
	a := &App{
		log:         log,
		be:          be,
		xm:          xm,
		sbm:         sbm,
		apiBase:     apiBase,
		socksPort:   socksPort,
		httpPort:    httpPort,
		manageProxy: os.Getenv("VPNCLIENT_NO_SYSPROXY") == "",
		state:       StateDisconnected,
		mode:        ModeProxy,
		lastMode:    ModeProxy,
	}
	// Авто-реконнект: если активный движок упал, пока мы в connected, пробуем
	// поднять заново. Оба менеджера зовут один и тот же обработник.
	xm.SetExitHandler(a.onEngineExit)
	if sbm != nil {
		sbm.SetExitHandler(a.onEngineExit)
	}
	return a
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
	st := Status{
		Authenticated: a.be.Authenticated(),
		Connected:     a.state == StateConnected,
		State:         a.state,
		Mode:          a.mode,
		Location:      loc,
		Since:         since,
		LastError:     a.lastError,
	}
	if a.proxyOn {
		st.ProxyAddress = fmt.Sprintf("127.0.0.1:%d", a.socksPort)
	}
	return st
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

// ParseMode normalises a mode string from the API. Empty defaults to proxy.
// Unknown values return an error.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "", string(ModeProxy):
		return ModeProxy, nil
	case string(ModeTUN):
		return ModeTUN, nil
	default:
		return "", fmt.Errorf("unknown mode %q (want proxy|tun)", s)
	}
}

// Connect fetches the VLESS config and brings up the tunnel in the requested
// mode. mode "" or "proxy" runs xray + the system proxy; mode "tun" runs
// sing-box with a full device-wide TUN (requires administrator rights).
// serverID may be nil.
func (a *App) Connect(ctx context.Context, serverID *string, mode string) (State, error) {
	m, err := ParseMode(mode)
	if err != nil {
		return a.fail(err)
	}

	// Запомним выбор для авто-реконнекта. Делаем копию, чтобы не держать
	// внешний указатель.
	a.mu.Lock()
	if serverID != nil {
		v := *serverID
		a.lastServerID = &v
	} else {
		a.lastServerID = nil
	}
	a.lastMode = m
	a.wantConnected = true
	a.mu.Unlock()

	return a.connect(ctx, serverID, m, true)
}

// connect performs the actual connection work in the given mode. manual
// distinguishes a user-initiated connect from an auto-reconnect (the caller owns
// lastServerID/lastMode/reconnecting bookkeeping).
func (a *App) connect(ctx context.Context, serverID *string, mode Mode, manual bool) (State, error) {
	_ = manual

	// TUN требует прав администратора и наличия TUN-движка в сборке.
	if mode == ModeTUN {
		if a.sbm == nil {
			return a.fail(fmt.Errorf("TUN-режим недоступен в этой сборке"))
		}
		if !elevation.IsElevated() {
			return a.fail(fmt.Errorf("TUN-режим требует запуска от администратора"))
		}
	}

	a.setMode(mode)
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

	if mode == ModeTUN {
		return a.connectTUN(ctx, cfg, serverID)
	}
	return a.connectProxy(ctx, cfg, serverID)
}

// connectProxy brings up xray + the Windows system proxy (existing behaviour).
func (a *App) connectProxy(ctx context.Context, cfg backend.VLESSConfig, serverID *string) (State, error) {
	// Если был активен TUN-движок (смена режима без disconnect) — остановим его.
	if a.sbm != nil {
		_ = a.sbm.Stop()
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

	a.setState(StateConnected, locFromServerID(serverID), "")
	a.log.Info("connected", slog.String("mode", string(ModeProxy)),
		slog.Int("socks", a.socksPort), slog.Int("http", a.httpPort))
	return StateConnected, nil
}

// connectTUN brings up sing-box with a full device-wide TUN. No system proxy is
// set: routing is handled by the TUN interface.
func (a *App) connectTUN(ctx context.Context, cfg backend.VLESSConfig, serverID *string) (State, error) {
	// Если был активен proxy-режим (смена режима без disconnect) — остановим xray
	// и снимем системный прокси, иначе он останется поверх TUN.
	_ = a.xm.Stop()
	a.mu.Lock()
	wasProxy := a.proxyOn
	a.proxyOn = false
	a.mu.Unlock()
	if wasProxy {
		if err := sysproxy.Clear(); err != nil {
			a.log.Error("clear system proxy", slog.String("err", err.Error()))
		}
	}

	confJSON, err := singbox.GenerateConfig(cfg)
	if err != nil {
		return a.fail(fmt.Errorf("generate sing-box config: %w", err))
	}

	if err := a.sbm.Start(ctx, confJSON); err != nil {
		return a.fail(fmt.Errorf("start sing-box: %w", err))
	}

	readyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := a.sbm.WaitReady(readyCtx); err != nil {
		_ = a.sbm.Stop()
		return a.fail(fmt.Errorf("sing-box not ready: %w", err))
	}

	a.setState(StateConnected, locFromServerID(serverID), "")
	a.log.Info("connected", slog.String("mode", string(ModeTUN)))
	return StateConnected, nil
}

func locFromServerID(serverID *string) *LocationInfo {
	if serverID == nil {
		return nil
	}
	return &LocationInfo{ID: *serverID}
}

// setMode records the active tunnelling mode.
func (a *App) setMode(m Mode) {
	a.mu.Lock()
	a.mode = m
	a.mu.Unlock()
}

func (a *App) fail(err error) (State, error) {
	a.log.Error("connect failed", slog.String("err", err.Error()))
	a.setState(StateError, nil, err.Error())
	return StateError, err
}

// Disconnect stops xray, снимает системный прокси и сбрасывает состояние.
// Это пользовательский disconnect: он отменяет любой авто-реконнект.
func (a *App) Disconnect(ctx context.Context) error {
	_ = ctx

	a.mu.Lock()
	wasProxy := a.proxyOn
	a.proxyOn = false
	a.wantConnected = false // явный disconnect — не переподключаемся
	a.lastServerID = nil
	a.mu.Unlock()
	if wasProxy {
		if err := sysproxy.Clear(); err != nil {
			a.log.Error("clear system proxy", slog.String("err", err.Error()))
		}
	}

	// Останавливаем оба движка: активным мог быть xray (proxy) или sing-box (tun).
	var errs []error
	if err := a.xm.Stop(); err != nil {
		errs = append(errs, err)
	}
	if a.sbm != nil {
		if err := a.sbm.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	a.setState(StateDisconnected, nil, "")
	a.log.Info("disconnected")
	if len(errs) > 0 {
		return fmt.Errorf("disconnect: %w", errors.Join(errs...))
	}
	return nil
}

// onEngineExit вызывается менеджером движка (xray или sing-box), когда процесс
// упал сам (не по Stop). Если мы в connected-состоянии — запускаем
// авто-переподключение с backoff в том же режиме.
func (a *App) onEngineExit() {
	a.mu.Lock()
	// Реагируем только если пользователь хочет быть подключённым и мы реально
	// были connected (а не в процессе остановки/ошибки).
	shouldReconnect := a.state == StateConnected && a.wantConnected && !a.reconnecting
	if shouldReconnect {
		a.reconnecting = true
	}
	a.mu.Unlock()
	if !shouldReconnect {
		return
	}

	a.log.Warn("tunnel engine exited unexpectedly; starting auto-reconnect")
	go a.reconnectLoop()
}

// reconnectLoop пытается переподключиться несколько раз с экспоненциальным
// backoff. Между попытками проверяет, не вмешался ли пользователь.
func (a *App) reconnectLoop() {
	defer func() {
		a.mu.Lock()
		a.reconnecting = false
		a.mu.Unlock()
	}()

	a.setState(StateConnecting, nil, "")

	backoff := reconnectBaseBackoff
	for attempt := 1; attempt <= reconnectMaxAttempts; attempt++ {
		// Пользователь мог нажать disconnect/logout, пока мы ждали.
		a.mu.Lock()
		serverID := a.lastServerID
		mode := a.lastMode
		canceled := !a.wantConnected
		a.mu.Unlock()
		if canceled {
			a.log.Info("auto-reconnect canceled (disconnected by user)")
			return
		}

		time.Sleep(backoff)

		a.mu.Lock()
		canceled = !a.wantConnected
		a.mu.Unlock()
		if canceled {
			a.log.Info("auto-reconnect canceled (disconnected by user)")
			return
		}

		a.log.Info("auto-reconnect attempt",
			slog.Int("attempt", attempt), slog.Int("max", reconnectMaxAttempts))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		state, err := a.reconnectOnce(ctx, serverID, mode)
		cancel()
		if err == nil && state == StateConnected {
			a.log.Info("auto-reconnect succeeded", slog.Int("attempt", attempt))
			return
		}
		a.log.Warn("auto-reconnect attempt failed",
			slog.Int("attempt", attempt), slog.String("err", errString(err)))

		backoff *= 2
		if backoff > reconnectMaxBackoff {
			backoff = reconnectMaxBackoff
		}
	}

	a.setState(StateError, nil, "auto-reconnect failed after retries")
	a.log.Error("auto-reconnect gave up")
}

// reconnectOnce performs a single connect attempt without touching lastServerID,
// lastMode or the reconnecting flag (the loop owns those).
func (a *App) reconnectOnce(ctx context.Context, serverID *string, mode Mode) (State, error) {
	return a.connect(ctx, serverID, mode, false)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ForceClearProxy снимает системный прокси без остановки xray и без захвата
// long-held состояния — пригодно для defer/обработчиков сигналов и паники,
// чтобы прокси не «завис» при некорректном завершении. Идемпотентно.
func (a *App) ForceClearProxy() {
	a.mu.Lock()
	wasProxy := a.proxyOn
	a.proxyOn = false
	a.mu.Unlock()
	if !wasProxy {
		return
	}
	if err := sysproxy.Clear(); err != nil {
		a.log.Error("force clear system proxy", slog.String("err", err.Error()))
		return
	}
	a.log.Info("system proxy cleared (crash-safe)")
}

// CleanupStaleProxy снимает наш системный прокси, если он остался включённым с
// прошлого (аварийно завершённого) запуска. Чужой прокси не трогает. Вызывать
// при старте ядра ДО первого connect.
func (a *App) CleanupStaleProxy() {
	if !a.manageProxy {
		return
	}
	cleared, err := sysproxy.ClearIfOurs(a.socksPort, a.httpPort)
	if err != nil {
		a.log.Warn("cleanup stale proxy", slog.String("err", err.Error()))
		return
	}
	if cleared {
		a.log.Warn("removed stale system proxy left from a previous run")
	}
}
