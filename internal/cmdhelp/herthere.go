package cmdhelp

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Outdir(cmd *cobra.Command, therePath string) (string, error) {
	outdir, err := cmd.Flags().GetString("outdir")
	if err != nil {
		return "", err
	}
	if outdir != "" {
		// Only error if outdir is a regular file (ie. allow existing and non-existing directories)
		fi, err := os.Stat(outdir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if !fi.IsDir() {
			return "", fmt.Errorf("outdir %s is a regular file", outdir)
		}
		return outdir, os.MkdirAll(outdir, 0700)
	}
	heredir, err := cmd.Flags().GetBool("here")
	if err != nil {
		return "", err
	}
	if heredir {
		return ".", nil
	}
	return therePath, nil
}
