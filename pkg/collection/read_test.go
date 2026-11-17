package collection

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCardNamesBySHA256(t *testing.T) {
	col := mustCreate(t)
	want := map[[sha256.Size]byte]string{}
	for _, name := range []string{"first-card", "second-card"} {
		data, filename := encodeNamed(t, name, nil)
		_, err := col.AddWebPostcard(filename, data)
		assert.NoError(t, err)
		want[sha256.Sum256(data)] = name
	}

	got, err := col.CardNamesBySHA256()
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}
