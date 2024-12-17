package main

import (
	"fmt"
	"os"

	postcards "github.com/jphastings/dotpostcard"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/metadata"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:     "info <some.postcard.ext>",
	Example: "  postcards info london.postcard.jpg\n  postcards info madrid.postcard.usdz",
	Short:   "Display information about a postcard file",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, inputPath []string) error {
		jsonInstead, err := cmd.Flags().GetBool("json")
		if err != nil {
			panic("JSON flag doesn't seem to be boolean")
		}

		bundles, err := postcards.MakeBundles(inputPath)
		if err != nil {
			return err
		}
		filename := inputPath[0]

		format := metadata.AsYAML
		if jsonInstead {
			format = metadata.AsJSON
		}

		if len(bundles) != 1 {
			return fmt.Errorf("no postcard information within '%s'", filename)
		}

		pc, err := bundles[0].Decode(formats.DecodeOptions{})
		if err != nil {
			return fmt.Errorf("unable to decode bundle '%s': %w", filename, err)
		}

		fws, err := metadata.Codec(format).Encode(pc, &formats.EncodeOptions{})
		if err != nil {
			return err
		}

		if format == metadata.AsYAML {
			fmt.Println("# The Postcard metadata stored within", filename)
		}
		return fws[0].WriteTo(os.Stdout)

	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().Bool("json", false, "Output in JSON format, instead of YAML")
}
