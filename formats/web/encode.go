package web

import (
	"bytes"
	"fmt"
	"image"
	"io"

	"github.com/chai2010/webp"
	_ "github.com/chai2010/webp"
	"golang.org/x/image/draw"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/xmp"
	"github.com/jphastings/postcards/types"
)

func (c codec) Encode(pc types.Postcard, opts formats.EncodeOptions, errs chan<- error) []formats.FileWriter {
	name := fmt.Sprintf("%s.webp", pc.Name)

	writer := func(w io.Writer) error {
		frontSize, finalSize := formats.DetermineSize(opts, pc.Front, pc.Back)

		combinedSize := finalSize
		if pc.Back != nil {
			combinedSize.Max.Y *= 2
		}

		combinedImg := image.NewRGBA(combinedSize)
		draw.CatmullRom.Scale(combinedImg, finalSize, pc.Front, frontSize, draw.Src, nil)

		if pc.Back != nil {
			backImg, backSize := rotateForWeb(pc.Back, pc.Meta.Flip)
			lowerSize := image.Rect(0, finalSize.Max.Y, finalSize.Max.X, finalSize.Max.Y*2)
			draw.CatmullRom.Scale(combinedImg, lowerSize, backImg, backSize, draw.Src, nil)
		}

		xmp.Codec().Encode(pc, nil)

		xmpData, err := xmp.MetadataToXMP(pc.Meta)
		if err != nil {
			return err
		}

		var webpOpts *webp.Options
		if opts.Archival {
			webpOpts = &webp.Options{Lossless: true}
		} else {
			webpOpts = &webp.Options{Lossless: false, Quality: 75}
		}

		data := new(bytes.Buffer)
		if err := webp.Encode(data, combinedImg, webpOpts); err != nil {
			return err
		}

		dataBytes, err := webp.SetMetadata(data.Bytes(), xmpData, "XMP")
		if err != nil {
			return err
		}

		_, err = w.Write(dataBytes)
		return err
	}

	return []formats.FileWriter{formats.NewFileWriter(name, writer, errs)}
}

func rotateForWeb(img image.Image, flip types.Flip) (image.Image, image.Rectangle) {
	bounds := img.Bounds()
	rotatedBounds := image.Rect(0, 0, bounds.Dy(), bounds.Dx())
	rotated := image.NewRGBA(rotatedBounds)

	switch flip {
	case types.FlipNone, types.FlipBook, types.FlipCalendar:
		return img, bounds

	case types.FlipLeftHand:
		// Top left of the source should be bottom left of the output
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				rotated.Set(y, bounds.Max.X-x, img.At(x, y))
			}
		}

	case types.FlipRightHand:
		// Top left of the source should be top right of the output
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				rotated.Set(bounds.Max.Y-y, x, img.At(x, y))
			}
		}
	}

	return rotated, rotatedBounds
}