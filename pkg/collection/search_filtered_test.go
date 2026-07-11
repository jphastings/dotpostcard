package collection

import (
	"testing"
	"time"

	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

func TestSearchFilteredFromName(t *testing.T) {
	col := mustCreate(t)

	data, filename := encodeCard(t, "card-claire", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Claire Jones"}
	})
	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	other, otherFilename := encodeCard(t, "card-dave", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Dave Smith"}
	})
	_, err = col.AddWebPostcard(otherFilename, other)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{From: []string{"Claire"}})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "card-claire", results[0].Name)
}

func TestSearchFilteredToName(t *testing.T) {
	col := mustCreate(t)

	data, filename := encodeCard(t, "card-to-claire", func(m *types.Metadata) {
		m.Recipient = types.Person{Name: "Claire Jones"}
	})
	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	other, otherFilename := encodeCard(t, "card-to-dave", func(m *types.Metadata) {
		m.Recipient = types.Person{Name: "Dave Smith"}
	})
	_, err = col.AddWebPostcard(otherFilename, other)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{To: []string{"Claire"}})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "card-to-claire", results[0].Name)
}

func TestSearchFilteredWithMatchesEitherSide(t *testing.T) {
	col := mustCreate(t)

	senderMatch, senderFilename := encodeCard(t, "card-sender-match", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Claire Jones"}
		m.Recipient = types.Person{Name: "Someone Else"}
	})
	_, err := col.AddWebPostcard(senderFilename, senderMatch)
	assert.NoError(t, err)

	recipientMatch, recipientFilename := encodeCard(t, "card-recipient-match", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Someone Else"}
		m.Recipient = types.Person{Name: "Claire Jones"}
	})
	_, err = col.AddWebPostcard(recipientFilename, recipientMatch)
	assert.NoError(t, err)

	noMatch, noMatchFilename := encodeCard(t, "card-no-match", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Someone Else"}
		m.Recipient = types.Person{Name: "Another Person"}
	})
	_, err = col.AddWebPostcard(noMatchFilename, noMatch)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{With: []string{"Claire"}})
	assert.NoError(t, err)

	var names []string
	for _, r := range results {
		names = append(names, r.Name)
	}
	assert.ElementsMatch(t, []string{"card-sender-match", "card-recipient-match"}, names)
}

func TestSearchFilteredWithDoesNotMatchCollector(t *testing.T) {
	col := mustCreate(t)

	data, filename := encodeCard(t, "card-claire-collector", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Someone Else"}
		m.Recipient = types.Person{Name: "Another Person"}
		m.Context.Author = types.Person{Name: "Claire Jones"}
	})
	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{With: []string{"Claire"}})
	assert.NoError(t, err)
	assert.Empty(t, results, "With must match only sender/recipient, never the collector/context author")
}

func TestSearchFilteredURIDistinguishesSameName(t *testing.T) {
	col := mustCreate(t)

	dataA, filenameA := encodeCard(t, "card-claire-a", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Claire Jones", Uri: "https://claire-a.example.com"}
	})
	_, err := col.AddWebPostcard(filenameA, dataA)
	assert.NoError(t, err)

	dataB, filenameB := encodeCard(t, "card-claire-b", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Claire Jones", Uri: "https://claire-b.example.com"}
	})
	_, err = col.AddWebPostcard(filenameB, dataB)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{From: []string{"https://claire-b.example.com"}})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "card-claire-b", results[0].Name)
}

func TestSearchFilteredCountry(t *testing.T) {
	col := mustCreate(t)

	esp, filenameESP := encodeCard(t, "card-esp", func(m *types.Metadata) {
		m.Location.CountryCode = "ESP"
	})
	_, err := col.AddWebPostcard(filenameESP, esp)
	assert.NoError(t, err)

	ita, filenameITA := encodeCard(t, "card-ita", func(m *types.Metadata) {
		m.Location.CountryCode = "ITA"
	})
	_, err = col.AddWebPostcard(filenameITA, ita)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{Country: []string{"ESP"}})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "card-esp", results[0].Name)
}

