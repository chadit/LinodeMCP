package tools_test

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "result content must be TextContent")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &out))

	return out
}

// canRunResults extracts the typed results slice (checked, for forcetypeassert).
func canRunResults(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()

	raw, ok := body["results"].([]any)
	require.True(t, ok, "results must be an array")

	out := make([]map[string]any, 0, len(raw))

	for _, entry := range raw {
		row, isMap := entry.(map[string]any)
		require.True(t, isMap, "each result must be an object")

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
		assert.Equal(t, canRunToolName, tool.Name)
		assert.Equal(t, profiles.CapMeta, capability)
		assert.Contains(t, tool.InputSchema.Properties, "calls")
	})

	t.Run("classifies every refusal category and the allow path", func(t *testing.T) {
		t.Parallel()

		body := callCanRun(t, canRunFixtureProfile, allFiveCalls)
		assert.Equal(t, "compute-readonly", body["active_profile"])

		results := canRunResults(t, body)
		require.Len(t, results, 5)

		assert.Equal(t, true, results[0]["allowed"])

		assert.Equal(t, false, results[1]["allowed"])
		assert.Equal(t, "environment not permitted by profile", results[1]["reason"])

		assert.Equal(t, false, results[2]["allowed"])
		assert.Equal(t, "tool not in profile's allowed_tools", results[2]["reason"])

		assert.Equal(t, false, results[3]["allowed"])
		assert.Contains(t, results[3]["reason"], "(CapDestroy)")

		assert.Equal(t, false, results[4]["allowed"])
		assert.Equal(t, "tool name not registered", results[4]["reason"])
	})

	t.Run("summary buckets and invariant", func(t *testing.T) {
		t.Parallel()

		body := callCanRun(t, canRunFixtureProfile, allFiveCalls)

		summary, summaryIsMap := body["summary"].(map[string]any)
		require.True(t, summaryIsMap)
		assert.InDelta(t, float64(5), summary["total"], 0)
		assert.InDelta(t, float64(1), summary["allowed"], 0)

		blocked, blockedIsFloat := summary["blocked"].(float64)
		require.True(t, blockedIsFloat)
		assert.InDelta(t, float64(4), blocked, 0)

		buckets, bucketsIsMap := summary["blocked_by_reason"].(map[string]any)
		require.True(t, bucketsIsMap)
		assert.InDelta(t, float64(1), buckets["unregistered"], 0)
		assert.InDelta(t, float64(1), buckets["profile_block"], 0)
		assert.InDelta(t, float64(1), buckets["environment_block"], 0)
		assert.InDelta(t, float64(1), buckets["capability_block"], 0)

		var bucketSum float64

		for _, value := range buckets {
			count, isFloat := value.(float64)
			require.True(t, isFloat)

			bucketSum += count
		}

		assert.LessOrEqual(t, bucketSum, blocked, "sum(blocked_by_reason) must be <= blocked")
	})

	t.Run("empty allowed_environments permits any environment", func(t *testing.T) {
		t.Parallel()

		provider := func() profiles.Profile {
			profile := canRunFixtureProfile()
			profile.AllowedEnvironments = nil

			return profile
		}

		results := canRunResults(t, callCanRun(t, provider, []any{canRunCall(canRunReadTool, canRunEnvDev)}))
		require.Len(t, results, 1)
		assert.Equal(t, true, results[0]["allowed"], "unrestricted env profile allows any environment")
	})

	t.Run("wildcard allowed_environments permits any environment", func(t *testing.T) {
		t.Parallel()

		provider := func() profiles.Profile {
			profile := canRunFixtureProfile()
			profile.AllowedEnvironments = []string{"*"}

			return profile
		}

		results := canRunResults(t, callCanRun(t, provider, []any{canRunCall(canRunReadTool, canRunEnvDev)}))
		require.Len(t, results, 1)
		assert.Equal(t, true, results[0]["allowed"])
	})
}
