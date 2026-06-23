//go:build !windows

package main

import "log/slog"

// hasUI reports whether a native UI window can be opened. WebView2 is
// Windows-only, so non-Windows builds run headless.
func hasUI() bool { return false }

func runUI(log *slog.Logger, title, url string) {}
