//go:build !darwin

// Package inprogress marks a file as still being written, where the OS has a way to show that.
// Only macOS does.
package inprogress

// Mark does nothing on this OS.
func Mark(path string) {}

// SetFraction does nothing on this OS.
func SetFraction(path string, fraction float64) {}

// Clear does nothing on this OS.
func Clear(path string) {}
