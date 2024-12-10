package web

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"io"

	"git.sr.ht/~jackmordaunt/go-libwebp/webp"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/xmp"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
	"github.com/jphastings/dotpostcard/types"
)

func (b bundle) Decode(decOpts formats.DecodeOptions) (types.Postcard, error) {
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
		imgDecoder = webp.Decode
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
