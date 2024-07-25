package component

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"math"
	"os"

	"git.sr.ht/~sbinet/gg"
	"github.com/ernyoke/imger/blur"
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

// func gray(c color.Color) (color.Gray, uint8) {
// 	r, g, b, a := c.RGBA()
// 	return color.Gray{
// 		Y: uint8(0.299*float64(r)/256 + 0.587*float64(g)/256 + 0.114*float64(b)/256),
// 	}, uint8(a / 256)
// }

func removeBackground(img image.Image) (image.Image, error) {
	if _, _, _, a := img.At(0, 0).RGBA(); a != 65535 {
		return nil, ErrAlreadyTransparent
	}

	newImg, err := removeBorder(img)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile("sobel.png", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	if err := png.Encode(f, newImg); err != nil {
		return nil, err
	}
	os.Exit(1)
	return img, nil
}

type rollingColor struct {
	avR  float64
	avG  float64
	avB  float64
	av2R float64
	av2G float64
	av2B float64

	stdDevR float64
	stdDevG float64
	stdDevB float64
}

func borderFinder(img image.Image, rows int) func(color.Color) bool {
	bounds := img.Bounds()
	var n uint32
	var stats rollingColor

	addToStats := func(c color.Color) {
		n++

		r, g, b, _ := c.RGBA()
		stats.avR = stats.avR + (float64(r)-float64(stats.avR))/float64(n)
		stats.avG = stats.avG + (float64(g)-float64(stats.avG))/float64(n)
		stats.avB = stats.avB + (float64(b)-float64(stats.avB))/float64(n)

		stats.av2R = stats.av2R + (float64(r)*float64(r)-stats.av2R)/float64(n)
		stats.av2G = stats.av2G + (float64(g)*float64(g)-stats.av2G)/float64(n)
		stats.av2B = stats.av2B + (float64(b)*float64(b)-stats.av2B)/float64(n)
	}

	// Top & bottom
	for y := 0; y < rows; y++ {
		for x := 0; x < bounds.Dx(); x++ {
			addToStats(img.At(x, y))
			addToStats(img.At(x, bounds.Dy()-y))
		}
	}
	// Left & Right
	for x := 0; x < rows; x++ {
		for y := rows; y < bounds.Dx()-rows; y++ {
			addToStats(img.At(x, y))
			addToStats(img.At(bounds.Dx()-x, y))
		}
	}

	stats.stdDevR = math.Sqrt(stats.av2R - (stats.avR * stats.avR))
	stats.stdDevG = math.Sqrt(stats.av2G - (stats.avG * stats.avG))
	stats.stdDevB = math.Sqrt(stats.av2B - (stats.avB * stats.avB))

	fmt.Println(stats)

	return func(c color.Color) bool {
		r, g, b, _ := c.RGBA()

		nearR := math.Abs(stats.avR-float64(r)) < 2.2*stats.stdDevR
		nearG := math.Abs(stats.avG-float64(g)) < 2.2*stats.stdDevG
		nearB := math.Abs(stats.avB-float64(b)) < 2.2*stats.stdDevB

		return nearR && nearG && nearB
	}
}

type direction uint

const (
	dirDown direction = iota
	dirLeft
	dirUp
	dirRight
)

type limiter struct {
	prev      int
	foundEdge bool
	allowable int
	midPoint  int
	dir       direction
}

func removeBorder(img image.Image) (image.Image, error) {
	bounds := img.Bounds()
	newImg := image.NewRGBA(bounds)
	draw.Copy(newImg, image.Point{}, img, bounds, draw.Src, nil)

	blurRadius := 4

	blurImg, err := blur.GaussianBlurRGBA(newImg, float64(blurRadius), 4, padding.BorderReflect)
	if err != nil {
		return nil, err
	}

	isBorder := borderFinder(blurImg, 8)

	var doBreak bool

	limRight := limiter{allowable: 1, midPoint: bounds.Dy(), dir: dirRight}
	limLeft := limiter{allowable: 1, midPoint: bounds.Dy(), dir: dirLeft}
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			doBreak, limRight = makeTransparent(x, y, blurRadius, img, blurImg, newImg, isBorder, limRight)
			if doBreak {
				break
			}
		}

		for x := bounds.Dx(); x > 0; x-- {
			doBreak, limLeft = makeTransparent(x, y, blurRadius, img, blurImg, newImg, isBorder, limLeft)
			if doBreak {
				break
			}
		}
	}

	limDown := limiter{allowable: 1, midPoint: bounds.Dy(), dir: dirDown}
	limUp := limiter{allowable: 1, midPoint: bounds.Dy(), dir: dirUp}
	for x := 0; x < bounds.Dx(); x++ {
		for y := 0; y < bounds.Dy(); y++ {
			doBreak, limDown = makeTransparent(x, y, blurRadius, img, blurImg, newImg, isBorder, limDown)
			if doBreak {
				break
			}
		}

		for y := bounds.Dy(); y > 0; y-- {
			doBreak, limUp = makeTransparent(x, y, blurRadius, img, blurImg, newImg, isBorder, limUp)
			if doBreak {
				break
			}
		}
	}

	return newImg, nil
}

func makeTransparent(x, y, blurRadius int, oImg image.Image, bImg, nImg *image.RGBA, isBorder func(color.Color) bool, lim limiter) (bool, limiter) {
	if isBorder(bImg.At(x, y)) {
		nImg.Set(x, y, color.Transparent)
	} else {
		for ex := 0; ex < blurRadius; ex++ {
			switch lim.dir {
			case dirDown:
				nImg.Set(x, y+ex, color.Transparent)
			case dirLeft:
				nImg.Set(x-ex, y, color.Transparent)
			case dirUp:
				nImg.Set(x, y-ex, color.Transparent)
			case dirRight:
				nImg.Set(x+ex, y, color.Transparent)
			}
		}

		switch lim.dir {
		case dirDown:
			lim.prev = y
			if y > lim.midPoint {
				lim.foundEdge = true
				fmt.Println("found edge while going down (from left)")
			}
		case dirLeft:
			lim.prev = x
			if x < lim.midPoint {
				lim.foundEdge = true
			}
		case dirUp:
			lim.prev = y
			if y < lim.midPoint {
				lim.foundEdge = true
			}
		case dirRight:
			lim.prev = x
			if x > lim.midPoint {
				lim.foundEdge = true
			}
		}

		return true, lim
	}
	if lim.foundEdge {
		switch lim.dir {
		case dirDown:
			if y > lim.prev+lim.allowable {
				break
			}
		case dirLeft:
			if x < lim.prev-lim.allowable {
				break
			}
		case dirUp:
			if y < lim.prev-lim.allowable {
				break
			}
		case dirRight:
			if x > lim.prev+lim.allowable {
				break
			}
		}
	}
	return false, lim
}
