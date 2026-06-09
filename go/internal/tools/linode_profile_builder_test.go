package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	// argCategory and argCapability are the request argument keys the
	// list_tools handler reads. Hoisted to constants so the goconst
	// linter doesn't flag the repeated literals across filter tests.
	argCategory   = "category"
	argCapability = "capability"
	// missingCategoryName is a deliberately-not-in-the-catalog string
	// for the empty-result filter tests.
	missingCategoryName = "no-such-category"
	// dnsCategory is the category fixture-`linode_domain_get` carries;
	// reused as a filter value across the category-filter tests.
	dnsCategory = "dns"
)

// fixtureCatalog returns a reproducible three-tool catalog used by the
// builder-tool tests. Includes one tool per category surface the
// filters care about (compute write, dns read, core meta) so the
// filter assertions exercise both inclusion and exclusion paths.
func fixtureCatalog() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		{Name: toolInstanceBoot, Capability: profiles.CapWrite},
		{Name: tcLinodeDomainGet, Capability: profiles.CapRead},
		{Name: "hello", Capability: profiles.CapMeta},
	}
}

// callListTools is a thin helper that invokes the list_tools handler
// with the given argument map and returns the parsed JSON entries.
// Cuts the boilerplate the parameterized filter tests would otherwise
// repeat per case.
func callListTools(t *testing.T, args map[string]any) []map[string]any {
	t.Helper()

	_, _, handler := tools.NewLinodeProfileListToolsTool(fixtureCatalog)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	var out []map[string]any

	if err := json.Unmarshal([]byte(textContent.Text), &out); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	return out
}

// TestListToolsRegistration checks the static contract: the tool's
// name, description presence, and CapMeta tag. CapMeta is what makes
// the tool always-available regardless of the active profile; a
// regression on the tag would silently break the builder UX under
// the default (read-only) profile.
func TestListToolsRegistration(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeProfileListToolsTool(fixtureCatalog)

	if tool.Name != "linode_profile_list_tools" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_list_tools")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapMeta {
		t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestListToolsReturnsAllEntriesUnfiltered locks the no-filter path:
// every catalog entry appears in the output with name, capability
// string, and the resolved categories list.
func TestListToolsReturnsAllEntriesUnfiltered(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, nil)

	if len(entries) != 3 {
		t.Errorf("len(entries) = %d, want %d", len(entries), 3)
	}

	got := make(map[string]string, len(entries))
	for _, entry := range entries {
		name, _ := entry["name"].(string)
		capability, _ := entry["capability"].(string)
		got[name] = capability
	}

	for key, want := range map[string]any{
		toolInstanceBoot:  "CapWrite",
		tcLinodeDomainGet: "CapRead",
		"hello":           "CapMeta",
	} {
		if !reflect.DeepEqual(got[key], want) {
			t.Errorf("got[%v] = %v, want %v", key, got[key], want)
		}
	}
}

// TestListToolsCategoriesPopulated verifies the categories field is
// the resolved profiles.Categories() output, not an empty array. The
// model relies on this to drive follow-up `category=` filters.
func TestListToolsCategoriesPopulated(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, nil)

	categories := make(map[string][]any, len(entries))
	for _, entry := range entries {
		name, _ := entry["name"].(string)
		cats, _ := entry["categories"].([]any)
		categories[name] = cats
	}

	if !slices.Contains(categories[toolInstanceBoot], "compute") {
		t.Errorf("collection does not contain %v", "compute")
	}

	if !slices.Contains(categories[tcLinodeDomainGet], dnsCategory) {
		t.Errorf("collection does not contain %v", dnsCategory)
	}

	if !slices.Contains(categories["hello"], "core") {
		t.Errorf("collection does not contain %v", "core")
	}
}

// TestListToolsCategoryFilterMatches restricts the output to one
// category. Verifies the exact-match contract: a tool is included
// only if the filter string appears in its Categories list verbatim.
func TestListToolsCategoryFilterMatches(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, map[string]any{argCategory: dnsCategory})

	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want %d", len(entries), 1)
	}

	if !reflect.DeepEqual(entries[0]["name"], tcLinodeDomainGet) {
		t.Errorf("got %v, want %v", entries[0]["name"], tcLinodeDomainGet)
	}
}

