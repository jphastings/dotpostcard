package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	postcards "github.com/jphastings/dotpostcard"
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
			if err := opts.LLM.Validate(); err != nil {
				return err
			}
			opts.Log = stderr
			opts.OnStart = func(formats int) { printStart(stdout, formats) }
			opts.OnCard = func(c fastfoto.CardResult) { printCard(stdout, c) }

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
	f.BoolVarP(&opts.Archival, "archival", "A", false, "Turn off image resizing, use lossless compression")
	f.StringSliceVarP(&opts.Formats, "formats", "f", nil, fmt.Sprintf("also convert each card to these formats (comma separated, any of: %s, atproto)", strings.Join(postcards.Codecs, ", ")))
	f.StringVar(&opts.ATUser, "at-user", "", "atproto handle or DID to upload as (format atproto only; falls back to $GOAT_USERNAME/$ATP_USERNAME/$ATP_AUTH_USERNAME)")
	f.StringVar(&opts.ATPassword, "at-password", "", "atproto PDS app password (format atproto only; falls back to $GOAT_PASSWORD/$ATP_PASSWORD/$ATP_AUTH_PASSWORD)")
	cmdhelp.AddLLMFlags(f, &opts.LLM, "describe and orient each card")
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd
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
	if r.Unpaired {
		fmt.Fprintln(w, "a card's second side never arrived: that side was dropped, feed the card again")
	}
}

func main() {
	cmd := newCommand(fastfoto.Run, os.Stdout, os.Stderr)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
