package tools_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
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

// canRunFixtureCatalog is a reproducible catalog covering one tool per
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
// to canRunEnvProd, so the four refusal categories are all reachable.
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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &out); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	return out
}

// canRunResults extracts the typed results slice (checked, for forcetypeassert).
func canRunResults(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()

	raw, ok := body["results"].([]any)
	if !ok {
		t.Error("ok = false, want true")
	}

	out := make([]map[string]any, 0, len(raw))

	for _, entry := range raw {
		row, isMap := entry.(map[string]any)
		if !isMap {
			t.Error("isMap = false, want true")
		}

		out = append(out, row)
	}

	return out
}

func TestLinodeProfileCanRunToolSchemaAndCapability(t *testing.T) {
	_ = []any{
		canRunCall(canRunReadTool, ""),
		canRunCall(canRunReadTool, canRunEnvDev),
		canRunCall(canRunWriteTool, ""),
		canRunCall(canRunDestroyTool, ""),
		canRunCall(canRunUnknownTool, ""),
	}

	t.Parallel()

	tool, capability, _ := tools.NewLinodeProfileCanRunTool(canRunFixtureCatalog, canRunFixtureProfile)
	if tool.Name != canRunToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, canRunToolName)
	}

	if capability != profiles.CapMeta {
		t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
	}

	if _, ok := tool.InputSchema.Properties["calls"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "calls")
	}
}

func TestLinodeProfileCanRunToolClassifiesEveryRefusalCategoryAndTheAllowPath(t *testing.T) {
	allFiveCalls := []any{
		canRunCall(canRunReadTool, ""),
		canRunCall(canRunReadTool, canRunEnvDev),
		canRunCall(canRunWriteTool, ""),
		canRunCall(canRunDestroyTool, ""),
		canRunCall(canRunUnknownTool, ""),
	}

	t.Parallel()

	body := callCanRun(t, canRunFixtureProfile, allFiveCalls)
	if !reflect.DeepEqual(body["active_profile"], "compute-readonly") {
		t.Errorf("got %v, want %v", body["active_profile"], "compute-readonly")
	}

	results := canRunResults(t, body)
	if len(results) != 5 {
		t.Errorf("len(results) = %d, want %d", len(results), 5)
	}

	if !reflect.DeepEqual(results[0]["allowed"], true) {
		t.Errorf("got %v, want %v", results[0]["allowed"], true)
	}

	if !reflect.DeepEqual(results[1]["allowed"], false) {
		t.Errorf("got %v, want %v", results[1]["allowed"], false)
	}

	if !reflect.DeepEqual(results[1]["reason"], "environment not permitted by profile") {
		t.Errorf("got %v, want %v", results[1]["reason"], "environment not permitted by profile")
	}

	if !reflect.DeepEqual(results[2]["allowed"], false) {
		t.Errorf("got %v, want %v", results[2]["allowed"], false)
	}

	if !reflect.DeepEqual(results[2]["reason"], "tool not in profile's allowed_tools") {
		t.Errorf("got %v, want %v", results[2]["reason"], "tool not in profile's allowed_tools")
	}

	if !reflect.DeepEqual(results[3]["allowed"], false) {
		t.Errorf("got %v, want %v", results[3]["allowed"], false)
	}

	if reason, ok := results[3]["reason"].(string); !ok || !strings.Contains(reason, "(CapDestroy)") {
		t.Errorf("reason %v does not contain %q", results[3]["reason"], "(CapDestroy)")
	}

	if !reflect.DeepEqual(results[4]["allowed"], false) {
		t.Errorf("got %v, want %v", results[4]["allowed"], false)
	}

	if !reflect.DeepEqual(results[4]["reason"], "tool name not registered") {
		t.Errorf("got %v, want %v", results[4]["reason"], "tool name not registered")
	}
}

func TestLinodeProfileCanRunToolSummaryBucketsAndInvariant(t *testing.T) {
	allFiveCalls := []any{
		canRunCall(canRunReadTool, ""),
		canRunCall(canRunReadTool, canRunEnvDev),
		canRunCall(canRunWriteTool, ""),
		canRunCall(canRunDestroyTool, ""),
		canRunCall(canRunUnknownTool, ""),
	}

	t.Parallel()

	body := callCanRun(t, canRunFixtureProfile, allFiveCalls)

	summary, summaryIsMap := body["summary"].(map[string]any)
	if !summaryIsMap {
		t.Error("summaryIsMap = false, want true")
	}

	if summary["total"] != float64(5) {
		t.Errorf("value = %v, want %v", summary["total"], float64(5))
	}

	if summary["allowed"] != float64(1) {
		t.Errorf("value = %v, want %v", summary["allowed"], float64(1))
	}

	blocked, blockedIsFloat := summary["blocked"].(float64)
	if !blockedIsFloat {
		t.Error("blockedIsFloat = false, want true")
	}

	if blocked != float64(4) {
		t.Errorf("value = %v, want %v", blocked, float64(4))
	}

	buckets, bucketsIsMap := summary["blocked_by_reason"].(map[string]any)
	if !bucketsIsMap {
		t.Error("bucketsIsMap = false, want true")
	}

	for key, want := range map[string]any{
		"unregistered":      float64(1),
		"profile_block":     float64(1),
		"environment_block": float64(1),
		"capability_block":  float64(1),
	} {
		if !reflect.DeepEqual(buckets[key], want) {
			t.Errorf("buckets[%v] = %v, want %v", key, buckets[key], want)
		}
	}

	var bucketSum float64

	for _, value := range buckets {
		count, isFloat := value.(float64)
		if !isFloat {
			t.Error("isFloat = false, want true")
		}

		bucketSum += count
	}

	if bucketSum > blocked {
		t.Errorf("sum(blocked_by_reason) must be <= blocked: %v > %v", bucketSum, blocked)
	}
}

func TestLinodeProfileCanRunToolEmptyAllowedEnvironmentsPermitsAnyEnvironment(t *testing.T) {
	_ = []any{
		canRunCall(canRunReadTool, ""),
		canRunCall(canRunReadTool, canRunEnvDev),
		canRunCall(canRunWriteTool, ""),
		canRunCall(canRunDestroyTool, ""),
		canRunCall(canRunUnknownTool, ""),
	}

	t.Parallel()

	provider := func() profiles.Profile {
		profile := canRunFixtureProfile()
		profile.AllowedEnvironments = nil

		return profile
	}

	results := canRunResults(t, callCanRun(t, provider, []any{canRunCall(canRunReadTool, canRunEnvDev)}))
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want %d", len(results), 1)
	}

	if !reflect.DeepEqual(results[0]["allowed"], true) {
		t.Errorf("got %v, want %v", results[0]["allowed"], true)
	}
}

func TestLinodeProfileCanRunToolWildcardAllowedEnvironmentsPermitsAnyEnvironment(t *testing.T) {
	_ = []any{
		canRunCall(canRunReadTool, ""),
		canRunCall(canRunReadTool, canRunEnvDev),
		canRunCall(canRunWriteTool, ""),
		canRunCall(canRunDestroyTool, ""),
		canRunCall(canRunUnknownTool, ""),
	}

	t.Parallel()

	provider := func() profiles.Profile {
		profile := canRunFixtureProfile()
		profile.AllowedEnvironments = []string{"*"}

		return profile
	}

	results := canRunResults(t, callCanRun(t, provider, []any{canRunCall(canRunReadTool, canRunEnvDev)}))
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want %d", len(results), 1)
	}

	if !reflect.DeepEqual(results[0]["allowed"], true) {
		t.Errorf("got %v, want %v", results[0]["allowed"], true)
	}
}
