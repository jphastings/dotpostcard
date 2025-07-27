package images

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"strings"
)

func ReadSVG(r io.Reader) (image.Image, []byte, error) {
	imgBytes, err := extractImageHref(r)
	if err != nil {
		return nil, nil, err
	}
	return ReadJPEG(bytes.NewReader(imgBytes))
}

const base64prefix = "data:image/jpeg;base64,"

func extractImageHref(r io.Reader) ([]byte, error) {
	decoder := xml.NewDecoder(r)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("no postcard image found in SVG")
		} else if err != nil {
			return nil, fmt.Errorf("unable to parse SVG: %w", err)
		}

		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "image" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "href" {
						if strings.HasPrefix(attr.Value, base64prefix) {
							b64 := attr.Value[len(base64prefix):]
							return base64.StdEncoding.DecodeString(b64)
						}
					}
				}
			}
		}
	}
}
