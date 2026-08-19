package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
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

// behaviorCase drives one dispatch. Exactly one outcome field is set:
// ExpectError (an exact local validation failure that forbids HTTP calls),
// ExpectAPIError (a shared substring in an error after at least one HTTP call),
// ExpectRequest (the one HTTP call the handler must make), or ExpectResult (the
// successful response content, compared as parsed JSON so formatting is
// irrelevant). ExpectAPIError uses a substring so language-specific error
// framing stays outside the shared contract.
//
// The fake API answers from APIResponses when present: a map keyed
// "METHOD /path" (no query string, so per-language pagination params don't
// fragment the routing) whose values are the JSON bodies to serve. A request
// with no matching key fails the case, but an unused key does not:
// implementations may fetch equivalent data from different endpoints, and
// the contract these fixtures pin is the OUTPUT, not the fetch pattern.
// Without APIResponses the single APIResponse (or {}) answers every request.
// APIResponseRaw serves exact bytes instead, including empty or malformed JSON,
// and APIStatus overrides HTTP 200 in single-response mode.
//
// A case whose args include dry_run:true additionally asserts that every
// captured request is a GET: a dry run may read whatever it needs to build
// its preview but must never mutate.
type behaviorCase struct {
	Name           string                     `json:"name"`
	Args           map[string]any             `json:"args"`
	APIResponse    json.RawMessage            `json:"api_response"`
	APIResponseRaw *string                    `json:"api_response_raw"`
	APIStatus      *int                       `json:"api_status"`
	APIResponses   map[string]json.RawMessage `json:"api_responses"`
	// APIResponseHeaders answers headers per routed key, for a leg whose result
	// is a header rather than a body: a presigned PUT reports the stored
	// object's entity tag in ETag and sends no JSON at all.
	APIResponseHeaders map[string]map[string]string `json:"api_response_headers"`
	// Files are materialized into a temp directory before the case runs, so a
	// tool taking a local path has something real to read. {{file:name}}
	// substitutes the absolute path into any string argument.
	Files          map[string]behaviorFile `json:"files"`
	Config         *behaviorConfig         `json:"config"`
	ExpectAPIError string                  `json:"expect_api_error"`
	ExpectError    string                  `json:"expect_error"`
	ExpectRequest  *behaviorRequest        `json:"expect_request"`
	ExpectResult   json.RawMessage         `json:"expect_result"`
}

// behaviorFile is one file a case needs on disk. Content writes literal text;
// Size and Fill generate a file too large to spell out in the fixture, which is
// how the over-ceiling case stays a 2 KB file rather than a 5 GiB one.
type behaviorFile struct {
	Content string `json:"content"`
	Fill    string `json:"fill"`
	Size    int    `json:"size"`
}

// behaviorConfig is the narrow config overlay a case may apply. Only the
// Object Storage data-plane block is reachable, by name: a general overlay
// would let a fixture change the profile or the auth it is being tested under,
// and a fixture that can change what it is being tested under is not a fixture.
//
// The keys are snake_case like the rest of the fixture schema, not the
// camelCase the operator's YAML uses: this is a fixture contract, and it is
// read by both runners rather than by the config loader.
type behaviorConfig struct {
	ObjectStorage *behaviorObjectStorage `json:"object_storage"`
}

// behaviorObjectStorage overlays the data-plane budgets a case runs under.
type behaviorObjectStorage struct {
	MaxSinglePartBytes int64 `json:"max_single_part_bytes"`
}

// behaviorRequest is the expected outgoing HTTP call: method, path (with any
// query string), and the JSON body compared structurally.
type behaviorRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	// APISurface is the base the call must go to, "" meaning v4. Absent is what
	// nearly every fixture says, so a tool that moved to another surface without
	// its fixture moving with it fails here rather than passing quietly.
	APISurface string          `json:"api_surface"`
	Body       json.RawMessage `json:"body"`
}

// surface is the declared surface, with the default filled in.
func (r *behaviorRequest) surface() string {
	if r.APISurface == "" {
		return linoderoute.DefaultSurfaceSegment
	}

	return r.APISurface
}

// splitSurface takes the leading version segment off a request path, answering
// it beside the rest. The fake base ends in one, so every request carries it;
// a path arriving without one means the client dropped the base's version.
func splitSurface(path string) (string, string) {
	trimmed := strings.TrimPrefix(path, "/")

	segment, rest, found := strings.Cut(trimmed, "/")
	if !found {
		return segment, "/"
	}

	return segment, "/" + rest
}

