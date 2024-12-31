package sql

import (
	"database/sql"

	"github.com/ayinke-llc/sdump/config"
	"github.com/oiime/logrusbun"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bunotel"
)

func newPsql(cfg config.DatabaseConfig) (*bun.DB, error) {
	pgdb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(cfg.DSN)))

	db := bun.NewDB(pgdb, pgdialect.New())
	log := logrus.New()

	db.AddQueryHook(bunotel.NewQueryHook(bunotel.WithDBName("getclaimclaim")))

	if cfg.LogQueries {
		db.AddQueryHook(logrusbun.NewQueryHook(logrusbun.QueryHookOptions{Logger: log}))
	}

	return db, db.Ping()
}

func newSqlite(cfg config.DatabaseConfig) (*bun.DB, error) {
	sqlite, err := sql.Open(sqliteshim.ShimName, cfg.DSN)
	if err != nil {
		panic(err)
	}

	db := bun.NewDB(sqlite, sqlitedialect.New())
	log := logrus.New()

	if cfg.LogQueries {
		db.AddQueryHook(logrusbun.NewQueryHook(logrusbun.QueryHookOptions{Logger: log}))
	}

	return db, db.Ping()
}

func New(cfg config.DatabaseConfig) (*bun.DB, error) {
	if cfg.Driver == config.DatabaseTypeSqlite {
		return newSqlite(cfg)
	}

	return newPsql(cfg)
}
