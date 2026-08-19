package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// schemaDocs reads a field's documentation out of the JSON schemas buf already
// generates from the same proto comments. Parsing the proto sources here instead
// would give one argument two descriptions, and only the schema's is the one a
// client sees.
type schemaDocs struct {
	// loaded caches descriptions by input message name, then proto field name.
	loaded map[string]map[string]string
	dir    string
}

// schemaFileSuffix is the per-input-message schema buf writes, the same file
// toolschemas.Schema embeds.
const schemaFileSuffix = ".schema.strict.json"

func newSchemaDocs(dir string) schemaDocs {
	return schemaDocs{dir: dir, loaded: make(map[string]map[string]string)}
}

// describe returns the documentation for one field of one input message, or ""
// when the schema does not describe it. An absent description is not an error:
// the emitter's output is not the advertised schema, which the runtime loads
// from this same file.
func (d schemaDocs) describe(message, fieldName string) string {
	fields, err := d.fields(message)
	if err != nil {
		return ""
	}

	return fields[fieldName]
}

func (d schemaDocs) fields(message string) (map[string]string, error) {
	if cached, found := d.loaded[message]; found {
		return cached, nil
	}

	path := filepath.Join(d.dir, message+schemaFileSuffix)

	raw, err := os.ReadFile(path) // #nosec G304 -- reads a generated schema this repo writes, named by the proto contract
	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", path, err)
	}

	var document struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}

	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("parse schema %s: %w", path, err)
	}

	descriptions := make(map[string]string, len(document.Properties))
	for name, property := range document.Properties {
		descriptions[name] = property.Description
	}

	d.loaded[message] = descriptions

	return descriptions, nil
}
