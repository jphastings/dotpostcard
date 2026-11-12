package fastfoto

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	postcards "github.com/jphastings/dotpostcard"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/pkg/describe"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { pollInterval = 0 }

func runSides(t *testing.T, dir string, sides [][]byte, mutate ...func(*Options)) (Report, error) {
	t.Helper()
	return runScan(t, dir, newFakeScanner(sides, -1, false), mutate...)
}

func runDefaultCard(t *testing.T, dir string, mutate ...func(*Options)) (Report, error) {
	t.Helper()
	return runSides(t, dir, scanOf(defaultSynth(), defaultSynth()), mutate...)
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
	sides := scanOf(back, front)

	report, err := runSides(t, dir, sides)
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
		sides := scanOf(s, s)
		_, err := runSides(t, dir, sides)
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
	sides := scanOf(s, s)
	report, err := runSides(t, dir, sides)
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
	sides := scanOf(s, s)
	report, err := runSides(t, dir, sides)
	require.NoError(t, err)
	assert.InDelta(t, 3.4, report.Cards[0].HeightCm, 0.1)
}

func TestShadowAlongSidesDoesNotSoftenTheEdge(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.sideShadow, s.chromaNoise = true, 4
	sides := scanOf(s, s)
	report, err := runSides(t, dir, sides)
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
	sides := scanOf(s, s)
	_, err := runSides(t, dir, sides)
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
	sides := scanOf(back, front)
	report, err := runSides(t, dir, sides)
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
	sides := scanOf(back, front)
	_, err := runSides(t, dir, sides)
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
	sides := scanOf(s, s)
	report, err := runSides(t, dir, sides)
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
	sides := scanOf(s, s)
	_, err := runSides(t, dir, sides)
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
	sides := scanOf(back, front)
	_, err := runSides(t, dir, sides)
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
	sides := scanOf(s, s)
	report, err := runSides(t, dir, sides)
	require.NoError(t, err)
	assert.InDelta(t, 5.0, report.Cards[0].WidthCm, 0.1)
	assert.Contains(t, strings.Join(report.Cards[0].Flags, "\n"), "uncertain")
}

func TestSkewedCardIsSetAsideForRescan(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.heightCm = 4
	s.tiltDeg = 6
	sides := scanOf(s, s)
	described := false
	report, err := runSides(t, dir, sides, func(o *Options) {
		o.Describe = func(context.Context, image.Image, image.Image) (describe.Description, error) {
			described = true
			return describe.Description{}, nil
		}
	})
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
}

func TestTiltedCardInACroppedFrameIsMeasuredWhole(t *testing.T) {
	dir := t.TempDir()
	sides := scanOf(croppedSynth(5, 3.4, 2.5, 115), croppedSynth(5, 3.4, -2.5, 115))
	report, err := runSides(t, dir, sides)
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
	sides := scanOf(s, s)
	report, err := runSides(t, dir, sides)
	require.NoError(t, err)
	assert.Empty(t, report.Cards[0].Flags)
	assert.False(t, report.Cards[0].Rescan)
}

func TestDarkRowsAtFrameTopAndBottomAreNotTheCard(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	s.frameBandPx = 8
	sides := scanOf(s, s)
	report, err := runSides(t, dir, sides)
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
	sides := scanOf(s, s)
	report, err := runSides(t, dir, sides)
	require.NoError(t, err)
	assert.Contains(t, strings.Join(report.Cards[0].Flags, "\n"), "double-feed")
}

func TestTopSideChoosesWhichScanIsTheFront(t *testing.T) {
	red, blue := defaultSynth(), defaultSynth()
	red.paper, blue.paper = color.RGBA{230, 60, 60, 255}, color.RGBA{60, 60, 230, 255}

	centre := func(dir, name string) color.NRGBA {
		img := loadPNG(t, filepath.Join(dir, name))
		return img.NRGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	}

	dir := t.TempDir()
	sides := scanOf(red, blue)
	_, err := runSides(t, dir, sides)
	require.NoError(t, err)
	assert.Greater(t, centre(dir, "0001-front.png").B, uint8(200), "second scan is the front by default")

	dir = t.TempDir()
	sides = scanOf(red, blue)
	_, err = runSides(t, dir, sides, func(o *Options) { o.TopSideFront = true })
	require.NoError(t, err)
	assert.Greater(t, centre(dir, "0001-front.png").R, uint8(200))
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
	opts := Options{OutDir: dir, Open: func() (Transport, error) { return scanner, nil }}
	for _, m := range mutate {
		m(&opts)
	}
	return Run(context.Background(), opts)
}

