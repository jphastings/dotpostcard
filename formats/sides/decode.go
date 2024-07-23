package sides

import (
	"bytes"
	"fmt"
	"image"
	"io"

	"github.com/jphastings/postcards/internal/resolution"
	"github.com/jphastings/postcards/types"
)

func (b bundle) Decode() (types.Postcard, error) {
	pc, err := b.metaBundle.Decode()
	if err != nil {
		return types.Postcard{}, err
	}

	pc.Name = b.name

	img, size, err := decodeImage(b.frontFile)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("couldn't decode postcard's front image: %w", err)
	}

	pc.Front = img
	pc.Meta.FrontDimensions = size

	if b.backFile != nil {
		img, size, err := decodeImage(b.backFile)
		if err != nil {
			return types.Postcard{}, fmt.Errorf("couldn't decode postcard's back image: %w", err)
		}

		if !size.SimilarPhysical(pc.Meta.FrontDimensions, pc.Meta.Flip) {
			return types.Postcard{}, fmt.Errorf("the front and back images are different physical sizes, are they of the same postcard?")
		}

		pc.Back = img
	}

	return pc, nil
}

func decodeImage(r io.Reader) (image.Image, types.Size, error) {
	var dataCopy bytes.Buffer
	t := io.TeeReader(r, &dataCopy)

	img, _, err := image.Decode(t)
	if err != nil {
		return nil, types.Size{}, err
	}
	bounds := img.Bounds()
	size := types.Size{
		PxWidth:  bounds.Dx(),
		PxHeight: bounds.Dy(),
	}

	xRes, yRes, err := resolution.Decode(dataCopy.Bytes())
	if err != nil {
		// Invalid physical dimensions just get ignored
		return img, size, nil
	}

	if xRes.Sign() != 0 && yRes.Sign() != 0 {
		size.SetResolution(xRes, yRes)
	}

	return img, size, nil
}
