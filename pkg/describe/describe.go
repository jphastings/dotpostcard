// Package describe asks a vision model to describe, transcribe and orient the two sides of a postcard.
package describe

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
	"math"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/jphastings/dotpostcard/pkg/claudecode"
	"github.com/jphastings/dotpostcard/types"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/text/language"
)

// DefaultModel is used by the Claude backends when no model is given.
const DefaultModel = "claude-opus-5-5"

// Timeout is how long one Describer call should be allowed to take.
const Timeout = 5 * time.Minute

const llmImageEdgePx = 1200

// Config selects the model that describes and orients each card; the zero value turns it off.
type Config struct {
	// Backend is "claude-code", "anthropic" or "openai".
	Backend string
	// Model is passed straight through as the model ID (or `claude --model` value).
	Model   string
	BaseURL string
	// Locale is the card's BCP-47 locale (e.g. en-GB); empty lets the model choose the card's main language.
	Locale string
}

// Validate checks the configuration without contacting anything.
func (c Config) Validate() error {
	switch c.Backend {
	case "", "claude-code", "anthropic":
	case "openai":
		if c.Model == "" {
			return errors.New("--llm openai needs --llm-model")
		}
	default:
		return fmt.Errorf("--llm must be claude-code, anthropic or openai, not %q", c.Backend)
	}
	if c.Locale != "" {
		tag, err := language.Parse(c.Locale)
		if err != nil {
			return fmt.Errorf("--locale must be a BCP-47 language tag like en-GB, not %q", c.Locale)
		}
		if tag.String() != c.Locale {
			return fmt.Errorf("--locale %q should be written %s", c.Locale, tag)
		}
	}
	return nil
}

// Run is a piece of a transcription in one style and language. The model gives runs rather than
// annotation offsets because it can't count bytes; Fill works the offsets out.
type Run struct {
	Text   string   `json:"text"`
	Styles []string `json:"styles"`
	Locale string   `json:"locale"`
}

// Side is what the model says about one side. Rotation is the clockwise turn
// ("0", "90", "180", "270" or "unknown") that makes the side upright.
type Side struct {
	Rotation      string `json:"rotation"`
	Description   string `json:"description"`
	Transcription []Run  `json:"transcription"`
	// AddressLines is asked for only so the model counts the address's lines before outlining
	// them: without it the last line (often just a country) was sometimes left showing.
	AddressLines []string `json:"address_lines"`
	// Secrets are polygons of [x, y] fractions of the side as scanned, clamped to the side.
	Secrets [][][2]float64 `json:"secrets"`
}

// Description is what the model says about a card.
type Description struct {
	Front Side `json:"front"`
	Back  Side `json:"back"`
	// Title is the card's title: a caption printed on it, or a short phrase for what it shows.
	Title     string
	Recipient string
	Sender    string
	SentOn    *types.Date
	Location  types.Location
	Locale    string
	// Notes are things worth telling the user that didn't stop the description working.
	Notes []string `json:"-"`
}

// Describer looks at the two sides of a card as scanned.
type Describer func(ctx context.Context, front, back image.Image) (Description, error)

const rotationEnum = `{"type":"string","enum":["0","90","180","270","unknown"]}`
const runsSchema = `{"type":"array","items":{"type":"object","additionalProperties":false,"required":["text","styles","locale"],"properties":{"text":{"type":"string"},"styles":{"type":"array","items":{"type":"string","enum":["em","strong","underline"]}},"locale":{"type":"string"}}}}`
const sideSchema = `{"type":"object","additionalProperties":false,"required":["rotation","description","transcription","address_lines","secrets"],"properties":{"rotation":` + rotationEnum + `,"description":{"type":"string"},"transcription":` + runsSchema + `,"address_lines":{"type":"array","items":{"type":"string"}},"secrets":{"type":"array","items":{"type":"array","items":{"type":"array","items":{"type":"number"}}}}}}`
const locationSchema = `{"type":"object","additionalProperties":false,"required":["name","latitude","longitude","country_code"],"properties":{"name":{"type":"string"},"latitude":{"type":["number","null"]},"longitude":{"type":["number","null"]},"country_code":{"type":"string"}}}`
const replySchema = `{"type":"object","additionalProperties":false,"required":["front","back","title","recipient","sender","sent_on","location","locale","orientation_evidence"],"properties":{"title":{"type":"string"},"recipient":{"type":"string"},"sender":{"type":"string"},"sent_on":{"type":"string"},"location":` + locationSchema + `,"locale":{"type":"string"},"front":` + sideSchema + `,"back":` + sideSchema + `,"orientation_evidence":{"type":"string"}}}`

