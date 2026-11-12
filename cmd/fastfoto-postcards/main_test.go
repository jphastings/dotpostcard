package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/jphastings/dotpostcard/internal/fastfoto"
	"github.com/jphastings/dotpostcard/pkg/describe"
	"github.com/stretchr/testify/assert"
)

func execute(t *testing.T, run runFunc, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := newCommand(run, &out, &errOut)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestFlagsReachTheRun(t *testing.T) {
	var got fastfoto.Options
	_, _, err := execute(t, func(_ context.Context, o fastfoto.Options) (fastfoto.Report, error) {
		got = o
		return fastfoto.Report{}, nil
	}, "--out-dir", "cards", "--top-side", "front", "--card-threshold", "30", "--feed-scale", "1.01", "-A")

	assert.NoError(t, err)
	assert.Equal(t, "cards", got.OutDir)
	assert.True(t, got.TopSideFront)
	assert.Equal(t, 30.0, got.CardThreshold)
	assert.Equal(t, 1.01, got.FeedScale)
	assert.True(t, got.Archival)
}

func TestRunFailureIsAnError(t *testing.T) {
	_, _, err := execute(t, func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
		return fastfoto.Report{}, errors.New("jam")
	})
	assert.EqualError(t, err, "jam")
}

func TestBadTopSideIsRejected(t *testing.T) {
	_, _, err := execute(t, nil, "--top-side", "left")
	assert.Error(t, err)
}

func TestReportListsFlagsAndUnpairedSides(t *testing.T) {
	out, _, err := execute(t, func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
		return fastfoto.Report{
			Cards:    []fastfoto.CardResult{{Name: "0001", WidthCm: 14.8, HeightCm: 10.5, Flags: []string{"clipped"}}},
			Unpaired: true,
		}, nil
	})
	assert.NoError(t, err)
	assert.Contains(t, out, "0001")
	assert.Contains(t, out, "clipped")
	assert.Contains(t, out, "feed the card again")
}

func TestReportSummarisesCardsToRescan(t *testing.T) {
	run := func(cards ...fastfoto.CardResult) func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
		return func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
			return fastfoto.Report{Cards: cards}, nil
		}
	}
	out, _, _ := execute(t, run(fastfoto.CardResult{Name: "0001"}, fastfoto.CardResult{Name: "0002", Rescan: true}, fastfoto.CardResult{Name: "0003", Rescan: true}))
	assert.Contains(t, out, "2 cards were scanned skewed, and need a rescan. See for-rescan-*.jpg\n")
	assert.Contains(t, out, "1 cards\n")

	out, _, _ = execute(t, run(fastfoto.CardResult{Name: "0002", Rescan: true}), "--out-dir", "scans")
	assert.Contains(t, out, "1 card was scanned skewed, and needs a rescan. See scans/for-rescan-*.jpg\n")
}

func TestHelpCarriesTheLoadingAdvice(t *testing.T) {
	out, _, err := execute(t, nil, "--help")
	assert.NoError(t, err)
	assert.Contains(t, out, "straightest edge")
	assert.Contains(t, out, "snug")
}

func TestFailedCardsAreReportedAndNotCounted(t *testing.T) {
	out, _, _ := execute(t, func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
		return fastfoto.Report{
			Cards: []fastfoto.CardResult{{Name: "0001"}, {Name: "0002", Err: errors.New("no card found")}},
		}, nil
	})
	assert.Contains(t, out, "no card found")
	assert.Contains(t, out, "1 cards")
}

func TestNonPositiveFeedScaleIsRejected(t *testing.T) {
	for _, scale := range []string{"0", "-1", "NaN", "Inf"} {
		_, _, err := execute(t, nil, "--feed-scale="+scale)
		assert.Error(t, err, scale)
	}
}

func TestBadLLMFlagsAreRejected(t *testing.T) {
	for _, args := range [][]string{{"--llm=gemini"}, {"--llm=openai"}} {
		_, _, err := execute(t, nil, args...)
		assert.Error(t, err, args)
	}
}

func TestLLMFlagsReachTheRun(t *testing.T) {
	var got fastfoto.Options
	_, _, err := execute(t, func(_ context.Context, o fastfoto.Options) (fastfoto.Report, error) {
		got = o
		return fastfoto.Report{}, nil
	}, "--llm=openai", "--llm-model", "m", "--llm-base-url", "http://x")
	assert.NoError(t, err)
	assert.Equal(t, describe.Config{Backend: "openai", Model: "m", BaseURL: "http://x"}, got.LLM)
}

func TestSummaryCountsCardsLeftAsScanned(t *testing.T) {
	out, _, err := execute(t, func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
		return fastfoto.Report{Cards: []fastfoto.CardResult{{Name: "0001", LLMFailed: true}, {Name: "0002", LLMFailed: true}, {Name: "0003"}}}, nil
	})
	assert.NoError(t, err)
	assert.Contains(t, out, "2 cards left as scanned: the LLM step failed")
}

func TestFormatsAndBareLLMReachTheRun(t *testing.T) {
	var got fastfoto.Options
	_, _, err := execute(t, func(_ context.Context, o fastfoto.Options) (fastfoto.Report, error) {
		got = o
		return fastfoto.Report{}, nil
	}, "-f", "web,usdz", "--llm")
	assert.NoError(t, err)
	assert.Equal(t, []string{"web", "usdz"}, got.Formats)
	assert.Equal(t, "claude-code", got.LLM.Backend)
}
