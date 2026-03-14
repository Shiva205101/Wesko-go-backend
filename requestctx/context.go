package requestctx

import (
	"context"
	"time"
)

type contextKey string

const (
	requestIDKey    contextKey = "request_id"
	requestStartKey contextKey = "request_start"
	traceParentKey  contextKey = "trace_parent"
	traceStateKey   contextKey = "trace_state"
)

type Metadata struct {
	RequestID    string
	RequestStart time.Time
	TraceParent  string
	TraceState   string
}

func WithMetadata(ctx context.Context, metadata Metadata) context.Context {
	if metadata.RequestID != "" {
		ctx = context.WithValue(ctx, requestIDKey, metadata.RequestID)
	}
	if !metadata.RequestStart.IsZero() {
		ctx = context.WithValue(ctx, requestStartKey, metadata.RequestStart)
	}
	if metadata.TraceParent != "" {
		ctx = context.WithValue(ctx, traceParentKey, metadata.TraceParent)
	}
	if metadata.TraceState != "" {
		ctx = context.WithValue(ctx, traceStateKey, metadata.TraceState)
	}
	return ctx
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func RequestStart(ctx context.Context) time.Time {
	value, _ := ctx.Value(requestStartKey).(time.Time)
	return value
}

func TraceParent(ctx context.Context) string {
	value, _ := ctx.Value(traceParentKey).(string)
	return value
}

func TraceState(ctx context.Context) string {
	value, _ := ctx.Value(traceStateKey).(string)
	return value
}
