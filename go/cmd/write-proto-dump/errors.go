package main

import "errors"

// errUnknownSurface reports a -surface value other than write, read, or input.
var errUnknownSurface = errors.New("unknown surface")
