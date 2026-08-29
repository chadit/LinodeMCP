package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	// argAllowedEnvironments, argRequiredTokenScopes, and argAllowYolo are the
	// draft's own setting names, which the set operation records a change
	// under. Hoisted so one spelling serves the setter and the answer.
	argAllowedEnvironments = "allowed_environments"
	argRequiredTokenScopes = "required_token_scopes"
	argAllowYolo           = "allow_yolo"
)

// stringArrayArg pulls a string-array argument out of the request,
// returning an empty slice when missing or malformed. The MCP wire
// hands arrays through as []any, so we convert per-element.
//
// The request is passed by pointer rather than by value because
// mcp.CallToolRequest is ~80 bytes; gocritic flags the value copy
// for handlers that call this helper repeatedly.
func stringArrayArg(request *mcp.CallToolRequest, key string) []string {
	args := request.GetArguments()

	raw, present := args[key]
	if !present {
		return nil
	}

	asArray, isArray := raw.([]any)
	if !isArray {
		return nil
	}

	out := make([]string, 0, len(asArray))

	for _, entry := range asArray {
		text, isString := entry.(string)
		if !isString {
			continue
		}

		out = append(out, text)
	}

	return out
}
