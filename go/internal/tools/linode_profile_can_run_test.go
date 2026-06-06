package tools_test

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	canRunToolName    = "linode_profile_can_run"
	canRunReadTool    = "linode_instance_list"
	canRunWriteTool   = "linode_instance_create"
	canRunDestroyTool = "linode_instance_delete"
	canRunUnknownTool = "linode_not_a_real_tool"
	canRunEnvProd     = "prod"
	canRunEnvDev      = "dev"
	// canRunKeyTool / canRunKeyArgs are the request entry keys, hoisted so
	// the repeated literals don't trip goconst across the fixtures.
	canRunKeyTool = "tool"
	canRunKeyArgs = "args"
	canRunKeyEnv  = "environment"
)

// canRunFixtureCatalog is a deterministic catalog covering one tool per
// capability the pre-check distinguishes.
func canRunFixtureCatalog() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		{Name: canRunReadTool, Capability: profiles.CapRead},
		{Name: canRunWriteTool, Capability: profiles.CapWrite},
		{Name: canRunDestroyTool, Capability: profiles.CapDestroy},
		{Name: canRunToolName, Capability: profiles.CapMeta},
	}
}

// canRunFixtureProfile permits only the read tool and restricts environments
// to "prod", so the four refusal categories are all reachable.
func canRunFixtureProfile() profiles.Profile {
	return profiles.Profile{
		Name:                "compute-readonly",
		AllowedTools:        []string{canRunReadTool},
		AllowedEnvironments: []string{canRunEnvProd},
	}
}

// canRunCall builds one {tool, args:{environment}} entry. env == "" omits args.
func canRunCall(toolName, env string) map[string]any {
	entry := map[string]any{canRunKeyTool: toolName}
	if env != "" {
		entry[canRunKeyArgs] = map[string]any{canRunKeyEnv: env}
	}

	return entry
}

// callCanRun invokes the pre-check handler against the given profile provider
// and returns the parsed response.
func callCanRun(t *testing.T, profile func() profiles.Profile, calls []any) map[string]any {
	t.Helper()

	_, _, handler := tools.NewLinodeProfileCanRunTool(canRunFixtureCatalog, profile)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"calls": calls}

	result, err := handler(t.Context(), req)
	expectNoError(t, err)
	expectNotNil(t, result)
	expectFalse(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	expectTrue(t, ok, "result content must be TextContent")

	var out map[string]any
	expectNoError(t, json.Unmarshal([]byte(textContent.Text), &out))

	return out
}

// canRunResults extracts the typed results slice (checked, for forcetypeassert).
func canRunResults(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()

	raw, ok := body["results"].([]any)
	expectTrue(t, ok, "results must be an array")

	out := make([]map[string]any, 0, len(raw))

	for _, entry := range raw {
		row, isMap := entry.(map[string]any)
		expectTrue(t, isMap, "each result must be an object")

		out = append(out, row)
	}

	return out
}

func TestLinodeProfileCanRunTool(t *testing.T) {
	t.Parallel()

	allFiveCalls := []any{
		canRunCall(canRunReadTool, ""),
		canRunCall(canRunReadTool, canRunEnvDev),
		canRunCall(canRunWriteTool, ""),
		canRunCall(canRunDestroyTool, ""),
		canRunCall(canRunUnknownTool, ""),
	}

	t.Run("schema and capability", func(t *testing.T) {
		t.Parallel()

		tool, capability, _ := tools.NewLinodeProfileCanRunTool(canRunFixtureCatalog, canRunFixtureProfile)
		checkEqual(t, canRunToolName, tool.Name)
		checkEqual(t, profiles.CapMeta, capability)
		expectContainsWithMode(t, false, tool.InputSchema.Properties, "calls")
	})

	t.Run("classifies every refusal category and the allow path", func(t *testing.T) {
		t.Parallel()

		body := callCanRun(t, canRunFixtureProfile, allFiveCalls)
		checkEqual(t, "compute-readonly", body["active_profile"])

		results := canRunResults(t, body)
		expectLen(t, results, 5)

		checkEqual(t, true, results[0]["allowed"])

		checkEqual(t, false, results[1]["allowed"])
		checkEqual(t, "environment not permitted by profile", results[1]["reason"])

		checkEqual(t, false, results[2]["allowed"])
		checkEqual(t, "tool not in profile's allowed_tools", results[2]["reason"])

		checkEqual(t, false, results[3]["allowed"])
		expectContainsWithMode(t, false, results[3]["reason"], "(CapDestroy)")

		checkEqual(t, false, results[4]["allowed"])
		checkEqual(t, "tool name not registered", results[4]["reason"])
	})

	t.Run("summary buckets and invariant", func(t *testing.T) {
		t.Parallel()

		body := callCanRun(t, canRunFixtureProfile, allFiveCalls)

		summary, summaryIsMap := body["summary"].(map[string]any)
		expectTrue(t, summaryIsMap)
		expectNumericEqual(t, float64(5), summary["total"])
		expectNumericEqual(t, float64(1), summary["allowed"])

		blocked, blockedIsFloat := summary["blocked"].(float64)
		expectTrue(t, blockedIsFloat)
		expectNumericEqual(t, float64(4), blocked)

		buckets, bucketsIsMap := summary["blocked_by_reason"].(map[string]any)
		expectTrue(t, bucketsIsMap)
		expectNumericEqual(t, float64(1), buckets["unregistered"])
		expectNumericEqual(t, float64(1), buckets["profile_block"])
		expectNumericEqual(t, float64(1), buckets["environment_block"])
		expectNumericEqual(t, float64(1), buckets["capability_block"])

		var bucketSum float64

		for _, value := range buckets {
			count, isFloat := value.(float64)
			expectTrue(t, isFloat)

			bucketSum += count
		}

		if bucketSum > blocked {
			t.Errorf("expected %v <= %v%s", bucketSum, blocked, expectationMessage([]string{"sum(blocked_by_reason) must be <= blocked"}))
		}
	})

	t.Run("empty allowed_environments permits any environment", func(t *testing.T) {
		t.Parallel()

		provider := func() profiles.Profile {
			profile := canRunFixtureProfile()
			profile.AllowedEnvironments = nil

			return profile
		}

		results := canRunResults(t, callCanRun(t, provider, []any{canRunCall(canRunReadTool, canRunEnvDev)}))
		expectLen(t, results, 1)
		checkEqual(t, true, results[0]["allowed"], "unrestricted env profile allows any environment")
	})

	t.Run("wildcard allowed_environments permits any environment", func(t *testing.T) {
		t.Parallel()

		provider := func() profiles.Profile {
			profile := canRunFixtureProfile()
			profile.AllowedEnvironments = []string{"*"}

			return profile
		}

		results := canRunResults(t, callCanRun(t, provider, []any{canRunCall(canRunReadTool, canRunEnvDev)}))
		expectLen(t, results, 1)
		checkEqual(t, true, results[0]["allowed"])
	})
}
