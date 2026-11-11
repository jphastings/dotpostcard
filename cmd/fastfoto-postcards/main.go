package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jphastings/dotpostcard/internal/fastfoto"
	"github.com/spf13/cobra"
)

const guide = `Scan a stack of postcards through an Epson FastFoto FF-680W (macOS, USB).

Per card you get NNNN-front.png, NNNN-back.png and NNNN-meta.yaml in --out-dir,
ready for the postcards tool. Raw scans are kept in raw/ so a run can be resumed.
To redo a card, delete its NNNN-meta.yaml and run again with --no-scan.

Loading: top edge first, back of the card facing you. Keep the paper guides
snug against the card, and put the card's straightest edge leading. A skewed
card loses its far corners to the scan; those cards are set aside as
for-rescan-NNNN.jpg, to feed again.

Only one app can scan at a time, so close Epson FastFoto or Image Capture if they are open.
Ctrl-C finishes the current card; press it again to abort. If the scanner
stops responding, switch it off and on again. Cards that overlap completely
in the feeder can't be detected, so check the card count.`

type runFunc func(context.Context, fastfoto.Options) (fastfoto.Report, error)

func newCommand(run runFunc, stdout, stderr io.Writer) *cobra.Command {
	var opts fastfoto.Options
	var firstSide string
	cmd := &cobra.Command{
		Use:           "fastfoto-postcards [flags]",
		Short:         "Scan a stack of postcards with an Epson FastFoto FF-680W",
		Long:          guide,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch firstSide {
			case "front":
				opts.FirstSideFront = true
			case "back":
			default:
				return fmt.Errorf("--first-side must be front or back, not %q", firstSide)
			}
			if !(opts.FeedScale > 0) || math.IsInf(opts.FeedScale, 0) {
				return fmt.Errorf("--feed-scale must be a positive number, not %v", opts.FeedScale)
			}
			if err := opts.LLM.Validate(); err != nil {
				return err
			}
			opts.Log = stderr

			ctx, stopNow := context.WithCancel(cmd.Context())
			defer stopNow()
			stop, finishCard := context.WithCancel(ctx)
			defer finishCard()
			opts.Stop = stop
			signals := make(chan os.Signal, 2)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(signals)
			go func() {
				<-signals
				finishCard()
				<-signals
				stopNow()
				signal.Stop(signals) // a third Ctrl-C kills the process normally
			}()

			report, err := run(ctx, opts)
			printReport(stdout, report, opts.OutDir)
			return err
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.OutDir, "out-dir", ".", "where to write cards and raw/")
	f.StringVar(&firstSide, "first-side", "back", "which side of each card comes through the scanner first: front or back")
	f.Float64Var(&opts.CardThreshold, "card-threshold", fastfoto.DefaultCardThreshold, "how far a pixel must differ from the backing to count as card")
	f.Float64Var(&opts.FeedScale, "feed-scale", 1, "multiplier for sizes along the feed direction")
	f.BoolVar(&opts.NoScan, "no-scan", false, "only process pages already in raw/")
	f.StringVar(&opts.LLM.Backend, "llm", "", "describe and orient each card with: claude-code (with web search), anthropic (ANTHROPIC_API_KEY) or openai (OPENAI_API_KEY). The anthropic and openai backends haven't been tested live")
	f.StringVar(&opts.LLM.Model, "llm-model", "", "model for --llm (default "+fastfoto.DefaultLLMModel+" for the Claude backends; required for openai)")
	f.StringVar(&opts.LLM.BaseURL, "llm-base-url", "", "OpenAI-compatible endpoint for --llm openai")
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd
}

func printReport(w io.Writer, r fastfoto.Report, outDir string) {
	built, llmFailed := 0, 0
	rescan := 0
	for _, c := range r.Cards {
		if c.Err != nil {
			fmt.Fprintf(w, "%s  FAILED: %v\n", c.Name, c.Err)
			continue
		}
		if c.LLMFailed {
			llmFailed++
		}
		if c.Rescan {
			rescan++
			fmt.Fprintf(w, "%s  scanned skewed, needs a rescan  tilt %.1f/%.1f deg\n", c.Name, c.TiltFront, c.TiltBack)
		} else {
			built++
			fmt.Fprintf(w, "%s  %.2f x %.2f cm  tilt %.1f/%.1f deg\n", c.Name, c.WidthCm, c.HeightCm, c.TiltFront, c.TiltBack)
		}
		for _, note := range c.Notes {
			fmt.Fprintf(w, "  %s\n", note)
		}
		for _, flag := range c.Flags {
			fmt.Fprintf(w, "  ! %s\n", flag)
		}
	}
	if len(r.Orphans) > 0 {
		pages := make([]string, len(r.Orphans))
		for i, p := range r.Orphans {
			pages[i] = fmt.Sprintf("%d", p)
		}
		fmt.Fprintf(w, "unpaired pages kept: %s\n", strings.Join(pages, ", "))
	}
	if llmFailed > 0 {
		fmt.Fprintf(w, "%d cards left as scanned: the LLM step failed\n", llmFailed)
	}
	if rescan > 0 {
		subject := "cards were scanned skewed, and need"
		if rescan == 1 {
			subject = "card was scanned skewed, and needs"
		}
		fmt.Fprintf(w, "%d %s a rescan. See %s\n", rescan, subject, filepath.Join(outDir, "for-rescan-*.jpg"))
	}
	if r.RawDir != "" {
		fmt.Fprintf(w, "%d cards; raw scans: %s (%.0f MB)\n", built, r.RawDir, float64(r.RawBytes)/1e6)
	}
}

func main() {
	cmd := newCommand(fastfoto.Run, os.Stdout, os.Stderr)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
