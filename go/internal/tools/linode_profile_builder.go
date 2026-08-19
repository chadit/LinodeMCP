package tools

import (
	"context"
	"slices"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// ProfileListToolsAnswer answers linode_profile_list_tools. It enumerates
// every tool the server could register, independent of the active profile's
// filter, so the model can plan profile composition against the full menu.
// Optional filters trim the output:
//
//   - "category": include only tools that carry the named category
//   - "capability": include only tools tagged with the named capability
//     (one of CapRead, CapWrite, CapDestroy, CapAdmin, CapMeta)
//
// Both filters use exact match. "capability" checks are case-insensitive
// against the Capability String() form (the "Cap" prefix is optional, so
// "read" or "CapRead" both work).
//
// The answer is name-sorted. The catalog itself is in registration order,
// which differs per language, so sorting here is what makes one tool answer
// one order whichever binary served it.
func ProfileListToolsAnswer(
	ctx context.Context, request *mcp.CallToolRequest, _ *config.Config,
) (*mcp.CallToolResult, error) {
	state, refusal := builderStateOrRefusal(ctx)
	if refusal != nil {
		return refusal, nil
	}

	categoryFilter := request.GetString("category", "")
	capabilityFilter := request.GetString("capability", "")

	entries := state.Catalog()
	out := make([]*linodev1.ProfileToolCatalogItem, 0, len(entries))

	for idx := range entries {
		cats := profiles.Categories(entries[idx].Name)
		if categoryFilter != "" && !slices.Contains(cats, categoryFilter) {
			continue
		}

		if capabilityFilter != "" && !capabilityMatches(entries[idx].Capability, capabilityFilter) {
			continue
		}

		out = append(out, &linodev1.ProfileToolCatalogItem{
			Name:       entries[idx].Name,
			Capability: entries[idx].Capability.String(),
			Categories: cats,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })

	return MarshalProtoToolResponse(&linodev1.ProfileToolListResponse{
		Count: IDToInt32(len(out)),
		Tools: out,
	})
}

// ProfileListCategoriesAnswer answers linode_profile_list_categories. It
// reduces the catalog to the deduplicated category list with tool counts,
// sorted by name, so the model can discover what categories exist before
// drilling in with list_tools(category=...).
func ProfileListCategoriesAnswer(
	ctx context.Context, _ *mcp.CallToolRequest, _ *config.Config,
) (*mcp.CallToolResult, error) {
	state, refusal := builderStateOrRefusal(ctx)
	if refusal != nil {
		return refusal, nil
	}

	entries := state.Catalog()
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

	out := make([]*linodev1.ProfileCategoryItem, len(names))
	for i, name := range names {
		out[i] = &linodev1.ProfileCategoryItem{Name: name, ToolCount: IDToInt32(counts[name])}
	}

	return MarshalProtoToolResponse(&linodev1.ProfileCategoryListResponse{
		Count:      IDToInt32(len(out)),
		Categories: out,
	})
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
