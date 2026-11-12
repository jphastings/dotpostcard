//go:build !darwin

package fastfoto

import "errors"

// OpenUSB connects to the scanner; only macOS is supported.
func OpenUSB() (Transport, error) {
	return nil, errors.New("scanning is only supported on macOS")
}
