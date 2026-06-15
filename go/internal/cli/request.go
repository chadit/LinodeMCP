package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// JSON-schema type names the coercion understands. Anything else (or an
// absent type) is treated as a string, matching how a permissive MCP
// client would pass an unknown field.
const (
	schemaTypeNumber  = "number"
	schemaTypeInteger = "integer"
	schemaTypeBoolean = "boolean"
)

// Safety-flag field names. These mirror the keys the MCP middleware
// reads off a tools/call, so folding a CLI flag into the request map
// under the same name reaches the same code path an MCP client would.
const (
	fieldDryRun         = "dry_run"
	fieldConfirm        = "confirm"
	fieldMode           = "mode"
	fieldPlanID         = "plan_id"
	fieldConfirmedDry   = "confirmed_dry_run"
	fieldYolo           = "yolo"
	fieldEnvironment    = "environment"
	fieldName           = "name"
	fieldArguments      = "arguments"
	jsonrpcMethodCall   = "tools/call"
	jsonrpcVersionField = "2.0"
)

// SafetyFlags carries the first-class safety controls a CLI invocation
// can set. They fold into the request map under the same field names the
// MCP dispatch reads, so the CLI never hand-crafts JSON for them and the
// dry-run/two-stage/confirm gating behaves identically to an MCP client.
//
// Bool flags are tri-state via pointers: nil means the flag was absent
// (leave the field off the request entirely), so a tool's own default
// applies. A non-nil false still writes the field, which lets a script
// be explicit. Mode, PlanID, and Environment are empty-means-absent.
type SafetyFlags struct {
	DryRun       *bool
	Confirm      *bool
	ConfirmedDry *bool
	Yolo         *bool
	Mode         string
	PlanID       string
	Environment  string
}

// BuildArguments turns the raw argument inputs into the map that becomes
// the tools/call "arguments" object. Exactly one of jsonArg / kvArgs may
// be non-empty. With --json, the object is parsed verbatim (the schema
// does not re-type it; the caller trusts the JSON). With --arg pairs,
// each value is coerced to the type the tool's schema declares for that
// property. Unknown properties stay strings.
//
// Exported because the Phase 2 TUI builds its tool-form requests through
// the same coercion, so the CLI and the TUI map schema types identically.
func BuildArguments(schema mcp.ToolInputSchema, jsonArg string, kvArgs []string) (map[string]any, error) {
	if jsonArg != "" && len(kvArgs) > 0 {
		return nil, ErrArgAndJSON
	}

	if jsonArg != "" {
		return decodeJSONObject(jsonArg)
	}

	args := make(map[string]any, len(kvArgs))

	for _, pair := range kvArgs {
		key, value, err := splitKeyValue(pair)
		if err != nil {
			return nil, err
		}

		args[key] = coerceValue(schema, key, value)
	}

	return args, nil
}

// decodeJSONObject parses a --json payload, requiring a top-level object
// so the result can serve as the arguments map. Malformed JSON is a parse
// error; valid-but-not-an-object JSON (an array, a scalar, null) is
// ErrJSONNotObject, so both bad shapes map cleanly to a usage error.
func decodeJSONObject(jsonArg string) (map[string]any, error) {
	var decoded any
	if err := json.Unmarshal([]byte(jsonArg), &decoded); err != nil {
		return nil, fmt.Errorf("parse --json: %w", err)
	}

	args, isObject := decoded.(map[string]any)
	if !isObject {
		return nil, ErrJSONNotObject
	}

	return args, nil
}

// splitKeyValue parses one key=value token. The value may itself contain
// '=' (e.g. a base64 string), so only the first '=' splits. An empty key
// or a token with no '=' is a format error.
func splitKeyValue(pair string) (string, string, error) {
	idx := strings.IndexByte(pair, '=')
	if idx <= 0 {
		return "", "", fmt.Errorf("%w: %q", ErrArgFormat, pair)
	}

	return pair[:idx], pair[idx+1:], nil
}

// coerceValue converts a string --arg value to the Go type matching the
// tool's declared schema type for key. number -> float64, integer ->
// int64 (falling back to float64 when it isn't a whole number), boolean
// -> bool. A value that doesn't parse to its declared type is left as
// the raw string so the server's own validation produces the error
// message, keeping one source of truth for argument validation.
func coerceValue(schema mcp.ToolInputSchema, key, value string) any {
	switch schemaPropType(schema, key) {
	case schemaTypeNumber:
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	case schemaTypeInteger:
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			return n
		}

		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	case schemaTypeBoolean:
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}

	return value
}

// schemaPropType returns the declared JSON-schema type for property key,
// or "" when the property is absent or untyped. The schema's Properties
// map holds JSON-schema-shaped sub-maps; each entry's "type" is a string.
func schemaPropType(schema mcp.ToolInputSchema, key string) string {
	entry, found := schema.Properties[key]
	if !found {
		return ""
	}

	prop, isMap := entry.(map[string]any)
	if !isMap {
		return ""
	}

	typeVal, _ := prop["type"].(string)

	return typeVal
}

// ApplySafetyFlags folds the safety controls into the arguments map under
// the field names the MCP dispatch reads. Absent flags are not written,
// so a tool's defaults stand. The map is mutated in place and returned
// for chaining. flags is taken by pointer to dodge gocritic's hugeParam.
//
// Exported alongside BuildArguments so the Phase 2 TUI folds the same
// safety controls into its requests the same way.
func ApplySafetyFlags(args map[string]any, flags *SafetyFlags) map[string]any {
	if flags.DryRun != nil {
		args[fieldDryRun] = *flags.DryRun
	}

	if flags.Confirm != nil {
		args[fieldConfirm] = *flags.Confirm
	}

	if flags.ConfirmedDry != nil {
		args[fieldConfirmedDry] = *flags.ConfirmedDry
	}

	if flags.Yolo != nil {
		args[fieldYolo] = *flags.Yolo
	}

	if flags.Mode != "" {
		args[fieldMode] = flags.Mode
	}

	if flags.PlanID != "" {
		args[fieldPlanID] = flags.PlanID
	}

	if flags.Environment != "" {
		args[fieldEnvironment] = flags.Environment
	}

	return args
}

// jsonrpcCallID is the id used for every tools/call the CLI and TUI make.
// Each dispatch is an independent one-shot request through HandleMessage,
// so a single fixed id is correct; the audit log records the tool and
// args, not the id.
const jsonrpcCallID = 1

// buildCallMessage assembles the JSON-RPC tools/call envelope that drives
// the server's HandleMessage. tool is the tool name; args is the (already
// coerced and safety-folded) argument map. The wire shape matches what the
// integration test constructs.
func buildCallMessage(tool string, args map[string]any) (json.RawMessage, error) {
	envelope := map[string]any{
		"jsonrpc": jsonrpcVersionField,
		"id":      jsonrpcCallID,
		"method":  jsonrpcMethodCall,
		"params": map[string]any{
			fieldName:      tool,
			fieldArguments: args,
		},
	}

	raw, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal tools/call request: %w", err)
	}

	return raw, nil
}
