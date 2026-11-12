package fastfoto

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	postcards "github.com/jphastings/dotpostcard"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { pollInterval = 0 }

func writeRaw(t *testing.T, dir string, page int, s synth) {
	t.Helper()
	raw := filepath.Join(dir, "raw")
	require.NoError(t, os.MkdirAll(raw, 0o755))
	b, err := patchHeight(s.jpegBytes(), s.h)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(raw, rawName(page)), b, 0o644))
}

func runNoScan(t *testing.T, dir string, mutate ...func(*Options)) (Report, error) {
	t.Helper()
	opts := Options{OutDir: dir, NoScan: true}
	for _, m := range mutate {
		m(&opts)
	}
	return Run(context.Background(), opts)
}

func loadPNG(t *testing.T, path string) *image.NRGBA {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	img, err := png.Decode(f)
	require.NoError(t, err)
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	return out
}

func alpha(img *image.NRGBA, x, y int) uint8 { return img.NRGBAAt(x, y).A }

func TestCardRoundTripsThroughPostcardsTool(t *testing.T) {
	dir := t.TempDir()
	back, front := defaultSynth(), defaultSynth()
	back.tiltDeg, front.tiltDeg = 1, -1
	writeRaw(t, dir, 1, back)
	writeRaw(t, dir, 2, front)

	report, err := runNoScan(t, dir)
	require.NoError(t, err)
	require.Len(t, report.Cards, 1)

	bundles, err := postcards.MakeBundles([]string{filepath.Join(dir, "0001-front.png"), filepath.Join(dir, "0001-back.png"), filepath.Join(dir, "0001-meta.yaml")})
	require.NoError(t, err)
	require.Len(t, bundles, 1)
	pc, err := bundles[0].Decode(formats.DecodeOptions{})
	require.NoError(t, err)

	assert.Equal(t, types.FlipBook, pc.Meta.Flip)
	assert.Equal(t, pc.Front.Bounds().Size(), pc.Back.Bounds().Size(), "both sides share one pixel size")
	w, h := pc.Meta.Physical.FrontDimensions.MustPhysical()
	assert.InDelta(t, 5.0, w, 0.05)
	assert.InDelta(t, 3.4, h, 0.1)
	assert.Empty(t, report.Cards[0].Flags)
}

func TestTiltedCardsComeOutStraight(t *testing.T) {
	for _, tilt := range []float64{-3, 3} {
		dir := t.TempDir()
		s := defaultSynth()
		s.tiltDeg = tilt
		writeRaw(t, dir, 1, s)
		writeRaw(t, dir, 2, s)
		_, err := runNoScan(t, dir)
		require.NoError(t, err)

		img := loadPNG(t, filepath.Join(dir, "0001-front.png"))
		b := img.Bounds()
		lo, hi := b.Dx(), 0
		for y := b.Dy() / 5; y < b.Dy()*4/5; y++ {
			for x := 0; x < b.Dx(); x++ {
				if alpha(img, x, y) >= 128 {
					lo, hi = min(lo, x), max(hi, x)
					break
				}
			}
		}
		assert.LessOrEqualf(t, hi-lo, 2, "left edge wobbles by %d px at %v degrees", hi-lo, tilt)
	}
}

func TestStockCloseToBackingInBrightnessIsStillCard(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.paper = color.RGBA{200, 170, 142, 255} // same luminance as the backing, very different colour
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	report, err := runNoScan(t, dir)
	require.NoError(t, err)

	assert.InDelta(t, 5.0, report.Cards[0].WidthCm, 0.1)
	assert.InDelta(t, 3.4, report.Cards[0].HeightCm, 0.1)
	img := loadPNG(t, filepath.Join(dir, "0001-front.png"))
	assert.Equal(t, uint8(255), alpha(img, img.Bounds().Dx()/2, img.Bounds().Dy()/2))
}

func TestShadowBeyondTheEdgeIsNotCard(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.shadow = true
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	report, err := runNoScan(t, dir)
	require.NoError(t, err)
	assert.InDelta(t, 3.4, report.Cards[0].HeightCm, 0.1)
}

