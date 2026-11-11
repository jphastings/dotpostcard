package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClaude installs a `claude` that records its arguments and stdin, then prints the canned lines.
func fakeClaude(t *testing.T, output string) (argsFile, stdinFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile, stdinFile = filepath.Join(dir, "args"), filepath.Join(dir, "stdin")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\ncat > " + stdinFile + "\ncat <<'EOF'\n" + output + "\nEOF\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return
}

func userWithImage() []*schema.Message {
	data := "QUJD"
	return []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "look"},
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &data, MIMEType: "image/jpeg"}}},
	}}}
}

const initLine = `{"type":"system","subtype":"init"}`

func TestStructuredOutputIsReturnedAndImagesAndSchemaAreSent(t *testing.T) {
	args, stdin := fakeClaude(t, initLine+"\n"+`{"type":"result","is_error":false,"result":"prose","structured_output":{"front":{"rotation":"0"}}}`)

	m := &Model{Name: "opus"}
	msg, err := m.Generate(context.Background(), userWithImage(), WithJSONSchema(`{"type":"object"}`), WithAllowedTools("WebSearch"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"front":{"rotation":"0"}}`, msg.Content)

	gotArgs, _ := os.ReadFile(args)
	assert.Contains(t, string(gotArgs), "--json-schema\n{\"type\":\"object\"}\n")
	assert.Contains(t, string(gotArgs), "--tools\nWebSearch\n")
	assert.Contains(t, string(gotArgs), "--allowedTools\nWebSearch\n")
	assert.Contains(t, string(gotArgs), "--model\nopus\n")
	assert.NotContains(t, string(gotArgs), "--bare")
	sent, _ := os.ReadFile(stdin)
	assert.Contains(t, string(sent), `"type":"image"`)
	assert.Contains(t, string(sent), `"media_type":"image/jpeg","type":"base64"`)
	assert.Contains(t, string(sent), `"data":"QUJD"`)
}

func TestPlainResultIsUsedWithoutASchema(t *testing.T) {
	fakeClaude(t, `{"type":"result","is_error":false,"result":"hello"}`)
	msg, err := (&Model{}).Generate(context.Background(), userWithImage())
	require.NoError(t, err)
	assert.Equal(t, "hello", msg.Content)
}

func TestErrorResultIsAnErrorWithTheCLIsMessage(t *testing.T) {
	fakeClaude(t, `{"type":"result","is_error":true,"result":"Not logged in"}`)
	_, err := (&Model{}).Generate(context.Background(), userWithImage())
	assert.ErrorContains(t, err, "Not logged in")
}

func TestMissingStructuredOutputIsAnError(t *testing.T) {
	fakeClaude(t, `{"type":"result","is_error":false,"result":"I cannot"}`)
	_, err := (&Model{}).Generate(context.Background(), userWithImage(), WithJSONSchema(`{}`))
	assert.ErrorContains(t, err, "no structured output")
}

func TestMissingClaudeGivesAClearError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := (&Model{}).Generate(context.Background(), userWithImage())
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "Claude Code CLI, `claude`, installed and logged in"))
	assert.Error(t, Check())
}

func TestCancellingStopsAHungClaude(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	script := "#!/bin/sh\necho $$ > " + pidFile + "\nexec sleep 60\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := (&Model{}).Generate(ctx, userWithImage())
	require.Error(t, err)
	assert.Less(t, time.Since(start), 10*time.Second)

	raw, readErr := os.ReadFile(pidFile)
	require.NoError(t, readErr)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	assert.ErrorIs(t, syscall.Kill(pid, 0), syscall.ESRCH, "claude is still running")
}

func TestDeniedToolsAreReportedOnTheReply(t *testing.T) {
	fakeClaude(t, `{"type":"result","is_error":false,"result":"hi","permission_denials":[{"tool_name":"WebSearch"}]}`)
	msg, err := (&Model{}).Generate(context.Background(), userWithImage())
	require.NoError(t, err)
	assert.Equal(t, []string{"WebSearch"}, msg.Extra[DeniedToolsKey])
}
