package fastfoto

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/jphastings/dotpostcard/pkg/claudecode"
	"github.com/jphastings/dotpostcard/types"
	xdraw "golang.org/x/image/draw"
)

// DefaultLLMModel is used by the Claude backends when no model is given.
const DefaultLLMModel = "claude-opus-5-5"

const (
	llmConcurrency = 3
	llmImageEdgePx = 1200
)

// LLMConfig selects the model that orients and describes each card; the zero value turns it off.
type LLMConfig struct {
	// Backend is "claude-code", "anthropic" or "openai".
	Backend string
	// Model is passed straight through as the model ID (or `claude --model` value).
	Model   string
	BaseURL string
}

// Validate checks the configuration without contacting anything.
func (c LLMConfig) Validate() error {
	switch c.Backend {
	case "", "claude-code", "anthropic":
	case "openai":
		if c.Model == "" {
			return errors.New("--llm openai needs --llm-model")
		}
	default:
		return fmt.Errorf("--llm must be claude-code, anthropic or openai, not %q", c.Backend)
	}
	return nil
}

// SideDescription is what the model says about one side. Rotation is the clockwise turn
// ("0", "90", "180", "270" or "unknown") that makes the side upright.
type SideDescription struct {
	Rotation      string `json:"rotation"`
	Description   string `json:"description"`
	Transcription string `json:"transcription"`
}

type Description struct {
	Front SideDescription `json:"front"`
	Back  SideDescription `json:"back"`
	// Notes are things worth telling the user that didn't stop the description working.
	Notes []string `json:"-"`
}

// Describer looks at the two sides of a card as scanned.
type Describer func(ctx context.Context, front, back image.Image) (Description, error)

const rotationEnum = `{"type":"string","enum":["0","90","180","270","unknown"]}`
const sideSchema = `{"type":"object","additionalProperties":false,"required":["rotation","description","transcription"],"properties":{"rotation":` + rotationEnum + `,"description":{"type":"string"},"transcription":{"type":"string"}}}`
const replySchema = `{"type":"object","additionalProperties":false,"required":["front","back","orientation_evidence"],"properties":{"front":` + sideSchema + `,"back":` + sideSchema + `,"orientation_evidence":{"type":"string"}}}`

const prompt = `These are the two sides of one postcard as scanned: first the front, then the back. Neither side is necessarily upright.

For each side, give the clockwise rotation in degrees (0, 90, 180 or 270) that makes it the right way up, or "unknown" if that side has no clear up (for example a blank or abstract side). Use both sides together: the back's printed caption may say what the front shows, and typical postcard layouts (stamp box top right, address lines on the right) are evidence. orientation_evidence is one or two sentences.

Other fields (an empty string means none):
- add a description for the front of the postcard in front.description (we're talking alt text; no need to describe it as a postcard as that's already contextual, imagine answering "what is this a postcard of?" in an alt-text concise manner for people who can't see it).
- if you think the back has anything worth describing then you can add a description in back.description just like the front description, but it should be something worth describing rather than just it's the back of a postcard. Things like copyright notices and names of photographers aren't worth adding here.
- transcription (for front or back) is only necessary if there has been text written/typed by someone sending the postcard — something added *after* it was created — a message. Try to capture that message as closely as possible in the transcription.
Keep each description to a single sentence of no more than about 20 words. If the card names the place shown, include it.`

// NewDescriber connects the configured backend. Web search is only available with claude-code.
func NewDescriber(ctx context.Context, c LLMConfig) (Describer, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	var m model.BaseChatModel
	var opts []model.Option
	switch c.Backend {
	case "claude-code":
		if err := claudecode.Check(); err != nil {
			return nil, err
		}
		name := c.Model
		if name == "" {
			name = DefaultLLMModel
		}
		m = &claudecode.Model{Name: name}
		opts = []model.Option{claudecode.WithJSONSchema(replySchema), claudecode.WithAllowedTools("WebSearch")}
	case "anthropic":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, errors.New("--llm anthropic needs ANTHROPIC_API_KEY set")
		}
		name := c.Model
		if name == "" {
			name = DefaultLLMModel
		}
		cm, err := claude.NewChatModel(ctx, &claude.Config{APIKey: key, Model: name, MaxTokens: 2000})
		if err != nil {
			return nil, err
		}
		m = cm
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, errors.New("--llm openai needs OPENAI_API_KEY set")
		}
		cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{APIKey: key, BaseURL: c.BaseURL, Model: c.Model})
		if err != nil {
			return nil, err
		}
		m = cm
	default:
		return nil, errors.New("no LLM backend selected")
	}
	return modelDescriber(m, c.Backend != "claude-code", opts...), nil
}

