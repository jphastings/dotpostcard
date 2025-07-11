package component

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"git.sr.ht/~sbinet/gg"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/resolution"
	"github.com/jphastings/dotpostcard/types"
	_ "github.com/sunshineplan/tiff"
	"golang.org/x/image/draw"
)

func (b bundle) Decode(opts formats.DecodeOptions) (types.Postcard, error) {
	if b.frontFile != nil {
		defer b.frontFile.Close()
	}
	if b.backFile != nil {
		defer b.backFile.Close()
	}

	pc, err := b.metaBundle.Decode(opts)
	if err != nil {
		return types.Postcard{}, err
	}
	forcedSize := pc.Meta.Physical.FrontDimensions

	pc.Name = b.name

	img, frontSize, hasTransparency, err := decodeImage(b.frontFile, opts)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("couldn't decode postcard's front image: %w", err)
	}
	pc.Meta.Physical.FrontDimensions = frontSize
	pc.Meta.HasTransparency = hasTransparency

	pc.Front, pc.Meta.Front.Secrets, err = hideSecrets(img, pc.Meta.Front.Secrets)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("couldn't hide secrets on front: %w", err)
	}

	if b.backFile == nil {
		pc.Meta.Flip = types.FlipNone
	} else {
		img, backSize, hasTransparencyBack, err := decodeImage(b.backFile, opts)
		if err != nil {
			return types.Postcard{}, fmt.Errorf("couldn't decode postcard's back image: %w", err)
		}
		// With Heterorientation cards it's possible one corner has transparency and the other doesn't
		pc.Meta.HasTransparency = pc.Meta.HasTransparency || hasTransparencyBack

		if !backSize.SimilarPhysical(frontSize, pc.Meta.Flip) {
			return types.Postcard{}, fmt.Errorf("the front and back images are different physical sizes (%v, %v), are they of the same postcard?", frontSize, backSize)
		}

		if err := types.CheckFlip(frontSize, backSize, pc.Meta.Flip); err != nil {
			return types.Postcard{}, err
		}

		pc.Back, pc.Meta.Back.Secrets, err = hideSecrets(img, pc.Meta.Back.Secrets)
		if err != nil {
			return types.Postcard{}, fmt.Errorf("couldn't hide secrets on back: %w", err)
		}
	}

	if forcedSize.HasPhysical() {
		pc.Meta.Physical.FrontDimensions.CmWidth = forcedSize.CmWidth
		pc.Meta.Physical.FrontDimensions.CmHeight = forcedSize.CmHeight
	}

	if err := pc.Validate(); err != nil {
		return types.Postcard{}, err
	}

	return pc, nil
}

func decodeImage(r io.Reader, decOpts formats.DecodeOptions) (image.Image, types.Size, bool, error) {
	var dataCopy bytes.Buffer
	t := io.TeeReader(r, &dataCopy)

	img, _, err := image.Decode(t)
	if err != nil {
		return nil, types.Size{}, false, err
	}
	bounds := img.Bounds()
	size := types.Size{
		PxWidth:  bounds.Dx(),
		PxHeight: bounds.Dy(),
	}

	_, _, _, a := img.At(0, 0).RGBA()
	hasTransparency := (a != 65535) && !decOpts.IgnoreTransparency

	if decOpts.RemoveBorder && !hasTransparency {
		img, err = removeBorder(img)
		if err != nil {
			return nil, types.Size{}, hasTransparency, err
		}
		hasTransparency = true
	}

	xRes, yRes, err := resolution.Decode(dataCopy.Bytes())
	if err != nil {
		// Invalid physical dimensions just get ignored
		return img, size, hasTransparency, nil
	}

	if xRes != nil && yRes != nil && xRes.Sign() != 0 && yRes.Sign() != 0 {
		size.SetResolution(xRes, yRes)
	}

	return img, size, hasTransparency, nil
}

func hideSecrets(img image.Image, secrets []types.Polygon) (image.Image, []types.Polygon, error) {
	noSecrets := true
	for _, secret := range secrets {
		if !secret.Prehidden {
			noSecrets = false
			break
		}
	}
	if noSecrets {
		return img, secrets, nil
	}

	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	overlay := image.NewRGBA(img.Bounds())
	draw.Copy(overlay, image.Point{}, img, img.Bounds(), draw.Over, nil)

	for i, poly := range secrets {
		if poly.Prehidden {
			continue
		}

		dc := gg.NewContext(w, h)

		x, y := poly.Points[0].ToPixels(w, h)
		bounds := image.Rect(int(x), int(y), int(x), int(y))

		dc.MoveTo(x, y)
		for _, p := range poly.Points[1:] {
			x, y := p.ToPixels(w, h)
			stretchBounds(&bounds, int(x), int(y))

			dc.LineTo(x, y)
		}

		dc.ClipPreserve()
		dc.DrawImage(img, 0, 0)

		dc.SetColor(modalColor(dc.Image(), bounds))
		dc.Fill()

		draw.Copy(overlay, image.Point{}, dc.Image(), img.Bounds(), draw.Over, nil)
		secrets[i].Prehidden = true
	}

	return overlay, secrets, nil
}

func stretchBounds(b *image.Rectangle, x, y int) {
	if x < b.Min.X {
		b.Min.X = int(x)
	} else if x > b.Max.X {
		b.Max.X = int(x)
	}

	if y < b.Min.Y {
		b.Min.Y = int(y)
	} else if y > b.Max.Y {
		b.Max.Y = int(y)
	}
}

func modalColor(img image.Image, within image.Rectangle) color.Color {
	counter := make(map[color.Color]uint)
	modal := color.Color(color.Transparent)
	max := uint(0)

	for y := within.Min.Y; y <= within.Max.Y; y++ {
		for x := within.Min.X; x <= within.Max.X; x++ {
			c := img.At(x, y)
			_, _, _, a := c.RGBA()
			if a < 65535 {
				continue
			}

			val := counter[c] + 1
			counter[c] = val
			if val > max {
				modal = c
				max = val
			}
		}
	}

	return modal
}
