package usdz

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/usd"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/types"
)

const codecName = "USDZ 3D model"

func Codec() formats.Codec { return codec{} }

type codec struct{}

func (c codec) Name() string { return codecName }

func (c codec) Bundle(group formats.FileGroup) ([]formats.Bundle, []fs.File, error) {
	var bundles []formats.Bundle
	var remaining []fs.File
	var finalErr error

	for _, file := range group.Files {
		filename, ok := formats.HasFileSuffix(file, ".usdz")
		if !ok {
			remaining = append(remaining, file)
			continue
		}

		tFile, err := usdzToTextureFile(file)
		if err != nil {
			finalErr = errors.Join(finalErr, err)
			continue
		}

		bundles = append(bundles, web.BundleFromReader(tFile, filename))
	}

	return bundles, remaining, finalErr
}

// This is a little hacky, as it assumes there's only one texture, but it works for now
func usdzToTextureFile(file fs.File) (fs.File, error) {
	st, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fz, ok := file.(io.ReaderAt)
	if !ok {
		return nil, fmt.Errorf("unable to seek USD zip file for %s", st.Name())
	}

	zr, err := zip.NewReader(fz, st.Size())
	if err != nil {
		return nil, fmt.Errorf("unable to read USD zip file '%s': %w", st.Name(), err)
	}

	for _, zf := range zr.File {
		name := path.Base(zf.Name)
		if strings.HasSuffix(name, ".postcard-texture.jpeg") || strings.HasSuffix(name, ".postcard-texture.png") {
			return fs.FS(zr).Open(zf.Name)
		}
	}

	return nil, fmt.Errorf("no postcard texture files found")
}

const (
	zipFileHeaderLength = 30
	usdZipHeaderID      = 0x1986
	usdFileAlignment    = 64
)

func makeExtraBytes(offset int) []byte {
	paddingNeeded := usdFileAlignment - offset%usdFileAlignment

	switch paddingNeeded {
	case 64:
		return []byte{}
	case 1, 2, 3:
		paddingNeeded += 64
	}

	extra := binary.LittleEndian.AppendUint16([]byte{}, usdZipHeaderID)
	extra = binary.LittleEndian.AppendUint16(extra, uint16(paddingNeeded)-4)

	return append(extra, make([]byte, paddingNeeded-4)...)
}

func (c codec) Encode(pc types.Postcard, opts *formats.EncodeOptions) ([]formats.FileWriter, error) {
	writeUSDZ := func(w io.Writer) error {
		fws, err := usd.Codec().Encode(pc, opts)
		if err != nil {
			return err
		}

		zw := zip.NewWriter(w)
		defer zw.Close()

		alignmentOffset := 0
		for _, fw := range fws {
			b, err := fw.Bytes()
			if err != nil {
				return err
			}

			// The file header and filename are written before the file
			alignmentOffset += len(fw.Filename) + zipFileHeaderLength

			zipFileHeader := &zip.FileHeader{
				Name:               fw.Filename,
				Method:             zip.Store,
				CRC32:              crc32.ChecksumIEEE(b),
				CompressedSize64:   uint64(len(b)),
				UncompressedSize64: uint64(len(b)),
				ReaderVersion:      0x0A,
				Extra:              makeExtraBytes(alignmentOffset),
			}

			f, err := zw.CreateRaw(zipFileHeader)
			if err != nil {
				return err
			}

			if _, err := f.Write(b); err != nil {
				return err
			}

			// Ensure we know where the next header's position will be
			alignmentOffset += len(zipFileHeader.Extra) + len(b)
		}

		return nil
	}

	usdzFilename := pc.Name + ".postcard.usdz"
	return []formats.FileWriter{
		formats.NewFileWriter(usdzFilename, "model/vnd.usdz+zip", writeUSDZ),
	}, nil
}
