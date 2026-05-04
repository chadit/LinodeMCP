package observability

import "errors"

// Sentinel errors for observability package.
var (
	errOTLPNotImplemented = errors.New("OTLP metrics not yet implemented")
)
