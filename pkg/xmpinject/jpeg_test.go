package xmpinject_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
	"github.com/stretchr/testify/assert"
)

func TestXMPintoJPEG(t *testing.T) {
	original, err := os.ReadFile("1px-nometa.jpg")
	assert.NoError(t, err)
	want, err := os.ReadFile("1px-xmp.jpg")
	assert.NoError(t, err)

	var b bytes.Buffer
	assert.NoError(t, xmpinject.XMPintoJPEG(&b, original, testhelpers.SampleXMP))

	// It's not ideal; but if I need to recreate the fixture data I use this
	// f, _ := os.OpenFile("1px-xmp.jpg", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	// _ = xmpinject.XMPintoJPEG(f, original, testhelpers.SampleXMP)

	assert.Equal(t, want, b.Bytes())
}

func TestXMPfromJPEG(t *testing.T) {
	original, err := os.ReadFile("1px-xmp.jpg")
	assert.NoError(t, err)

	xmpData, err := xmpinject.XMPfromJPEG(original)
	assert.NoError(t, err)

	assert.Equal(t, testhelpers.SampleXMP, xmpData)
}
