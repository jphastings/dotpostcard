package atproto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrRecordNotFound is returned (wrapped) by GetRecord/RecordExists when the PDS reports the
// record doesn't exist.
var ErrRecordNotFound = errors.New("atproto: record not found")

const requestTimeout = 30 * time.Second

// httpClient is used for identity/PDS resolution (handle→DID, DID→PDS document) — a package
// var rather than one constructed per call, so tests can swap in an httptest TLS server's
// client to resolve against a fake plc.directory/did:web host hermetically.
var httpClient = &http.Client{Timeout: requestTimeout}

// Client talks to a single PDS, optionally as a logged-in session.
type Client struct {
	httpClient *http.Client
	pdsHost    string
	did        string
	accessJwt  string
}

func (c *Client) DID() string { return c.did }

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: requestTimeout}
}

// Login resolves user (a handle or DID) to its PDS, unless pdsHostOverride is given, then
// authenticates against it, keeping the session's DID and access token. plcHostOverride is
// used in place of the default PLC directory when user resolves via a did:plc.
func Login(user, password, pdsHostOverride, plcHostOverride string) (*Client, error) {
	pdsHost := strings.TrimRight(pdsHostOverride, "/")
	if pdsHost == "" {
		did, err := resolveToDID(user)
		if err != nil {
			return nil, err
		}
		pdsHost, err = resolvePDS(did, plcHostOverride)
		if err != nil {
			return nil, err
		}
	}

	c := &Client{httpClient: newHTTPClient(), pdsHost: pdsHost}

	var session struct {
		Did       string `json:"did"`
		AccessJwt string `json:"accessJwt"`
	}
	if err := c.post("com.atproto.server.createSession", map[string]string{
		"identifier": user,
		"password":   password,
	}, false, &session); err != nil {
		return nil, fmt.Errorf("logging in as %q: %w", user, err)
	}

	c.did = session.Did
	c.accessJwt = session.AccessJwt
	return c, nil
}

// Anonymous resolves authority (a handle or DID) to its DID and PDS, for read-only access.
// pdsHostOverride skips PDS lookup; if authority is a handle in that case, it's still resolved
// to a DID via the override host's own resolveHandle XRPC (so tests can stay hermetic).
// plcHostOverride is used in place of the default PLC directory when authority resolves via a
// did:plc.
func Anonymous(authority, pdsHostOverride, plcHostOverride string) (*Client, error) {
	pdsHost := strings.TrimRight(pdsHostOverride, "/")

	if pdsHost == "" {
		did, err := resolveToDID(authority)
		if err != nil {
			return nil, err
		}
		pds, err := resolvePDS(did, plcHostOverride)
		if err != nil {
			return nil, err
		}
		return &Client{httpClient: newHTTPClient(), pdsHost: pds, did: did}, nil
	}

	if strings.HasPrefix(authority, "did:") {
		return &Client{httpClient: newHTTPClient(), pdsHost: pdsHost, did: authority}, nil
	}

	c := &Client{httpClient: newHTTPClient(), pdsHost: pdsHost}
	var out struct {
		Did string `json:"did"`
	}
	if err := c.get("com.atproto.identity.resolveHandle", url.Values{"handle": {authority}}, false, &out); err != nil {
		return nil, fmt.Errorf("resolving handle %q via %s: %w", authority, pdsHost, err)
	}
	c.did = out.Did
	return c, nil
}

// resolveToDID returns authority unchanged if it's already a DID, otherwise resolves it as a
// handle.
func resolveToDID(authority string) (string, error) {
	if strings.HasPrefix(authority, "did:") {
		return authority, nil
	}
	return resolveHandleToDID(authority)
}

// resolveHandleToDID tries the HTTPS well-known file first, falling back to a DNS TXT record.
func resolveHandleToDID(handle string) (string, error) {
	if resp, err := httpClient.Get(fmt.Sprintf("https://%s/.well-known/atproto-did", handle)); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
			if err == nil {
				if did := strings.TrimSpace(string(body)); strings.HasPrefix(did, "did:") {
					return did, nil
				}
			}
		}
	}

	recs, err := net.LookupTXT("_atproto." + handle)
	if err != nil {
		return "", fmt.Errorf("resolving handle %q: no atproto-did file, and DNS TXT lookup failed: %w", handle, err)
	}
	for _, rec := range recs {
		if did, ok := strings.CutPrefix(rec, "did="); ok {
			return did, nil
		}
	}

	return "", fmt.Errorf("resolving handle %q: no did= TXT record found at _atproto.%s", handle, handle)
}

type didDocument struct {
	Service []struct {
		ID              string `json:"id"`
		ServiceEndpoint string `json:"serviceEndpoint"`
	} `json:"service"`
}

func pdsFromDIDDocument(doc didDocument) (string, error) {
	for _, svc := range doc.Service {
		if strings.HasSuffix(svc.ID, "#atproto_pds") {
			return strings.TrimRight(svc.ServiceEndpoint, "/"), nil
		}
	}
	return "", fmt.Errorf("no #atproto_pds service found in DID document")
}

// defaultPLCHost is the production PLC directory, used unless plcHostOverride says otherwise.
const defaultPLCHost = "https://plc.directory"

