package describe

import (
	"strings"
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
	good := "```json\n" + `{"front":{"rotation":"90","description":"d","transcription":"","secrets":[]},"back":{"rotation":"unknown","description":"","transcription":"hi","secrets":[]},"recipient":"","sender":"","sent_on":"","locale":"","location":{"name":"","latitude":null,"longitude":null,"country_code":""},"orientation_evidence":"x"}` + "\n```"
	d, err := parseReply(good)
	require.NoError(t, err)
	assert.Equal(t, "90", d.Front.Rotation)
	assert.Equal(t, "hi", d.Back.Transcription)
	assert.Empty(t, d.Title)

	d, err = parseReply(`{"title":"CNN Center","front":{"rotation":"0","description":"","transcription":"","secrets":[]},"back":{"rotation":"0","description":"","transcription":"","secrets":[]},"recipient":"","sender":"","sent_on":"","locale":"","location":{"name":"","latitude":null,"longitude":null,"country_code":""},"orientation_evidence":"x"}`)
	require.NoError(t, err)
	assert.Equal(t, "CNN Center", d.Title)

	_, err = parseReply(`{"front":{"rotation":"45","description":"","transcription":"","secrets":[]},"back":{"rotation":"0","description":"","transcription":"","secrets":[]}}`)
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

func TestFillSetsTheTitleOnlyWhenEmpty(t *testing.T) {
	printed := Description{Title: "Grüße aus Köln, 1962!"}

	var untitled types.Metadata
	printed.Fill(&untitled)
	assert.Equal(t, "Grüße aus Köln, 1962!", untitled.Title, "a printed caption is copied verbatim")

	titled := types.Metadata{Title: "Mine"}
	printed.Fill(&titled)
	assert.Equal(t, "Mine", titled.Title)

	var noTitle types.Metadata
	Description{}.Fill(&noTitle)
	assert.Empty(t, noTitle.Title)
}

const richReply = `{"title":"t","recipient":"Mr A. Smith","sender":"Jo","sent_on":"1987-08-14","locale":"en-GB",
"location":{"name":"Lyme Regis, UK","latitude":50.72,"longitude":-2.93,"country_code":"gbr"},"orientation_evidence":"x",
"front":{"rotation":"0","description":"","transcription":"","secrets":[]},
"back":{"rotation":"0","description":"","transcription":"hi","secrets":[[[0.5,0.5],[1.5,0.5],[1.5,-1]],[[0,0],[1,1]]]}}`

func TestParseReplyReadsTheHandwrittenFieldsAndFillKeepsExistingOnes(t *testing.T) {
	d, err := parseReply(richReply)
	require.NoError(t, err)
	assert.Empty(t, d.Notes)
	assert.Equal(t, [][][2]float64{{{0.5, 0.5}, {1, 0.5}, {1, 0}}}, d.Back.Secrets, "clamped; the two-point polygon is dropped")

	var meta types.Metadata
	d.Fill(&meta)
	assert.Equal(t, "Mr A. Smith", meta.Recipient.Name)
	assert.Equal(t, "Jo", meta.Sender.Name)
	assert.Equal(t, "1987-08-14", meta.SentOn.Format("2006-01-02"))
	assert.Equal(t, "en-GB", meta.Locale)
	assert.Equal(t, "Lyme Regis, UK", meta.Location.Name)
	assert.Equal(t, "GBR", meta.Location.CountryCode)
	assert.Equal(t, -2.93, *meta.Location.Longitude)
	assert.Empty(t, meta.Back.Secrets, "a decoded card can't hide secrets")

	kept := types.Metadata{Recipient: types.Person{Name: "Mine"}, Locale: "fr-FR", Location: types.Location{Name: "Paris"}}
	d.Fill(&kept)
	assert.Equal(t, "Mine", kept.Recipient.Name)
	assert.Equal(t, "fr-FR", kept.Locale)
	assert.Equal(t, types.Location{Name: "Paris"}, kept.Location)
}

func TestParseReplyDropsBadOptionalValuesWithoutFailing(t *testing.T) {
	bad := strings.NewReplacer(`"1987-08-14"`, `"14 Aug"`, `"latitude":50.72`, `"latitude":95`, `"gbr"`, `"UK"`).Replace(richReply)
	d, err := parseReply(bad)
	require.NoError(t, err)
	assert.Nil(t, d.SentOn)
	assert.Contains(t, d.Notes[0], "14 Aug")
	assert.Equal(t, "Lyme Regis, UK", d.Location.Name)
	assert.Nil(t, d.Location.Latitude)
	assert.Nil(t, d.Location.Longitude)
	assert.Empty(t, d.Location.CountryCode)
}

func TestPolygonsFollowTheSidesQuarterTurns(t *testing.T) {
	s := Side{Secrets: [][][2]float64{{{0.25, 0.5}, {0.5, 0.5}, {0.5, 1}}}}
	assert.Equal(t, types.Point{X: 0.25, Y: 0.5}, s.Polygons(0)[0].Points[0])
	assert.Equal(t, []types.Point{{X: 0.5, Y: 0.25}, {X: 0.5, Y: 0.5}, {X: 0, Y: 0.5}}, s.Polygons(1)[0].Points)
	assert.False(t, s.Polygons(1)[0].Prehidden)
}
