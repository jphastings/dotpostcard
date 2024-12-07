package xmp

import (
	"github.com/jphastings/dotpostcard/types"
)

var (
	privateExplainer = map[string]string{
		"en": "Private information",
	}
)

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
	ParseType string  `xml:"rdf:parseType,attr"` // Should always be 'resource'
	X         float64 `xml:"Iptc4xmpExt:rbX"`
	Y         float64 `xml:"Iptc4xmpExt:rbY"`
}

func addIPTCExtSection(sections []interface{}, meta types.Metadata) []interface{} {
	hasSecrets := len(meta.Front.Secrets)+len(meta.Back.Secrets) > 0
	hasMessage := len(meta.Front.Transcription.Text)+len(meta.Back.Transcription.Text) > 0

	if !hasSecrets && !hasMessage {
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
			vertices = append(vertices, iptcRegionVertex{ParseType: "Resource", X: p.X, Y: p.Y})
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
