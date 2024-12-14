package web

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"io"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/xmp"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
	"github.com/jphastings/dotpostcard/types"
)

func (b bundle) Decode(decOpts formats.DecodeOptions) (types.Postcard, error) {
	defer b.Close()

	var dataCopy bytes.Buffer
	t := io.TeeReader(b, &dataCopy)

	format, err := determineFormat(t)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("unable to determine image file format: %w", err)
	}

	var imgDecoder func(io.Reader) (image.Image, error)
	var xmpDecoder func([]byte) ([]byte, error)
	switch format {
	case "webp":
		// This function is defined in multiple files so we can keep the WebP package out of WASM builds.
		imgDecoder = images.ReadWebP
		xmpDecoder = xmpinject.XMPfromWebP
	case "jpeg":
		imgDecoder = jpeg.Decode
		xmpDecoder = xmpinject.XMPfromJPEG
	default:
		return types.Postcard{}, fmt.Errorf("no XMP extractor for %s format", format)
	}

	img, err := imgDecoder(t)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("unable to decode image: %w", err)
	}

	xmpData, err := xmpDecoder(dataCopy.Bytes())
	if err != nil {
		return types.Postcard{}, fmt.Errorf("couldn't extract XMP metadata: %w", err)
	}

	if len(xmpData) == 0 {
		return types.Postcard{}, fmt.Errorf("image didn't contain XMP metadata, it's not readable as a postcard")
	}

	pc, err := xmp.BundleFromBytes(xmpData, b.refPath).Decode(decOpts)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("image didn't contain postcard metadata: %w", err)
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
