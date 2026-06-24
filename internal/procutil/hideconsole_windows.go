//go:build windows

package procutil

import (
	"os/exec"
	"syscall"
)

// createNoWindow is the CREATE_NO_WINDOW process creation flag — the child
// console process (xray / sing-box) runs without popping up a console window.
const createNoWindow = 0x08000000

// HideConsole makes cmd start without a visible console window. Call it on the
// *exec.Cmd before Start. No-op on non-Windows.
func HideConsole(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
