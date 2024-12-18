package xmpinject

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
)

const (
	vp8xHasICC       byte = 1 << 5
	vp8xHasAlpha     byte = 1 << 4
	vp8xHasEXIF      byte = 1 << 3
	vp8xHasXMP       byte = 1 << 2
	vp8xHasAnimation byte = 1 << 1
)

var chunkOrder = []string{"VP8X", "ICCP", "ANIM", "ALPH", "VP8 ", "VP8L", "EXIF", "XMP "}

// The VP8X header (that declares XMP data is present) also includes the image width and height & context on whether there is Alpha. This *can* be extracted from the VP8 or VP8L data, but providing the data here is faster & easier.
func XMPintoWebP(out io.Writer, webpData []byte, xmpData []byte, bounds image.Rectangle, hasAlpha bool) error {
	chunks, err := parseChunks(webpData)
	if err != nil {
		return err
	}

	chunks["XMP "] = xmpData

	flags := vp8xHasXMP
	if hasAlpha {
		flags |= vp8xHasAlpha
	}
	chunks["VP8X"] = makeWebpChunkVP8X(chunks["VP8X"], flags, bounds)

	// File size for RIFF header
	riffSize := 4
	for _, chunk := range chunks {
		chunkLen := len(chunk)
		riffSize += 8 + chunkLen

		if chunkLen%2 != 0 {
			riffSize++
		}
	}

	// Magic bytes & header
	if _, err := out.Write([]byte("RIFF" + string(chunkSize(riffSize)) + "WEBP")); err != nil {
		return err
	}

	// All known chunks, in their required order
	for _, fourCC := range chunkOrder {
		chunk, ok := chunks[fourCC]
		if !ok {
			continue
		}

		if err := writeWebpChunk(out, fourCC, chunk); err != nil {
			return err
		}

		delete(chunks, fourCC)
	}

	// Any remaining unknown chunks
	for fourCC, chunk := range chunks {
		if err := writeWebpChunk(out, fourCC, chunk); err != nil {
			return err
		}
	}

	return nil
}

func writeWebpChunk(out io.Writer, fourCC string, chunk []byte) error {
	// Header
	if _, err := out.Write(append([]byte(fourCC), chunkSize(len(chunk))...)); err != nil {
		return err
	}
	// Content
	if _, err := out.Write(chunk); err != nil {
		return err
	}

	// Extra padding byte if chunk length isn't even (as all chunks must start on an even byte, and all header data is even length)
	if len(chunk)%2 != 0 {
		if _, err := out.Write([]byte{0x00}); err != nil {
			return err
		}
	}

	return nil
}

func chunkSize(size int) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(size))
	return b
}

func makeWebpChunkVP8X(previousVP8X []byte, flags byte, bounds image.Rectangle) []byte {
	features := byte(0x0)
	if len(previousVP8X) > 1 {
		features = previousVP8X[0]
	}

	features |= flags

	chunk := []byte{features, 0x0, 0x0, 0x0}

	w := make([]byte, 4)
	binary.LittleEndian.PutUint32(w, uint32(bounds.Dx()-1))
	chunk = append(chunk, w[0:3]...)
	h := make([]byte, 4)
	binary.LittleEndian.PutUint32(h, uint32(bounds.Dy()-1))
	chunk = append(chunk, h[0:3]...)

	return chunk
}

func parseChunks(webpData []byte) (map[string][]byte, error) {
	if len(webpData) < 20 || string(webpData[:4]) != "RIFF" || string(webpData[8:12]) != "WEBP" {
		return nil, fmt.Errorf("provided data is not a WebP image")
	}
	riffLen := int(binary.LittleEndian.Uint32(webpData[4:8]))
	if len(webpData) < riffLen {
		return nil, fmt.Errorf("some of the WebP data is missing (RIFF header says %d bytes, but only %d provided)", riffLen, len(webpData))
	}

	chunks := make(map[string][]byte)

	for i := 12; i < riffLen; i += 0 {
		fourCC := string(webpData[i : i+4])
		chunkLen := int(binary.LittleEndian.Uint32(webpData[i+4 : i+8]))
		i += 8

		chunks[fourCC] = webpData[i : i+chunkLen]
		i += chunkLen
		// Skip a null byte if we're on an odd boundary, inserted as per WebP spec
		if i%2 == 1 && webpData[i] == '\x00' {
			i++
		}
	}

	// Discard data beyond the RIFF envelope, as per the spec

	return chunks, nil
}

func XMPfromWebP(webpData []byte) ([]byte, error) {
	chunks, err := parseChunks(webpData)
	if err != nil {
		return nil, err
	}
	return chunks["XMP "], nil
}

func EXIFfromWebP(webpData []byte) ([]byte, error) {
	chunks, err := parseChunks(webpData)
	if err != nil {
		return nil, err
	}
	return chunks["EXIF"], nil
}
