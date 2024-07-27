package xmp

import (
	"math/big"

	"github.com/jphastings/dotpostcard/types"
)

type xmlTiff struct {
	Namespace      string   `xml:"xmlns:tiff,attr"`
	ImageWidth     int      `xml:"tiff:ImageWidth,omitempty"`
	ImageHeight    int      `xml:"tiff:ImageLength,omitempty"`
	ResolutionUnit uint     `xml:"tiff:ResolutionUnit,omitempty"`
	XResolution    *big.Rat `xml:"tiff:XResolution,omitempty"`
	YResolution    *big.Rat `xml:"tiff:YResolution,omitempty"`
}

const tiffUnitsCentimetres = 3

func addTIFFSection(sections []interface{}, dims types.Size) []interface{} {
	// TODO: I need to extend the Px and Cm height to handle double size web format postcards

	data := xmlTiff{
		Namespace:   "http://ns.adobe.com/tiff/1.0/",
		ImageWidth:  dims.PxWidth,
		ImageHeight: dims.PxHeight,
	}
	if dims.HasPhysical() {
		data.ResolutionUnit = tiffUnitsCentimetres
		xRes, yRes := dims.Resolution()
		data.XResolution = xRes
		data.YResolution = yRes
	}

	return append(sections, data)
}
