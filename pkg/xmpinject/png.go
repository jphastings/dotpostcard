package xmpinject

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

const (
	pngMagic = "\x89\x50\x4E\x47\x0D\x0A\x1A\x0A"
	lenMagic = len(pngMagic)
)

func XMPintoPNG(out io.Writer, pngData []byte, xmpData []byte) error {
	if len(pngData) < len(pngMagic) || string(pngData[0:lenMagic]) != pngMagic {
		return fmt.Errorf("provided data is not a PNG image")
	}

	// Magic byte & iHDR chunk (which must always be first)
	ihdrLen := binary.BigEndian.Uint32(pngData[lenMagic : lenMagic+4])
	skipTo := lenMagic + 12 + int(ihdrLen) // 12 = 4 for length, 4 for type, 4 for checksum
	if _, err := out.Write(pngData[:skipTo]); err != nil {
		return err
	}

	// iTXt chunk
	if _, err := out.Write(xmpToITXT(xmpData)); err != nil {
		return err
	}

	// Remaining PNG data
	_, err := out.Write(pngData[skipTo:])
	return err
}

var (
	iTXTcode = []byte("iTXt")
	iTXTkey  = []byte("XML:com.adobe.xmp")
)

func xmpToITXT(xmpData []byte) []byte {
	// XMP data inside PNG data can't be edited in place, because of the CRC values
	xmpData = bytes.Replace(xmpData, []byte("<?xpacket end='w'?>"), []byte("<?xpacket end='r'?>"), 1)

	// http://www.libpng.org/pub/png/spec/1.2/PNG-Chunks.html#C.iTXt
	content := append(iTXTkey, []byte{
		0x00, // separator
		0x00, // uncompressed
		0x00, // no compression method
		0x00, // separator
		0x00, // separator
	}...)
	content = append(content, xmpData...)

	// Compute the length and CRC
	chunk := bytes.NewBuffer(nil)
	binary.Write(chunk, binary.BigEndian, uint32(len(content)))
	chunk.Write(iTXTcode)
	chunk.Write(content)
	crc := crc32.Checksum(append(iTXTcode, content...), crc32.IEEETable)
	binary.Write(chunk, binary.BigEndian, crc)

	return chunk.Bytes()
}

func XMPfromPNG(pngData []byte) ([]byte, error) {
	if len(pngData) < lenMagic || string(pngData[:lenMagic]) != string(pngMagic) {
		return nil, fmt.Errorf("provided data is not a PNG image")
	}
	pos := lenMagic
	for pos <= len(pngData) {
		length := binary.BigEndian.Uint32(pngData[pos : pos+4])
		code := pngData[pos+4 : pos+8]

		start := pos + 8
		end := start + int(length)
		if string(code) == string(iTXTcode) {
			xmp, ok := xmpFromITXdata(pngData[start:end])
			if ok {
				return xmp, nil
			}
		}

		pos = end + 4
	}
	return nil, fmt.Errorf("no XMP iTXt chunk present")
}

func xmpFromITXdata(itxt []byte) ([]byte, bool) {
	pos := len(iTXTkey)
	if string(itxt[:pos]) != string(iTXTkey) {
		return nil, false
	}

	if string(itxt[pos:pos+3]) != "\x00\x00\x00" {
		// We can onnly extract uncompressed XMP data
		return nil, false
	}
	pos += 3

	// Ignore Locale & translated strings
	nullBytesLeft := 2
	for ; pos < len(itxt); pos++ {
		if itxt[pos] == '\x00' {
			nullBytesLeft--
			if nullBytesLeft == 0 {
				break
			}
		}
	}
	if nullBytesLeft != 0 {
		return nil, false
	}

	// +1 for separator
	return itxt[pos+1:], true
}
