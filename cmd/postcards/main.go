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
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "postcards",
	Short:   "A tool for converting between formats for representing images of postcards",
	Version: postcards.Version,
	RunE: func(cmd *cobra.Command, inputPaths []string) error {
		// Grab relevant flags
		formatList, err := cmd.Flags().GetStringSlice("output")
		if err != nil {
			panic("Output flag doesn't seem to be a string slice")
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
					go func(filename string, fw formats.FileWriter) {
						defer wg.Done()

						fileStartT := time.Now()
						dst, err := fw.WriteFile(targetDir, overwrite)
						if err != nil {
							fmt.Fprintf(sso, "⚠︎ %s: %v", filename, err)
							return
						}

						fileDur := time.Now().Sub(fileStartT)
						fmt.Fprintf(sso, "%s (%s) → %s (%s)\n", filename, bundle.Name(), dst, fileDur)
					}(filename, fw)
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
	rootCmd.Flags().Bool("here", false, "Output files in the current working directory")
	rootCmd.Flags().Bool("there", true, "Output files in the same directory as the source data")
	rootCmd.Flags().String("outdir", "", "Output files to the given directory")
	rootCmd.MarkFlagsMutuallyExclusive("here", "there", "outdir")

	rootCmd.Flags().StringSlice("output", []string{}, "Formats to convert to")
	rootCmd.Flags().Bool("archival", false, "Turn off resizing of images and use lossy compression")
	rootCmd.Flags().Bool("overwrite", false, "Overwrite output files")

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
