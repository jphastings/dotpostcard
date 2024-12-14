//go:build !wasm
// +build !wasm

package web

import (
	"image"
	"io"

	"git.sr.ht/~jackmordaunt/go-libwebp/webp"
)

// the goalang.org/x/image/webp decoder bugs out on Alpha layers; it gets Registered with the image package
// when jackmordaunt's webp parser is loaded, but sits at the top of the pack — so using image.Decode to
// decode automatically won't work (it uses golang.org version, which breaks) and using image.DecodeConfig
// to get the format also fails (as that also uses the golang.org version, which breaks)
// This slightly hacky approach means we can manually use only jackmordaunt's version
func determineFormat(r io.Reader) (string, error) {
	_, err := webp.DecodeConfig(r)
	if err == nil {
		return "webp", nil
	}

	_, format, err := image.Decode(r)
	return format, err
}
