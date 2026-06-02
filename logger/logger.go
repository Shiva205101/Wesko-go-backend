package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"vesko/requestctx"

	"go.opentelemetry.io/otel/trace"
)

type contextKey struct{}

var loggerKey = contextKey{}

// New creates a new slog.Logger with environment-specific handlers.
func New(service string, env string) *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production", "staging":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		// Pretty handler for development
		handler = &PrettyHandler{
			out:  os.Stdout,
			opts: opts,
		}
	}

	return slog.New(handler).With("service", service)
}

// FromContext returns the logger stored in the context.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// ToContext returns a new context with the given logger stored.
func ToContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// PrettyHandler is a custom handler for clean, human-readable logs.
type PrettyHandler struct {
	out  io.Writer
	opts *slog.HandlerOptions
}

func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

const (
	reset   = "\033[0m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	gray    = "\033[90m"
)

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()
	levelColor := reset
	switch r.Level {
	case slog.LevelDebug:
		levelColor = gray
	case slog.LevelInfo:
		levelColor = green
	case slog.LevelWarn:
		levelColor = yellow
	case slog.LevelError:
		levelColor = red
	}

	timeStr := r.Time.Format("15:04:05.000")

	// Get IDs from record attributes or context
	requestID := ""
	traceID := ""

	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "request_id" {
			requestID = a.Value.String()
		}
		if a.Key == "trace_id" {
			traceID = a.Value.String()
		}
		return true
	})

	// Fallback to context if not in attributes
	if requestID == "" {
		requestID = requestctx.RequestID(ctx)
	}
	if traceID == "" {
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			traceID = span.SpanContext().TraceID().String()
		}
	}

	// Format IDs
	idPart := ""
	if requestID != "" || traceID != "" {
		req := "none"
		if requestID != "" {
			req = requestID
			// Shorten only if very long (like a UUID), but 8-16 is usually fine
			if len(req) > 16 {
				req = req[:8]
			}
		}

		trace := "none"
		if traceID != "" {
			trace = traceID
			// Standard OTel trace IDs are 32 chars, keep them full as requested
		}

		idPart = fmt.Sprintf("%s[req:%s] [trace:%s]%s ", cyan, req, trace, reset)
	}

	// Build the message
	msg := r.Message

	// If it's an HTTP log, make it even cleaner
	isHTTP := strings.Contains(msg, "http request")
	method, path, status, latency := "", "", 0, 0.0

	// Collect other attributes
	attrs := ""
	r.Attrs(func(a slog.Attr) bool {
		// Skip noisy metadata
		if a.Key == "service" || a.Key == "component" || a.Key == "request_id" || a.Key == "trace_id" {
			return true
		}

		// If it's HTTP, capture specific fields and skip them in generic attrs
		if isHTTP {
			switch a.Key {
			case "method":
				method = a.Value.String()
				return true
			case "path":
				path = a.Value.String()
				return true
			case "status":
				status = int(a.Value.Int64())
				return true
			case "latency_ms":
				latency = a.Value.Float64()
				return true
			case "created_at": // We already have the time
				return true
			}
		}

		attrs += fmt.Sprintf(" %s%s%s=%v", magenta, a.Key, reset, a.Value.Any())
		return true
	})

	if isHTTP && method != "" {
		statusColor := green
		if status >= 500 {
			statusColor = red
		} else if status >= 400 {
			statusColor = yellow
		}
		// Added missing %s for statusColor
		fmt.Fprintf(h.out, "%s%s %s%s%s %s%s %s%s %s%s %s%d%s (%.2fms)%s\n",
			gray, timeStr, reset, levelColor, level, reset, idPart, blue, method, reset, path, statusColor, status, reset, latency, attrs)
		return nil
	}

	fmt.Fprintf(h.out, "%s%s %s%s%s %s%s%s%s\n", gray, timeStr, reset, levelColor, level, reset, idPart, msg, attrs)
	return nil
}

func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Simple implementation for WithAttrs
	return h
}

func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	return h
}
