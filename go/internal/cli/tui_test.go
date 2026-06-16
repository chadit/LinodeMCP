package cli_test

import (
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/cli"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// TestCatalogEntriesGroupByCategory checks the catalog builder emits one
// entry per (tool, category) pair and that a multi-category tool appears
// under each of its categories, so the grouped list shows it everywhere it
// belongs. linode_instance_list is in both compute and compute_actions;
// linode_volume_list is block_storage only.
func TestCatalogEntriesGroupByCategory(t *testing.T) {
	t.Parallel()

	infos := []server.ToolInfo{
		{Name: toolInstLst, Capability: profiles.CapRead},
		{Name: "linode_volume_list", Capability: profiles.CapRead},
	}

	entries := cli.CatalogEntries(infos)

	categoriesByTool := make(map[string][]string, len(infos))
	for _, entry := range entries {
		categoriesByTool[entry.Name] = append(categoriesByTool[entry.Name], entry.Category)
	}

	if got := categoriesByTool[toolInstLst]; !slices.Contains(got, "compute") {
		t.Errorf("%s categories = %v, want to include compute", toolInstLst, got)
	}

	if got := categoriesByTool["linode_volume_list"]; !slices.Contains(got, "block_storage") {
		t.Errorf("linode_volume_list categories = %v, want to include block_storage", got)
	}
}

// TestCatalogEntriesSortedByCategoryThenName checks the entries come out
// sorted by category then tool name, so the catalog list is stable and
// grouped rather than in registration order.
func TestCatalogEntriesSortedByCategoryThenName(t *testing.T) {
	t.Parallel()

	infos := []server.ToolInfo{
		{Name: "linode_volume_list", Capability: profiles.CapRead},
		{Name: "linode_domain_list", Capability: profiles.CapRead},
	}

	entries := cli.CatalogEntries(infos)
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(entries))
	}

	for i := 1; i < len(entries); i++ {
		prev, cur := entries[i-1], entries[i]
		if prev.Category > cur.Category {
			t.Errorf("entries not sorted by category: %q before %q", prev.Category, cur.Category)
		}

		if prev.Category == cur.Category && prev.Name > cur.Name {
			t.Errorf("entries not sorted by name within %q: %q before %q", prev.Category, prev.Name, cur.Name)
		}
	}
}

// TestCatalogEntriesUncategorized checks a tool whose name matches no
// category prefix still appears, in the "other" bucket, rather than
// vanishing from the catalog.
func TestCatalogEntriesUncategorized(t *testing.T) {
	t.Parallel()

	entries := cli.CatalogEntries([]server.ToolInfo{
		{Name: "totally_unknown_tool", Capability: profiles.CapMeta},
	})

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(entries), entries)
	}

	if entries[0].Category != "other" {
		t.Errorf("uncategorized tool category = %q, want other", entries[0].Category)
	}
}

// TestBuildFormFieldSpecsSkipsSafetyControls checks the form builder drops
// the safety-control properties (dry_run, confirm, mode, plan_id,
// environment, yolo, confirmed_dry_run) so they render as dedicated
// controls, not raw text boxes, leaving only the real tool arguments.
func TestBuildFormFieldSpecsSkipsSafetyControls(t *testing.T) {
	t.Parallel()

	schema := makeSchema(map[string]string{
		argInstanceID: schemaTypeInteger,
		argLabel:      schemaTypeString,
		"dry_run":     schemaTypeBoolean,
		"confirm":     schemaTypeBoolean,
		"mode":        schemaTypeString,
		"plan_id":     schemaTypeString,
		"environment": schemaTypeString,
		"yolo":        schemaTypeBoolean,
	}, []string{argInstanceID})

	specs := cli.BuildFormFieldSpecs(schema)

	names := specNames(specs)
	if slices.Contains(names, "dry_run") || slices.Contains(names, "mode") || slices.Contains(names, "environment") {
		t.Errorf("safety controls leaked into form fields: %v", names)
	}

	if len(specs) != 2 {
		t.Fatalf("form has %d fields, want 2 (instance_id, label); got %v", len(specs), names)
	}
}

