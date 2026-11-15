package inprogress

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func birthtime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	ts := info.Sys().(*syscall.Stat_t).Birthtimespec
	return time.Unix(ts.Sec, ts.Nsec)
}

func fraction(path string) string {
	buf := make([]byte, 32)
	n, err := unix.Getxattr(path, fractionAttr, buf)
	if err != nil {
		return ""
	}
	return string(buf[:n])
}

func TestMarkedFileShowsItsProgressUntilCleared(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.postcards")
	require.NoError(t, os.WriteFile(path, nil, 0o644))

	Mark(path)
	SetFraction(path, 0.25)
	assert.Equal(t, marker, birthtime(t, path).UTC())
	assert.Equal(t, "0.25", fraction(path))

	Clear(path)
	assert.WithinDuration(t, time.Now(), birthtime(t, path), time.Minute)
	assert.Empty(t, fraction(path))
}

func TestMarkingAMissingPathDoesNothing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")
	assert.NotPanics(t, func() {
		Mark(missing)
		SetFraction(missing, 0.5)
		Clear(missing)
	})
}
