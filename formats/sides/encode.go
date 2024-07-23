package sides

import (
	"fmt"
	"image"
	"io"

	"github.com/chai2010/webp"
	_ "github.com/chai2010/webp"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/types"
)

// The structure information is stored in the internal/types/postcard.go file, because Go.
func (c codec) Encode(pc types.Postcard, opts formats.EncodeOptions, errs chan<- error) []formats.FileWriter {
	encImg := func(side image.Image) func(io.Writer) error {
		return func(w io.Writer) error {
			var webpOpts *webp.Options
			if opts.Archival {
				webpOpts = &webp.Options{Lossless: true}
			} else {
				// TODO: Resize image
				webpOpts = &webp.Options{Lossless: false, Quality: 75}
			}

			return webp.Encode(w, side, webpOpts)
		}
	}

	frontName := fmt.Sprintf("%s-front.webp", pc.Name)
	frontW := formats.NewFileWriter(frontName, encImg(pc.Front), errs)

	backName := fmt.Sprintf("%s-back.webp", pc.Name)
	backW := formats.NewFileWriter(backName, encImg(pc.Back), errs)

	return []formats.FileWriter{frontW, backW}
}
