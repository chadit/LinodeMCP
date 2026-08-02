package server_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/server"
)

// placeholderSchema is what toolschemas.Schema hands back for a proto message
// name the contract does not define. A tool advertising it validates nothing.
const placeholderSchema = `{"type":"object"}`

// generatedSchema stands in for a real proto-derived schema. Any shape other
// than the placeholder works; this one mirrors the generated files, which all
// carry $schema, type, and properties.
const generatedSchema = `{"$schema":"https://json-schema.org/draft/2020-12/schema",` +
	`"type":"object","properties":{"environment":{"type":"string"}}}`

// TestValidateGeneratedSchemasRejectsPlaceholder is the reason the check exists.
// A tool that reached the placeholder would accept any arguments at all, so the
// server has to refuse it rather than serve it.
//
// The catalog New builds cannot contain one, since the factories embed their
// schemas at compile time, so the tool is constructed here directly. The good
// tool comes first to prove the scan does not stop at index zero.
func TestValidateGeneratedSchemasRejectsPlaceholder(t *testing.T) {
	t.Parallel()

	list := []mcp.Tool{
		{Name: "linode_instance_get", RawInputSchema: json.RawMessage(generatedSchema)},
		{Name: "linode_broken_tool", RawInputSchema: json.RawMessage(placeholderSchema)},
	}

	if err := server.ValidateGeneratedSchemas(list); !errors.Is(err, server.ErrSchemaNotGenerated) {
		t.Fatalf("ValidateGeneratedSchemas() error = %v, want ErrSchemaNotGenerated", err)
	}
}

// TestValidateGeneratedSchemasAcceptsGenerated is the other half: the check is
// only safe to run at startup if a real catalog passes it. A false positive here
// would stop the server from starting at all.
func TestValidateGeneratedSchemasAcceptsGenerated(t *testing.T) {
	t.Parallel()

	list := []mcp.Tool{
		{Name: "linode_instance_get", RawInputSchema: json.RawMessage(generatedSchema)},
		{Name: "linode_instance_list", RawInputSchema: json.RawMessage(generatedSchema)},
	}

	if err := server.ValidateGeneratedSchemas(list); err != nil {
		t.Errorf("ValidateGeneratedSchemas() = %v, want nil", err)
	}
}

// TestValidateGeneratedSchemasAcceptsEmptyCatalog pins the degenerate input:
// nothing to check is not a failure. A profile can filter every tool out.
func TestValidateGeneratedSchemasAcceptsEmptyCatalog(t *testing.T) {
	t.Parallel()

	if err := server.ValidateGeneratedSchemas(nil); err != nil {
		t.Errorf("ValidateGeneratedSchemas(nil) = %v, want nil", err)
	}
}

// TestNewAcceptsTheRealCatalog is the integration side. Every tool a real server
// registers has to carry a proto-derived schema, so New running the check over
// the full catalog must still succeed. This is what breaks if a factory is
// pointed at a proto message name the contract does not define.
func TestNewAcceptsTheRealCatalog(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(srv.AllToolInfos()) == 0 {
		t.Fatal("len(AllToolInfos()) = 0, want the full catalog")
	}
}