// TestListToolsCategoryFilterRejectsUnknown is the empty-result path:
// an unknown category filter returns zero entries, not a fallback to
// "all entries." A non-empty response would silently mask a user
// typo, which the builder UX cannot afford.
func TestListToolsCategoryFilterRejectsUnknown(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, map[string]any{argCategory: missingCategoryName})

	if len(entries) != 0 {
		t.Errorf("entries = %v, want empty", entries)
	}
}

// TestListToolsCapabilityFilterLongForm checks the CapXxx form of the
// capability filter. Used by callers that round-trip the string from
// a prior list_tools response.
func TestListToolsCapabilityFilterLongForm(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, map[string]any{argCapability: "CapWrite"})

	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want %d", len(entries), 1)
	}

	if !reflect.DeepEqual(entries[0]["name"], toolInstanceBoot) {
		t.Errorf("got %v, want %v", entries[0]["name"], toolInstanceBoot)
	}
}

// TestListToolsCapabilityFilterShortForm checks the short form
// ("write") of the capability filter. Equivalent to the long-form
// path; the model typically picks the form that reads more naturally
// in its natural-language reasoning.
func TestListToolsCapabilityFilterShortForm(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, map[string]any{argCapability: "write"})

	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want %d", len(entries), 1)
	}

	if !reflect.DeepEqual(entries[0]["name"], toolInstanceBoot) {
		t.Errorf("got %v, want %v", entries[0]["name"], toolInstanceBoot)
	}
}

// TestListToolsCapabilityFilterCaseInsensitive confirms case folding
// applies to both forms. Spelled-as-typed input ("WRITE", "Read")
// must work, otherwise the model has to remember the exact casing.
func TestListToolsCapabilityFilterCaseInsensitive(t *testing.T) {
	t.Parallel()

	upper := callListTools(t, map[string]any{argCapability: "WRITE"})
	if len(upper) != 1 {
		t.Errorf("len(upper) = %d, want %d", len(upper), 1)
	}

	if !reflect.DeepEqual(upper[0]["name"], toolInstanceBoot) {
		t.Errorf("got %v, want %v", upper[0]["name"], toolInstanceBoot)
	}

	mixed := callListTools(t, map[string]any{argCapability: "Read"})
	if len(mixed) != 1 {
		t.Errorf("len(mixed) = %d, want %d", len(mixed), 1)
	}

	if !reflect.DeepEqual(mixed[0]["name"], tcLinodeDomainGet) {
		t.Errorf("got %v, want %v", mixed[0]["name"], tcLinodeDomainGet)
	}
}

// TestListToolsCombinedFilters runs category and capability together.
// Lock the AND semantics: a tool must match both filters to appear.
func TestListToolsCombinedFilters(t *testing.T) {
	t.Parallel()

	// dns + Read matches linode_domain_get.
	matchEntries := callListTools(t, map[string]any{
		argCategory:   dnsCategory,
		argCapability: "read",
	})
	if len(matchEntries) != 1 {
		t.Errorf("len(matchEntries) = %d, want %d", len(matchEntries), 1)
	}

	if !reflect.DeepEqual(matchEntries[0]["name"], tcLinodeDomainGet) {
		t.Errorf("got %v, want %v", matchEntries[0]["name"], tcLinodeDomainGet)
	}

	// dns + Write matches nothing (no DNS write tools in the fixture).
	missEntries := callListTools(t, map[string]any{
		argCategory:   dnsCategory,
		argCapability: "write",
	})
	if len(missEntries) != 0 {
		t.Errorf("missEntries = %v, want empty", missEntries)
	}
}

