package xmpinject

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

const xmpPrefixStr = "http://ns.adobe.com/xap/1.0/\x00"

var xmpPrefix = []byte(xmpPrefixStr)

// maxSingleSegmentXMPsize is the most XMP data that fits in one APP1
// segment: the segment's 16-bit length field counts itself as well as the
// prefix and the payload, so both it and the prefix come off the 0xFFFF cap.
const maxSingleSegmentXMPsize = 0xFFFF - 2 - len(xmpPrefixStr)

const extendedXMPPrefixStr = "http://ns.adobe.com/xmp/extension/\x00"

var extendedXMPPrefix = []byte(extendedXMPPrefixStr)

// guidSize is the length of the uppercase-hex MD5 digest Adobe's spec uses
// to correlate a standard XMP packet's HasExtendedXMP marker with the
// ExtendedXMP APP1 segments that carry the real payload.
const guidSize = md5.Size * 2

// maxExtendedChunkSize is the most payload bytes one ExtendedXMP APP1
// segment can carry: the 16-bit segment length minus the extension
// signature, the GUID, and the 4-byte total-length/offset pair.
const maxExtendedChunkSize = 0xFFFF - 2 - len(extendedXMPPrefixStr) - guidSize - 4 - 4

// maxExtendedSegments caps how many ExtendedXMP segments XMPintoJPEG will
// emit, so a pathological input fails fast with a clear error instead of
// silently producing a multi-megabyte JPEG.
const maxExtendedSegments = 256

const maxXMPsize = maxExtendedChunkSize * maxExtendedSegments

// XMPintoJPEG writes jpgData back out to out, with xmpData injected as XMP
// metadata immediately after the SOI marker.
//
// When xmpData fits in a single APP1 segment it is written as a standard
// Adobe XMP APP1 chunk, byte-for-byte as before. Larger payloads use
// Adobe's ExtendedXMP mechanism (XMP Specification Part 3): a minimal
// standard XMP APP1 carrying an xmpNote:HasExtendedXMP marker, followed by
// one or more ExtendedXMP APP1 segments carrying the payload in order.
func XMPintoJPEG(out io.Writer, jpgData []byte, xmpData []byte) error {
	if len(xmpData) > maxXMPsize {
		return fmt.Errorf("the XMP data provided (%d bytes) is larger than can be fit into a JPEG image (max %d bytes)", len(xmpData), maxXMPsize)
	}

	if len(jpgData) < 2 || jpgData[0] != 0xFF || jpgData[1] != 0xD8 {
		return fmt.Errorf("provided data is not a JPEG image")
	}

	// Magic bytes
	if _, err := out.Write(jpgData[:2]); err != nil {
		return err
	}

	if len(xmpData) <= maxSingleSegmentXMPsize {
		if err := writeAPP1(out, xmpPrefix, xmpData); err != nil {
			return err
		}
	} else if err := writeExtendedXMP(out, xmpData); err != nil {
		return err
	}

	// Remaining JPEG data
	_, err := out.Write(jpgData[2:])
	return err
}

// writeAPP1 writes a single APP1 segment (marker, big-endian length, then
// content) whose content is prefix followed by payload.
func writeAPP1(out io.Writer, prefix, payload []byte) error {
	length := len(prefix) + len(payload) + 2 // +2 for the length field itself
	header := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(header[2:], uint16(length))

	if _, err := out.Write(header); err != nil {
		return err
	}
	if len(prefix) > 0 {
		if _, err := out.Write(prefix); err != nil {
			return err
		}
	}
	_, err := out.Write(payload)
	return err
}

// writeExtendedXMP writes the standard XMP APP1 (carrying only the
// HasExtendedXMP marker) followed by the ExtendedXMP APP1 segments that
// together carry the whole of xmpData, in ascending offset order.
func writeExtendedXMP(out io.Writer, xmpData []byte) error {
	guidBytes := md5.Sum(xmpData)
	guid := strings.ToUpper(hex.EncodeToString(guidBytes[:]))

	mainPacket := fmt.Sprintf(
		`<?xpacket begin='' id='W5M0MpCehiHzreSzNTczkc9d'?><x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="" xmlns:xmpNote="http://ns.adobe.com/xmp/note/" xmpNote:HasExtendedXMP="%s"/></rdf:RDF></x:xmpmeta><?xpacket end='w'?>`,
		guid,
	)
	if err := writeAPP1(out, xmpPrefix, []byte(mainPacket)); err != nil {
		return err
	}

	total := uint32(len(xmpData))
	for offset := 0; offset < len(xmpData); offset += maxExtendedChunkSize {
		end := min(offset+maxExtendedChunkSize, len(xmpData))

		chunk := make([]byte, 0, guidSize+4+4+(end-offset))
		chunk = append(chunk, guid...)
		chunk = binary.BigEndian.AppendUint32(chunk, total)
		chunk = binary.BigEndian.AppendUint32(chunk, uint32(offset))
		chunk = append(chunk, xmpData[offset:end]...)

		if err := writeAPP1(out, extendedXMPPrefix, chunk); err != nil {
			return err
		}
	}

	return nil
}

