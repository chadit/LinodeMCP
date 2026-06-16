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
	tagLabelParamTest        = "tag_label"
	tagLabelRequiredMessage  = "tag_label must be a non-empty string"
	tagLabelPathErrorMessage = "tag_label must not contain"
	taggedObjectLabelFixture = "tagged-web-1"
)

func TestLinodeTaggedObjectsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeTaggedObjectsTool(cfg)

	if tool.Name != "linode_tag_object_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_tag_object_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[tagLabelParamTest]; !ok {
		t.Errorf("props missing key %v", tagLabelParamTest)
	}

	if _, ok := props[keyPage]; !ok {
		t.Errorf("props missing key %v", keyPage)
	}

	if _, ok := props[keyPageSize]; !ok {
		t.Errorf("props missing key %v", keyPageSize)
	}

	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}
}

func TestLinodeTaggedObjectsToolSuccess(t *testing.T) {
	t.Parallel()

	objects := linode.PaginatedResponse[linode.TaggedObject]{
		Data:    []linode.TaggedObject{{keyBetaID: float64(123), keyLabel: taggedObjectLabelFixture, keyType: monitorAlertDefinitionToolServiceType}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != "/tags/prod%2Fweb" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/tags/prod%2Fweb")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(objects); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeTaggedObjectsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{tagLabelParamTest: envProd + "/web", keyPage: 2, keyPageSize: 25})

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

	if !strings.Contains(textContent.Text, taggedObjectLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", taggedObjectLabelFixture)
	}

	if !strings.Contains(textContent.Text, monitorAlertDefinitionToolServiceType) {
		t.Errorf("textContent.Text does not contain %v", monitorAlertDefinitionToolServiceType)
	}
}

func TestLinodeTaggedObjectsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/tags/"+envProd {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/tags/"+envProd)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeTaggedObjectsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{tagLabelParamTest: envProd})

	result, err := handler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_tag_object_list") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_tag_object_list")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeTaggedObjectsToolInvalidTagLabelRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{}, want: tagLabelRequiredMessage},
		{name: caseEmpty, args: map[string]any{tagLabelParamTest: ""}, want: tagLabelRequiredMessage},
		{name: "query", args: map[string]any{tagLabelParamTest: envProd + "?web"}, want: tagLabelPathErrorMessage},
		{name: caseFragment, args: map[string]any{tagLabelParamTest: envProd + "#web"}, want: tagLabelPathErrorMessage},
		{name: "tag label traversal", args: map[string]any{tagLabelParamTest: pathTraversalValue}, want: tagLabelPathErrorMessage},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeTaggedObjectsTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
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
