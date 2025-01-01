//go:build integration
// +build integration

package sql

import (
	"context"
	"testing"

	"github.com/ayinke-llc/sdump"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// see users.yml
var userID = uuid.MustParse("8511ac86-5079-42ae-a030-cb46e6dbfbda")

func TestURLRepositoryTable_Create(t *testing.T) {
	client, teardownFunc := setupPostgresDatabase(t)
	defer teardownFunc()

	urlStore := NewURLRepositoryTable(client)

	require.NoError(t, urlStore.Create(context.Background(),
		sdump.NewURLEndpoint(userID)))
}

func TestURLRepositoryTable_Get(t *testing.T) {
	client, teardownFunc := setupPostgresDatabase(t)
	defer teardownFunc()

	urlStore := NewURLRepositoryTable(client)

	_, err := urlStore.Get(context.Background(), &sdump.FindURLOptions{
		Reference: uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, err, sdump.ErrURLEndpointNotFound)

	_, err = urlStore.Get(context.Background(), &sdump.FindURLOptions{
		Reference: "cmltfm6g330l5l1vq110", // see fixtures/urls.yml
	})
	require.NoError(t, err)
}

func TestURLRepositoryTable_Latest(t *testing.T) {
	client, teardownFunc := setupPostgresDatabase(t)
	defer teardownFunc()

	urlStore := NewURLRepositoryTable(client)

	endpoint, err := urlStore.Latest(context.Background(), userID)
	require.NoError(t, err)

	require.Equal(t, endpoint.Reference, "cmltg1eg330l5l1vq11g")
}
