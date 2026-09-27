// Package testpds is a minimal in-memory stand-in for an atproto PDS, exercised over real HTTP
// so both formats/atproto and formats/atproto/pdsbundle can test their client/record-fetching
// code end to end. It stores records as raw JSON rather than a typed atproto.Record, so this
// package doesn't need to import formats/atproto (which would create an import cycle for the
// internal *_test.go files of the atproto package itself).
package testpds

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

type server struct {
	mu         sync.Mutex
	did        string
	collection string
	blobs      map[string][]byte
	records    map[string]json.RawMessage
	nextCID    int
}

// New starts a fake PDS for did, whose records are reported under the given lexicon collection.
func New(did, collection string) *httptest.Server {
	f := &server{did: did, collection: collection, blobs: map[string][]byte{}, records: map[string]json.RawMessage{}}

	mux := http.NewServeMux()
	mux.HandleFunc("/xrpc/com.atproto.server.createSession", f.createSession)
	mux.HandleFunc("/xrpc/com.atproto.identity.resolveHandle", f.resolveHandle)
	mux.HandleFunc("/xrpc/com.atproto.repo.uploadBlob", f.uploadBlob)
	mux.HandleFunc("/xrpc/com.atproto.repo.putRecord", f.putRecord)
	mux.HandleFunc("/xrpc/com.atproto.repo.getRecord", f.getRecord)
	mux.HandleFunc("/xrpc/com.atproto.sync.getBlob", f.getBlob)
	return httptest.NewServer(mux)
}

func writeXRPCError(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": errCode, "message": message})
}

func (f *server) createSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeXRPCError(w, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}
	if body.Identifier == "" || body.Password == "" {
		writeXRPCError(w, http.StatusUnauthorized, "AuthenticationRequired", "missing credentials")
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"did": f.did, "accessJwt": "fake-access-jwt"})
}

func (f *server) resolveHandle(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"did": f.did})
}

func (f *server) uploadBlob(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" {
		writeXRPCError(w, http.StatusUnauthorized, "AuthenticationRequired", "missing bearer token")
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeXRPCError(w, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}

	f.mu.Lock()
	f.nextCID++
	cid := fmt.Sprintf("bafyfakecid%d", f.nextCID)
	f.blobs[cid] = data
	f.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]any{
		"blob": map[string]any{
			"$type":    "blob",
			"ref":      map[string]string{"$link": cid},
			"mimeType": r.Header.Get("Content-Type"),
			"size":     len(data),
		},
	})
}

func (f *server) putRecord(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" {
		writeXRPCError(w, http.StatusUnauthorized, "AuthenticationRequired", "missing bearer token")
		return
	}

	var body struct {
		Rkey   string          `json:"rkey"`
		Record json.RawMessage `json:"record"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeXRPCError(w, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}

	f.mu.Lock()
	f.records[body.Rkey] = body.Record
	f.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]string{
		"uri": fmt.Sprintf("at://%s/%s/%s", f.did, f.collection, body.Rkey),
		"cid": "bafyreifakerecordcidpadding",
	})
}

func (f *server) getRecord(w http.ResponseWriter, r *http.Request) {
	rkey := r.URL.Query().Get("rkey")

	f.mu.Lock()
	record, ok := f.records[rkey]
	f.mu.Unlock()

	if !ok {
		writeXRPCError(w, http.StatusBadRequest, "RecordNotFound", fmt.Sprintf("no record found for %q", rkey))
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"uri":   fmt.Sprintf("at://%s/%s/%s", f.did, f.collection, rkey),
		"cid":   "bafyreifakerecordcidpadding",
		"value": record,
	})
}

func (f *server) getBlob(w http.ResponseWriter, r *http.Request) {
	cid := r.URL.Query().Get("cid")

	f.mu.Lock()
	data, ok := f.blobs[cid]
	f.mu.Unlock()

	if !ok {
		writeXRPCError(w, http.StatusBadRequest, "BlobNotFound", fmt.Sprintf("no blob found for %q", cid))
		return
	}

	w.Write(data)
}
