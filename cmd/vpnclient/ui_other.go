//go:build !windows

package main

import "log/slog"

// hasUI reports whether a native UI window can be opened. WebView2 is
// Windows-only, so non-Windows builds run headless.
func hasUI() bool { return false }

// windowManager is a no-op on non-Windows platforms.
type windowManager struct {
	log *slog.Logger
	url string
}

func newWindowManager(log *slog.Logger, title, url string) *windowManager {
	_ = title
	return &windowManager{log: log, url: url}
}

func (m *windowManager) Open()           {}
func (m *windowManager) Close()          {}
func (m *windowManager) Minimize()       {}
func (m *windowManager) ToggleMaximize() {}
func (m *windowManager) IsOpen() bool    { return false }
