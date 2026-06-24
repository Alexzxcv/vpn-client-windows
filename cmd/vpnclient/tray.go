package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/Alexzxcv/vpn-client-windows/internal/app"
)

// trayController wires the system tray to the App and the UI window. It runs the
// tray event loop (systray.Run) which owns the process lifecycle: the window can
// be closed without quitting (minimize-to-tray); Quit exits everything.
type trayController struct {
	log    *slog.Logger
	app    *app.App
	window *windowManager

	// quit closes once the user picks Quit (or a signal arrives); the caller's
	// shutdown sequence then runs.
	quit     chan struct{}
	quitOnce sync.Once

	mConnect    *systray.MenuItem
	mDisconnect *systray.MenuItem
	mStatus     *systray.MenuItem
	mOpen       *systray.MenuItem
	mQuit       *systray.MenuItem

	stopPoll chan struct{}
}

func newTrayController(log *slog.Logger, application *app.App, window *windowManager) *trayController {
	return &trayController{
		log:      log,
		app:      application,
		window:   window,
		quit:     make(chan struct{}),
		stopPoll: make(chan struct{}),
	}
}

// QuitChan is closed when the user requests quit via the tray.
func (t *trayController) QuitChan() <-chan struct{} { return t.quit }

// Run starts the tray event loop. It blocks until systray.Quit is called.
func (t *trayController) Run() {
	systray.Run(t.onReady, t.onExit)
}

// RequestQuit stops the tray loop (e.g. from a signal handler).
func (t *trayController) RequestQuit() {
	systray.Quit()
}

func (t *trayController) onReady() {
	systray.SetIcon(iconDisconnected)
	systray.SetTitle("VPN Client")
	systray.SetTooltip("VPN Client — disconnected")

	t.mStatus = systray.AddMenuItem("Status: disconnected", "Current connection status")
	t.mStatus.Disable()
	systray.AddSeparator()
	t.mConnect = systray.AddMenuItem("Connect", "Connect to the VPN")
	t.mDisconnect = systray.AddMenuItem("Disconnect", "Disconnect from the VPN")
	systray.AddSeparator()
	t.mOpen = systray.AddMenuItem("Open", "Open the VPN client window")
	systray.AddSeparator()
	t.mQuit = systray.AddMenuItem("Quit", "Exit the VPN client")

	go t.handleClicks()
	go t.pollStatus()
}

func (t *trayController) onExit() {
	close(t.stopPoll)
	t.signalQuit()
}

func (t *trayController) signalQuit() {
	t.quitOnce.Do(func() { close(t.quit) })
}

func (t *trayController) handleClicks() {
	for {
		select {
		case <-t.mConnect.ClickedCh:
			go t.doConnect()
		case <-t.mDisconnect.ClickedCh:
			go t.doDisconnect()
		case <-t.mOpen.ClickedCh:
			t.openWindow()
		case <-t.mQuit.ClickedCh:
			systray.Quit()
			return
		case <-t.stopPoll:
			return
		}
	}
}

func (t *trayController) openWindow() {
	if !hasUI() {
		t.log.Warn("UI window not available in this build")
		return
	}
	if t.window.IsOpen() {
		return
	}
	go t.window.Open() // runs on its own locked OS thread; returns on close
}

func (t *trayController) doConnect() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Tray quick-connect uses the default (proxy) mode — no admin required.
	if _, err := t.app.Connect(ctx, nil, ""); err != nil {
		t.log.Error("tray connect", slog.String("err", err.Error()))
	}
}

func (t *trayController) doDisconnect() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := t.app.Disconnect(ctx); err != nil {
		t.log.Error("tray disconnect", slog.String("err", err.Error()))
	}
}

// pollStatus refreshes the tray icon/labels from the App state.
func (t *trayController) pollStatus() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	t.applyStatus()
	for {
		select {
		case <-ticker.C:
			t.applyStatus()
		case <-t.stopPoll:
			return
		}
	}
}

func (t *trayController) applyStatus() {
	st := t.app.Status()
	switch st.State {
	case app.StateConnected:
		systray.SetIcon(iconConnected)
		systray.SetTooltip("VPN Client — connected")
		t.mStatus.SetTitle("Status: connected")
		t.mConnect.Disable()
		t.mDisconnect.Enable()
	case app.StateConnecting:
		systray.SetIcon(iconDisconnected)
		systray.SetTooltip("VPN Client — connecting…")
		t.mStatus.SetTitle("Status: connecting…")
		t.mConnect.Disable()
		t.mDisconnect.Enable()
	case app.StateError:
		systray.SetIcon(iconDisconnected)
		systray.SetTooltip("VPN Client — error")
		t.mStatus.SetTitle("Status: error")
		t.mConnect.Enable()
		t.mDisconnect.Disable()
	default: // disconnected
		systray.SetIcon(iconDisconnected)
		systray.SetTooltip("VPN Client — disconnected")
		t.mStatus.SetTitle("Status: disconnected")
		t.mConnect.Enable()
		t.mDisconnect.Disable()
	}
}
