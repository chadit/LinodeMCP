package tools_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

type testItem struct {
	Name   string
	Status string
}

// TestSetLiveConfigSourceLifecycle exercises the process-wide hook used by
// main.go to bridge the config Watcher to tool handlers. Cannot observe
// resolveConfig from outside the package (cairnlint forbids export_test.go),
// but the hook's observable contract is "stored callbacks can be set and
// cleared without panic", which this test pins. Suppression must be
// inline on the func declaration so newer golangci-lint releases
// associate the directive with the function.
func TestSetLiveConfigSourceLifecycle(_ *testing.T) { //nolint:paralleltest // SetLiveConfigSource manipulates a process-wide hook.
	defer tools.SetLiveConfigSource(nil)

	snapshot := &config.Config{Server: config.ServerConfig{Name: "snap"}}

	// Set, clear, set with a different function, clear again. Each step
	// must not panic. The store is a sync/atomic.Pointer; correctness of
	// load/store under concurrent access is stdlib's contract.
	tools.SetLiveConfigSource(func() *config.Config { return snapshot })
	tools.SetLiveConfigSource(nil)
	tools.SetLiveConfigSource(func() *config.Config { return nil })
	tools.SetLiveConfigSource(nil)
}

// Validates the safety gate for destructive write operations.
func TestRequireConfirm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args      map[string]any
		name      string
		checkBody string
		wantNil   bool
	}{
		{
			name:    "true boolean",
			args:    map[string]any{keyConfirm: true},
			wantNil: true,
		},
		{
			name:      "false boolean",
			args:      map[string]any{keyConfirm: false},
			wantNil:   false,
			checkBody: errConfirmEqualsTrue,
		},
		{
			name:    "missing confirm key",
			args:    map[string]any{},
			wantNil: false,
		},
		{
			name:      "string true rejected",
			args:      map[string]any{keyConfirm: boolStringTrue},
			wantNil:   false,
			checkBody: errConfirmEqualsTrue,
		},
		{
			name:      "integer one rejected",
			args:      map[string]any{keyConfirm: 1},
			wantNil:   false,
			checkBody: errConfirmEqualsTrue,
		},
		{
			name:    "string yes not recognized by GetBool",
			args:    map[string]any{keyConfirm: phase2NonBoolean},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: tt.args,
				},
			}

			result := tools.RequireConfirm(&request, "set confirm=true to proceed")

			if tt.wantNil {
				if result != nil {
					t.Errorf("result = %v, want nil", result)
				}

				return
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if tt.checkBody != "" {
				textContent, ok := result.Content[0].(mcp.TextContent)
				if !ok {
					t.Fatal("ok = false, want true")
				}

				if !strings.Contains(textContent.Text, tt.checkBody) {
					t.Errorf("textContent.Text does not contain %v", tt.checkBody)
				}
			}
		})
	}
}

// Ensures list filtering by exact field value works for all edge cases.
func TestFilterByField(t *testing.T) {
	t.Parallel()

	statusExtractor := func(item testItem) string { return item.Status }

	tests := []struct {
		name      string
		filter    string
		items     []testItem
		wantNames []string
		wantLen   int
	}{
		{
			name: "exact match",
			items: []testItem{
				{Name: stageAlpha, Status: statusActive},
				{Name: stageBeta, Status: "inactive"},
				{Name: stageGamma, Status: statusActive},
			},
			filter:    statusActive,
			wantLen:   2,
			wantNames: []string{stageAlpha, stageGamma},
		},
		{
			name: "case insensitive",
			items: []testItem{
				{Name: stageAlpha, Status: "Active"},
				{Name: stageBeta, Status: "ACTIVE"},
			},
			filter:  statusActive,
			wantLen: 2,
		},
		{
			name: "no match",
			items: []testItem{
				{Name: stageAlpha, Status: statusActive},
			},
			filter:  "deleted",
			wantLen: 0,
		},
		{
			name: "empty filter value",
			items: []testItem{
				{Name: stageAlpha, Status: statusActive},
			},
			filter:  "",
			wantLen: 0,
		},
		{
			name:    "empty slice",
			items:   nil,
			filter:  statusActive,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			filtered := tools.FilterByField(tt.items, tt.filter, statusExtractor)
			if len(filtered) != tt.wantLen {
				t.Errorf("len(filtered) = %d, want %d", len(filtered), tt.wantLen)
			}

			for idx, wantName := range tt.wantNames {
				if filtered[idx].Name != wantName {
					t.Errorf("filtered[idx].Name = %v, want %v", filtered[idx].Name, wantName)
				}
			}
		})
	}
}

// Ensures list filtering by substring containment works for all edge cases.
func TestFilterByContains(t *testing.T) {
	t.Parallel()

	nameExtractor := func(item testItem) string { return item.Name }

	tests := []struct {
		name      string
		substr    string
		items     []testItem
		wantNames []string
		wantLen   int
	}{
		{
			name: "substring match",
			items: []testItem{
				{Name: "web-server-01"},
				{Name: "db-server-01"},
				{Name: "web-proxy"},
			},
			substr:    "web",
			wantLen:   2,
			wantNames: []string{"web-server-01", "web-proxy"},
		},
		{
			name: "case insensitive",
			items: []testItem{
				{Name: "WebServer"},
				{Name: "WEBPROXY"},
				{Name: "database"},
			},
			substr:  "web",
			wantLen: 2,
		},
		{
			name: "no match",
			items: []testItem{
				{Name: stageAlpha},
				{Name: stageBeta},
			},
			substr:  stageGamma,
			wantLen: 0,
		},
		{
			name: "empty substring matches all",
			items: []testItem{
				{Name: stageAlpha},
				{Name: stageBeta},
			},
			substr:  "",
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			filtered := tools.FilterByContains(tt.items, tt.substr, nameExtractor)
			if len(filtered) != tt.wantLen {
				t.Errorf("len(filtered) = %d, want %d", len(filtered), tt.wantLen)
			}

			for idx, wantName := range tt.wantNames {
				if filtered[idx].Name != wantName {
					t.Errorf("filtered[idx].Name = %v, want %v", filtered[idx].Name, wantName)
				}
			}
		})
	}
}

