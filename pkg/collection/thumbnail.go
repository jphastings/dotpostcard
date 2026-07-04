package collection

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"math"

	"golang.org/x/image/draw"
)

const (
	thumbnailMaxDimension = 512
	thumbnailQuality      = 80
)

// makeThumbnail scales front down (never up) so its largest dimension is at
// most thumbnailMaxDimension, and encodes it as a JPEG.
func makeThumbnail(front image.Image) ([]byte, error) {
	bounds := front.Bounds()
	dstW, dstH := bounds.Dx(), bounds.Dy()

	if maxDim := max(dstW, dstH); maxDim > thumbnailMaxDimension {
		scale := float64(thumbnailMaxDimension) / float64(maxDim)
		dstW = int(math.Round(float64(dstW) * scale))
		dstH = int(math.Round(float64(dstH) * scale))
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), front, bounds, draw.Src, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: thumbnailQuality}); err != nil {
		return nil, fmt.Errorf("encoding thumbnail: %w", err)
	}

	return buf.Bytes(), nil
}
