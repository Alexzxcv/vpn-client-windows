// Package killswitch blocks all network egress while the VPN is connected so a
// tunnel drop cannot leak traffic outside the tunnel.
//
// On Windows it installs Windows Firewall rules (a dedicated rule group that is
// easy to remove and to clean up after a crash). On other platforms it is a
// no-op so the rest of the code compiles for CI.
//
// Crash-safe: all rules live under a single named group; CleanupStale removes
// any rules left from a previous run, and Disengage is idempotent.
package killswitch

import (
	"log/slog"
	"sync"
)

// Params describes the allow-list for the kill-switch: the VPN node (so the
// tunnel itself can connect) and the tunnel interface.
type Params struct {
	// ServerHost is the VPN node host (IP or hostname). Traffic to it on
	// ServerPort is allowed so the tunnel can establish.
	ServerHost string
	// ServerPort is the VPN node port.
	ServerPort int
	// TunInterface is the TUN interface name (sing-box). Traffic over it is
	// allowed (that is the tunnel).
	TunInterface string
	// AllowLAN permits private/LAN ranges and loopback (DHCP, local devices).
	AllowLAN bool
}

// Guard owns the kill-switch lifecycle. Safe for concurrent use.
type Guard struct {
	log *slog.Logger

	mu      sync.Mutex
	engaged bool
}

// New creates a Guard. log may be nil.
func New(log *slog.Logger) *Guard {
	if log == nil {
		log = slog.Default()
	}
	return &Guard{log: log}
}

// Engaged reports whether the kill-switch is currently active.
func (g *Guard) Engaged() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.engaged
}
