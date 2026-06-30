package app

// Multi-proxy: несколько локальных SOCKS5-прокси одновременно, каждый на своём
// порту туннелирует к своему серверу. Каждый запущенный прокси — отдельный
// процесс xray (свой xray.Manager) ради независимого start/stop, изоляции краша
// и пер-инстансного рефреша. Конфигурация (маппинги) хранится в settings;
// рантайм-состояние — в App.multi. Только proxy-режим; взаимоисключающе с
// одиночным подключением и TUN (см. connect()/StartMultiProxy).

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/settings"
	"github.com/Alexzxcv/vpn-client-windows/internal/sysproxy"
	"github.com/Alexzxcv/vpn-client-windows/internal/xray"
)

// MultiProxyEntry — один мульти-прокси для UI: конфиг + рантайм-состояние.
type MultiProxyEntry struct {
	ID       string `json:"id"`
	Port     int    `json:"port"`
	ServerID string `json:"server_id"`
	Main     bool   `json:"main"`
	State    State  `json:"state"`
	Error    string `json:"error,omitempty"`
	Address  string `json:"address,omitempty"` // 127.0.0.1:<port>, когда запущен
}

// proxyInstance — рантайм запущенного (или недавно запущенного) мульти-прокси.
type proxyInstance struct {
	port         int
	serverID     *string
	main         bool
	mgr          *xray.Manager
	state        State
	lastErr      string
	expiresAt    time.Time
	wantRun      bool // пользователь хочет, чтобы прокси работал (для авто-реконнекта)
	reconnecting bool
}

// multiServerID приводит сохранённый server_id к виду, который понимает connect-
// флоу: "" и "auto" → nil (бэкенд выберёт ноду), иначе указатель.
func multiServerID(s string) *string {
	if s == "" || s == AutoServerID {
		return nil
	}
	v := s
	return &v
}

