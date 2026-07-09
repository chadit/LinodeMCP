package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// behaviorFixture is one shared cross-language behavior contract file from
// testdata/behavior/. The Python runner (tests/unit/test_behavior_conformance.py)
// replays the same cases, so a handler whose validation, coercion, error text,
// or outgoing HTTP request drifts from the other language fails one of the two
// runners. The HTTP layer is an in-process fake: no network, no credentials.
type behaviorFixture struct {
	Tool  string         `json:"tool"`
	Cases []behaviorCase `json:"cases"`
}

// behaviorCase drives one dispatch. Exactly one of ExpectError (a local
// validation failure, bare message text) or ExpectRequest (the HTTP call the
// handler must make) is set.
type behaviorCase struct {
	Name          string           `json:"name"`
	Args          map[string]any   `json:"args"`
	APIResponse   json.RawMessage  `json:"api_response"`
	ExpectError   string           `json:"expect_error"`
	ExpectRequest *behaviorRequest `json:"expect_request"`
}

// behaviorRequest is the expected outgoing HTTP call: method, path (with any
// query string), and the JSON body compared structurally.
type behaviorRequest struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body"`
}

// capturedRequest is one HTTP request the fake transport observed.
type capturedRequest struct {
	method string
	path   string
	body   []byte
}

// decodeBehaviorResult extracts isError and the first content text from a
// marshaled tools/call JSON-RPC response. Decoded through maps because the
// MCP envelope uses camelCase keys (isError) that the repo's JSON tag lint
// rejects on struct tags.
func decodeBehaviorResult(t *testing.T, rawResponse []byte) (bool, string) {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(rawResponse, &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, hasResult := decoded["result"].(map[string]any)
	if !hasResult {
		t.Fatalf("no result in response: %s", rawResponse)
	}

	isError, _ := result["isError"].(bool)

	content, hasContent := result["content"].([]any)
	if !hasContent || len(content) == 0 {
		t.Fatalf("no content in response: %s", rawResponse)
	}

	first, isObject := content[0].(map[string]any)
	if !isObject {
		t.Fatalf("unexpected content shape: %s", rawResponse)
	}

	text, _ := first["text"].(string)

	return isError, text
}

// TestBehaviorConformance replays every shared behavior fixture through the
// real server dispatch path (registration, profile filter, middleware,
// handler, client) with the HTTP transport faked by httptest.
func TestBehaviorConformance(t *testing.T) {
	t.Parallel()

	dir := filepath.Join("..", "..", "..", "testdata", "behavior")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec // fixture path from testdata
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var fixture behaviorFixture
		if err := json.Unmarshal(raw, &fixture); err != nil {
			t.Fatalf("%s: unexpected error: %v", entry.Name(), err)
		}

		for _, testCase := range fixture.Cases {
			t.Run(fixture.Tool+"/"+testCase.Name, func(t *testing.T) {
				t.Parallel()
				runBehaviorCase(t, fixture.Tool, testCase)
			})
		}
	}
}

// runBehaviorCase dispatches one case and checks its expected outcome.
func runBehaviorCase(t *testing.T, toolName string, testCase behaviorCase) {
	t.Helper()

	var (
		captureMu sync.Mutex
		captured  []capturedRequest
	)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		record := capturedRequest{method: r.Method, path: r.URL.RequestURI(), body: body}

		func() {
			captureMu.Lock()
			defer captureMu.Unlock()

			captured = append(captured, record)
		}()

		w.Header().Set("Content-Type", "application/json")

		response := testCase.APIResponse
		if response == nil {
			response = json.RawMessage(`{}`)
		}

		_, _ = w.Write(response)
	}))
	defer apiSrv.Close()

	cfg := fullAccessConfig()
	cfg.Environments[envKeyDefault] = config.EnvironmentConfig{
		Label:  envLabelDefault,
		Linode: config.LinodeConfig{APIURL: apiSrv.URL, Token: tokenShort},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	message, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": toolName, "arguments": testCase.Args},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rawResponse, err := json.Marshal(srv.HandleMessage(t.Context(), message))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	isError, text := decodeBehaviorResult(t, rawResponse)

	if testCase.ExpectError != "" {
		checkBehaviorError(t, isError, text, testCase.ExpectError)

		return
	}

	checkBehaviorRequest(t, isError, text, captured, testCase.ExpectRequest)
}

// checkBehaviorError asserts a local validation failure with the exact bare
// message. Go emits the message unwrapped; the Python runner strips its
// "Error: " prefix so both compare against the same fixture text.
func checkBehaviorError(t *testing.T, isError bool, text, want string) {
	t.Helper()

	if !isError {
		t.Fatalf("isError = false, want true (text %q)", text)
	}

	if text != want {
		t.Errorf("error text = %q, want %q", text, want)
	}
}

// checkBehaviorRequest asserts the handler made exactly the expected HTTP
// call and did not error locally.
func checkBehaviorRequest(t *testing.T, isError bool, text string, captured []capturedRequest, want *behaviorRequest) {
	t.Helper()

	if want == nil {
		t.Fatal("case has neither expect_error nor expect_request")
	}

	if isError {
		t.Fatalf("isError = true, want false (text %q)", text)
	}

	if len(captured) != 1 {
		t.Fatalf("captured %d requests, want 1", len(captured))
	}

	got := captured[0]

	if got.method != want.Method {
		t.Errorf("method = %q, want %q", got.method, want.Method)
	}

	if got.path != want.Path {
		t.Errorf("path = %q, want %q", got.path, want.Path)
	}

	if want.Body == nil {
		return
	}

	var gotBody, wantBody any

	if err := json.Unmarshal(got.body, &gotBody); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := json.Unmarshal(want.Body, &wantBody); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("request body = %s, want %s", got.body, want.Body)
	}
}