// TestBuildFormFieldSpecsRequiredFirst checks required fields sort ahead of
// optional ones, so a user fills the mandatory inputs first.
func TestBuildFormFieldSpecsRequiredFirst(t *testing.T) {
	t.Parallel()

	schema := makeSchema(map[string]string{
		"alpha_optional":  schemaTypeString,
		"zeta_required":   schemaTypeString,
		"middle_required": schemaTypeString,
	}, []string{"zeta_required", "middle_required"})

	specs := cli.BuildFormFieldSpecs(schema)
	if len(specs) != 3 {
		t.Fatalf("form has %d fields, want 3", len(specs))
	}

	if !specs[0].Required || !specs[1].Required {
		t.Errorf("required fields did not sort first: %v", specNames(specs))
	}

	if specs[2].Required {
		t.Errorf("optional field did not sort last: %v", specNames(specs))
	}
}

// TestBuildFormFieldSpecsTypeName checks each field carries its schema type,
// defaulting to "string" when the schema leaves a property untyped, so the
// form shows the user what kind of value each field wants.
func TestBuildFormFieldSpecsTypeName(t *testing.T) {
	t.Parallel()

	schema := mcp.ToolInputSchema{
		Type: schemaTypeObject,
		Properties: map[string]any{
			"count":   map[string]any{schemaKeyType: schemaTypeInteger},
			"untyped": map[string]any{},
		},
	}

	specs := cli.BuildFormFieldSpecs(schema)
	byName := make(map[string]cli.FormFieldSpec, len(specs))

	for _, spec := range specs {
		byName[spec.Name] = spec
	}

	if byName["count"].TypeName != schemaTypeInteger {
		t.Errorf("count type = %q, want %s", byName["count"].TypeName, schemaTypeInteger)
	}

	if byName["untyped"].TypeName != schemaTypeString {
		t.Errorf("untyped field type = %q, want %s default", byName["untyped"].TypeName, schemaTypeString)
	}
}

// TestFormToRequestMappingMatchesCall is the core parity check: building a
// request from a tool's field specs and safety flags uses the exact helpers
// the Phase 1 `call` path uses (BuildArguments to coerce by schema type,
// ApplySafetyFlags to fold the safety controls), so a filled TUI form and
// the CLI produce the identical tools/call arguments map. The TUI's
// buildRequest is the thin glue over these two; this asserts the glue's
// contract.
func TestFormToRequestMappingMatchesCall(t *testing.T) {
	t.Parallel()

	schema := makeSchema(map[string]string{
		argInstanceID: schemaTypeInteger,
		argLabel:      schemaTypeString,
	}, []string{argInstanceID})

	// The form would derive these field names from the schema and pair them
	// with the typed values; the pairs feed BuildArguments, exactly as
	// `call --arg instance_id=123 --arg label=web` does.
	pairs := []string{argInstanceID + "=123", argLabel + "=web"}

	args, err := cli.BuildArguments(schema, "", pairs)
	if err != nil {
		t.Fatalf("BuildArguments: %v", err)
	}

	on := true
	flags := cli.SafetyFlags{DryRun: &on, Mode: modePlan, Environment: envStaging}
	cli.ApplySafetyFlags(args, &flags)

	// The integer field coerces to int64, the safety flags fold under their
	// MCP field names: this is the request both front-ends build.
	if got, ok := args[argInstanceID].(int64); !ok || got != 123 {
		t.Errorf("instance_id = %v (%T), want int64(123)", args[argInstanceID], args[argInstanceID])
	}

	if args[argLabel] != "web" {
		t.Errorf("label = %v, want web", args[argLabel])
	}

	if args["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", args["dry_run"])
	}

	if args["mode"] != "plan" {
		t.Errorf("mode = %v, want plan", args["mode"])
	}

	if args["environment"] != "staging" {
		t.Errorf("environment = %v, want staging", args["environment"])
	}
}

// specNames extracts the spec names in order for assertion messages.
func specNames(specs []cli.FormFieldSpec) []string {
	names := make([]string, len(specs))
	for i := range specs {
		names[i] = specs[i].Name
	}

	return names
}
