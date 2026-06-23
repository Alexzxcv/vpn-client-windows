// Command vpnclient is the Windows VPN client core: it starts the local control
// server and opens a WebView2 window pointed at it.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/Alexzxcv/vpn-client-windows/internal/app"
	"github.com/Alexzxcv/vpn-client-windows/internal/backend"
	"github.com/Alexzxcv/vpn-client-windows/internal/control"
	"github.com/Alexzxcv/vpn-client-windows/internal/xray"
)

func main() {
	logLevel := slog.LevelInfo
	if os.Getenv("VPNCLIENT_DEBUG") != "" {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(log)

	apiBase := backend.DefaultAPIBase(os.Getenv("VPNCLIENT_API_BASE"))
	be := backend.New(apiBase, nil)
	xm := xray.NewManager(log)
	application := app.New(log, be, xm, apiBase, 0, 0)

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

	// graceful shutdown helper
	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = application.Disconnect(ctx)
		_ = srv.Shutdown(ctx)
		log.Info("shutdown complete")
	}

	// Open the UI window. runUI blocks until the window is closed. On builds
	// without WebView2 support it returns immediately and we wait for a signal.
	if hasUI() {
		runUI(log, "VPN Client", url)
		shutdown()
		return
	}

	log.Warn("WebView2 UI not available in this build; running headless. Open the URL manually.",
		slog.String("url", url))
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	shutdown()
}
