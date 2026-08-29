package main

import (
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The tool-to-category half of the contract. Every tool input, meta included,
// declares the profile categories it belongs to (or an explicit none), and the
// string spelling of each enum member lives here alone. Declaration order is
// preserved into the registries: the catalog TUI groups a tool under its
// first category.

// categoryStrings is the string spelling of each declarable category member.
func categoryStrings() map[linodev1.ToolCategory]string {
	return map[linodev1.ToolCategory]string{
		linodev1.ToolCategory_TOOL_CATEGORY_ACCOUNT:        "account",
		linodev1.ToolCategory_TOOL_CATEGORY_BLOCK_STORAGE:  "block_storage",
		linodev1.ToolCategory_TOOL_CATEGORY_COMPUTE:        "compute",
		linodev1.ToolCategory_TOOL_CATEGORY_COMPUTE_DEEP:   "compute_deep",
		linodev1.ToolCategory_TOOL_CATEGORY_CORE:           "core",
		linodev1.ToolCategory_TOOL_CATEGORY_DATABASES:      "databases",
		linodev1.ToolCategory_TOOL_CATEGORY_DNS:            "dns",
		linodev1.ToolCategory_TOOL_CATEGORY_IAM:            "iam",
		linodev1.ToolCategory_TOOL_CATEGORY_LKE:            "lke",
		linodev1.ToolCategory_TOOL_CATEGORY_LONGVIEW:       "longview",
		linodev1.ToolCategory_TOOL_CATEGORY_MONITOR:        "monitor",
		linodev1.ToolCategory_TOOL_CATEGORY_NETWORKING:     "networking",
		linodev1.ToolCategory_TOOL_CATEGORY_OBJECT_STORAGE: "object_storage",
		linodev1.ToolCategory_TOOL_CATEGORY_SECURITY:       "security",
		linodev1.ToolCategory_TOOL_CATEGORY_VPCS:           "vpcs",
	}
}

// readCategories resolves the declared tool_categories into the strings the
// registries emit. Unlike readScopes it accepts no meta exemption: two core
// tools are meta, and a mutator declaring nothing would be served by no
// category-scoped profile in silence.
func (c *contract) readCategories(options protoreflect.ProtoMessage) error {
	declared, found := categoriesOption(options)

	if !found || (!declared.GetNone() && len(declared.GetCategory()) == 0) {
		return fmt.Errorf("%w: %s", errNoCategories, c.Name)
	}

	if declared.GetNone() && len(declared.GetCategory()) > 0 {
		return fmt.Errorf("%w: %s", errCategoriesNoneBesideList, c.Name)
	}

	return c.resolveCategoryMembers(declared.GetCategory())
}

// resolveCategoryMembers turns the declared members into strings, refusing a
// repeat and a member with no spelling to render.
func (c *contract) resolveCategoryMembers(members []linodev1.ToolCategory) error {
	spellings := categoryStrings()
	seen := make(map[string]bool, len(members))

	for _, member := range members {
		text, mapped := spellings[member]
		if !mapped {
			return fmt.Errorf("%w: %s declares %s", errCategoryUnrenderable, c.Name, member)
		}

		if seen[text] {
			return fmt.Errorf("%w: %s declares %s twice", errCategoryRepeated, c.Name, text)
		}

		seen[text] = true
		c.Categories = append(c.Categories, text)
	}

	return nil
}

// categoriesOption reads the declared tool_categories, reporting whether the
// message declares one at all: presence is what the completeness refusal reads.
func categoriesOption(options protoreflect.ProtoMessage) (*linodev1.ToolCategories, bool) {
	if !proto.HasExtension(options, linodev1.E_ToolCategories) {
		return nil, false
	}

	declared, ok := proto.GetExtension(options, linodev1.E_ToolCategories).(*linodev1.ToolCategories)

	return declared, ok
}

// categorizedContracts is the cohort's category-declaring tools, tool-name
// sorted so both registries render the table in one order. Tools declaring
// none are left out: an absent key already answers empty.
func categorizedContracts(contracts []contract) []*contract {
	categorized := make([]*contract, 0, len(contracts))

	for i := range contracts {
		if len(contracts[i].Categories) > 0 {
			categorized = append(categorized, &contracts[i])
		}
	}

	slices.SortFunc(categorized, func(left, right *contract) int {
		return strings.Compare(left.Name, right.Name)
	})

	return categorized
}
