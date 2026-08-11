package xmp

import (
	"encoding/xml"
	"math"
	"strconv"

	"github.com/jphastings/dotpostcard/types"
)

var (
	privateExplainer = map[string]string{
		"en": "Private information",
	}
	outlineFrontExplainer = map[string]string{
		"en": "Postcard outline (front)",
	}
	outlineBackExplainer = map[string]string{
		"en": "Postcard outline (back)",
	}
)

// Outlines holds the computed outlines for front and back of a postcard.
// These are generated from transparency detection and stored as IPTC regions.
type Outlines struct {
	Front []types.Point
	Back  []types.Point
}

type xmpIptc4xmpExt struct {
	Namespace string       `xml:"xmlns:Iptc4xmpExt,attr"`
	Regions   []iptcRegion `xml:"Iptc4xmpExt:ImageRegion>rdf:Bag>rdf:li,omitempty"`
}

type iptcRegion struct {
	ParseType string             `xml:"rdf:parseType,attr"` // Should always be 'resource'
	Name      langText           `xml:"Iptc4xmpExt:Name>rdf:Alt>rdf:li"`
	Boundary  iptcRegionBoundary `xml:"Iptc4xmpExt:RegionBoundary"`
}

type iptcRegionBoundary struct {
	ParseType string             `xml:"rdf:parseType,attr"`  // Should always be 'resource'
	Unit      string             `xml:"Iptc4xmpExt:rbUnit"`  // Should always be 'relative'
	Shape     string             `xml:"Iptc4xmpExt:rbShape"` // Should always be 'polygon'
	Vertices  []iptcRegionVertex `xml:"Iptc4xmpExt:rbVertices>rdf:Seq>rdf:li"`
}

type iptcRegionVertex struct {
	ParseType string    `xml:"rdf:parseType,attr"` // Should always be 'resource'
	X         iptcCoord `xml:"Iptc4xmpExt:rbX"`
	Y         iptcCoord `xml:"Iptc4xmpExt:rbY"`
}

// iptcCoord is a coordinate fraction (0-1) of image width/height in an IPTC region
// vertex. float64's default XML marshalling writes the full shortest-round-trip
// representation, which for values coming out of a coordinate transform can run to
// 17+ digits of no practical use. Rounding to 6 decimal places is well under a
// hundredth of a pixel even on the largest plausible postcard scan, so nothing
// meaningful is lost.
type iptcCoord float64

// MarshalXML must honour the xml.StartElement it's handed, otherwise the field's
// struct tag name (Iptc4xmpExt:rbX/rbY) is lost from the output.
func (c iptcCoord) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	rounded := math.Round(float64(c)*1e6) / 1e6
	// 'f' avoids exponent notation: the 'g' default would render 0.000001 as
	// "1e-06", which is legal XML but awkward for IPTC consumers to parse.
	s := strconv.FormatFloat(rounded, 'f', -1, 64)

	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.CharData(s)); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

func addIPTCExtSection(sections []interface{}, meta types.Metadata, outlines *Outlines) []interface{} {
	hasSecrets := len(meta.Front.Secrets)+len(meta.Back.Secrets) > 0
	hasMessage := len(meta.Front.Transcription.Text)+len(meta.Back.Transcription.Text) > 0
	hasOutlines := outlines != nil && (len(outlines.Front) > 0 || len(outlines.Back) > 0)

	if !hasSecrets && !hasMessage && !hasOutlines {
		return sections
	}

	prvExp := langText{Lang: meta.Locale}
	if text, ok := privateExplainer[prvExp.Lang]; ok {
		prvExp.Text = text
	} else {
		prvExp.Lang = "en"
		prvExp.Text = privateExplainer["en"]
	}

	var regions []iptcRegion
	regions = append(regions, regionsForSide(prvExp, true, meta.Flip, meta.Front.Secrets)...)
	regions = append(regions, regionsForSide(prvExp, false, meta.Flip, meta.Back.Secrets)...)

	if hasOutlines {
		frontOutExp := langText{Lang: meta.Locale}
		if text, ok := outlineFrontExplainer[frontOutExp.Lang]; ok {
			frontOutExp.Text = text
		} else {
			frontOutExp.Lang = "en"
			frontOutExp.Text = outlineFrontExplainer["en"]
		}

		backOutExp := langText{Lang: meta.Locale}
		if text, ok := outlineBackExplainer[backOutExp.Lang]; ok {
			backOutExp.Text = text
		} else {
			backOutExp.Lang = "en"
			backOutExp.Text = outlineBackExplainer["en"]
		}

		regions = append(regions, outlineRegionsForSide(frontOutExp, true, meta.Flip, outlines.Front)...)
		regions = append(regions, outlineRegionsForSide(backOutExp, false, meta.Flip, outlines.Back)...)
	}

	return append(sections, xmpIptc4xmpExt{
		Namespace: "http://iptc.org/std/Iptc4xmpExt/2008-02-29/",
		Regions:   regions,
	})
}

func regionsForSide(prvExp langText, onFront bool, flip types.Flip, secrets []types.Polygon) []iptcRegion {
	var regions []iptcRegion
	for _, secret := range secrets {
		var vertices []iptcRegionVertex

		for _, point := range secret.Points {
			p := point.TransformToDoubleSided(onFront, flip)
			vertices = append(vertices, iptcRegionVertex{ParseType: "Resource", X: iptcCoord(p.X), Y: iptcCoord(p.Y)})
		}

		regions = append(regions, iptcRegion{
			ParseType: "Resource",
			Name:      prvExp,
			Boundary: iptcRegionBoundary{
				ParseType: "Resource",
				Unit:      "relative",
				Shape:     "polygon",
				Vertices:  vertices,
			},
		})
	}

	return regions
}

func outlineRegionsForSide(outExp langText, onFront bool, flip types.Flip, outline []types.Point) []iptcRegion {
	if len(outline) == 0 {
		return nil
	}

	var vertices []iptcRegionVertex
	for _, point := range outline {
		p := point.TransformToDoubleSided(onFront, flip)
		vertices = append(vertices, iptcRegionVertex{ParseType: "Resource", X: iptcCoord(p.X), Y: iptcCoord(p.Y)})
	}

	return []iptcRegion{{
		ParseType: "Resource",
		Name:      outExp,
		Boundary: iptcRegionBoundary{
			ParseType: "Resource",
			Unit:      "relative",
			Shape:     "polygon",
			Vertices:  vertices,
		},
	}}
}
