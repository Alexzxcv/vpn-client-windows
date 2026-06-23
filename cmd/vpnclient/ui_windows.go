//go:build windows

package main

import (
	"log/slog"
	"runtime"
	"sync"

	"github.com/jchv/go-webview2"
)

// hasUI reports whether a native UI window can be opened.
func hasUI() bool { return true }

// windowManager owns the lifecycle of the WebView2 window. The window can be
// closed (minimize-to-tray) and re-opened from the tray without exiting the
// process. Only one window exists at a time.
type windowManager struct {
	log   *slog.Logger
	title string
	url   string

	mu   sync.Mutex
	wv   webview2.WebView
	open bool
}

func newWindowManager(log *slog.Logger, title, url string) *windowManager {
	return &windowManager{log: log, title: title, url: url}
}

// Open shows the window. If one is already open it is a no-op (the existing
// window stays). Open blocks on its own locked OS thread until the window is
// closed; callers run it in a dedicated goroutine.
func (m *windowManager) Open() {
	m.mu.Lock()
	if m.open {
		m.mu.Unlock()
		return
	}
	m.open = true
	m.mu.Unlock()

	// WebView2 requires its message loop on a single OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug: false,
		WindowOptions: webview2.WindowOptions{
			Title:  m.title,
			Width:  420,
			Height: 720,
			IconId: 0,
			Center: true,
		},
	})
	if w == nil {
		m.log.Error("failed to create WebView2 window (is the WebView2 runtime installed?)")
		m.mu.Lock()
		m.open = false
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.wv = w
	m.mu.Unlock()

	w.Navigate(m.url)
	w.Run() // blocks until the window is closed

	w.Destroy()
	m.mu.Lock()
	m.wv = nil
	m.open = false
	m.mu.Unlock()
}

// Close requests the currently open window (if any) to close. Safe to call from
// another goroutine.
func (m *windowManager) Close() {
	m.mu.Lock()
	w := m.wv
	m.mu.Unlock()
	if w != nil {
		w.Terminate()
	}
}

// IsOpen reports whether a window is currently shown.
func (m *windowManager) IsOpen() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.open
}
