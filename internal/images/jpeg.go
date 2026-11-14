package images

import (
	"image"
	"image/color"
	"io"
	"sort"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
)

func WriteJPEG(w io.Writer, combinedImg image.Image, xmpData []byte, bgColor color.RGBA, quality formats.Quality) error {
	jpegQuality := efficientJPEGQuality
	if quality == formats.Detail {
		jpegQuality = detailJPEGQuality
	}
	return WriteJPEGAt(w, combinedImg, xmpData, bgColor, jpegQuality)
}

// WriteJPEGAt is WriteJPEG with an explicit encoder quality (1–100).
func WriteJPEGAt(w io.Writer, combinedImg image.Image, xmpData []byte, bgColor color.RGBA, jpegQuality int) error {
	jpgData, err := encodeJPEG(combinedImg, bgColor, jpegQuality)
	if err != nil {
		return err
	}
	return xmpinject.XMPintoJPEG(w, jpgData, xmpData)
}

// JPEGQualityForSize gives the highest JPEG quality (1–100) whose pixel data, flattened onto bg,
// encodes within maxBytes. Quality 1 is returned if nothing fits.
func JPEGQualityForSize(img image.Image, bg color.RGBA, maxBytes int) int {
	// ponytail: about 7 full-size encodes; seed the search from the source's quantisation tables if it gets slow
	i := sort.Search(100, func(i int) bool {
		data, err := encodeJPEG(img, bg, 100-i)
		return err == nil && len(data) <= maxBytes
	})
	return max(1, 100-i)
}
