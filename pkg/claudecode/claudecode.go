// Package claudecode is an Eino chat model that runs the user's installed Claude Code CLI
// (`claude -p`), so a Claude subscription login can be used where no API key exists.
package claudecode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const maxTurns = 6

// killGrace is how long a cancelled `claude` (and any child still holding its output) gets before being abandoned.
const killGrace = 3 * time.Second

// DeniedToolsKey is the Extra key on a reply listing the tools Claude Code refused to run for lack of permission.
const DeniedToolsKey = "claudecode.permission_denials"

var errMissing = errors.New("needs the Claude Code CLI, `claude`, installed and logged in")

// Check reports whether the claude executable can be found.
func Check() error {
	if _, err := exec.LookPath("claude"); err != nil {
		return errMissing
	}
	return nil
}

type options struct {
	jsonSchema string
	tools      []string
}

// WithJSONSchema makes the reply conform to the given JSON schema.
func WithJSONSchema(schema string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) { o.jsonSchema = schema })
}

// WithAllowedTools lists the Claude Code tools (such as WebSearch) the model may use. By default it has none.
func WithAllowedTools(tools ...string) model.Option {
	return model.WrapImplSpecificOptFn(func(o *options) { o.tools = tools })
}

// Model implements model.BaseChatModel over `claude -p`.
type Model struct {
	// Name is passed to `claude --model` (an ID or alias such as "opus"); empty means the CLI's default.
	Name string
}

var _ model.BaseChatModel = (*Model)(nil)

func (m *Model) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if err := Check(); err != nil {
		return nil, err
	}
	common := model.GetCommonOptions(&model.Options{Model: &m.Name}, opts...)
	o := model.GetImplSpecificOptions(&options{}, opts...)

	line, err := userMessageLine(input)
	if err != nil {
		return nil, err
	}
	args := []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose",
		"--no-session-persistence", "--safe-mode", "--max-turns", strconv.Itoa(maxTurns),
		"--tools", strings.Join(o.tools, ",")}
	if len(o.tools) > 0 {
		args = append(args, "--allowedTools", strings.Join(o.tools, ","))
	}
	if *common.Model != "" {
		args = append(args, "--model", *common.Model)
	}
	if o.jsonSchema != "" {
		args = append(args, "--json-schema", o.jsonSchema)
	}

	// An empty directory keeps the user's project files and CLAUDE.md out of the call.
	dir, err := os.MkdirTemp("", "claudecode")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(line)
	cmd.WaitDelay = killGrace
	detachFromTerminal(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	waitErr := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	var result map[string]any
	for _, l := range bytes.Split(stdout.Bytes(), []byte("\n")) {
		var ev map[string]any
		if json.Unmarshal(l, &ev) == nil && ev["type"] == "result" {
			result = ev
		}
	}
	if result == nil {
		return nil, fmt.Errorf("claude gave no result (%v): %s", waitErr, strings.TrimSpace(stderr.String()))
	}
	msg, err := replyFrom(result, o.jsonSchema != "")
	if err == nil {
		if denied := deniedTools(result); len(denied) > 0 {
			msg.Extra = map[string]any{DeniedToolsKey: denied}
		}
	}
	return msg, err
}

func deniedTools(result map[string]any) []string {
	var names []string
	denials, _ := result["permission_denials"].([]any)
	for _, d := range denials {
		if m, ok := d.(map[string]any); ok {
			if n, _ := m["tool_name"].(string); n != "" {
				names = append(names, n)
			}
		}
	}
	return names
}

// Stream returns the whole reply as a single chunk.
func (m *Model) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func replyFrom(result map[string]any, wantStructured bool) (*schema.Message, error) {
	text, _ := result["result"].(string)
	if isErr, _ := result["is_error"].(bool); isErr {
		return nil, fmt.Errorf("claude reported an error: %s", text)
	}
	if !wantStructured {
		return schema.AssistantMessage(text, nil), nil
	}
	structured, ok := result["structured_output"]
	if !ok || structured == nil {
		return nil, fmt.Errorf("claude returned no structured output: %s", text)
	}
	b, err := json.Marshal(structured)
	if err != nil {
		return nil, err
	}
	return schema.AssistantMessage(string(b), nil), nil
}

// userMessageLine folds every message into the one stream-json user message the CLI reads.
func userMessageLine(input []*schema.Message) ([]byte, error) {
	var content []map[string]any
	for _, msg := range input {
		if msg.Content != "" {
			content = append(content, map[string]any{"type": "text", "text": msg.Content})
		}
		for _, p := range msg.UserInputMultiContent {
			switch {
			case p.Type == schema.ChatMessagePartTypeText:
				content = append(content, map[string]any{"type": "text", "text": p.Text})
			case p.Type == schema.ChatMessagePartTypeImageURL && p.Image != nil && p.Image.Base64Data != nil:
				content = append(content, map[string]any{"type": "image", "source": map[string]any{
					"type": "base64", "media_type": p.Image.MIMEType, "data": *p.Image.Base64Data,
				}})
			default:
				return nil, fmt.Errorf("claudecode only takes text and base64 images, not %q parts", p.Type)
			}
		}
	}
	line, err := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": content}})
	return append(line, '\n'), err
}
