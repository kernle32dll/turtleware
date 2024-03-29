package turtleware

import (
	"context"
	"log/slog"
)

type ctxLoggingKey int

const (
	ctxSlogLogger ctxLoggingKey = iota
)

// FromContext returns a slog.Logger from ctx or nil if no such Logger is found.
func FromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(ctxSlogLogger).(*slog.Logger)
	if !ok {
		return nil
	}

	return logger
}

// FromContextOrDiscard returns a slog.Logger from ctx. If no Logger is found, this
// returns a slog.Logger that discards all log messages.
func FromContextOrDiscard(ctx context.Context) *slog.Logger {
	if logger := FromContext(ctx); logger != nil {
		return logger
	}
	return slog.New(slog.DiscardHandler)
}

// NewContext returns a new Context, derived from ctx, which carries the
// provided slog.Logger.
func NewContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxSlogLogger, logger)
}
