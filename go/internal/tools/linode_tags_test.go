package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeTagDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeTagDeleteTool(cfg)

	if tool.Name != "linode_tag_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_tag_delete")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
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

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if _, ok := props[keyDryRun]; !ok {
		t.Errorf("props missing key %v", keyDryRun)
	}

	for _, key := range []string{tagLabelParamTest, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeTagDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.EscapedPath() != "/tags/prod%2Fweb" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/tags/prod%2Fweb")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeTagDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{tagLabelParamTest: envProd + "/web", keyConfirm: true, keyConfirmedDryRun: true})

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

	if !strings.Contains(textContent.Text, envProd) {
		t.Errorf("textContent.Text does not contain %v", envProd)
	}
}

func TestLinodeTagDeleteToolConfirmRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissing, args: map[string]any{tagLabelParamTest: envProd}},
		{name: caseFalse, args: map[string]any{tagLabelParamTest: envProd, keyConfirm: false}},
		{name: caseString, args: map[string]any{tagLabelParamTest: envProd, keyConfirm: boolStringTrue}},
		{name: "number", args: map[string]any{tagLabelParamTest: envProd, keyConfirm: 1}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeTagDeleteTool(cfg)
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

			if !strings.Contains(textContent.Text, "confirm must be true") {
				t.Errorf("textContent.Text does not contain %v", "confirm must be true")
			}
		})
	}
}

func TestLinodeTagDeleteToolInvalidTagLabelRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing tag", args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, want: tagLabelRequiredMessage},
		{name: caseEmpty, args: map[string]any{tagLabelParamTest: "", keyConfirm: true, keyConfirmedDryRun: true}, want: tagLabelRequiredMessage},
		{name: "query", args: map[string]any{tagLabelParamTest: envProd + "?web", keyConfirm: true, keyConfirmedDryRun: true}, want: tagLabelPathErrorMessage},
		{name: caseFragment, args: map[string]any{tagLabelParamTest: envProd + "#web", keyConfirm: true, keyConfirmedDryRun: true}, want: tagLabelPathErrorMessage},
		{name: "tag label traversal", args: map[string]any{tagLabelParamTest: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, want: tagLabelPathErrorMessage},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeTagDeleteTool(cfg)
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

func TestLinodeTagDeleteToolDryRunPreviewsWithoutDeleting(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, "/tags/"+envProd+"/web", linode.PaginatedResponse[linode.TaggedObject]{
		Data: []linode.TaggedObject{{keyLabel: taggedObjectLabelFixture}},
	})
	_, _, handler := tools.NewLinodeTagDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{tagLabelParamTest: envProd + "/web", keyDryRun: true})

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

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "dry_run") {
		t.Errorf("textContent.Text does not contain %v", "dry_run")
	}

	if !strings.Contains(textContent.Text, "DELETE") {
		t.Errorf("textContent.Text does not contain %v", "DELETE")
	}

	if !strings.Contains(textContent.Text, "/tags/prod%2Fweb") {
		t.Errorf("textContent.Text does not contain %v", "/tags/prod%2Fweb")
	}

	if !strings.Contains(textContent.Text, "dependencies") {
		t.Errorf("textContent.Text does not contain %v", "dependencies")
	}

	if !strings.Contains(textContent.Text, "removed") {
		t.Errorf("textContent.Text does not contain %v", "removed")
	}

	if !strings.Contains(textContent.Text, "tagged object") {
		t.Errorf("textContent.Text does not contain %v", "tagged object")
	}
}

func TestLinodeTagDeleteToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
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
	_, _, handler := tools.NewLinodeTagDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{tagLabelParamTest: envProd, keyConfirm: true, keyConfirmedDryRun: true})

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

	if !strings.Contains(textContent.Text, "linode_tag_delete failed") {
		t.Errorf("textContent.Text does not contain %v", "linode_tag_delete failed")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeTagsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeTagsTool(cfg)

	if tool.Name != "linode_tag_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_tag_list")
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
}

func TestLinodeTagsToolSuccess(t *testing.T) {
	const tagLabel = "production"

	t.Parallel()

	tags := linode.PaginatedResponse[linode.Tag]{
		Data:    []linode.Tag{{Label: tagLabel}},
		Page:    2,
		Pages:   3,
		Results: 51,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcTags {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcTags)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(tags); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeTagsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

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

	if !strings.Contains(textContent.Text, tagLabel) {
		t.Errorf("textContent.Text does not contain %v", tagLabel)
	}
}

func TestLinodeTagsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeTagsTool(cfg)
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

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeTagsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcTags {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcTags)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeTagsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

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

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_tag_list") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_tag_list")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

const (
	toolLinodeTagCreate         = "linode_tag_create"
	tagCreateLabelFixture       = "production"
	tagCreateSuccessMessage     = "Tag 'production' created successfully"
	tagCreateConfirmError       = "This creates a Linode tag. Set confirm=true to proceed."
	tagCreateDomainsParam       = "domains"
	tagCreateLinodesParam       = "linodes"
	tagCreateNodeBalancersParam = "nodebalancers"
	tagCreateVolumesParam       = "volumes"
)

