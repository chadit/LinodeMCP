package main_test

import (
	"testing"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The tool_categories refusals. The declaration is required on every input,
// meta included, so a mutator cannot ship unreachable by every category-scoped
// profile in silence.

func TestRefusesACategoryDeclarationThatCannotAnswer(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "tool declaring neither none nor a category",
			refusal: "errNoCategories",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeUncategorizedInput",
					getOptions(withoutCategories), pathInt(probeIDArg)))
			},
		},
		{
			name:    "meta tool declaring neither none nor a category",
			refusal: "errNoCategories",
			build:   metaProbe("ProbeMetaUncategorizedInput", withoutCategories),
		},
		{
			name:    "none beside a category list",
			refusal: "errCategoriesNoneBesideList",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeCategoriesBothInput",
					getOptions(withCategories(&linodev1.ToolCategories{
						None:     true,
						Category: []linodev1.ToolCategory{linodev1.ToolCategory_TOOL_CATEGORY_COMPUTE},
					})), pathInt(probeIDArg)))
			},
		},
		{
			name:    "one category declared twice",
			refusal: "errCategoryRepeated",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeCategoryTwiceInput",
					getOptions(withCategories(&linodev1.ToolCategories{
						Category: []linodev1.ToolCategory{
							linodev1.ToolCategory_TOOL_CATEGORY_COMPUTE,
							linodev1.ToolCategory_TOOL_CATEGORY_COMPUTE,
						},
					})), pathInt(probeIDArg)))
			},
		},
		{
			name:    "a member with no spelling",
			refusal: "errCategoryUnrenderable",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeCategoryUnspecifiedInput",
					getOptions(withCategories(&linodev1.ToolCategories{
						Category: []linodev1.ToolCategory{linodev1.ToolCategory_TOOL_CATEGORY_UNSPECIFIED},
					})), pathInt(probeIDArg)))
			},
		},
	})
}
