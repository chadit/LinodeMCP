package config

import "errors"

// Sentinel errors for configuration operations.
var (
	ErrConfigFileNotFound   = errors.New("configuration file not found")
	ErrConfigInvalid        = errors.New("configuration file is invalid")
	ErrConfigPermissions    = errors.New("insufficient permissions to access configuration file")
	ErrConfigMalformed      = errors.New("configuration file is malformed")
	ErrEnvironmentNotFound  = errors.New("environment not found in configuration")
	ErrNoEnvironments       = errors.New("no environments defined in configuration")
	ErrEmptyEnvironmentName = errors.New("environment name cannot be empty")
	ErrEmptyServerName      = errors.New("server name cannot be empty")
	ErrEmptyLogLevel        = errors.New("log level cannot be empty")
	ErrMissingAPIURL        = errors.New("api URL is required when token is provided")
	ErrMissingToken         = errors.New("token is required when API URL is provided")
	ErrWatcherStopped       = errors.New("config watcher stopped")
	// ErrNegativeRetentionDays is returned when audit.retention_days is
	// set below zero. Zero means "never delete"; negative is nonsense.
	ErrNegativeRetentionDays = errors.New("audit.retention_days cannot be negative")
	// ErrNilConfig is returned by WriteAtomic when the caller passes
	// nil instead of a Config pointer. Callers can match with
	// errors.Is to distinguish the programmer-error path from on-disk
	// I/O failures.
	ErrNilConfig = errors.New("config: cannot write nil config")
	// ErrInvalidReportOutput is returned when a custom report's output
	// mode is neither "summary" nor "list".
	ErrInvalidReportOutput = errors.New("audit report output must be 'summary' or 'list'")
	// ErrReportScalarAndList is returned when a report filter sets both
	// the scalar and the _in list form of the same field (e.g. both
	// capability and capability_in).
	ErrReportScalarAndList = errors.New("audit report filter cannot set both scalar and _in list for a field")
	// ErrInvalidReportDuration is returned when a report filter's
	// since_offset is not a valid Go duration (e.g. "24h").
	ErrInvalidReportDuration = errors.New("audit report since_offset is not a valid duration")
	// ErrInvalidReportTimestamp is returned when a report filter's since
	// or until is not a valid RFC 3339 timestamp.
	ErrInvalidReportTimestamp = errors.New("audit report since/until is not a valid RFC 3339 timestamp")
)
