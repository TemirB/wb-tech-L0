package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
)

func MustPool(ctx context.Context, dsn string, logger *slog.Logger) *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		panic(err)
	}
	cfg.ConnConfig.Tracer = &tracelog.TraceLog{
		Logger:   newSlogTracer(logger),
		LogLevel: tracelog.LogLevelInfo,
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return pool
}
