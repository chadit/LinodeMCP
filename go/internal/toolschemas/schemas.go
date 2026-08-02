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

// fallbackSchema is what Schema returns for a name the contract does not
// define. It accepts any object, so a tool advertising it validates nothing.
const fallbackSchema = `{"type":"object"}`

// IsFallback reports whether a schema is the permissive placeholder Schema
// returns on a miss, rather than a generated one.
//
// server.New rejects any tool whose schema satisfies this, which is what turns a
// miss into a startup failure instead of a tool that accepts anything. Nothing
// in the generated set collides: no generated schema equals these bytes, and
// none carries fewer than three top-level keys, since every one emits $id,
// $schema, and type.
func IsFallback(schema json.RawMessage) bool {
	return string(schema) == fallbackSchema
}

// Schema returns the input JSON Schema for a proto message full name, such as
// "linode.mcp.v1.InstanceGetInput". It returns the permissive placeholder
// IsFallback recognizes when the name has no generated schema.
//
// A miss is a build defect, not a runtime condition: buf generates a schema for
// every input message in the proto contract and the whole data/ directory is
// embedded at compile time, so reaching the fallback means asking for a name the
// contract does not define. Callers run while the server assembles its tool
// catalog (the tool factories, invoked once from server.New), never while
// serving a request, so server.New catches the fallback before the server
// starts. That matches the Python twin, linodemcp.tools.toolschemas.schema,
// which raises FileNotFoundError on the same input: both abort startup rather
// than register a tool that accepts anything.
//
// scripts/verify_input_proto.py does not catch a miss. It statically classifies
// whether a factory calls this function at all versus hand-building a schema
// from mcp.With* options; it never executes a factory and never inspects the
// bytes returned here.
func Schema(fullName string) json.RawMessage {
	data, err := schemaFS.ReadFile("data/" + fullName + ".schema.strict.json")
	if err != nil {
		return json.RawMessage(fallbackSchema)
	}

	return data
}
