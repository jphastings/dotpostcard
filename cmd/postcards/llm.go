//go:build !wasm

package main

import (
	"context"
	"fmt"

	"github.com/jphastings/dotpostcard/internal/cmdhelp"
	"github.com/jphastings/dotpostcard/pkg/describe"
	"github.com/jphastings/dotpostcard/types"
	"github.com/spf13/cobra"
)

var llmConfig describe.Config

// newDescriber is swapped for a fake in tests.
var newDescriber = describe.New

func registerLLMFlags() {
	llmConfig = describe.Config{}
	cmdhelp.AddLLMFlags(rootCmd.Flags(), &llmConfig, "describe each card that has no description yet")
}

func init() {
	describeSetup = func(cmd *cobra.Command) (describeFunc, error) {
		if llmConfig.Backend == "" {
			return nil, nil
		}
		if err := llmConfig.Validate(); err != nil {
			return nil, err
		}
		d, err := newDescriber(cmd.Context(), llmConfig)
		if err != nil {
			return nil, err
		}
		return func(pc *types.Postcard) error {
			// A card that's already described is left alone, without spending a model call.
			if pc.Meta.Front.Description != "" {
				return nil
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), describe.Timeout)
			defer cancel()
			desc, err := d(ctx, pc.Front, pc.Back)
			if err != nil {
				return fmt.Errorf("couldn't describe it, converting without: %w", err)
			}
			desc.Fill(&pc.Meta)
			return nil
		}, nil
	}
}
