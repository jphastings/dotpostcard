package usd

import (
	"bytes"
	"errors"
	"image/color"
	"path"
	"slices"
	"strings"

	"fmt"
	"io"
	"io/fs"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/geom3d"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/internal/version"
	"github.com/jphastings/dotpostcard/types"
)

const codecName = "USD 3D model"

//go:generate qtc -file postcard.usda.qtpl

const (
	postcardGSM float64 = 350
	gsmToKgscm  float64 = 0.0000001
	extension           = ".postcard.usd"
)

var (
	beforeTextureMarker = []byte("asset inputs:file = @")
	afterTextureMarker  = []byte("@")
)

func Codec() formats.Codec { return codec{} }

type codec struct{}

func (c codec) Name() string { return codecName }

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	var bundles []formats.Bundle
	var remaining []fs.File
	var finalErr error
	var skip []string

	for _, file := range group.Files {
		filename, ok := formats.HasFileSuffix(file, extension, ".postcard.usda", ".postcard-texture.jpg", ".postcard-texture.png")
		if !ok {
			remaining = append(remaining, file)
		}
		if slices.Contains(skip, filename) {
			continue
		}

		if strings.HasPrefix(path.Ext(filename), ".usd") {
			tFile, tFilename, err := usdaToTextureFile(file, group.Dir)
			if err != nil {
				finalErr = errors.Join(finalErr, fmt.Errorf("unable to open the texture file: %w", err))
				continue
			}
			if slices.Contains(skip, tFilename) {
				continue
			}
			file = tFile
			skip = append(skip, tFilename)
		}
		bundles = append(bundles, web.BundleFromReader(file, filename))
		skip = append(skip, filename)
	}
	return bundles, remaining, finalErr
}

func usdaToTextureFile(file fs.File, dir fs.FS) (fs.File, string, error) {
	usda, err := io.ReadAll(file)
	if err != nil {
		return nil, "", fmt.Errorf("unable to read USDA file to discover texture name: %w", err)
	}

	textureStart := bytes.Index(usda, beforeTextureMarker)
	if textureStart == -1 {
		// Not a postcard USDA
		return nil, "", nil
	}
	textureStart += len(beforeTextureMarker)
	textureLen := bytes.Index(usda[textureStart:], afterTextureMarker)
	if textureStart == -1 {
		// Not a postcard USDA, probably not a valid USDA either
		return nil, "", nil
	}
	texturePath := string(usda[textureStart : textureStart+textureLen])

	texture, err := dir.Open(texturePath)
	if err != nil {
		return nil, "", fmt.Errorf("unable to open the texture file: %w", err)
	}
	return texture, texturePath, nil
}

type usdParams struct {
	Creator   string
	CardColor color.RGBA

	MaxX float64
	MaxY float64
	MaxZ float64

	FrontPoints    []geom3d.Point
	BackPoints     []geom3d.Point
	BackSidePoints []geom3d.Point
	FrontTriangles []int
	BackTriangles  []int
	SideTriangles  []int

	SidesFilename string

	MassKg   float64
	FlipAxis []float64
}

func (c codec) Encode(pc types.Postcard, opts *formats.EncodeOptions) ([]formats.FileWriter, error) {
	usdFilename := pc.Name + extension

	// Grab the filename of the texture image, as it might be JPG or PNG
	webImg, _ := web.Codec("jpeg", "png")
	// We can scrub the transparency data (it's represented in mesh points)
	// And make a significantly smaller (JPEG powered) texture.
	// We must not do this for archival requests, as it loses the transparency data forever.
	opts.NoTransparency = !opts.WantsLossless()
	fws, err := webImg.Encode(pc, opts)
	if err != nil {
		return nil, err
	}

	if len(fws) != 1 {
		return nil, fmt.Errorf("couldn't encode postcard textures")
	}
	fw := fws[0]

	ext := path.Ext(fw.Filename)
	sideFilename := strings.TrimSuffix(fw.Filename, ext) + "-texture" + ext
	writeImage := func(w io.Writer) error { return fw.WriteTo(w) }
	imageMimetype := fw.Mimetype

	writeUSD := func(w io.Writer) error {
		// Both sides are required to be the same physical size, so this is safe
		maxX, maxY := pc.Meta.Physical.FrontDimensions.MustPhysical()

		// TODO: Coregister front & back?
		// TODO: Handle no back

		frontPoints, err := images.Outline(pc.Front, false, true)
		if err != nil {
			return fmt.Errorf("front image can't be outlined: %w", err)
		}
		fTris := geom3d.Triangulate(frontPoints)

		backPoints, err := images.Outline(pc.Back, true, true)
		if err != nil {
			return fmt.Errorf("back image can't be outlined: %w", err)
		}
		// Generate triangles on unrotated points
		bTris := geom3d.Triangulate(backPoints)
		backPoints = geom3d.RotateForFlip(backPoints, pc.Meta.Flip)

		sTris := geom3d.SideMesh(frontPoints, backPoints)

		params := usdParams{
			// TODO: Fix circular import to get to version number here.
			Creator:   fmt.Sprintf("postcards v%s (https://dotpostcard.org)", version.Version),
			CardColor: pc.Meta.Physical.GetCardColor(),

			MaxX:   maxX,
			MaxY:   maxY,
			MaxZ:   pc.Meta.Physical.GetThicknessMM() / 10.0,
			MassKg: (postcardGSM * maxX * maxY) * gsmToKgscm,

			FrontPoints:    frontPoints,
			BackPoints:     backPoints,
			FrontTriangles: fTris,
			BackTriangles:  bTris,
			SideTriangles:  sTris,

			SidesFilename: sideFilename,
		}

		switch pc.Meta.Flip {
		case types.FlipLeftHand:
			params.FlipAxis = []float64{1, 1, 0}
		case types.FlipRightHand:
			params.FlipAxis = []float64{1, -1, 0}
		case types.FlipCalendar:
			params.FlipAxis = []float64{1, 0, 0}
		case types.FlipBook:
			params.FlipAxis = []float64{0, 1, 0}
		}

		WriteUSDA(w, params)
		return nil
	}

	return []formats.FileWriter{
		formats.NewFileWriter(usdFilename, "model/vnd.usda", writeUSD),
		formats.NewFileWriter(sideFilename, imageMimetype, writeImage),
	}, nil
}
