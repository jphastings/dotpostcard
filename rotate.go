package postcards

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/types"
)

// QuarterTurns converts a clockwise rotation in degrees to quarter turns.
func QuarterTurns(degrees int) (int, error) {
	switch degrees {
	case -90, 0, 90, 180, 270:
		return (degrees/90 + 4) % 4, nil
	}
	return 0, fmt.Errorf("can only rotate by -90, 0, 90, 180 or 270 degrees, not %d", degrees)
}

// RotateFile turns the sides of a single-file web postcard clockwise by the given quarter turns and
// replaces the file atomically, returning the postcard's new flip. On any error the file is untouched.
func RotateFile(path string, front, back int) (types.Flip, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	rotated, flip, err := web.Rotate(data, front, back)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".rotate-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(rotated); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), info.Mode().Perm()); err != nil {
		return "", err
	}
	return flip, os.Rename(tmp.Name(), path)
}
