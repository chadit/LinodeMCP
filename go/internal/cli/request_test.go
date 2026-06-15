package cli_test

import (
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/cli"
)

// schemaWith builds a ToolInputSchema whose properties carry the given
// JSON-schema types, with no required fields. A thin wrapper over the
// shared makeSchema so the request tests read the same way they did.
func schemaWith(types map[string]string) mcp.ToolInputSchema {
	return makeSchema(types, nil)
}

// TestBuildArgumentsCoercesBySchema checks that each --arg value is typed
// from the tool's schema: numbers become float64, integers become int64,
// booleans become bool, and strings stay strings. A property absent from
// the schema falls back to a string, matching the permissive default.
func TestBuildArgumentsCoercesBySchema(t *testing.T) {
	t.Parallel()

	schema := schemaWith(map[string]string{
		"size":    schemaTypeInteger,
		"ratio":   "number",
		"enabled": schemaTypeBoolean,
		argLabel:  schemaTypeString,
	})

	args, err := cli.BuildArguments(schema, "", []string{
		"size=20",
		"ratio=1.5",
		"enabled=true",
		"label=web",
		"unknown=keepme",
	})
	if err != nil {
		t.Fatalf("BuildArguments returned error: %v", err)
	}

	assertEqual(t, "size", args["size"], int64(20))
	assertEqual(t, "ratio", args["ratio"], 1.5)
	assertEqual(t, "enabled", args["enabled"], true)
	assertEqual(t, "label", args["label"], "web")
	assertEqual(t, "unknown", args["unknown"], "keepme")
}

// TestBuildArgumentsIntegerFallsBackToFloat checks that an integer-typed
// property given a fractional value is not silently truncated: it stays a
// float64 so the server's validation, not the CLI, decides it's invalid.
func TestBuildArgumentsIntegerFallsBackToFloat(t *testing.T) {
	t.Parallel()

	schema := schemaWith(map[string]string{"count": "integer"})

	args, err := cli.BuildArguments(schema, "", []string{"count=1.5"})
	if err != nil {
		t.Fatalf("BuildArguments returned error: %v", err)
	}

	assertEqual(t, "count", args["count"], 1.5)
}

// TestBuildArgumentsBadNumberStaysString checks that a non-numeric value
// for a number-typed property is left as the raw string rather than
// dropped, so the dispatch surfaces one authoritative validation error.
func TestBuildArgumentsBadNumberStaysString(t *testing.T) {
	t.Parallel()

	schema := schemaWith(map[string]string{"size": "number"})

	args, err := cli.BuildArguments(schema, "", []string{"size=notanumber"})
	if err != nil {
		t.Fatalf("BuildArguments returned error: %v", err)
	}

	assertEqual(t, "size", args["size"], "notanumber")
}

// TestBuildArgumentsJSONObject checks the --json path: a JSON object is
// used verbatim as the arguments map, types preserved as JSON decoded
// them.
func TestBuildArgumentsJSONObject(t *testing.T) {
	t.Parallel()

	args, err := cli.BuildArguments(mcp.ToolInputSchema{}, `{"label":"v1","size":20}`, nil)
	if err != nil {
		t.Fatalf("BuildArguments returned error: %v", err)
	}

	assertEqual(t, "label", args["label"], "v1")
	assertEqual(t, "size", args["size"], float64(20))
}

// TestBuildArgumentsRejectsArgAndJSON checks the mutual-exclusion rule:
// supplying both --json and --arg is a usage error.
func TestBuildArgumentsRejectsArgAndJSON(t *testing.T) {
	t.Parallel()

	_, err := cli.BuildArguments(mcp.ToolInputSchema{}, `{"a":1}`, []string{"b=2"})
	if !errors.Is(err, cli.ErrArgAndJSON) {
		t.Fatalf("error = %v, want ErrArgAndJSON", err)
	}
}

