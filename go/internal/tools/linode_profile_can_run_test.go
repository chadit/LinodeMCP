package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
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

	state := builderState(builder.NewRegistry(), canRunFixtureCatalog(), profile, emptyConfig())

	handler := generatedBuilder(gentools.NewLinodeProfileCanRunTool, nil)

	return builderBody(t, callBuilder(t, state, handler, map[string]any{tcCalls: calls}))
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

	tool, capability, _ := gentools.NewLinodeProfileCanRunTool(&config.Config{})
	if tool.Name != canRunToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, canRunToolName)
	}

	if capability != profiles.CapMeta {
		t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
	}

	if !strings.Contains(string(tool.RawInputSchema), tcCalls) {
		t.Errorf("tool.RawInputSchema missing key %v", tcCalls)
	}
}

// TestLinodeProfileCanRunSkipsEntriesThatAreNotObjects covers the shape guard
// on the calls array. The schema says each entry is an object, but the pre-check
// reads the arguments as they arrived, so a bare string among them must be
// passed over rather than counted or crashed on: the verdicts that follow it
// still have to be answered.
func TestLinodeProfileCanRunSkipsEntriesThatAreNotObjects(t *testing.T) {
	t.Parallel()

	body := callCanRun(t, canRunFixtureProfile, []any{
		"not-an-object",
		canRunCall(canRunReadTool, ""),
	})

	results := canRunResults(t, body)
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1: the malformed entry must be skipped", len(results))
	}

	if results[0][canRunKeyTool] != canRunReadTool {
		t.Errorf("results[0] tool = %v, want %v", results[0][canRunKeyTool], canRunReadTool)
	}

	summary, ok := body["summary"].(map[string]any)
	if !ok {
		t.Fatal("summary must be an object")
	}

	if summary["total"] != float64(1) {
		t.Errorf("summary total = %v, want 1", summary["total"])
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

// TestLinodeProfileCanRunToolRemediesMatchThePythonWording pins the four remedy
// sentences. They are an exact-match contract with the Python implementation the
// same way the reason strings above are, and they were the half of that contract
// neither language pinned, so a one-sided reword read as green in both.
func TestLinodeProfileCanRunToolRemediesMatchThePythonWording(t *testing.T) {
	t.Parallel()

	body := callCanRun(t, canRunFixtureProfile, []any{
		canRunCall(canRunReadTool, ""),
		canRunCall(canRunReadTool, canRunEnvDev),
		canRunCall(canRunWriteTool, ""),
		canRunCall(canRunDestroyTool, ""),
		canRunCall(canRunUnknownTool, ""),
	})

	results := canRunResults(t, body)
	if len(results) != 5 {
		t.Fatalf("len(results) = %d, want %d", len(results), 5)
	}

	for index, want := range map[int]string{
		1: "target an environment in the profile's allowed_environments, or switch to a profile that permits this environment",
		2: "switch to a profile that permits linode_instance_create, or add it to the current profile",
		3: "switch to a profile that permits linode_instance_delete, or use yolo on a profile that allows it",
		4: "check spelling or call linode_profile_list_tools to discover the registered tool surface",
	} {
		if !reflect.DeepEqual(results[index]["remedy"], want) {
			t.Errorf("results[%d][remedy] = %v, want %q", index, results[index]["remedy"], want)
		}
	}

	if _, present := results[0]["remedy"]; present {
		t.Errorf("results[0] carries a remedy %v, want none on an allowed call", results[0]["remedy"])
	}
}

// canRunSharedFixture mirrors testdata/profile/can_run_verdicts.json, the
// fixture the Python suite asserts against too.
type canRunSharedFixture struct {
	Profile struct {
		Name                string   `json:"name"`
		AllowedTools        []string `json:"allowed_tools"`
		AllowedEnvironments []string `json:"allowed_environments"`
	} `json:"profile"`
	ExpectResult map[string]any `json:"expect_result"`
	Catalog      []struct {
		Tool       string `json:"tool"`
		Capability string `json:"capability"`
	} `json:"catalog"`
	Calls []map[string]any `json:"calls"`
}

// canRunCapabilities maps the fixture's capability spellings onto the tags the
// pre-check reads. The fixture names them the way a profile file does.
func canRunCapabilities() map[string]profiles.Capability {
	return map[string]profiles.Capability{
		capabilityRead:    profiles.CapRead,
		tcCapabilityWrite: profiles.CapWrite,
		"destroy":         profiles.CapDestroy,
	}
}

// readCanRunSharedFixture loads the shared verdict fixture.
func readCanRunSharedFixture(t *testing.T) *canRunSharedFixture {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(
		"..", "..", "..", "testdata", "profile", "can_run_verdicts.json",
	))
	if err != nil {
		t.Fatalf("read shared fixture: %v", err)
	}

	fixture := new(canRunSharedFixture)
	if err := json.Unmarshal(raw, fixture); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fixture.Calls) == 0 {
		t.Fatal("the shared fixture names no calls, so this test measures nothing")
	}

	return fixture
}

// TestCanRunVerdictsMatchSharedFixture holds every reason and remedy the
// pre-check answers to the shared cross-language fixture.
//
// A can_run reason is a member of a successful answer rather than a refusal, so
// no declaration words it and each language keeps its own copy. The behavior
// fixtures run under full-access, which permits every tool in every
// environment, so three of the four blocked categories are unreachable there:
// this fixture is what stops a one-sided reword.
func TestCanRunVerdictsMatchSharedFixture(t *testing.T) {
	t.Parallel()

	fixture := readCanRunSharedFixture(t)
	tags := canRunCapabilities()

	catalog := make([]profiles.ToolDescriptor, 0, len(fixture.Catalog))

	for _, entry := range fixture.Catalog {
		capability, known := tags[entry.Capability]
		if !known {
			t.Fatalf("fixture capability %q is outside the pre-check's vocabulary", entry.Capability)
		}

		catalog = append(catalog, profiles.ToolDescriptor{Name: entry.Tool, Capability: capability})
	}

	calls := make([]any, 0, len(fixture.Calls))
	for _, call := range fixture.Calls {
		calls = append(calls, call)
	}

	profile := profiles.Profile{
		Name:                fixture.Profile.Name,
		AllowedTools:        fixture.Profile.AllowedTools,
		AllowedEnvironments: fixture.Profile.AllowedEnvironments,
	}

	state := builderState(
		builder.NewRegistry(), catalog,
		func() profiles.Profile { return profile }, emptyConfig(),
	)
	handler := generatedBuilder(gentools.NewLinodeProfileCanRunTool, nil)
	body := builderBody(t, callBuilder(t, state, handler, map[string]any{tcCalls: calls}))

	want := canRunFixtureJSON(t, fixture.ExpectResult)
	if got := canRunFixtureJSON(t, body); got != want {
		t.Errorf("answer = %s, want %s", got, want)
	}
}

// canRunFixtureJSON is one answer as key-sorted JSON, so the comparison reads
// values rather than the order two decoders happened to build a map in.
func canRunFixtureJSON(t *testing.T, value any) string {
	t.Helper()

	encoded, err := json.Marshal(canRunNumbersAsText(value))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return string(encoded)
}

// canRunNumbersAsText rewrites every number as its text, because the fixture
// decodes counts as float64 and the answer decodes them the same way only by
// accident of both going through JSON.
func canRunNumbersAsText(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, member := range typed {
			out[key] = canRunNumbersAsText(member)
		}

		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, member := range typed {
			out = append(out, canRunNumbersAsText(member))
		}

		return out
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	default:
		return value
	}
}
