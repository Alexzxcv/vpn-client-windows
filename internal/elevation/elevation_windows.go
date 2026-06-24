//go:build windows

// Package elevation reports whether the current process runs with administrator
// (elevated) privileges. TUN mode needs elevation to create the virtual network
// interface and rewrite the routing table.
package elevation

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// IsElevated reports whether the current process token is elevated (running as
// administrator). It queries the token elevation information; on any error it
// conservatively returns false.
func IsElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_QUERY,
		&token,
	); err != nil {
		return false
	}
	defer token.Close()

	var elevation uint32
	var outLen uint32
	err := windows.GetTokenInformation(
		token,
		windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elevation)),
		uint32(unsafe.Sizeof(elevation)),
		&outLen,
	)
	if err != nil {
		return false
	}
	return elevation != 0
}
