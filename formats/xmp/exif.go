package xmp

import (
	"fmt"
	"math"

	"github.com/jphastings/dotpostcard/types"
)

type xmpExif struct {
	Namespace        string `xml:"xmlns:exif,attr"`
	OriginalDateTime string `xml:"exif:DateTimeOriginal"`
	Placename        string `xml:"exif:GPSAreaInformation"`
	Latitude         string `xml:"exif:GPSLatitude"`
	Longitude        string `xml:"exif:GPSLongitude"`
}

func addExifSection(sections []interface{}, meta types.Metadata) []interface{} {
	hasSentOn := meta.SentOn != ""
	hasLocation := meta.Location != types.Location{}

	if !hasSentOn && !hasLocation {
		return sections
	}

	exif := xmpExif{
		Namespace:        "http://ns.adobe.com/exif/1.0/",
		OriginalDateTime: string(meta.SentOn),
		Placename:        meta.Location.Name,
		Latitude:         fmtEXIFDegrees(meta.Location.Latitude, true),
		Longitude:        fmtEXIFDegrees(meta.Location.Longitude, false),
	}

	return append(sections, exif)
}

func fmtEXIFDegrees(ang *float64, isLat bool) string {
	if ang == nil {
		return ""
	}

	var dir string
	if isLat {
		if *ang >= 0 {
			dir = "N"
		} else {
			dir = "S"
		}
	} else {
		if *ang >= 0 {
			dir = "E"
		} else {
			dir = "W"
		}
	}

	degs := math.Floor(math.Abs(*ang))
	mins := (math.Abs(*ang) - degs) * 60

	return fmt.Sprintf("%.0f,%.8f%s", degs, mins, dir)
}
