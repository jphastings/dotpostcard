package xmp

import (
	"math/big"

	"github.com/jphastings/postcards/types"
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
	data := xmlTiff{
		Namespace:   "http://ns.adobe.com/tiff/1.0/",
		ImageWidth:  dims.PxWidth,
		ImageHeight: dims.PxHeight,
	}
	if dims.CmWidth != nil && dims.CmHeight != nil {
		data.ResolutionUnit = tiffUnitsCentimetres
		data.XResolution = dims.CmWidth
		data.YResolution = dims.CmHeight
	}

	return append(sections, data)
}
