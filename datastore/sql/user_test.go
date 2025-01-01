//go:build integration
// +build integration

package sql

import (
	"context"
	"testing"

	"github.com/ayinke-llc/sdump"
	"github.com/stretchr/testify/require"
)

// see users.yml
// var userID = uuid.MustParse("8511ac86-5079-42ae-a030-cb46e6dbfbda")

func TestUserRepository_Create(t *testing.T) {
	client, teardownFunc := setupPostgresDatabase(t)
	defer teardownFunc()

	userStore := NewUserRepositoryTable(client)

	require.NoError(t, userStore.Create(context.Background(), &sdump.User{
		SSHFingerPrint: "oops",
		IsBanned:       true,
	}))
}

func TestUserRepository_Find(t *testing.T) {
	client, teardownFunc := setupPostgresDatabase(t)
	defer teardownFunc()

	userStore := NewUserRepositoryTable(client)

	require.NoError(t, userStore.Create(context.Background(), &sdump.User{
		SSHFingerPrint: "oops",
		IsBanned:       true,
	}))

	_, err := userStore.Find(context.Background(), &sdump.FindUserOptions{
		SSHKeyFingerprint: "oops",
	})

	require.NoError(t, err)
}
