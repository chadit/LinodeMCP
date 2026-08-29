package main

import "errors"

var (
	// A clean report over zero files would prove nothing.
	errScannedNothing = errors.New("scan covered nothing")
	errNoModules      = errors.New("buf.yaml declares no module to scan")

	// A shape the model cannot re-render is one it cannot put back.
	errUnparsedDeclaration = errors.New("declaration does not parse")
	errUnparsedTail        = errors.New("declaration tail does not parse")
	errStrayClose          = errors.New("closing brace outside any block")
	errUnclosedBlock       = errors.New("block is never closed")

	// The second declaration would answer for the first everywhere.
	errDuplicateDeclaration = errors.New("two declarations claim one surface name")
	errDuplicateRoute       = errors.New("two messages claim one surface route")

	errNoSurfaceField = errors.New("overlay anchors a field the surface does not carry")
	errNoSurfaceValue = errors.New("overlay anchors an enum value the surface does not carry")
	errNoSurfaceRoute = errors.New("overlay anchors a route the surface does not carry")

	// Rendering it would leave nothing after the equals.
	errNoSurfaceLocation = errors.New("tail states a field_location the surface does not carry")

	// Copying the option through would leave no route and no sign it was skipped.
	errUnparsedRoute       = errors.New("tool route does not parse")
	errRouteOutsideMessage = errors.New("tool route sits outside any message")

	// The addition would reach the tree with none of the MCP semantics this repo states.
	errUncoveredField = errors.New("upstream field has no overlay entry")
	errUncoveredValue = errors.New("upstream enum value has no overlay entry")
	errUncoveredRoute = errors.New("upstream route has no overlay entry")

	// Merging against a half-read upstream would read as drift.
	errUnreadableSurface = errors.New("surface descriptor does not parse")
	errEmptySurface      = errors.New("surface descriptor states no field, enum value, or route")

	errNoRetiredAnchor = errors.New("no declaration to retire under that name")
	errModeConflict    = errors.New("-emit-surface and -surface name two different runs")
	errNoOutDir        = errors.New("-surface needs -out to write the merged tree to")
)
