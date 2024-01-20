package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	testfixtures "github.com/go-testfixtures/testfixtures/v3"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uptrace/bun"
)

func prepareTestDatabase(t *testing.T, dsn string) {
	t.Helper()

	var err error

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)

	require.NoError(t, db.Ping())

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	require.NoError(t, err)

	migrator, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", "migrations"), "postgres", driver)
	require.NoError(t, err)

	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		require.NoError(t, err)
	}

	fixtures, err := testfixtures.New(
		testfixtures.Database(db),
		testfixtures.Dialect("postgres"),
		testfixtures.Directory("testdata/fixtures"),
	)
	require.NoError(t, err)

	require.NoError(t, fixtures.Load())
}

// setupDatabase spins up a new Postgres container and returns a closure
// please always make sure to call the closure as it is the teardown function
func setupDatabase(t *testing.T) (*bun.DB, func()) {
	t.Helper()

	var dsn string

	containerReq := testcontainers.ContainerRequest{
		Image:        "postgres:latest",
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForListeningPort("5432/tcp"),
		Env: map[string]string{
			"POSTGRES_DB":       "sdumptest",
			"POSTGRES_PASSWORD": "sdump",
			"POSTGRES_USER":     "sdump",
		},
	}

	dbContainer, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: containerReq,
			Started:          true,
		})

	require.NoError(t, err)

	port, err := dbContainer.MappedPort(context.Background(), "5432")
	require.NoError(t, err)

	dsn = fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", "sdump", "sdump",
		fmt.Sprintf("localhost:%s", port.Port()), "sdumptest")

	prepareTestDatabase(t, dsn)

	client, err := New(dsn, false)
	require.NoError(t, err)

	return client, func() {
		err := dbContainer.Terminate(context.Background())
		require.NoError(t, err)
	}
}

func verifyMatch(t *testing.T, v interface{}) {
	g := goldie.New(t, goldie.WithFixtureDir("./testdata"))

	b := new(bytes.Buffer)

	err := json.NewEncoder(b).Encode(v)
	require.NoError(t, err)

	g.Assert(t, t.Name(), b.Bytes())
}
