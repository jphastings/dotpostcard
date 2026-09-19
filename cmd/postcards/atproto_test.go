package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestPDS is a minimal fake PDS, just enough to exercise the CLI's login/upload path:
// createSession, uploadBlob, putRecord, and getRecord (for the pre-upload existence check).
// The returned map is filled in (keyed by rkey) as records are put, so tests can assert on it
// directly rather than scraping stdout.
func newTestPDS(t *testing.T, did string) (*httptest.Server, map[string]json.RawMessage) {
	t.Helper()
	records := map[string]json.RawMessage{}

	mux := http.NewServeMux()
	mux.HandleFunc("/xrpc/com.atproto.server.createSession", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"did": did, "accessJwt": "test-jwt"})
	})
	mux.HandleFunc("/xrpc/com.atproto.repo.uploadBlob", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"blob": atproto.Blob{Type: "blob", Ref: atproto.BlobRef{Link: "bafytestcid"}, MimeType: r.Header.Get("Content-Type"), Size: 1},
		})
	})
	mux.HandleFunc("/xrpc/com.atproto.repo.putRecord", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Rkey   string          `json:"rkey"`
			Record json.RawMessage `json:"record"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		records[body.Rkey] = body.Record
		json.NewEncoder(w).Encode(map[string]string{"uri": "at://" + did + "/org.dotpostcard.postcard/" + body.Rkey})
	})
	mux.HandleFunc("/xrpc/com.atproto.repo.getRecord", func(w http.ResponseWriter, r *http.Request) {
		record, ok := records[r.URL.Query().Get("rkey")]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "RecordNotFound", "message": "no record"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"uri": "at://" + did, "cid": "bafyrecord", "value": record})
	})

	return httptest.NewServer(mux), records
}

func TestRunEATProtoFailsFastWithoutCredentials(t *testing.T) {
	for _, name := range []string{"GOAT_USERNAME", "GOAT_PASSWORD", "ATP_USERNAME", "ATP_PASSWORD", "ATP_AUTH_USERNAME", "ATP_AUTH_PASSWORD"} {
		t.Setenv(name, "")
	}

	dir := t.TempDir()
	frontPath := writeComponentFixture(t, dir, "sample", testhelpers.RawTestImage("sample-front.png"))

	resetRootCmd()
	rootCmd.SetArgs([]string{"-f", "atproto", "--out-dir", dir, frontPath})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials")
}

// TestFormatCountIncludesATProto covers the "Converting… into N different formats" line's
// count directly, rather than through the CLI: capturing that line means capturing stdout
// around a real (goroutine, network-calling) encode, which proved flaky elsewhere in this file
// (see TestRunEATProtoUploadsCard) — this is the same logic without that risk.
func TestFormatCountIncludesATProto(t *testing.T) {
	assert.Equal(t, 1, formatCount(0, true), "-f atproto alone")
	assert.Equal(t, 2, formatCount(1, true), "-f web,atproto")
	assert.Equal(t, 1, formatCount(1, false), "-f web alone")
}

func TestRunEATProtoUploadsCard(t *testing.T) {
	dir := t.TempDir()
	frontPath := writeComponentFixture(t, dir, "sample", testhelpers.RawTestImage("sample-front.png"))

	srv, records := newTestPDS(t, "did:plc:testuser")
	defer srv.Close()
	t.Setenv("ATP_PDS_HOST", srv.URL)

	resetRootCmd()
	rootCmd.SetArgs([]string{"-f", "atproto", "--out-dir", dir, "--at-user", "alice.example", "--at-password", "app-pass", frontPath})

	// Deliberately not using captureStdout here: it swaps os.Stdout for a pipe for the
	// duration of the run, and doing that around a real (background-goroutine) image encode
	// plus live HTTP round trips proved flaky against other tests in this file that also
	// redirect os.Stdout. Asserting on the record the fake PDS actually received is just as
	// strong a check, and avoids the interaction entirely.
	require.NoError(t, rootCmd.Execute())

	// The rkey is now a TID derived from the encoded image, not the card's name, so there's
	// no fixed key to look up — but exactly one record should have landed.
	require.Len(t, records, 1, "the fake PDS should have received exactly one record")

	var record atproto.Record
	for _, raw := range records {
		require.NoError(t, json.Unmarshal(raw, &record))
	}
	assert.Equal(t, atproto.RecordType, record.Type)
	assert.NotEmpty(t, record.Image.Ref.Link)
}

// TestRunEATProtoReuploadHitsSameTID doubles as a determinism check: if encoding the same
// card twice produced different image bytes, the second run would compute a different TID and
// upload a duplicate record instead of hitting the existing one.
func TestRunEATProtoReuploadHitsSameTID(t *testing.T) {
	dir := t.TempDir()
	frontPath := writeComponentFixture(t, dir, "sample", testhelpers.RawTestImage("sample-front.png"))

	srv, records := newTestPDS(t, "did:plc:testuser")
	defer srv.Close()
	t.Setenv("ATP_PDS_HOST", srv.URL)

	args := []string{"-f", "atproto", "--out-dir", dir, "--at-user", "alice.example", "--at-password", "app-pass", frontPath}

	resetRootCmd()
	rootCmd.SetArgs(args)
	require.NoError(t, rootCmd.Execute())
	require.Len(t, records, 1)

	resetRootCmd()
	rootCmd.SetArgs(append(append([]string{}, args...), "--skip-existing"))
	require.NoError(t, rootCmd.Execute())
	assert.Len(t, records, 1, "re-uploading the unchanged card should hit the same TID and be skipped, not duplicated")

	resetRootCmd()
	rootCmd.SetArgs(args)
	assert.Error(t, rootCmd.Execute(), "re-uploading without --overwrite/--skip-existing should fail the card")
}
