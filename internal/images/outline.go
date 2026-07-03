package images

import (
	"fmt"
	"image"
	"slices"

	"github.com/jphastings/dotpostcard/internal/geom3d"
)

const (
	defaultThreshold = 128
	defaultEpsilonPx = 1.5
)

type OutlineOpts struct {
	// Pixels with alpha >= Threshold are treated as part of the postcard. The
	// zero value means 128 (half opacity).
	Threshold uint8
	// EpsilonPx is the simplification tolerance, in pixels. The zero value
	// means 1.5px.
	EpsilonPx float64
}

// Returns the outline of the image's transparency as an _anticlockwise_ series of X/Y points
func Outline(im image.Image, invertX, invertY bool) ([]geom3d.Point, error) {
	return OutlineWithOpts(im, invertX, invertY, OutlineOpts{})
}

// OutlineWithOpts traces the largest opaque region's outline along pixel
// boundaries, so coordinates span the full [0,1] range and a fully opaque
// image outlines to exactly the unit square.
func OutlineWithOpts(im image.Image, invertX, invertY bool, opts OutlineOpts) ([]geom3d.Point, error) {
	if opts.Threshold == 0 {
		opts.Threshold = defaultThreshold
	}
	if opts.EpsilonPx == 0 {
		opts.EpsilonPx = defaultEpsilonPx
	}

	alpha := toAlpha(im)
	contour := traceLargestContour(alpha, opts.Threshold)
	if contour == nil {
		return nil, fmt.Errorf("the image appears to be fully transparent")
	}

	path := simplifyClosed(contour, opts.EpsilonPx)
	if len(path) < 3 {
		return nil, fmt.Errorf("the outline of the image is only %d points; it doesn't enclose any area", len(path))
	}

	w, h := float64(alpha.Rect.Dx()), float64(alpha.Rect.Dy())
	geomPath := make([]geom3d.Point, len(path))
	for i, p := range path {
		geomPath[i] = geom3d.Point{X: float64(p.x) / w, Y: float64(p.y) / h}
		if invertX {
			geomPath[i].X = 1 - geomPath[i].X
		}
		if invertY {
			geomPath[i].Y = 1 - geomPath[i].Y
		}
	}

	return ensureDirection(geomPath), nil
}

func ensureDirection(points []geom3d.Point) []geom3d.Point {
	if geom3d.Area(points) > 0 {
		slices.Reverse(points)
	}

	return points
}
