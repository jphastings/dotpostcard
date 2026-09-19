package atproto

import (
	"testing"

	"github.com/jphastings/dotpostcard/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginAndRecordExists(t *testing.T) {
	srv := newFakePDS("did:plc:testuser")
	defer srv.Close()

	client, err := Login("alice.example", "app-password", srv.URL, "")
	require.NoError(t, err)
	assert.Equal(t, "did:plc:testuser", client.DID())

	exists, err := client.RecordExists("does-not-exist")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, client.PutRecord("some-card", FromMetadata(types.Metadata{}, testBlob)))

	exists, err = client.RecordExists("some-card")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestAnonymousResolvesHandleViaOverrideHost(t *testing.T) {
	srv := newFakePDS("did:plc:testuser")
	defer srv.Close()

	client, err := Anonymous("alice.example", srv.URL, "")
	require.NoError(t, err)
	assert.Equal(t, "did:plc:testuser", client.DID())
}

func TestAnonymousWithDIDAuthoritySkipsLookup(t *testing.T) {
	client, err := Anonymous("did:plc:testuser", "http://pds.invalid", "")
	require.NoError(t, err)
	assert.Equal(t, "did:plc:testuser", client.DID())
}
