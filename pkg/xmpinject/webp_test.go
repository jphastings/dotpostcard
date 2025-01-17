package xmpinject_test

import (
	"bytes"
	"image"
	"os"
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
	"github.com/stretchr/testify/assert"
)

func TestXMPintoWebP(t *testing.T) {
	original, err := os.ReadFile("1px-nometa.webp")
	assert.NoError(t, err)
	want, err := os.ReadFile("1px-xmp.webp")
	assert.NoError(t, err)

	var b bytes.Buffer
	assert.NoError(t, xmpinject.XMPintoWebP(&b, original, testhelpers.SampleXMP, image.Rect(0, 0, 1, 1), false))

	assert.Equal(t, want, b.Bytes())
}

func TestXMPfromWebP(t *testing.T) {
	original, err := os.ReadFile("1px-xmp.webp")
	assert.NoError(t, err)

	xmpData, err := xmpinject.XMPfromWebP(original)
	assert.NoError(t, err)

	assert.Equal(t, testhelpers.SampleXMP, xmpData)
}
