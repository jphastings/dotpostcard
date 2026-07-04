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
	Short: "Manage *.postcard.db collection files",
}

var collectionCreateCmd = &cobra.Command{
	Use:     "create <collection.postcard.db> [card files/dirs...]",
	Example: "  postcards collection create trip.postcard.db pyramids.postcard.webp\n  postcards collection create trip.postcard.db ./scanned",
	Short:   "Create a new, empty collection, optionally adding cards to it",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath := args[0]

		c, err := collection.Create(dbPath)
		if err != nil {
			return fmt.Errorf("creating collection: %w", err)
		}
		defer c.Close()

		fmt.Printf("Created collection %s\n", dbPath)

		if len(args) == 1 {
			return nil
		}

		return addCards(c, args[1:])
	},
}

var collectionAddCmd = &cobra.Command{
	Use:     "add <collection.postcard.db> <files-or-dirs...>",
	Example: "  postcards collection add trip.postcard.db pyramids.postcard.webp\n  postcards collection add trip.postcard.db ./scanned",
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
	Use:     "remove <collection.postcard.db> <card-name...>",
	Example: "  postcards collection remove trip.postcard.db pyramids",
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
	Use:     "ls <collection.postcard.db>",
	Example: "  postcards collection ls trip.postcard.db",
	Short:   "List the cards in a collection",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := collection.OpenReadOnly(args[0])
		if err != nil {
			return fmt.Errorf("opening collection: %w", err)
		}
		defer c.Close()

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

var collectionSearchCmd = &cobra.Command{
	Use:     "search <collection.postcard.db> <query...>",
	Example: "  postcards collection search trip.postcard.db pyramids giza",
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
	collectionCmd.AddCommand(collectionCreateCmd, collectionAddCmd, collectionRemoveCmd, collectionLsCmd, collectionSearchCmd)
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
