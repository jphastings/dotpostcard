package component

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"

	"git.sr.ht/~sbinet/gg"
	"github.com/ernyoke/imger/blur"
	"github.com/ernyoke/imger/edgedetection"
	"github.com/ernyoke/imger/padding"
	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/internal/resolution"
	"github.com/jphastings/postcards/types"
	_ "github.com/sunshineplan/tiff"
	"golang.org/x/image/draw"
)

func (b bundle) Decode(opts formats.DecodeOptions) (types.Postcard, error) {
	pc, err := b.metaBundle.Decode(opts)
	if err != nil {
		return types.Postcard{}, err
	}

	pc.Name = b.name

	img, size, err := decodeImage(b.frontFile, opts)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("couldn't decode postcard's front image: %w", err)
	}

	pc.Front, pc.Meta.Front.Secrets, err = hideSecrets(img, pc.Meta.Front.Secrets)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("couldn't hide secrets on front: %w", err)
	}
	pc.Meta.Physical.FrontDimensions = size

	if b.backFile == nil {
		pc.Meta.Flip = types.FlipNone
	} else {
		img, size, err := decodeImage(b.backFile, opts)
		if err != nil {
			return types.Postcard{}, fmt.Errorf("couldn't decode postcard's back image: %w", err)
		}

		if !size.SimilarPhysical(pc.Meta.Physical.FrontDimensions, pc.Meta.Flip) {
			return types.Postcard{}, fmt.Errorf("the front and back images are different physical sizes, are they of the same postcard?")
		}

		pc.Back, pc.Meta.Back.Secrets, err = hideSecrets(img, pc.Meta.Back.Secrets)
		if err != nil {
			return types.Postcard{}, fmt.Errorf("couldn't hide secrets on back: %w", err)
		}
	}

	if err := validateMetadata(pc); err != nil {
		return types.Postcard{}, err
	}

	return pc, nil
}

