// Package procutil provides cross-platform helpers for managing the engine
// subprocesses (xray / sing-box). Its main job is crash-recovery: at startup we
// may find orphaned engine processes left behind by a previous run that died
// without killing its children (on Windows a parent exit does not cascade to
// children). KillOrphansByPath terminates ONLY processes whose executable image
// is exactly the binary we ship (bin/xray.exe, bin/sing-box.exe) so we never
// touch an unrelated process that happens to share the name.
package procutil

// KillOrphansByPath terminates every running process whose executable image
// path equals exePath (case-insensitively on Windows), except the current
// process. It returns the number of processes terminated. exePath should be an
// absolute path to one of our bundled engine binaries; passing a bare name is a
// no-op-safe miss. The non-Windows build is a no-op (returns 0, nil).
func KillOrphansByPath(exePath string) (int, error) {
	return killOrphansByPath(exePath)
}
