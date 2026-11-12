//go:build !wasm
// +build !wasm

package images

import (
	"bytes"
	"image"
	"io"

	"git.sr.ht/~jackmordaunt/go-libwebp/v2/webp"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

const (
	efficientWebPQuality = 0.8
	detailWebPQuality    = 0.9
)

func WriteWebP(w io.Writer, img image.Image, xmpData []byte, quality formats.Quality, hasAlpha bool) error {
	var webpOpts []webp.EncodeOption
	switch quality {
	case formats.Archive:
		webpOpts = []webp.EncodeOption{webp.Lossless()}
	case formats.Detail:
		webpOpts = []webp.EncodeOption{webp.Quality(detailWebPQuality)}
	default:
		webpOpts = []webp.EncodeOption{webp.Quality(efficientWebPQuality)}
	}

	webpData := new(bytes.Buffer)
	if err := webp.Encode(webpData, img, webpOpts...); err != nil {
		return err
	}

	return xmpinject.XMPintoWebP(w, webpData.Bytes(), xmpData, img.Bounds(), hasAlpha)
}

var decodeWebP = webp.Decode
