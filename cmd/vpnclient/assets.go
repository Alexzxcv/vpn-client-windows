package main

import _ "embed"

// Embedded icons (ICO for Windows) so the single binary is self-contained.
var (
	// appIcon is the SAPN brand mark (multi-size 16/32/48/256), used for the
	// window and the system tray. Generated from the landing favicon monogram.
	//go:embed assets/app.ico
	appIcon []byte

	//go:embed assets/tray_connected.ico
	iconConnected []byte
	//go:embed assets/tray_disconnected.ico
	iconDisconnected []byte
)
