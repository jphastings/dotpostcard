package xmp

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"time"

	"github.com/jphastings/dotpostcard/internal/general"
	"github.com/jphastings/dotpostcard/types"

	"github.com/trimmer-io/go-xmp/xmp"
)

func MetadataToXMP(meta types.Metadata, dims *types.Size) ([]byte, error) {
	var sections []interface{}
	if dims != nil {
		sections = addTIFFSection(sections, *dims)
	}
	sections = addIPTCCoreSection(sections, meta)
	sections = addIPTCExtSection(sections, meta)
	sections = addExifSection(sections, meta)
	sections = addDCSection(sections, meta)
	sections = addPostcardSection(sections, meta)

	x := xmpXML{
		NamespaceX:     "adobe:ns:meta/",
		NamespaceXMPTK: fmt.Sprintf("postcards/v%s", general.Version),
		RDF: rdfXML{
			Namespace: "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
			Sections:  sections,
		},
	}

	d := &bytes.Buffer{}

	// Intro
	if _, err := d.Write([]byte("<?xpacket begin='' id='W5M0MpCehiHzreSzNTczkc9d'?>")); err != nil {
		return nil, fmt.Errorf("unable to write start of XMP XML data: %w", err)
	}

	// XML
	if err := xml.NewEncoder(d).Encode(x); err != nil {
		return nil, fmt.Errorf("unable to write XMP XML data: %w", err)
	}

	// Outro
	if _, err := d.Write([]byte("<?xpacket end='w'?>")); err != nil {
		return nil, fmt.Errorf("unable to write end of XMP XML data: %w", err)
	}

	return d.Bytes(), nil
}

func MetadataFromXMP(r io.Reader) (types.Metadata, types.Size, error) {
	d := xmp.NewDecoder(r)
	doc := &xmp.Document{}
	if err := d.Decode(doc); err != nil {
		return types.Metadata{}, types.Size{}, err
	}

	xPaths, err := doc.ListPaths()
	if err != nil {
		return types.Metadata{}, types.Size{}, err
	}

	var meta types.Metadata
	var size types.Size
	var resolution [3]*big.Rat

	for _, xPath := range xPaths {
		switch xPath.Path {
		case "Postcard:Flip":
			meta.Flip = types.Flip(xPath.Value)

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

		case "exif:GPSAreaInformation":
			meta.Location.Name = xPath.Value
		case "exif:GPSLatitude":
			meta.Location.Latitude = scanDegrees(xPath.Value)
		case "exif:GPSLongitude":
			meta.Location.Longitude = scanDegrees(xPath.Value)

		case "Postcard:Recipient":
			meta.Recipient = scanPerson(xPath.Value)
		case "Postcard:Sender":
			meta.Sender = scanPerson(xPath.Value)
		case "Postcard:Context":
			meta.Context.Description = xPath.Value
		case "Postcard:ContextAuthor":
			meta.Context.Author = scanPerson(xPath.Value)

		case "exif:DateTimeOriginal":
			if date, err := time.Parse(`2006-01-02`, xPath.Value); err == nil {
				meta.SentOn = types.Date{Time: date}
			}

		case "Postcard:DescriptionFront":
			meta.Front.Description = xPath.Value
		case "Postcard:DescriptionBack":
			meta.Back.Description = xPath.Value
		case "Postcard:TranscriptionFront":
			_ = json.Unmarshal([]byte(xPath.Value), &meta.Front.Transcription)
		case "Postcard:TranscriptionBack":
			_ = json.Unmarshal([]byte(xPath.Value), &meta.Back.Transcription)

		default:
			// TODO: secret sections
			// fmt.Printf("%s // %T: %v\n", xPath.Path, xPath.Value, xPath.Value)
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
	var dir string

	if _, err := fmt.Sscanf(str, "%d,%f%s", &deg, &min, &dir); err != nil {
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