func TestShadowAlongSidesDoesNotSoftenTheEdge(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.sideShadow, s.chromaNoise = true, 4
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	report, err := runNoScan(t, dir)
	require.NoError(t, err)
	assert.InDelta(t, 5.0, report.Cards[0].WidthCm, 0.15)

	img := loadPNG(t, filepath.Join(dir, "0001-front.png"))
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	for _, y := range []int{h / 4, h / 2, 3 * h / 4} {
		for _, fromRight := range []bool{false, true} {
			at := func(i int) uint8 {
				if fromRight {
					return alpha(img, w-1-i, y)
				}
				return alpha(img, i, y)
			}
			first, opaque := -1, -1
			for i := 0; i < w/2 && opaque < 0; i++ {
				if first < 0 && at(i) >= trimAlpha {
					first = i
				}
				if at(i) == 255 {
					opaque = i
				}
			}
			assert.LessOrEqualf(t, opaque-first, 16, "row %d, fromRight=%v: alpha takes %d px to become opaque", y, fromRight, opaque-first)
		}
	}
}

func TestPinkedEdgeKeepsItsTeethOnBothSides(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.zigzagPx = 14
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	_, err := runNoScan(t, dir)
	require.NoError(t, err)

	for _, side := range []string{"front", "back"} {
		img := loadPNG(t, filepath.Join(dir, "0001-"+side+".png"))
		b := img.Bounds()
		lastOpaque := func(x int) int {
			for y := b.Dy() - 1; y >= 0; y-- {
				if alpha(img, x, y) >= 128 {
					return y
				}
			}
			return -1
		}
		lo, hi := b.Dy(), 0
		for x := b.Dx() / 4; x < b.Dx()*3/4; x++ {
			lo, hi = min(lo, lastOpaque(x)), max(hi, lastOpaque(x))
		}
		assert.GreaterOrEqual(t, hi-lo, 10, "%s: tooth depth should survive, not be smoothed flat", side)
	}
}

func TestDarkFrontBesideShadowBandsFollowsTheLightBack(t *testing.T) {
	dir := t.TempDir()
	back, front := defaultSynth(), defaultSynth()
	front.paper = color.RGBA{55, 55, 55, 255} // no step between this and the band beside it
	back.bandPx, front.bandPx = [2]int{14, 14}, [2]int{14, 14}
	writeRaw(t, dir, 1, back)
	writeRaw(t, dir, 2, front)
	report, err := runNoScan(t, dir)
	require.NoError(t, err)
	assert.Empty(t, report.Cards[0].Flags)

	for _, side := range []string{"front", "back"} {
		img := loadPNG(t, filepath.Join(dir, "0001-"+side+".png"))
		for _, y := range []int{img.Bounds().Dy() / 4, img.Bounds().Dy() / 2} {
			first, last := -1, -1
			for x := 0; x < img.Bounds().Dx(); x++ {
				if alpha(img, x, y) >= 128 {
					last = x
					if first < 0 {
						first = x
					}
				}
			}
			assert.InDeltaf(t, 5*pxPerCm, float64(last-first+1), 3, "%s row %d: shadow fringe widens, a bite narrows", side, y)
		}
	}
}

func TestSidesOffsetByAShadowBandAreRegistered(t *testing.T) {
	dir := t.TempDir()
	back, front := defaultSynth(), defaultSynth()
	front.bandPx = [2]int{14, 0} // widens only the front's measured box, so its canvas sits 7px off the back's
	writeRaw(t, dir, 1, back)
	writeRaw(t, dir, 2, front)
	_, err := runNoScan(t, dir)
	require.NoError(t, err)

	img := loadPNG(t, filepath.Join(dir, "0001-front.png"))
	y := img.Bounds().Dy() / 2
	first, last := -1, -1
	for x := 0; x < img.Bounds().Dx(); x++ {
		if alpha(img, x, y) == 255 {
			last = x
			if first < 0 {
				first = x
			}
		}
	}
	assert.Greater(t, img.NRGBAAt(first+2, y).R, uint8(200), "the front's left edge is paper, not the band")
	assert.Greater(t, img.NRGBAAt(last-2, y).R, uint8(200), "the front's right edge is paper")
	assert.InDelta(t, 5*pxPerCm, float64(last-first+1), 3, "no paper cut off at either side")
}

func TestBackingShadowedButRedderIsNotCard(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.redShadow = true
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	report, err := runNoScan(t, dir)
	require.NoError(t, err)

	img := loadPNG(t, filepath.Join(dir, "0001-front.png"))
	assert.InDelta(t, 5*pxPerCm, float64(img.Bounds().Dx()), 5)
	assert.InDelta(t, 3.4*pxPerCm, float64(img.Bounds().Dy()), 5)
	assert.Empty(t, report.Cards[0].Flags)
}

