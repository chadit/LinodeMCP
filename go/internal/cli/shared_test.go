package cli_test

import "github.com/mark3labs/mcp-go/mcp"

// Shared test literals. goconst counts string occurrences across the whole
// cli_test package, so the JSON-schema type names and the tool/arg names
// that recur across the test files live here as constants.
const (
	schemaTypeObject  = "object"
	schemaTypeInteger = "integer"
	schemaTypeBoolean = "boolean"
	schemaTypeString  = "string"
	schemaKeyType     = "type"

	argLabel      = "label"
	argInstanceID = "instance_id"

	modePlan    = "plan"
	envStaging  = "staging"
	toolInstLst = "linode_instance_list"

	// testVersion is a fixed version string used across the TUI extras
	// tests so the version block can be asserted without depending on the
	// real build version.
	testVersion = "9.9.9"
)

// makeSchema builds a ToolInputSchema whose properties carry the given
// JSON-schema types and whose Required lists the given names, mirroring the
// shape mcp-go produces for a tool. Shared by the request and TUI tests so
// the schema shape is defined once.
func makeSchema(types map[string]string, required []string) mcp.ToolInputSchema {
	props := make(map[string]any, len(types))
	for name, typ := range types {
		props[name] = map[string]any{schemaKeyType: typ}
	}

	return mcp.ToolInputSchema{Type: schemaTypeObject, Properties: props, Required: required}
}
