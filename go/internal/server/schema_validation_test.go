package server_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// placeholderSchema is what toolschemas.Schema hands back for a proto message
// name the contract does not define. A tool advertising it validates nothing.
const placeholderSchema = `{"type":"object"}`

// routedTool is a name the proto contract declares, so the route half of the
// catalog check has something real to match.
const routedTool = "linode_instance_get"

// generatedSchema stands in for a proto-derived schema. Any shape other than
// the placeholder works; this one mirrors the generated files.
const generatedSchema = `{"$schema":"https://json-schema.org/draft/2020-12/schema",` +
	`"type":"object","properties":{"environment":{"type":"string"}}}`

// A tool left on the placeholder would accept any arguments at all. The catalog
// New builds cannot contain one, since factories embed their schemas at compile
// time, so the tools are hand-built here. The good tool comes first to prove the
// scan does not stop at index zero.
func TestValidateGeneratedSchemasRejectsPlaceholder(t *testing.T) {
	t.Parallel()

	list := []mcp.Tool{
		{Name: routedTool, RawInputSchema: json.RawMessage(generatedSchema)},
		{Name: "linode_broken_tool", RawInputSchema: json.RawMessage(placeholderSchema)},
	}

	if err := server.ValidateGeneratedSchemas(list); !errors.Is(err, server.ErrSchemaNotGenerated) {
		t.Fatalf("ValidateGeneratedSchemas() error = %v, want ErrSchemaNotGenerated", err)
	}
}

// The check runs at startup, so a false positive here would stop the server
// from starting at all.
func TestValidateGeneratedSchemasAcceptsGenerated(t *testing.T) {
	t.Parallel()

	list := []mcp.Tool{
		{Name: routedTool, RawInputSchema: json.RawMessage(generatedSchema)},
		{Name: "linode_instance_list", RawInputSchema: json.RawMessage(generatedSchema)},
	}

	if err := server.ValidateGeneratedSchemas(list); err != nil {
		t.Errorf("ValidateGeneratedSchemas() = %v, want nil", err)
	}
}

// Nothing to check is not a failure: a profile can filter every tool out.
func TestValidateGeneratedSchemasAcceptsEmptyCatalog(t *testing.T) {
	t.Parallel()

	if err := server.ValidateGeneratedSchemas(nil); err != nil {
		t.Errorf("ValidateGeneratedSchemas(nil) = %v, want nil", err)
	}
}

// One staged tool leaves every other declared tool unstaged, so the server
// would serve a smaller surface than the contract describes.
func TestValidateCatalogRejectsACatalogTheContractDoesNotDescribe(t *testing.T) {
	t.Parallel()

	list := []mcp.Tool{
		{Name: routedTool, RawInputSchema: json.RawMessage(generatedSchema)},
	}

	err := server.ValidateCatalog(list)
	if !errors.Is(err, linoderoute.ErrRegistered) {
		t.Fatalf("ValidateCatalog() error = %v, want ErrRegistered", err)
	}
}

// A handler staged under an undeclared name has no tier to filter it by, so a
// profile could not keep it out of a read-only session.
func TestValidateCatalogRejectsAToolTheContractNeverNamed(t *testing.T) {
	t.Parallel()

	list := append(declaredCatalog(t), mcp.Tool{
		Name: "linode_undeclared_tool", RawInputSchema: json.RawMessage(generatedSchema),
	})

	err := server.ValidateCatalog(list)
	if !errors.Is(err, linoderoute.ErrRegistered) {
		t.Fatalf("ValidateCatalog() error = %v, want ErrRegistered", err)
	}
}

// A catalog failing both checks reports the schema, the one saying the tool
// would accept any arguments at all.
func TestValidateCatalogRunsTheSchemaCheckFirst(t *testing.T) {
	t.Parallel()

	list := []mcp.Tool{
		{Name: "linode_broken_tool", RawInputSchema: json.RawMessage(placeholderSchema)},
	}

	err := server.ValidateCatalog(list)
	if !errors.Is(err, server.ErrSchemaNotGenerated) {
		t.Fatalf("ValidateCatalog() error = %v, want ErrSchemaNotGenerated", err)
	}
}

// Staging exactly what the contract declares has to pass, or a correct server
// never starts.
func TestValidateCatalogAcceptsTheDeclaredCatalog(t *testing.T) {
	t.Parallel()

	if err := server.ValidateCatalog(declaredCatalog(t)); err != nil {
		t.Errorf("ValidateCatalog() = %v, want nil", err)
	}
}

// declaredCatalog builds the catalog a correct server stages: one tool per name
// the proto contract declares, each advertising a generated schema.
func declaredCatalog(t *testing.T) []mcp.Tool {
	t.Helper()

	declared := linoderoute.Tools()
	if len(declared) == 0 {
		t.Fatal("len(linoderoute.Tools()) = 0, want the contract to declare tools")
	}

	list := make([]mcp.Tool, 0, len(declared))
	for _, tool := range declared {
		list = append(list, mcp.Tool{
			Name: tool.Name, RawInputSchema: json.RawMessage(generatedSchema),
		})
	}

	return list
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
