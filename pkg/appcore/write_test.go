package appcore

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/stretchr/testify/assert"
)

func TestSetCollectionTitleRoundTrips(t *testing.T) {
	path := buildCollection(t)

	assert.NoError(t, SetCollectionTitle(path, "My Trip"))

	c, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c.Close()

	title, err := c.Title()
	assert.NoError(t, err)
	assert.Equal(t, "My Trip", title)
}

func TestAddCardToCollectionAdds(t *testing.T) {
	path := buildCollection(t)
	data, filename := encodePostcard(t, "card-one")

	summaryJSON, err := AddCardToCollection(path, filename, data)
	assert.NoError(t, err)

	var summary collection.CardSummary
	assert.NoError(t, json.Unmarshal([]byte(summaryJSON), &summary))
	assert.Equal(t, "card-one", summary.Name)

	c, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c.Close()

	count, err := c.CardCount()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestAddCardToCollectionGarbageErrorsAndLeavesCollectionUnchanged(t *testing.T) {
	path := buildCollection(t, "card-one")

	_, err := AddCardToCollection(path, "garbage.postcard.webp", []byte("not a real image"))
	assert.Error(t, err)

	c, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c.Close()

	count, err := c.CardCount()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count, "a failed add must not leave a row behind")
}

func TestRemoveCardFromCollectionRemoves(t *testing.T) {
	path := buildCollection(t, "card-one")

	assert.NoError(t, RemoveCardFromCollection(path, "card-one"))

	c, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c.Close()

	count, err := c.CardCount()
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestRemoveCardFromCollectionMissingErrors(t *testing.T) {
	path := buildCollection(t, "card-one")

	err := RemoveCardFromCollection(path, "nope")
	assert.Error(t, err)
}

func TestCreateCollectionCreatesTitledEmptyCollection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.postcards")

	assert.NoError(t, CreateCollection(path, "Fresh Start"))

	c, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c.Close()

	title, err := c.Title()
	assert.NoError(t, err)
	assert.Equal(t, "Fresh Start", title)

	count, err := c.CardCount()
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestCreateCollectionErrorsIfFileExists(t *testing.T) {
	path := buildCollection(t)

	err := CreateCollection(path, "")
	assert.Error(t, err)
}

func TestWriteSucceedsWhileReadOnlyHandleOpen(t *testing.T) {
	path := buildCollection(t, "card-one")

	// This mirrors the app's real usage pattern: a long-lived read-only
	// Collection handle stays open on path while a transient write runs
	// against the same file. The write must succeed rather than hit
	// SQLITE_BUSY or a lock conflict.
	c, err := OpenCollection(path)
	assert.NoError(t, err)

	assert.NoError(t, SetCollectionTitle(path, "Updated While Open"))
	assert.NoError(t, c.Close())

	// After invalidating and reopening the read handle (as the app's
	// GoCore.invalidateSource does), the change is visible.
	c2, err := OpenCollection(path)
	assert.NoError(t, err)
	defer c2.Close()

	title, err := c2.Title()
	assert.NoError(t, err)
	assert.Equal(t, "Updated While Open", title)
}
