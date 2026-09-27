package postoffice

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/jphastings/dotpostcard/types"
)

func compileRequest(t *testing.T, fields map[string]string) *http.Request {
	t.Helper()

	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)

	fw, err := mw.CreateFormFile("front", "sample-front.png")
	require.NoError(t, err)
	_, err = fw.Write(testhelpers.RawTestImage("sample-front.png"))
	require.NoError(t, err)

	for k, v := range fields {
		require.NoError(t, mw.WriteField(k, v))
	}
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/compile", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func requestToPostcardWith(t *testing.T, fields map[string]string) types.Postcard {
	t.Helper()

	codecChoices, err := DefaultCodecChoices()
	require.NoError(t, err)

	pc, _, _, err := requestToPostcard(codecChoices, compileRequest(t, fields))
	require.NoError(t, err)
	return pc
}

func TestRequestToPostcardParsesCardColor(t *testing.T) {
	pc := requestToPostcardWith(t, map[string]string{
		"codec-choice":        "web",
		"physical.card-color": "#112233",
	})

	require.NotNil(t, pc.Meta.Physical.CardColor)
	assert.Equal(t, "#112233", pc.Meta.Physical.CardColor.String())
}

func TestRequestToPostcardWithoutCardColorLeavesDefault(t *testing.T) {
	pc := requestToPostcardWith(t, map[string]string{
		"codec-choice": "web",
	})

	assert.Nil(t, pc.Meta.Physical.CardColor)
}

func TestRequestToPostcardRejectsInvalidCardColor(t *testing.T) {
	codecChoices, err := DefaultCodecChoices()
	require.NoError(t, err)

	req := compileRequest(t, map[string]string{
		"codec-choice":        "web",
		"physical.card-color": "not-a-color",
	})

	_, _, _, err = requestToPostcard(codecChoices, req)
	assert.Error(t, err)
}

func TestCompileFromFormEmbedsCardColorInXMP(t *testing.T) {
	codecChoices, err := DefaultCodecChoices()
	require.NoError(t, err)

	req := compileRequest(t, map[string]string{
		"codec-choice":        "web",
		"physical.card-color": "#112233",
	})

	rec := httptest.NewRecorder()
	CompileFromForm(codecChoices)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Postcard:CardColor")
	assert.Contains(t, rec.Body.String(), "#112233")
}
