package collection

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"

	"golang.org/x/image/draw"
)

const (
	thumbnailMaxDimension = 512
	thumbnailQuality      = 80
)

// makeThumbnail scales front down (never up) so its largest dimension is at
// most thumbnailMaxDimension. Cards with meaningful alpha are encoded as PNG
// so the thumbnail keeps its transparency; fully opaque cards are encoded as
// JPEG, which is smaller. hasTransparency is the decoder's best guess
// (types.Metadata.HasTransparency); since that's derived from a single
// sampled pixel, it's trusted when true but double-checked with a full scan
// of front when false, so a transparent image is never misdetected as opaque.
func makeThumbnail(front image.Image, hasTransparency bool) ([]byte, error) {
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
	if hasTransparency || hasMeaningfulAlpha(front) {
		if err := png.Encode(&buf, dst); err != nil {
			return nil, fmt.Errorf("encoding thumbnail: %w", err)
		}
		return buf.Bytes(), nil
	}

	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: thumbnailQuality}); err != nil {
		return nil, fmt.Errorf("encoding thumbnail: %w", err)
	}
	return buf.Bytes(), nil
}

// hasMeaningfulAlpha reports whether img has any non-fully-opaque pixel.
func hasMeaningfulAlpha(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0xffff {
				return true
			}
		}
	}
	return false
}
