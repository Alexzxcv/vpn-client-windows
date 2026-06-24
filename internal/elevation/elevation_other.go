//go:build !windows

package elevation

// IsElevated is a stub on non-Windows platforms (CI/linters). TUN mode is a
// Windows-only feature; off Windows we report true so unit tests of the app
// layer can exercise the TUN code path without an admin check.
func IsElevated() bool {
	return true
}
