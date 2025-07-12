package webp

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	stdwebp "golang.org/x/image/webp"
)

//go:embed testdata/golden-in.png
var goldenIn []byte

//go:embed testdata/golden-out.webp
var goldenOut []byte

func TestLossless(t *testing.T) {
	m, err := png.Decode(bytes.NewReader(goldenIn))
	if err != nil {
		t.Fatalf("decoding image: %v", err)
	}

	buf1 := bytes.NewBuffer(nil)
	buf2 := bytes.NewBuffer(nil)

	if err := Encode(io.MultiWriter(buf1, buf2), m, Lossless()); err != nil {
		t.Fatalf("encoding webp: %v", err)
	}

	if err := os.WriteFile(filepath.Join("testdata", "golden-got.webp"), buf1.Bytes(), 0o644); err != nil {
		t.Errorf("writing output png: %v", err)
	}

	t.Run("stdlib webp outupt", func(t *testing.T) {
		assertOutput(t, m, buf1, stdwebp.Decode)
	})

	t.Run("transpiled webp output", func(t *testing.T) {
		assertOutput(t, m, buf2, Decode)
	})
}

func BenchmarkDecode(b *testing.B) {
	b.Run("stdwebp", func(b *testing.B) {
		for ii := 0; ii < b.N; ii++ {
			r := bytes.NewReader(goldenOut)
			m, err := stdwebp.Decode(r)
			if err != nil {
				b.Fatalf("error: %v", err)
			}
			_ = m
		}
	})
	b.Run("transpiled", func(b *testing.B) {
		for ii := 0; ii < b.N; ii++ {
			m, err := decodeTranspiled(goldenOut)
			if err != nil {
				b.Fatalf("error: %v", err)
			}
			_ = m
		}
	})
	b.Run("dynamic", func(b *testing.B) {
		for ii := 0; ii < b.N; ii++ {
			m, err := decodeDynamic(goldenOut)
			if err != nil {
				b.Fatalf("error: %v", err)
			}
			_ = m
		}
	})
}

func assertOutput(t *testing.T, m image.Image, src io.Reader, decode func(io.Reader) (image.Image, error)) {
	t.Helper()

	out, err := decode(src)
	if err != nil {
		t.Fatalf("decoding webp: %v", err)
	}

	outb := out.Bounds()
	for xx := outb.Min.X; xx < outb.Max.X; xx++ {
		for yy := outb.Min.Y; yy < outb.Max.Y; yy++ {
			if got, want := out.At(xx, yy), m.At(xx, yy); !colorEqual(got, want) {
				save(t, fmt.Sprintf("got-%d_%d.webp", xx, yy), out)
				save(t, fmt.Sprintf("want-%d_%d.webp", xx, yy), m)
				t.Fatalf("color mismatch after lossless encode: Point = (%d, %d) Got = %v Want = %v Bounds= %d", xx, yy, got, want, outb)
			}
		}
	}
}

func save(t *testing.T, name string, m image.Image) {
	t.Helper()

	if err := func() error {
		f, err := os.Create(name)
		if err != nil {
			return err
		}
		defer f.Close()
		defer f.Sync()
		return Encode(f, m, Lossless())

	}(); err != nil {
		t.Error(err)
	}
}

func colorEqual(left, right color.Color) bool {
	lr, lg, lb, la := left.RGBA()
	rr, rg, rb, ra := right.RGBA()
	return lr == rr && lg == rg && lb == rb && la == ra
}

func FuzzEncode(f *testing.F) {
	f.Add(uint16(000), uint16(000), int64(0), float32(0.00), true)
	f.Add(uint16(000), uint16(000), int64(0), float32(0.00), false)
	f.Add(uint16(001), uint16(001), int64(1), float32(1.00), true)
	f.Add(uint16(001), uint16(001), int64(1), float32(1.00), false)
	f.Add(uint16(001), uint16(001), int64(2), float32(0.50), true)
	f.Add(uint16(001), uint16(001), int64(2), float32(0.50), false)
	f.Add(uint16(100), uint16(100), int64(3), float32(0.75), true)
	f.Add(uint16(100), uint16(100), int64(3), float32(0.75), false)
	f.Add(uint16(100), uint16(100), int64(3), float32(0.90), true)
	f.Add(uint16(100), uint16(100), int64(3), float32(0.90), false)
	f.Add(uint16(100), uint16(100), int64(3), float32(0.95), true)
	f.Add(uint16(100), uint16(100), int64(3), float32(0.95), false)
	f.Add(uint16(100), uint16(100), int64(3), float32(1.00), true)
	f.Add(uint16(100), uint16(100), int64(3), float32(1.00), false)

	f.Fuzz(func(t *testing.T, x1, y1 uint16, seed int64, quality float32, lossless bool) {
		if quality <= 0 || quality > 1 {
			t.Skip()
			return
		}

		if int(x1) <= 0 || int(y1) <= 0 {
			t.Skip()
			return
		}

		t.Logf("(%d, %d), quality=%.2f, lossless=%v", x1, y1, quality, lossless)

		rng := rand.New(rand.NewSource(seed))

		m := image.NewNRGBA(image.Rect(0, 0, int(x1), int(y1)))

		for x := uint16(0); x < x1; x += 1 {
			for y := uint16(0); y < y1; y += 1 {
				colors := rng.Uint32()
				r := uint8(colors)
				g := uint8(colors >> 8)
				b := uint8(colors >> 16)
				a := uint8(colors >> 24)
				m.SetNRGBA(int(x), int(y), color.NRGBA{R: r, G: g, B: b, A: a})
			}
		}

		opts := []EncodeOption{Quality(quality)}

		if lossless {
			opts = append(opts, Lossless())
		}

		buf := bytes.NewBuffer(nil)

		if err := Encode(buf, m, opts...); err != nil {
			t.Errorf("encode error: %v", err)
		}

		if lossless {
			// If lossless, we can do a pixel-wise comparison between the original and the
			// decoded image.
			assertOutput(t, m, buf, Decode)
		}
	})
}
