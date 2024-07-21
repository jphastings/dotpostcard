package xmp

import (
	"math/big"

	"github.com/jphastings/postcards/types"
)

type xmlTiff struct {
	Namespace      string   `xml:"xmlns:tiff,attr"`
	ImageWidth     int      `xml:"tiff:ImageWidth"`
	ImageHeight    int      `xml:"tiff:ImageLength"`
	ResolutionUnit uint     `xml:"tiff:ResolutionUnit"`
	XResolution    *big.Rat `xml:"tiff:XResolution"`
	YResolution    *big.Rat `xml:"tiff:YResolution"`
}

const tiffUnitsCentimetres = 3

func addTIFFSection(sections []interface{}, dims types.Size) []interface{} {
	return append(sections, xmlTiff{
		Namespace:      "http://ns.adobe.com/tiff/1.0/",
		ImageWidth:     dims.PxWidth,
		ImageHeight:    dims.PxHeight,
		ResolutionUnit: tiffUnitsCentimetres,
		XResolution:    dims.CmWidth,
		YResolution:    dims.CmHeight,
	})
}
