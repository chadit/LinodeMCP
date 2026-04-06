package server

import "errors"

// Sentinel errors for server operations.
var (
	ErrConfigNil             = errors.New("config cannot be nil")
	ErrExecuteNotImplemented = errors.New("execute method not implemented for wrapper")
)
