package xmp

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"time"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/resolution"
	"github.com/jphastings/dotpostcard/types"
	"github.com/trimmer-io/go-xmp/xmp"
)

func (b bundle) Decode(_ formats.DecodeOptions) (types.Postcard, error) {
	meta, err := MetadataFromXMP(b.r)
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
		TIFF tiffTags `json:"tiff"`
	} `json:"models"`
}

type tiffTags struct {
	Height string `json:"tiff:ImageLength"`
	Width  string `json:"tiff:ImageWidth"`

	ResUnit string   `json:"tiff:ResolutionUnit"`
	XRes    *big.Rat `json:"tiff:XResolution"`
	YRes    *big.Rat `json:"tiff:YResolution"`
}

func MetadataFromXMP(r io.Reader) (types.Metadata, error) {
	d := xmp.NewDecoder(r)
	doc := &xmp.Document{}
	if err := d.Decode(doc); err != nil {
		return types.Metadata{}, err
	}

	jb, err := doc.MarshalJSON()
	if err != nil {
		return types.Metadata{}, fmt.Errorf("unable to parse contents of XMP: %w", err)
	}

	var js xmpJSON
	if err := json.Unmarshal(jb, &js); err != nil {
		return types.Metadata{}, fmt.Errorf("unable to parse contents of JSONified XMP: %w", err)
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

	sides := 2
	if meta.Flip == types.FlipNone {
		sides = 1
	}

	meta.Physical.FrontDimensions = tiffXMPToSize(js.Models.TIFF, sides)

	return meta, nil
}

func tiffXMPToSize(tiff tiffTags, sidesInHeight int) (s types.Size) {
	w, wErr := strconv.ParseInt(tiff.Width, 10, 0)
	h, hErr := strconv.ParseInt(tiff.Height, 10, 0)
	if wErr != nil && hErr != nil {
		return types.Size{}
	}

	s.PxWidth = int(w)
	s.PxHeight = int(h) / sidesInHeight

	// Fallback to the default if it's not parseable
	exifUnit, _ := strconv.ParseUint(tiff.ResUnit, 10, 16)
	toCm := resolution.ResolutionToCm(uint16(exifUnit))

	s.SetResolution(toCm(tiff.XRes), toCm(tiff.YRes))

	return s
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
