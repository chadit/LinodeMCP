package cli

import "errors"

// Sentinel errors for the CLI subcommands. Per the project convention,
// every sentinel lives here rather than beside its use site.
//
// The request/usage errors map to exit code 2 (a problem with how the
// call was framed); the dispatch errors are transport/protocol faults
// surfaced to the user as a failed call.
var (
	// ErrArgFormat means a --arg value was not key=value.
	ErrArgFormat = errors.New("argument must be key=value")
	// ErrArgAndJSON means both --arg and --json were supplied; they are
	// mutually exclusive so a script never mixes two argument sources.
	ErrArgAndJSON = errors.New("--arg and --json are mutually exclusive")
	// ErrJSONNotObject means --json parsed but was not a JSON object, so
	// it can't become the tool's argument map.
	ErrJSONNotObject = errors.New("--json must decode to a JSON object")

	// ErrNoResult means the JSON-RPC response carried neither a result
	// nor an error object, which should never happen for tools/call.
	ErrNoResult = errors.New("response has no result")
	// ErrRPCError means the dispatch returned a JSON-RPC error envelope
	// (bad method, malformed params) rather than a tool result.
	ErrRPCError = errors.New("json-rpc error")

	// errInvalidBool is returned when a boolean safety flag gets a value
	// that isn't a recognized true/false literal.
	errInvalidBool = errors.New("not a boolean (want true or false)")

	// errProfileSwitchFailed is reported by the TUI profile switcher when
	// the underlying RunProfileUse config write returns a non-zero code
	// (unknown profile, or the config file could not be written).
	errProfileSwitchFailed = errors.New("profile switch failed")
)
