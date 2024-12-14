//go:build !wasm
// +build !wasm

package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	postcards "github.com/jphastings/dotpostcard"
	"github.com/spf13/cobra"
)

var formatCmd = &cobra.Command{
	Use:     "format <format>",
	Example: formatExamples(),
	Short:   "Information about the formats this tool works with",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		doc, err := postcards.FormatDocs(args[0])
		if err != nil {
			return fmt.Errorf("unable to find the docs for %s: %w", args[0], err)
		}

		r, err := glamour.NewTermRenderer(
			// detect background color and pick either the default dark or light theme
			glamour.WithAutoStyle(),
		)
		if err != nil {
			return fmt.Errorf("unable to create a doc renderer: %w", err)
		}

		out, err := r.Render(doc)
		if err != nil {
			return fmt.Errorf("unable to render docs: %w", err)
		}

		fmt.Print(out)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(formatCmd)
}

func formatExamples() string {
	return "  postcards format " + strings.Join(postcards.Codecs, "\n  postcards format ")
}
