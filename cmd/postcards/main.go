package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/jphastings/postcards"
	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/internal/cmdhelp"
	"github.com/jphastings/postcards/internal/general"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "postcards --formats=output,formats [flags] postcard-file.ext...",
	Example: "  postcards -f web,json postcard1-front.jpg postcard2.webp directory/*\n  postcards -f components --archival --overwrite pc.webp",
	Short:   "A tool for converting between formats for representing images of postcards",
	Version: general.Version,
	RunE: func(cmd *cobra.Command, inputPaths []string) error {
		// Grab relevant flags
		formatList, err := cmd.Flags().GetStringSlice("formats")
		if err != nil {
			panic("Formats flag doesn't seem to be a string slice")
		}
		archival, err := cmd.Flags().GetBool("archival")
		if err != nil {
			panic("Archival flag doesn't seem to be boolean")
		}
		overwrite, err := cmd.Flags().GetBool("overwrite")
		if err != nil {
			panic("Overwrite flag doesn't seem to be boolean")
		}

		codecs, err := postcards.CodecsByFormat(formatList)
		if err != nil {
			return err
		}

		encOpts := formats.EncodeOptions{
			Archival: archival,
		}

		bundles, err := postcards.MakeBundles(inputPaths)
		if err != nil {
			return err
		}

		if len(bundles) == 0 {
			return cmd.Usage()
		}

		fmt.Fprintf(os.Stdout, "⚙︎ Converting %s into %s…\n", count(len(bundles), "postcard"), count(len(codecs), "different format"))

		sso := &safeWrite{w: os.Stdout}
		var wg sync.WaitGroup

		for _, bundle := range bundles {
			targetDir, err := cmdhelp.Outdir(cmd, path.Dir(bundle.RefPath()))
			if err != nil {
				return err
			}
			filename := path.Base(bundle.RefPath())

			pc, err := bundle.Decode()
			if err != nil {
				return err
			}

			for _, codec := range codecs {
				for _, fw := range codec.Encode(pc, encOpts) {
					wg.Add(1)
					go func(filename, codecName string, fw formats.FileWriter) {
						defer wg.Done()

						fileStartT := time.Now()
						dst, err := fw.WriteFile(targetDir, overwrite)
						if err != nil {
							fmt.Fprintf(sso, "⚠︎ %s: %v", filename, err)
							return
						}

						fileDur := time.Now().Sub(fileStartT)
						fmt.Fprintf(sso, "%s (%s) → (%s) %s (%s)\n", filename, bundle.Name(), codecName, dst, fileDur)
					}(filename, codec.Name(), fw)
				}
			}
		}

		wg.Wait()

		return nil
	},
}

type safeWrite struct {
	w  io.Writer
	mu sync.Mutex
}

func (s *safeWrite) Write(b []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(b)
}

func count(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

func main() {
	rootCmd.Flags().Bool("out-here", false, "Output files in the current working directory")
	rootCmd.Flags().Bool("out-there", true, "Output files in the same directory as the source data")
	rootCmd.Flags().String("out-dir", "", "Output files to the given directory")
	rootCmd.MarkFlagsMutuallyExclusive("out-here", "out-there", "out-dir")

	rootCmd.Flags().StringSliceP("formats", "f", []string{}, "Formats to convert to")
	rootCmd.Flags().BoolP("archival", "A", false, "Turn off image resizing, use lossless compression")
	rootCmd.Flags().Bool("overwrite", false, "Overwrite output files")

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
