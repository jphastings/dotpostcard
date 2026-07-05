//go:build !wasm
// +build !wasm

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/spf13/cobra"
)

// webFileSuffixes mirrors the suffixes formats/web recognises as a web-format
// postcard file (see formats/web/bundle.go).
var webFileSuffixes = []string{".postcard.webp", ".postcard.jpg", ".postcard.jpeg", ".postcard.png"}

var collectionCmd = &cobra.Command{
	Use:   "collection",
	Short: "Manage *.postcards collection files",
}

var collectionCreateTitle string

var collectionCreateCmd = &cobra.Command{
	Use:     "create <collection.postcards> [card files/dirs...]",
	Example: "  postcards collection create trip.postcards pyramids.postcard.webp\n  postcards collection create trip.postcards ./scanned",
	Short:   "Create a new, empty collection, optionally adding cards to it",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath := args[0]

		c, err := collection.Create(dbPath)
		if err != nil {
			return fmt.Errorf("creating collection: %w", err)
		}
		defer c.Close()

		if collectionCreateTitle != "" {
			if err := c.SetTitle(collectionCreateTitle); err != nil {
				return fmt.Errorf("setting title: %w", err)
			}
		}

		fmt.Printf("Created collection %s\n", dbPath)

		if len(args) == 1 {
			return nil
		}

		return addCards(c, args[1:])
	},
}

var collectionAddCmd = &cobra.Command{
	Use:     "add <collection.postcards> <files-or-dirs...>",
	Example: "  postcards collection add trip.postcards pyramids.postcard.webp\n  postcards collection add trip.postcards ./scanned",
	Short:   "Add web-format postcard files to a collection",
	Args:    cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := collection.Open(args[0])
		if err != nil {
			return fmt.Errorf("opening collection: %w", err)
		}
		defer c.Close()

		return addCards(c, args[1:])
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
			fmt.Printf("%s: %s\n", r.Name, plainSnippet(r.Snippet))
		}

		fmt.Printf("%s\n", count(len(results), "result"))
		return nil
	},
}

func init() {
	collectionCreateCmd.Flags().StringVar(&collectionCreateTitle, "title", "", "Title to give the new collection")
	collectionCmd.AddCommand(collectionCreateCmd, collectionAddCmd, collectionRemoveCmd, collectionLsCmd, collectionSearchCmd, collectionTitleCmd)
	rootCmd.AddCommand(collectionCmd)
}

// addCards reads each web-format postcard file found amongst paths (which may
// be files or, non-recursively, directories) and adds it to c.
func addCards(c *collection.Collection, paths []string) error {
	files, err := expandCardPaths(paths)
	if err != nil {
		return err
	}

	added := 0
	defer func() { fmt.Printf("Added %s\n", count(added, "card")) }()

	for _, file := range files {
		if !isWebFormatFile(file) {
			fmt.Fprintf(os.Stderr, "⚠︎ skipping %s: not a web-format postcard file\n", file)
			continue
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("reading %s: %w", file, err)
		}

		summary, err := c.AddWebPostcard(filepath.Base(file), data)
		if err != nil {
			return fmt.Errorf("adding %s: %w", file, err)
		}

		fmt.Println(summary.Name)
		added++
	}

	return nil
}

// expandCardPaths turns a list of file and directory paths into a flat list
// of file paths, non-recursively listing any directories.
func expandCardPaths(paths []string) ([]string, error) {
	var files []string

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}

		if !info.IsDir() {
			files = append(files, p)
			continue
		}

		entries, err := os.ReadDir(p)
		if err != nil {
			return nil, fmt.Errorf("reading directory %s: %w", p, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			files = append(files, filepath.Join(p, entry.Name()))
		}
	}

	return files, nil
}

func isWebFormatFile(path string) bool {
	for _, suffix := range webFileSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func formatCardLine(card collection.CardSummary) string {
	line := card.Name

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
