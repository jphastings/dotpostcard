package web

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"io"
	"strings"

	"github.com/jphastings/dotpostcard/internal/geom3d"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/types"
)

//go:generate qtc -file postcard.svg.qtpl

type svgVars struct {
	size        types.Size
	b64Img      string
	frontPoints []geom3d.Point
	backPoints  []geom3d.Point
	hasBack     bool
}

// This keeps the whole JPEG in memory. TODO: Figure out how to do this while streaming
func writeSVG(w io.Writer, pc types.Postcard, combinedImg image.Image, xmpData []byte, hasTransparency bool) error {
	v := svgVars{
		size:    pc.Meta.Physical.FrontDimensions,
		hasBack: pc.Back != nil,
	}

	if hasTransparency {
		frontPoints, err := images.Outline(pc.Front, false, false)
		if err != nil {
			return fmt.Errorf("front image can't be outlined: %w", err)
		}
		v.frontPoints = frontPoints

		if v.hasBack {
			backPoints, err := images.Outline(pc.Back, false, false)
			if err != nil {
				return fmt.Errorf("back image can't be outlined: %w", err)
			}
			v.backPoints = backPoints
		}
	}

	var buf bytes.Buffer
	b64W := base64.NewEncoder(base64.StdEncoding, &buf)
	if err := images.WriteJPEG(b64W, combinedImg, xmpData); err != nil {
		return err
	}
	v.b64Img = buf.String()

	WriteSVG(w, v)
	return nil
}

// TODO: Figure out if bezier curves are worth it
// const (
// 	bezierTension    = 1.0 / 6.0
// 	smoothUnderAngle = 45
// )

type bezierPoint struct {
	px, py float64
	// TODO: Figure out if bezier curves are worth it
	// c1x, c1y float64
	// c2x, c2y float64
}

func pointsToPath(points []geom3d.Point, w, h int, back bool) string {
	offsetH := 0
	if back {
		offsetH = h
	}

	bp := make([]bezierPoint, len(points))
	for i, p := range points {
		bp[i].px = p.X * float64(w)
		bp[i].py = p.Y*float64(h) + float64(offsetH)
	}
	// TODO: Figure out if bezier curves are worth it
	// for i, p := range bp {
	// 	p0, p1, p2, p3 := bp[(i-1+len(bp))%len(bp)], p, bp[(i+1)%len(bp)], bp[(i+2)%len(bp)]

	// 	angle := calcAngle(p0, p1, p2)
	// 	if angle < smoothUnderAngle {
	// 		bp[i].c1x = p1.px + (p2.px-p0.px)*bezierTension
	// 		bp[i].c1y = p1.py + (p2.py-p0.py)*bezierTension
	// 		bp[i].c2x = p2.px + (p3.px-p1.px)*bezierTension
	// 		bp[i].c2y = p2.py + (p3.py-p1.py)*bezierTension
	// 	}
	// }

	var path strings.Builder
	for i, p := range bp {
		if i == 0 {
			path.WriteString(fmt.Sprintf(
				"M%.1f %.1f",
				p.px, p.py,
			))
			// TODO: Figure out if bezier curves are worth it
			// } else if p.c1x != 0 && p.c1y != 0 && p.c2x != 0 && p.c2y != 0 {
			// 	path.WriteString(fmt.Sprintf(
			// 		"C%.1f %.1f,%.1f %.1f,%.1f %.1f",
			// 		p.c1x, p.c1y,
			// 		p.c2x, p.c2y,
			// 		p.px, p.py,
			// 	))
		} else {
			path.WriteString(fmt.Sprintf(
				"L%.1f %.1f",
				p.px, p.py,
			))
		}
	}
	return path.String()
}

// TODO: Figure out if bezier curves are worth it
// func calcAngle(p0, p1, p2 bezierPoint) float64 {
// 	v1x, v1y := p1.px-p0.px, p1.py-p0.py
// 	v2x, v2y := p2.px-p1.px, p2.py-p1.py

// 	dot := v1x*v2x + v1y*v2y
// 	mag1 := math.Hypot(v1x, v1y)
// 	mag2 := math.Hypot(v2x, v2y)

// 	// Points are on top of each other, treat as straight
// 	if mag1 == 0 || mag2 == 0 {
// 		return 180
// 	}

// 	cosTheta := dot / (mag1 * mag2)
// 	return 180 * math.Acos(math.Max(-1, math.Min(1, cosTheta))) / math.Pi
// }
