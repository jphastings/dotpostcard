/* This file draws *heavily* from Go's standard library (image/format.go)
 * this is because the webp library golang.org/x/image/webp cannot decode (at all!)
 * WebP images with alpha layers due to a bug The "git.sr.ht/~jackmordaunt/go-libwebp/webp"
 * WebP library *can* decode WebP images with alpha, but it accidentally loads the
 * 'golang.org/x/image/webp' library earlier than when it registers itself with image.Register
 * meaning that the 'golang.org/x/image/webp' decoder takes precedence, preventing correct
 * decode of WebP images with alpha.
 *
 * As image formats can't be skipped or unregistered, this replica of the code in image/format.go
 * ensures that the correct WebP library is used to decode (when available).
 */
package images

import (
	"bufio"
	"fmt"
	"image"
	"io"
)

type decoder func(io.Reader) (image.Image, []byte, error)

type format struct {
	magic string
	decoder
}

var formats = []format{
	{"\xff\xd8", ReadJPEG},
	{"\x89PNG\r\n\x1a\n", ReadPNG},
	{"RIFF????WEBPVP8", ReadWebP},
	{"<svg", ReadSVG},
}

type readPeeker interface {
	io.Reader
	Peek(int) ([]byte, error)
}

func Decode(r io.Reader) (image.Image, []byte, error) {
	p := asReadPeeker(r)

	f, ok := sniff(p)
	if !ok {
		return nil, nil, fmt.Errorf("unsupported image format")
	}

	return f.decoder(p)
}

func asReadPeeker(r io.Reader) readPeeker {
	if rr, ok := r.(readPeeker); ok {
		return rr
	}
	return bufio.NewReader(r)
}

func sniff(r readPeeker) (format, bool) {
	for _, f := range formats {
		b, err := r.Peek(len(f.magic))
		if err == nil && match(f.magic, b) {
			return f, true
		}
	}
	return format{}, false
}

func match(magic string, b []byte) bool {
	if len(magic) != len(b) {
		return false
	}
	for i, c := range b {
		if magic[i] != c && magic[i] != '?' {
			return false
		}
	}
	return true
}