// TestIntSliceArgument pins the ID-array reader the generated hooks reach. It
// is exercised through the exported helper rather than through a tool because
// the last handler that read an array of ids by hand moved to the generator.
func TestIntSliceArgument(t *testing.T) {
	t.Parallel()

	const name = "configs"

	positiveIntegers := name + " must be an array of positive integers"
	atLeastOne := name + " must include at least one ID"

	cases := map[string]struct {
		raw     any
		message string
		want    []int
	}{
		"typed slice":            {raw: []int{11, 22}, want: []int{11, 22}},
		"empty typed slice":      {raw: []int{}, message: atLeastOne},
		"typed slice with zero":  {raw: []int{11, 0}, message: positiveIntegers},
		"json numbers":           {raw: []any{float64(11), float64(22)}, want: []int{11, 22}},
		"empty json array":       {raw: []any{}, message: atLeastOne},
		"fractional json number": {raw: []any{1.5}, message: positiveIntegers},
		"negative json number":   {raw: []any{float64(-1)}, message: positiveIntegers},
		"boundary json number":   {raw: []any{math.MaxFloat64}, message: positiveIntegers},
		"untyped int":            {raw: []any{7}, want: []int{7}},
		"untyped int below one":  {raw: []any{0}, message: positiveIntegers},
		"entry of another kind":  {raw: []any{"11"}, message: positiveIntegers},
		"not an array":           {raw: "11,22", message: positiveIntegers},
	}

	for testName, testCase := range cases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			ids, message := tools.IntSliceArgument(testCase.raw, name)
			if message != testCase.message {
				t.Fatalf("message = %q, want %q", message, testCase.message)
			}

			if !slices.Equal(ids, testCase.want) {
				t.Errorf("ids = %v, want %v", ids, testCase.want)
			}
		})
	}
}

// TestObjectMapArgumentReadsBothObjectForms covers every branch of the reader
// the service-transfer entities argument still goes through: the native map the
// schema produces, the JSON-string form some clients send, and the blank value
// that means the caller sent nothing.
const (
	// entityKindTransfer is the one value the entities reader table carries,
	// named so the table states what a service transfer's entities object holds.
	entityKindTransfer = "service-transfer"
	// errEntitiesNotObject is the sentence the reader answers for every shape
	// that is not an object.
	errEntitiesNotObject = "entities must be an object"
)

func TestObjectMapArgumentReadsBothObjectForms(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw     any
		want    string
		message string
	}{
		"absent":       {raw: nil},
		"native map":   {raw: map[string]any{"kind": entityKindTransfer}, want: entityKindTransfer},
		"json string":  {raw: `{"kind":"service-transfer"}`, want: entityKindTransfer},
		"blank string": {raw: "   "},
		"bad json":     {raw: "{oops", message: errEntitiesNotObject},
		"array":        {raw: []any{entityKindTransfer}, message: errEntitiesNotObject},
		caseNumeric:    {raw: float64(5), message: errEntitiesNotObject},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			value, message := tools.ObjectMapArgument(testCase.raw, "entities")
			if message != testCase.message {
				t.Errorf("message = %q, want %q", message, testCase.message)
			}

			var kind string
			if raw, found := value["kind"]; found {
				kind, _ = raw.(string)
			}

			if kind != testCase.want {
				t.Errorf("value[\"kind\"] = %q, want %q", kind, testCase.want)
			}
		})
	}
}

// TestRequiredPresentArgumentReadsPresenceNotValue pins the one question this
// reader answers: did the caller send the argument. An empty list is a value
// the routes using it act on (it removes every assignment), so it has to pass
// where an absent argument is refused.
func TestRequiredPresentArgumentReadsPresenceNotValue(t *testing.T) {
	t.Parallel()

	const firewallIDsKey = "firewall_ids"

	missing := firewallIDsKey + " is required"

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{absentEnvironment, map[string]any{}, missing},
		{"empty list is a value", map[string]any{firewallIDsKey: []any{}}, ""},
		{"populated list", map[string]any{firewallIDsKey: []any{float64(1)}}, ""},
		{"explicit null is a value", map[string]any{firewallIDsKey: nil}, ""},
		{
			"another argument does not answer for it",
			map[string]any{keyManagedLinodeSettingsLinodeID: float64(1)},
			missing,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := createRequestWithArgs(t, testCase.args)
			if got := tools.RequiredPresentArgument(&request, firewallIDsKey); got != testCase.want {
				t.Errorf("RequiredPresentArgument = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestToolsCallTheEnvironmentTheCallerNames is the found half of the
// environment lookup. The default environment points at a closed port so the
// call can only succeed by reaching the one the caller named.
func TestToolsCallTheEnvironmentTheCallerNames(t *testing.T) {
	t.Parallel()

	var gotQuery string

	srv := listPageServer(t, listPagePathDomains, &gotQuery)
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "http://127.0.0.1:1", Token: tokenTest}},
		envProd:       {Label: envProd, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}

	_, _, handler := gentools.NewLinodeDomainListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyEnvironment: envProd}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result = %q, want the named environment's server answer", resultText(t, result))
	}
}