func TestScannedSidesBecomeACardWhateverOrderTheirMarkersArrive(t *testing.T) {
	for _, typAfterPst := range []bool{false, true} {
		dir := t.TempDir()
		red, blue := defaultSynth(), defaultSynth()
		red.paper, blue.paper = color.RGBA{230, 60, 60, 255}, color.RGBA{60, 60, 230, 255}
		scanner := newFakeScanner(scanOf(red, blue), -1, typAfterPst)
		report, err := runScan(t, dir, scanner)
		require.NoError(t, err)
		require.Len(t, report.Cards, 1)
		assert.True(t, scanner.sentFIN())
		assert.InDelta(t, 3.4, report.Cards[0].HeightCm, 0.1, "the scanner's claimed height is replaced by the real one")

		img := loadPNG(t, filepath.Join(dir, "0001-front.png"))
		assert.Greater(t, img.NRGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2).B, uint8(200), "side B is the front by default")
	}
}

func TestJamKeepsGoodCardsDropsTheLoneSideThenLaterStacksContinue(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	scanner := newFakeScanner(scanOf(s, s, s, s), 3, false)
	report, err := runScan(t, dir, scanner)

	require.ErrorIs(t, err, ErrJam)
	assert.True(t, scanner.sentFIN(), "the scanner is released even after a failure")
	require.Len(t, report.Cards, 1)
	assert.True(t, report.Unpaired)
	assert.FileExists(t, filepath.Join(dir, "0001-meta.yaml"))
	assert.NoFileExists(t, filepath.Join(dir, "0002-meta.yaml"))

	before, err := os.ReadFile(filepath.Join(dir, "0001-front.png"))
	require.NoError(t, err)

	next, err := runScan(t, dir, newFakeScanner(scanOf(s, s), -1, false))
	require.NoError(t, err)
	require.Len(t, next.Cards, 1)
	assert.Equal(t, 2, next.Cards[0].Number)
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
	assert.False(t, report.Unpaired)
	assert.NotContains(t, scanner.Sent, "IMG", "nothing is fed once stopped")
	assert.True(t, scanner.sentFIN())
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

func TestErrorSharingAReplyWithTheEndOfASideStillDeliversThatSide(t *testing.T) {
	dir := t.TempDir()
	jpegA := defaultSynth().jpegBytes()
	scanner := &fakeScanner{replies: sideReplies(jpegA, "A", "errADF PJ")}
	report, err := runScan(t, dir, scanner)
	require.ErrorIs(t, err, ErrJam)
	assert.True(t, report.Unpaired, "the side arrived, but has no partner")
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
	good := defaultSynth().jpegBytes()
	truncated := good[:len(good)/2]
	report, err := runScan(t, dir, newFakeScanner([][]byte{truncated, good, good}, 2, false))
	require.ErrorIs(t, err, ErrJam)
	assert.ErrorContains(t, err, "1 postcard had a problem; see above")
	require.Len(t, report.Cards, 1)
	assert.Error(t, report.Cards[0].Err)
}

func TestCreatedFilesAreWorldReadable(t *testing.T) {
	dir := t.TempDir()
	_, err := runDefaultCard(t, dir)
	require.NoError(t, err)
	info, err := os.Stat(filepath.Join(dir, "0001-front.png"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestUntiltedCardKeepsItsExactSourcePixels(t *testing.T) {
	dir := t.TempDir()
	s := defaultSynth()
	sides := scanOf(s, s)
	_, err := runSides(t, dir, sides)
	require.NoError(t, err)

	patched, err := patchHeight(sides[1], s.h)
	require.NoError(t, err)
	src, err := decodeFrame(bytes.NewReader(patched))
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

func smallCard() [][]byte {
	s := croppedSynth(5, 3.4, 0, 115)
	return scanOf(s, s)
}

func componentFiles(t *testing.T, dir string) []string {
	t.Helper()
	got, err := filepath.Glob(filepath.Join(dir, "0001-*"))
	require.NoError(t, err)
	return got
}

func TestFormatsWithoutComponentLeaveNoComponentsOrTempDirInOutDir(t *testing.T) {
	dir := t.TempDir()
	_, err := runSides(t, dir, smallCard(), func(o *Options) { o.Formats = []string{"web"} })
	require.NoError(t, err)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "0001.postcard.jpeg", entries[0].Name())
}

func TestFailedConversionKeepsTheStagedComponentsInOutDir(t *testing.T) {
	dir := t.TempDir()
	report, err := runSides(t, dir, smallCard(), func(o *Options) {
		o.Formats = []string{"web"}
		// Taking the output's name after the card is numbered makes its conversion fail.
		o.Describe = func(context.Context, image.Image, image.Image) (describe.Description, error) {
			return describe.Description{}, os.Mkdir(filepath.Join(dir, "0001.postcard.jpeg"), 0o755)
		}
	})
	require.Error(t, err)
	assert.Contains(t, report.Cards[0].ConvertErr.Error(), "kept in "+dir)
	assert.Len(t, componentFiles(t, dir), 3)
	hidden, _ := filepath.Glob(filepath.Join(dir, ".fastfoto-*"))
	assert.Empty(t, hidden)
}

func TestNumberingContinuesAfterAnyFileAlreadyInOutDir(t *testing.T) {
	for _, existing := range []string{"0003-meta.yaml", "0003.postcard.webp", "0003.html", "for-rescan-0003.jpg"} {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, existing), nil, 0o644))
		report, err := runSides(t, dir, smallCard())
		require.NoError(t, err)
		require.Len(t, report.Cards, 1)
		assert.Equal(t, 4, report.Cards[0].Number, existing)
		assert.FileExists(t, filepath.Join(dir, "0004-front.png"))
	}
}

func TestALoneTrailingSideIsReportedAndNotWritten(t *testing.T) {
	dir := t.TempDir()
	s := croppedSynth(5, 3.4, 0, 115)
	report, err := runScan(t, dir, newFakeScanner(scanOf(s, s, s), -1, false))
	require.Error(t, err)
	assert.True(t, report.Unpaired)
	require.Len(t, report.Cards, 1)
	assert.NoFileExists(t, filepath.Join(dir, "0002-front.png"))
}

func TestFormatsWithComponentKeepComponentsInOutDir(t *testing.T) {
	dir := t.TempDir()

	_, err := runSides(t, dir, smallCard(), func(o *Options) { o.Formats = []string{"web", "component"} })
	require.NoError(t, err)

	assert.Len(t, componentFiles(t, dir), 3)
	written, _ := filepath.Glob(filepath.Join(dir, "0001.postcard.*"))
	assert.Len(t, written, 1)
}

func TestFormatsConvertEachBuiltCard(t *testing.T) {
	dir := t.TempDir()

	rep, err := runSides(t, dir, smallCard(), func(o *Options) { o.Formats = []string{"web"} })
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
	css := filepath.Join(dir, "postcards.css")
	require.NoError(t, os.WriteFile(css, []byte("hand-edited"), 0o644))

	_, err := runSides(t, dir, smallCard(), func(o *Options) { o.Formats = []string{"web", "css"} })
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

func TestArchiveWebOutputIsLosslessWebP(t *testing.T) {
	dir := t.TempDir()
	_, err := runSides(t, dir, smallCard(), func(o *Options) { o.Formats = []string{"web"}; o.Quality = formats.Archive })
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "0001.postcard.webp"), "JPEG can't be lossless")
}

