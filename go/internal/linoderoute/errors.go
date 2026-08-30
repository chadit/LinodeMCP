package linoderoute

import "errors"

var (
	// ErrNoRoute reports a tool the proto contract declares no route for.
	ErrNoRoute = errors.New("tool declares no route")
	// ErrNoToolRoute reports a message handed to ToolOf that carries no
	// tool_route: a response type, or a meta tool's input.
	ErrNoToolRoute = errors.New("message declares no tool_route")
	// ErrValueCount reports the wrong number of path values for a route.
	ErrValueCount = errors.New("path value count does not match the route")
	// ErrValueType reports a path value of a type that cannot fill a slot.
	ErrValueType = errors.New("path value type cannot fill a path slot")
	// ErrEmptyValue reports an empty path value.
	ErrEmptyValue = errors.New("empty path value")
	// ErrTemplate reports a path template whose slots do not parse, or a route
	// whose slots disagree with its template.
	ErrTemplate = errors.New("malformed path template")
	// ErrAPISurface reports a declared surface this build cannot address. It is
	// refused rather than defaulted to v4: a beta-only route answered on /v4 is
	// a 404, and a surface with no path segment is not reachable by swapping one.
	ErrAPISurface = errors.New("unknown API surface")
	// ErrDeclaration reports a message whose options do not describe exactly
	// one tool at exactly one capability tier.
	ErrDeclaration = errors.New("broken tool declaration")
	// ErrRegistered reports staged tools that differ from the contract.
	ErrRegistered = errors.New("staged tools do not match the contract")
)

// IsContractError reports whether err came from building a request: no route in
// the proto contract, or path values that cannot fill one. Nothing was sent, so
// callers keep these out of the transport error class and do not retry.
// ErrDeclaration and ErrRegistered are startup failures no request path reaches.
func IsContractError(err error) bool {
	return errors.Is(err, ErrNoRoute) ||
		errors.Is(err, ErrNoToolRoute) ||
		errors.Is(err, ErrValueCount) ||
		errors.Is(err, ErrValueType) ||
		errors.Is(err, ErrEmptyValue) ||
		errors.Is(err, ErrTemplate) ||
		errors.Is(err, ErrAPISurface)
}
