package types

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const maxSlugLength = 60

var stripAccents = runes.Remove(runes.In(unicode.Mn))

// Slug turns a title into a lowercase kebab-case name, with accents stripped, of at most 60
// characters cut at a word boundary; it is empty when the title has no Latin letters or digits.
func Slug(title string) string {
	plain, _, err := transform.String(transform.Chain(norm.NFD, stripAccents, norm.NFC), strings.ToLower(title))
	if err != nil {
		return ""
	}
	// Apostrophes join rather than split, so "St Paul's" is "st-pauls", not "st-paul-s".
	plain = strings.NewReplacer("'", "", "’", "", "ß", "ss", "ø", "o", "æ", "ae", "œ", "oe", "ł", "l", "đ", "d").Replace(plain)
	words := strings.FieldsFunc(plain, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	slug := strings.Join(words, "-")
	if len(slug) <= maxSlugLength {
		return slug
	}
	if i := strings.LastIndex(slug[:maxSlugLength+1], "-"); i > 0 {
		return slug[:i]
	}
	return slug[:maxSlugLength]
}

// TitleFromName derives a display title from a kebab-case name: "cnn-center" is "Cnn Center".
func TitleFromName(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' })
	for i, w := range words {
		r, size := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[size:]
	}
	return strings.Join(words, " ")
}

// EffectiveTitle is the card's own title, or the one derived from its name when it has none.
func (m Metadata) EffectiveTitle(name string) string {
	if strings.TrimSpace(m.Title) != "" {
		return m.Title
	}
	return TitleFromName(name)
}
