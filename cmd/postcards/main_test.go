package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetRootCmd gives rootCmd a clean flag state, undoing whatever a previous
// rootCmd.Execute() call in this test binary left behind (pflag only touches
// flags actually present in a given argument list, so stale --overwrite /
// --skip-existing values would otherwise leak between test cases).
func resetRootCmd() {
	rootCmd.ResetFlags()
	registerRootFlags()
}

// captureStdout redirects os.Stdout for the duration of fn and returns what
// was written. RunE writes its per-file progress lines straight to
// os.Stdout (via safeWrite), not through cmd.OutOrStdout(), so this is the
// only way to observe them from a test.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

// writeComponentFixture writes a single-sided ("only") component postcard
// input — front image plus minimal flip:none metadata — named cardName in
// dir, so it decodes to "{cardName}.postcard.jpeg" by default.
func writeComponentFixture(t *testing.T, dir, cardName string, frontData []byte) string {
	t.Helper()

	frontPath := filepath.Join(dir, cardName+"-only.png")
	require.NoError(t, os.WriteFile(frontPath, frontData, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, cardName+"-meta.json"), []byte(`{"flip":"none"}`), 0644))

	return frontPath
}

func TestRunEFailsFastOnExistingOutputWithoutDecoding(t *testing.T) {
	dir := t.TempDir()
	frontPath := writeComponentFixture(t, dir, "sample", []byte("not a real image"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sample.postcard.jpeg"), []byte("already here"), 0644))

	resetRootCmd()
	rootCmd.SetArgs([]string{"-f", "web", "--out-dir", dir, frontPath})

	var runErr error
	stdout := captureStdout(t, func() {
		runErr = rootCmd.Execute()
	})

	require.Error(t, runErr)
	assert.NotContains(t, runErr.Error(), "decode", "the pre-check must block before Decode ever runs, so a corrupt image must never surface as a decode error")
	assert.Contains(t, stdout, "already exists")
}

func TestRunESkipExistingExitsCleanly(t *testing.T) {
	dir := t.TempDir()
	frontPath := writeComponentFixture(t, dir, "sample", []byte("not a real image"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sample.postcard.jpeg"), []byte("already here"), 0644))

	resetRootCmd()
	rootCmd.SetArgs([]string{"-f", "web", "--out-dir", dir, "--skip-existing", frontPath})

	var runErr error
	stdout := captureStdout(t, func() {
		runErr = rootCmd.Execute()
	})

	assert.NoError(t, runErr)
	assert.Contains(t, stdout, "skipping")
}

func TestRunEOverwriteProceedsPastExistingOutput(t *testing.T) {
	dir := t.TempDir()
	frontPath := writeComponentFixture(t, dir, "sample", testhelpers.RawTestImage("sample-front.png"))
	target := filepath.Join(dir, "sample.postcard.jpeg")
	require.NoError(t, os.WriteFile(target, []byte("stale placeholder"), 0644))

	resetRootCmd()
	rootCmd.SetArgs([]string{"-f", "web", "--out-dir", dir, "--overwrite", frontPath})

	runErr := rootCmd.Execute()
	require.NoError(t, runErr)

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.NotEqual(t, "stale placeholder", string(data), "--overwrite must actually replace the existing file")
}

func TestRunEMultiCardSupportFilesDontCollide(t *testing.T) {
	dir := t.TempDir()
	frontData := testhelpers.RawTestImage("sample-front.png")
	frontA := writeComponentFixture(t, dir, "card-a", frontData)
	frontB := writeComponentFixture(t, dir, "card-b", frontData)

	resetRootCmd()
	rootCmd.SetArgs([]string{"-f", "web,html", "--out-dir", dir, frontA, frontB})

	runErr := rootCmd.Execute()
	require.NoError(t, runErr)

	assert.FileExists(t, filepath.Join(dir, "postcards.css"))
	assert.FileExists(t, filepath.Join(dir, "card-a.html"))
	assert.FileExists(t, filepath.Join(dir, "card-b.html"))
}
