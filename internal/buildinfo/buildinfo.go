// Package buildinfo carries the client build metadata. Version is meant to be
// overridden at build time via ldflags, e.g.:
//
//	go build -ldflags "-X github.com/Alexzxcv/vpn-client-windows/internal/buildinfo.Version=v0.2.0" ./cmd/vpnclient
//
// When not overridden it stays "dev" (treated as the lowest possible version by
// the updater, so a dev build never prompts to "downgrade").
package buildinfo

// Version is the current client version (a git tag like "v0.2.0", or "dev").
var Version = "dev"
