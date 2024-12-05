package usd

import (
	_ "embed"

	"fmt"
	"io"
	"io/fs"
	"text/template"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/general"
	"github.com/jphastings/dotpostcard/types"
)

const codecName = "USD 3D model"

//go:embed postcard.usda.tmpl
var usdTmplData string
var usdTmpl *template.Template

var postcardGSM float64 = 350

const gsmToKgscm float64 = 0.0000001

var funcs = template.FuncMap{
	"half": func(n float64) float64 { return n / 2 },
}

func init() {
	tmpl, err := template.New("postcard-usd").Funcs(funcs).Parse(usdTmplData)
	if err != nil {
		panic(fmt.Sprintf("Couldn't parse USD template: %v", err))
	}
	usdTmpl = tmpl
}

func Codec() formats.Codec { return codec{} }

type codec struct{}

func (c codec) Name() string { return codecName }

// USD can't be decoded yet
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

	MassKg   float64
	FlipAxis []float64
}

const pcThickCm = 0.04

var clockwise = []usdPoint{
	{0, 1},
	{0, 0},
	{1, 0},
	{1, 1},
}

func (c codec) Encode(pc types.Postcard, opts *formats.EncodeOptions) []formats.FileWriter {
	// Note: USDZ files must contain a *binary encoded* USD layer, so we can't create a USDZ here
	// without using the USD C++ API. (Which… perhaps on a rainy Sunday)
	usdFilename := pc.Name + ".usd"
	sideFilename := pc.Name + "-texture.jpg"

	writeUSD := func(w io.Writer) error {
		maxX, _ := pc.Meta.Physical.FrontDimensions.CmWidth.Float64()
		maxY, _ := pc.Meta.Physical.FrontDimensions.CmHeight.Float64()

		frontPoints := make([]usdPoint, len(clockwise))
		backPoints := make([]usdPoint, len(clockwise))
		frontPrimVars := make([]usdPoint, len(clockwise))
		backPrimVars := make([]usdPoint, len(clockwise))

		for i, mul := range clockwise {
			frontPoints[i] = usdPoint{X: mul.X*maxX - maxX/2, Y: mul.Y*maxY - maxY/2}

			switch pc.Meta.Flip {
			case types.FlipNone:
				backPoints[i] = usdPoint{X: mul.X*maxX - maxX/2, Y: mul.Y*maxY - maxY/2}
				frontPrimVars[i] = usdPoint{X: mul.X, Y: mul.Y}
				backPrimVars[i] = frontPrimVars[i]
			case types.FlipCalendar:
				backPoints[(i+2)%4] = usdPoint{X: mul.X*maxX - maxX/2, Y: mul.Y*maxY - maxY/2}
				// Scale & transform Y values to take top and bottom of texture, respectively
				frontPrimVars[i] = usdPoint{X: mul.X, Y: mul.Y*0.5 + 0.5}
				backPrimVars[i] = usdPoint{X: mul.X, Y: mul.Y * 0.5}
			default:
				backPoints[i] = usdPoint{X: mul.X*maxX - maxX/2, Y: mul.Y*maxY - maxY/2}
				// Scale & transform Y values to take top and bottom of texture, respectively
				frontPrimVars[i] = usdPoint{X: mul.X, Y: mul.Y*0.5 + 0.5}
				backPrimVars[i] = usdPoint{X: mul.X, Y: mul.Y * 0.5}
			}
		}

		params := usdParams{
			Creator: fmt.Sprintf("postcards v%s (https://dotpostcard.org)", general.Version),

			MaxX:   maxX,
			MaxY:   maxY,
			MaxZ:   pcThickCm,
			MassKg: (postcardGSM * maxX * maxY) * gsmToKgscm,

			FrontPoints:   frontPoints,
			BackPoints:    backPoints,
			FrontPrimVars: frontPrimVars,
			BackPrimVars:  backPrimVars,

			SidesFilename: sideFilename,
		}

		switch pc.Meta.Flip {
		case types.FlipLeftHand:
			params.FlipAxis = []float64{1, 1, 0}
		case types.FlipRightHand:
			params.FlipAxis = []float64{-1, 1, 0}
		case types.FlipCalendar:
			params.FlipAxis = []float64{1, 0, 0}
		case types.FlipBook:
			params.FlipAxis = []float64{0, 1, 0}
		}

		return usdTmpl.Execute(w, params)
	}

	writeJPG := func(w io.Writer) error {
		webJPG, _ := web.Codec("jpg")
		fws := webJPG.Encode(pc, opts)
		if len(fws) != 1 {
			return fmt.Errorf("couldn't encode postcard textures into JPG")
		}
		return fws[0].WriteTo(w)
	}

	return []formats.FileWriter{
		formats.NewFileWriter(usdFilename, writeUSD),
		formats.NewFileWriter(sideFilename, writeJPG),
	}
}
