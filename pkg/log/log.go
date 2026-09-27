package log

import (
	"context"
	"log/slog"
)

var (
	_ Logger        = (*slog.Logger)(nil)
	_ ContextLogger = (*slog.Logger)(nil)
	_ FullLogger    = (*slog.Logger)(nil)
)

// Handler represents a logging handler that processes and formats log records for output or further handling.
type Handler slog.Handler

// Logger defines the core logging methods without context.
type Logger interface {
	// Debug logs at Debug level
	Debug(msg string, args ...any)

	// Info logs at Info level
	Info(msg string, args ...any)

	// Warn logs at Warn level
	Warn(msg string, args ...any)

	// Error logs at Error level
	Error(msg string, args ...any)
}

// ContextLogger defines the core logging methods with context.
type ContextLogger interface {
	// DebugContext logs at Debug level with context
	DebugContext(ctx context.Context, msg string, args ...any)

	// InfoContext logs at Info level with context
	InfoContext(ctx context.Context, msg string, args ...any)

	// WarnContext logs at Warn level with context
	WarnContext(ctx context.Context, msg string, args ...any)

	// ErrorContext logs at Error level with context
	ErrorContext(ctx context.Context, msg string, args ...any)
}

// FullLogger combines Logger and ContextLogger with access to the underlying handler. It matches
// *slog.Logger, so the standard logger can be passed wherever a FullLogger is required.
type FullLogger interface {
	Logger
	ContextLogger

	// Handler returns the handler that processes the records.
	Handler() slog.Handler
}
