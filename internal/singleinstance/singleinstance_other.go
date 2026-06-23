//go:build !windows

package singleinstance

// Lock — заглушка на не-Windows платформах (клиент целевой под Windows).
type Lock struct{}

// Acquire вне Windows всегда успешен (single-instance не enforced).
func Acquire(name string) (*Lock, error) { _ = name; return &Lock{}, nil }

// Release вне Windows ничего не делает.
func (l *Lock) Release() {}