func TestPrintingGreyerThanBackingUpToTheEdgeIsStillCard(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.edgeDots = true
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	_, err := runNoScan(t, dir)
	require.NoError(t, err)

	img := loadPNG(t, filepath.Join(dir, "0001-front.png"))
	assert.InDelta(t, 3.4*pxPerCm, float64(img.Bounds().Dy()), 5)
	for x := img.Bounds().Dx() / 4; x < img.Bounds().Dx()*3/4; x++ {
		require.GreaterOrEqualf(t, alpha(img, x, 15), uint8(250), "column %d", x)
	}
}

func TestFrontAlphaIsTheBacksMirrored(t *testing.T) {
	dir := t.TempDir()
	back, front := defaultSynth(), defaultSynth()
	back.tiltDeg, front.tiltDeg = 1, -1
	front.bandPx = [2]int{14, 0}
	back.zigzagPx = 14
	writeRaw(t, dir, 1, back)
	writeRaw(t, dir, 2, front)
	_, err := runNoScan(t, dir)
	require.NoError(t, err)

	f, b := loadPNG(t, filepath.Join(dir, "0001-front.png")), loadPNG(t, filepath.Join(dir, "0001-back.png"))
	require.Equal(t, f.Bounds(), b.Bounds())
	w := f.Bounds().Dx()
	for y := 0; y < f.Bounds().Dy(); y++ {
		for x := 0; x < w; x++ {
			require.Equalf(t, alpha(f, x, y), alpha(b, w-1-x, y), "alpha at %d,%d", x, y)
		}
	}
}

func TestCardDarkOnBothSidesFallsBackAndFlagsUncertainEdges(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.paper = color.RGBA{50, 50, 50, 255}
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	report, err := runNoScan(t, dir)
	require.NoError(t, err)
	assert.InDelta(t, 5.0, report.Cards[0].WidthCm, 0.1)
	assert.Contains(t, strings.Join(report.Cards[0].Flags, "\n"), "uncertain")
}

func TestSkewedCardIsSetAsideForRescan(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.heightCm = 4
	s.tiltDeg = 6
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	described := false
	opts := Options{OutDir: dir, NoScan: true, Describe: func(context.Context, image.Image, image.Image) (Description, error) {
		described = true
		return Description{}, nil
	}}
	report, err := Run(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, report.Cards[0].Rescan)
	assert.False(t, described)

	f, err := os.Open(filepath.Join(dir, "for-rescan-0001.jpg"))
	require.NoError(t, err)
	defer f.Close()
	_, err = jpeg.Decode(f)
	assert.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(dir, "0001-front.png"))
	assert.NoFileExists(t, filepath.Join(dir, "0001-meta.yaml"))

	report, err = Run(context.Background(), opts)
	require.NoError(t, err)
	assert.Empty(t, report.Cards)
}

func TestTiltedCardInACroppedFrameIsMeasuredWhole(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, 1, croppedSynth(5, 3.4, 2.5, 115))
	writeRaw(t, dir, 2, croppedSynth(5, 3.4, -2.5, 115))
	report, err := runNoScan(t, dir)
	require.NoError(t, err)
	require.Len(t, report.Cards, 1)
	assert.False(t, report.Cards[0].Rescan)
	assert.Empty(t, report.Cards[0].Flags)
	assert.InDelta(t, 5.0, report.Cards[0].WidthCm, 0.1)
	assert.InDelta(t, 3.4, report.Cards[0].HeightCm, 0.1)

	front := loadPNG(t, filepath.Join(dir, "0001-front.png"))
	b := front.Bounds()
	for _, corner := range [][2]int{{12, 12}, {b.Dx() - 13, 12}, {12, b.Dy() - 13}, {b.Dx() - 13, b.Dy() - 13}} {
		assert.Equalf(t, uint8(255), alpha(front, corner[0], corner[1]), "corner %v is cut off", corner)
	}
}

func TestSlightlySkewedCardIsNotWarnedAbout(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.tiltDeg = 1
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	report, err := runNoScan(t, dir)
	require.NoError(t, err)
	assert.Empty(t, report.Cards[0].Flags)
	assert.False(t, report.Cards[0].Rescan)
}

func TestDarkRowsAtFrameTopAndBottomAreNotTheCard(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.frameBandPx = 8
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	report, err := runNoScan(t, dir)
	require.NoError(t, err)
	assert.Empty(t, report.Cards[0].Flags)
	assert.InDelta(t, 5.0, report.Cards[0].WidthCm, 0.1)
	assert.InDelta(t, 3.4, report.Cards[0].HeightCm, 0.1)
}

