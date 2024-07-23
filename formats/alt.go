package formats

import (
	"fmt"

	"github.com/jphastings/postcards/types"
)

var (
	// The first %s will contain the description of the front of the postcard, and the second the transcription of the back.
	altTextFormats = map[string][]string{
		"en": {
			"On the front of a postcard: %s",
			"Both sides of a postcard. On the front: %s On the back: %s",
		},
	}
)

// AltText returns a descriptive text for a given postcard that's suitable for those unable to see it.
func AltText(meta types.Metadata, lang string) (string, string) {
	alts, ok := altTextFormats[lang]
	if !ok {
		alts = altTextFormats["en"]
		lang = "en"
	}

	var alt string

	if meta.Back.Transcription == "" {
		alt = fmt.Sprintf(alts[0], meta.Front.Description)
	} else {
		alt = fmt.Sprintf(alts[1], meta.Front.Description, meta.Back.Transcription)
	}

	return alt, lang
}
