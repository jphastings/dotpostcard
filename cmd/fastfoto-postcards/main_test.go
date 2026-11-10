package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/jphastings/dotpostcard/internal/fastfoto"
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
	}, "--out-dir", "cards", "--first-side", "front", "--card-threshold", "30", "--feed-scale", "1.01", "--no-scan")

	assert.NoError(t, err)
	assert.Equal(t, "cards", got.OutDir)
	assert.True(t, got.FirstSideFront)
	assert.Equal(t, 30.0, got.CardThreshold)
	assert.Equal(t, 1.01, got.FeedScale)
	assert.True(t, got.NoScan)
}

func TestRunFailureIsAnError(t *testing.T) {
	_, _, err := execute(t, func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
		return fastfoto.Report{}, errors.New("jam")
	})
	assert.EqualError(t, err, "jam")
}

func TestBadFirstSideIsRejected(t *testing.T) {
	_, _, err := execute(t, nil, "--first-side", "left")
	assert.Error(t, err)
}

func TestReportListsFlagsAndOrphans(t *testing.T) {
	out, _, err := execute(t, func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
		return fastfoto.Report{
			Cards:   []fastfoto.CardResult{{Name: "0001", WidthCm: 14.8, HeightCm: 10.5, Flags: []string{"clipped"}}},
			Orphans: []int{3},
		}, nil
	})
	assert.NoError(t, err)
	assert.Contains(t, out, "0001")
	assert.Contains(t, out, "clipped")
	assert.Contains(t, out, "3")
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
			Cards:  []fastfoto.CardResult{{Name: "0001"}, {Name: "0002", Err: errors.New("no card found")}},
			RawDir: "raw",
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
