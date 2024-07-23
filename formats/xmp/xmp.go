package xmp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math/big"

	"github.com/jphastings/postcards/types"
)

func MetadataToXMP(meta types.Metadata) ([]byte, error) {
	dims := types.Size{
		CmWidth:  meta.FrontDimensions.CmWidth,
		CmHeight: meta.FrontDimensions.CmHeight,
		PxWidth:  meta.FrontDimensions.PxWidth,
		PxHeight: meta.FrontDimensions.PxHeight,
	}
	// If this XMP represents an image that has both sides of the postcard, then its pixel
	// and physical height will be twice its front height.
	if meta.Flip != types.FlipNone {
		dims.PxHeight *= 2

		if dims.CmHeight != nil {
			dims.CmHeight = meta.FrontDimensions.CmHeight.Mul(dims.CmHeight, big.NewRat(2, 1))
		}
	}

	var sections []interface{}
	sections = addTIFFSection(sections, dims)
	sections = addIPTCCoreSection(sections, meta)
	sections = addIPTCExtSection(sections, meta)
	sections = addExifSection(sections, meta)
	sections = addDCSection(sections, meta)
	sections = addPostcardSection(sections, meta)

	x := xmpXML{
		NamespaceX:     "adobe:ns:meta/",
		NamespaceXMPTK: "postcards/v0.1",
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
