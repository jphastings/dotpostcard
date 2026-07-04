package appcore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
)

// encodePostcard encodes a copy of testhelpers.SamplePostcard (named name)
// via the web codec, returning the resulting file's bytes and filename.
//
// testhelpers.SamplePostcard.Front/Back are nil: fixtures.go builds that
// struct literal from testhelpers.TestImages before TestImages is populated
// by an init() func, which Go runs after package-level var initializers.
// Substitute the real images here, mirroring pkg/collection/helpers_test.go.
func encodePostcard(t *testing.T, name string) (data []byte, filename string) {
	t.Helper()

	pc := testhelpers.SamplePostcard
	pc.Name = name
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	assert.NotNil(t, pc.Front)
	assert.NotNil(t, pc.Back)

	fws, err := web.DefaultCodec.Encode(pc, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, fws)

	data, err = fws[0].Bytes()
	assert.NoError(t, err)

	return data, fws[0].Filename
}

// buildCollection creates a fresh collection file containing one card per
// name given, returning its path.
func buildCollection(t *testing.T, cardNames ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.postcard.db")
	col, err := collection.Create(path)
	assert.NoError(t, err)

	for _, name := range cardNames {
		data, filename := encodePostcard(t, name)
		_, err := col.AddWebPostcard(filename, data)
		assert.NoError(t, err)
	}

	assert.NoError(t, col.Close())
	return path
}

// metaProbe decodes just enough of a types.Metadata JSON blob to assert on
// in tests. types.Metadata can't be round-tripped through encoding/json
// directly when secrets are present: types.Polygon only implements
// json.Unmarshaler, and that Unmarshal requires a "type" discriminator
// field the default (reflection-based) Marshal of a bare types.Polygon never
// writes (see pkg/collection/metadata.go's storedMetadata doc comment).
type metaProbe struct {
	Sender    types.Person `json:"sender"`
	Recipient types.Person `json:"recipient"`
}

func writeBareFile(t *testing.T, filename string, data []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), filename)
	assert.NoError(t, os.WriteFile(path, data, 0644))
	return path
}

func TestOpenCollectionListJSON(t *testing.T) {
	path := buildCollection(t, "card-one")

	c, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c.Close()

	assert.Equal(t, path, c.Path())

	count, err := c.CardCount()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)

	listJSON, err := c.ListJSON()
	assert.NoError(t, err)

	var summaries []collection.CardSummary
	assert.NoError(t, json.Unmarshal([]byte(listJSON), &summaries))
	assert.Len(t, summaries, 1)
	assert.Equal(t, "card-one", summaries[0].Name)
	assert.True(t, summaries[0].HasBack)
	assert.Greater(t, summaries[0].FrontPxW, 0)
}

func TestCollectionSearchJSON(t *testing.T) {
	path := buildCollection(t, "card-one")

	c, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c.Close()

	searchJSON, err := c.SearchJSON("Alice")
	assert.NoError(t, err)

	var results []collection.SearchResult
	assert.NoError(t, json.Unmarshal([]byte(searchJSON), &results))
	assert.NotEmpty(t, results)
	assert.Equal(t, "card-one", results[0].Name)
	assert.NotEmpty(t, results[0].Snippet)
}

func TestCollectionCardImageAndMimetype(t *testing.T) {
	path := buildCollection(t)
	data, filename := encodePostcard(t, "card-one")

	col, err := collection.Open(path)
	assert.NoError(t, err)
	_, err = col.AddWebPostcard(filename, data)
	assert.NoError(t, err)
	assert.NoError(t, col.Close())

	c, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c.Close()

	img, err := c.CardImage("card-one")
	assert.NoError(t, err)
	assert.Equal(t, data, img)

	mimetype, err := c.CardMimetype("card-one")
	assert.NoError(t, err)
	assert.Equal(t, "image/jpeg", mimetype)

	thumb, err := c.Thumbnail("card-one")
	assert.NoError(t, err)
	assert.NotEmpty(t, thumb)
}

func TestCollectionCardMetaJSON(t *testing.T) {
	path := buildCollection(t, "card-one")

	c, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c.Close()

	metaJSON, err := c.CardMetaJSON("card-one")
	assert.NoError(t, err)

	var meta metaProbe
	assert.NoError(t, json.Unmarshal([]byte(metaJSON), &meta))
	assert.Equal(t, "Alice", meta.Sender.Name)
	assert.Equal(t, "Bob", meta.Recipient.Name)
}

func TestCollectionMissingCardErrors(t *testing.T) {
	path := buildCollection(t, "card-one")

	c, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c.Close()

	_, err = c.CardImage("nope")
	assert.Error(t, err)

	_, err = c.CardMimetype("nope")
	assert.Error(t, err)

	_, err = c.CardMetaJSON("nope")
	assert.Error(t, err)

	_, err = c.Thumbnail("nope")
	assert.Error(t, err)
}