// modelDescriber asks for the schema in the prompt when the backend can't enforce it.
func modelDescriber(m model.BaseChatModel, schemaInPrompt bool, opts ...model.Option) Describer {
	return func(ctx context.Context, front, back image.Image) (Description, error) {
		text := prompt
		if schemaInPrompt {
			text += "\n\nReply with only a JSON object matching this JSON schema:\n" + replySchema
		}
		fb, err := jpegPart(front)
		if err != nil {
			return Description{}, err
		}
		bb, err := jpegPart(back)
		if err != nil {
			return Description{}, err
		}
		msg := &schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: text},
			{Type: schema.ChatMessagePartTypeText, Text: "Front image:"}, fb,
			{Type: schema.ChatMessagePartTypeText, Text: "Back image:"}, bb,
		}}
		reply, err := m.Generate(ctx, []*schema.Message{msg}, opts...)
		if err != nil {
			return Description{}, err
		}
		d, err := parseReply(reply.Content)
		if denied, ok := reply.Extra[claudecode.DeniedToolsKey].([]string); ok && err == nil {
			d.Notes = append(d.Notes, fmt.Sprintf("Claude Code wasn't permitted to use: %s", strings.Join(denied, ", ")))
		}
		return d, err
	}
}

// parseReply tolerates code fences around the JSON but nothing else.
func parseReply(s string) (Description, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSpace(strings.TrimSuffix(s, "```"))
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	var flat struct {
		Front    *SideDescription `json:"front"`
		Back     *SideDescription `json:"back"`
		Evidence *string          `json:"orientation_evidence"`
	}
	if err := dec.Decode(&flat); err != nil {
		return Description{}, fmt.Errorf("the model's reply isn't the expected JSON: %w", err)
	}
	if flat.Front == nil || flat.Back == nil {
		return Description{}, errors.New("the model's reply is missing the front or back")
	}
	for _, r := range []string{flat.Front.Rotation, flat.Back.Rotation} {
		if _, ok := quarterTurns[r]; !ok {
			return Description{}, fmt.Errorf("the model gave an invalid rotation %q", r)
		}
	}
	return Description{Front: *flat.Front, Back: *flat.Back}, nil
}

// jpegPart flattens the card onto white (the model can't see transparency), shrinks it and encodes it.
func jpegPart(img image.Image) (schema.MessageInputPart, error) {
	b := img.Bounds()
	flat := image.NewRGBA(b)
	xdraw.Draw(flat, b, image.NewUniform(color.White), image.Point{}, xdraw.Src)
	xdraw.Draw(flat, b, img, b.Min, xdraw.Over)

	var out image.Image = flat
	if long := max(b.Dx(), b.Dy()); long > llmImageEdgePx {
		w, h := b.Dx()*llmImageEdgePx/long, b.Dy()*llmImageEdgePx/long
		small := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
		xdraw.CatmullRom.Scale(small, small.Bounds(), flat, b, xdraw.Src, nil)
		out = small
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 85}); err != nil {
		return schema.MessageInputPart{}, err
	}
	data := base64.StdEncoding.EncodeToString(buf.Bytes())
	return schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
		MessagePartCommon: schema.MessagePartCommon{Base64Data: &data, MIMEType: "image/jpeg"},
	}}, nil
}

var quarterTurns = map[string]int{"0": 0, "90": 1, "180": 2, "270": 3, "unknown": -1}

// resolveTurns turns the model's rotations into clockwise quarter turns for each side and the
// flip they imply. A side with no clear up takes the turn that makes the card book-flip with
// the other side; two such sides stay as scanned.
func resolveTurns(front, back string) (f, b int, flip types.Flip) {
	f, b = quarterTurns[front], quarterTurns[back]
	switch {
	case f < 0 && b < 0:
		f, b = 0, 0
	case f < 0:
		f = (4 - b) % 4
	case b < 0:
		b = (4 - f) % 4
	}
	return f, b, [4]types.Flip{types.FlipBook, types.FlipLeftHand, types.FlipCalendar, types.FlipRightHand}[(f+b)%4]
}