// hasExtendedXMPRe extracts the xmpNote:HasExtendedXMP GUID from the
// standard XMP packet without pulling in a full XML parser: the value is
// always a bare 32-char hex GUID, so a targeted attribute scan is enough.
var hasExtendedXMPRe = regexp.MustCompile(`xmpNote:HasExtendedXMP=(?:"([0-9A-Fa-f]{32})"|'([0-9A-Fa-f]{32})')`)

func extendedXMPGUID(standardPacket []byte) string {
	m := hasExtendedXMPRe.FindSubmatch(standardPacket)
	if m == nil {
		return ""
	}
	if len(m[1]) > 0 {
		return string(m[1])
	}
	return string(m[2])
}

type extendedChunk struct {
	guid   string
	total  uint32
	offset uint32
	data   []byte
}

// XMPfromJPEG extracts the XMP payload embedded in a JPEG file's APP1
// segments, reassembling an Adobe ExtendedXMP payload that spans multiple
// segments if the standard packet declares one.
func XMPfromJPEG(jpgData []byte) ([]byte, error) {
	const noXMPErr = "this JPEG isn't a web format postcard (it doesn't have XMP data as its first APP1 chunk)"

	if len(jpgData) < 2 || jpgData[0] != 0xFF || jpgData[1] != 0xD8 {
		return nil, errors.New(noXMPErr)
	}

	var standardPacket []byte
	var chunks []extendedChunk

	pos := 2
	for pos < len(jpgData) {
		if jpgData[pos] != 0xFF {
			return nil, fmt.Errorf("malformed JPEG: expected a marker at offset %d", pos)
		}
		pos++

		// Any number of 0xFF fill bytes may precede the actual marker code.
		for pos < len(jpgData) && jpgData[pos] == 0xFF {
			pos++
		}
		if pos >= len(jpgData) {
			return nil, fmt.Errorf("the JPEG image has been truncated")
		}

		marker := jpgData[pos]
		pos++

		// Standalone markers (TEM, RSTn, EOI) carry no length/payload.
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD9) {
			if marker == 0xD9 { // EOI: nothing more to scan
				break
			}
			continue
		}

		// SOS marks the start of entropy-coded scan data, which must not
		// be parsed as further markers/segments.
		if marker == 0xDA {
			break
		}

		if pos+2 > len(jpgData) {
			return nil, fmt.Errorf("the JPEG image has been truncated, and is missing a segment length")
		}
		segLength := int(binary.BigEndian.Uint16(jpgData[pos : pos+2]))
		if segLength < 2 || pos+segLength > len(jpgData) {
			return nil, fmt.Errorf("the JPEG image has been truncated, and is missing some of the XMP data")
		}
		content := jpgData[pos+2 : pos+segLength]
		pos += segLength

		if marker != 0xE1 { // Not an APP1 segment
			continue
		}

		switch {
		case standardPacket == nil && bytes.HasPrefix(content, xmpPrefix):
			standardPacket = content[len(xmpPrefix):]
		case bytes.HasPrefix(content, extendedXMPPrefix):
			rest := content[len(extendedXMPPrefix):]
			if len(rest) < guidSize+8 {
				return nil, fmt.Errorf("malformed ExtendedXMP segment: too short to contain its header")
			}
			chunks = append(chunks, extendedChunk{
				guid:   string(rest[:guidSize]),
				total:  binary.BigEndian.Uint32(rest[guidSize : guidSize+4]),
				offset: binary.BigEndian.Uint32(rest[guidSize+4 : guidSize+8]),
				data:   rest[guidSize+8:],
			})
		}
	}

	if standardPacket == nil {
		return nil, errors.New(noXMPErr)
	}

	guid := extendedXMPGUID(standardPacket)
	if guid == "" {
		return standardPacket, nil
	}

	return assembleExtendedXMP(guid, chunks)
}

// assembleExtendedXMP reassembles the ExtendedXMP chunks matching guid into
// the single payload they were split from, validating that they agree on a
// total length, tile it without gaps or overlaps, and hash to the GUID.
func assembleExtendedXMP(guid string, chunks []extendedChunk) ([]byte, error) {
	var matching []extendedChunk
	for _, c := range chunks {
		if strings.EqualFold(c.guid, guid) {
			matching = append(matching, c)
		}
	}
	if len(matching) == 0 {
		return nil, fmt.Errorf("this JPEG declares ExtendedXMP GUID %s but has no matching ExtendedXMP segments", guid)
	}

	sort.Slice(matching, func(i, j int) bool { return matching[i].offset < matching[j].offset })

	total := matching[0].total
	assembled := make([]byte, 0, total)
	for _, c := range matching {
		if c.total != total {
			return nil, fmt.Errorf("malformed ExtendedXMP: segments disagree on the total payload length")
		}
		if uint32(len(assembled)) != c.offset {
			return nil, fmt.Errorf("malformed ExtendedXMP: chunk at offset %d does not tile contiguously (expected offset %d)", c.offset, len(assembled))
		}
		assembled = append(assembled, c.data...)
	}

	if uint32(len(assembled)) != total {
		return nil, fmt.Errorf("malformed ExtendedXMP: assembled payload is %d bytes, expected %d", len(assembled), total)
	}

	sum := md5.Sum(assembled)
	gotGUID := hex.EncodeToString(sum[:])
	if !strings.EqualFold(gotGUID, guid) {
		return nil, fmt.Errorf("malformed ExtendedXMP: assembled payload's MD5 does not match its declared GUID")
	}

	return assembled, nil
}
