package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The two merge refusal directions, and the malformed-option wording.
const (
	routeDanglingLine  = "overlay anchors a route the surface does not carry"
	routeUncoveredLine = "upstream route has no overlay entry"
	routeUnparsedLine  = "tool route does not parse"
	routeCountWord     = "surface route(s)"
	routeKey           = "Sample"
)

// A tree whose one message declares a route, which is what the route arm splits.
const routeProto = `syntax = "proto3";

package sample;

// Sample is the input contract for linode_sample_get.
message Sample {
  option (linode.mcp.v1.tool_route) = {
    tool: "linode_sample_get"
    method: "GET"
    path: "/sample/{sample_id}"
  };
  option (linode.mcp.v1.tool_capability) = TOOL_CAPABILITY_READ;

  // The label.
  string label = 1 [(linode.mcp.v1.field_location) = FIELD_LOCATION_PATH];
}
`

// routeTree writes the synthetic workspace around the routed proto.
func routeTree(t *testing.T) string {
	t.Helper()

	return treeWith(t, routeProto)
}

// routes refuses an empty map, so a case cannot pass by reshaping nothing.
func routes(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()

	table, ok := doc["routes"].(map[string]any)
	if !ok || len(table) == 0 {
		t.Fatalf("the descriptor carries no route map, so the case would measure nothing")
	}

	return table
}

