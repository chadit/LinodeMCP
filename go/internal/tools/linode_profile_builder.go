package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// CatalogProvider returns the full server tool catalog. The Phase 8.2
// builder tools take this as an injected dependency rather than a
// global so test code can supply a reproducible fixture. The provider
// runs at handler call time so hot-reload changes to the catalog are
// reflected without re-registering the tool.
type CatalogProvider func() []profiles.ToolDescriptor

// toolCatalogJSONEntry is the wire shape for one row of the
// linode_profile_list_tools response. Lowercase JSON tags match the
// Python implementation; the model reads this structure to drive
// follow-up builder operations.
type toolCatalogJSONEntry struct {
	Name       string   `json:"name"`
	Capability string   `json:"capability"`
	Categories []string `json:"categories"`
}

// categoryJSONEntry is the wire shape for one row of the
// linode_profile_list_categories response. ToolCount is the number of
// tools the category covers across the full catalog (not filtered by
// capability).
type categoryJSONEntry struct {
	Name      string `json:"name"`
	ToolCount int    `json:"tool_count"`
}

// NewLinodeProfileListToolsTool returns the linode_profile_list_tools
// builder tool. It enumerates every tool the server could register,
// independent of the active profile's filter, so the model can plan
// profile composition against the full menu. Optional filters trim
// the output:
//
//   - "category": include only tools that carry the named category
//   - "capability": include only tools tagged with the named capability
//     (one of CapRead, CapWrite, CapDestroy, CapAdmin, CapMeta)
//
// Both filters use exact match. "capability" checks are
// case-insensitive against the Capability String() form (the "Cap"
// prefix is optional, so "read" or "CapRead" both work).
func NewLinodeProfileListToolsTool(
	provider CatalogProvider,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_profile_list_tools",
		mcp.WithDescription(
			"List every registerable tool with its capability tag and categories. "+
				"Used by the profile builder to enumerate the full menu before "+
				"composing a user-defined profile. Optional filters: category, capability.",
		),
		mcp.WithString(
			"category",
			mcp.Description(`Filter to tools whose Categories include this exact name (e.g. "compute", "dns").`),
		),
		mcp.WithString(
			"capability",
			mcp.Description("Filter to tools with this capability. Accepts the short form (read, write, destroy, admin, meta) or the long form (CapRead, CapWrite, ...)."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		categoryFilter := request.GetString("category", "")
		capabilityFilter := request.GetString("capability", "")

		entries := provider()
		out := make([]toolCatalogJSONEntry, 0, len(entries))

		for idx := range entries {
			cats := profiles.Categories(entries[idx].Name)
			if categoryFilter != "" && !slices.Contains(cats, categoryFilter) {
				continue
			}

			if capabilityFilter != "" && !capabilityMatches(entries[idx].Capability, capabilityFilter) {
				continue
			}

			out = append(out, toolCatalogJSONEntry{
				Name:       entries[idx].Name,
				Capability: entries[idx].Capability.String(),
				Categories: cats,
			})
		}

		body, err := json.Marshal(out)
		if err != nil {
			return nil, fmt.Errorf("marshal tool catalog: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
}

// NewLinodeProfileListCategoriesTool returns the
// linode_profile_list_categories builder tool. It reduces the catalog
// to the deduplicated category list with tool counts, sorted by name.
// Used to discover what categories exist before drilling in with
// list_tools(category=...).
func NewLinodeProfileListCategoriesTool(
	provider CatalogProvider,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_profile_list_categories",
		mcp.WithDescription(
			"List tool categories with the number of tools each covers. "+
				"Used by the profile builder to discover available categories "+
				"before drilling into a category with linode_profile_list_tools.",
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_ = request

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		entries := provider()
		counts := make(map[string]int, 16)

		for idx := range entries {
			for _, cat := range profiles.Categories(entries[idx].Name) {
				counts[cat]++
			}
		}

		names := make([]string, 0, len(counts))
		for name := range counts {
			names = append(names, name)
		}

		sort.Strings(names)

		out := make([]categoryJSONEntry, len(names))
		for i, name := range names {
			out[i] = categoryJSONEntry{Name: name, ToolCount: counts[name]}
		}

		body, err := json.Marshal(out)
		if err != nil {
			return nil, fmt.Errorf("marshal category list: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
}

// capabilityMatches reports whether the given capability tag matches
// the filter string. Accepts both short ("read") and long ("CapRead")
// forms, case-insensitive, so the model can use either without an
// exact-string failure.
func capabilityMatches(capability profiles.Capability, filter string) bool {
	long := capability.String()
	if strings.EqualFold(long, filter) {
		return true
	}

	// "Cap" prefix is consistent across all Capability stringers, so
	// trimming it gives the short form. CapRead -> Read.
	const capPrefix = "Cap"
	if short, ok := strings.CutPrefix(long, capPrefix); ok && strings.EqualFold(short, filter) {
		return true
	}

	return false
}
