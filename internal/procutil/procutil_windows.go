//go:build windows

package procutil

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// killOrphansByPath enumerates running processes via a toolhelp snapshot,
// resolves each PID's full image path and terminates those whose image equals
// exePath (case-insensitive). The current process is always skipped. Best
// effort: per-process errors (access denied, race on exit) are ignored so one
// stubborn process does not block cleaning the rest.
func killOrphansByPath(exePath string) (int, error) {
	if exePath == "" {
		return 0, nil
	}
	abs, err := filepath.Abs(exePath)
	if err != nil {
		abs = exePath
	}
	want := strings.ToLower(filepath.Clean(abs))
	wantBase := strings.ToLower(filepath.Base(want))

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, fmt.Errorf("procutil: snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	self := windows.GetCurrentProcessId()

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return 0, fmt.Errorf("procutil: first process: %w", err)
	}

	killed := 0
	for {
		pid := entry.ProcessID
		// Fast path: skip self and entries whose exe base name does not match
		// (avoids the expensive full-path resolve for unrelated processes).
		if pid != self && pid != 0 &&
			strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), wantBase) {
			if full, ok := imagePath(pid); ok && strings.ToLower(filepath.Clean(full)) == want {
				if terminate(pid) {
					killed++
				}
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break // ERROR_NO_MORE_FILES at the end of the list
		}
	}
	return killed, nil
}

// imagePath returns the full executable path for a PID, or ok=false if it could
// not be resolved (process gone, access denied, system process).
func imagePath(pid uint32) (string, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil || h == 0 {
		return "", false
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", false
	}
	return windows.UTF16ToString(buf[:size]), true
}

// terminate force-kills a PID. Returns true on success.
func terminate(pid uint32) bool {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil || h == 0 {
		return false
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1) == nil
}
