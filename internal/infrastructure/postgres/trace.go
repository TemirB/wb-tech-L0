package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/tracelog"
)

type slogTracer struct {
	logger *slog.Logger
}

func newSlogTracer(l *slog.Logger) *slogTracer {
	return &slogTracer{logger: l}
}

func (t *slogTracer) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	attrs := []any{
		"sql", data["sql"],
		"args", data["args"],
		"time", data["time"],
	}

	if level == tracelog.LogLevelError {
		attrs = append(attrs, "err", data["err"])
	}

	switch level {
	case tracelog.LogLevelDebug:
		t.logger.Debug(msg, attrs...)
	case tracelog.LogLevelInfo:
		t.logger.Info(msg, attrs...)
	case tracelog.LogLevelWarn:
		t.logger.Warn(msg, attrs...)
	case tracelog.LogLevelError:
		t.logger.Error(msg, attrs...)
	}
}
