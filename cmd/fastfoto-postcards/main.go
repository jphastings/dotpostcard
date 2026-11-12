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

	postcards "github.com/jphastings/dotpostcard"
	"github.com/jphastings/dotpostcard/internal/cmdhelp"
	"github.com/jphastings/dotpostcard/internal/fastfoto"
	"github.com/spf13/cobra"
)

const guide = `Scan a stack of postcards through an Epson FastFoto FF-680W (macOS, USB).

By default each card is NNNN-front.png, NNNN-back.png and NNNN-meta.yaml in --out-dir,
ready for the postcards tool. -f converts each card as postcards -f would, and
replaces those files unless it includes component (-f web,component). Cards
number on from the highest number already in --out-dir. Scans are processed in
memory and not kept: to redo a card, feed it again.

Loading: top edge first, back of the card facing you. Keep the paper guides
snug against the card, and put the card's straightest edge leading. A card
that goes in so skewed that a corner is cut off is set aside as
for-rescan-NNNN.jpg, to feed again.

Only one app can scan at a time, so close Epson FastFoto or Image Capture if they are open.
Ctrl-C finishes the current card; press it again to abort. If the scanner
stops responding, switch it off and on again. Cards that overlap completely
in the feeder can't be detected, so check the card count.`

type runFunc func(context.Context, fastfoto.Options) (fastfoto.Report, error)

func newCommand(run runFunc, stdout, stderr io.Writer) *cobra.Command {
	var opts fastfoto.Options
	var topSide string
	cmd := &cobra.Command{
		Use:           "fastfoto-postcards [flags]",
		Short:         "Scan a stack of postcards with an Epson FastFoto FF-680W",
		Long:          guide,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch topSide {
			case "front":
				opts.TopSideFront = true
			case "back":
			default:
				return fmt.Errorf("--top-side must be front or back, not %q", topSide)
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
	f.StringVar(&opts.OutDir, "out-dir", ".", "where to write cards")
	f.StringVar(&topSide, "top-side", "back", "which side of the cards faces outward in the feeder: front or back")
	f.Float64Var(&opts.CardThreshold, "card-threshold", fastfoto.DefaultCardThreshold, "how far a pixel must differ from the backing to count as card")
	f.Float64Var(&opts.FeedScale, "feed-scale", 1, "multiplier for sizes along the feed direction")
	f.BoolVarP(&opts.Archival, "archival", "A", false, "Turn off image resizing, use lossless compression")
	f.StringSliceVarP(&opts.Formats, "formats", "f", nil, fmt.Sprintf("also convert each card to these formats (comma separated, any of: %s, atproto)", strings.Join(postcards.Codecs, ", ")))
	f.StringVar(&opts.ATUser, "at-user", "", "atproto handle or DID to upload as (format atproto only; falls back to $GOAT_USERNAME/$ATP_USERNAME/$ATP_AUTH_USERNAME)")
	f.StringVar(&opts.ATPassword, "at-password", "", "atproto PDS app password (format atproto only; falls back to $GOAT_PASSWORD/$ATP_PASSWORD/$ATP_AUTH_PASSWORD)")
	cmdhelp.AddLLMFlags(f, &opts.LLM, "describe and orient each card")
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
		if c.ConvertErr != nil {
			fmt.Fprintf(w, "  ! converting failed: %v\n", c.ConvertErr)
		}
	}
	if r.Unpaired {
		fmt.Fprintln(w, "a card's second side never arrived: that side was dropped, feed the card again")
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
	fmt.Fprintf(w, "%d cards\n", built)
}

func main() {
	cmd := newCommand(fastfoto.Run, os.Stdout, os.Stderr)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
