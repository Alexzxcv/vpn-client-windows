package app

import (
	"fmt"
	"net/url"
)

// OpenExternal opens an http(s) URL in the user's default browser. The scheme is
// validated so the UI cannot hand an arbitrary shell target (file:, custom
// protocol handlers, …) to the OS through this path.
func (a *App) OpenExternal(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("open external: parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("open external: unsupported scheme %q", u.Scheme)
	}
	return openURL(raw)
}
