//go:build !wasm
// +build !wasm

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/atproto/pdscollection"
	"github.com/jphastings/dotpostcard/internal/cmdhelp"
	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/jphastings/dotpostcard/types"
	"github.com/spf13/cobra"
)

var collectionCmd = &cobra.Command{
	Use:   "collection",
	Short: "Manage *.postcards collection files",
}

const qualityHelp = "Re-encode each card at this quality: efficient, detail or archive (default: store cards untouched)"

const createLong = `Build a collection from postcard files, collections and atproto records, and
write it as <title>.postcards, publish it to atproto, or both.

Sources can be web-format postcard files, *.postcards files (all their
cards), directories (their postcard and *.postcards files, not recursively),
at://.../org.dotpostcard.postcard/... URIs, and at://.../org.dotpostcard.collection/...
URIs (all their cards). Compile scans with postcards first.

-f picks the destinations and defaults to "postcards", which writes
<title>.postcards. Use "-f atproto" to publish without writing a file, or
"-f postcards,atproto" to do both; publishing needs --at-user/--at-password (or
the environment variables goat uses). The collection's record is named after
its title, so creating it again with the same title updates the same record.

Cards are stored untouched unless -Q is given, which re-encodes each one at
that quality (efficient, detail or archive). The title comes from --title, or
from the source when that's a single collection.`

var collectionCreateCmd = &cobra.Command{
	Use:     "create [-f postcards,atproto] [-Q quality] --title <title> <sources...>",
	Example: "  postcards collection create --title \"Trip\" pyramids.postcard.webp ./scanned\n  postcards collection create -f atproto -Q detail --title \"Trip\" ./scanned\n  postcards collection create at://alice.example/org.dotpostcard.collection/trip",
	Short:   "Create a collection from postcards, writing a file and/or publishing it to atproto",
	Long:    createLong,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		formatList, _ := cmd.Flags().GetStringSlice("formats")
		var opts pdscollection.CreateOptions
		publish := false
		for _, format := range formatList {
			switch format {
			case "postcards":
				opts.WriteFile = true
			case atprotoFormatName:
				publish = true
			default:
				return fmt.Errorf("unknown format %q: choose postcards, atproto", format)
			}
		}

		var err error
		if opts.BuildOptions, err = buildOptions(cmd); err != nil {
			return err
		}
		opts.Title, _ = cmd.Flags().GetString("title")
		opts.Overwrite, _ = cmd.Flags().GetBool("overwrite")
		opts.Publish.DryRun, _ = cmd.Flags().GetBool("dry-run")
		opts.Publish.OnCard = printPublishedCard(opts.Publish.DryRun)
		if opts.OutDir, err = cmdhelp.Outdir(cmd, ""); err != nil {
			return err
		}
		stop := stopOnSignal(cmd, &opts.BuildOptions)
		defer stop()
		if publish {
			if opts.Client, err = cmdhelp.ATLogin(cmd); err != nil {
				return err
			}
		}

		result, err := pdscollection.Create(args, opts)
		if err != nil {
			return err
		}
		if result.Path != "" {
			fmt.Printf("Created %s with %s\n", result.Path, count(len(result.Cards), "card"))
		}
		if result.Published != nil {
			printPublishSummary(*result.Published, opts.Publish.DryRun)
		}
		return nil
	},
}

var collectionAddCmd = &cobra.Command{
	Use:     "add <collection.postcards> [-Q quality] <sources...>",
	Example: "  postcards collection add trip.postcards pyramids.postcard.webp\n  postcards collection add trip.postcards -Q efficient ./scanned",
	Short:   "Add postcards to a collection (sources as for create)",
	Args:    cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		opts, err := buildOptions(cmd)
		if err != nil {
			return err
		}
		stop := stopOnSignal(cmd, &opts)
		defer stop()
		names, err := pdscollection.Add(args[0], args[1:], opts)
		if err != nil {
			return err
		}
		fmt.Printf("Added %s\n", count(len(names), "card"))
		return nil
	},
}