func TestOnCardReportsEachOutputWritten(t *testing.T) {
	var cards []CardResult
	started := -1
	_, err := runSides(t, t.TempDir(), smallCard(), func(o *Options) {
		o.Formats = []string{"web"}
		o.OnStart = func(n int) { started = n }
		o.OnCard = func(c CardResult) { cards = append(cards, c) }
	})
	require.NoError(t, err)

	assert.Equal(t, 1, started)
	require.Len(t, cards, 1)
	require.Len(t, cards[0].Outputs, 1)
	out := cards[0].Outputs[0]
	assert.Equal(t, "Web", out.Format)
	assert.Contains(t, out.Path, "0001.postcard.")
	assert.NotZero(t, out.Bytes)
}

func TestOnScannedFiresOncePerCardFed(t *testing.T) {
	scanned := 0
	_, err := runSides(t, t.TempDir(), smallCard(), func(o *Options) {
		o.Formats = []string{"web"}
		o.OnScanned = func() { scanned++ }
	})
	require.NoError(t, err)
	assert.Equal(t, 1, scanned)
}

func TestOnStartIsNotCalledForAnUnknownFormat(t *testing.T) {
	called := false
	_, err := runScan(t, t.TempDir(), newFakeScanner(nil, 0, false), func(o *Options) {
		o.Formats = []string{"nonsense"}
		o.OnStart = func(int) { called = true }
	})
	assert.Error(t, err)
	assert.False(t, called)
}

