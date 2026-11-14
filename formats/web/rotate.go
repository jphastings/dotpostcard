package web

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/types"
)

// A re-encoded JPEG may be this much bigger than the pixel data of the original.
const jpegSizeSlack = 0.05

type subImager interface {
	SubImage(image.Rectangle) image.Image
}

// Rotate turns the front and back of a single-file web postcard clockwise by the given quarter turns
// (negative for anticlockwise), returning the new file and the flip it implies. PNG and lossless WebP
// keep their pixels exactly; JPEG is re-encoded at the highest quality that keeps its size; lossy WebP at detail quality.
func Rotate(data []byte, frontTurns, backTurns int) ([]byte, types.Flip, error) {
	format := strings.TrimPrefix(http.DetectContentType(data), "image/")
	if format != "jpeg" && format != "png" && format != "webp" {
		return nil, "", fmt.Errorf("can only rotate JPEG, PNG or WebP postcards, not %s", format)
	}

	pc, img, srcXMP, err := decodeParts(bundle{ReadCloser: io.NopCloser(bytes.NewReader(data))}, formats.DecodeOptions{})
	if err != nil {
		return nil, "", err
	}

	oldFlip := pc.Meta.Flip
	if oldFlip == types.FlipNone && backTurns%4 != 0 {
		return nil, "", fmt.Errorf("this postcard has no back to rotate")
	}

	b := img.Bounds()
	top := img
	var bottom image.Image
	if oldFlip != types.FlipNone {
		mid := b.Min.Y + b.Dy()/2
		top = img.(subImager).SubImage(image.Rect(b.Min.X, b.Min.Y, b.Max.X, mid))
		bottom = img.(subImager).SubImage(image.Rect(b.Min.X, mid, b.Max.X, mid+b.Dy()/2))
	}

	front := images.Turn(top, frontTurns)
	pc.Front = front
	combined := front
	if bottom != nil {
		pc.Back = images.Turn(bottom, backTurns-webTurns(oldFlip))
	}
	pc.Meta.Turn(frontTurns, backTurns)
	if pc.Back != nil {
		packedBack := images.Turn(pc.Back, webTurns(pc.Meta.Flip))
		if packedBack.Bounds() != front.Bounds() {
			return nil, "", fmt.Errorf("the rotated sides no longer fit together (%v and %v)", front.Bounds().Size(), packedBack.Bounds().Size())
		}
		combined = image.NewNRGBA(image.Rect(0, 0, packedBack.Rect.Dx(), 2*packedBack.Rect.Dy()))
		copy(combined.Pix, front.Pix)
		copy(combined.Pix[len(front.Pix):], packedBack.Pix)
	}
	pc.Meta.HasTransparency = hasAnyAlpha(combined)

	opts := &formats.EncodeOptions{Quality: formats.Archive}
	jpegQuality := 0
	switch format {
	case "webp":
		if !images.IsLosslessWebP(data) {
			opts.Quality = formats.Detail
		}
	case "jpeg":
		bg := pc.Meta.Physical.GetCardColor()
		budget := int(float64(len(data)-len(srcXMP)) * (1 + jpegSizeSlack))
		jpegQuality = images.JPEGQualityForSize(combined, bg, budget)
	}

	var out bytes.Buffer
	if err := writeImage(&out, format, pc, combined, opts, jpegQuality); err != nil {
		return nil, "", err
	}
	return out.Bytes(), pc.Meta.Flip, nil
}

func hasAnyAlpha(img *image.NRGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 255 {
			return true
		}
	}
	return false
}
