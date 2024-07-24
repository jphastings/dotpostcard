package usd

import (
	_ "embed"

	"fmt"
	"io"
	"io/fs"
	"text/template"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/web"
	"github.com/jphastings/postcards/internal/general"
	"github.com/jphastings/postcards/types"
)

const codecName = "USDZ 3D model"

//go:embed postcard.usda.tmpl
var usdTmplData string
var usdTmpl *template.Template

func init() {
	tmpl, err := template.New("postcard-usd").Parse(usdTmplData)
	if err != nil {
		panic(fmt.Sprintf("Couldn't parse USD template: %v", err))
	}
	usdTmpl = tmpl
}

func Codec() formats.Codec { return codec{} }

type codec struct{}

func (c codec) Name() string { return codecName }

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	return nil, group.Files, nil
}

type usdPoint struct {
	X float64
	Y float64
}

type usdParams struct {
	Creator string

	MaxX float64
	MaxY float64
	MaxZ float64

	FrontPoints   []usdPoint
	FrontPrimVars []usdPoint
	BackPoints    []usdPoint
	BackPrimVars  []usdPoint

	SidesFilename string
}

const pcThickCm = 0.04

var clockwise = []usdPoint{
	{0, 1},
	{0, 0},
	{1, 0},
	{1, 1},
}

func (c codec) Encode(pc types.Postcard, opts formats.EncodeOptions) []formats.FileWriter {
	// Note: USDZ files must contain a *binary encoded* USD layer, so we can't create a USDZ here
	// without using the USD C++ API. (Which… perhaps on a rainy Sunday)
	usdFilename := pc.Name + ".usd"
	sideFilename := pc.Name + "-texture.png"

	writeUSD := func(w io.Writer) error {
		maxX, _ := pc.Meta.FrontDimensions.CmWidth.Float64()
		maxY, _ := pc.Meta.FrontDimensions.CmHeight.Float64()

		frontPoints := make([]usdPoint, len(clockwise))
		backPoints := make([]usdPoint, len(clockwise))
		frontPrimVars := make([]usdPoint, len(clockwise))
		backPrimVars := make([]usdPoint, len(clockwise))

		for i, mul := range clockwise {
			frontPoints[i] = usdPoint{X: mul.X * maxX, Y: mul.Y * maxY}

			switch pc.Meta.Flip {
			case types.FlipNone:
				backPoints[i] = usdPoint{X: mul.X * maxX, Y: mul.Y * maxY}
				frontPrimVars[i] = usdPoint{X: mul.X, Y: mul.Y}
				backPrimVars[i] = frontPrimVars[i]
			case types.FlipCalendar:
				backPoints[(i+2)%4] = usdPoint{X: mul.X * maxX, Y: mul.Y * maxY}
				// Scale & transform Y values to take top and bottom of texture, respectively
				frontPrimVars[i] = usdPoint{X: mul.X, Y: mul.Y*0.5 + 0.5}
				backPrimVars[i] = usdPoint{X: mul.X, Y: mul.Y * 0.5}
			default:
				backPoints[i] = usdPoint{X: mul.X * maxX, Y: mul.Y * maxY}
				// Scale & transform Y values to take top and bottom of texture, respectively
				frontPrimVars[i] = usdPoint{X: mul.X, Y: mul.Y*0.5 + 0.5}
				backPrimVars[i] = usdPoint{X: mul.X, Y: mul.Y * 0.5}
			}
		}

		params := usdParams{
			Creator: fmt.Sprintf("postcards v%s (https://dotpostcard.org)", general.Version),

			MaxX: maxX,
			MaxY: maxY,
			MaxZ: pcThickCm,

			FrontPoints:   frontPoints,
			BackPoints:    backPoints,
			FrontPrimVars: frontPrimVars,
			BackPrimVars:  backPrimVars,

			SidesFilename: sideFilename,
		}

		return usdTmpl.Execute(w, params)
	}

	writePNG := func(w io.Writer) error {
		fws := web.Codec("png").Encode(pc, opts)
		if len(fws) != 1 {
			return fmt.Errorf("couldn't encode postcard textures into PNG")
		}
		return fws[0].WriteTo(w)
	}

	return []formats.FileWriter{
		formats.NewFileWriter(usdFilename, writeUSD),
		formats.NewFileWriter(sideFilename, writePNG),
	}
}
