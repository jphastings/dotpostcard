package pdsupload

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/internal/testpds"
	"github.com/jphastings/dotpostcard/formats/web"
	"github.com/jphastings/dotpostcard/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadWebFileKeepsBytesAndKeyStable(t *testing.T) {
	srv := testpds.New("did:plc:testuser")
	defer srv.Close()
	client, err := atproto.Login("alice.example", "app-password", srv.URL, "")
	require.NoError(t, err)

	pc := testhelpers.SamplePostcard
	pc.Name = "harbour"
	pc.Meta.SentOn = nil
	pc.Front = testhelpers.TestImages["sample-front.png"]
	pc.Back = testhelpers.TestImages["sample-back.png"]
	fws, err := web.DefaultCodec.Encode(pc, nil)
	require.NoError(t, err)
	data, err := fws[0].Bytes()
	require.NoError(t, err)

	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	uri, cid, reused, err := UploadWebFile(client, data, fws[0].Mimetype, "harbour", day)
	require.NoError(t, err)
	assert.False(t, reused)

	record, recordCID, err := client.GetRecord(client.DID(), uri[len(uri)-13:])
	require.NoError(t, err)
	assert.Equal(t, cid, recordCID)
	assert.Equal(t, "Harbour", record.Title)
	blob, err := client.GetBlob(client.DID(), record.Image.Ref.Link)
	require.NoError(t, err)
	assert.Equal(t, sha256.Sum256(data), sha256.Sum256(blob))

	uri2, cid2, reused, err := UploadWebFile(client, data, fws[0].Mimetype, "harbour", day)
	require.NoError(t, err)
	assert.True(t, reused)
	assert.Equal(t, uri, uri2)
	assert.Equal(t, cid, cid2)

	otherDay, _, _, err := UploadWebFile(client, data, fws[0].Mimetype, "harbour", day.AddDate(0, 0, 1))
	require.NoError(t, err)
	assert.NotEqual(t, uri, otherDay, "a different fallback day is a different card")
}
