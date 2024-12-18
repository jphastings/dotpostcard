package xmpinject_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/pkg/xmpinject"
	"github.com/stretchr/testify/assert"
)

func TestXMPintoPNG(t *testing.T) {
	original, err := os.ReadFile("1px-nometa.png")
	assert.NoError(t, err)
	want, err := os.ReadFile("1px-xmp.png")
	assert.NoError(t, err)

	var b bytes.Buffer
	assert.NoError(t, xmpinject.XMPintoPNG(&b, original, testhelpers.SampleXMP))

	assert.Equal(t, want, b.Bytes())
}

func TestXMPfromPNG(t *testing.T) {
	original, err := os.ReadFile("1px-xmp.png")
	assert.NoError(t, err)

	xmpData, err := xmpinject.XMPfromPNG(original)
	assert.NoError(t, err)

	// We don't mind whether the XMPdata is writeable in place or not, and the samplexmp data declares that it is
	// so swap this out here for a passing test
	xmpData = bytes.Replace(xmpData, []byte("<?xpacket end='r'?>"), []byte("<?xpacket end='w'?>"), 1)

	assert.Equal(t, testhelpers.SampleXMP, xmpData)
}
