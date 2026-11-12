package formats_test

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileLeavesNoFileWhenEncodingFails(t *testing.T) {
	dir := t.TempDir()
	fw := formats.NewFileWriter("card.png", "image/png", func(io.Writer) error { return errors.New("boom") })

	_, err := fw.WriteFile(dir, false)
	require.Error(t, err)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
