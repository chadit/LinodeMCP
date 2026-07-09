package main

import "errors"

// errTargetMissing is the hard-fail sentinel: a target symbol was renamed,
// deleted, or produced an empty set. Callers wrap it (never redefine it) so a
// silently empty extraction can never look like success to the Python gate.
var errTargetMissing = errors.New("target not found or extracted an empty value set")
