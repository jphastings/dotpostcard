package xmp

import "encoding/xml"

type xmpXML struct {
	XMLName        xml.Name `xml:"x:xmpmeta"`
	NamespaceX     string   `xml:"xmlns:x,attr"`
	NamespaceXMPTK string   `xml:"xmlns:xmptk,attr"`
	RDF            rdfXML   `xml:"rdf:RDF"`
}

type rdfXML struct {
	Namespace string        `xml:"xmlns:rdf,attr"`
	Sections  []interface{} `xml:"rdf:Description"`
}

type langText struct {
	Text string `xml:",chardata"`
	Lang string `xml:"xml:lang,attr"`
}