func cardReplies(jpeg []byte, cards int) []fakeReply {
	var replies []fakeReply
	for i := 0; i < cards; i++ {
		replies = append(replies, sideReplies(jpeg, "A")...)
		replies = append(replies, sideReplies(jpeg, "B")...)
	}
	return replies
}

func TestStopFinishesTheCardInTheScannerThenCancelsAndReleases(t *testing.T) {
	cases := map[string]struct {
		stopAtIMG int
		wantCards int
	}{
		"mid-card":                      {stopAtIMG: 2, wantCards: 1},
		"as the next card's side opens": {stopAtIMG: 5, wantCards: 2},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			stop, finish := context.WithCancel(context.Background())
			scanner := &fakeScanner{replies: cardReplies(defaultSynth().jpegBytes(), 3)}
			scanner.onIMG = func(n int) {
				if n == c.stopAtIMG {
					finish()
				}
			}
			report, err := runScan(t, dir, scanner, func(o *Options) { o.Stop = stop })

			require.NoError(t, err)
			assert.Len(t, report.Cards, c.wantCards)
			assert.False(t, report.Interrupted)
			assert.Equal(t, []string{"CAN", "FIN"}, scanner.Sent[len(scanner.Sent)-2:])
			assert.LessOrEqual(t, scanner.imgs, c.wantCards*4+1, "no more of the stack is scanned")
			assert.NoFileExists(t, filepath.Join(dir, fmt.Sprintf("%04d-meta.yaml", c.wantCards+1)))
		})
	}
}

func TestAbortCancelsReleasesAndDropsUnwrittenCardsQuietly(t *testing.T) {
	dir := t.TempDir()
	ctx, abort := context.WithCancel(context.Background())
	scanner := &fakeScanner{replies: cardReplies(defaultSynth().jpegBytes(), 2)}
	scanner.onIMG = func(n int) {
		if n == 5 { // card 1 is with the LLM, card 2's first side has just opened
			abort()
		}
	}
	report, err := Run(ctx, Options{
		OutDir: dir,
		Open:   func() (Transport, error) { return scanner, nil },
		Describe: func(ctx context.Context, _, _ image.Image) (describe.Description, error) {
			<-ctx.Done()
			return describe.Description{}, ctx.Err()
		},
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"CAN", "FIN"}, scanner.Sent[len(scanner.Sent)-2:])
	assert.True(t, report.Interrupted)
	assert.True(t, report.StoppedMidCard)
	assert.Equal(t, 1, report.Discarded)
	assert.Empty(t, report.Cards, "a dropped card isn't reported as written, with or without a warning")
	assert.Empty(t, componentFiles(t, dir))
}
