package component

import (
	"errors"
	"image"
	"image/color"
	"math"

	"git.sr.ht/~sbinet/gg"
	"github.com/ernyoke/imger/edgedetection"
	"github.com/ernyoke/imger/padding"
)

var ErrAlreadyTransparent = errors.New("this image already has transparent pixels, ")

type rollingColor struct {
	av     float64
	av2    float64
	stdDev float64
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

const (
	pcPixelsInLine = 80
	travelExtra    = 2 // px further after finding sobel edge
	borderMinThick = 8
)

func removeBorder(img image.Image) (image.Image, error) {
	bounds := img.Bounds()

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
	dc.Clip()
	dc.DrawImage(img, 0, 0)

	return dc.Image(), nil
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

func findTopBorderEdgePoints(img *image.Gray) ([]image.Point, int, error) {
	bounds := img.Bounds()

	bImg, err := edgedetection.HorizontalSobelGray(img, padding.BorderReflect)
	if err != nil {
		return nil, 0, err
	}

	isEdge := borderFinder(bImg, borderMinThick)

	modeTrack := make(map[int]int)
	modeMax := 0
	modeY := 0

	var edge []image.Point
	for x := 0; x < bounds.Dx(); x++ {
		for y := 0; y < bounds.Dy(); y++ {
			c := bImg.At(x, y)
			if isEdge(c) {
				if y < bounds.Dy()/8 {
					// Go two extra pixels inwards
					y += travelExtra
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

	devCountdown := ((bounds.Dx() * pcPixelsInLine) / 100) - modeMax
	devPos := 1
	devNeg := 1

	for i := 1; i < 25; i++ {
		if devCountdown <= 0 {
			break
		}
		amtPos := modeTrack[modeY+devPos]
		amtNeg := modeTrack[modeY-devNeg]
		if amtPos > amtNeg {
			devPos++
			devCountdown -= amtPos
		} else {
			devNeg++
			devCountdown -= amtNeg
		}
	}

	for i, e := range edge {
		if e.Y > modeY+devPos || e.Y < modeY-devNeg {
			brightestY := modeY
			brightestVal := uint32(0)
			for y := modeY; y <= modeY+devPos; y++ {
				val, _, _, _ := bImg.At(e.X, y).RGBA()
				if val > brightestVal {
					brightestY = y
					brightestVal = val
				}
			}
			edge[i] = image.Point{X: e.X, Y: brightestY + travelExtra}
		}
	}

	return edge, modeY, nil
}
