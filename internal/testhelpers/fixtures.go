package testhelpers

import (
	"embed"
	"fmt"
	"image"
	_ "image/png"
	"math/big"
	"time"

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
		Locale: "en-GB",
		Location: types.Location{
			Name:      "Front, Italy",
			Latitude:  &([]float64{45.28}[0]),
			Longitude: &([]float64{7.66}[0]),
		},
		Flip: "book",
		SentOn: types.Date{
			Time: time.Date(2006, time.January, 2, 0, 0, 0, 0, time.UTC),
		},
		Sender: types.Person{
			Name: "Alice",
			Uri:  "https://alice.example.com",
		},
		Recipient: types.Person{
			Name: "Bob",
			Uri:  "https://bob.example.org",
		},
		Front: types.Side{
			Description: "The word 'Front' in large blue letters",
			Transcription: types.AnnotatedText{
				Text: "Front",
			},
			Secrets: []types.Polygon{{
				Prehidden: true,
				Points: []types.Point{
					{0.3, 0.6},
					{0.3, 0.8},
					{0.4, 0.8},
					{0.4, 0.6},
				},
			}},
		},
		Back: types.Side{
			Description: "The word 'Back' in large red letters",
			Transcription: types.AnnotatedText{
				Text: "Back",
				Annotations: []types.Annotation{{
					Type:  types.ATLocale,
					Value: "en-GB",
					Start: 0,
					End:   4,
				}},
			},
			Secrets: []types.Polygon{{
				Prehidden: true,
				Points: []types.Point{
					{0, 0},
					{0, 0.3},
					{0.1, 0.3},
					{0.1, 0},
				},
			}},
		},
		Context: types.Context{
			Author: types.Person{
				Name: "Carol",
				Uri:  "https://carol.example.net",
			},
			Description: "This is a sample postcard, with all fields expressed.",
		},

		Physical: types.Physical{
			FrontDimensions: types.Size{
				PxWidth:  1480,
				PxHeight: 1050,
				CmWidth:  big.NewRat(148, 10),
				CmHeight: big.NewRat(105, 10),
			},
			ThicknessMM: 0.4,
		},
	},
	Front: testImages["front-landscape.png"],
	Back:  testImages["back-landscape.png"],
}

//go:embed samplexmp.xml
var SampleXMP []byte
