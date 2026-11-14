package main

import (
	"fmt"

	postcards "github.com/jphastings/dotpostcard"
	"github.com/spf13/cobra"
)

var rotateCmd = &cobra.Command{
	Use:     "rotate [--front DEG] [--back DEG] <some.postcard>",
	Example: "  postcards rotate --front 90 landscape-book.postcard\n  postcards rotate --front -90 --back 180 pyramid.postcard.png",
	Short:   "Turn the sides of a postcard file clockwise, in place",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		frontDeg, _ := cmd.Flags().GetInt("front")
		backDeg, _ := cmd.Flags().GetInt("back")

		front, err := postcards.QuarterTurns(frontDeg)
		if err != nil {
			return fmt.Errorf("--front: %w", err)
		}
		back, err := postcards.QuarterTurns(backDeg)
		if err != nil {
			return fmt.Errorf("--back: %w", err)
		}
		if front == 0 && back == 0 {
			fmt.Println("nothing to rotate")
			return nil
		}

		flip, err := postcards.RotateFile(args[0], front, back)
		if err != nil {
			return err
		}
		fmt.Printf("↻ %s (flip: %s)\n", args[0], flip)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(rotateCmd)
	rotateCmd.Flags().Int("front", 0, "Degrees to turn the front clockwise: -90, 90, 180 or 270")
	rotateCmd.Flags().Int("back", 0, "Degrees to turn the back clockwise: -90, 90, 180 or 270")
}
