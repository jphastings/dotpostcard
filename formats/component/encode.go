package component

import (
	"fmt"
	"image"
	"io"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/types"
	"golang.org/x/image/draw"
)

// The structure information is stored in the internal/types/postcard.go file, because Go.
func (c codec) Encode(pc types.Postcard, opts *formats.EncodeOptions) ([]formats.FileWriter, error) {
	_, finalSize := formats.DetermineSize(opts, pc.Front, pc.Back)

	encImg := func(side image.Image) func(io.Writer) error {
		return func(w io.Writer) error {
			if opts != nil && opts.Archival {
				return images.WriteWebP(w, side, nil, true, pc.Meta.HasTransparency)
			}

			startSize := side.Bounds()
			startW := startSize.Dx()
			startH := startSize.Dy()
			finalW := finalSize.Dx()
			finalH := finalSize.Dy()

			// Swap the width and height if this side is the opposite orientation to the 'finalSize' (the front)
			if (finalW > finalH) != (startW > startH) {
				finalW = finalSize.Dy()
				finalH = finalSize.Dx()
			}

			if finalW < startW || finalH < startH {
				resizedSize := image.Rect(0, 0, finalW, finalH)
				resizedImg := image.NewRGBA(resizedSize)
				draw.CatmullRom.Scale(resizedImg, resizedSize, side, startSize, draw.Src, nil)

				side = resizedImg
			}

			return images.WriteWebP(w, side, nil, opts.Archival, pc.Meta.HasTransparency)
		}
	}

	frontName := fmt.Sprintf("%s-front.webp", pc.Name)
	frontW := formats.NewFileWriter(frontName, "image/webp", encImg(pc.Front))

	backName := fmt.Sprintf("%s-back.webp", pc.Name)
	backW := formats.NewFileWriter(backName, "image/webp", encImg(pc.Back))

	return []formats.FileWriter{frontW, backW}, nil
}
