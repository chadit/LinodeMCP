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
	// ErrNilConfig is returned by WriteAtomic when the caller passes
	// nil instead of a Config pointer. Callers can match with
	// errors.Is to distinguish the programmer-error path from on-disk
	// I/O failures.
	ErrNilConfig = errors.New("config: cannot write nil config")
)
