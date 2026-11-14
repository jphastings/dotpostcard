package fastfoto

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	postcards "github.com/jphastings/dotpostcard"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/pkg/describe"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func twoSidedDir(t *testing.T) string {
	return t.TempDir()
}

func describing(d describe.Description, err error) func(*Options) {
	return func(o *Options) {
		o.Describe = func(context.Context, image.Image, image.Image) (describe.Description, error) { return d, err }
	}
}

func TestDescriberTitleIsWrittenToMetadataAndNamesTheFiles(t *testing.T) {
	dir := twoSidedDir(t)
	report, err := runDefaultCard(t, dir, describing(describe.Description{
		Title: "Notre-Dame de Paris",
		Front: describe.Side{Rotation: "0"},
		Back:  describe.Side{Rotation: "0"},
	}, nil))
	require.NoError(t, err)
	assert.Equal(t, "notre-dame-de-paris", report.Cards[0].Name)

	b, err := os.ReadFile(filepath.Join(dir, "notre-dame-de-paris-meta.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(b), "title: Notre-Dame de Paris")
}

func TestHeterorientedAnswerTurnsTheSidesAndSetsTheFlip(t *testing.T) {
	dir := twoSidedDir(t)
	report, err := runDefaultCard(t, dir, describing(describe.Description{
		Front: describe.Side{Rotation: "90", Description: "A harbour at dusk", Transcription: "Wish you were here"},
		Back:  describe.Side{Rotation: "0", Description: "A map of the bay"},
	}, nil))
	require.NoError(t, err)
	assert.Equal(t, types.FlipLeftHand, report.Cards[0].Flip)
	assert.True(t, report.Cards[0].Described)

	bundles, err := postcards.MakeBundles([]string{filepath.Join(dir, "0001-front.png"), filepath.Join(dir, "0001-back.png"), filepath.Join(dir, "0001-meta.yaml")})
	require.NoError(t, err)
	pc, err := bundles[0].Decode(formats.DecodeOptions{})
	require.NoError(t, err)

	assert.Equal(t, types.FlipLeftHand, pc.Meta.Flip)
	assert.NoError(t, pc.Validate())
	assert.Greater(t, pc.Front.Bounds().Dy(), pc.Front.Bounds().Dx(), "front is now portrait")
	assert.Greater(t, pc.Back.Bounds().Dx(), pc.Back.Bounds().Dy(), "back stays landscape")
	w, h := pc.Meta.Physical.FrontDimensions.MustPhysical()
	assert.InDelta(t, 3.4, w, 0.1)
	assert.InDelta(t, 5.0, h, 0.1)
	assert.Equal(t, "A harbour at dusk", pc.Meta.Front.Description)
	assert.Equal(t, "Wish you were here", pc.Meta.Front.Transcription.Text)
	assert.Equal(t, "A map of the bay", pc.Meta.Back.Description)
}

func TestDescriberFailureLeavesTheCardAsScanned(t *testing.T) {
	dir := twoSidedDir(t)
	report, err := runDefaultCard(t, dir, describing(describe.Description{}, errors.New("quota")))
	require.NoError(t, err)
	assert.Contains(t, report.Cards[0].Notes[0], "quota")

	bundles, err := postcards.MakeBundles([]string{filepath.Join(dir, "0001-front.png"), filepath.Join(dir, "0001-back.png"), filepath.Join(dir, "0001-meta.yaml")})
	require.NoError(t, err)
	pc, err := bundles[0].Decode(formats.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, types.FlipBook, pc.Meta.Flip)
	assert.Empty(t, pc.Meta.Front.Description)
}

func TestDescriberPanicLeavesTheCardAsScanned(t *testing.T) {
	dir := twoSidedDir(t)
	report, err := runDefaultCard(t, dir, func(o *Options) {
		o.Describe = func(context.Context, image.Image, image.Image) (describe.Description, error) { panic("boom") }
	})
	require.NoError(t, err)
	assert.Contains(t, report.Cards[0].Notes[0], "boom")
	for _, f := range []string{"front.png", "back.png", "meta.yaml"} {
		assert.FileExists(t, filepath.Join(dir, "0001-"+f))
	}
}

func TestClaudeCodeBackendDescribesCardsWithoutScanning(t *testing.T) {
	bin := t.TempDir()
	reply := `{"type":"result","is_error":false,"result":"","structured_output":{"front":{"rotation":"0","description":"A harbour","transcription":"","secrets":[]},"back":{"rotation":"0","description":"","transcription":"","secrets":[]},"recipient":"","sender":"","sent_on":"","locale":"","location":{"name":"","latitude":null,"longitude":null,"country_code":""},"orientation_evidence":"x"}}`
	script := "#!/bin/sh\ncat >/dev/null\ncat <<'EOF'\n" + reply + "\nEOF\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := twoSidedDir(t)
	report, err := runDefaultCard(t, dir, func(o *Options) { o.LLM = describe.Config{Backend: "claude-code"} })
	require.NoError(t, err)
	require.NoError(t, report.Cards[0].Err)
	assert.Empty(t, report.Cards[0].Notes)

	bundles, err := postcards.MakeBundles([]string{filepath.Join(dir, "0001-front.png"), filepath.Join(dir, "0001-back.png"), filepath.Join(dir, "0001-meta.yaml")})
	require.NoError(t, err)
	pc, err := bundles[0].Decode(formats.DecodeOptions{})
	require.NoError(t, err)
	assert.Equal(t, "A harbour", pc.Meta.Front.Description)
}

func TestDescriberGetsTheRunsContext(t *testing.T) {
	dir := twoSidedDir(t)
	_, err := runDefaultCard(t, dir, func(o *Options) {
		o.Describe = func(ctx context.Context, _, _ image.Image) (describe.Description, error) {
			require.NotNil(t, ctx)
			return describe.Description{}, ctx.Err()
		}
	})
	require.NoError(t, err)
}

func TestHungDescriberTimesOutAndLeavesTheCardAsScanned(t *testing.T) {
	old := describeTimeout
	describeTimeout = 50 * time.Millisecond
	defer func() { describeTimeout = old }()

	dir := twoSidedDir(t)
	report, err := runDefaultCard(t, dir, func(o *Options) {
		o.Describe = func(ctx context.Context, _, _ image.Image) (describe.Description, error) {
			<-ctx.Done()
			return describe.Description{}, ctx.Err()
		}
	})
	require.NoError(t, err)
	assert.True(t, report.Cards[0].LLMFailed)
	assert.Contains(t, strings.Join(report.Cards[0].Notes, "\n"), "the model didn't answer within")
	assert.NotContains(t, strings.Join(report.Cards[0].Notes, "\n"), "deadline exceeded")
	assert.FileExists(t, filepath.Join(dir, "0001-meta.yaml"))
}

func TestDeniedWebSearchIsANoteNotAnError(t *testing.T) {
	bin := t.TempDir()
	reply := `{"type":"result","is_error":false,"result":"","permission_denials":[{"tool_name":"WebSearch"}],"structured_output":{"front":{"rotation":"0","description":"","transcription":"","secrets":[]},"back":{"rotation":"0","description":"","transcription":"","secrets":[]},"recipient":"","sender":"","sent_on":"","locale":"","location":{"name":"","latitude":null,"longitude":null,"country_code":""},"orientation_evidence":"x"}}`
	script := "#!/bin/sh\ncat >/dev/null\ncat <<'EOF'\n" + reply + "\nEOF\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	report, err := runDefaultCard(t, twoSidedDir(t), func(o *Options) { o.LLM = describe.Config{Backend: "claude-code"} })
	require.NoError(t, err)
	assert.False(t, report.Cards[0].LLMFailed)
	assert.Contains(t, report.Cards[0].Notes[0], "WebSearch")
}

func titled(title string) func(*Options) {
	return describing(describe.Description{
		Front: describe.Side{Rotation: "0"}, Back: describe.Side{Rotation: "0"}, Title: title,
	}, nil)
}

func twoCards() [][]byte {
	return scanOf(defaultSynth(), defaultSynth(), defaultSynth(), defaultSynth())
}

func TestTitleNamesTheCardAndRepeatsAreNumbered(t *testing.T) {
	dir := t.TempDir()
	report, err := runSides(t, dir, twoCards(), titled("CNN Center"), func(o *Options) { o.Formats = []string{"web", "component"} })
	require.NoError(t, err)
	require.Len(t, report.Cards, 2)

	assert.ElementsMatch(t, []string{"cnn-center", "cnn-center-2"}, []string{report.Cards[0].Name, report.Cards[1].Name})
	assert.Equal(t, []int{1, 2}, []int{report.Cards[0].Number, report.Cards[1].Number})
	for _, name := range []string{"cnn-center", "cnn-center-2"} {
		assert.FileExists(t, filepath.Join(dir, name+"-front.png"))
		assert.FileExists(t, filepath.Join(dir, name+"-meta.yaml"))
		written, _ := filepath.Glob(filepath.Join(dir, name+".postcard.*"))
		assert.Len(t, written, 1, name)
	}
}

func TestTitleNameAvoidsFilesFromAnEarlierRun(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cnn-center.postcard.webp"), nil, 0o644))
	report, err := runDefaultCard(t, dir, titled("CNN Center"), func(o *Options) { o.Formats = []string{"web"} })
	require.NoError(t, err)
	assert.Equal(t, "cnn-center-2", report.Cards[0].Name)
}

func TestWithoutAUsableTitleCardsKeepTheirNumber(t *testing.T) {
	for name, mutate := range map[string]func(*Options){
		"empty title":  titled(""),
		"digits only":  titled("1950"),
		"failed call":  describing(describe.Description{}, errors.New("quota")),
		"no describer": func(*Options) {},
	} {
		dir := t.TempDir()
		report, err := runDefaultCard(t, dir, mutate)
		require.NoError(t, err, name)
		assert.Equal(t, "0001", report.Cards[0].Name, name)
		assert.FileExists(t, filepath.Join(dir, "0001-front.png"), name)
	}
}

func TestTitledNamesStartingWithDigitsDoNotShiftNumbering(t *testing.T) {
	for _, existing := range []string{"1950-diner-front.png", "1950-diner-meta.yaml", "1950-diner.postcard.webp", "1950-diner.html"} {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, existing), nil, 0o644))
		report, err := runDefaultCard(t, dir)
		require.NoError(t, err)
		assert.Equal(t, 1, report.Cards[0].Number, existing)
	}
}