const prompt = `These are the two sides of one postcard as scanned: first the front, then the back. Neither side is necessarily upright.

For each side, give the clockwise rotation in degrees (0, 90, 180 or 270) that makes it the right way up, or "unknown" if that side has no clear up (for example a blank or abstract side). Use both sides together: the back's printed caption may say what the front shows, and typical postcard layouts (stamp box top right, address lines on the right) are evidence. orientation_evidence is one or two sentences.

Other fields (an empty string means none):
- add a description for the front of the postcard in front.description (we're talking alt text; no need to describe it as a postcard as that's already contextual, imagine answering "what is this a postcard of?" in an alt-text concise manner for people who can't see it).
- if you think the back has anything worth describing then you can add a description in back.description just like the front description, but it should be something worth describing rather than just it's the back of a postcard. Things like copyright notices and names of photographers aren't worth adding here.
- transcription (for front or back) is only for text written or typed by the person sending the postcard: a message added *after* the card was made. Transcribe exactly what is written: the same words, spelling, punctuation and line breaks, with nothing added, corrected or described. Write it in the order it reads; don't pad with spaces to mimic where words sit on the card. Never describe a doodle or drawing in words. Give anything the sender drew (a smiley, a heart, a dove) as the one emoji that matches it, placed where it sits among the text; leave it out only if no emoji suits. Leave the recipient's name and address out of the transcription, and a handwritten date too when you give it as sent_on: they have their own fields.
- transcription is given as runs: pieces of text in reading order which, joined with nothing between them, are the whole transcription (keep spaces and line breaks inside the runs). Start a new run wherever the styling or language changes. A run's styles lists only what the sender clearly did: "underline" for underlined writing, "strong" for bold or heavily gone-over writing, "em" for italic writing set apart from the rest; empty for ordinary writing. A run's locale is the BCP-47 locale of a run written in a language other than the card's locale; empty otherwise.
- recipient is the name part of the handwritten address, as written (e.g. "Mr & Mrs A. Smith"), with none of the address.
- sender is the name the message is signed off with, only if it is legible; don't guess. (The sign-off stays in the transcription as well.)
- sent_on is the date the card was sent, as YYYY-MM-DD. Use a date the sender wrote on the card by hand if there is one, otherwise a legible postmark; where one of them is incomplete (no day, or no year) the other may complete it. Read a postmark's day and month in the order its country uses. If you can't settle on a full date, leave sent_on empty and keep any handwritten date, as written, in the transcription.
- location is where the card was sent from, if that is a recognisable place: what the card shows or names, the postmark, or the message may tell you. name is a short place name (e.g. "Lyme Regis, UK"), latitude and longitude are that place's decimal degrees (null if you aren't confident of them), country_code is its ISO 3166-1 alpha-3 code. Leave name empty if the place isn't recognisable.
{locale}
- address_lines (for front or back) lists each line of a handwritten recipient address below the name, top to bottom, one entry per line as written; empty when there is none. It is only used to check the secrets.
- secrets (for front or back) marks the address part of a handwritten recipient address (street, town, postcode, country) so it can be cut out cleanly. Never include the recipient's name. Each secret is a polygon: a list of [x, y] points, each a fraction of that image's width and height exactly as provided (before any rotation), with [0, 0] the top left and [1, 1] the bottom right. Keep the shape simple: horizontal and vertical edges only, and as few points as will do. Usually that is one secret of four points, a rectangle around every address line. Only if that rectangle would also cover something that matters (the name, part of the message, the stamp) add corners to step around it, or use several separate secrets; never more than six rectangles' worth. Put each edge in the clear space just outside the address: the top edge above the tallest strokes of the first line, in the gap under the name, and the bottom edge below the last line. Check the result against address_lines: every line listed must lie wholly inside a secret, from its first character to its last. An empty list when there is no handwritten address.
- title is the card's title. If either side carries text that reads as a title for what is shown (a printed caption, the name of an artwork, a description of the photograph), copy that text verbatim: {titleCopy}. Leave out credits, publisher names and serial numbers. Only if there is no such text, write a short title of 1 to 5 words saying what the card shows. Never include the word "postcard".
Keep each description to a single sentence of no more than about 20 words. If the card names the place shown, include it.`

