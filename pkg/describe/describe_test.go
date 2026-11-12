package describe

import (
	"testing"

	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTurns(t *testing.T) {
	for name, c := range map[string]struct {
		front, back string
		f, b        int
		flip        types.Flip
	}{
		"upright":          {"0", "0", 0, 0, types.FlipBook},
		"back unknown":     {"270", "unknown", 3, 1, types.FlipBook},
		"both upside down": {"180", "180", 2, 2, types.FlipBook},
		"sideways front":   {"90", "0", 1, 0, types.FlipLeftHand},
		"calendar":         {"0", "180", 0, 2, types.FlipCalendar},
		"right hand":       {"270", "0", 3, 0, types.FlipRightHand},
		"front unknown":    {"unknown", "90", 3, 1, types.FlipBook},
		"both unknown":     {"unknown", "unknown", 0, 0, types.FlipBook},
	} {
		d := Description{Front: Side{Rotation: c.front}, Back: Side{Rotation: c.back}}
		f, b, flip := d.Turns()
		assert.Equal(t, [3]any{c.f, c.b, c.flip}, [3]any{f, b, flip}, name)
	}
}

func TestParseReplyAcceptsFencesAndRejectsBadRotations(t *testing.T) {
	good := "```json\n" + `{"front":{"rotation":"90","description":"d","transcription":""},"back":{"rotation":"unknown","description":"","transcription":"hi"},"orientation_evidence":"x"}` + "\n```"
	d, err := parseReply(good)
	require.NoError(t, err)
	assert.Equal(t, "90", d.Front.Rotation)
	assert.Equal(t, "hi", d.Back.Transcription)
	assert.Empty(t, d.Title)

	d, err = parseReply(`{"title":"CNN Center","front":{"rotation":"0","description":"","transcription":""},"back":{"rotation":"0","description":"","transcription":""},"orientation_evidence":"x"}`)
	require.NoError(t, err)
	assert.Equal(t, "CNN Center", d.Title)

	_, err = parseReply(`{"front":{"rotation":"45","description":"","transcription":""},"back":{"rotation":"0","description":"","transcription":""}}`)
	assert.Error(t, err)
	_, err = parseReply(`not json`)
	assert.Error(t, err)
}

func TestFillKeepsWhatIsAlreadyThere(t *testing.T) {
	model := Description{
		Front: Side{Description: "model front", Transcription: "model front text"},
		Back:  Side{Description: "model back", Transcription: "model back text"},
	}
	for name, c := range map[string]struct{ before, after types.Metadata }{
		"empty is filled": {
			types.Metadata{},
			types.Metadata{
				Front: types.Side{Description: "model front", Transcription: types.AnnotatedText{Text: "model front text"}},
				Back:  types.Side{Description: "model back", Transcription: types.AnnotatedText{Text: "model back text"}},
			},
		},
		"existing is kept, the rest filled": {
			types.Metadata{
				Front: types.Side{Description: "mine", Transcription: types.AnnotatedText{Text: "my text"}},
				Back:  types.Side{Transcription: types.AnnotatedText{Text: "my back text"}},
			},
			types.Metadata{
				Front: types.Side{Description: "mine", Transcription: types.AnnotatedText{Text: "my text"}},
				Back:  types.Side{Description: "model back", Transcription: types.AnnotatedText{Text: "my back text"}},
			},
		},
	} {
		meta := c.before
		model.Fill(&meta)
		assert.Equal(t, c.after, meta, name)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"CNN Center":          "cnn-center",
		"Notre-Dame":          "notre-dame",
		"Façade de l'Église":  "facade-de-leglise",
		"St Paul’s":           "st-pauls",
		"  Radley  Beagles! ": "radley-beagles",
		"a b c d e":           "a-b-c",
		"!!!":                 "",
		"":                    "",
	} {
		assert.Equal(t, want, Slug(in), in)
	}
}