func TestSecretsAreWrittenUnhiddenAndTurnedWithTheirSide(t *testing.T) {
	secret := [][][2]float64{{{0.25, 0.5}, {0.5, 0.5}, {0.5, 1}}}
	dir := twoSidedDir(t)
	_, err := runDefaultCard(t, dir, describing(describe.Description{
		Recipient: "Mr A. Smith",
		Front:     describe.Side{Rotation: "90"},
		Back:      describe.Side{Rotation: "0", Secrets: secret},
	}, nil))
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(dir, "0001-meta.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(b), "name: Mr A. Smith")
	assert.Contains(t, string(b), "prehidden: false")
	assert.Contains(t, string(b), "0.25", "back isn't turned, so its secret is as given")

	dir = twoSidedDir(t)
	_, err = runDefaultCard(t, dir, describing(describe.Description{
		Front: describe.Side{Rotation: "90", Secrets: secret},
		Back:  describe.Side{Rotation: "0"},
	}, nil))
	require.NoError(t, err)
	bundles, err := postcards.MakeBundles([]string{filepath.Join(dir, "0001-front.png"), filepath.Join(dir, "0001-back.png"), filepath.Join(dir, "0001-meta.yaml")})
	require.NoError(t, err)
	pc, err := bundles[0].Decode(formats.DecodeOptions{})
	require.NoError(t, err)
	require.Len(t, pc.Meta.Front.Secrets, 1)
	assert.Equal(t, types.Point{X: 0.5, Y: 0.25}, pc.Meta.Front.Secrets[0].Points[0])
}
