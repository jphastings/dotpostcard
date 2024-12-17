package xmp

import (
	"fmt"
	"math"

	"github.com/jphastings/dotpostcard/types"
)

type xmpExif struct {
	Namespace        string `xml:"xmlns:exif,attr"`
	OriginalDateTime string `xml:"exif:DateTimeOriginal,omitempty"`
	Placename        string `xml:"exif:GPSAreaInformation,omitempty"`
	Latitude         string `xml:"exif:GPSLatitude,omitempty"`
	Longitude        string `xml:"exif:GPSLongitude,omitempty"`
}

func addExifSection(sections []interface{}, meta types.Metadata) []interface{} {
	hasSentOn := meta.SentOn != nil
	hasLocation := meta.Location != types.Location{}

	if !hasSentOn && !hasLocation {
		return sections
	}

	exif := xmpExif{
		Namespace: "http://ns.adobe.com/exif/1.0/",
		Placename: meta.Location.Name,
		Latitude:  fmtEXIFDegrees(meta.Location.Latitude, true),
		Longitude: fmtEXIFDegrees(meta.Location.Longitude, false),
	}

	if meta.SentOn != nil {
		yy, mm, dd := meta.SentOn.Date()
		exif.OriginalDateTime = fmt.Sprintf("%d-%02d-%02d", yy, mm, dd)
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
