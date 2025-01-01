package sql

import (
	"context"

	"github.com/ayinke-llc/sdump"
	"github.com/uptrace/bun"
)

type ingestRepository struct {
	inner *bun.DB
}

func NewIngestRepository(db *bun.DB) sdump.IngestRepository {
	return &ingestRepository{
		inner: db,
	}
}

func (u *ingestRepository) Create(ctx context.Context,
	model *sdump.IngestHTTPRequest,
) error {
	_, err := bun.NewInsertQuery(u.inner).Model(model).
		Exec(ctx)
	return err
}

func (u *ingestRepository) Delete(ctx context.Context,
	opts *sdump.DeleteIngestedRequestOptions,
) error {
	// This prevents us from deleting the entire database
	// so enforce a time limit is available
	if opts == nil {
		return nil
	}

	deleteQuery := bun.NewDeleteQuery(u.inner).
		Model((*sdump.IngestHTTPRequest)(nil)).
		Where("created_at < ?", opts.Before)

	if !opts.UseSoftDeletes {
		deleteQuery = deleteQuery.ForceDelete()
	}

	_, err := deleteQuery.Exec(ctx)
	return err
}
