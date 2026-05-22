package audit

import "errors"

// ErrJSONLSinkClosed indicates the JSONL sink received a Write call
// after Close. The Sink interface drops the error and logs at
// slog.Warn; tests can detect the condition directly by comparing
// against this sentinel.
var ErrJSONLSinkClosed = errors.New("audit: jsonl sink is closed")

// ErrUnknownGroupByColumn indicates a summary query requested a
// group-by column that is not in the allowlist. Returned by
// ValidateGroupBy so a typo surfaces instead of producing an empty
// grouping.
var ErrUnknownGroupByColumn = errors.New("audit: unknown group_by column")

// ErrUnknownExportFormat indicates an export request named a format
// that is not one of json, csv, or ndjson. Returned by EncodeEvents so
// a typo surfaces rather than producing an empty file.
var ErrUnknownExportFormat = errors.New("audit: unknown export format")

// errAuditDirMissing signals that the audit directory does not exist
// yet. openReadRoot returns it so readers can treat "queried before
// the first event" as an empty result rather than an error, without a
// (nil, nil) return.
var errAuditDirMissing = errors.New("audit: directory does not exist")
