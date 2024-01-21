//go:build integration
// +build integration

package postgres

import (
	"context"
	"testing"

	"github.com/adelowo/sdump"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestURLRepositoryTable_Create(t *testing.T) {
	client, teardownFunc := setupDatabase(t)
	defer teardownFunc()

	urlStore := NewURLRepositoryTable(client)

	require.NoError(t, urlStore.Create(context.Background(), sdump.NewURLEndpoint()))
}

func TestURLRepositoryTable_Get(t *testing.T) {
	client, teardownFunc := setupDatabase(t)
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