func TestOpenCardFile(t *testing.T) {
	data, filename := encodePostcard(t, "bare-card")
	path := writeBareFile(t, filename, data)

	cf, err := OpenCardFile(path)
	assert.NoError(t, err)

	assert.Equal(t, "bare-card", cf.Name())
	assert.Equal(t, path, cf.Path())

	img, err := cf.Image()
	assert.NoError(t, err)
	assert.Equal(t, data, img)

	summaryJSON, err := cf.SummaryJSON()
	assert.NoError(t, err)

	var summary collection.CardSummary
	assert.NoError(t, json.Unmarshal([]byte(summaryJSON), &summary))
	assert.Equal(t, "bare-card", summary.Name)
	assert.Equal(t, filename, summary.Filename)
	assert.Equal(t, "image/jpeg", summary.Mimetype)
	assert.Equal(t, "Alice", summary.SenderName)
	assert.True(t, summary.HasBack)
	assert.Greater(t, summary.FrontPxW, 0)
	assert.Greater(t, summary.FrontPxH, 0)

	metaJSON, err := cf.MetaJSON()
	assert.NoError(t, err)

	var meta metaProbe
	assert.NoError(t, json.Unmarshal([]byte(metaJSON), &meta))
	assert.Equal(t, "Alice", meta.Sender.Name)
}

func TestOpenCardFileMissingFile(t *testing.T) {
	_, err := OpenCardFile(filepath.Join(t.TempDir(), "nope.postcard.webp"))
	assert.Error(t, err)
}

func TestLibrarySearchMergesCollectionAndBareFile(t *testing.T) {
	collectionPath := buildCollection(t, "card-alpha")
	data, filename := encodePostcard(t, "card-bravo")
	barePath := writeBareFile(t, filename, data)

	lib := NewLibrary()
	defer lib.Close()

	sourcesJSON := fmt.Sprintf(`{"collections":[%q],"cards":[%q]}`, collectionPath, barePath)
	assert.NoError(t, lib.SetSourcesJSON(sourcesJSON))

	resultsJSON, err := lib.SearchJSON("Alice")
	assert.NoError(t, err)

	var hits []libraryHit
	assert.NoError(t, json.Unmarshal([]byte(resultsJSON), &hits))
	assert.Len(t, hits, 2)

	// Collection hits come first (in the collection's own rank order),
	// then bare-file hits are appended.
	assert.Equal(t, "card-alpha", hits[0].Card.Name)
	assert.Equal(t, collectionPath, hits[0].Source)

	assert.Equal(t, "card-bravo", hits[1].Card.Name)
	assert.Equal(t, barePath, hits[1].Source)
}

func TestLibrarySearchMergeOrderAcrossCollections(t *testing.T) {
	// buildCollection's temp dirs are created in creation order, so pathA
	// sorts before pathB; that's what determines merge order here since
	// each collection contributes exactly one, equally-ranked hit.
	pathA := buildCollection(t, "card-alpha")
	pathB := buildCollection(t, "card-alpha-2")
	first, second := pathA, pathB
	if pathB < pathA {
		first, second = pathB, pathA
	}

	lib := NewLibrary()
	defer lib.Close()

	sourcesJSON := fmt.Sprintf(`{"collections":[%q,%q]}`, pathB, pathA)
	assert.NoError(t, lib.SetSourcesJSON(sourcesJSON))

	resultsJSON, err := lib.SearchJSON("Alice")
	assert.NoError(t, err)

	var hits []libraryHit
	assert.NoError(t, json.Unmarshal([]byte(resultsJSON), &hits))
	assert.Len(t, hits, 2)

	assert.Equal(t, first, hits[0].Source)
	assert.Equal(t, second, hits[1].Source)
}

func TestLibrarySetSourcesJSONKeepsOpenablePaths(t *testing.T) {
	goodPath := buildCollection(t, "card-good")
	badPath := filepath.Join(t.TempDir(), "missing.postcard.db")

	lib := NewLibrary()
	defer lib.Close()

	err := lib.SetSourcesJSON(fmt.Sprintf(`{"collections":[%q,%q]}`, goodPath, badPath))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), badPath)

	resultsJSON, err := lib.SearchJSON("Alice")
	assert.NoError(t, err)

	var hits []libraryHit
	assert.NoError(t, json.Unmarshal([]byte(resultsJSON), &hits))
	assert.Len(t, hits, 1)
	assert.Equal(t, "card-good", hits[0].Card.Name)
}

func TestLibrarySearchNoMatch(t *testing.T) {
	collectionPath := buildCollection(t, "card-alpha")

	lib := NewLibrary()
	defer lib.Close()

	assert.NoError(t, lib.SetSourcesJSON(fmt.Sprintf(`{"collections":[%q]}`, collectionPath)))

	resultsJSON, err := lib.SearchJSON("thiswordisnotinanycard")
	assert.NoError(t, err)

	var hits []libraryHit
	assert.NoError(t, json.Unmarshal([]byte(resultsJSON), &hits))
	assert.Empty(t, hits)
}
