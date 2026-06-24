//go:build !windows

package killswitch

// On non-Windows platforms the kill-switch is a no-op (the client ships on
// Windows). These stubs let the package compile for CI/linting.

// Engage is a no-op.
func (g *Guard) Engage(p Params) error {
	g.mu.Lock()
	g.engaged = true
	g.mu.Unlock()
	return nil
}

// Disengage is a no-op.
func (g *Guard) Disengage() error {
	g.mu.Lock()
	g.engaged = false
	g.mu.Unlock()
	return nil
}

// CleanupStale is a no-op.
func (g *Guard) CleanupStale() error { return nil }
