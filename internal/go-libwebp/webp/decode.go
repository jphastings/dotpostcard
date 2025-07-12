package webp

import (
	"fmt"
	"image"
	"io"

	dynamic "git.sr.ht/~jackmordaunt/go-libwebp/lib/dynamic/webp"
	transpiled "git.sr.ht/~jackmordaunt/go-libwebp/lib/transpiled/webp"
	stdwebp "golang.org/x/image/webp"
)

// Decode reads a WEBP image from r and returns it as an image.Image.
func Decode(r io.Reader) (image.Image, error) {
	by, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("buffering data: %w", err)
	}
	if err := dynamic.Init(); err == nil {
		return dynamic.DecodeImpl(by)
	}
	return transpiled.DecodeImpl(by)
}

func decodeDynamic(by []byte) (image.Image, error) {
	if err := dynamic.Init(); err != nil {
		return nil, err
	}
	return dynamic.DecodeImpl(by)
}

func decodeTranspiled(by []byte) (image.Image, error) {
	return transpiled.DecodeImpl(by)
}

// DecodeConfig returns the color model and dimensions of a WEBP image without
// decoding the entire image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	return stdwebp.DecodeConfig(r)
}
