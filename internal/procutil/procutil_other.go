//go:build !windows

package procutil

// killOrphansByPath is a no-op on non-Windows builds. The client targets
// Windows; the stub keeps the package buildable for `go vet`/tests on dev boxes.
func killOrphansByPath(string) (int, error) { return 0, nil }