func TestLinodeTagCreateToolLabelOnlySuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcTags {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcTags)
		}

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		if decodeErr != nil {
			t.Errorf("unexpected error: %v", decodeErr)
		}

		if decodeErr != nil {
			return
		}

		if !reflect.DeepEqual(body, map[string]any{keyLabel: tagCreateLabelFixture}) {
			t.Errorf("body = %v, want %v", body, map[string]any{keyLabel: tagCreateLabelFixture})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Tag{Label: tagCreateLabelFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeTagCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLabel: tagCreateLabelFixture, keyConfirm: true})

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
}

func TestLinodeTagCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeTagCreateTool(cfg)

	if tool.Name != toolLinodeTagCreate {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolLinodeTagCreate)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	if _, ok := props[tagCreateDomainsParam]; !ok {
		t.Errorf("props missing key %v", tagCreateDomainsParam)
	}

	if _, ok := props[tagCreateLinodesParam]; !ok {
		t.Errorf("props missing key %v", tagCreateLinodesParam)
	}

	if _, ok := props[tagCreateNodeBalancersParam]; !ok {
		t.Errorf("props missing key %v", tagCreateNodeBalancersParam)
	}

	if _, ok := props[tagCreateVolumesParam]; !ok {
		t.Errorf("props missing key %v", tagCreateVolumesParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if _, ok := props["dry_run"]; !ok {
		t.Errorf("props missing key %v", "dry_run")
	}

	if !slices.Contains(tool.InputSchema.Required, keyLabel) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyLabel)
	}

	if slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required should not contain %v", keyConfirm)
	}
}

func TestLinodeTagCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcTags {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcTags)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		if decodeErr != nil {
			t.Errorf("unexpected error: %v", decodeErr)
		}

		if decodeErr != nil {
			return
		}

		for key, want := range map[string]any{
			keyLabel:                    tagCreateLabelFixture,
			tagCreateLinodesParam:       []any{float64(101), float64(102)},
			tagCreateDomainsParam:       []any{float64(201)},
			tagCreateNodeBalancersParam: []any{float64(301)},
			tagCreateVolumesParam:       []any{float64(401)},
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Tag{Label: tagCreateLabelFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeTagCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLabel:                    tagCreateLabelFixture,
		tagCreateLinodesParam:       []any{float64(101), float64(102)},
		tagCreateDomainsParam:       []any{float64(201)},
		tagCreateNodeBalancersParam: []any{float64(301)},
		tagCreateVolumesParam:       []any{float64(401)},
		keyConfirm:                  true,
	})

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

	if !strings.Contains(textContent.Text, tagCreateSuccessMessage) {
		t.Errorf("textContent.Text does not contain %v", tagCreateSuccessMessage)
	}
}

func TestLinodeTagCreateToolDryRunPreviewsRequestWithoutConfirmOrClient(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeTagCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLabel:              tagCreateLabelFixture,
		tagCreateLinodesParam: []any{float64(101)},
		"dry_run":             true,
	})

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

	if !strings.Contains(textContent.Text, `"method": "POST"`) {
		t.Errorf("textContent.Text does not contain %v", `"method": "POST"`)
	}

	if !strings.Contains(textContent.Text, `"path": "/tags"`) {
		t.Errorf("textContent.Text does not contain %v", `"path": "/tags"`)
	}

	if !strings.Contains(textContent.Text, tagCreateLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", tagCreateLabelFixture)
	}

	if !strings.Contains(textContent.Text, "side_effects") {
		t.Errorf("textContent.Text does not contain %v", "side_effects")
	}

	if !strings.Contains(textContent.Text, "new tag") {
		t.Errorf("textContent.Text does not contain %v", "new tag")
	}
}

func TestLinodeTagCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcTags {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcTags)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeTagCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLabel: tagCreateLabelFixture, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "Failed to create tag") {
		t.Errorf("textContent.Text does not contain %v", "Failed to create tag")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeTagCreateToolConfirmRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumeric, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeTagCreateTool(cfg)

			args := map[string]any{keyLabel: tagCreateLabelFixture}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

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

			if !strings.Contains(textContent.Text, tagCreateConfirmError) {
				t.Errorf("textContent.Text does not contain %v", tagCreateConfirmError)
			}
		})
	}
}

func TestLinodeTagCreateToolInvalidInputsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingLabel, args: map[string]any{keyConfirm: true}, want: errLabelRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyLabel: blankString, keyConfirm: true}, want: errLabelRequired},
		{name: "string linode ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateLinodesParam: []any{"101"}, keyConfirm: true}, want: "linodes must be an array of positive integers"},
		{name: caseZeroLinodeID, args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateLinodesParam: []any{float64(0)}, keyConfirm: true}, want: "linodes must be an array of positive integers"},
		{name: "empty domain ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateDomainsParam: []any{}, keyConfirm: true}, want: "domains must include at least one ID"},
		{name: "non-array nodebalancer ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateNodeBalancersParam: "301", keyConfirm: true}, want: "nodebalancers must be an array of positive integers"},
		{name: "fractional nodebalancer ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateNodeBalancersParam: []any{float64(301.5)}, keyConfirm: true}, want: "nodebalancers must be an array of positive integers"},
		{name: "negative volume ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateVolumesParam: []any{float64(-401)}, keyConfirm: true}, want: "volumes must be an array of positive integers"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeTagCreateTool(cfg)
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
