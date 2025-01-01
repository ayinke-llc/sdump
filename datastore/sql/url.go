package sql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ayinke-llc/sdump"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type urlRepositoryTable struct {
	inner *bun.DB
}

func NewURLRepositoryTable(db *bun.DB) sdump.URLRepository {
	return &urlRepositoryTable{
		inner: db,
	}
}

func (u *urlRepositoryTable) Create(ctx context.Context,
	model *sdump.URLEndpoint,
) error {
	_, err := bun.NewInsertQuery(u.inner).Model(model).
		Exec(ctx)
	return err
}

func (u *urlRepositoryTable) Get(ctx context.Context,
	opts *sdump.FindURLOptions,
) (*sdump.URLEndpoint, error) {
	res := new(sdump.URLEndpoint)

	query := bun.NewSelectQuery(u.inner).Model(res)

	if opts.ID != uuid.Nil {
		query = query.Where("id = ?", opts.ID)
	}

	if opts.Reference != "" {
		query = query.Where("reference = ?", opts.Reference)
	}

	err := query.Scan(ctx, res)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sdump.ErrURLEndpointNotFound
	}

	return res, err
}

func (u *urlRepositoryTable) Latest(ctx context.Context, userID uuid.UUID) (
	*sdump.URLEndpoint, error,
) {
	ret := new(sdump.URLEndpoint)

	err := bun.NewSelectQuery(u.inner).Model(ret).
		Order("created_at DESC").
		Where("user_id = ?", userID).
		Limit(1).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, sdump.ErrURLEndpointNotFound
	}

	return ret, err
}
