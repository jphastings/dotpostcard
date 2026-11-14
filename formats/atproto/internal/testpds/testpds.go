// Package testpds is a minimal in-memory stand-in for an atproto PDS, exercised over real HTTP
// so both formats/atproto and formats/atproto/pdsbundle can test their client/record-fetching
// code end to end. It stores records as raw JSON rather than a typed atproto.Record, so this
// package doesn't need to import formats/atproto (which would create an import cycle for the
// internal *_test.go files of the atproto package itself).
package testpds

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"sync"
)

type server struct {
	mu      sync.Mutex
	did     string
	blobs   map[string][]byte
	records map[string]map[string]json.RawMessage // collection -> rkey -> record
	nextCID int
}

// New starts a fake PDS for did, holding records of any collection.
func New(did string) *httptest.Server {
	f := &server{did: did, blobs: map[string][]byte{}, records: map[string]map[string]json.RawMessage{}}

	mux := http.NewServeMux()
	mux.HandleFunc("/xrpc/com.atproto.server.createSession", f.createSession)
	mux.HandleFunc("/xrpc/com.atproto.identity.resolveHandle", f.resolveHandle)
	mux.HandleFunc("/xrpc/com.atproto.repo.uploadBlob", f.uploadBlob)
	mux.HandleFunc("/xrpc/com.atproto.repo.putRecord", f.putRecord)
	mux.HandleFunc("/xrpc/com.atproto.repo.getRecord", f.getRecord)
	mux.HandleFunc("/xrpc/com.atproto.repo.listRecords", f.listRecords)
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

func recordCID(record json.RawMessage) string {
	return fmt.Sprintf("bafyrei%x", sha256.Sum256(record))
}

func (f *server) putRecord(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" {
		writeXRPCError(w, http.StatusUnauthorized, "AuthenticationRequired", "missing bearer token")
		return
	}

	var body struct {
		Collection string          `json:"collection"`
		Rkey       string          `json:"rkey"`
		Record     json.RawMessage `json:"record"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeXRPCError(w, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}

	f.mu.Lock()
	if f.records[body.Collection] == nil {
		f.records[body.Collection] = map[string]json.RawMessage{}
	}
	f.records[body.Collection][body.Rkey] = body.Record
	f.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]string{
		"uri": fmt.Sprintf("at://%s/%s/%s", f.did, body.Collection, body.Rkey),
		"cid": recordCID(body.Record),
	})
}

func (f *server) getRecord(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	collection, rkey := q.Get("collection"), q.Get("rkey")

	f.mu.Lock()
	record, ok := f.records[collection][rkey]
	f.mu.Unlock()

	if !ok {
		writeXRPCError(w, http.StatusBadRequest, "RecordNotFound", fmt.Sprintf("no record found for %q", rkey))
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"uri":   fmt.Sprintf("at://%s/%s/%s", f.did, collection, rkey),
		"cid":   recordCID(record),
		"value": record,
	})
}

// listRecords pages through a collection ordered by rkey, with the last rkey as the cursor.
func (f *server) listRecords(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	collection, cursor := q.Get("collection"), q.Get("cursor")
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil || limit < 1 {
		limit = 50
	}

	f.mu.Lock()
	rkeys := make([]string, 0, len(f.records[collection]))
	for rkey := range f.records[collection] {
		if rkey > cursor {
			rkeys = append(rkeys, rkey)
		}
	}
	sort.Strings(rkeys)

	type entry struct {
		URI   string          `json:"uri"`
		CID   string          `json:"cid"`
		Value json.RawMessage `json:"value"`
	}
	page := []entry{}
	for _, rkey := range rkeys[:min(limit, len(rkeys))] {
		record := f.records[collection][rkey]
		page = append(page, entry{fmt.Sprintf("at://%s/%s/%s", f.did, collection, rkey), recordCID(record), record})
	}
	f.mu.Unlock()

	out := map[string]any{"records": page}
	if len(rkeys) > limit {
		out["cursor"] = rkeys[limit-1]
	}
	json.NewEncoder(w).Encode(out)
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
