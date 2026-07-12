// Package matting separates a scanned postcard from the scanner backboard
// with soft (fractional) alpha, preserving fibrous edge detail while
// removing the backboard's colour contribution. The approach is a trimap
// band around a rough geometric mask, a colour-guided filter to estimate
// alpha within the band, and per-pixel colour decontamination.
package matting

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
)

// debugEnvVar names a directory to dump the trimap, estimated alpha, and
// final matte as PNGs — for tuning parameters against real scans.
const debugEnvVar = "POSTCARDS_DEBUG_MATTE"

const (
	defaultEps = 4e-4
	// Band tiles are processed at this core size, padded by the largest
	// filter radius, bounding memory regardless of scan resolution.
	tileSize = 192
)

type Options struct {
	// BandHalfWidthPx is the half-width of the unknown band around the
	// geometric mask edge, in pixels. Required (choose from scan DPI).
	BandHalfWidthPx int
	// GuidedRadiusPx is the guided filter radius; 0 means BandHalfWidthPx.
	GuidedRadiusPx int
	// Eps is the guided filter regularisation; 0 means 4e-4.
	Eps float64
}

// Apply mattes img against its backboard. mask is a row-major w×h grid
// marking pixels roughly inside the card; alpha is estimated in a band
// around the mask's boundary and the returned image carries straight
// (non-premultiplied) soft alpha.
func Apply(img image.Image, mask []bool, opts Options) (*image.NRGBA, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if len(mask) != w*h {
		return nil, fmt.Errorf("mask size %d doesn't match image %d×%d", len(mask), w, h)
	}
	if opts.BandHalfWidthPx <= 0 {
		return nil, fmt.Errorf("BandHalfWidthPx must be positive")
	}
	if opts.GuidedRadiusPx <= 0 {
		// ~¼ of the band width recovers hand-mask-level fibre softness on
		// torn edges without over-feathering die-cut ones (tuned against the
		// ground-truth fixtures; see sweep_test.go).
		opts.GuidedRadiusPx = max(2, opts.BandHalfWidthPx/4)
	}
	if opts.Eps <= 0 {
		opts.Eps = defaultEps
	}

	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)

	tm := BuildTrimap(mask, w, h, opts.BandHalfWidthPx)

	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	var bgSum [3]float64
	var bgN int
	for i := 0; i < w*h; i++ {
		switch {
		case tm.FG[i]:
			copy(out.Pix[i*4:i*4+3], src.Pix[i*4:i*4+3])
			out.Pix[i*4+3] = 255
		case tm.BG[i]:
			bgSum[0] += float64(src.Pix[i*4])
			bgSum[1] += float64(src.Pix[i*4+1])
			bgSum[2] += float64(src.Pix[i*4+2])
			bgN++
		}
	}
	var bgMean [3]float32
	if bgN > 0 {
		for c := 0; c < 3; c++ {
			bgMean[c] = float32(bgSum[c] / float64(bgN) / 255)
		}
	}

	var debugAlpha *image.Gray
	debugDir := os.Getenv(debugEnvVar)
	if debugDir != "" {
		debugAlpha = image.NewGray(image.Rect(0, 0, w, h))
	}

	bgRadius := 3 * opts.BandHalfWidthPx
	pad := max(opts.GuidedRadiusPx, bgRadius) + 1

	for ty := 0; ty < h; ty += tileSize {
		for tx := 0; tx < w; tx += tileSize {
			core := image.Rect(tx, ty, min(tx+tileSize, w), min(ty+tileSize, h))
			if !bandInRect(tm, core) {
				continue
			}
			padded := core.Inset(-pad).Intersect(image.Rect(0, 0, w, h))
			matteTile(src, out, tm, mask, core, padded, opts, bgRadius, bgMean, debugAlpha)
		}
	}

	fillInteriorHoles(out, src)

	if debugDir != "" {
		if err := dumpDebug(debugDir, tm, debugAlpha, out); err != nil {
			return nil, fmt.Errorf("couldn't write %s debug images: %w", debugEnvVar, err)
		}
	}

	return out, nil
}

func bandInRect(tm Trimap, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := y * tm.W
		for x := r.Min.X; x < r.Max.X; x++ {
			if tm.Band[row+x] {
				return true
			}
		}
	}
	return false
}

// matteTile estimates alpha and decontaminated colour for the band pixels
// within core, using image data from the surrounding padded rect.
func matteTile(src, out *image.NRGBA, tm Trimap, mask []bool, core, padded image.Rectangle, opts Options, bgRadius int, bgMean [3]float32, debugAlpha *image.Gray) {
	tw, th := padded.Dx(), padded.Dy()

	var I [3]*plane
	for c := 0; c < 3; c++ {
		I[c] = newPlane(tw, th)
	}
	p := newPlane(tw, th)
	bgm := newPlane(tw, th)
	for y := 0; y < th; y++ {
		srcRow := ((padded.Min.Y + y) * tm.W) + padded.Min.X
		for x := 0; x < tw; x++ {
			i := srcRow + x
			ti := y*tw + x
			I[0].v[ti] = float32(src.Pix[i*4]) / 255
			I[1].v[ti] = float32(src.Pix[i*4+1]) / 255
			I[2].v[ti] = float32(src.Pix[i*4+2]) / 255
			if mask[i] {
				p.v[ti] = 1
			}
			if tm.BG[i] {
				bgm.v[ti] = 1
			}
		}
	}

	alpha := guidedAlpha(I, p, opts.GuidedRadiusPx, float32(opts.Eps))
	B := backgroundField(I, bgm, bgRadius, bgMean)

	for y := core.Min.Y; y < core.Max.Y; y++ {
		for x := core.Min.X; x < core.Max.X; x++ {
			i := y*tm.W + x
			if !tm.Band[i] {
				continue
			}
			ti := (y-padded.Min.Y)*tw + (x - padded.Min.X)

			a := levels(alpha.v[ti])
			if a < alphaFloor {
				a = 0
			}

			if debugAlpha != nil {
				debugAlpha.Pix[i] = uint8(a*255 + 0.5)
			}
			if a == 0 {
				continue // out is zero-initialised (transparent)
			}

			f := decontaminate(
				[3]float32{I[0].v[ti], I[1].v[ti], I[2].v[ti]},
				[3]float32{B[0].v[ti], B[1].v[ti], B[2].v[ti]},
				a,
			)
			out.Pix[i*4] = uint8(f[0]*255 + 0.5)
			out.Pix[i*4+1] = uint8(f[1]*255 + 0.5)
			out.Pix[i*4+2] = uint8(f[2]*255 + 0.5)
			out.Pix[i*4+3] = uint8(a*255 + 0.5)
		}
	}
}

var debugCounter atomic.Int32

func dumpDebug(dir string, tm Trimap, alpha *image.Gray, result *image.NRGBA) error {
	n := debugCounter.Add(1)

	trimapImg := image.NewGray(image.Rect(0, 0, tm.W, tm.H))
	for i := range tm.FG {
		switch {
		case tm.FG[i]:
			trimapImg.Pix[i] = 255
		case tm.Band[i]:
			trimapImg.Pix[i] = 128
		}
	}

	for name, im := range map[string]image.Image{
		"trimap": trimapImg,
		"alpha":  alpha,
		"result": result,
	} {
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("matte-%d-%s.png", n, name)))
		if err != nil {
			return err
		}
		if err := png.Encode(f, im); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}
