//go:build wasm
// +build wasm

package web

import (
	"image"
	"io"
)

// This is a trivial substitute for the webp enabled version that requires hackery.
func determineFormat(r io.Reader) (string, error) {
	_, format, err := image.Decode(r)
	return format, err
}
