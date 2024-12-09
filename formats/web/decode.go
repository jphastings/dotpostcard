package web

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"io"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/xmp"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
	"github.com/jphastings/dotpostcard/types"
)

func (b bundle) Decode(_ *formats.DecodeOptions) (types.Postcard, error) {
	var dataCopy bytes.Buffer
	t := io.TeeReader(b, &dataCopy)

	img, format, err := image.Decode(t)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("unable to decode image: %w", err)
	}

	var xmpData []byte
	switch format {
	case "webp":
		xmpData, err = xmpinject.XMPfromWebP(dataCopy.Bytes())
	case "jpeg":
		xmpData, err = xmpinject.XMPfromJPEG(dataCopy.Bytes())
	default:
		err = fmt.Errorf("no XMP extractor for %s format", format)
	}
	if err != nil {
		return types.Postcard{}, fmt.Errorf("couldn't extract XMP metadata: %w", err)
	}

	pc, err := xmp.BundleFromBytes(xmpData, b.refPath).Decode(nil)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("didn't contain postcard metadata: %w", err)
	}
	pc.Name = b.name

	if pc.Meta.Flip == types.FlipNone {
		pc.Front = img
		return pc, nil
	}

	bounds := img.Bounds()
	sideW := bounds.Dx()
	sideH := bounds.Dy() / 2

	frontBounds := image.Rectangle{
		Min: image.Point{0, 0},
		Max: image.Point{sideW, sideH},
	}

	front := image.NewRGBA(image.Rect(0, 0, sideW, sideH))
	pc.Front = front
	draw.Draw(front, frontBounds, img, image.Point{}, draw.Src)

	back := image.NewRGBA(image.Rect(0, 0, sideW, sideH))
	draw.Draw(back, frontBounds, img, image.Point{0, sideH}, draw.Src)

	if pc.Meta.Flip == types.FlipLeftHand {
		// Use the opposite flip to return to the right orientation
		pc.Back, _ = rotateForWeb(back, types.FlipRightHand)
	} else if pc.Meta.Flip == types.FlipRightHand {
		// Use the opposite flip to return to the right orientation
		pc.Back, _ = rotateForWeb(back, types.FlipLeftHand)
	} else {
		pc.Back = back
	}

	return pc, nil
}
