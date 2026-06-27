// Package toolschemas exposes the MCP input JSON Schemas generated from the
// proto contract. buf writes the schemas into data/ (gitignored) and they are
// embedded here so each tool factory can advertise the proto-derived schema
// without reflecting over descriptors at runtime.
package toolschemas

import (
	"embed"
	"encoding/json"
)

// schemaFS holds the snake_case, flat, strict schema variant for every message.
// The glob deliberately excludes the camelCase ".jsonschema." and ".bundle."
// variants the generator also emits.
//
//go:embed data/*.schema.strict.json
var schemaFS embed.FS

// Schema returns the input JSON Schema for a proto message full name, such as
// "linode.mcp.v1.InstanceGetInput". It returns a minimal object schema when the
// name has no generated schema, which the cross-language parity gate then flags.
func Schema(fullName string) json.RawMessage {
	data, err := schemaFS.ReadFile("data/" + fullName + ".schema.strict.json")
	if err != nil {
		return json.RawMessage(`{"type":"object"}`)
	}

	return data
}