const (
	mainLocaleRule   = "- locale is the BCP-47 locale (e.g. en-GB, fr-FR) of the card's main language: the one most of the transcription is written in, or with no transcription the language you write the descriptions in. Write the descriptions, and any title you make up, in that language."
	forcedLocaleRule = "- the card's locale is %s. Write the descriptions and the title in %[1]s, translating a printed caption into %[1]s if it is in another language. Give %[1]s as locale."
	mainTitleCopy    = "same language, spelling, capitalisation and punctuation"
	forcedTitleCopy  = "same spelling, capitalisation and punctuation, but translated into the card's locale if it is in another language"
)

// promptFor is the prompt for a card whose locale is forced, or chosen by the model when empty.
func promptFor(locale string) string {
	rule, titleCopy := mainLocaleRule, mainTitleCopy
	if locale != "" {
		rule, titleCopy = fmt.Sprintf(forcedLocaleRule, locale), forcedTitleCopy
	}
	return strings.NewReplacer("{locale}", rule, "{titleCopy}", titleCopy).Replace(prompt)
}

// New connects the configured backend. Web search is only available with claude-code.
func New(ctx context.Context, c Config) (Describer, error) {
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
			name = DefaultModel
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
			name = DefaultModel
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
	return modelDescriber(m, c.Backend != "claude-code", c.Locale, opts...), nil
}

// modelDescriber asks for the schema in the prompt when the backend can't enforce it.
func modelDescriber(m model.BaseChatModel, schemaInPrompt bool, locale string, opts ...model.Option) Describer {
	return func(ctx context.Context, front, back image.Image) (Description, error) {
		text := promptFor(locale)
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
		if locale != "" {
			d.Locale = locale
		}
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
		Front     *Side  `json:"front"`
		Back      *Side  `json:"back"`
		Title     string `json:"title"`
		Recipient string `json:"recipient"`
		Sender    string `json:"sender"`
		SentOn    string `json:"sent_on"`
		Locale    string `json:"locale"`
		Location  struct {
			Name        string   `json:"name"`
			Latitude    *float64 `json:"latitude"`
			Longitude   *float64 `json:"longitude"`
			CountryCode string   `json:"country_code"`
		} `json:"location"`
		Evidence *string `json:"orientation_evidence"`
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
	d := Description{
		Front: *flat.Front, Back: *flat.Back, Title: flat.Title,
		Recipient: flat.Recipient, Sender: flat.Sender, Locale: flat.Locale,
	}
	d.Front.Secrets, d.Back.Secrets = cleanSecrets(d.Front.Secrets), cleanSecrets(d.Back.Secrets)
	if flat.SentOn != "" {
		if t, err := time.Parse("2006-01-02", flat.SentOn); err != nil {
			d.Notes = append(d.Notes, fmt.Sprintf("Ignored the model's sent_on %q: not a YYYY-MM-DD date", flat.SentOn))
		} else {
			d.SentOn = &types.Date{Time: t}
		}
	}
	if l := flat.Location; l.Name != "" {
		d.Location.Name = l.Name
		if l.Latitude != nil && l.Longitude != nil && math.Abs(*l.Latitude) <= 90 && math.Abs(*l.Longitude) <= 180 {
			d.Location.Latitude, d.Location.Longitude = l.Latitude, l.Longitude
		}
		if len(l.CountryCode) == 3 && strings.IndexFunc(l.CountryCode, func(r rune) bool { return !unicode.IsLetter(r) || r > unicode.MaxASCII }) < 0 {
			d.Location.CountryCode = strings.ToUpper(l.CountryCode)
		}
	}
	return d, nil
}

// cleanSecrets clamps points into the image and drops polygons too small to enclose anything.
func cleanSecrets(polys [][][2]float64) [][][2]float64 {
	var out [][][2]float64
	for _, poly := range polys {
		if len(poly) < 3 {
			continue
		}
		clamped := make([][2]float64, len(poly))
		for i, p := range poly {
			clamped[i] = [2]float64{min(max(p[0], 0), 1), min(max(p[1], 0), 1)}
		}
		out = append(out, clamped)
	}
	return out
}

// Polygons gives the side's secrets for the side after the given clockwise quarter turns.
func (s Side) Polygons(turns int) []types.Polygon {
	var out []types.Polygon
	for _, poly := range s.Secrets {
		points := make([]types.Point, len(poly))
		for i, p := range poly {
			x, y := p[0], p[1]
			for range turns {
				x, y = 1-y, x
			}
			points[i] = types.Point{X: x, Y: y}
		}
		out = append(out, types.Polygon{Points: points})
	}
	return out
}

// ShrunkJPEG flattens img onto white (the model can't see transparency), shrinks its long edge
// to llmImageEdgePx and encodes it.
func ShrunkJPEG(img image.Image) ([]byte, error) {
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
	err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 85})
	return buf.Bytes(), err
}

