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

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeLongviewClientsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeLongviewClientsTool(cfg)
	if tool.Name != "linode_longview_client_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_longview_client_list")
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

func TestLinodeLongviewClientsToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewClientsBasePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewClientsBasePath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyLongviewAPIKey:      "longview-api-key-secret",
				keyLongviewApps:        map[string]bool{keyLongviewAppApache: true, databaseEngineName: true, keyLongviewAppNginx: false},
				keyCreated:             longviewClientCreatedFixture,
				keyID:                  789,
				keyLongviewInstallCode: "longview-install-code-secret",
				keyLabel:               longviewClientLabelFixture,
				keyUpdated:             longviewClientUpdatedFixture,
			}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 75,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewClientsTool(cfg)

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

	if !strings.Contains(textContent.Text, longviewClientLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", longviewClientLabelFixture)
	}

	if strings.Contains(textContent.Text, "longview-api-key-secret") {
		t.Errorf("textContent.Text should not contain %v", "longview-api-key-secret")
	}

	if strings.Contains(textContent.Text, "longview-install-code-secret") {
		t.Errorf("textContent.Text should not contain %v", "longview-install-code-secret")
	}
}

func TestLinodeLongviewClientsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != longviewClientsBasePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewClientsBasePath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewClientsTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_longview_client_list") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_longview_client_list")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeLongviewClientsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeLongviewClientsTool(cfg)

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

func TestLinodeLongviewClientUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)
	if tool.Name != "linode_longview_client_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_longview_client_update")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyClientID, keyLabel, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyClientID, keyConfirm, keyLabel} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLongviewClientUpdateToolSuccess(t *testing.T) {
	const (
		longviewClientUpdatedLabel  = "renamed-client"
		errLongviewClientIDPositive = "client_id must be a positive integer"
		errLongviewClientLabel      = "label must be 3-32 characters"
	)

	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body, map[string]any{keyLabel: longviewClientUpdatedLabel}) {
			t.Errorf("body = %v, want %v", body, map[string]any{keyLabel: longviewClientUpdatedLabel})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyLongviewAPIKey:      "longview-api-key-secret",
			keyLongviewApps:        map[string]bool{keyLongviewAppApache: true, databaseEngineName: true, keyLongviewAppNginx: false},
			keyID:                  789,
			keyLongviewInstallCode: "longview-install-code-secret",
			keyLabel:               longviewClientUpdatedLabel,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyClientID: 789, keyLabel: longviewClientUpdatedLabel, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, longviewClientUpdatedLabel) {
		t.Errorf("textContent.Text does not contain %v", longviewClientUpdatedLabel)
	}

	if strings.Contains(textContent.Text, "longview-api-key-secret") {
		t.Errorf("textContent.Text should not contain %v", "longview-api-key-secret")
	}

	if strings.Contains(textContent.Text, "longview-install-code-secret") {
		t.Errorf("textContent.Text should not contain %v", "longview-install-code-secret")
	}
}

func TestLinodeLongviewClientUpdateToolApiError(t *testing.T) {
	const (
		longviewClientUpdatedLabel  = "renamed-client"
		errLongviewClientIDPositive = "client_id must be a positive integer"
		errLongviewClientLabel      = "label must be 3-32 characters"
	)

	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyClientID: 789, keyLabel: longviewClientUpdatedLabel, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "Failed to update linode_longview_client_update") {
		t.Errorf("textContent.Text does not contain %v", "Failed to update linode_longview_client_update")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeLongviewClientUpdateToolConfirmRejectsBeforeClient(t *testing.T) {
	const (
		longviewClientUpdatedLabel  = "renamed-client"
		errLongviewClientIDPositive = "client_id must be a positive integer"
		errLongviewClientLabel      = "label must be 3-32 characters"
	)

	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)

			args := map[string]any{keyClientID: 789, keyLabel: longviewClientUpdatedLabel}
			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
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

			if !strings.Contains(textContent.Text, errConfirmEqualsTrue) {
				t.Errorf("textContent.Text does not contain %v", errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeLongviewClientUpdateToolValidationRejectsBeforeClient(t *testing.T) {
	const (
		longviewClientUpdatedLabel  = "renamed-client"
		errLongviewClientIDPositive = "client_id must be a positive integer"
		errLongviewClientLabel      = "label must be 3-32 characters"
	)

	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing client id", args: map[string]any{keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
		{name: "zero client id", args: map[string]any{keyClientID: 0, keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
		{name: "unsafe large client id", args: map[string]any{keyClientID: 9007199254740992.0, keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
		{name: "slash client id", args: map[string]any{keyClientID: longviewClientSlashID, keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
		{name: "query client id", args: map[string]any{keyClientID: "789?x=1", keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
		{name: "traversal client id", args: map[string]any{keyClientID: pathTraversalValue, keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
		{name: caseMissingLabel, args: map[string]any{keyClientID: 789, keyConfirm: true}, want: errLabelRequired},
		{name: "short label", args: map[string]any{keyClientID: 789, keyLabel: "ab", keyConfirm: true}, want: errLongviewClientLabel},
		{name: "slash label", args: map[string]any{keyClientID: 789, keyLabel: "bad/label", keyConfirm: true}, want: errLongviewClientLabel},
		{name: "long label", args: map[string]any{keyClientID: 789, keyLabel: "abcdefghijklmnopqrstuvwxyzabcdefg", keyConfirm: true}, want: errLongviewClientLabel},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)

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

func TestLinodeLongviewClientDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)
	if tool.Name != "linode_longview_client_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_longview_client_delete")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyClientID, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyClientID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLongviewClientDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
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
	_, _, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyClientID: 789, keyConfirm: true, keyConfirmedDryRun: true})

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

	if !strings.Contains(textContent.Text, "Longview client deleted successfully") {
		t.Errorf("textContent.Text does not contain %v", "Longview client deleted successfully")
	}

	if !strings.Contains(textContent.Text, "789") {
		t.Errorf("textContent.Text does not contain %v", "789")
	}
}

func TestLinodeLongviewClientDeleteToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyClientID: 789, keyConfirm: true, keyConfirmedDryRun: true})

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

	if !strings.Contains(textContent.Text, "Failed to delete linode_longview_client_delete") {
		t.Errorf("textContent.Text does not contain %v", "Failed to delete linode_longview_client_delete")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeLongviewClientDeleteToolConfirmRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)

			args := map[string]any{keyClientID: 789}
			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
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

			if !strings.Contains(textContent.Text, errConfirmEqualsTrue) {
				t.Errorf("textContent.Text does not contain %v", errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeLongviewClientDeleteToolValidationRejectsBeforeClient(t *testing.T) {
	const errLongviewClientIDPositive = "client_id must be a positive integer"

	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing client id", args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, want: errLongviewClientIDPositive},
		{name: "zero client id", args: map[string]any{keyClientID: 0, keyConfirm: true, keyConfirmedDryRun: true}, want: errLongviewClientIDPositive},
		{name: "unsafe large client id", args: map[string]any{keyClientID: 9007199254740992.0, keyConfirm: true, keyConfirmedDryRun: true}, want: errLongviewClientIDPositive},
		{name: "slash client id", args: map[string]any{keyClientID: longviewClientSlashID, keyConfirm: true, keyConfirmedDryRun: true}, want: errLongviewClientIDPositive},
		{name: "query client id", args: map[string]any{keyClientID: "789?x=1", keyConfirm: true, keyConfirmedDryRun: true}, want: errLongviewClientIDPositive},
		{name: "traversal client id", args: map[string]any{keyClientID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errLongviewClientIDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)

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
