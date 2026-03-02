package logger

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// New creates a configured slog.Logger.
// level: "debug", "info", "warn", "error"
func New(serviceName, level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	})

	return slog.New(handler).With(slog.String("service", serviceName))
}

// WithContext extracts trace_id, span_id, and request_id from context
// and returns a logger with those fields attached.
func WithContext(logger *slog.Logger, ctx context.Context) *slog.Logger {
	l := logger

	// Extract OpenTelemetry trace context
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		l = l.With(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	// Extract request ID if present
	if reqID, ok := ctx.Value(requestIDKey).(string); ok {
		l = l.With(slog.String("request_id", reqID))
	}

	return l
}

// ContextWithRequestID adds a request ID to the context.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}
