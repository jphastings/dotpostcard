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

var iTXTcode = []byte("iTXt")

func xmpToITXT(xmpData []byte) []byte {
	// https://www.libpng.org/pub/png/spec/1.2/PNG-Chunks.html#C.iTXt
	content := []byte("XML:com.adobe.xmp")
	content = append(content, []byte{
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
	return nil, fmt.Errorf("extracting XMP data from PNG files is not yet supported")
}
