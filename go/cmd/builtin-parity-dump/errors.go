package main

import "errors"

// ErrUnknownCapability is returned when the fixture names a capability that
// the profiles package does not recognize.
var ErrUnknownCapability = errors.New("unknown capability")
