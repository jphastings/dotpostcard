package postcards

import (
	"errors"
	"os"
	"path"

	"github.com/jphastings/dotpostcard/formats"
)

// ExistingOutputs returns the paths in targetDir that the given bundle would collide with,
// across all the requested codecs. An empty result means it is safe to decode.
//
// Bundles with no filesystem origin (formats.Bundle.CardName() == "") always return nil:
// there's nothing to predict a collision against.
func ExistingOutputs(b formats.Bundle, codecs []formats.Codec, opts *formats.EncodeOptions, targetDir string) []string {
	cardName := b.CardName()
	if cardName == "" {
		return nil
	}

	var existing []string
	seen := make(map[string]bool)

	for _, codec := range codecs {
		for _, name := range codec.OutputNames(cardName, opts) {
			p := path.Join(targetDir, name)
			if seen[p] {
				continue
			}

			// Anything other than "definitely absent" is treated as a collision: the
			// pre-check only decides whether decoding is worth attempting, and WriteFile's
			// O_EXCL remains the authoritative guard, so erring towards blocking here just
			// costs an --overwrite retry rather than risking a silently dropped output.
			if _, err := os.Stat(p); err == nil || !errors.Is(err, os.ErrNotExist) {
				seen[p] = true
				existing = append(existing, p)
			}
		}
	}

	return existing
}
