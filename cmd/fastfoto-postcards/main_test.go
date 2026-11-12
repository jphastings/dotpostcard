package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/fastfoto"
	"github.com/jphastings/dotpostcard/pkg/describe"
	"github.com/jphastings/dotpostcard/types"
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
	}, "--out-dir", "cards", "--top-side", "front", "--card-threshold", "30", "-Q", "archive")

	assert.NoError(t, err)
	assert.Equal(t, "cards", got.OutDir)
	assert.True(t, got.TopSideFront)
	assert.Equal(t, 30.0, got.CardThreshold)
	assert.Equal(t, formats.Archive, got.Quality)
}

func TestUnknownQualityIsRejected(t *testing.T) {
	_, _, err := execute(t, func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
		return fastfoto.Report{}, nil
	}, "--quality", "lossless")

	assert.ErrorContains(t, err, "efficient, detail, archive")
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

func TestHelpCarriesTheLoadingAdvice(t *testing.T) {
	out, _, err := execute(t, nil, "--help")
	assert.NoError(t, err)
	assert.Contains(t, out, "straightest edge")
	assert.Contains(t, out, "snug")
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

func TestRunPrintsEachCardAsItFinishes(t *testing.T) {
	out, _, err := execute(t, func(_ context.Context, o fastfoto.Options) (fastfoto.Report, error) {
		o.OnStart(2)
		o.OnCard(fastfoto.CardResult{
			Name: "0001", WidthCm: 10.71, HeightCm: 14.83, Flip: types.FlipBook, Elapsed: 2300 * time.Millisecond,
			TiltFront: 1.5,
			Outputs: []fastfoto.Output{
				{Format: "Web", Path: "0001.postcard.webp", Bytes: 494_000},
				{Format: "USDZ 3D model", Path: "0001.postcard.usdz", Bytes: 316_000},
			},
		})
		o.OnCard(fastfoto.CardResult{
			Name: "0003", WidthCm: 14.8, HeightCm: 10.5, Flip: types.FlipLeftHand, Described: true, Elapsed: 16300 * time.Millisecond,
			Outputs:    []fastfoto.Output{{Format: "Component files", Path: "0003-front.png, 0003-back.png, 0003-meta.yaml", Bytes: 24_100_000}, {Format: "ATProto", Path: "at://did:plc:x/app.postcards/0003"}},
			Flags:      []string{"clipped"},
			Notes:      []string{"LLM step failed, left as scanned: quota"},
			ConvertErr: errors.New("disk full"),
		})
		o.OnCard(fastfoto.CardResult{Name: "0002", Rescan: true, Outputs: []fastfoto.Output{{Path: "for-rescan-0002.jpg"}}})
		o.OnCard(fastfoto.CardResult{Name: "0004", Err: errors.New("no card found")})
		return fastfoto.Report{
			Cards:    []fastfoto.CardResult{{Name: "0002", Rescan: true}},
			Unpaired: true,
		}, nil
	}, "--out-dir", "scans")
	assert.NoError(t, err)
	t.Log("\n" + out)

	assert.Equal(t, `⚙︎ Scanning postcards into 2 different formats…
0001 (10.71 x 14.83 cm, 2.3s)
- (Web) 0001.postcard.webp (494 KB)
- (USDZ 3D model) 0001.postcard.usdz (316 KB)
0003 (14.80 x 10.50 cm, detected left-hand flip, LLM description, 16.3s)
- (Component files) 0003-front.png, 0003-back.png, 0003-meta.yaml (24.1 MB)
- (ATProto) at://did:plc:x/app.postcards/0003
⚠︎ clipped
⚠︎ LLM step failed, left as scanned: quota
⚠︎ converting failed: disk full
0002 set aside: scanned skewed, feed it again (for-rescan-0002.jpg)
✗ 0004: no card found
1 card set aside to feed again. See scans/for-rescan-*.jpg
a card's second side never arrived: that side was dropped, feed the card again
`, out)
	assert.NotContains(t, out, "tilt")
}

func TestNothingIsPrintedWhenTheRunFailsBeforeFeeding(t *testing.T) {
	out, _, err := execute(t, func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
		return fastfoto.Report{}, errors.New("bad format")
	})
	assert.Error(t, err)
	assert.Empty(t, out)
}

func TestSingleFormatHeaderIsSingular(t *testing.T) {
	out, _, _ := execute(t, func(_ context.Context, o fastfoto.Options) (fastfoto.Report, error) {
		o.OnStart(1)
		return fastfoto.Report{}, nil
	})
	assert.Equal(t, "⚙︎ Scanning postcards into 1 different format…\n", out)
}

func TestInterruptsHaltThenAbort(t *testing.T) {
	var out bytes.Buffer
	signals := make(chan os.Signal, 2)
	stopped, aborted, released := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go watchSignals(signals, &out, func() { close(stopped) }, func() { close(aborted) }, func() { close(released) })

	signals <- os.Interrupt
	<-stopped
	select {
	case <-aborted:
		t.Fatal("the first interrupt must not abort")
	default:
	}
	signals <- os.Interrupt
	<-aborted
	<-released

	assert.Equal(t, "⏹ Halting scanning after the card in the scanner… (Ctrl-C again to stop without saving the postcards still being processed)\n"+
		"⏹ Stopping now; postcards not yet saved are discarded\n", out.String())
}

func TestStoppedScannerAnnouncesTheCardsStillBeingCompleted(t *testing.T) {
	out, _, err := execute(t, func(_ context.Context, o fastfoto.Options) (fastfoto.Report, error) {
		o.OnScannerStopped(0)
		o.OnScannerStopped(3)
		return fastfoto.Report{}, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "Completing 3 scanned but unsaved postcards…\n", out)
}

func TestAbortReportsUnsavedCardsAndExits130WithoutErrorText(t *testing.T) {
	out, _, err := execute(t, func(context.Context, fastfoto.Options) (fastfoto.Report, error) {
		return fastfoto.Report{Interrupted: true, Discarded: 2, StoppedMidCard: true}, context.Canceled
	})
	assert.Equal(t, "the card being scanned when you stopped wasn't saved: feed it again\n2 scanned postcards weren't saved: feed them again\n", out)
	assert.Equal(t, 130, exitCode(err))
	assert.Empty(t, err.Error())
}

func TestOrdinaryFailuresExitWith1(t *testing.T) {
	assert.Equal(t, 1, exitCode(errors.New("jam")))
}
