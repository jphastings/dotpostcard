package usdz

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
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
		if strings.HasSuffix(name, ".postcard-texture.jpg") || strings.HasSuffix(name, ".postcard-texture.png") {
			return fs.FS(zr).Open(zf.Name)
		}
	}

	return nil, fmt.Errorf("no texture postcard texture files found")
}

func (c codec) Encode(pc types.Postcard, opts *formats.EncodeOptions) ([]formats.FileWriter, error) {
	writeUSDZ := func(w io.Writer) error {
		usdzip, err := exec.LookPath("usdzip")
		if err != nil {
			return fmt.Errorf("unable to find the usdzip executable in PATH: %w", err)
		}

		tempDir, err := os.MkdirTemp("", "postcards-usdz-*")
		if err != nil {
			return fmt.Errorf("unable to create temporary directory to compress USD: %w", err)
		}
		defer os.RemoveAll(tempDir)

		outTmpFilename := "out.usdz"
		args := []string{outTmpFilename}
		fws, err := usd.Codec().Encode(pc, opts)
		if err != nil {
			return err
		}

		for _, fw := range fws {
			fname, err := fw.WriteFile(tempDir, true)
			if err != nil {
				return fmt.Errorf("unable to write USD component files to temporary directory: %w", err)
			}
			args = append(args, fname)
		}

		var stderr bytes.Buffer
		cmd := exec.Command(usdzip, args...)
		cmd.Dir = tempDir
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			errOut := stderr.String()
			if len(errOut) > 0 {
				err = errors.Join(fmt.Errorf("unable to run usdzip: %w", err), fmt.Errorf("usdzip error: %s", errOut))
			}
			return fmt.Errorf("unable to compress USD into USDZ - error calling usdzip: %w", err)
		}

		f, err := os.Open(path.Join(tempDir, outTmpFilename))
		if err != nil {
			return fmt.Errorf("temporary USDZ file couldn't be openned: %w", err)
		}
		defer f.Close()

		if _, err := io.Copy(w, f); err != nil {
			return fmt.Errorf("unable to move USDZ from temporary location: %w", err)
		}

		return nil
	}

	usdzFilename := pc.Name + ".postcard.usdz"
	return []formats.FileWriter{
		formats.NewFileWriter(usdzFilename, writeUSDZ),
	}, nil
}