// TestListToolsEmptyCatalogReturnsEmptyArray locks the JSON shape on
// the empty path: “[]“ not “null“. The model would handle null as
// "tool failed"; an empty array is the correct "no tools matched".
func TestListToolsEmptyCatalogReturnsEmptyArray(t *testing.T) {
	t.Parallel()

	emptyProvider := func() []profiles.ToolDescriptor { return nil }
	_, _, handler := tools.NewLinodeProfileListToolsTool(emptyProvider)

	result, err := handler(t.Context(), mcp.CallToolRequest{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if textContent.Text != databaseJSONArray {
		t.Errorf("textContent.Text = %v, want %v", textContent.Text, databaseJSONArray)
	}
}

// TestListToolsRespectsContextCancellation locks the cancellation
// contract: a canceled context surfaces ctx.Err and produces no
// result. Standard MCP handler hygiene; the test exists to catch a
// future refactor that drops the select gate.
func TestListToolsRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeProfileListToolsTool(fixtureCatalog)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := handler(ctx, mcp.CallToolRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

// TestListCategoriesRegistration mirrors the registration contract
// check for the categories tool.
func TestListCategoriesRegistration(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeProfileListCategoriesTool(fixtureCatalog)

	if tool.Name != "linode_profile_list_categories" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_list_categories")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapMeta {
		t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestListCategoriesReturnsDeduplicatedCounts is the substantive
// behavior test: every category referenced by some tool appears
// exactly once, with a tool_count equal to the number of tools that
// carry it. A regression where duplicates leak through (e.g. a typo
// in the dedup map) would surface here.
func TestListCategoriesReturnsDeduplicatedCounts(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeProfileListCategoriesTool(fixtureCatalog)

	result, err := handler(t.Context(), mcp.CallToolRequest{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &entries); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// JSON unmarshal yields float64 for numbers; convert to int so
	// the comparison is exact rather than approximate.
	counts := make(map[string]int, len(entries))
	for _, entry := range entries {
		name, _ := entry["name"].(string)
		raw, _ := entry["tool_count"].(float64)
		counts[name] = int(raw)
	}

	// linode_instance_boot carries both compute and compute_actions.
	// linode_domain_get carries dns. hello carries core. Each tool
	// contributes 1 to each of its categories.
	for key, want := range map[string]any{
		"compute":         1,
		"compute_actions": 1,
		dnsCategory:       1,
		"core":            1,
	} {
		if !reflect.DeepEqual(counts[key], want) {
			t.Errorf("counts[%v] = %v, want %v", key, counts[key], want)
		}
	}
}

// TestListCategoriesSortedByName locks the reproducible-output
// contract. JSON map iteration order is non-reproducible, so the
// handler explicitly sorts. A refactor that drops the sort would
// cause the cross-language parity test to flake.
func TestListCategoriesSortedByName(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeProfileListCategoriesTool(fixtureCatalog)

	result, err := handler(t.Context(), mcp.CallToolRequest{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	textContent, _ := result.Content[0].(mcp.TextContent)

	var entries []map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &entries); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i], _ = entry["name"].(string)
	}

	for nameIndex := 1; nameIndex < len(names); nameIndex++ {
		if names[nameIndex-1] > names[nameIndex] {
			t.Errorf(
				"categories must come back in sorted order: values must increase at index %d: %q > %q",
				nameIndex,
				names[nameIndex-1],
				names[nameIndex],
			)
		}
	}
}

// TestListCategoriesEmptyCatalogReturnsEmptyArray mirrors the
// list_tools empty-array test for the categories tool.
func TestListCategoriesEmptyCatalogReturnsEmptyArray(t *testing.T) {
	t.Parallel()

	emptyProvider := func() []profiles.ToolDescriptor { return nil }
	_, _, handler := tools.NewLinodeProfileListCategoriesTool(emptyProvider)

	result, err := handler(t.Context(), mcp.CallToolRequest{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	textContent, _ := result.Content[0].(mcp.TextContent)
	if textContent.Text != databaseJSONArray {
		t.Errorf("textContent.Text = %v, want %v", textContent.Text, databaseJSONArray)
	}
}
