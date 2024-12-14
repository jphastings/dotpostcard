package postoffice

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path"
	"strings"
	"time"

	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/component"
	"github.com/jphastings/dotpostcard/types"
)

type CodecChoices map[string][]formats.Codec

func HTTPFormHander(codecChoices CodecChoices) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pc, codecs, encOpts, err := requestToPostcard(codecChoices, r)
		if err != nil {
			http.Error(w, fmt.Sprintf("Unable to create postcard: %v", err), http.StatusBadRequest)
		}

		var files []formats.FileWriter

		for _, c := range codecs {
			fws, err := c.Encode(pc, &encOpts)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			files = append(files, fws...)
		}

		if len(files) < 1 {
			http.Error(w, "No postcard image created", http.StatusInternalServerError)
			return
		}

		if len(files) == 1 {
			w.Header().Add("Content-Type", files[0].Mimetype)
			w.Header().Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, files[0].Filename))
			files[0].WriteTo(w)
			return
		}

		mw := multipart.NewWriter(w)
		w.Header().Add("Content-Type", mw.FormDataContentType())

		for _, f := range files {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, f.Filename, f.Filename))
			h.Set("Content-Type", f.Mimetype)

			ww, err := mw.CreatePart(h)
			if err != nil {
				http.Error(w, "Unable to combine files", http.StatusInternalServerError)
				return
			}

			if err := f.WriteTo(ww); err != nil {
				http.Error(w, "Unable to write files", http.StatusInternalServerError)
				return
			}
		}

		mw.Close()
	}
}

func checkboxBool(formVal string) bool { return formVal == "on" }

func requestToPostcard(codecChoices CodecChoices, r *http.Request) (types.Postcard, []formats.Codec, formats.EncodeOptions, error) {
	codecs, ok := codecChoices[r.FormValue("codec-choice")]
	if !ok {
		return types.Postcard{}, nil, formats.EncodeOptions{}, fmt.Errorf("no acceptable codec choice provided")
	}

	decOpts := formats.DecodeOptions{
		IgnoreTransparency: checkboxBool(r.FormValue("ignore-transparency")),
		RemoveBorder:       checkboxBool(r.FormValue("remove-border")),
	}
	encOpts := formats.EncodeOptions{
		Archival: checkboxBool(r.FormValue("archival")),
	}

	var meta types.Metadata

	err := r.ParseMultipartForm(50 << 20) // 50 MB limit
	if err != nil {
		return types.Postcard{}, nil, encOpts, err
	}

	meta.Name = r.FormValue("name")
	meta.Flip = types.Flip(r.FormValue("flip"))
	meta.Location.SetStrings(
		r.FormValue("location.name"),
		r.FormValue("location.latitude"),
		r.FormValue("location.longitude"),
	)

	if t, err := time.Parse(`2006-01-02`, r.FormValue("sent-on")); err == nil {
		meta.SentOn = types.Date{Time: t}
	}

	meta.Sender.Scan(r.FormValue("sender"))
	meta.Recipient.Scan(r.FormValue("recipient"))

	meta.Front.Description = r.FormValue("front.description")
	meta.Back.Description = r.FormValue("back.description")

	meta.Front.Transcription.Text = r.FormValue("front.transcription.text")
	meta.Back.Transcription.Text = r.FormValue("back.transcription.text")

	meta.Context.Description = r.FormValue("context.description")
	meta.Context.Author.Scan(r.FormValue("context.author"))

	frontR, nameGuess, err := formToFile(r.MultipartForm.File["front"])
	if err != nil {
		return types.Postcard{}, nil, encOpts, err
	}
	backR, _, err := formToFile(r.MultipartForm.File["back"])
	if err != nil {
		return types.Postcard{}, nil, encOpts, err
	}

	if meta.Name == "" {
		meta.Name = nameGuess
	}

	pc, err := component.BundleFromReaders(meta, frontR, backR).Decode(decOpts)
	if err != nil {
		return types.Postcard{}, nil, encOpts, err
	}

	return pc, codecs, encOpts, nil
}

func formToFile(fhs []*multipart.FileHeader) (io.ReadCloser, string, error) {
	if len(fhs) == 0 {
		return nil, "", nil
	}

	file, err := fhs[0].Open()
	if err != nil {
		return nil, "", err
	}

	name := fhs[0].Filename
	suffixesToRemove := []string{
		path.Ext(name),
		"-front",
		"-only",
		"-back",
	}

	for _, suffix := range suffixesToRemove {
		name = strings.TrimSuffix(name, suffix)
	}

	return file, name, nil
}