// The route travels in the descriptor, so a merge has something to compare
// when upstream stops carrying it.
func TestSurfaceDescriptorCarriesTheDeclaredRoute(t *testing.T) {
	t.Parallel()

	root := routeTree(t)

	raw, err := os.ReadFile(emit(t, root))
	if err != nil {
		t.Fatalf("read the descriptor: %v", err)
	}

	text := string(raw)
	for _, want := range []string{`"` + routeKey + `"`, `"method": "GET"`, `"path": "/sample/{sample_id}"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("the descriptor does not state %s:\n%s", want, text)
		}
	}

	if strings.Contains(text, "linode_sample_get") {
		t.Fatalf("the descriptor states the tool name, which is MCP naming the overlay owns:\n%s", text)
	}
}

// The repo contract round-trips with routes rendered, not spliced, as counted.
func TestRepoTreeRoundTripReportsItsRoutes(t *testing.T) {
	t.Parallel()

	output, code := runGate(t, repoRoot)
	if code != 0 {
		t.Fatalf("the contract does not round-trip (exit %d):\n%s", code, output)
	}

	if !strings.Contains(output, routeCountWord) || strings.Contains(output, "0 "+routeCountWord) {
		t.Fatalf("the scan report states no route:\n%s", output)
	}
}

// The rendered method comes from the descriptor, so an upstream move lands.
func TestMergeRendersTheRouteTheDescriptorStates(t *testing.T) {
	t.Parallel()

	root := routeTree(t)
	surface := emit(t, root)

	reshape(t, surface, func(_, doc map[string]any) {
		routes(t, doc)[routeKey] = map[string]any{"method": "POST", "path": "/sample/{sample_id}/rebuild"}
	})

	output, code, out := mergeInto(t, root, surface)
	if code != 0 {
		t.Fatalf("the merge refused a route it can render (exit %d):\n%s", code, output)
	}

	merged, err := os.ReadFile(filepath.Join(out, sampleRel))
	if err != nil {
		t.Fatalf("read the merged file: %v", err)
	}

	text := string(merged)
	if !strings.Contains(text, `method: "POST"`) || !strings.Contains(text, `path: "/sample/{sample_id}/rebuild"`) {
		t.Fatalf("the merged tree kept the old route:\n%s", text)
	}

	if !strings.Contains(text, `tool: "linode_sample_get"`) {
		t.Fatalf("the merged tree lost the tool name the overlay owns:\n%s", text)
	}
}

// REQ-D4, dropped route: refused by name rather than merged from the tree.
func TestDroppedUpstreamRouteIsRefused(t *testing.T) {
	t.Parallel()

	root := routeTree(t)
	surface := emit(t, root)

	reshape(t, surface, func(_, doc map[string]any) {
		delete(routes(t, doc), routeKey)
	})

	output, code, out := mergeInto(t, root, surface)
	if code == 0 {
		t.Fatalf("a dropped upstream route merged clean:\n%s", output)
	}

	if !strings.Contains(output, routeDanglingLine) || !strings.Contains(output, routeKey) {
		t.Fatalf("the refusal does not name the dropped route:\n%s", output)
	}

	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("the refused merge wrote a tree to %s", out)
	}
}

// The other direction: an added route has no tool name and no MCP semantics.
func TestAddedUpstreamRouteIsRefused(t *testing.T) {
	t.Parallel()

	root := routeTree(t)
	surface := emit(t, root)

	reshape(t, surface, func(_, doc map[string]any) {
		routes(t, doc)["Newcomer"] = map[string]any{"method": "GET", "path": "/newcomer"}
	})

	output, code, _ := mergeInto(t, root, surface)
	if code == 0 {
		t.Fatalf("an added upstream route merged clean:\n%s", output)
	}

	if !strings.Contains(output, routeUncoveredLine) || !strings.Contains(output, "Newcomer") {
		t.Fatalf("the refusal does not name the added route:\n%s", output)
	}
}

// Copying an unmodellable route through would leave the message with no route
// and no sign the extraction skipped it.
func TestUnmodelledRouteOptionIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		from string
		into string
	}{
		{"entries out of order", `    tool: "linode_sample_get"
    method: "GET"`, `    method: "GET"
    tool: "linode_sample_get"`},
		{"entry indent does not line up", `    method: "GET"`, `      method: "GET"`},
		{"an entry the model has no slot for", `    path: "/sample/{sample_id}"`, `    path: "/sample/{sample_id}"
    note: "extra"`},
		{"an unquoted entry value", `    method: "GET"`, `    method: GET`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			root := routeTree(t)
			plant(t, root, testCase.from, testCase.into)

			output, code := runGate(t, root)
			if code == 0 {
				t.Fatalf("a route the model cannot re-render passed:\n%s", output)
			}

			if !strings.Contains(output, routeUnparsedLine) || !strings.Contains(output, sampleRel+":7") {
				t.Fatalf("the refusal does not name the option:\n%s", output)
			}
		})
	}
}

// A statement ending before its entries is refused, not read past its own end.
func TestTruncatedRouteOptionIsRefused(t *testing.T) {
	t.Parallel()

	root := treeWith(t, `syntax = "proto3";

package sample;

message Sample {
  option (linode.mcp.v1.tool_route) = {
    tool: "linode_sample_get"
`)

	output, code := runGate(t, root)
	if code == 0 {
		t.Fatalf("a truncated route option passed:\n%s", output)
	}

	if !strings.Contains(output, routeUnparsedLine) || !strings.Contains(output, sampleRel+":6") {
		t.Fatalf("the refusal does not name the option:\n%s", output)
	}
}

// A route at file scope has no message to key it under.
func TestRouteOutsideAnyMessageIsRefused(t *testing.T) {
	t.Parallel()

	root := treeWith(t, `syntax = "proto3";

package sample;

option (linode.mcp.v1.tool_route) = {
  tool: "linode_sample_get"
  method: "GET"
  path: "/sample"
};
`)

	output, code := runGate(t, root)
	if code == 0 {
		t.Fatalf("a file-scope route option passed:\n%s", output)
	}

	if !strings.Contains(output, "tool route sits outside any message") {
		t.Fatalf("the refusal does not name the scope problem:\n%s", output)
	}
}

// Two messages under one surface key would leave the second answering for the
// first everywhere the route is read.
func TestDuplicateRouteKeyIsRefused(t *testing.T) {
	t.Parallel()

	root := routeTree(t)
	writeFile(t, filepath.Join(root, "proto", "twin.proto"), strings.Replace(
		routeProto, "package sample;", "package twin;", 1,
	))

	output, code := runGate(t, root)
	if code == 0 {
		t.Fatalf("two messages claimed one route key:\n%s", output)
	}

	if !strings.Contains(output, "two messages claim one surface route") {
		t.Fatalf("the refusal does not name the duplicate:\n%s", output)
	}
}
