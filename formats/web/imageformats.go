package web

import (
	"bytes"
	"image"
	"image/png"
	"io"

	"git.sr.ht/~jackmordaunt/go-libwebp/webp"
	"github.com/gen2brain/jpegli"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

func writeWebP(w io.Writer, combinedImg image.Image, xmpData []byte, archival, hasAlpha bool) error {
	var webpOpts []webp.EncodeOption
	if archival {
		webpOpts = []webp.EncodeOption{webp.Lossless()}
	} else {
		webpOpts = []webp.EncodeOption{webp.Quality(70)}
	}

	webpData := new(bytes.Buffer)
	if err := webp.Encode(webpData, combinedImg, webpOpts...); err != nil {
		return err
	}

	return xmpinject.XMPintoWebP(w, webpData.Bytes(), xmpData, combinedImg.Bounds(), hasAlpha)
}

func writePNG(w io.Writer, combinedImg image.Image, xmpData []byte, _ bool) error {
	pngData := new(bytes.Buffer)
	if err := png.Encode(pngData, combinedImg); err != nil {
		return err
	}

	return xmpinject.XMPintoPNG(w, pngData.Bytes(), xmpData)
}

func writeJPG(w io.Writer, combinedImg image.Image, xmpData []byte) error {
	jpegliOpts := &jpegli.EncodingOptions{
		Quality:           70,
		ProgressiveLevel:  2,
		FancyDownsampling: true,
	}

	jpgData := new(bytes.Buffer)
	if err := jpegli.Encode(jpgData, combinedImg, jpegliOpts); err != nil {
		return err
	}

	return xmpinject.XMPintoJPEG(w, jpgData.Bytes(), xmpData)
}
