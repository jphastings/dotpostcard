package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	postcards "github.com/jphastings/dotpostcard"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/cmdhelp"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "postcards --formats=output,formats [flags] postcard-file.ext...",
	Example: "  postcards -f web,json postcard1-front.jpg postcard2.webp directory/*\n  postcards -f components --archival --overwrite pc.webp",
	Short:   "A tool for converting between formats for representing images of postcards",
	Long:    longMessage(),
	Version: postcards.Version,
	Args:    cobra.MinimumNArgs(1),
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
		removeBorder, err := cmd.Flags().GetBool("remove-border")
		if err != nil {
			panic("Remove border flag doesn't seem to be boolean")
		}
		ignoreTransparency, err := cmd.Flags().GetBool("ignore-transparency")
		if err != nil {
			panic("Ignore transparency flag doesn't seem to be boolean")
		}
		decOpts := formats.DecodeOptions{RemoveBorder: removeBorder, IgnoreTransparency: ignoreTransparency}

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

			pc, err := bundle.Decode(decOpts)
			if err != nil {
				return fmt.Errorf("unable to decode bundle '%s': %w", filename, err)
			}

			for _, codec := range codecs {
				fws, err := codec.Encode(pc, &encOpts)
				if err != nil {
					return err
				}
				for _, fw := range fws {
					wg.Add(1)
					go func(filename, bundleName, codecName string, fw formats.FileWriter) {
						defer wg.Done()

						fileStartT := time.Now()
						dst, err := fw.WriteFile(targetDir, overwrite)
						if err != nil {
							fmt.Fprintf(sso, "⚠︎ %s: %v\n", filename, err)
							return
						}

						fileDur := time.Since(fileStartT)
						fmt.Fprintf(sso, "%s (%s) → (%s) %s (%s)\n", filename, bundleName, codecName, dst, fileDur)
					}(filename, bundle.Name(), codec.Name(), fw)
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
	rootCmd.Flags().Bool("out-here", false, "Output files in the current working directory (default)")
	rootCmd.Flags().Bool("out-there", true, "Output files in the same directory as the source data")
	rootCmd.Flags().String("out-dir", "", "Output files to the given directory")
	rootCmd.MarkFlagsMutuallyExclusive("out-here", "out-there", "out-dir")

	formatsExpl := fmt.Sprintf("Formats to convert to (comma separated, any of: %s)", strings.Join(postcards.Codecs, ", "))
	rootCmd.Flags().StringSliceP("formats", "f", []string{}, formatsExpl)
	rootCmd.Flags().BoolP("archival", "A", false, "Turn off image resizing, use lossless compression")
	rootCmd.Flags().BoolP("remove-border", "B", false, "Attempts to turn the border around a postcard scan transparent (experimental; component input only)")
	rootCmd.Flags().BoolP("ignore-transparency", "T", false, "Ignores any transparency in the source images")
	rootCmd.Flags().Bool("overwrite", false, "Overwrite output files")

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func longMessage() string {
	return `Convert digital representations of postcards between various formats.

To start from scratch, scan both sides of your postcard and name them
whatever-front.png and whatever-back.png then run:
$ postcards init whatever-front.png

This will generate the metadata file "whatever-meta.yaml" for you to fill out.
Once you're ready you can then run:
$ postcards -f web,usdz whatever-front.png

Which will compile your postcard into the "web" format and the "usdz" format.
Advice on doing this well in this tool's readme at:
  https://github.com/jphastings/dotpostcard
`
}