// capturedRequest is one HTTP request the fake transport observed.
type capturedRequest struct {
	method  string
	path    string
	surface string
	body    []byte
}

// behaviorOutcomeCount returns the number of usable outcome assertions set on
// a case. Empty error strings do not assert anything and are therefore invalid.
func behaviorOutcomeCount(testCase *behaviorCase) int {
	var count int

	if testCase.ExpectError != "" {
		count++
	}

	if testCase.ExpectAPIError != "" {
		count++
	}

	if testCase.ExpectRequest != nil {
		count++
	}

	if testCase.ExpectResult != nil {
		count++
	}

	return count
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

// callServerTool dispatches one tools/call through the real server and returns
// the answer text with its error flag. One copy of the JSON-RPC envelope keeps
// the tests that drive the server through its own wire from drifting apart.
func callServerTool(
	t *testing.T, srv *server.Server, name string, args map[string]any,
) (bool, string) {
	t.Helper()

	message, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{callNameKey: name, "arguments": args},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rawResponse, err := json.Marshal(srv.HandleMessage(t.Context(), message))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return decodeBehaviorResult(t, rawResponse)
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

		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
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
				runBehaviorCase(t, fixture.Tool, &testCase)
			})
		}
	}
}

func TestBehaviorOutcomeCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testCase behaviorCase
		want     int
	}{
		{name: "empty API error", testCase: behaviorCase{ExpectAPIError: ""}, want: 0},
		{
			name: "multiple outcomes",
			testCase: behaviorCase{
				ExpectAPIError: "response",
				ExpectResult:   json.RawMessage(`false`),
			},
			want: 2,
		},
		{name: "false result", testCase: behaviorCase{ExpectResult: json.RawMessage(`false`)}, want: 1},
		{
			name: "empty errors with result",
			testCase: behaviorCase{
				ExpectError:    "",
				ExpectAPIError: "",
				ExpectResult:   json.RawMessage(`false`),
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := behaviorOutcomeCount(&tt.testCase); got != tt.want {
				t.Errorf("behaviorOutcomeCount() = %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("unmarshaled null result", func(t *testing.T) {
		t.Parallel()

		var testCase behaviorCase
		if err := json.Unmarshal([]byte(`{"expect_result":null}`), &testCase); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if testCase.ExpectResult == nil {
			t.Fatal("ExpectResult = nil, want the JSON null literal")
		}

		if got := behaviorOutcomeCount(&testCase); got != 1 {
			t.Errorf("behaviorOutcomeCount() = %d, want 1", got)
		}
	})
}

// resolveBehaviorResponse picks the body and status for one fake-API request.
// Routed mode (api_responses) matches on "METHOD /path" with the query string
// stripped; a miss serves 404 and reports notFound so the case fails loudly.
func resolveBehaviorResponse(testCase *behaviorCase, method, path string) ([]byte, int, bool) {
	if testCase.APIResponses == nil {
		var response []byte

		switch {
		case testCase.APIResponseRaw != nil:
			response = []byte(*testCase.APIResponseRaw)
		case testCase.APIResponse != nil:
			response = testCase.APIResponse
		default:
			response = []byte(`{}`)
		}

		status := http.StatusOK
		if testCase.APIStatus != nil {
			status = *testCase.APIStatus
		}

		return response, status, true
	}

	response, ok := testCase.APIResponses[method+" "+path]
	if !ok {
		return []byte(`{}`), http.StatusNotFound, false
	}

	return response, http.StatusOK, true
}

// materializeBehaviorFiles writes the case's declared files into a temp
// directory and answers the arguments with {{file:name}} resolved to their
// absolute paths. A tool whose whole job is reading a local file cannot be
// pinned by a fixture that never puts one on disk.
//
// {{file_dir}} names the directory itself, which is what a tool that WRITES a
// file needs: a destination inside a directory the case owns and that no file
// occupies yet.
func materializeBehaviorFiles(
	t *testing.T, testCase *behaviorCase,
) map[string]any {
	t.Helper()

	dir := t.TempDir()
	defer func() {
		testCase.ExpectError = substituteFilePaths(testCase.ExpectError, dir, testCase.Files)
	}()

	paths := make(map[string]string, len(testCase.Files))

	for name, declared := range testCase.Files {
		content := []byte(declared.Content)
		if declared.Size > 0 {
			content = bytes.Repeat([]byte(declared.Fill), declared.Size)
		}

		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		paths[name] = path
	}

	resolved := make(map[string]any, len(testCase.Args))

	for key, value := range testCase.Args {
		text, isText := value.(string)
		if !isText {
			resolved[key] = value

			continue
		}

		for name, path := range paths {
			text = strings.ReplaceAll(text, "{{file:"+name+"}}", path)
		}

		resolved[key] = strings.ReplaceAll(text, "{{file_dir}}", dir)
	}

	return resolved
}

// substituteFilePaths resolves the file tokens in one string. The expected
// error text needs them as much as the arguments do: a refusal that names the
// local path it refused cannot be pinned any other way, because the path is a
// per-run temp directory.
func substituteFilePaths(text, dir string, files map[string]behaviorFile) string {
	for name := range files {
		text = strings.ReplaceAll(text, "{{file:"+name+"}}", filepath.Join(dir, name))
	}

	return strings.ReplaceAll(text, "{{file_dir}}", dir)
}

// substituteAPIBase resolves {{api_base}} in the routed responses. The fake
// server's port is picked at run time, so a fixture that has to name it (a
// presigned URL pointing back at the fake) cannot spell it out.
func substituteAPIBase(
	responses map[string]json.RawMessage, base string,
) map[string]json.RawMessage {
	resolved := make(map[string]json.RawMessage, len(responses))

	for key, value := range responses {
		resolved[key] = json.RawMessage(
			strings.ReplaceAll(string(value), "{{api_base}}", base))
	}

	return resolved
}

// behaviorConfigFor builds the config one case runs under, pointed at the fake
// API and carrying the case's narrow overlay when it declared one.
func behaviorConfigFor(testCase *behaviorCase, apiURL string) *config.Config {
	cfg := fullAccessConfig()

	if testCase.Config != nil && testCase.Config.ObjectStorage != nil {
		cfg.ObjectStorage.MaxSinglePartBytes = testCase.Config.ObjectStorage.MaxSinglePartBytes
	}

	cfg.Environments[envKeyDefault] = config.EnvironmentConfig{
		Label: envLabelDefault,
		// The version segment is what makes the fake base the shape a real
		// apiUrl has, so a surface swap can fire against it.
		Linode: config.LinodeConfig{
			APIURL: apiURL + "/" + linoderoute.DefaultSurfaceSegment,
			Token:  tokenShort,
		},
	}

	return cfg
}

// runBehaviorCase dispatches one case and checks its expected outcome.
func runBehaviorCase(t *testing.T, toolName string, testCase *behaviorCase) {
	t.Helper()

	if count := behaviorOutcomeCount(testCase); count != 1 {
		t.Fatalf("outcome fields set = %d, want exactly 1 non-empty outcome", count)
	}

	if testCase.APIResponse != nil && testCase.APIResponseRaw != nil {
		t.Fatal("api_response and api_response_raw are mutually exclusive")
	}

	if testCase.APIResponses != nil &&
		(testCase.APIResponse != nil || testCase.APIResponseRaw != nil || testCase.APIStatus != nil) {
		t.Fatal("api_responses cannot be combined with single-response fields")
	}

	var (
		captureMu sync.Mutex
		captured  []capturedRequest
		unmatched []string
	)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Match on the escaped path, not r.URL.Path: net/http decodes %2F back to
		// a slash, so a path arg carrying one would match a key the client never
		// sent, and the Python runner reads the raw URL. Keys stay comparable.
		// The fake base carries a version segment the way a real apiUrl does, so
		// the surface swap fires here exactly as it does in the Python runner.
		// Stripping it before matching keeps every fixture's api_responses key
		// and expected path written without one.
		surface, matchPath := splitSurface(r.URL.EscapedPath())
		_, requestURI := splitSurface(r.URL.RequestURI())
		record := capturedRequest{
			method: r.Method, path: requestURI, surface: surface, body: body,
		}
		response, status, matched := resolveBehaviorResponse(testCase, r.Method, matchPath)

		for header, value := range testCase.APIResponseHeaders[r.Method+" "+matchPath] {
			w.Header().Set(header, value)
		}

		func() {
			captureMu.Lock()
			defer captureMu.Unlock()

			captured = append(captured, record)

			if !matched {
				unmatched = append(unmatched, r.Method+" "+matchPath)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(response)
	}))
	defer apiSrv.Close()

	args := materializeBehaviorFiles(t, testCase)

	// Resolved after the server is up because the port is only known now, and
	// the routed responses are what carry a URL pointing back at it.
	if testCase.APIResponses != nil {
		testCase.APIResponses = substituteAPIBase(testCase.APIResponses,
			apiSrv.URL+"/"+linoderoute.DefaultSurfaceSegment)
	}

	srv, err := server.New(behaviorConfigFor(testCase, apiSrv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	message, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": toolName, "arguments": args},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rawResponse, err := json.Marshal(srv.HandleMessage(t.Context(), message))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	isError, text := decodeBehaviorResult(t, rawResponse)

	if len(unmatched) > 0 {
		t.Errorf("requests with no api_responses entry: %s", strings.Join(unmatched, ", "))
	}

	if dryRun, _ := testCase.Args["dry_run"].(bool); dryRun {
		checkBehaviorNoMutation(t, captured)
	}

	switch {
	case testCase.ExpectError != "":
		checkBehaviorError(t, isError, text, captured, testCase.ExpectError)
	case testCase.ExpectAPIError != "":
		checkBehaviorAPIError(t, isError, text, captured, testCase.ExpectAPIError)
	case testCase.ExpectResult != nil:
		checkBehaviorResult(t, isError, text, testCase.ExpectResult)
	default:
		checkBehaviorRequest(t, isError, text, captured, testCase.ExpectRequest)
	}
}

func TestResolveBehaviorResponse(t *testing.T) {
	t.Parallel()

	t.Run("raw empty body and status", func(t *testing.T) {
		t.Parallel()

		var empty string

		status := http.StatusBadRequest
		body, gotStatus, matched := resolveBehaviorResponse(
			&behaviorCase{APIResponseRaw: &empty, APIStatus: &status},
			http.MethodPost,
			"/domains",
		)

		if !matched {
			t.Fatal("matched = false, want true")
		}

		if gotStatus != status {
			t.Errorf("status = %d, want %d", gotStatus, status)
		}

		if len(body) != 0 {
			t.Errorf("body = %q, want empty", body)
		}
	})

	t.Run("raw malformed body", func(t *testing.T) {
		t.Parallel()

		raw := `{"id":`
		body, status, matched := resolveBehaviorResponse(
			&behaviorCase{APIResponseRaw: &raw},
			http.MethodPost,
			"/domains",
		)

		if !matched {
			t.Fatal("matched = false, want true")
		}

		if status != http.StatusOK {
			t.Errorf("status = %d, want %d", status, http.StatusOK)
		}

		if string(body) != raw {
			t.Errorf("body = %q, want %q", body, raw)
		}
	})
}

// checkBehaviorNoMutation asserts a dry-run case only ever read: the walk may
// GET whatever it needs for the preview, but any other verb is a mutation the
// dry-run contract forbids.
func checkBehaviorNoMutation(t *testing.T, captured []capturedRequest) {
	t.Helper()

	for _, request := range captured {
		if request.method != http.MethodGet {
			t.Errorf("dry-run case issued %s %s; only GET is allowed", request.method, request.path)
		}
	}
}

// checkBehaviorResult asserts a successful response whose content equals the
// expected JSON value. Comparison happens on parsed values so whitespace and
// key order are irrelevant; both languages must produce the same content for
// the same fixture, which is the cross-language contract this pins.
func checkBehaviorResult(t *testing.T, isError bool, text string, want json.RawMessage) {
	t.Helper()

	if isError {
		t.Fatalf("isError = true, want false (text %q)", text)
	}

	var gotValue, wantValue any

	if err := json.Unmarshal([]byte(text), &gotValue); err != nil {
		t.Fatalf("result is not JSON: %v\nresult text:\n%s", err, text)
	}

	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("result mismatch\ngot:\n%s\nwant:\n%s", text, want)
	}
}

// checkBehaviorError asserts a local validation failure with the exact bare
// message and no HTTP request. Go emits the message unwrapped; the Python
// runner adds an "Error: " prefix before comparing with the fixture text.
func checkBehaviorError(t *testing.T, isError bool, text string, captured []capturedRequest, want string) {
	t.Helper()

	if !isError {
		t.Fatalf("isError = false, want true (text %q)", text)
	}

	if text != want {
		t.Errorf("error text = %q, want %q", text, want)
	}

	if len(captured) != 0 {
		t.Errorf("captured %d requests, want 0", len(captured))
	}
}

// checkBehaviorAPIError asserts an error containing the shared failure reason
// after the handler made at least one HTTP request.
func checkBehaviorAPIError(t *testing.T, isError bool, text string, captured []capturedRequest, want string) {
	t.Helper()

	if !isError {
		t.Fatalf("isError = false, want true (text %q)", text)
	}

	if !strings.Contains(text, want) {
		t.Errorf("error text = %q, want substring %q", text, want)
	}

	if len(captured) == 0 {
		t.Fatal("captured 0 requests, want at least 1")
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

	if got.surface != want.surface() {
		t.Errorf("api surface = %q, want %q", got.surface, want.surface())
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
