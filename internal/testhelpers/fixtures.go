package testhelpers

import (
	"embed"
	"fmt"
	"image"
	_ "image/png"
	"math/big"

	"github.com/jphastings/dotpostcard/types"
)

var (
	//go:embed *.png
	testImagesData embed.FS

	testImages = make(map[string]image.Image)
)

func init() {
	files, err := testImagesData.ReadDir(".")
	if err != nil {
		panic("couldn't read from embedded filesystem")
	}

	for _, de := range files {
		f, err := testImagesData.Open(de.Name())
		if err != nil {
			panic(fmt.Sprintf("couldn't read '%s' from embedded filesystem: %v", de.Name(), err))
		}

		img, _, err := image.Decode(f)
		if err != nil {
			panic(fmt.Sprintf("embedded test image couldn't be read: %v", err))
		}

		testImages[de.Name()] = img
	}
}

var SamplePostcard = types.Postcard{
	Name: "some-postcard",
	Meta: types.Metadata{
		Flip: "book",
		Physical: types.Physical{
			FrontDimensions: types.Size{
				PxWidth:  1480,
				PxHeight: 1050,
				CmWidth:  big.NewRat(148, 10),
				CmHeight: big.NewRat(105, 10),
			},
		},
		Front: types.Side{
			Description: "The word 'Front' in large blue letters",
		},
		Back: types.Side{
			Description: "The word 'Back' in large red letters",
		},
	},
	Front: testImages["front-landscape.png"],
	Back:  testImages["back-landscape.png"],
}

//go:embed samplexmp.xml
var SampleXMP []byte