func TestShingledDoubleFeedIsFlagged(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.w, s.h = 3200, 1700
	s.doubleFeed = true
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	report, err := runNoScan(t, dir)
	require.NoError(t, err)
	assert.Contains(t, strings.Join(report.Cards[0].Flags, "\n"), "double-feed")
}

func TestFirstSideChoosesWhichPageIsTheFront(t *testing.T) {
	red, blue := defaultSynth(), defaultSynth()
	red.paper, blue.paper = color.RGBA{230, 60, 60, 255}, color.RGBA{60, 60, 230, 255}

	centre := func(dir, name string) color.NRGBA {
		img := loadPNG(t, filepath.Join(dir, name))
		return img.NRGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	}

	dir := t.TempDir()
	writeRaw(t, dir, 1, red)
	writeRaw(t, dir, 2, blue)
	_, err := runNoScan(t, dir)
	require.NoError(t, err)
	assert.Greater(t, centre(dir, "0001-front.png").B, uint8(200), "second page is the front by default")

	dir = t.TempDir()
	writeRaw(t, dir, 1, red)
	writeRaw(t, dir, 2, blue)
	_, err = runNoScan(t, dir, func(o *Options) { o.FirstSideFront = true })
	require.NoError(t, err)
	assert.Greater(t, centre(dir, "0001-front.png").R, uint8(200))
}

func TestNoScanOnlyFinishesUnfinishedCards(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	first, err := runNoScan(t, dir)
	require.NoError(t, err)
	require.Len(t, first.Cards, 1)

	again, err := runNoScan(t, dir)
	require.NoError(t, err)
	assert.Empty(t, again.Cards, "a finished card is left alone")

	require.NoError(t, os.Remove(filepath.Join(dir, "0001-meta.yaml")))
	resumed, err := runNoScan(t, dir)
	require.NoError(t, err)
	assert.Len(t, resumed.Cards, 1, "a card without metadata is redone")
}

func scanOf(sides ...synth) [][]byte {
	var out [][]byte
	for _, s := range sides {
		out = append(out, s.jpegBytes())
	}
	return out
}

func runScan(t *testing.T, dir string, scanner *fakeScanner, mutate ...func(*Options)) (Report, error) {
	t.Helper()
	return runNoScanWith(dir, func(o *Options) {
		o.NoScan = false
		o.Open = func() (Transport, error) { return scanner, nil }
		for _, m := range mutate {
			m(o)
		}
	})
}

func runNoScanWith(dir string, mutate func(*Options)) (Report, error) {
	opts := Options{OutDir: dir}
	mutate(&opts)
	return Run(context.Background(), opts)
}

func TestScannedSidesAreSavedWithTheirRealHeight(t *testing.T) {
	for _, typAfterPst := range []bool{false, true} {
		dir := t.TempDir()
		s, red, blue := defaultSynth(), defaultSynth(), defaultSynth()
		red.paper, blue.paper = color.RGBA{230, 60, 60, 255}, color.RGBA{60, 60, 230, 255}
		scanner := newFakeScanner(scanOf(red, blue), -1, typAfterPst)
		report, err := runScan(t, dir, scanner)
		require.NoError(t, err)
		require.Len(t, report.Cards, 1)
		assert.True(t, scanner.sentFIN())

		for page, wantRed := range map[int]bool{1: true, 2: false} {
			raw, err := os.ReadFile(filepath.Join(dir, "raw", rawName(page)))
			require.NoError(t, err)
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
			require.NoError(t, err)
			assert.Equal(t, s.h, cfg.Height)
			fr, err := decodeFrame(bytes.NewReader(raw))
			require.NoError(t, err)
			centre := fr.at(fr.w/2, fr.h/2)
			assert.Equal(t, wantRed, centre[0] > centre[2], "side A is page 1, side B page 2")
		}
	}
}