func decodeImage(r io.Reader, decOpts formats.DecodeOptions) (image.Image, types.Size, error) {
	var dataCopy bytes.Buffer
	t := io.TeeReader(r, &dataCopy)

	img, _, err := image.Decode(t)
	if err != nil {
		return nil, types.Size{}, err
	}
	bounds := img.Bounds()
	size := types.Size{
		PxWidth:  bounds.Dx(),
		PxHeight: bounds.Dy(),
	}

	if decOpts.RemoveBackground {
		img, err = removeBackground(img)
		if err != nil {
			return nil, types.Size{}, err
		}
	}

	xRes, yRes, err := resolution.Decode(dataCopy.Bytes())
	if err != nil {
		// Invalid physical dimensions just get ignored
		return img, size, nil
	}

	if xRes.Sign() != 0 && yRes.Sign() != 0 {
		size.SetResolution(xRes, yRes)
	}

	return img, size, nil
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
	var modal color.Color
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

var ErrAlreadyTransparent = errors.New("this image already has transparent pixels, ")

func removeBackground(img image.Image) (image.Image, error) {
	if _, _, _, a := img.At(0, 0).RGBA(); a != 65535 {
		return nil, ErrAlreadyTransparent
	}

	return removeBorder(img)
}

type rollingColor struct {
	av     float64
	av2    float64
	stdDev float64
}

func borderFinder(img *image.Gray, rows int) func(color.Color) bool {
	bounds := img.Bounds()
	var n uint32
	var stats rollingColor

	addToStats := func(c color.Color) {
		n++

		gray, _, _, _ := c.RGBA()
		stats.av = stats.av + (float64(gray)-float64(stats.av))/float64(n)
		stats.av2 = stats.av2 + (float64(gray)*float64(gray)-stats.av2)/float64(n)
	}

	for y := 0; y < rows; y++ {
		for x := 0; x < bounds.Dx(); x++ {
			addToStats(img.At(x, y))
		}
	}

	stats.stdDev = math.Sqrt(stats.av2 - (stats.av * stats.av))

	most := stats.av + 4*stats.stdDev
	thresh := most + (65535-most)*2/3

	return func(c color.Color) bool {
		gray, _, _, _ := c.RGBA()

		return float64(gray) > thresh
	}
}

var rotation = map[int]func(image.Rectangle, int, int) (int, int){
	0: func(bnd image.Rectangle, x, y int) (int, int) { return x, y },
	1: func(bnd image.Rectangle, x, y int) (int, int) { return y, bnd.Dx() - x },
	2: func(bnd image.Rectangle, x, y int) (int, int) { return bnd.Dx() - x, bnd.Dy() - y },
	3: func(bnd image.Rectangle, x, y int) (int, int) { return bnd.Dy() - y, x },
}

type borderEdge struct {
	isHorizontal bool
	points       []image.Point
	mode         int
}

func removeBorder(img image.Image) (image.Image, error) {
	bounds := img.Bounds()
	newImg := image.NewRGBA(bounds)

	var borderEdges [4]borderEdge

	for side := 0; side < 4; side++ {
		be := borderEdge{
			isHorizontal: side%2 == 0,
		}
		var b image.Rectangle
		if be.isHorizontal {
			b = bounds
		} else {
			b = image.Rect(0, 0, bounds.Dy(), bounds.Dx())
		}
		fImg := image.NewGray(b)

		for ry := 0; ry < b.Dy(); ry++ {
			for rx := 0; rx < b.Dx(); rx++ {
				x, y := rotation[side](b, rx, ry)
				fImg.Set(rx, ry, img.At(x, y))
			}
		}

		edge, mode, err := findTopBorderEdgePoints(fImg)
		if err != nil {
			return nil, err
		}

		// Find the line for the border either side
		cx, cy := rotation[side](b, 0, mode)
		if be.isHorizontal {
			be.mode = cy
		} else {
			be.mode = cx
		}

		for _, e := range edge {
			x, y := rotation[side](b, e.X, e.Y)
			be.points = append(be.points, image.Point{X: x, Y: y})
		}
		borderEdges[side] = be
	}

	dc := gg.NewContext(bounds.Dx(), bounds.Dy())

	for side := 3; side >= 0; side-- {
		be := borderEdges[side]

		acModeXY := borderEdges[(side+3)%4].mode
		ccModeXY := borderEdges[(side+1)%4].mode
		// TODO: Perhaps a better way of doing this
		if side == 0 || side == 3 {
			t := acModeXY
			acModeXY = ccModeXY
			ccModeXY = t
		}

		for _, e := range be.points {
			isBorderHorizontal := be.isHorizontal && (e.X < acModeXY || e.X > ccModeXY)
			isBorderVertical := !be.isHorizontal && (e.Y < acModeXY || e.Y > ccModeXY)
			if isBorderHorizontal || isBorderVertical {
				continue
			}
			dc.LineTo(float64(e.X), float64(e.Y))
		}
	}
	dc.SetRGB(0, 0, 0)
	dc.Fill()

	gray := image.NewGray(bounds)
	mask := dc.AsMask()
	draw.Copy(gray, image.Point{}, mask, bounds, draw.Src, nil)

	blurred, err := blur.GaussianBlurGray(gray, 1, 1, padding.BorderReflect)
	if err != nil {
		return nil, err
	}

	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			gray, _, _, _ := blurred.At(x, y).RGBA()

			c := color.NRGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(gray >> 8),
			}
			newImg.Set(x, y, c)
		}
	}

	return newImg, nil
}

// TODO: Swap this to a stddev of the mode?
var allowDev = 1200

func findTopBorderEdgePoints(img *image.Gray) ([]image.Point, int, error) {
	bounds := img.Bounds()
	deviation := bounds.Dx() / allowDev
	if devY := bounds.Dy() / allowDev; devY > deviation {
		deviation = devY
	}

	bImg, err := edgedetection.HorizontalSobelGray(img, padding.BorderReflect)
	if err != nil {
		return nil, 0, err
	}

	isEdge := borderFinder(bImg, 8)

	modeTrack := make(map[int]int)
	modeMax := 0
	modeY := 0

	var edge []image.Point
	for x := 0; x < bounds.Dx(); x++ {
		for y := 0; y < bounds.Dy(); y++ {
			c := bImg.At(x, y)
			if isEdge(c) {
				if y != 0 && y != bounds.Dy() {
					// Go two extra pixels inwards
					y += 2
					modeTrack[y]++
					if modeTrack[y] > modeMax {
						modeMax = modeTrack[y]
						modeY = y
					}
					edge = append(edge, image.Point{X: x, Y: y})
				}
				break
			}
		}
	}

	for i, e := range edge {
		if e.Y > modeY+deviation || e.Y < modeY-deviation {
			brightestY := 0
			brightestVal := uint32(0)
			for y := modeY - deviation; y < modeY+deviation; y++ {
				val, _, _, _ := bImg.At(e.X, y).RGBA()
				if val > brightestVal {
					brightestY = y
					brightestVal = val
				}
			}
			edge[i] = image.Point{X: e.X, Y: brightestY}
		}
	}

	return edge, modeY, nil
}
