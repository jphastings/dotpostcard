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

func xmpToITXT(xmpData []byte) []byte {
	crc := crc32.Checksum(xmpData, crc32.IEEETable)

	content := []byte("XML:com.adobe.xmp")
	content = append(content, []byte{
		0x00, // separator
		0x00, // uncompressed
		0x00, // no compression method
		0x00, // separator
	}...)
	content = append(content, xmpData...)

	// Compute the length and CRC
	chunk := bytes.NewBuffer(nil)
	binary.Write(chunk, binary.BigEndian, uint32(len(content)))
	chunk.WriteString("iTXt")
	chunk.Write(content)
	binary.Write(chunk, binary.BigEndian, crc)

	return chunk.Bytes()
}
