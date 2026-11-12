//go:build !wasm

package cmdhelp

import (
	"github.com/jphastings/dotpostcard/pkg/describe"
	"github.com/spf13/pflag"
)

// AddLLMFlags registers --llm, --llm-model and --llm-base-url, filling cfg. purpose starts
// --llm's help, e.g. "describe each card".
func AddLLMFlags(fs *pflag.FlagSet, cfg *describe.Config, purpose string) {
	fs.StringVar(&cfg.Backend, "llm", "", purpose+" with: claude-code (with web search; the default when given bare), anthropic (ANTHROPIC_API_KEY) or openai (OPENAI_API_KEY). Name a backend with --llm=anthropic, not --llm anthropic. The anthropic and openai backends haven't been tested live")
	fs.Lookup("llm").NoOptDefVal = "claude-code"
	fs.StringVar(&cfg.Model, "llm-model", "", "model for --llm (default "+describe.DefaultModel+" for the Claude backends; required for openai)")
	fs.StringVar(&cfg.BaseURL, "llm-base-url", "", "OpenAI-compatible endpoint for --llm openai")
}
