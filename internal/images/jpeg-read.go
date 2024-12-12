package images

import (
	"image"
	"image/jpeg"
	"io"
)

func ReadJPEG(r io.Reader) (image.Image, error) {
	return jpeg.Decode(r)
}
