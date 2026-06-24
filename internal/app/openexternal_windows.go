//go:build windows

package app

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	modshell32        = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = modshell32.NewProc("ShellExecuteW")
)

// openURL opens raw in the default browser via ShellExecuteW(..., "open", ...).
// ShellExecuteW returns a value >32 on success.
func openURL(raw string) error {
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := syscall.UTF16PtrFromString(raw)
	if err != nil {
		return err
	}
	const swShowNormal = 1
	r, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(target)),
		0, 0,
		swShowNormal,
	)
	if r <= 32 {
		return fmt.Errorf("ShellExecuteW failed: code %d", r)
	}
	return nil
}
