package postcards

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRotateFileReplacesInPlaceKeepingModeAndLeavingNoTempFiles(t *testing.T) {
	pc := testhelpers.SamplePostcard
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	fws, err := web.PostcardCodec.Encode(pc, &formats.EncodeOptions{Quality: formats.Archive})
	require.NoError(t, err)
	data, err := fws[0].Bytes()
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "card.postcard")
	require.NoError(t, os.WriteFile(path, data, 0o640))

	flip, err := RotateFile(path, 1, 0)

	require.NoError(t, err)
	assert.Equal(t, types.FlipLeftHand, flip)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	rotated, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotEqual(t, data, rotated)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestRotateFileLeavesNonPostcardsUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.postcard")
	require.NoError(t, os.WriteFile(path, []byte("not an image"), 0o644))

	_, err := RotateFile(path, 1, 0)

	assert.Error(t, err)
	after, _ := os.ReadFile(path)
	assert.Equal(t, []byte("not an image"), after)
}
