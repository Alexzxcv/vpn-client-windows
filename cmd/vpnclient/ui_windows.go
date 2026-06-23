//go:build windows

package main

import (
	"log/slog"

	"github.com/jchv/go-webview2"
)

// hasUI reports whether a native UI window can be opened.
func hasUI() bool { return true }

// runUI opens a WebView2 window pointed at url and blocks until it is closed.
func runUI(log *slog.Logger, title, url string) {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug: false,
		WindowOptions: webview2.WindowOptions{
			Title:  title,
			Width:  420,
			Height: 720,
			IconId: 0,
			Center: true,
		},
	})
	if w == nil {
		log.Error("failed to create WebView2 window (is the WebView2 runtime installed?)")
		return
	}
	defer w.Destroy()
	w.Navigate(url)
	w.Run()
}
