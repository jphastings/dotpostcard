package resolution

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

// tiffWithOrientations builds a little-endian TIFF whose IFD0 declares the
// first orientation and, when given, whose IFD1 (the thumbnail) the second.
func tiffWithOrientations(ifd0 uint16, ifd1 ...uint16) []byte {
	le := binary.LittleEndian
	ifd := func(orientation uint16, next uint32) []byte {
		b := make([]byte, 2+12+4)
		le.PutUint16(b[0:], 1)
		le.PutUint16(b[2:], 0x0112)
		le.PutUint16(b[4:], 3)
		le.PutUint32(b[6:], 1)
		le.PutUint16(b[10:], orientation)
		le.PutUint32(b[14:], next)
		return b
	}
	blob := []byte{'I', 'I', '*', 0, 8, 0, 0, 0}
	if len(ifd1) == 0 {
		return append(blob, ifd(ifd0, 0)...)
	}
	blob = append(blob, ifd(ifd0, 8+18)...)
	return append(blob, ifd(ifd1[0], 0)...)
}

func jpegWithExif(t *testing.T, exif []byte) []byte {
	var buf bytes.Buffer
	assert.NoError(t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil))
	img := buf.Bytes()
	app1 := append([]byte{0xFF, 0xE1, 0, 0}, []byte("Exif\x00\x00")...)
	app1 = append(app1, exif...)
	binary.BigEndian.PutUint16(app1[2:], uint16(len(app1)-2))
	return append(append(img[:2:2], app1...), img[2:]...)
}

func TestOrientation(t *testing.T) {
	assert.Equal(t, 6, Orientation(jpegWithExif(t, tiffWithOrientations(6))))
	assert.Equal(t, 6, Orientation(jpegWithExif(t, tiffWithOrientations(6, 3))), "thumbnail's orientation must not win")
	assert.Equal(t, 1, Orientation(jpegWithExif(t, tiffWithOrientations(0))))
	assert.Equal(t, 1, Orientation(jpegWithExif(t, tiffWithOrientations(9))))

	var plain bytes.Buffer
	assert.NoError(t, jpeg.Encode(&plain, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil))
	assert.Equal(t, 1, Orientation(plain.Bytes()))
	assert.Equal(t, 1, Orientation([]byte("not an image at all")))
}

// tiffWithResolutions builds a little-endian TIFF whose IFD0 declares res dpi
// and whose thumbnail IFD1 declares thumbRes dpi (both in inches).
func tiffWithResolutions(res, thumbRes, xDenominator uint32) []byte {
	le := binary.LittleEndian
	const ifdSize = 2 + 3*12 + 4
	ifd := func(start uint32, r, xDen, next uint32) []byte {
		data := start + ifdSize
		b := le.AppendUint16(nil, 3)
		for i, tag := range []uint16{0x011A, 0x011B} {
			b = le.AppendUint16(b, tag)
			b = le.AppendUint16(b, 5)
			b = le.AppendUint32(b, 1)
			b = le.AppendUint32(b, data+uint32(i)*8)
		}
		b = le.AppendUint16(b, 0x0128)
		b = le.AppendUint16(b, 3)
		b = le.AppendUint32(b, 1)
		b = le.AppendUint32(b, 2)
		b = le.AppendUint32(b, next)
		b = le.AppendUint32(b, r)
		b = le.AppendUint32(b, xDen)
		b = le.AppendUint32(b, r)
		return le.AppendUint32(b, 1)
	}
	blob := []byte{'I', 'I', '*', 0, 8, 0, 0, 0}
	blob = append(blob, ifd(8, res, xDenominator, 8+ifdSize+16)...)
	return append(blob, ifd(8+ifdSize+16, thumbRes, 1, 0)...)
}

func TestDecodeUsesMainImageResolutionNotThumbnail(t *testing.T) {
	x, y, err := Decode(jpegWithExif(t, tiffWithResolutions(300, 72, 1)))
	assert.NoError(t, err)
	assert.Equal(t, "300", new(big.Rat).Mul(x, big.NewRat(254, 100)).RatString())
	assert.Equal(t, "300", new(big.Rat).Mul(y, big.NewRat(254, 100)).RatString())
}
