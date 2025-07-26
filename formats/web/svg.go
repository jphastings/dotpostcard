package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
	metadata    []byte
}

// This keeps the whole JPEG in memory. TODO: Figure out how to do this while streaming
func writeSVG(w io.Writer, pc types.Postcard, combinedImg image.Image, xmpData []byte) error {
	frontPoints, err := images.Outline(pc.Front, false, false)
	if err != nil {
		return fmt.Errorf("front image can't be outlined: %w", err)
	}
	// TODO: Handle no back
	backPoints, err := images.Outline(pc.Back, false, false)
	if err != nil {
		return fmt.Errorf("back image can't be outlined: %w", err)
	}

	var buf bytes.Buffer
	b64W := base64.NewEncoder(base64.StdEncoding, &buf)
	if err := images.WriteJPEG(b64W, combinedImg, xmpData); err != nil {
		return err
	}

	meta, err := json.Marshal(pc.Meta)
	if err != nil {
		return err
	}

	v := svgVars{
		size:        pc.Meta.Physical.FrontDimensions,
		b64Img:      buf.String(),
		frontPoints: frontPoints,
		backPoints:  backPoints,
		metadata:    meta,
	}

	WriteSVG(w, v)
	return nil
}

func pointsToPath(points []geom3d.Point, w, h int, back bool) string {
	offsetH := 0
	if back {
		offsetH = h
	}

	var path strings.Builder
	for i, p := range points {
		if i == 0 {
			path.WriteString("M")
		} else {
			path.WriteString("L")
		}
		x := p.X * float64(w)
		y := p.Y*float64(h) + float64(offsetH)
		path.WriteString(fmt.Sprintf("%.1f %.1f", x, y))

	}
	path.WriteString("Z")
	return path.String()
}
