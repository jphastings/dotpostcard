package atproto

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoImageToolchainDependency guards the whole point of pulling pdsbundle out of this
// package: consumers of the record types and TID rule (eg. an indexer) must be able to import
// formats/atproto without dragging in the image toolchain (webp/jpegli/wazero/EXIF) that
// formats/web needs.
func TestNoImageToolchainDependency(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	require.NoError(t, err)

	deps := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, forbidden := range []string{
		"github.com/jphastings/dotpostcard/formats/web",
		"github.com/jphastings/dotpostcard/pkg/collection",
		"modernc.org/sqlite",
		"github.com/mattn/go-sqlite3",
	} {
		assert.NotContains(t, deps, forbidden)
	}
}
