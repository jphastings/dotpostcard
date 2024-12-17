//go:build !wasm
// +build !wasm

package images

import (
	"bytes"
	"image"
	"io"

	"git.sr.ht/~jackmordaunt/go-libwebp/webp"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

func WriteWebP(w io.Writer, img image.Image, xmpData []byte, archival, hasAlpha bool) error {
	var webpOpts []webp.EncodeOption
	if archival {
		webpOpts = []webp.EncodeOption{webp.Lossless()}
	} else {
		webpOpts = []webp.EncodeOption{webp.Quality(70)}
	}

	webpData := new(bytes.Buffer)
	if err := webp.Encode(webpData, img, webpOpts...); err != nil {
		return err
	}

	return xmpinject.XMPintoWebP(w, webpData.Bytes(), xmpData, img.Bounds(), hasAlpha)
}

var decodeWebP = webp.Decode
