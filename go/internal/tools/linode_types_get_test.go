package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	linodeTypeGetToolName       = "linode_type_get"
	linodeTypeGetID             = "g6-standard-2"
	linodeTypeGetPath           = "/linode/types/g6-standard-2"
	linodeTypeGetLabel          = "Linode 4GB"
	linodeTypeIDRequiredMessage = "type_id must be a non-empty string"
	linodeTypeIDInvalidMessage  = "type_id must not contain '/', '?', '#', or '..'"
)

func TestLinodeTypeGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeTypeGetTool(cfg)

		assert.Equal(t, linodeTypeGetToolName, tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should declare read capability")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, linodeTypeGetPath, r.URL.EscapedPath(), "request path should include type id")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.InstanceType{ID: linodeTypeGetID, Label: linodeTypeGetLabel, Class: classStandard}))
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, linodeTypeGetID, "response should contain type id")
		assert.Contains(t, textContent.Text, linodeTypeGetLabel, "response should contain type label")
	})
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

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError, "invalid type_id should return tool error")

			textContent, ok := result.Content[0].(mcp.TextContent)
			require.True(t, ok, "content should be TextContent")
			assert.Contains(t, textContent.Text, testCase.want)
		})
	}
}
