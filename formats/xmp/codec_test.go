package xmp

import (
	"bytes"
	"io"
	"testing"

	"github.com/jphastings/postcards/formats"
	"github.com/jphastings/postcards/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestBundle(t *testing.T) {
	xmpFilenames := []string{"sample.xmp"}
	anyOldFilenames := []string{"any.jpg", "other.json", "files-meta.yaml"}
	files, dir := testhelpers.TestFiles(append(anyOldFilenames, xmpFilenames...))

	bnd, remaining, errs := Codec().Bundle(files, dir)
	assert.Empty(t, errs)

	assert.Len(t, bnd, 1)

	r, ok := bnd[0].(io.Reader)
	assert.True(t, ok, "bundle did not contain a reader")

	data, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, testhelpers.DataForTestFile("sample.xmp"), data)

	assert.Equal(t, anyOldFilenames, testhelpers.Filenames(remaining))
}

// sampleXMP is the fixture that equates to the samplePostcard test fixture
var sampleXMP = []byte(`<?xpacket begin='' id='W5M0MpCehiHzreSzNTczkc9d'?><x:xmpmeta xmlns:x="adobe:ns:meta/" xmlns:xmptk="postcards/v0.1"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description xmlns:tiff="http://ns.adobe.com/tiff/1.0/"><tiff:ImageWidth>1480</tiff:ImageWidth><tiff:ImageLength>2100</tiff:ImageLength><tiff:ResolutionUnit>3</tiff:ResolutionUnit><tiff:XResolution>74/5</tiff:XResolution><tiff:YResolution>21</tiff:YResolution></rdf:Description><rdf:Description xmlns:Iptc4xmpCore="http://iptc.org/std/Iptc4xmpCore/1.0/xmlns/"><Iptc4xmpCore:AltTextAccessibility><rdf:Alt><rdf:li xml:lang="en">On the front of a postcard: The word &#39;Front&#39; in large red letters%!(EXTRA string=The word &#39;Front&#39; in large red letters, string=)</rdf:li></rdf:Alt></Iptc4xmpCore:AltTextAccessibility></rdf:Description><rdf:Description xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:description><rdf:Alt><rdf:li xml:lang="en">Both sides of a postcard, stored in the &#39;.postcard&#39; format (https://dotpostcard.org)</rdf:li></rdf:Alt></dc:description></rdf:Description><rdf:Description xmlns:Postcard="https://dotpostcard.org/xmp/1.0/"><Postcard:Flip>book</Postcard:Flip><Postcard:Sender></Postcard:Sender><Postcard:Recipient></Postcard:Recipient><Postcard:Context xml:lang=""></Postcard:Context><Postcard:ContextAuthor></Postcard:ContextAuthor></rdf:Description></rdf:RDF></x:xmpmeta><?xpacket end='w'?>`)

func TestEncode(t *testing.T) {
	fws := Codec().Encode(testhelpers.SamplePostcard, formats.EncodeOptions{})

	assert.Len(t, fws, 1)

	content, err := fws[0].Bytes()
	assert.NoError(t, err)

	assert.Equal(t, sampleXMP, content)
}

func TestDecode(t *testing.T) {
	bnd := bundle{bytes.NewReader(sampleXMP)}

	pc, err := bnd.Decode()
	_ = pc
	assert.Error(t, err, "decoding is not yet implemented")

	// assert.NoError(t, err)
	// assert.Equal(t, testhelpers.SamplePostcard.Meta, pc.Meta)
}
