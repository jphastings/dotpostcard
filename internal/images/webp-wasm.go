//go:build wasm
// +build wasm

package images

import (
	"fmt"
	"image"
	"io"

	"golang.org/x/image/webp"
)

func WriteWebP(w io.Writer, img image.Image, xmpData []byte, archival, hasAlpha bool) error {
	return fmt.Errorf("writing webP images is not available on this platform")
}

var decodeWebP = webp.Decode
