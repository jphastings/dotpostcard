package web_test

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
)

// Decode a web-format image into a postcard whose sides are separate and upright,
// whatever way up the web format stored the back.
func ExampleBundleFromReader() {
	file := heterorientedWebFile()

	pc, err := web.BundleFromReader(io.NopCloser(bytes.NewReader(file)), "card.postcard.png").
		Decode(formats.DecodeOptions{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("front:", pc.Front.Bounds().Dx(), "x", pc.Front.Bounds().Dy())
	fmt.Println("back:", pc.Back.Bounds().Dx(), "x", pc.Back.Bounds().Dy())
	// Output:
	// front: 672 x 947
	// back: 947 x 672
}

// heterorientedWebFile fabricates the input: a portrait front with a landscape back, which
// the web format stores with the back turned to match the front's width.
func heterorientedWebFile() []byte {
	fws, err := web.DefaultCodec.Encode(testhelpers.HeterorientedLeftHand(), nil)
	if err != nil {
		log.Fatal(err)
	}
	data, err := fws[0].Bytes()
	if err != nil {
		log.Fatal(err)
	}
	return data
}
