package component

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	red   = color.NRGBA{255, 0, 0, 255}
	blue  = color.NRGBA{0, 0, 255, 255}
	white = color.NRGBA{255, 255, 255, 255}
)

// exifBlob builds a little-endian TIFF declaring an orientation and, if
// xRes is non-zero, a resolution in pixels per cm.
func exifBlob(orientation uint16, xRes, yRes uint32) []byte {
	le := binary.LittleEndian
	type entry struct {
		tag, typ uint16
		count    uint32
		value    uint32
	}
	entries := []entry{{0x0112, 3, 1, uint32(orientation)}}
	var extra []byte
	if xRes != 0 {
		dataStart := uint32(8 + 2 + 4*12 + 4)
		entries = append(entries,
			entry{0x011A, 5, 1, dataStart},
			entry{0x011B, 5, 1, dataStart + 8},
			entry{0x0128, 3, 1, 3},
		)
		for _, r := range []uint32{xRes, yRes} {
			extra = le.AppendUint32(extra, r)
			extra = le.AppendUint32(extra, 1)
		}
	}
	blob := []byte{'I', 'I', '*', 0, 8, 0, 0, 0}
	blob = le.AppendUint16(blob, uint16(len(entries)))
	for _, e := range entries {
		blob = le.AppendUint16(blob, e.tag)
		blob = le.AppendUint16(blob, e.typ)
		blob = le.AppendUint32(blob, e.count)
		blob = le.AppendUint32(blob, e.value)
	}
	blob = le.AppendUint32(blob, 0)
	return append(blob, extra...)
}

// quadrantImage is red top-left, blue bottom-right and white elsewhere.
func quadrantImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case x < 100 && y < 100:
				img.SetNRGBA(x, y, red)
			case x >= w-100 && y >= h-100:
				img.SetNRGBA(x, y, blue)
			default:
				img.SetNRGBA(x, y, white)
			}
		}
	}
	return img
}

func jpegWithExif(t *testing.T, img image.Image, exif []byte) io.ReadCloser {
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}))
	data := buf.Bytes()
	app1 := append([]byte{0xFF, 0xE1, 0, 0}, []byte("Exif\x00\x00")...)
	app1 = append(app1, exif...)
	binary.BigEndian.PutUint16(app1[2:], uint16(len(app1)-2))
	return io.NopCloser(bytes.NewReader(append(append(data[:2:2], app1...), data[2:]...)))
}

func pngWithExif(t *testing.T, img image.Image, exif []byte) io.ReadCloser {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	data := buf.Bytes()
	const afterIHDR = 8 + 25
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(exif)))
	chunk = append(chunk, "eXIf"...)
	chunk = append(chunk, exif...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	return io.NopCloser(bytes.NewReader(append(append(data[:afterIHDR:afterIHDR], chunk...), data[afterIHDR:]...)))
}

func assertNear(t *testing.T, want color.NRGBA, img image.Image, x, y int) {
	t.Helper()
	got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	for _, d := range []int{int(got.R) - int(want.R), int(got.G) - int(want.G), int(got.B) - int(want.B)} {
		assert.InDelta(t, 0, d, 40, "pixel (%d,%d) is %v, want ~%v", x, y, got, want)
	}
}

func TestDecodeHonoursJPEGOrientation(t *testing.T) {
	front := jpegWithExif(t, quadrantImage(300, 200), exifBlob(6, 0, 0))

	pc, err := BundleFromReaders(types.Metadata{}, front).Decode(formats.DecodeOptions{})
	require.NoError(t, err)

	assert.Equal(t, image.Rect(0, 0, 200, 300), pc.Front.Bounds())
	assert.Equal(t, 200, pc.Meta.Physical.FrontDimensions.PxWidth)
	assert.Equal(t, 300, pc.Meta.Physical.FrontDimensions.PxHeight)
	assertNear(t, red, pc.Front, 149, 50)
	assertNear(t, blue, pc.Front, 49, 250)
}

func TestDecodeSwapsResolutionAxesWithOrientation(t *testing.T) {
	front := jpegWithExif(t, quadrantImage(300, 200), exifBlob(6, 50, 20))

	pc, err := BundleFromReaders(types.Metadata{}, front).Decode(formats.DecodeOptions{})
	require.NoError(t, err)

	size := pc.Meta.Physical.FrontDimensions
	assert.Equal(t, "10", size.CmWidth.RatString()) // 200px tall stored, at 20px/cm
	assert.Equal(t, "6", size.CmHeight.RatString()) // 300px wide stored, at 50px/cm
}

func TestDecodePairsRotatedFrontWithPortraitBack(t *testing.T) {
	front := jpegWithExif(t, quadrantImage(300, 200), exifBlob(6, 30, 30))
	back := jpegWithExif(t, quadrantImage(200, 300), exifBlob(1, 30, 30))

	_, err := BundleFromReaders(types.Metadata{Flip: types.FlipBook}, front, back).Decode(formats.DecodeOptions{})
	assert.NoError(t, err)
}

func TestDecodeHonoursPNGOrientation(t *testing.T) {
	front := pngWithExif(t, quadrantImage(300, 200), exifBlob(6, 0, 0))

	pc, err := BundleFromReaders(types.Metadata{}, front).Decode(formats.DecodeOptions{})
	require.NoError(t, err)

	assert.Equal(t, image.Rect(0, 0, 200, 300), pc.Front.Bounds())
	assertNear(t, red, pc.Front, 149, 50)
}
