package tools_test

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
)

type testItem struct {
	Name   string
	Status string
}

// TestSetLiveConfigSourceLifecycle exercises the process-wide hook used by
// main.go to bridge the config Watcher to tool handlers. Cannot observe
// resolveConfig from outside the package (cairnlint forbids export_test.go),
// but the hook's observable contract is "stored callbacks can be set and
// cleared without panic", which this test pins.
//
//nolint:paralleltest // SetLiveConfigSource manipulates a process-wide hook.
func TestSetLiveConfigSourceLifecycle(t *testing.T) {
	defer tools.SetLiveConfigSource(nil)

	snapshot := &config.Config{Server: config.ServerConfig{Name: "snap"}}

	// Set, clear, set with a different function, clear again. Each step
	// must not panic. The store is a sync/atomic.Pointer; correctness of
	// load/store under concurrent access is stdlib's contract.
	require.NotPanics(t, func() {
		tools.SetLiveConfigSource(func() *config.Config { return snapshot })
	})
	require.NotPanics(t, func() {
		tools.SetLiveConfigSource(nil)
	})
	require.NotPanics(t, func() {
		tools.SetLiveConfigSource(func() *config.Config { return nil })
	})
	require.NotPanics(t, func() {
		tools.SetLiveConfigSource(nil)
	})
}

// Ensures all tool response serialization paths work correctly.
func TestMarshalToolResponse(t *testing.T) {
	t.Parallel()

	t.Run("valid struct", func(t *testing.T) {
		t.Parallel()

		input := map[string]string{"key": "value"}

		result, err := tools.MarshalToolResponse(input)
		require.NoError(t, err, "valid struct should marshal without error")
		require.NotNil(t, result, "result should not be nil for valid input")
		require.NotEmpty(t, result.Content, "result content should not be empty")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "expected TextContent type")
		assert.Contains(t, textContent.Text, `"key"`, "output should contain the key")
		assert.Contains(t, textContent.Text, `"value"`, "output should contain the value")

		// Verify the output is valid JSON.
		var parsed map[string]string
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &parsed), "output should be valid JSON")
		assert.Equal(t, "value", parsed["key"], "parsed value should match input")
	})

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()

		result, err := tools.MarshalToolResponse(nil)
		require.NoError(t, err, "nil input should marshal without error")
		require.NotNil(t, result, "result should not be nil for nil input")
		require.NotEmpty(t, result.Content, "result content should not be empty for nil input")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "expected TextContent type for nil input")
		assert.Equal(t, "null", textContent.Text, "nil input should produce null JSON text")
	})

	t.Run("unmarshalable type", func(t *testing.T) {
		t.Parallel()

		result, err := tools.MarshalToolResponse(make(chan int))
		require.Error(t, err, "channel type should fail to marshal")
		assert.Nil(t, result, "result should be nil when marshaling fails")
		assert.ErrorContains(t, err, "failed to marshal response", "error should describe the marshal failure")
	})
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
				assert.Nil(t, result, "confirm result should be nil for %s", tt.name)

				return
			}

			require.NotNil(t, result, "confirm result should be non-nil for %s", tt.name)
			assert.True(t, result.IsError, "result should be an error for %s", tt.name)

			if tt.checkBody != "" {
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "expected TextContent type for %s", tt.name)
				assert.Contains(t, textContent.Text, tt.checkBody, "error body should contain expected text for %s", tt.name)
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
			assert.Len(t, filtered, tt.wantLen, "filtered length for %s", tt.name)

			for idx, wantName := range tt.wantNames {
				assert.Equal(t, wantName, filtered[idx].Name, "filtered item name at index %d for %s", idx, tt.name)
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
			assert.Len(t, filtered, tt.wantLen, "filtered length for %s", tt.name)

			for idx, wantName := range tt.wantNames {
				assert.Equal(t, wantName, filtered[idx].Name, "filtered item name at index %d for %s", idx, tt.name)
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
			require.NoError(t, err, "FormatListResponse should not error for %s", tt.name)
			require.NotNil(t, result, "result should not be nil for %s", tt.name)

			textContent, ok := result.Content[0].(mcp.TextContent)
			require.True(t, ok, "expected TextContent type for %s", tt.name)

			for _, want := range tt.wantContains {
				assert.Contains(t, textContent.Text, want, "response should contain %q for %s", want, tt.name)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, textContent.Text, notWant, "response should not contain %q for %s", notWant, tt.name)
			}
		})
	}
}
