package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlug(t *testing.T) {
	longWords := strings.Repeat("abcdefghi ", 10)
	for in, want := range map[string]string{
		"CNN Center":          "cnn-center",
		"Notre-Dame de Paris": "notre-dame-de-paris",
		"Façade de l'Église":  "facade-de-leglise",
		"St Paul’s":           "st-pauls",
		"  Radley  Beagles! ": "radley-beagles",
		"Straße Øresund Æble Œuvre Łódź Đà": "strasse-oresund-aeble-oeuvre-lodz-da",
		"a b c d e":             "a-b-c-d-e",
		"東京タワー":                 "",
		"!!!":                   "",
		"":                      "",
		longWords:               strings.TrimSuffix(strings.Repeat("abcdefghi-", 6), "-"),
		strings.Repeat("x", 80): strings.Repeat("x", 60),
	} {
		assert.Equal(t, want, Slug(in), in)
	}
}

func TestTitleFromNameRoundTripsThroughSlug(t *testing.T) {
	for _, name := range []string{"cnn-center", "notre-dame-de-paris", "st-pauls", "1923-beach"} {
		assert.Equal(t, name, Slug(TitleFromName(name)), name)
	}
	assert.Equal(t, "Cnn Center", TitleFromName("cnn-center"))
}

func TestEffectiveTitle(t *testing.T) {
	assert.Equal(t, "Own Title", Metadata{Title: "Own Title"}.EffectiveTitle("cnn-center"))
	assert.Equal(t, "Cnn Center", Metadata{Title: "  "}.EffectiveTitle("cnn-center"))
}
