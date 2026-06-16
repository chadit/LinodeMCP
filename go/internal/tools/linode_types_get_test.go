package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	linodeTypeGetToolName       = "linode_type_get"
	linodeTypeGetID             = "g6-standard-2"
	linodeTypeGetPath           = "/linode/types/g6-standard-2"
	linodeTypeGetLabel          = "Linode 4GB"
	linodeTypeIDRequiredMessage = "type_id must be a non-empty string"
	linodeTypeIDInvalidMessage  = "type_id must not contain '/', '?', '#', or '..'"
)

func TestLinodeTypeGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeTypeGetTool(cfg)

	if tool.Name != linodeTypeGetToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, linodeTypeGetToolName)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeTypeGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != linodeTypeGetPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), linodeTypeGetPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.InstanceType{ID: linodeTypeGetID, Label: linodeTypeGetLabel, Class: classStandard}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeTypeGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{databaseTypeIDParam: linodeTypeGetID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, linodeTypeGetID) {
		t.Errorf("textContent.Text does not contain %v", linodeTypeGetID)
	}

	if !strings.Contains(textContent.Text, linodeTypeGetLabel) {
		t.Errorf("textContent.Text does not contain %v", linodeTypeGetLabel)
	}
}

func TestLinodeTypeGetToolValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing type_id", args: map[string]any{}, want: linodeTypeIDRequiredMessage},
		{name: "blank type_id", args: map[string]any{databaseTypeIDParam: blankString}, want: linodeTypeIDRequiredMessage},
		{name: "slash type_id", args: map[string]any{databaseTypeIDParam: "g6/standard"}, want: linodeTypeIDInvalidMessage},
		{name: "query type_id", args: map[string]any{databaseTypeIDParam: "g6-standard?plan=2"}, want: linodeTypeIDInvalidMessage},
		{name: "fragment type_id", args: map[string]any{databaseTypeIDParam: "g6-standard#frag"}, want: linodeTypeIDInvalidMessage},
		{name: "traversal type_id", args: map[string]any{databaseTypeIDParam: "g6-..-standard"}, want: linodeTypeIDInvalidMessage},
		{name: "numeric type_id", args: map[string]any{databaseTypeIDParam: 123}, want: linodeTypeIDRequiredMessage},
	}

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeTypeGetTool(cfg)

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.want) {
				t.Errorf("textContent.Text does not contain %v", testCase.want)
			}
		})
	}
}