func jpegPart(img image.Image) (schema.MessageInputPart, error) {
	b, err := ShrunkJPEG(img)
	if err != nil {
		return schema.MessageInputPart{}, err
	}
	data := base64.StdEncoding.EncodeToString(b)
	return schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
		MessagePartCommon: schema.MessagePartCommon{Base64Data: &data, MIMEType: "image/jpeg"},
	}}, nil
}

var quarterTurns = map[string]int{"0": 0, "90": 1, "180": 2, "270": 3, "unknown": -1}

// Fill sets what the model said, other than secrets, only where it's empty. Secrets are left
// out because a card that's already decoded can no longer hide them.
func (d Description) Fill(meta *types.Metadata) {
	fill := func(dst *string, v string) {
		if *dst == "" {
			*dst = v
		}
	}
	fill(&meta.Title, d.Title)
	fill(&meta.Front.Description, d.Front.Description)
	fill(&meta.Locale, d.Locale)
	fillText := func(dst *types.AnnotatedText, runs []Run) {
		if dst.Text == "" {
			*dst = annotate(runs, meta.Locale)
		}
	}
	fillText(&meta.Front.Transcription, d.Front.Transcription)
	fill(&meta.Back.Description, d.Back.Description)
	fillText(&meta.Back.Transcription, d.Back.Transcription)
	fill(&meta.Recipient.Name, d.Recipient)
	fill(&meta.Sender.Name, d.Sender)
	if meta.SentOn == nil {
		meta.SentOn = d.SentOn
	}
	if meta.Location.Name == "" {
		meta.Location = d.Location
	}
}

// Turns gives the clockwise quarter turns for each side and the flip they imply. A side with
// no clear up takes the turn that makes the card book-flip with the other side; two such
// sides stay as scanned.
func (d Description) Turns() (f, b int, flip types.Flip) {
	f, b = quarterTurns[d.Front.Rotation], quarterTurns[d.Back.Rotation]
	switch {
	case f < 0 && b < 0:
		f, b = 0, 0
	case f < 0:
		f = (4 - b) % 4
	case b < 0:
		b = (4 - f) % 4
	}
	return f, b, types.FlipBook.Turned(f + b)
}

// annotate joins the runs and marks their styles and languages, merging neighbours that share one.
// A run in the card's own language isn't marked.
func annotate(runs []Run, cardLocale string) types.AnnotatedText {
	var text strings.Builder
	var out []types.Annotation
	open := map[types.Annotation]int{} // type and value -> the annotation in out that ends where the next run starts
	cardBase, _ := language.Make(cardLocale).Base()
	for _, r := range runs {
		if r.Text == "" {
			continue
		}
		start := text.Len()
		text.WriteString(r.Text)
		var marks []types.Annotation
		for _, st := range r.Styles {
			if t := types.AnnotationType(st); t == types.ATEmphasis || t == types.ATStrong || t == types.ATUnderline {
				marks = append(marks, types.Annotation{Type: t})
			}
		}
		if tag, err := language.Parse(r.Locale); err == nil && tag != language.Und {
			if base, _ := tag.Base(); base != cardBase {
				marks = append(marks, types.Annotation{Type: types.ATLocale, Value: tag.String()})
			}
		}
		next := map[types.Annotation]int{}
		for _, m := range marks {
			if _, dup := next[m]; dup {
				continue
			}
			if i, ok := open[m]; ok {
				out[i].End = uint(text.Len())
				next[m] = i
				continue
			}
			next[m] = len(out)
			out = append(out, types.Annotation{Type: m.Type, Value: m.Value, Start: uint(start), End: uint(text.Len())})
		}
		open = next
	}
	return types.AnnotatedText{Text: text.String(), Annotations: out}
}
