package xmp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/internal/general"
	"github.com/jphastings/dotpostcard/types"
)

func (c codec) Encode(pc types.Postcard, _ *formats.EncodeOptions) ([]formats.FileWriter, error) {
	filename := fmt.Sprintf("%s-meta.xmp", pc.Name)
	writer := func(w io.Writer) error {
		// Don't write pixel & physical size information to an XMP which isn't embedded
		if xmp, err := MetadataToXMP(pc.Meta, nil); err == nil {
			_, writeErr := w.Write(xmp)
			return writeErr
		} else {
			return err
		}
	}
	fw := formats.NewFileWriter(filename, writer)

	return []formats.FileWriter{fw}, nil
}

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
