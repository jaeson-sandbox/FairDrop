//go:build darwin && !cgo

package main

// Cross-compilation only. A native Wails macOS application requires cgo;
// this path cannot claim to have checked Foundation's temporary directory.
func nativeLockTempDir() string { return "" }
