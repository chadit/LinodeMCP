package audit

import "errors"

// ErrJSONLSinkClosed indicates the JSONL sink received a Write call
// after Close. The Sink interface drops the error and logs at
// slog.Warn; tests can detect the condition directly by comparing
// against this sentinel.
var ErrJSONLSinkClosed = errors.New("audit: jsonl sink is closed")
