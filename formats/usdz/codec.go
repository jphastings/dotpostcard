package usdz

import (
	"archive/zip"
	_ "embed"

	"fmt"
	"io"
	"io/fs"
	"text/template"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/internal/general"
	"github.com/jphastings/postcards/types"
)

const codecName = "USDZ 3D model"

//go:embed postcard.usd.tmpl
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

	FrontPoints []usdPoint
	BackPoints  []usdPoint

	SidesFilename string
}

const pcThickCm = 0.4

var clockwise = []usdPoint{
	{0, 1},
	{0, 0},
	{1, 0},
	{1, 1},
}

func (c codec) Encode(pc types.Postcard, _ formats.EncodeOptions) []formats.FileWriter {
	name := pc.Name + ".usdz"

	writer := func(w io.Writer) error {
		zw := zip.NewWriter(w)
		usdW, err := zw.Create(pc.Name + ".usd")
		if err != nil {
			return err
		}

		maxX, _ := pc.Meta.FrontDimensions.CmWidth.Float64()
		maxY, _ := pc.Meta.FrontDimensions.CmHeight.Float64()

		frontPoints := make([]usdPoint, len(clockwise))
		for i, mul := range clockwise {
			frontPoints[i] = usdPoint{
				X: mul.X * maxX,
				Y: mul.Y * maxY,
			}
		}

		params := usdParams{
			Creator: fmt.Sprintf("postcards v%s (https://dotpostcard.org)", general.Version),

			MaxX: maxX,
			MaxY: maxY,
			MaxZ: pcThickCm,

			FrontPoints: frontPoints,
			BackPoints:  frontPoints,

			SidesFilename: pc.Name + ".png",
			// TODO: Calendar Flip orientation
			// TODO: Single sided postcards
		}

		if err := usdTmpl.Execute(usdW, params); err != nil {
			return err
		}

		return zw.Close()
	}

	return []formats.FileWriter{formats.NewFileWriter(name, writer)}
}
