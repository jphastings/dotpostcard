package main

import (
	"fmt"
	"os"
	"path"

	"github.com/jphastings/dotpostcard/formats/metadata"
	"github.com/jphastings/dotpostcard/internal/cmdhelp"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:     "init <postcard name>...",
	Example: "  postcards init mine\n",
	Short:   "Creates a template YAML metadata file to fill out for your postcard.",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, names []string) error {
		overwrite, err := cmd.Flags().GetBool("overwrite")
		if err != nil {
			panic("Overwrite flag doesn't seem to be boolean")
		}

		flags := os.O_CREATE | os.O_WRONLY
		if overwrite {
			flags |= os.O_TRUNC
		} else {
			flags |= os.O_EXCL
		}

		plural := ""
		if len(names) > 1 {
			plural = "s"
		}
		fmt.Printf("⚙︎ Generating %d postcard metadata file%s…\n", len(names), plural)

		targetDir, err := cmdhelp.Outdir(cmd, "")
		if err != nil {
			return err
		}

		for _, name := range names {
			if err := createYAML(targetDir, name, flags); err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	initCmd.Flags().Bool("out-here", false, "Output files in the current working directory (default)")
	initCmd.Flags().String("out-dir", "", "Output files to the given directory")
	initCmd.MarkFlagsMutuallyExclusive("out-here", "out-dir")

	initCmd.Flags().Bool("overwrite", false, "Overwrite existing YAML metadata")
	rootCmd.AddCommand(initCmd)
}

func createYAML(targetDir, name string, flags int) error {
	filename := fmt.Sprintf("%s-meta.yaml", name)
	f, err := os.OpenFile(path.Join(targetDir, filename), flags, 0644)
	if err != nil {
		return err
	}

	if _, err := f.WriteString(metadata.GuideYAML); err != nil {
		return err
	}

	fmt.Printf("Template (Metadata) → (Metadata) %s\n", filename)

	return nil
}