func TestSearchFilteredTextAndFromCombined(t *testing.T) {
	col := mustCreate(t)

	data, filename := encodeCard(t, "card-match", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Claire Jones"}
		m.Front.Description = "a sunny beach scene"
	})
	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	wrongSender, wrongSenderFilename := encodeCard(t, "card-wrong-sender", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Dave Smith"}
		m.Front.Description = "a sunny beach scene"
	})
	_, err = col.AddWebPostcard(wrongSenderFilename, wrongSender)
	assert.NoError(t, err)

	wrongText, wrongTextFilename := encodeCard(t, "card-wrong-text", func(m *types.Metadata) {
		m.Sender = types.Person{Name: "Claire Jones"}
		m.Front.Description = "a snowy mountain scene"
	})
	_, err = col.AddWebPostcard(wrongTextFilename, wrongText)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{Text: "beach", From: []string{"Claire"}})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "card-match", results[0].Name)
}

func TestSearchFilteredEmptyReturnsEverything(t *testing.T) {
	col := mustCreate(t)

	data, filename := encodeCard(t, "card-one", func(m *types.Metadata) {})
	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "card-one", results[0].Name)
}

func TestSearchFilteredSentOnRange(t *testing.T) {
	col := mustCreate(t)

	inRange, inRangeFilename := encodeCard(t, "card-in-range", func(m *types.Metadata) {
		m.SentOn = &types.Date{Time: time.Date(2020, time.June, 15, 0, 0, 0, 0, time.UTC)}
	})
	_, err := col.AddWebPostcard(inRangeFilename, inRange)
	assert.NoError(t, err)

	onUntilBoundary, onUntilBoundaryFilename := encodeCard(t, "card-on-until-boundary", func(m *types.Metadata) {
		m.SentOn = &types.Date{Time: time.Date(2020, time.July, 1, 0, 0, 0, 0, time.UTC)}
	})
	_, err = col.AddWebPostcard(onUntilBoundaryFilename, onUntilBoundary)
	assert.NoError(t, err)

	beforeRange, beforeRangeFilename := encodeCard(t, "card-before-range", func(m *types.Metadata) {
		m.SentOn = &types.Date{Time: time.Date(2020, time.May, 31, 0, 0, 0, 0, time.UTC)}
	})
	_, err = col.AddWebPostcard(beforeRangeFilename, beforeRange)
	assert.NoError(t, err)

	undated, undatedFilename := encodeCard(t, "card-undated", func(m *types.Metadata) {
		m.SentOn = nil
	})
	_, err = col.AddWebPostcard(undatedFilename, undated)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{SentFrom: "2020-06-01", SentUntil: "2020-07-01"})
	assert.NoError(t, err)
	assert.Len(t, results, 1, "lower bound is inclusive, upper bound is exclusive, and undated cards never match")
	assert.Equal(t, "card-in-range", results[0].Name)
}

func TestSearchFilteredCollectorName(t *testing.T) {
	col := mustCreate(t)

	data, filename := encodeCard(t, "card-collected-by-claire", func(m *types.Metadata) {
		m.Context.Author = types.Person{Name: "Claire Jones"}
	})
	_, err := col.AddWebPostcard(filename, data)
	assert.NoError(t, err)

	other, otherFilename := encodeCard(t, "card-collected-by-dave", func(m *types.Metadata) {
		m.Context.Author = types.Person{Name: "Dave Smith"}
	})
	_, err = col.AddWebPostcard(otherFilename, other)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{Collector: []string{"Claire"}})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "card-collected-by-claire", results[0].Name)
}

func TestSearchFilteredCollectorURI(t *testing.T) {
	col := mustCreate(t)

	dataA, filenameA := encodeCard(t, "card-collector-a", func(m *types.Metadata) {
		m.Context.Author = types.Person{Name: "Claire Jones", Uri: "https://claire-a.example.com"}
	})
	_, err := col.AddWebPostcard(filenameA, dataA)
	assert.NoError(t, err)

	dataB, filenameB := encodeCard(t, "card-collector-b", func(m *types.Metadata) {
		m.Context.Author = types.Person{Name: "Claire Jones", Uri: "https://claire-b.example.com"}
	})
	_, err = col.AddWebPostcard(filenameB, dataB)
	assert.NoError(t, err)

	results, err := col.SearchFiltered(SearchFilter{Collector: []string{"https://claire-b.example.com"}})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "card-collector-b", results[0].Name)
}
