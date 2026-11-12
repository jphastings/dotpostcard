package fastfoto

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
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

func TestHeterorientedAnswerTurnsTheSidesAndSetsTheFlip(t *testing.T) {
	dir := twoSidedDir(t)
	report, err := runDefaultCard(t, dir, describing(describe.Description{
		Front: describe.Side{Rotation: "90", Description: "A harbour at dusk", Transcription: "Wish you were here"},
		Back:  describe.Side{Rotation: "0", Description: "A map of the bay"},
	}, nil))
	require.NoError(t, err)
	assert.Contains(t, report.Cards[0].Notes[0], "left-hand")

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
	reply := `{"type":"result","is_error":false,"result":"","structured_output":{"front":{"rotation":"0","description":"A harbour","transcription":""},"back":{"rotation":"0","description":"","transcription":""},"orientation_evidence":"x"}}`
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
	assert.FileExists(t, filepath.Join(dir, "0001-meta.yaml"))
}

func TestDeniedWebSearchIsANoteNotAnError(t *testing.T) {
	bin := t.TempDir()
	reply := `{"type":"result","is_error":false,"result":"","permission_denials":[{"tool_name":"WebSearch"}],"structured_output":{"front":{"rotation":"0","description":"","transcription":""},"back":{"rotation":"0","description":"","transcription":""},"orientation_evidence":"x"}}`
	script := "#!/bin/sh\ncat >/dev/null\ncat <<'EOF'\n" + reply + "\nEOF\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	report, err := runDefaultCard(t, twoSidedDir(t), func(o *Options) { o.LLM = describe.Config{Backend: "claude-code"} })
	require.NoError(t, err)
	assert.False(t, report.Cards[0].LLMFailed)
	assert.Contains(t, report.Cards[0].Notes[0], "WebSearch")
}