// TestBuildArgumentsRejectsBadJSON checks that malformed --json is a
// usage error rather than a silent empty map.
func TestBuildArgumentsRejectsBadJSON(t *testing.T) {
	t.Parallel()

	_, err := cli.BuildArguments(mcp.ToolInputSchema{}, `{not json`, nil)
	if err == nil {
		t.Fatal("BuildArguments accepted malformed JSON")
	}
}

// TestBuildArgumentsRejectsJSONArray checks that a JSON array (valid JSON
// but not an object) is rejected, since it can't be an arguments map.
func TestBuildArgumentsRejectsJSONArray(t *testing.T) {
	t.Parallel()

	_, err := cli.BuildArguments(mcp.ToolInputSchema{}, `[1,2,3]`, nil)
	if !errors.Is(err, cli.ErrJSONNotObject) {
		t.Fatalf("error = %v, want ErrJSONNotObject", err)
	}
}

// TestBuildArgumentsRejectsBadPair checks that a --arg token without '='
// is a format error.
func TestBuildArgumentsRejectsBadPair(t *testing.T) {
	t.Parallel()

	_, err := cli.BuildArguments(mcp.ToolInputSchema{}, "", []string{"noequalsign"})
	if !errors.Is(err, cli.ErrArgFormat) {
		t.Fatalf("error = %v, want ErrArgFormat", err)
	}
}

// TestBuildArgumentsValueWithEquals checks that only the first '=' splits
// a pair, so a value containing '=' (a base64 token, say) survives whole.
func TestBuildArgumentsValueWithEquals(t *testing.T) {
	t.Parallel()

	args, err := cli.BuildArguments(mcp.ToolInputSchema{}, "", []string{"data=YWJjPT0="})
	if err != nil {
		t.Fatalf("BuildArguments returned error: %v", err)
	}

	assertEqual(t, "data", args["data"], "YWJjPT0=")
}

// TestApplySafetyFlagsFoldsSetFields checks that set safety flags land in
// the arguments map under the MCP field names, and unset ones (nil bool
// pointers, empty strings) stay absent so the tool's defaults apply.
func TestApplySafetyFlagsFoldsSetFields(t *testing.T) {
	t.Parallel()

	dryRun := true

	var confirm bool

	args := map[string]any{"instance_id": int64(123)}
	flags := cli.SafetyFlags{
		DryRun:      &dryRun,
		Confirm:     &confirm,
		Mode:        "plan",
		PlanID:      "plan-abc",
		Environment: "staging",
	}

	cli.ApplySafetyFlags(args, &flags)

	assertEqual(t, "dry_run", args["dry_run"], true)
	assertEqual(t, "confirm", args["confirm"], false)
	assertEqual(t, "mode", args["mode"], "plan")
	assertEqual(t, "plan_id", args["plan_id"], "plan-abc")
	assertEqual(t, "environment", args["environment"], "staging")

	if _, present := args["yolo"]; present {
		t.Error("yolo was folded in despite being unset")
	}

	if _, present := args["confirmed_dry_run"]; present {
		t.Error("confirmed_dry_run was folded in despite being unset")
	}
}

// TestApplySafetyFlagsEmptyLeavesArgsUntouched checks that an all-unset
// SafetyFlags adds nothing, so a plain read tool's request stays clean.
func TestApplySafetyFlagsEmptyLeavesArgsUntouched(t *testing.T) {
	t.Parallel()

	args := map[string]any{"region": "us-east"}
	flags := cli.SafetyFlags{}

	cli.ApplySafetyFlags(args, &flags)

	if len(args) != 1 {
		t.Fatalf("args grew to %d entries, want 1", len(args))
	}
}

// assertEqual fails the test when got != want, naming the field. Kept
// local to avoid an assertion-library dependency (project policy: stdlib
// testing only).
func assertEqual(t *testing.T, field string, got, want any) {
	t.Helper()

	if got != want {
		t.Errorf("%s = %v (%T), want %v (%T)", field, got, got, want, want)
	}
}
