package tools_test

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/tools"
)

type testItem struct {
	Name   string
	Status string
}

// TestMarshalToolResponse verifies JSON serialization of various input types
// into MCP text results, including valid structs, nil, and unmarshalable types.
//
// Workflow:
//  1. **Setup**: Prepare each input value (map, nil, channel)
//  2. **Execute**: Call MarshalToolResponse for each case
//  3. **Verify**: Check that valid inputs produce correct JSON text content
//     and unmarshalable types return an error
//
// Expected Behavior:
//   - Valid struct produces indented JSON wrapped in TextContent
//   - Nil input produces "null" text content
//   - Unmarshalable type (channel) returns an error with "failed to marshal response"
//
// Purpose: Ensures all tool response serialization paths work correctly.
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

// TestRequireConfirm verifies that the confirm parameter gate correctly allows
// or blocks write operations based on boolean-coercible argument values.
//
// Workflow:
//  1. **Setup**: Build CallToolRequest with various confirm argument types
//  2. **Execute**: Call RequireConfirm for each case
//  3. **Verify**: Check nil (allowed) vs non-nil error result (blocked)
//
// Expected Behavior:
//   - Boolean true returns nil (operation proceeds)
//   - Boolean false returns an error result
//   - Missing confirm key returns an error result
//   - String "true" is coerced to true via strconv.ParseBool
//   - Integer 1 is coerced to true via GetBool
//   - String "yes" is not recognized by GetBool and treated as false
//
// Purpose: Validates the safety gate for destructive write operations.
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
			args:    map[string]any{"confirm": true},
			wantNil: true,
		},
		{
			name:      "false boolean",
			args:      map[string]any{"confirm": false},
			wantNil:   false,
			checkBody: "confirm=true",
		},
		{
			name:    "missing confirm key",
			args:    map[string]any{},
			wantNil: false,
		},
		{
			name:    "string true coerced via ParseBool",
			args:    map[string]any{"confirm": "true"},
			wantNil: true,
		},
		{
			name:    "integer one coerced via GetBool",
			args:    map[string]any{"confirm": 1},
			wantNil: true,
		},
		{
			name:    "string yes not recognized by GetBool",
			args:    map[string]any{"confirm": "yes"},
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

// TestFilterByField verifies exact case-insensitive field matching across
// various scenarios including matches, misses, empty filters, and empty slices.
//
// Workflow:
//  1. **Setup**: Define test items and filter values for each case
//  2. **Execute**: Call FilterByField with a Status field extractor
//  3. **Verify**: Check the returned slice length and content
//
// Expected Behavior:
//   - Exact status match returns matching items (case-insensitive)
//   - Non-matching filter returns empty slice
//   - Empty filter value only matches items with empty field values
//   - Empty input slice returns empty result
//
// Purpose: Ensures list filtering by exact field value works for all edge cases.
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
				{Name: "alpha", Status: "active"},
				{Name: "beta", Status: "inactive"},
				{Name: "gamma", Status: "active"},
			},
			filter:    "active",
			wantLen:   2,
			wantNames: []string{"alpha", "gamma"},
		},
		{
			name: "case insensitive",
			items: []testItem{
				{Name: "alpha", Status: "Active"},
				{Name: "beta", Status: "ACTIVE"},
			},
			filter:  "active",
			wantLen: 2,
		},
		{
			name: "no match",
			items: []testItem{
				{Name: "alpha", Status: "active"},
			},
			filter:  "deleted",
			wantLen: 0,
		},
		{
			name: "empty filter value",
			items: []testItem{
				{Name: "alpha", Status: "active"},
			},
			filter:  "",
			wantLen: 0,
		},
		{
			name:    "empty slice",
			items:   nil,
			filter:  "active",
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

// TestFilterByContains verifies substring-based case-insensitive matching
// across various scenarios including partial matches and empty substrings.
//
// Workflow:
//  1. **Setup**: Define test items and substring values for each case
//  2. **Execute**: Call FilterByContains with a Name field extractor
//  3. **Verify**: Check the returned slice length and content
//
// Expected Behavior:
//   - Substring present in name returns matching items
//   - Matching is case-insensitive
//   - Non-matching substring returns empty slice
//   - Empty substring matches all items (every string contains "")
//
// Purpose: Ensures list filtering by substring containment works for all edge cases.
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
				{Name: "alpha"},
				{Name: "beta"},
			},
			substr:  "gamma",
			wantLen: 0,
		},
		{
			name: "empty substring matches all",
			items: []testItem{
				{Name: "alpha"},
				{Name: "beta"},
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

// TestFormatListResponse verifies that list responses are correctly formatted
// with item counts, applied filters, and JSON structure.
//
// Workflow:
//  1. **Setup**: Prepare items and optional filter slices for each case
//  2. **Execute**: Call FormatListResponse with the test data
//  3. **Verify**: Check the JSON text content for expected fields and values
//
// Expected Behavior:
//   - Non-empty items produce correct count and item names in output
//   - Empty items produce count of zero
//   - Applied filters appear in the output text
//   - Nil filters produce no filter key in the output
//
// Purpose: Ensures the standard list response envelope is built correctly.
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
				{Name: "alpha", Status: "active"},
				{Name: "beta", Status: "inactive"},
			},
			filters:        nil,
			wantContains:   []string{`"count": 2`, "alpha", "beta"},
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
				{Name: "alpha", Status: "active"},
			},
			filters:      []string{"status=active", "region=us-east"},
			wantContains: []string{"status=active", "region=us-east"},
		},
		{
			name: "no filters",
			items: []testItem{
				{Name: "alpha"},
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
