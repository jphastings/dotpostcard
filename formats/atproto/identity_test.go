package atproto

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeDIDDocument(w http.ResponseWriter, pds string) {
	json.NewEncoder(w).Encode(map[string]any{
		"service": []map[string]string{
			{"id": "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": pds},
		},
	})
}

// withHTTPClient swaps the package's identity-resolution client for c for the duration of the
// test, restoring it afterwards.
func withHTTPClient(t *testing.T, c *http.Client) {
	t.Helper()
	orig := httpClient
	httpClient = c
	t.Cleanup(func() { httpClient = orig })
}

func TestResolvePDSViaFakePLCDirectory(t *testing.T) {
	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/did:plc:testsubject", r.URL.Path)
		writeDIDDocument(w, "https://pds.example.test")
	}))
	defer plc.Close()

	pds, err := resolvePDS("did:plc:testsubject", plc.URL)
	require.NoError(t, err)
	assert.Equal(t, "https://pds.example.test", pds)
}

func TestResolvePDSViaDIDWebOverTLS(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/did.json", r.URL.Path)
		writeDIDDocument(w, "https://pds.example.test")
	}))
	defer ts.Close()
	withHTTPClient(t, ts.Client())

	host := strings.TrimPrefix(ts.URL, "https://")
	did := "did:web:" + strings.ReplaceAll(host, ":", "%3A")

	pds, err := resolvePDS(did, "")
	require.NoError(t, err)
	assert.Equal(t, "https://pds.example.test", pds)
}

func TestDIDWebDocumentURLRejectsPathComponent(t *testing.T) {
	_, err := didWebDocumentURL("did:web:example.com:user:alice")
	assert.Error(t, err)
}

func TestResolvePDSRejectsUnknownDIDMethod(t *testing.T) {
	_, err := resolvePDS("did:key:z6MkfakeDID", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key")
}

// TestAnonymousResolvesDIDPLCViaFakeDirectory exercises did:plc resolution through the public
// Anonymous entry point: a fake PLC directory hands back a fake PDS's (plain HTTP) address,
// and Anonymous should be able to read from it without any override besides the PLC one.
func TestAnonymousResolvesDIDPLCViaFakeDirectory(t *testing.T) {
	pds := newFakePDS("did:plc:testsubject")
	defer pds.Close()

	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeDIDDocument(w, pds.URL)
	}))
	defer plc.Close()

	client, err := Anonymous("did:plc:testsubject", "", plc.URL)
	require.NoError(t, err)
	assert.Equal(t, "did:plc:testsubject", client.DID())

	exists, err := client.RecordExists("anything")
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestAnonymousResolvesHandleToDIDWebPDS covers handle → did:web → PDS in one hop: the same
// TLS test server answers both the handle's atproto-did well-known file and its own DID
// document, so a single fake server is enough to exercise the whole chain.
func TestAnonymousResolvesHandleToDIDWebPDS(t *testing.T) {
	var host string
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/atproto-did":
			fmt.Fprintf(w, "did:web:%s", strings.ReplaceAll(host, ":", "%3A"))
		case "/.well-known/did.json":
			writeDIDDocument(w, "https://pds.example.test")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()
	withHTTPClient(t, ts.Client())

	host = strings.TrimPrefix(ts.URL, "https://")

	client, err := Anonymous(host, "", "")
	require.NoError(t, err)
	assert.Equal(t, "did:web:"+strings.ReplaceAll(host, ":", "%3A"), client.DID())
}
