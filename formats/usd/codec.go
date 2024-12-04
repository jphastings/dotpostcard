package usd

import (
	_ "embed"
	"math"
	"time"

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

	Flip *usdParamsFlip
}

type usdParamsFlip struct {
	FrameCount uint
	Keyframes  map[uint]float64
	Orient     int
	Unorient   int
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
		flip := makeFlip(pc.Meta.Flip, 24, 0, []keyframe{
			{0, 10 * time.Second},
			{180, 1 * time.Second},
			{180, 5 * time.Second},
			{360, 1 * time.Second},
		}...)

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

			Flip: flip,
		}

		return usdTmpl.Execute(w, params)
	}

	writeJPG := func(w io.Writer) error {
		fws := web.Codec("jpg").Encode(pc, opts)
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

type keyframe struct {
	angle float64
	dur   time.Duration
}

func makeFlip(f types.Flip, fps uint, startAngle float64, kfs ...keyframe) *usdParamsFlip {
	var orient int
	switch f {
	case types.FlipNone:
		return nil
	case types.FlipBook:
		orient = 180
	case types.FlipLeftHand:
		orient = 225
	case types.FlipCalendar:
		orient = 270
	case types.FlipRightHand:
		orient = 315
	}
	flip := &usdParamsFlip{
		Orient:    orient,
		Unorient:  orient * -1,
		Keyframes: map[uint]float64{0: startAngle},
	}

	lastFrame := uint(0)
	lastAngle := startAngle
	for _, kf := range kfs {
		nextFrame := lastFrame + uint(kf.dur.Seconds()*float64(fps))
		nextAngle := kf.angle

		if nextAngle == lastAngle {
			flip.Keyframes[nextFrame] = nextAngle
			lastFrame = nextFrame
			continue
		}

		dt := int(nextFrame - lastFrame)
		da := float64(nextAngle - lastAngle)
		for i := 0; i <= dt; i++ {
			flip.Keyframes[lastFrame+uint(i)] = lastAngle + da*(1-math.Cos(math.Pi*float64(i)/float64(dt)))/2
		}

		lastFrame = nextFrame
		lastAngle = nextAngle
	}

	flip.FrameCount = lastFrame

	return flip
}
