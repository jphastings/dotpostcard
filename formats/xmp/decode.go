package xmp

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"time"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/types"
	"github.com/trimmer-io/go-xmp/xmp"
)

func (b bundle) Decode(_ *formats.DecodeOptions) (types.Postcard, error) {
	meta, _, err := MetadataFromXMP(b.r)
	// TODO: How do I get the name here?
	return types.Postcard{Meta: meta}, err
}

type xmpAlt struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

type xmpRegion struct {
	Names    []xmpAlt `json:"Iptc4xmpExt:Name"`
	Boundary struct {
		Shape    string `json:"Iptc4xmpExt:rbShape"`
		Unit     string `json:"Iptc4xmpExt:rbUnit"`
		Vertices []struct {
			X string `json:"Iptc4xmpExt:rbX"`
			Y string `json:"Iptc4xmpExt:rbY"`
		} `json:"Iptc4xmpExt:rbVertices"`
	} `json:"Iptc4xmpExt:RegionBoundary"`
}

type xmpJSON struct {
	Models struct {
		Iptc4xmpExt struct {
			Regions []xmpRegion `json:"Iptc4xmpExt:ImageRegion"`
		} `json:"Iptc4xmpExt"`
		Postcard struct {
			Context             []xmpAlt   `json:"Postcard:Context"`
			ContextAuthor       string     `json:"Postcard:ContextAuthor"`
			DescriptionFront    string     `json:"Postcard:DescriptionFront"`
			DescriptionBack     string     `json:"Postcard:DescriptionBack"`
			Flip                types.Flip `json:"Postcard:Flip"`
			Sender              string     `json:"Postcard:Sender"`
			Recipient           string     `json:"Postcard:Recipient"`
			TranscriptionFront  string     `json:"Postcard:TranscriptionFront"`
			TranscriptionBack   string     `json:"Postcard:TranscriptionBack"`
			PhysicalThicknessMM string     `json:"Postcard:PhysicalThicknessMM"`
		} `json:"Postcard"`
		EXIF struct {
			Date         string `json:"exif:DateTimeOriginal"`
			LocationName string `json:"exif:GPSAreaInformation"`
			Latitude     string `json:"exif:GPSLatitude"`
			Longitude    string `json:"exif:GPSLongitude"`
		} `json:"exif"`
	} `json:"models"`
}

func MetadataFromXMP(r io.Reader) (types.Metadata, types.Size, error) {
	d := xmp.NewDecoder(r)
	doc := &xmp.Document{}
	if err := d.Decode(doc); err != nil {
		return types.Metadata{}, types.Size{}, err
	}

	jb, err := doc.MarshalJSON()
	if err != nil {
		return types.Metadata{}, types.Size{}, fmt.Errorf("unable to parse contents of XMP: %w", err)
	}

	var js xmpJSON
	if err := json.Unmarshal(jb, &js); err != nil {
		return types.Metadata{}, types.Size{}, fmt.Errorf("unable to parse contents of JSONified XMP: %w", err)
	}

	var meta types.Metadata

	if len(js.Models.Postcard.Context) > 0 {
		meta.Locale = js.Models.Postcard.Context[0].Lang
		meta.Context.Description = js.Models.Postcard.Context[0].Value
		meta.Context.Author = scanPerson(js.Models.Postcard.ContextAuthor)
	}

	meta.Flip = js.Models.Postcard.Flip
	meta.Sender = scanPerson(js.Models.Postcard.Sender)
	meta.Recipient = scanPerson(js.Models.Postcard.Recipient)
	meta.Front.Description = js.Models.Postcard.DescriptionFront
	meta.Back.Description = js.Models.Postcard.DescriptionBack
	if thick, err := strconv.ParseFloat(js.Models.Postcard.PhysicalThicknessMM, 64); err == nil {
		meta.Physical.ThicknessMM = thick
	}

	json.Unmarshal([]byte(js.Models.Postcard.TranscriptionFront), &meta.Front.Transcription)
	json.Unmarshal([]byte(js.Models.Postcard.TranscriptionBack), &meta.Back.Transcription)

	if date, err := time.Parse(`2006-01-02`, js.Models.EXIF.Date); err == nil {
		meta.SentOn = types.Date{Time: date}
	}
	meta.Location.Name = js.Models.EXIF.LocationName
	meta.Location.Latitude = scanDegrees(js.Models.EXIF.Latitude)
	meta.Location.Longitude = scanDegrees(js.Models.EXIF.Longitude)

	front, back := extractSecrets(meta.Flip, js.Models.Iptc4xmpExt.Regions)
	meta.Front.Secrets = front
	meta.Back.Secrets = back

	xPaths, err := doc.ListPaths()
	if err != nil {
		return types.Metadata{}, types.Size{}, err
	}

	var size types.Size
	// X, Y, scaling factor
	var resolution [3]*big.Rat

	for _, xPath := range xPaths {
		switch xPath.Path {
		case "tiff:ImageWidth":
			size.PxWidth, _ = strconv.Atoi(xPath.Value)
		case "tiff:ImageLength":
			size.PxHeight, _ = strconv.Atoi(xPath.Value)
		case "tiff:XResolution":
			res, err := scanBigRat(xPath.Value)
			if err == nil {
				resolution[0] = res
			}
		case "tiff:YResolution":
			res, err := scanBigRat(xPath.Value)
			if err == nil {
				resolution[1] = res
			}
		case "tiff:ResolutionUnit":
			if xPath.Value != "3" {
				// Assume inches
				resolution[2] = big.NewRat(100, 254)
			}
		}
	}

	if meta.Flip != types.FlipNone {
		size.PxHeight /= 2
	}

	if resolution[0] != nil {
		// Convert units, if necessary
		if resolution[2] != nil {
			resolution[0].Mul(resolution[0], resolution[2])
			resolution[1].Mul(resolution[1], resolution[2])
		}

		size.SetResolution(resolution[0], resolution[1])
		// Postcard height is half reported dimensions if Flip isn't none (ie. this is a stacked web format postcard image)
		if meta.Flip != types.FlipNone {
			size.CmHeight.Quo(size.CmHeight, big.NewRat(2, 1))
		}
		meta.Physical.FrontDimensions = size
	}

	return meta, size, nil
}

func scanBigRat(str string) (*big.Rat, error) {
	var a, b int64
	if _, err := fmt.Sscanf(str, "%d/%d", &a, &b); err != nil {
		return nil, err
	}
	return big.NewRat(a, b), nil
}

func scanDegrees(str string) *float64 {
	var deg int
	var min float64
	dir := str[len(str)-1:]
	num := str[:len(str)-1]

	if _, err := fmt.Sscanf(num, "%d,%f", &deg, &min); err != nil {
		return nil
	}

	fl := float64(deg) + min/60
	switch dir {
	case "N":
		if fl > 90 || fl < 0 {
			return nil
		}
	case "E":
		if fl > 180 || fl < 0 {
			return nil
		}
	case "S":
		if fl > 90 || fl < 0 {
			return nil
		}
		fl *= -1
	case "W":
		if fl > 180 || fl < 0 {
			return nil
		}
		fl *= -1
	default:
		return nil
	}

	return &fl
}

func scanPerson(str string) types.Person {
	var p types.Person
	p.Scan(str)
	return p
}