var collectionRemoveCmd = &cobra.Command{
	Use:     "remove <collection.postcards> <card-name...>",
	Example: "  postcards collection remove trip.postcards pyramids",
	Short:   "Remove cards from a collection",
	Args:    cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := collection.Open(args[0])
		if err != nil {
			return fmt.Errorf("opening collection: %w", err)
		}
		defer c.Close()

		for _, name := range args[1:] {
			if err := c.Remove(name); err != nil {
				return err
			}
			fmt.Println(name)
		}

		fmt.Printf("Removed %s\n", count(len(args[1:]), "card"))
		return nil
	},
}

var collectionLsCmd = &cobra.Command{
	Use:     "ls <collection.postcards>",
	Example: "  postcards collection ls trip.postcards",
	Short:   "List the cards in a collection",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := collection.OpenReadOnly(args[0])
		if err != nil {
			return fmt.Errorf("opening collection: %w", err)
		}
		defer c.Close()

		title, err := c.Title()
		if err != nil {
			return fmt.Errorf("reading title: %w", err)
		}
		if title != "" {
			fmt.Println(title)
		}

		cards, err := c.List()
		if err != nil {
			return fmt.Errorf("listing cards: %w", err)
		}

		for _, card := range cards {
			fmt.Println(formatCardLine(card))
		}

		fmt.Printf("%s\n", count(len(cards), "card"))
		return nil
	},
}

var collectionTitleCmd = &cobra.Command{
	Use:     "title <collection.postcards> [new-title]",
	Example: "  postcards collection title trip.postcards\n  postcards collection title trip.postcards \"Summer in Italy\"",
	Short:   "Show or set a collection's title",
	Args:    cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			c, err := collection.OpenReadOnly(args[0])
			if err != nil {
				return fmt.Errorf("opening collection: %w", err)
			}
			defer c.Close()

			title, err := c.Title()
			if err != nil {
				return fmt.Errorf("reading title: %w", err)
			}
			fmt.Println(title)
			return nil
		}

		c, err := collection.Open(args[0])
		if err != nil {
			return fmt.Errorf("opening collection: %w", err)
		}
		defer c.Close()

		if err := c.SetTitle(args[1]); err != nil {
			return fmt.Errorf("setting title: %w", err)
		}
		fmt.Println(args[1])
		return nil
	},
}

var collectionSearchCmd = &cobra.Command{
	Use:     "search <collection.postcards> <query...>",
	Example: "  postcards collection search trip.postcards pyramids giza",
	Short:   "Search the cards in a collection",
	Args:    cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := collection.OpenReadOnly(args[0])
		if err != nil {
			return fmt.Errorf("opening collection: %w", err)
		}
		defer c.Close()

		query := strings.Join(args[1:], " ")
		results, err := c.Search(query)
		if err != nil {
			return fmt.Errorf("searching: %w", err)
		}

		for _, r := range results {
			fmt.Printf("%s: %s\n", titleWithName(r.CardSummary), plainSnippet(r.Snippet))
		}

		fmt.Printf("%s\n", count(len(results), "result"))
		return nil
	},
}

var collectionPublishCmd = &cobra.Command{
	Use:     "publish <collection.postcards>",
	Example: "  postcards collection publish blanks.postcards --at-user alice.example\n  postcards collection publish blanks.postcards --dry-run",
	Short:   "Publish a collection to an atproto PDS",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		client, err := cmdhelp.ATLogin(cmd)
		if err != nil {
			return err
		}

		c, err := collection.Open(args[0])
		if err != nil {
			return fmt.Errorf("opening collection: %w", err)
		}
		defer c.Close()

		result, err := pdscollection.Publish(client, c, pdscollection.PublishOptions{
			DryRun:       dryRun,
			DefaultTitle: strings.TrimSuffix(filepath.Base(args[0]), filepath.Ext(args[0])),
			OnCard:       printPublishedCard(dryRun),
		})
		if err != nil {
			return err
		}
		printPublishSummary(result, dryRun)
		return nil
	},
}

func printPublishedCard(dryRun bool) func(pdscollection.PublishedCard) {
	return func(card pdscollection.PublishedCard) {
		line := fmt.Sprintf("%s → (ATProto) %s", card.Name, card.URI)
		switch {
		case card.Reused:
			line += " (already published)"
		case dryRun:
			line += " (would publish)"
		}
		fmt.Println(line)
	}
}

