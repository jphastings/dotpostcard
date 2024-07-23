package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/jphastings/postcards"
	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/formats/sides"
	"github.com/jphastings/postcards/formats/web"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "postcards",
	Short:   "A tool for converting between formats for representing images of postcards",
	Version: postcards.Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := "/private/tmp/postcard-test/portugal-front.png"
		dir := os.DirFS(path.Dir(filename))
		file, err := dir.Open(path.Base(filename))
		if err != nil {
			return err
		}

		bnds, remaining, errs := sides.Codec().Bundle([]fs.File{file}, dir)
		for file, bErr := range errs {
			err = errors.Join(err, fmt.Errorf("couldn't process %s: %w", file, bErr))
		}
		if err != nil {
			return err
		}
		if len(bnds) != 1 {
			return fmt.Errorf("should have 1 bundle, got %d (%d remaining)", len(bnds), len(remaining))
		}

		pc, err := bnds[0].Decode()
		if err != nil {
			return err
		}

		errc := make(chan error)
		fws := web.Codec().Encode(pc, formats.EncodeOptions{Archival: true}, errc)

		var encErrs error
		go func() {
			for {
				encErrs = errors.Join(encErrs, <-errc)
			}
		}()

		for _, fw := range fws {
			if err := fw.WriteFile(path.Dir(filename)); err != nil {
				return err
			}
		}

		return encErrs
	},
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	rootCmd.Flags().Bool("here", false, "Output files in the current working directory")
	rootCmd.Flags().Bool("there", true, "Output files in the same directory as the source data")
	rootCmd.Flags().String("outdir", "", "Output files to the given directory")
	rootCmd.MarkFlagsMutuallyExclusive("here", "there", "outdir")

	rootCmd.Flags().StringSlice("output", []string{}, "Formats to convert to")
	rootCmd.Flags().Bool("archival", false, "Turn off resizing of images and use lossy compression")

	checkErr(rootCmd.Execute())
}
