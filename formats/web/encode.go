package web

import (
	"fmt"
	"image"
	"io"
	"math/big"
	"strings"

	"golang.org/x/image/draw"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/xmp"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/types"
)

func (c codec) pickFormat(meta types.Metadata, opts *formats.EncodeOptions) (string, string, error) {
	needs := capabilities{
		transparency: meta.HasTransparency && !opts.IgnoreTransparency(),
		lossless:     opts.WantsLossless(),
	}

	var format string
	for _, f := range c.formats {
		if meetsNeeds(f, needs) {
			format = f
			break
		}
	}
	if format == "" {
		return "", "", fmt.Errorf(
			"none of the configured formats (%s) meet the needs of this postcard & options (%s)",
			strings.Join(c.formats, ", "),
			needs.String(),
		)
	}

	return format, "image/" + format, nil
}

func (c codec) Encode(pc types.Postcard, opts *formats.EncodeOptions) ([]formats.FileWriter, error) {
	format, mimetype, err := c.pickFormat(pc.Meta, opts)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("%s.postcard.%s", pc.Name, format)

	writer := func(w io.Writer) error {
		frontSize, finalSize := formats.DetermineSize(opts, pc.Front, pc.Back)

		pc.Meta.Physical.FrontDimensions.PxWidth = finalSize.Dx()
		pc.Meta.Physical.FrontDimensions.PxHeight = finalSize.Dy()
		outputImageSize := pc.Meta.Physical.FrontDimensions

		combinedSize := finalSize
		if pc.Back != nil {
			combinedSize.Max.Y *= 2
			outputImageSize.PxHeight *= 2
			if outputImageSize.HasPhysical() {
				outputImageSize.CmHeight = (&big.Rat{}).Mul(outputImageSize.CmHeight, big.NewRat(2, 1))
			}
		}

		combinedImg := image.NewRGBA(combinedSize)
		// Fill the backdrop with the card colour if we're ignoring transparency
		if opts.IgnoreTransparency() {
			bg := &image.Uniform{pc.Meta.Physical.GetCardColor()}
			draw.Draw(combinedImg, combinedImg.Bounds(), bg, image.Point{}, draw.Src)
		}
		// Add the front of the postcard
		draw.CatmullRom.Scale(combinedImg, finalSize, pc.Front, frontSize, draw.Over, nil)

		if pc.Back != nil {
			backImg, backSize := rotateForWeb(pc.Back, pc.Meta.Flip)
			lowerSize := image.Rect(0, finalSize.Max.Y, finalSize.Max.X, finalSize.Max.Y*2)
			draw.CatmullRom.Scale(combinedImg, lowerSize, backImg, backSize, draw.Over, nil)
		}

		xmpData, err := xmp.MetadataToXMP(pc.Meta, &outputImageSize)
		if err != nil {
			return fmt.Errorf("couldn't generate XMP metadata for postcard: %w", err)
		}

		switch format {
		case "webp":
			err = images.WriteWebP(w, combinedImg, xmpData, opts.Archival, pc.Meta.HasTransparency)
		case "png":
			err = images.WritePNG(w, combinedImg, xmpData, opts.Archival)
		case "jpeg":
			err = images.WriteJPEG(w, combinedImg, xmpData)
		default:
			err = fmt.Errorf("unsupported output image format: %s", format)
		}

		return err
	}

	return []formats.FileWriter{formats.NewFileWriter(name, mimetype, writer)}, nil
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
