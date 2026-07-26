package main

import "errors"

// errNoRoutes is the hard-fail sentinel: the resolver walked the package and
// produced an empty route set. Callers wrap it (never redefine it) so a
// resolver that quietly stopped following the client's call graph can never
// reach the gate looking like a client that legitimately has no routes.
var errNoRoutes = errors.New("no routes resolved from the client package")

// errRequestFuncMissing is the other hard fail: the function every route is
// resolved outward from was renamed, or its method and path parameters were.
// Resolution has no seed then, and the whole surface would come back empty for
// a reason that has nothing to do with the routes.
var errRequestFuncMissing = errors.New("request function not found with the expected parameters")
