package trace

import (
	"context"

	"github.com/google/uuid"
)

type traceIDKey struct{}

func NewTraceID() string {
	return uuid.NewString()
}

func PackTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

func UnpackTraceID(ctx context.Context) string {
	value := ctx.Value(traceIDKey{})

	traceID, ok := value.(string)
	if !ok {
		panic("trace ID not found in context")
	}

	return traceID
}
