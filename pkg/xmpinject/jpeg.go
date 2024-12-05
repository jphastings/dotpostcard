package xmpinject

import (
	"fmt"
	"io"
)

var xmpPrefix = []byte("http://ns.adobe.com/xap/1.0/\x00")

func XMPintoJPEG(out io.Writer, jpgData []byte, xmpData []byte) error {
	if len(jpgData) < 2 || jpgData[0] != 0xFF || jpgData[1] != 0xD8 {
		return fmt.Errorf("provided data is not a JPEG image")
	}

	// Magic bytes
	if _, err := out.Write(jpgData[:2]); err != nil {
		return err
	}

	// APP1 header
	length := len(xmpPrefix) + len(xmpData) + 2 // Length requires two bytes to encode
	app1Length := []byte{byte(length >> 8), byte(length & 0xFF)}
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
