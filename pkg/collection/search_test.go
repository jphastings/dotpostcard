package collection

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchFindsCardsByField(t *testing.T) {
	data, filename, wantMeta, _ := encodeSample(t)
	col := mustCreate(t)

	summary, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	for name, query := range map[string]string{
		"sender name":         wantMeta.Sender.Name,
		"location name":       wantMeta.Location.Name,
		"front transcription": wantMeta.Front.Transcription.Text,
	} {
		t.Run(name, func(t *testing.T) {
			results, err := col.Search(query)
			assert.NoError(t, err)
			assert.NotEmpty(t, results)
			assert.Equal(t, summary.Name, results[0].Name)
			assert.NotEmpty(t, results[0].Snippet)
		})
	}
}

func TestSearchNoMatch(t *testing.T) {
	data, filename, _, _ := encodeSample(t)
	col := mustCreate(t)

	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	results, err := col.Search("thiswordisnotinanycard")
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchSpecialCharactersDontError(t *testing.T) {
	data, filename, _, _ := encodeSample(t)
	col := mustCreate(t)

	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	for _, query := range []string{`"`, `*`, `(`, `)`, `foo" OR "bar`, `NEAR(a b)`, ``} {
		t.Run(query, func(t *testing.T) {
			_, err := col.Search(query)
			assert.NoError(t, err)
		})
	}
}
