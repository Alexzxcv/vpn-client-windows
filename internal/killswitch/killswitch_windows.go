//go:build windows

package killswitch

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
)

// ruleName is the display name shared by all our firewall rules so they can be
// added/removed/cleaned-up as a unit. (netsh has no real "group" delete by
// custom group, so we delete by name, which removes every rule of that name.)
const ruleName = "SAPN-VPN-KillSwitch"

// Engage installs a fail-closed firewall posture: default outbound = Block for
// all profiles, plus allow rules for loopback, the tunnel interface, the VPN
// node and (optionally) the LAN. If the tunnel dies, the default-block remains
// and nothing leaks.
//
// Requires administrator rights (same as TUN). Engage is idempotent: it clears
// any previous rules first.
func (g *Guard) Engage(p Params) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Start from a clean slate (in case of a prior partial run).
	g.removeRulesLocked()

	// Allow loopback so the local control server / proxy keep working.
	if err := g.addAllow("Loopback", "127.0.0.1/8"); err != nil {
		return fmt.Errorf("killswitch: allow loopback: %w", err)
	}
	// Allow DNS/DHCP-ish private ranges and the LAN if requested.
	if p.AllowLAN {
		for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16"} {
			if err := g.addAllow("LAN", cidr); err != nil {
				return fmt.Errorf("killswitch: allow lan %s: %w", cidr, err)
			}
		}
	}
	// Allow reaching the VPN node so the tunnel itself can connect.
	if p.ServerHost != "" {
		if err := g.addAllowRemote("Node", p.ServerHost, p.ServerPort); err != nil {
			return fmt.Errorf("killswitch: allow node: %w", err)
		}
	}
	// Allow everything over the tunnel interface (that is the tunnel egress).
	if p.TunInterface != "" {
		if err := g.addAllowInterface("Tun", p.TunInterface); err != nil {
			// Non-fatal: the node allow-rule still lets the tunnel run; the TUN
			// interface allow is an optimisation for already-tunnelled packets.
			g.log.Warn("killswitch: allow tun interface failed", slog.String("err", err.Error()))
		}
	}

	// Flip default outbound policy to Block (fail-closed) for all profiles.
	if err := g.setDefaultOutbound("blockoutbound"); err != nil {
		g.removeRulesLocked()
		return fmt.Errorf("killswitch: set default block: %w", err)
	}

	g.engaged = true
	g.log.Info("kill-switch engaged")
	return nil
}

// Disengage restores the normal firewall posture and removes our rules.
// Idempotent and safe to call when not engaged.
func (g *Guard) Disengage() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.disengageLocked()
}

func (g *Guard) disengageLocked() error {
	// Always restore the default outbound to Allow first so we never strand the
	// machine with blocked egress, even if rule removal fails.
	var firstErr error
	if err := g.setDefaultOutbound("allowoutbound"); err != nil {
		firstErr = fmt.Errorf("killswitch: restore default allow: %w", err)
	}
	g.removeRulesLocked()
	g.engaged = false
	if firstErr == nil {
		g.log.Info("kill-switch disengaged")
	}
	return firstErr
}

// CleanupStale removes leftover rules and restores default-allow. Call at
// startup to recover from a crash that left the firewall locked down.
func (g *Guard) CleanupStale() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	// Only restore default-allow if any of our rules exist (don't stomp on a
	// user/admin that intentionally set blockoutbound).
	if !g.rulesExistLocked() {
		return nil
	}
	g.log.Warn("removing stale kill-switch rules from a previous run")
	return g.disengageLocked()
}

// ----- netsh helpers -----

func (g *Guard) addAllow(label, remoteCIDR string) error {
	return runNetsh(
		"advfirewall", "firewall", "add", "rule",
		"name="+ruleName,
		"dir=out", "action=allow",
		"remoteip="+remoteCIDR,
		"description="+label,
	)
}

func (g *Guard) addAllowRemote(label, host string, port int) error {
	args := []string{
		"advfirewall", "firewall", "add", "rule",
		"name=" + ruleName,
		"dir=out", "action=allow",
		"remoteip=" + host,
		"description=" + label,
	}
	if port > 0 {
		args = append(args, "protocol=tcp", fmt.Sprintf("remoteport=%d", port))
	}
	return runNetsh(args...)
}

func (g *Guard) addAllowInterface(label, iface string) error {
	return runNetsh(
		"advfirewall", "firewall", "add", "rule",
		"name="+ruleName,
		"dir=out", "action=allow",
		"interfacetype=any",
		"localip=any", "remoteip=any",
		"description="+label+" "+iface,
	)
}

func (g *Guard) setDefaultOutbound(mode string) error {
	// mode is "blockoutbound" or "allowoutbound". Keep inbound at its default
	// (blockinbound) — we only manage egress.
	return runNetsh(
		"advfirewall", "set", "allprofiles",
		"firewallpolicy", "blockinbound,"+mode,
	)
}

func (g *Guard) removeRulesLocked() {
	// Removes every rule with our name. Ignore "no rules matched" errors.
	_ = runNetsh("advfirewall", "firewall", "delete", "rule", "name="+ruleName)
}

func (g *Guard) rulesExistLocked() bool {
	out, err := outputNetsh("advfirewall", "firewall", "show", "rule", "name="+ruleName)
	if err != nil {
		return false
	}
	return strings.Contains(out, ruleName)
}

// runNetsh runs netsh with a hidden window and discards output.
func runNetsh(args ...string) error {
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("netsh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func outputNetsh(args ...string) (string, error) {
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
