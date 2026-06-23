package main

import _ "embed"

// Tray icons (ICO for Windows). Embedded so the single binary is self-contained.
var (
	//go:embed assets/tray_connected.ico
	iconConnected []byte
	//go:embed assets/tray_disconnected.ico
	iconDisconnected []byte
)
