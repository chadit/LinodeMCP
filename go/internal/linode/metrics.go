package linode

import (
	"context"
	"strings"
)

// APIRecorder records metrics for a single Linode API HTTP round trip. The
// server injects an implementation into the request context (see
// WithAPIRecorder) so the client records without taking a direct dependency
// on the observability package. *observability.Observability satisfies it
// through its RecordAPIRequest method.
type APIRecorder interface {
	RecordAPIRequest(ctx context.Context, endpoint, method string, status int, duration float64)
}

type apiRecorderKey struct{}

// WithAPIRecorder returns a context carrying the recorder the client uses to
// report each API request. A nil recorder is ignored so callers can pass a
// not-yet-wired recorder without branching.
func WithAPIRecorder(ctx context.Context, recorder APIRecorder) context.Context {
	if recorder == nil {
		return ctx
	}

	return context.WithValue(ctx, apiRecorderKey{}, recorder)
}

// apiRecorderFromContext returns the recorder set by WithAPIRecorder, or nil
// when none is present (the default, so a client used outside the server
// dispatch path records nothing rather than crashing).
func apiRecorderFromContext(ctx context.Context) APIRecorder {
	recorder, _ := ctx.Value(apiRecorderKey{}).(APIRecorder)

	return recorder
}

// metricsEndpoint trims the query string so the endpoint metric label stays
// at the path level. Path IDs still vary, so the endpoint label is higher
// cardinality than method or status; tighten it to route templates if the
// series count becomes a memory concern.
func metricsEndpoint(endpoint string) string {
	if before, _, ok := strings.Cut(endpoint, "?"); ok {
		return before
	}

	return endpoint
}
