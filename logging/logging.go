// Package logging provides structured, trace-aware logging for MIST tools.
// Built on log/slog (standard library since Go 1.21), it automatically
// includes trace_id and span_id from context in every log entry.
//
// Usage:
//
//	log := logging.New("matchspec", logging.LevelInfo)
//	log.Info(ctx, "eval started", "suite", "math", "tasks", 42)
//	log.Error(ctx, "eval failed", "error", err)
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/greynewell/mist-go/trace"
)

// Level aliases slog.Level for convenience.
type Level = slog.Level

// Standard log levels.
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Logger is a trace-aware structured logger for MIST tools.
type Logger struct {
	slog  *slog.Logger
	tool  string
	level *slog.LevelVar
}

// Option configures a Logger.
type Option func(*config)

type config struct {
	writer io.Writer
	format string // "json" or "text"
}

// WithWriter sets the output writer. Default is os.Stderr.
func WithWriter(w io.Writer) Option {
	return func(c *config) { c.writer = w }
}

// WithFormat sets the output format: "json" for production, "text" for dev.
// Default is "json".
func WithFormat(format string) Option {
	return func(c *config) { c.format = format }
}

// New creates a Logger for the given tool name and minimum level.
func New(tool string, level Level, opts ...Option) *Logger {
	cfg := config{
		writer: os.Stderr,
		format: "json",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	lv := &slog.LevelVar{}
	lv.Set(level)

	var handler slog.Handler
	handlerOpts := &slog.HandlerOptions{Level: lv}

	if cfg.format == "text" {
		handler = slog.NewTextHandler(cfg.writer, handlerOpts)
	} else {
		handler = slog.NewJSONHandler(cfg.writer, handlerOpts)
	}

	// Add tool name as a permanent attribute.
	handler = handler.WithAttrs([]slog.Attr{
		slog.String("tool", tool),
	})

	return &Logger{
		slog:  slog.New(handler),
		tool:  tool,
		level: lv,
	}
}

// SetLevel dynamically changes the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.level.Set(level)
}

// Debug logs at debug level.
func (l *Logger) Debug(ctx context.Context, msg string, args ...any) {
	l.log(ctx, LevelDebug, msg, args...)
}

// Info logs at info level.
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	l.log(ctx, LevelInfo, msg, args...)
}

// Warn logs at warn level.
func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
	l.log(ctx, LevelWarn, msg, args...)
}

// Error logs at error level.
func (l *Logger) Error(ctx context.Context, msg string, args ...any) {
	l.log(ctx, LevelError, msg, args...)
}

func (l *Logger) log(ctx context.Context, level Level, msg string, args ...any) {
	if !l.slog.Enabled(ctx, level) {
		return
	}

	// Inject trace context if available.
	if span := trace.FromContext(ctx); span != nil {
		args = append(args, "trace_id", span.TraceID, "span_id", span.SpanID)
	}

	l.slog.Log(ctx, level, msg, args...)
}

// With returns a new Logger with additional permanent attributes.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		slog:  l.slog.With(args...),
		tool:  l.tool,
		level: l.level,
	}
}

// Slog returns the underlying slog.Logger for interop with libraries
// that accept *slog.Logger.
func (l *Logger) Slog() *slog.Logger {
	return l.slog
}
