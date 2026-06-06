package tools_test

import (
	"context"
	"encoding/json"
	"errors"
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

// fixtureCatalog returns a deterministic three-tool catalog used by the
// builder-tool tests. Includes one tool per category surface the
// filters care about (compute write, dns read, core meta) so the
// filter assertions exercise both inclusion and exclusion paths.
func fixtureCatalog() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		{Name: "linode_instance_boot", Capability: profiles.CapWrite},
		{Name: "linode_domain_get", Capability: profiles.CapRead},
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
	expectNoError(t, err)
	expectNotNil(t, result)

	textContent, ok := result.Content[0].(mcp.TextContent)
	expectTrue(t, ok, "result content must be TextContent")

	var out []map[string]any

	expectNoError(t, json.Unmarshal([]byte(textContent.Text), &out))

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

	checkEqual(t, "linode_profile_list_tools", tool.Name)
	expectNotEmpty(t, tool.Description)
	checkEqual(t, profiles.CapMeta, capability)
	expectNotNil(t, handler)
}

// TestListToolsReturnsAllEntriesUnfiltered locks the no-filter path:
// every catalog entry appears in the output with name, capability
// string, and the resolved categories list.
func TestListToolsReturnsAllEntriesUnfiltered(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, nil)

	expectLen(t, entries, 3)

	got := make(map[string]string, len(entries))
	for _, entry := range entries {
		name, _ := entry["name"].(string)
		capability, _ := entry["capability"].(string)
		got[name] = capability
	}

	checkEqual(t, "CapWrite", got["linode_instance_boot"])
	checkEqual(t, "CapRead", got["linode_domain_get"])
	checkEqual(t, "CapMeta", got["hello"])
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

	expectContainsWithMode(t, false, categories["linode_instance_boot"], "compute", "linode_instance_boot must carry the compute category")
	expectContainsWithMode(t, false, categories["linode_domain_get"], dnsCategory, "linode_domain_get must carry the dns category")
	expectContainsWithMode(t, false, categories["hello"], "core", "hello must carry the core category")
}

// TestListToolsCategoryFilterMatches restricts the output to one
// category. Verifies the exact-match contract: a tool is included
// only if the filter string appears in its Categories list verbatim.
func TestListToolsCategoryFilterMatches(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, map[string]any{argCategory: dnsCategory})

	expectLen(t, entries, 1, "only linode_domain_get carries the dns category")
	checkEqual(t, "linode_domain_get", entries[0]["name"])
}

// TestListToolsCategoryFilterRejectsUnknown is the empty-result path:
// an unknown category filter returns zero entries, not a fallback to
// "all entries." A non-empty response would silently mask a user
// typo, which the builder UX cannot afford.
func TestListToolsCategoryFilterRejectsUnknown(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, map[string]any{argCategory: missingCategoryName})

	checkEmpty(t, entries)
}

// TestListToolsCapabilityFilterLongForm checks the CapXxx form of the
// capability filter. Used by callers that round-trip the string from
// a prior list_tools response.
func TestListToolsCapabilityFilterLongForm(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, map[string]any{argCapability: "CapWrite"})

	expectLen(t, entries, 1)
	checkEqual(t, "linode_instance_boot", entries[0]["name"])
}

// TestListToolsCapabilityFilterShortForm checks the short form
// ("write") of the capability filter. Equivalent to the long-form
// path; the model typically picks the form that reads more naturally
// in its natural-language reasoning.
func TestListToolsCapabilityFilterShortForm(t *testing.T) {
	t.Parallel()

	entries := callListTools(t, map[string]any{argCapability: "write"})

	expectLen(t, entries, 1)
	checkEqual(t, "linode_instance_boot", entries[0]["name"])
}

// TestListToolsCapabilityFilterCaseInsensitive confirms case folding
// applies to both forms. Spelled-as-typed input ("WRITE", "Read")
// must work, otherwise the model has to remember the exact casing.
func TestListToolsCapabilityFilterCaseInsensitive(t *testing.T) {
	t.Parallel()

	upper := callListTools(t, map[string]any{argCapability: "WRITE"})
	expectLen(t, upper, 1)
	checkEqual(t, "linode_instance_boot", upper[0]["name"])

	mixed := callListTools(t, map[string]any{argCapability: "Read"})
	expectLen(t, mixed, 1)
	checkEqual(t, "linode_domain_get", mixed[0]["name"])
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
	expectLen(t, matchEntries, 1)
	checkEqual(t, "linode_domain_get", matchEntries[0]["name"])

	// dns + Write matches nothing (no DNS write tools in the fixture).
	missEntries := callListTools(t, map[string]any{
		argCategory:   dnsCategory,
		argCapability: "write",
	})
	checkEmpty(t, missEntries)
}

// TestListToolsEmptyCatalogReturnsEmptyArray locks the JSON shape on
// the empty path: “[]“ not “null“. The model would handle null as
// "tool failed"; an empty array is the correct "no tools matched".
func TestListToolsEmptyCatalogReturnsEmptyArray(t *testing.T) {
	t.Parallel()

	emptyProvider := func() []profiles.ToolDescriptor { return nil }
	_, _, handler := tools.NewLinodeProfileListToolsTool(emptyProvider)

	result, err := handler(t.Context(), mcp.CallToolRequest{})
	expectNoError(t, err)

	textContent, ok := result.Content[0].(mcp.TextContent)
	expectTrue(t, ok)
	checkEqual(t, "[]", textContent.Text, "empty catalog must serialize as [], not null")
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

	expectNil(t, result)
}

// TestListCategoriesRegistration mirrors the registration contract
// check for the categories tool.
func TestListCategoriesRegistration(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeProfileListCategoriesTool(fixtureCatalog)

	checkEqual(t, "linode_profile_list_categories", tool.Name)
	expectNotEmpty(t, tool.Description)
	checkEqual(t, profiles.CapMeta, capability)
	expectNotNil(t, handler)
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
	expectNoError(t, err)

	textContent, ok := result.Content[0].(mcp.TextContent)
	expectTrue(t, ok)

	var entries []map[string]any
	expectNoError(t, json.Unmarshal([]byte(textContent.Text), &entries))

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
	checkEqual(t, 1, counts["compute"], "compute count should be 1")
	checkEqual(t, 1, counts["compute_actions"], "compute_actions count should be 1")
	checkEqual(t, 1, counts[dnsCategory], "dns count should be 1")
	checkEqual(t, 1, counts["core"], "core count should be 1")
}

// TestListCategoriesSortedByName locks the deterministic-output
// contract. JSON map iteration order is non-deterministic, so the
// handler explicitly sorts. A refactor that drops the sort would
// cause the cross-language parity test to flake.
func TestListCategoriesSortedByName(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeProfileListCategoriesTool(fixtureCatalog)

	result, err := handler(t.Context(), mcp.CallToolRequest{})
	expectNoError(t, err)

	textContent, _ := result.Content[0].(mcp.TextContent)

	var entries []map[string]any
	expectNoError(t, json.Unmarshal([]byte(textContent.Text), &entries))

	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i], _ = entry["name"].(string)
	}

	for nameIndex := 1; nameIndex < len(names); nameIndex++ {
		if names[nameIndex-1] > names[nameIndex] {
			t.Errorf(
				"expected values to be increasing at index %d: %q > %q%s",
				nameIndex,
				names[nameIndex-1],
				names[nameIndex],
				expectationMessage([]string{"categories must come back in sorted order"}),
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
	expectNoError(t, err)

	textContent, _ := result.Content[0].(mcp.TextContent)
	checkEqual(t, "[]", textContent.Text, "empty catalog must serialize as [], not null")
}
