package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	postcards "github.com/jphastings/dotpostcard"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/cmdhelp"
	"github.com/jphastings/dotpostcard/internal/fastfoto"
	"github.com/jphastings/dotpostcard/types"
	"github.com/spf13/cobra"
)

const guide = `Scan a stack of postcards through an Epson FastFoto FF-680W (macOS, USB).

By default each card is NNNN-front.png, NNNN-back.png and NNNN-meta.yaml in --out-dir,
ready for the postcards tool. -f converts each card as postcards -f would, and
replaces those files unless it includes component (-f web,component). Cards
number on from the highest number already in --out-dir; with --llm they're named
from a short title instead (cnn-center-front.png). Scans are processed in
memory and not kept: to redo a card, feed it again.

Loading: top edge first, back of the card facing you. Keep the paper guides
snug against the card, and put the card's straightest edge leading. A card
that goes in so skewed that a corner is cut off is set aside as
for-rescan-NNNN.jpg, to feed again.

Only one app can scan at a time, so close Epson FastFoto or Image Capture if they are open.
Ctrl-C finishes the card in the scanner and saves everything scanned; press it
again to stop without saving the postcards still being processed. If the scanner
stops responding, switch it off and on again. Cards that overlap completely
in the feeder can't be detected, so check the card count.`

type runFunc func(context.Context, fastfoto.Options) (fastfoto.Report, error)

func newCommand(run runFunc, stdout, stderr io.Writer) *cobra.Command {
	var opts fastfoto.Options
	var topSide, quality string
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
			var err error
			if opts.Quality, err = formats.ParseQuality(quality); err != nil {
				return err
			}
			if err := opts.LLM.Validate(); err != nil {
				return err
			}
			opts.Log = stderr
			opts.OnStart = func(formats int) { printStart(stdout, formats) }
			opts.OnCard = func(c fastfoto.CardResult) { printCard(stdout, c) }
			opts.OnScannerStopped = func(unsaved int) {
				if unsaved > 0 {
					fmt.Fprintf(stdout, "Completing %d scanned but unsaved %s…\n", unsaved, plural(unsaved, "postcard"))
				}
			}

			ctx, stopNow := context.WithCancel(cmd.Context())
			defer stopNow()
			stop, finishCard := context.WithCancel(ctx)
			defer finishCard()
			opts.Stop = stop
			signals := make(chan os.Signal, 2)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(signals)
			go watchSignals(signals, stdout, finishCard, stopNow, func() { signal.Stop(signals) })

			report, err := run(ctx, opts)
			printReport(stdout, report, opts.OutDir)
			if report.Interrupted {
				return &exitError{code: exitInterrupted}
			}
			return err
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.OutDir, "out-dir", ".", "where to write cards")
	f.StringVar(&topSide, "top-side", "back", "which side of the cards faces outward in the feeder: front or back")
	f.Float64Var(&opts.CardThreshold, "card-threshold", fastfoto.DefaultCardThreshold, "how far a pixel must differ from the backing to count as card")
	f.StringVarP(&quality, "quality", "Q", "efficient", "image quality: efficient (resized for sharing), detail (higher resolution, few artefacts) or archive (full resolution, lossless)")
	f.StringSliceVarP(&opts.Formats, "formats", "f", nil, fmt.Sprintf("also convert each card to these formats (comma separated, any of: %s, atproto)", strings.Join(postcards.Codecs, ", ")))
	f.StringVar(&opts.ATUser, "at-user", "", "atproto handle or DID to upload as (format atproto only; falls back to $GOAT_USERNAME/$ATP_USERNAME/$ATP_AUTH_USERNAME)")
	f.StringVar(&opts.ATPassword, "at-password", "", "atproto PDS app password (format atproto only; falls back to $GOAT_PASSWORD/$ATP_PASSWORD/$ATP_AUTH_PASSWORD)")
	cmdhelp.AddLLMFlags(f, &opts.LLM, "describe and orient each card")
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd
}

const exitInterrupted = 130

// exitError ends the program with a status and, when it has a message, prints it.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func exitCode(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return 1
}

