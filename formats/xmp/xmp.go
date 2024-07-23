package xmp

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/jphastings/postcards/types"
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
