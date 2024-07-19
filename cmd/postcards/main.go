package cmd

import (
	"fmt"
	"os"

	"github.com/jphastings/postcards"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "postcards",
	Short:   "A tool for converting between formats for representing images of postcards",
	Version: postcards.Version,
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func Execute() {
	rootCmd.Flags().Bool("here", false, "Output files in the current working directory")
	rootCmd.Flags().Bool("there", true, "Output files in the same directory as the source data")
	rootCmd.Flags().String("outdir", "", "Output files to the given directory")
	rootCmd.MarkFlagsMutuallyExclusive("here", "there", "outdir")

	rootCmd.Flags().StringSlice("output", []string{}, "Formats to convert to")
	rootCmd.Flags().Bool("archival", false, "Turn off resizing of images and use lossy compression")

	checkErr(rootCmd.Execute())
}
