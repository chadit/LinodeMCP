package tools_test

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// TestStandardPaginationRejections covers the shared page/page_size reader
// through the tools that adopted it. Each case must be rejected before any
// client is built, so the assertions hold with no server configured: a value
// that slipped past validation would instead surface as a transport error.
func TestStandardPaginationRejections(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         "non-integer page",
			args:         map[string]any{keyPage: "x"},
			wantContains: "page must be an integer",
		},
		{
			name:         "non-integer page_size",
			args:         map[string]any{keyPageSize: "x"},
			wantContains: "page_size must be an integer",
		},
		{
			name:         "page below minimum",
			args:         map[string]any{keyPage: float64(0)},
			wantContains: "page must be",
		},
		{
			name:         "page_size below minimum",
			args:         map[string]any{keyPageSize: float64(1)},
			wantContains: errPageSizeRange,
		},
		{
			name:         "page_size above maximum",
			args:         map[string]any{keyPageSize: float64(501)},
			wantContains: errPageSizeRange,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeRegionListTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Fatal("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantContains) {
				t.Errorf("text = %q, want it to contain %q", textContent.Text, testCase.wantContains)
			}
		})
	}
}

// TestInstanceListRejectsBadPagination covers the instance list handler's own
// pagination branch. That handler is hand-written rather than built from the
// shared paginated factory, so its validation needs its own guard: a regression
// there would send an unvalidated page straight to the API.
func TestInstanceListRejectsBadPagination(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeInstanceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPageSize: "x"})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatal("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "page_size must be an integer") {
		t.Errorf("text = %q, want a page_size validation message", textContent.Text)
	}
}