// watchSignals turns the first interrupt into finishing the card in the scanner and the
// second into abandoning everything unsaved. release is called after the second, so that a
// third kills the process.
func watchSignals(signals <-chan os.Signal, w io.Writer, finishCard, stopNow func(), release func()) {
	<-signals
	fmt.Fprintln(w, "⏹ Halting scanning after the card in the scanner… (Ctrl-C again to stop without saving the postcards still being processed)")
	finishCard()
	<-signals
	fmt.Fprintln(w, "⏹ Stopping now; postcards not yet saved are discarded")
	stopNow()
	release()
}

func plural(n int, singular string) string {
	if n == 1 {
		return singular
	}
	return singular + "s"
}

func count(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

func humanBytes(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1f MB", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%.0f KB", float64(n)/1e3)
	}
	return fmt.Sprintf("%d B", n)
}

func printStart(w io.Writer, formats int) {
	fmt.Fprintf(w, "⚙︎ Scanning postcards into %s…\n", count(formats, "different format"))
}

// printCard writes a card's whole block in one call, so blocks never interleave.
func printCard(w io.Writer, c fastfoto.CardResult) {
	var b strings.Builder
	switch {
	case c.Err != nil:
		fmt.Fprintf(&b, "✗ %s: %v\n", c.Name, c.Err)
	case c.Rescan:
		fmt.Fprintf(&b, "%s set aside: scanned skewed, feed it again (%s)\n", c.Name, c.Outputs[0].Path)
	default:
		details := []string{fmt.Sprintf("%.2f x %.2f cm", c.WidthCm, c.HeightCm)}
		if c.Flip != types.FlipBook {
			details = append(details, fmt.Sprintf("detected %s flip", c.Flip))
		}
		if c.Described {
			details = append(details, "LLM description")
		}
		details = append(details, fmt.Sprintf("%.1fs", c.Elapsed.Seconds()))
		fmt.Fprintf(&b, "%s (%s)\n", c.Name, strings.Join(details, ", "))
		for _, o := range c.Outputs {
			if o.Bytes == 0 {
				fmt.Fprintf(&b, "- (%s) %s\n", o.Format, o.Path)
			} else {
				fmt.Fprintf(&b, "- (%s) %s (%s)\n", o.Format, o.Path, humanBytes(o.Bytes))
			}
		}
		for _, warning := range slices.Concat(c.Flags, c.Notes, convertWarning(c)) {
			fmt.Fprintf(&b, "⚠︎ %s\n", warning)
		}
	}
	io.WriteString(w, b.String())
}

func convertWarning(c fastfoto.CardResult) []string {
	if c.ConvertErr == nil {
		return nil
	}
	return []string{fmt.Sprintf("converting failed: %v", c.ConvertErr)}
}

// printReport says only what needs acting on once the run is over.
func printReport(w io.Writer, r fastfoto.Report, outDir string) {
	rescan := 0
	for _, c := range r.Cards {
		if c.Rescan {
			rescan++
		}
	}
	if rescan > 0 {
		fmt.Fprintf(w, "%s set aside to feed again. See %s\n", count(rescan, "card"), filepath.Join(outDir, "for-rescan-*.jpg"))
	}
	if r.StoppedMidCard {
		fmt.Fprintln(w, "the card being scanned when you stopped wasn't saved: feed it again")
	}
	if r.Interrupted && r.Discarded > 0 {
		if r.Discarded == 1 {
			fmt.Fprintln(w, "1 scanned postcard wasn't saved: feed it again")
		} else {
			fmt.Fprintf(w, "%d scanned postcards weren't saved: feed them again\n", r.Discarded)
		}
	}
	if r.Unpaired {
		fmt.Fprintln(w, "a card's second side never arrived: that side was dropped, feed the card again")
	}
}

func main() {
	cmd := newCommand(fastfoto.Run, os.Stdout, os.Stderr)
	if err := cmd.Execute(); err != nil {
		if err.Error() != "" {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(exitCode(err))
	}
}