func printPublishSummary(result pdscollection.PublishResult, dryRun bool) {
	verb := "Published"
	if dryRun {
		verb = "Would publish"
	}
	fmt.Printf("%s %s as %s\n", verb, count(len(result.Cards), "card"), result.URI)
}

// stopOnSignal makes the build stop at the next card on interrupt or terminate. The signal
// handling is released once triggered, so a second Ctrl-C kills the process.
func stopOnSignal(cmd *cobra.Command, opts *pdscollection.BuildOptions) (release func()) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	context.AfterFunc(ctx, stop)
	opts.Stop = ctx
	return stop
}

// buildOptions reads the flags shared by the commands that gather cards from sources.
func buildOptions(cmd *cobra.Command) (pdscollection.BuildOptions, error) {
	opts := pdscollection.BuildOptions{
		PDSHost: cmdhelp.ATPDSHostOverride(),
		PLCHost: cmdhelp.ATPLCHostOverride(),
		Warn:    func(msg string) { fmt.Fprintf(os.Stderr, "⚠︎ %s\n", msg) },
		OnCard:  func(name string) { fmt.Println(name) },
	}
	if cmd.Flags().Changed("quality") {
		name, _ := cmd.Flags().GetString("quality")
		quality, err := formats.ParseQuality(name)
		if err != nil {
			return opts, err
		}
		opts.Quality = &quality
	}
	return opts, nil
}

func init() {
	cmdhelp.AddATFlags(collectionPublishCmd)
	collectionPublishCmd.Flags().Bool("dry-run", false, "Log in and read, but write nothing, reporting what would be published")

	registerCollectionCreateFlags()
	collectionAddCmd.Flags().StringP("quality", "Q", "", qualityHelp)

	collectionCmd.AddCommand(collectionCreateCmd, collectionAddCmd, collectionRemoveCmd, collectionLsCmd, collectionSearchCmd, collectionTitleCmd, collectionPublishCmd)
	rootCmd.AddCommand(collectionCmd)
}

// registerCollectionCreateFlags attaches create's flags; tests pair it with ResetFlags for a clean slate.
func registerCollectionCreateFlags() {
	cmdhelp.AddATFlags(collectionCreateCmd)
	collectionCreateCmd.Flags().StringSliceP("formats", "f", []string{"postcards"}, "Where to put the collection (comma separated, any of: postcards, atproto)")
	collectionCreateCmd.Flags().String("title", "", "Title of the collection (default: the title of a single collection source)")
	collectionCreateCmd.Flags().Bool("out-here", false, "Write the file in the current working directory (default)")
	collectionCreateCmd.Flags().String("out-dir", "", "Write the file to the given directory")
	collectionCreateCmd.MarkFlagsMutuallyExclusive("out-here", "out-dir")
	collectionCreateCmd.Flags().Bool("overwrite", false, "Replace the file if it already exists")
	collectionCreateCmd.Flags().Bool("dry-run", false, "With atproto: log in and read, but write no file and no record, reporting what would be published")
	collectionCreateCmd.Flags().StringP("quality", "Q", "", qualityHelp)
}

// titleWithName appends the card's name, which `remove` needs, unless its title already implies it.
func titleWithName(card collection.CardSummary) string {
	if card.Name == types.Slug(card.Title) {
		return card.Title
	}
	return fmt.Sprintf("%s [%s]", card.Title, card.Name)
}

func formatCardLine(card collection.CardSummary) string {
	line := titleWithName(card)

	if card.SenderName != "" || card.RecipientName != "" {
		sender := card.SenderName
		if sender == "" {
			sender = "?"
		}
		recipient := card.RecipientName
		if recipient == "" {
			recipient = "?"
		}
		line += fmt.Sprintf(" — %s → %s", sender, recipient)
	}

	if card.SentOn != nil {
		line += fmt.Sprintf(" (%s)", card.SentOn.Format("2006-01-02"))
	}

	return line
}

// plainSnippet strips the <b>/</b> highlight markers Search() wraps matches
// in, leaving plain text simple enough for any terminal.
func plainSnippet(snippet string) string {
	snippet = strings.ReplaceAll(snippet, "<b>", "")
	snippet = strings.ReplaceAll(snippet, "</b>", "")
	return snippet
}