func TestJamKeepsGoodCardsAndTheOrphanThenLaterStacksContinue(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	scanner := newFakeScanner(scanOf(s, s, s, s), 3, false)
	report, err := runScan(t, dir, scanner)

	require.ErrorIs(t, err, ErrJam)
	assert.True(t, scanner.sentFIN(), "the scanner is released even after a failure")
	require.Len(t, report.Cards, 1)
	assert.Equal(t, []int{3}, report.Orphans)
	assert.FileExists(t, filepath.Join(dir, "0001-meta.yaml"))
	assert.FileExists(t, filepath.Join(dir, "raw", rawName(3)))

	before, err := os.ReadFile(filepath.Join(dir, "0001-front.png"))
	require.NoError(t, err)

	next, err := runScan(t, dir, newFakeScanner(scanOf(s, s), -1, false))
	require.NoError(t, err)
	require.Len(t, next.Cards, 1)
	assert.Equal(t, 3, next.Cards[0].Number, "card numbers carry on past the orphan's")
	after, err := os.ReadFile(filepath.Join(dir, "0001-front.png"))
	require.NoError(t, err)
	assert.Equal(t, before, after, "earlier cards are never overwritten")
}

func TestStoppingBeforeFeedingLeavesNothingHalfScanned(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	scanner := newFakeScanner(scanOf(s, s, s, s), -1, false)
	stop, finish := context.WithCancel(context.Background())
	finish()
	report, err := runScan(t, dir, scanner, func(o *Options) { o.Stop = stop })
	require.NoError(t, err)
	assert.Empty(t, report.Cards)
	assert.Empty(t, report.Orphans)
	assert.NotContains(t, scanner.Sent, "IMG", "nothing is fed once stopped")
	assert.True(t, scanner.sentFIN())
}

func TestSizesMeasuredAlongTheFeedAreScaled(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	report, err := runNoScan(t, dir, func(o *Options) { o.FeedScale = 1.25 })
	require.NoError(t, err)
	assert.InDelta(t, 3.4*1.25, report.Cards[0].HeightCm, 0.1)
	assert.InDelta(t, 5.0, report.Cards[0].WidthCm, 0.1)
}

func TestStaleRepliesFromAnEarlierSessionDoNotStopARun(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	scanner := newFakeScanner(scanOf(s, s), -1, false)
	scanner.out.WriteString("stale reply left by a bad session")

	report, err := runScan(t, dir, scanner)
	require.NoError(t, err)
	assert.Len(t, report.Cards, 1)
	assert.Equal(t, []string{"FIN", "LOCK"}, scanner.Sent[:2])
}

func TestScannerErrorMidScanCancelsThenFinishes(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	scanner := newFakeScanner(scanOf(s, s, s), 1, false)
	_, err := runScan(t, dir, scanner)
	require.ErrorIs(t, err, ErrJam)
	assert.Equal(t, []string{"CAN", "FIN"}, scanner.Sent[len(scanner.Sent)-2:])
}

func TestEmptyFeederInTheStatDataStopsBeforeAnySettingsAreSent(t *testing.T) {
	scanner := &fakeScanner{statData: "#ERRADF PE "}
	_, err := runScan(t, t.TempDir(), scanner)
	require.ErrorIs(t, err, ErrFeederEmpty)
	assert.NotContains(t, scanner.Sent, "PARA")
	assert.NotContains(t, scanner.Sent, "TRDT")
}

func sideReplies(jpeg []byte, letter string, extraEnd ...string) []fakeReply {
	return []fakeReply{
		{tokens: []string{"psti0001700d000i0009283", "typIMG" + letter}, data: jpeg},
		{tokens: append([]string{"peni0001700i0000920", "typIMG" + letter}, extraEnd...)},
	}
}

func TestErrorSharingAReplyWithTheEndOfASideStillSavesThatSide(t *testing.T) {
	dir := t.TempDir()
	jpegA := defaultSynth().jpegBytes()
	scanner := &fakeScanner{replies: sideReplies(jpegA, "A", "errADF PJ")}
	_, err := runScan(t, dir, scanner)
	require.ErrorIs(t, err, ErrJam)
	assert.FileExists(t, filepath.Join(dir, "raw", rawName(1)))
}

func TestLostOrMisorderedSidesAreErrors(t *testing.T) {
	j := defaultSynth().jpegBytes()
	cases := map[string][]fakeReply{
		"batch ends with a side open": append(sideReplies(j, "A")[:1], fakeReply{tokens: []string{"lftd000"}}),
		"batch ends after one side":   append(sideReplies(j, "A"), fakeReply{tokens: []string{"lftd000"}}),
		"B arrives first":             sideReplies(j, "B"),
		"two As in a row":             append(sideReplies(j, "A"), sideReplies(j, "A")...),
		"side ends without starting":  {{tokens: []string{"peni0001700i0000920", "typIMGA"}}},
		"side starts twice":           {{tokens: []string{"psti0001700d000i0009283", "typIMGA"}}, {tokens: []string{"psti0001700d000i0009283", "typIMGA"}}},
	}
	for name, replies := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := runScan(t, t.TempDir(), &fakeScanner{replies: replies})
			assert.Error(t, err)
		})
	}
}

