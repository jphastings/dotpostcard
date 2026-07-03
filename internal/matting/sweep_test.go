package matting_test

// Temporary parameter-tuning harness: SWEEP=1 go test -run TestSweep -v ./internal/matting/

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/jphastings/dotpostcard/internal/matting"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
)

func TestSweep(t *testing.T) {
	if os.Getenv("SWEEP") == "" {
		t.Skip("set SWEEP=1 to run the tuning sweep")
	}

	cases := []struct {
		scan, truth string
		band        int
	}{
		{"matte-sin-city-scan.jpeg", "matte-sin-city-alpha.png", 12},
		{"matte-seattle-scan.jpeg", "matte-seattle-alpha.png", 6},
	}

	for _, c := range cases {
		scan := testhelpers.TestImages[c.scan]
		truthImg := testhelpers.TestImages[c.truth]
		b := truthImg.Bounds()
		w, h := b.Dx(), b.Dy()
		truth := make([]float64, w*h)
		mask := make([]bool, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				g, _, _, _ := truthImg.At(b.Min.X+x, b.Min.Y+y).RGBA()
				truth[y*w+x] = float64(g>>8) / 255
				mask[y*w+x] = g>>8 >= 128
			}
		}

		fmt.Printf("== %s (band %d)\n", c.scan, c.band)
		for _, radius := range []int{2, 3, 4, 6, 8, c.band} {
			for _, eps := range []float64{1e-4} {
				got, err := matting.Apply(scan, mask, matting.Options{
					BandHalfWidthPx: c.band, GuidedRadiusPx: radius, Eps: eps,
				})
				if err != nil {
					t.Fatal(err)
				}

				var alphaErr float64
				var n, gotSoft, truthSoft, misclass int
				for i := 0; i < w*h; i++ {
					tA := truth[i]
					gA := float64(got.Pix[i*4+3]) / 255
					if (tA > 0.02 && tA < 0.98) || (gA > 0.02 && gA < 0.98) {
						d := gA - tA
						if d < 0 {
							d = -d
						}
						alphaErr += d
						n++
					}
					if gA > 0.05 && gA < 0.95 {
						gotSoft++
					}
					if tA > 0.05 && tA < 0.95 {
						truthSoft++
					}
					if (gA >= 0.5) != (tA >= 0.5) {
						misclass++
					}
				}
				fmt.Printf("  r=%2d eps=%.0e  meanErr=%.3f  soft got/truth=%d/%d (%.2fx)  misclass=%d (%.3f%% of band %d)\n",
					radius, eps, alphaErr/float64(n), gotSoft, truthSoft, float64(gotSoft)/float64(truthSoft),
					misclass, 100*float64(misclass)/float64(n), n)

				if dir := os.Getenv("SWEEP_DUMP"); dir != "" && radius == 4 {
					vis := image.NewRGBA(image.Rect(0, 0, w, h))
					for i := 0; i < w*h; i++ {
						gA := float64(got.Pix[i*4+3]) / 255
						v := uint8(truth[i] * 200)
						px := color.RGBA{v, v, v, 255}
						if gA >= 0.5 && truth[i] < 0.5 {
							px = color.RGBA{255, 0, 0, 255} // we say card, truth says backboard
						} else if gA < 0.5 && truth[i] >= 0.5 {
							px = color.RGBA{0, 128, 255, 255} // we say backboard, truth says card
						}
						vis.Set(i%w, i/w, px)
					}
					f, _ := os.Create(fmt.Sprintf("%s/misclass-%s.png", dir, c.scan))
					png.Encode(f, vis)
					f.Close()
				}
			}
		}
	}
}
