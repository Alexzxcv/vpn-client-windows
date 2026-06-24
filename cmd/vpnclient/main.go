// Command vpnclient is the Windows VPN client core: it enforces a single
// instance, starts the local control server, runs a system-tray controller and
// opens a WebView2 window pointed at the control server. The system proxy is
// removed crash-safely (on signal, panic and at startup if stale).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/Alexzxcv/vpn-client-windows/internal/app"
	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/control"
	"github.com/Alexzxcv/vpn-client-windows/internal/deviceid"
	"github.com/Alexzxcv/vpn-client-windows/internal/settings"
	"github.com/Alexzxcv/vpn-client-windows/internal/singbox"
	"github.com/Alexzxcv/vpn-client-windows/internal/singleinstance"
	"github.com/Alexzxcv/vpn-client-windows/internal/tokenstore"
	"github.com/Alexzxcv/vpn-client-windows/internal/xray"
)

// singleInstanceName is the named-mutex identity for the single-instance guard.
const singleInstanceName = "Global\\vpn-client-windows-singleton"

func main() {
	logLevel := slog.LevelInfo
	if os.Getenv("VPNCLIENT_DEBUG") != "" {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(log)

	// Single instance: a second launch would conflict on ports and the tray.
	lock, err := singleinstance.Acquire(singleInstanceName)
	if err != nil {
		if err == singleinstance.ErrAlreadyRunning {
			log.Warn("another instance is already running; exiting")
			return
		}
		log.Error("single-instance check", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer lock.Release()

	apiBase := backend.DefaultAPIBase(os.Getenv("VPNCLIENT_API_BASE"))
	be := backend.New(apiBase, nil)
	// Персистентность входа: подхватываем сохранённые токены и сохраняем при
	// каждом изменении (login/refresh/logout) — вход переживает перезапуск.
	if a, r := tokenstore.Load(); r != "" {
		be.SetTokens(a, r)
	}
	be.OnTokens = tokenstore.Save
	// On logout, guarantee the on-disk token file is removed (no leak), even if
	// OnTokens somehow did not. Device key is intentionally preserved.
	be.OnLogout = func() {
		if err := tokenstore.Clear(); err != nil {
			log.Warn("clear tokens on logout", slog.String("err", err.Error()))
		}
	}
	xm := xray.NewManager(log)
	sbm := singbox.NewManager(log) // TUN-движок (полный туннель)

	// Device crypto identity: Ed25519 keypair, private key sealed via DPAPI.
	id, err := deviceid.Load()
	if err != nil {
		log.Error("load device identity", slog.String("err", err.Error()))
		os.Exit(1)
	}

	set := settings.Load()
	application := app.New(log, be, xm, sbm, id, set, apiBase)

	// Адрес веб-дашборда для кнопки «Создать аккаунт» (регистрация в браузере).
	// Через env, т.к. домены ещё сменятся; дефолт — текущий прод.
	dashboardURL := os.Getenv("VPNCLIENT_DASHBOARD_URL")
	if dashboardURL == "" {
		dashboardURL = "https://bot.niffty.ru/"
	}
	application.SetDashboardURL(dashboardURL)

	// Crash-safe recovery: if a previous run died while connected, our system
	// proxy and/or kill-switch firewall rules may still be active — remove them
	// before doing anything else. Orphaned engine processes (xray/sing-box) that
	// outlived the crashed parent are terminated too so they cannot hold the
	// proxy ports or a stale TUN interface.
	application.CleanupStaleEngines()
	application.CleanupStaleProxy()
	application.CleanupStaleKillSwitch()

	srv, err := control.New(log, application, "")
	if err != nil {
		log.Error("create control server", slog.String("err", err.Error()))
		os.Exit(1)
	}
	if err := srv.Start(); err != nil {
		log.Error("start control server", slog.String("err", err.Error()))
		os.Exit(1)
	}

	url := srv.URL()
	log.Info("ui available", slog.String("url", url))

	// Window manager owns the native frameless window; the control server drives
	// its title-bar buttons via /api/window/*.
	window := newWindowManager(log, "SAPN VPN", url)
	srv.SetWindowController(window)

	// Background update checker (GitHub Releases). Gated by env; never
	// auto-applies — it only caches availability for the UI to surface.
	application.StartUpdateChecker()

	// graceful shutdown: stop xray, drop the system proxy, stop the server.
	var shutdownOnce bool
	shutdown := func() {
		if shutdownOnce {
			return
		}
		shutdownOnce = true
		application.StopUpdateChecker()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = application.Disconnect(ctx)
		application.ForceClearProxy() // belt-and-suspenders
		_ = srv.Shutdown(ctx)
		log.Info("shutdown complete")
	}

	// Crash-safe proxy removal on panic: clear before re-panicking.
	defer func() {
		if r := recover(); r != nil {
			application.ForceClearProxy()
			log.Error("panic; cleared system proxy", slog.Any("recover", r))
			panic(r)
		}
	}()

	// Signal handling: SIGINT/SIGTERM trigger a clean shutdown.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	if !hasUI() {
		// Headless build: no tray/window. Wait for a signal then shut down.
		log.Warn("native UI not available in this build; running headless. Open the URL manually.",
			slog.String("url", url))
		<-sig
		shutdown()
		return
	}

	tray := newTrayController(log, application, window)

	go func() {
		<-sig
		log.Info("signal received; shutting down")
		application.ForceClearProxy()
		tray.RequestQuit() // unblocks tray.Run() -> we shutdown below
	}()

	// Open the window once at startup so the app is visible on first run —
	// unless launched with --minimized (autostart-at-login), which starts the
	// app straight into the tray.
	if !slices.Contains(os.Args[1:], "--minimized") {
		tray.openWindow()
	}

	// tray.Run blocks until the user picks Quit (or RequestQuit from a signal).
	tray.Run()

	// Tray loop ended: tear everything down.
	window.Close()
	shutdown()
}