func TestFailedCardBuildStillFailsTheRunAfterAScanError(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, 2, defaultSynth())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "raw", rawName(1)), []byte("not a jpeg"), 0o644))
	report, err := runScan(t, dir, newFakeScanner(nil, 0, false))
	require.ErrorIs(t, err, ErrJam)
	assert.ErrorContains(t, err, "card 0001")
	require.Len(t, report.Cards, 1)
	assert.Error(t, report.Cards[0].Err)
}

func TestCreatedFilesAreWorldReadable(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, 1, defaultSynth())
	writeRaw(t, dir, 2, defaultSynth())
	_, err := runNoScan(t, dir)
	require.NoError(t, err)
	info, err := os.Stat(filepath.Join(dir, "0001-front.png"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestNonPositiveFeedScaleIsRejected(t *testing.T) {
	for _, scale := range []float64{-1, math.NaN(), math.Inf(1)} {
		_, err := runNoScan(t, t.TempDir(), func(o *Options) { o.FeedScale = scale })
		assert.Error(t, err)
	}
}

func TestUntiltedCardKeepsItsExactSourcePixels(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
	_, err := runNoScan(t, dir)
	require.NoError(t, err)

	raw, err := os.ReadFile(filepath.Join(dir, "raw", rawName(2)))
	require.NoError(t, err)
	src, err := decodeFrame(bytes.NewReader(raw))
	require.NoError(t, err)
	out := loadPNG(t, filepath.Join(dir, "0001-front.png"))
	b := out.Bounds()
	const run = 200
	row, x0 := b.Dy()/2, b.Dx()/2-run/2

	matches := func(sy, sx int) bool {
		for i := 0; i < run; i++ {
			c, p := out.NRGBAAt(x0+i, row), src.at(sx+i, sy)
			if float64(c.R) != p[0] || float64(c.G) != p[1] || float64(c.B) != p[2] {
				return false
			}
		}
		return true
	}
	found := false
	for sy := 0; sy < src.h && !found; sy++ {
		for sx := 0; sx+run <= src.w && !found; sx++ {
			found = matches(sy, sx)
		}
	}
	assert.True(t, found, "interior pixels are copied, not blended with neighbours")
}

func writeSmallCard(t *testing.T, dir string) {
	t.Helper()
	s := croppedSynth(5, 3.4, 0, 115)
	writeRaw(t, dir, 1, s)
	writeRaw(t, dir, 2, s)
}

func TestFormatsConvertEachBuiltCardNextToItsComponents(t *testing.T) {
	dir := t.TempDir()
	writeSmallCard(t, dir)

	rep, err := runNoScan(t, dir, func(o *Options) { o.Formats = []string{"web"} })
	require.NoError(t, err)
	require.Len(t, rep.Cards, 1)

	written, err := filepath.Glob(filepath.Join(dir, "0001.postcard.*"))
	require.NoError(t, err)
	require.Len(t, written, 1)
	bundles, err := postcards.MakeBundles(written)
	require.NoError(t, err)
	require.Len(t, bundles, 1)
	pc, err := bundles[0].Decode(formats.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, "0001", pc.Name)
}

func TestConvertingLeavesAnExistingSharedFileAlone(t *testing.T) {
	dir := t.TempDir()
	writeSmallCard(t, dir)
	css := filepath.Join(dir, "postcards.css")
	require.NoError(t, os.WriteFile(css, []byte("hand-edited"), 0o644))

	_, err := runNoScan(t, dir, func(o *Options) { o.Formats = []string{"web", "css"} })
	require.NoError(t, err)

	got, err := os.ReadFile(css)
	require.NoError(t, err)
	assert.Equal(t, "hand-edited", string(got))
}

func TestUnknownFormatFailsBeforeScanning(t *testing.T) {
	dir := t.TempDir()
	opened := false
	_, err := runScan(t, dir, newFakeScanner(nil, 0, false), func(o *Options) {
		o.Formats = []string{"web", "nonsense"}
		o.Open = func() (Transport, error) { opened = true; return nil, nil }
	})
	assert.Error(t, err)
	assert.False(t, opened)
}
