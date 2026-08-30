package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestLinodeTagsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeTagListTool(cfg)

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

	tags := PaginatedResponse[Tag]{
		Data:    []Tag{{Label: tagLabel}},
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
	_, _, handler := gentools.NewLinodeTagListTool(cfg)

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
			_, _, handler := gentools.NewLinodeTagListTool(cfg)
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
	_, _, handler := gentools.NewLinodeTagListTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
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

		if err := json.NewEncoder(w).Encode(Tag{Label: tagCreateLabelFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeTagCreateTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeTagCreateTool(cfg)

	if tool.Name != toolLinodeTagCreate {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolLinodeTagCreate)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{
		keyLabel,
		tagCreateDomainsParam,
		tagCreateLinodesParam,
		tagCreateNodeBalancersParam,
		tagCreateVolumesParam,
		keyConfirm,
		keyDryRun,
	} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
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

		if err := json.NewEncoder(w).Encode(Tag{Label: tagCreateLabelFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeTagCreateTool(cfg)

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
	_, _, handler := gentools.NewLinodeTagCreateTool(cfg)

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

	assertDryRunRequest(t, decodeBody(t, textContent.Text), "POST", "/tags")

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
	_, _, handler := gentools.NewLinodeTagCreateTool(cfg)

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
		confirm any
		name    string
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
			_, _, handler := gentools.NewLinodeTagCreateTool(cfg)

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
		{name: "string linode ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateLinodesParam: []any{"101"}, keyConfirm: true}, want: "linodes must be an array of integers"},
		{name: caseZeroLinodeID, args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateLinodesParam: []any{float64(0)}, keyConfirm: true}, want: "linodes must be an array of positive integers"},
		{name: "non-array nodebalancer ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateNodeBalancersParam: "301", keyConfirm: true}, want: "nodebalancers must be an array of integers"},
		{name: "fractional nodebalancer ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateNodeBalancersParam: []any{float64(301.5)}, keyConfirm: true}, want: "nodebalancers must be an array of integers"},
		{name: "negative volume ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateVolumesParam: []any{float64(-401)}, keyConfirm: true}, want: "volumes must be an array of positive integers"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := gentools.NewLinodeTagCreateTool(cfg)
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
