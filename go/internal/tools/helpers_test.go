package tools_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
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

// Ensures all tool response serialization paths work correctly.
func TestMarshalToolResponseValidStruct(t *testing.T) {
	t.Parallel()

	input := map[string]string{"key": "value"}

	result, err := tools.MarshalToolResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("result.Content is empty")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, `"key"`) {
		t.Errorf("textContent.Text does not contain %v", `"key"`)
	}

	if !strings.Contains(textContent.Text, `"value"`) {
		t.Errorf("textContent.Text does not contain %v", `"value"`)
	}

	// Verify the output is valid JSON.
	var parsed map[string]string
	if err := json.Unmarshal([]byte(textContent.Text), &parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed["key"] != "value" {
		t.Errorf("got %v, want %v", parsed["key"], "value")
	}
}

func TestMarshalToolResponseNilInput(t *testing.T) {
	t.Parallel()

	result, err := tools.MarshalToolResponse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("result.Content is empty")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if textContent.Text != databaseJSONNull {
		t.Errorf("textContent.Text = %v, want %v", textContent.Text, databaseJSONNull)
	}
}

func TestMarshalToolResponseUnmarshalableType(t *testing.T) {
	t.Parallel()

	result, err := tools.MarshalToolResponse(make(chan int))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	if err == nil {
		t.Error("expected a marshal error, got nil")
	}
}

// Validates the safety gate for destructive write operations.
func TestRequireConfirm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      map[string]any
		wantNil   bool
		checkBody string
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
			args:    map[string]any{keyConfirm: "yes"},
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
		items     []testItem
		filter    string
		wantLen   int
		wantNames []string
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
		items     []testItem
		substr    string
		wantLen   int
		wantNames []string
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

// Ensures the standard list response envelope is built correctly.
func TestFormatListResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		items          []testItem
		filters        []string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "with items",
			items: []testItem{
				{Name: stageAlpha, Status: statusActive},
				{Name: stageBeta, Status: "inactive"},
			},
			filters:        nil,
			wantContains:   []string{`"count": 2`, stageAlpha, stageBeta},
			wantNotContain: []string{"filter"},
		},
		{
			name:         "empty items",
			items:        nil,
			filters:      nil,
			wantContains: []string{`"count": 0`},
		},
		{
			name: "with filters",
			items: []testItem{
				{Name: stageAlpha, Status: statusActive},
			},
			filters:      []string{"status=active", "region=us-east"},
			wantContains: []string{"status=active", "region=us-east"},
		},
		{
			name: "no filters",
			items: []testItem{
				{Name: stageAlpha},
			},
			filters:        nil,
			wantNotContain: []string{"filter"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := tools.FormatListResponse(tt.items, tt.filters, "items")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(textContent.Text, want) {
					t.Errorf("textContent.Text does not contain %v", want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(textContent.Text, notWant) {
					t.Errorf("textContent.Text should not contain %v", notWant)
				}
			}
		})
	}
}