func newProxyID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("multi-proxy: gen id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ListMultiProxy возвращает все настроенные маппинги, дополненные текущим
// рантайм-состоянием.
func (a *App) ListMultiProxy() []MultiProxyEntry {
	cur := a.set.Get()
	a.multiMu.Lock()
	defer a.multiMu.Unlock()
	out := make([]MultiProxyEntry, 0, len(cur.ProxyMappings))
	for _, m := range cur.ProxyMappings {
		e := MultiProxyEntry{ID: m.ID, Port: m.Port, ServerID: m.ServerID, Main: m.Main, State: StateDisconnected}
		if inst := a.multi[m.ID]; inst != nil {
			e.State = inst.state
			e.Error = inst.lastErr
			if inst.state == StateConnected {
				e.Address = fmt.Sprintf("127.0.0.1:%d", inst.port)
			}
		}
		out = append(out, e)
	}
	return out
}

// AddMultiProxy добавляет маппинг «порт → сервер». serverID: "auto" | id ноды |
// "custom:<id>". Порт должен быть свободен (среди маппингов и портов одиночного
// прокси). main=true делает прокси «основным» (на него выставляется системный
// прокси), сбрасывая флаг у остальных.
func (a *App) AddMultiProxy(port int, serverID string, main bool) (MultiProxyEntry, error) {
	cur := a.set.Get()
	if err := a.validatePort(port, "", cur); err != nil {
		return MultiProxyEntry{}, err
	}
	id, err := newProxyID()
	if err != nil {
		return MultiProxyEntry{}, err
	}
	if main {
		for i := range cur.ProxyMappings {
			cur.ProxyMappings[i].Main = false
		}
	}
	cur.ProxyMappings = append(cur.ProxyMappings, settings.ProxyMapping{
		ID: id, Port: port, ServerID: serverID, Main: main,
	})
	if err := a.set.Save(cur); err != nil {
		return MultiProxyEntry{}, err
	}
	return MultiProxyEntry{ID: id, Port: port, ServerID: serverID, Main: main, State: StateDisconnected}, nil
}

// UpdateMultiProxy меняет маппинг. Если прокси был запущен — останавливает его,
// чтобы изменения применились при следующем старте.
func (a *App) UpdateMultiProxy(id string, port int, serverID string, main bool) error {
	cur := a.set.Get()
	idx := -1
	for i := range cur.ProxyMappings {
		if cur.ProxyMappings[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("multi-proxy %q not found", id)
	}
	if err := a.validatePort(port, id, cur); err != nil {
		return err
	}
	if main {
		for i := range cur.ProxyMappings {
			cur.ProxyMappings[i].Main = false
		}
	}
	cur.ProxyMappings[idx].Port = port
	cur.ProxyMappings[idx].ServerID = serverID
	cur.ProxyMappings[idx].Main = main
	if err := a.set.Save(cur); err != nil {
		return err
	}
	_, _ = a.StopMultiProxy(id)
	return nil
}

// RemoveMultiProxy останавливает (если запущен) и удаляет маппинг.
func (a *App) RemoveMultiProxy(id string) error {
	_, _ = a.StopMultiProxy(id)
	cur := a.set.Get()
	out := cur.ProxyMappings[:0:0]
	for _, m := range cur.ProxyMappings {
		if m.ID != id {
			out = append(out, m)
		}
	}
	cur.ProxyMappings = out
	return a.set.Save(cur)
}

// validatePort проверяет, что порт валиден и не занят другими маппингами
// (исключая selfID) и портами одиночного прокси.
func (a *App) validatePort(port int, selfID string, cur settings.Settings) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	if port == cur.SocksPort || port == cur.HTTPPort {
		return fmt.Errorf("port %d is used by the main proxy", port)
	}
	for _, m := range cur.ProxyMappings {
		if m.ID != selfID && m.Port == port {
			return fmt.Errorf("port %d is already used", port)
		}
	}
	return nil
}

// StartMultiProxy запускает один мульти-прокси по id. Останавливает одиночное
// подключение/TUN (взаимоисключение), но другие мульти-прокси не трогает.
func (a *App) StartMultiProxy(ctx context.Context, id string) (State, error) {
	cur := a.set.Get()
	var m *settings.ProxyMapping
	for i := range cur.ProxyMappings {
		if cur.ProxyMappings[i].ID == id {
			m = &cur.ProxyMappings[i]
			break
		}
	}
	if m == nil {
		return StateError, fmt.Errorf("multi-proxy %q not found", id)
	}
	// Взаимоисключение: гасим одиночное подключение/TUN (они владеют xm/sbm и
	// системным прокси). Другие мульти-прокси продолжают работать.
	_ = a.Disconnect(ctx)
	return a.startProxyInstance(ctx, *m)
}

// startProxyInstance получает VLESS-конфиг сервера, генерирует SOCKS-only
// xray-конфиг на порт маппинга, запускает отдельный процесс xray и (если main)
// выставляет системный прокси. Заменяет предыдущий инстанс с тем же id.
func (a *App) startProxyInstance(ctx context.Context, m settings.ProxyMapping) (State, error) {
	sid := multiServerID(m.ServerID)

	var cfg backend.VLESSConfig
	if c, _, ok := a.customConfig(sid); ok {
		cfg = c // свой сервер — без backend
	} else {
		if err := a.ensureRegistered(ctx); err != nil {
			return a.failMulti(m.ID, err)
		}
		c, err := a.be.VPNConfig(ctx, a.id, sid)
		if err != nil {
			return a.failMulti(m.ID, fmt.Errorf("fetch vpn config: %w", err))
		}
		cfg = c
	}

	confJSON, err := xray.GenerateConfigWith(cfg, m.Port, 0, a.routeOptions())
	if err != nil {
		return a.failMulti(m.ID, fmt.Errorf("generate xray config: %w", err))
	}

	mgr := xray.NewManager(a.log)
	id := m.ID
	mgr.SetExitHandler(func() { a.onMultiExit(id) })

	a.multiMu.Lock()
	if old := a.multi[id]; old != nil && old.mgr != nil {
		_ = old.mgr.Stop() // плановая остановка прежнего процесса (onExit не дёрнется)
	}
	inst := &proxyInstance{
		port: m.Port, serverID: sid, main: m.Main, mgr: mgr,
		state: StateConnecting, wantRun: true, expiresAt: cfg.ExpiresAt,
	}
	a.multi[id] = inst
	a.multiMu.Unlock()

	if err := mgr.Start(ctx, confJSON); err != nil {
		return a.failMulti(id, fmt.Errorf("start xray: %w", err))
	}
	readyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	err = mgr.WaitReady(readyCtx, m.Port)
	cancel()
	if err != nil {
		_ = mgr.Stop()
		return a.failMulti(id, fmt.Errorf("xray not ready: %w", err))
	}

	if m.Main && a.manageProxy {
		if serr := sysproxy.SetSocks(fmt.Sprintf("127.0.0.1:%d", m.Port)); serr != nil {
			a.log.Error("multi-proxy: set system proxy", slog.String("err", serr.Error()))
		} else {
			a.multiMu.Lock()
			a.multiMainPort = m.Port
			a.multiMu.Unlock()
		}
	}

	a.multiMu.Lock()
	inst.state = StateConnected
	inst.lastErr = ""
	a.multiMu.Unlock()

	a.startMultiRefresh()
	a.log.Info("multi-proxy started", slog.String("id", id), slog.Int("port", m.Port))
	return StateConnected, nil
}

func (a *App) failMulti(id string, err error) (State, error) {
	a.log.Error("multi-proxy failed", slog.String("id", id), slog.String("err", err.Error()))
	a.multiMu.Lock()
	if inst := a.multi[id]; inst != nil {
		inst.state = StateError
		inst.lastErr = err.Error()
	}
	a.multiMu.Unlock()
	return StateError, err
}

// StopMultiProxy останавливает один мульти-прокси.
func (a *App) StopMultiProxy(id string) (State, error) {
	a.multiMu.Lock()
	inst := a.multi[id]
	wasMainPort := 0
	if inst != nil {
		inst.wantRun = false
		if inst.mgr != nil {
			_ = inst.mgr.Stop()
		}
		if inst.main {
			wasMainPort = inst.port
		}
		delete(a.multi, id)
	}
	a.multiMu.Unlock()

	if wasMainPort != 0 {
		a.clearMultiSysproxy(wasMainPort)
	}
	a.stopMultiRefreshIfIdle()
	return StateDisconnected, nil
}

// stopAllMulti останавливает все мульти-прокси (для взаимоисключения с одиночным
// подключением и при logout).
func (a *App) stopAllMulti() {
	a.multiMu.Lock()
	insts := a.multi
	a.multi = make(map[string]*proxyInstance)
	mainOwned := a.multiMainPort != 0
	a.multiMainPort = 0
	if a.multiRefreshStop != nil {
		close(a.multiRefreshStop)
		a.multiRefreshStop = nil
	}
	a.multiMu.Unlock()

	for _, inst := range insts {
		inst.wantRun = false
		if inst.mgr != nil {
			_ = inst.mgr.Stop()
		}
	}
	if mainOwned && a.manageProxy {
		if err := sysproxy.Clear(); err != nil {
			a.log.Error("multi-proxy: clear system proxy", slog.String("err", err.Error()))
		}
	}
}

// clearMultiSysproxy снимает системный прокси, только если он принадлежит
// мульти-прокси на этом порту.
func (a *App) clearMultiSysproxy(port int) {
	a.multiMu.Lock()
	own := a.multiMainPort == port
	if own {
		a.multiMainPort = 0
	}
	a.multiMu.Unlock()
	if own && a.manageProxy {
		if err := sysproxy.Clear(); err != nil {
			a.log.Error("multi-proxy: clear system proxy", slog.String("err", err.Error()))
		}
	}
}

// onMultiExit вызывается xray.Manager'ом инстанса при аварийном выходе процесса —
// запускает пер-инстансный авто-реконнект.
func (a *App) onMultiExit(id string) {
	a.multiMu.Lock()
	inst := a.multi[id]
	should := inst != nil && inst.state == StateConnected && inst.wantRun && !inst.reconnecting
	if should {
		inst.reconnecting = true
		inst.state = StateConnecting
	}
	a.multiMu.Unlock()
	if !should {
		return
	}
	a.log.Warn("multi-proxy engine exited; reconnecting", slog.String("id", id))
	go a.reconnectMulti(id)
}

func (a *App) reconnectMulti(id string) {
	backoff := reconnectBaseBackoff
	for attempt := 1; attempt <= reconnectMaxAttempts; attempt++ {
		a.multiMu.Lock()
		inst := a.multi[id]
		want := inst != nil && inst.wantRun
		a.multiMu.Unlock()
		if !want {
			return
		}
		time.Sleep(backoff)

		cur := a.set.Get()
		var m *settings.ProxyMapping
		for i := range cur.ProxyMappings {
			if cur.ProxyMappings[i].ID == id {
				m = &cur.ProxyMappings[i]
				break
			}
		}
		if m == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		state, err := a.startProxyInstance(ctx, *m)
		cancel()
		if err == nil && state == StateConnected {
			a.log.Info("multi-proxy reconnected", slog.String("id", id), slog.Int("attempt", attempt))
			return
		}
		backoff *= 2
		if backoff > reconnectMaxBackoff {
			backoff = reconnectMaxBackoff
		}
	}
	a.multiMu.Lock()
	if inst := a.multi[id]; inst != nil {
		inst.reconnecting = false
		inst.state = StateError
		inst.lastErr = "auto-reconnect failed"
	}
	a.multiMu.Unlock()
	a.log.Error("multi-proxy auto-reconnect gave up", slog.String("id", id))
}

// --- refresh credential'ов мульти-прокси (общий тикер) ---

func (a *App) startMultiRefresh() {
	a.multiMu.Lock()
	if a.multiRefreshStop != nil {
		a.multiMu.Unlock()
		return
	}
	stop := make(chan struct{})
	a.multiRefreshStop = stop
	a.multiMu.Unlock()
	go a.multiRefreshLoop(stop)
}

func (a *App) stopMultiRefreshIfIdle() {
	a.multiMu.Lock()
	if len(a.multi) == 0 && a.multiRefreshStop != nil {
		close(a.multiRefreshStop)
		a.multiRefreshStop = nil
	}
	a.multiMu.Unlock()
}

func (a *App) multiRefreshLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(refreshCheckEvery)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			a.maybeRefreshMulti()
		}
	}
}

// maybeRefreshMulti перевыпускает credential'ы у backend-прокси, близких к
// истечению (свои серверы expiry не имеют → пропускаются), и горячо
// перезапускает их процессы xray.
func (a *App) maybeRefreshMulti() {
	a.multiMu.Lock()
	var ids []string
	for id, inst := range a.multi {
		if inst.state == StateConnected && !inst.expiresAt.IsZero() &&
			time.Until(inst.expiresAt) <= refreshLeadTime && !inst.reconnecting {
			ids = append(ids, id)
		}
	}
	a.multiMu.Unlock()

	for _, id := range ids {
		cur := a.set.Get()
		var m *settings.ProxyMapping
		for i := range cur.ProxyMappings {
			if cur.ProxyMappings[i].ID == id {
				m = &cur.ProxyMappings[i]
				break
			}
		}
		if m == nil {
			continue
		}
		a.log.Info("multi-proxy credential nearing expiry; refreshing", slog.String("id", id))
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _ = a.startProxyInstance(ctx, *m)
		cancel()
	}
}
