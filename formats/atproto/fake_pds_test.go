package atproto

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

// fakePDS is a minimal in-memory stand-in for a PDS, exercised over real HTTP so client.go's
// request/response handling is tested end to end.
type fakePDS struct {
	mu      sync.Mutex
	did     string
	blobs   map[string][]byte
	records map[string]Record
	nextCID int
}

func newFakePDS(did string) *httptest.Server {
	f := &fakePDS{did: did, blobs: map[string][]byte{}, records: map[string]Record{}}

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

func (f *fakePDS) createSession(w http.ResponseWriter, r *http.Request) {
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

func (f *fakePDS) resolveHandle(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"did": f.did})
}

func (f *fakePDS) uploadBlob(w http.ResponseWriter, r *http.Request) {
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
		"blob": Blob{
			Type:     "blob",
			Ref:      BlobRef{Link: cid},
			MimeType: r.Header.Get("Content-Type"),
			Size:     int64(len(data)),
		},
	})
}

func (f *fakePDS) putRecord(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" {
		writeXRPCError(w, http.StatusUnauthorized, "AuthenticationRequired", "missing bearer token")
		return
	}

	var body struct {
		Rkey   string `json:"rkey"`
		Record Record `json:"record"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeXRPCError(w, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}

	f.mu.Lock()
	f.records[body.Rkey] = body.Record
	f.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]string{
		"uri": fmt.Sprintf("at://%s/%s/%s", f.did, RecordType, body.Rkey),
		"cid": "bafyreifakerecordcidpadding",
	})
}

func (f *fakePDS) getRecord(w http.ResponseWriter, r *http.Request) {
	rkey := r.URL.Query().Get("rkey")

	f.mu.Lock()
	record, ok := f.records[rkey]
	f.mu.Unlock()

	if !ok {
		writeXRPCError(w, http.StatusBadRequest, "RecordNotFound", fmt.Sprintf("no record found for %q", rkey))
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"uri":   fmt.Sprintf("at://%s/%s/%s", f.did, RecordType, rkey),
		"cid":   "bafyreifakerecordcidpadding",
		"value": record,
	})
}

func (f *fakePDS) getBlob(w http.ResponseWriter, r *http.Request) {
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

// putRecordDirect lets tests simulate an out-of-band edit to a stored record (eg. someone
// editing the record without re-uploading the image).
func (f *fakePDS) putRecordDirect(rkey string, record Record) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records[rkey] = record
}
