package main_test

import (
	"testing"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The tool_scopes refusals. The declaration is what stands between a routed
// tool and shipping silently unrestricted, so a missing, double-spoken, or
// unrenderable scope answer has to fail generation by tool name.

func TestRefusesAScopeDeclarationThatCannotAnswer(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "routed tool declaring neither none nor a scope",
			refusal: "errNoScopes",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeScopelessInput",
					getOptions(withoutScopes), pathInt(probeIDArg)))
			},
		},
		{
			name:    "none beside a scope list",
			refusal: "errScopesNoneBesideList",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeScopesBothInput",
					getOptions(withScopes(&linodev1.ToolScopes{
						None:  true,
						Scope: []linodev1.ToolScope{linodev1.ToolScope_TOOL_SCOPE_LINODES_READ_ONLY},
					})), pathInt(probeIDArg)))
			},
		},
		{
			name:    "meta tool declaring scopes",
			refusal: "errScopesOnMeta",
			build:   metaProbe("ProbeMetaScopesInput", probeScopes()),
		},
		{
			name:    "one scope declared twice",
			refusal: "errScopeRepeated",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeScopeTwiceInput",
					getOptions(withScopes(&linodev1.ToolScopes{
						Scope: []linodev1.ToolScope{
							linodev1.ToolScope_TOOL_SCOPE_LINODES_READ_ONLY,
							linodev1.ToolScope_TOOL_SCOPE_LINODES_READ_ONLY,
						},
					})), pathInt(probeIDArg)))
			},
		},
		{
			name:    "a member with no wire spelling",
			refusal: "errScopeUnrenderable",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbeScopeUnspecifiedInput",
					getOptions(withScopes(&linodev1.ToolScopes{
						Scope: []linodev1.ToolScope{linodev1.ToolScope_TOOL_SCOPE_UNSPECIFIED},
					})), pathInt(probeIDArg)))
			},
		},
	})
}
