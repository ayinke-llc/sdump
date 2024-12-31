package main

import (
	"context"
	"time"

	"github.com/ayinke-llc/sdump"
	"github.com/ayinke-llc/sdump/config"
	sdumpSql "github.com/ayinke-llc/sdump/datastore/sql"
	"github.com/spf13/cobra"
)

func createDeleteCommand(rootCmd *cobra.Command, cfg *config.Config) {
	cmd := &cobra.Command{
		Use:     "delete-http",
		Aliases: []string{"d"},
		Short:   "Deletes all old HTTP requests to preserve DB space",
		RunE: func(_ *cobra.Command, _ []string) error {
			db, err := sdumpSql.New(cfg.HTTP.Database)
			if err != nil {
				return err
			}

			ingestStore := sdumpSql.NewIngestRepository(db)

			before := time.Now().Add(-1 * cfg.Cron.TTL)

			return ingestStore.Delete(context.Background(), &sdump.DeleteIngestedRequestOptions{
				Before:         before,
				UseSoftDeletes: cfg.Cron.SoftDeletes,
			})
		},
	}

	rootCmd.AddCommand(cmd)
}
