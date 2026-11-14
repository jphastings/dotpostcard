package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/pkg/collection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCollectionCreate(t *testing.T, args ...string) error {
	t.Helper()
	collectionCreateCmd.ResetFlags()
	registerCollectionCreateFlags()
	rootCmd.SetArgs(append([]string{"collection", "create"}, args...))
	var err error
	captureStdout(t, func() { err = rootCmd.Execute() })
	return err
}

func writeWebCard(t *testing.T, dir string) string {
	t.Helper()
	pc := testhelpers.SamplePostcard
	pc.Name = "alpha"
	pc.Meta.Front.Secrets, pc.Meta.Back.Secrets = nil, nil
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	fws, err := web.DefaultCodec.Encode(pc, nil)
	require.NoError(t, err)
	_, err = fws[0].WriteFile(dir, true)
	require.NoError(t, err)
	return filepath.Join(dir, fws[0].Filename)
}

func TestCollectionCreateWritesAFileNamedAfterTheTitleByDefault(t *testing.T) {
	cards, out := t.TempDir(), t.TempDir()
	writeWebCard(t, cards)

	require.NoError(t, runCollectionCreate(t, "--title", "Blanks", "--out-dir", out, cards))

	coll, err := collection.OpenReadOnly(filepath.Join(out, "blanks.postcards"))
	require.NoError(t, err)
	defer coll.Close()
	count, err := coll.Count()
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestCollectionCreateRefusesBadInput(t *testing.T) {
	cards, out := t.TempDir(), t.TempDir()
	writeWebCard(t, cards)

	assert.ErrorContains(t, runCollectionCreate(t, "-f", "zip", "--title", "Blanks", "--out-dir", out, cards), "zip")
	assert.ErrorContains(t, runCollectionCreate(t, "--out-dir", out, cards), "title")

	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
