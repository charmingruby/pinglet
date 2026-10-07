package logging

import (
	"context"
	"log/slog"
	"os"
)

var Log *Logger

type Logger = slog.Logger

type loggerCtxKey struct{}

func InitLogger() *Logger {
	Log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return Log
}

func WithLogger(ctx context.Context, log *Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, log)
}

func LoggerFromContext(ctx context.Context) *Logger {
	if ctx != nil {
		if log, ok := ctx.Value(loggerCtxKey{}).(*Logger); ok && log != nil {
			return log
		}
	}

	if Log != nil {
		return Log
	}

	return slog.New(slog.DiscardHandler)
}