// resolvePDS looks up the PDS serviceEndpoint for a did:plc or did:web identifier.
// plcHostOverride replaces the default PLC directory host for a did:plc lookup.
func resolvePDS(did, plcHostOverride string) (string, error) {
	var docURL string
	switch {
	case strings.HasPrefix(did, "did:plc:"):
		plcHost := strings.TrimRight(plcHostOverride, "/")
		if plcHost == "" {
			plcHost = defaultPLCHost
		}
		docURL = plcHost + "/" + did
	case strings.HasPrefix(did, "did:web:"):
		u, err := didWebDocumentURL(did)
		if err != nil {
			return "", err
		}
		docURL = u
	default:
		method, _, _ := strings.Cut(strings.TrimPrefix(did, "did:"), ":")
		return "", fmt.Errorf("unsupported DID method %q in %q", method, did)
	}

	resp, err := httpClient.Get(docURL)
	if err != nil {
		return "", fmt.Errorf("resolving PDS for %q: %w", did, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolving PDS for %q: %s returned %s", did, docURL, resp.Status)
	}

	var doc didDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("resolving PDS for %q: %w", did, err)
	}
	return pdsFromDIDDocument(doc)
}

// didWebDocumentURL turns a did:web identifier into the https:// URL of its DID document.
// atproto only resolves hostname-level did:web identifiers: per the did:web spec a literal
// colon (once past "did:web:") denotes a path segment, which is rejected here rather than
// silently resolved from the wrong place; a port must be given %3A-encoded, and is decoded
// back to a literal ':' since it isn't a path separator.
func didWebDocumentURL(did string) (string, error) {
	rest := strings.TrimPrefix(did, "did:web:")
	if rest == "" {
		return "", fmt.Errorf("malformed did:web %q", did)
	}

	if strings.Contains(rest, ":") {
		return "", fmt.Errorf("did:web %q has a path component, which atproto doesn't resolve a PDS document from", did)
	}

	host, err := url.QueryUnescape(rest)
	if err != nil {
		return "", fmt.Errorf("malformed did:web %q: %w", did, err)
	}

	return fmt.Sprintf("https://%s/.well-known/did.json", host), nil
}

func (c *Client) get(nsid string, query url.Values, authed bool, out any) error {
	u := fmt.Sprintf("%s/xrpc/%s", c.pdsHost, nsid)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return c.do(req, authed, out)
}

func (c *Client) post(nsid string, body any, authed bool, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/xrpc/%s", c.pdsHost, nsid), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, authed, out)
}

func (c *Client) do(req *http.Request, authed bool, out any) error {
	if authed {
		req.Header.Set("Authorization", "Bearer "+c.accessJwt)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return xrpcError(req.Method, req.URL.Path, resp.StatusCode, body)
	}

	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

func xrpcError(method, path string, status int, body []byte) error {
	var xerr struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	json.Unmarshal(body, &xerr)

	if status == http.StatusBadRequest && xerr.Error == "RecordNotFound" {
		return fmt.Errorf("%w: %s", ErrRecordNotFound, xerr.Message)
	}

	return fmt.Errorf("%s %s: %d %s: %s", method, path, status, xerr.Error, xerr.Message)
}

// UploadBlob uploads data to the PDS, returning the blob reference the server assigns (its
// CID is computed server-side, never locally).
func (c *Client) UploadBlob(data []byte, mimetype string) (Blob, error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/xrpc/com.atproto.repo.uploadBlob", c.pdsHost), bytes.NewReader(data))
	if err != nil {
		return Blob{}, err
	}
	req.Header.Set("Content-Type", mimetype)

	var out struct {
		Blob Blob `json:"blob"`
	}
	if err := c.do(req, true, &out); err != nil {
		return Blob{}, fmt.Errorf("uploading blob: %w", err)
	}
	return out.Blob, nil
}

// PutRecord writes record to this client's own repo under rkey.
func (c *Client) PutRecord(rkey string, record Record) error {
	body := map[string]any{
		"repo":       c.did,
		"collection": RecordType,
		"rkey":       rkey,
		"record":     record,
	}
	if err := c.post("com.atproto.repo.putRecord", body, true, nil); err != nil {
		return fmt.Errorf("putting record %q: %w", rkey, err)
	}
	return nil
}

// GetRecord fetches the record at rkey in did's repo, along with its CID.
func (c *Client) GetRecord(did, rkey string) (Record, string, error) {
	var out struct {
		CID   string `json:"cid"`
		Value Record `json:"value"`
	}
	query := url.Values{"repo": {did}, "collection": {RecordType}, "rkey": {rkey}}
	if err := c.get("com.atproto.repo.getRecord", query, false, &out); err != nil {
		return Record{}, "", err
	}
	return out.Value, out.CID, nil
}

// RecordExists reports whether rkey already exists in this client's own repo.
func (c *Client) RecordExists(rkey string) (bool, error) {
	_, _, err := c.GetRecord(c.did, rkey)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

// GetBlob fetches the raw bytes of the blob identified by cid from did's repo.
func (c *Client) GetBlob(did, cid string) ([]byte, error) {
	u := fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?%s", c.pdsHost, url.Values{"did": {did}, "cid": {cid}}.Encode())

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("getting blob %s: %w", cid, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("getting blob %s: %w", cid, xrpcError(http.MethodGet, resp.Request.URL.Path, resp.StatusCode, body))
	}

	return body, nil
}
