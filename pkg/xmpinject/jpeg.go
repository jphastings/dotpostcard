package xmpinject

import (
	"encoding/binary"
	"fmt"
	"io"
)

var xmpPrefix = []byte("http://ns.adobe.com/xap/1.0/\x00")

const maxXMPsize = 65535

func XMPintoJPEG(out io.Writer, jpgData []byte, xmpData []byte) error {
	if len(xmpData) > maxXMPsize {
		return fmt.Errorf("the XMP data provided is larger than can (easily) be fit into JPEG images")
	}

	if len(jpgData) < 2 || jpgData[0] != 0xFF || jpgData[1] != 0xD8 {
		return fmt.Errorf("provided data is not a JPEG image")
	}

	// Magic bytes
	if _, err := out.Write(jpgData[:2]); err != nil {
		return err
	}

	// APP1 header
	length := len(xmpPrefix) + len(xmpData) + 2 // Length requires two bytes to encode
	app1Length := make([]byte, 2)
	binary.BigEndian.PutUint16(app1Length, uint16(length))
	if _, err := out.Write(append([]byte{0xFF, 0xE1}, app1Length...)); err != nil {
		return err
	}

	// APP1 content (XMP data)
	if _, err := out.Write(append(xmpPrefix, xmpData...)); err != nil {
		return err
	}

	// Remaining JPEG data
	_, err := out.Write(jpgData[2:])
	return err
}

func XMPfromJPEG(jpgData []byte) ([]byte, error) {
	if len(jpgData) < 6 || jpgData[0] != 0xFF || jpgData[1] != 0xD8 || jpgData[2] != 0xFF || jpgData[3] != 0xE1 {
		return nil, fmt.Errorf("provided data is not a JPEG image with XMP data")
	}

	app1Length := int(binary.BigEndian.Uint16(jpgData[4:6])) - 2 // minus two bytes for the length itself
	endXMP := 6 + app1Length
	if len(jpgData) < endXMP {
		return nil, fmt.Errorf("the JPEG image has been truncated, and is missing some of the XMP data")
	}

	endPrefix := 6 + len(xmpPrefix)
	if string(jpgData[6:endPrefix]) != string(xmpPrefix) {
		return nil, fmt.Errorf("this JPEG isn't a web format postcard (it doesn't have XMP data as its first APP1 chunk)")
	}

	xmpData := jpgData[endPrefix:endXMP]
	return xmpData, nil
}
