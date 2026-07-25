package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

const (
	wantPaginationRejection = "page must be an integer"
	pageArg                 = "page"
	pageSizeArg             = "page_size"
	callNameKey             = "name"

	// Tools whose other arguments take a format this test cannot synthesize
	// (a UUID, a "private/12345" image id) validate those first, so they
	// report that instead of the pagination message. They still must reject
	// the call without reaching the API, which is the assertion that holds
	// for every tool. This floor keeps the specific-message assertion honest:
	// if the shared reader stopped being wired up, the count would collapse
	// rather than a hand-maintained skip list going quietly stale.
	minToolsRejectingWithPaginationMessage = 60
)

// paginatedToolArgs builds the arguments for one surface case: placeholder
// values for the tool's required params (so validation reaches pagination
// rather than stopping on a missing id) plus the bad page under test. Types
// and enums come from the tool's own schema, so a new required param needs no
// edit here.
func paginatedToolArgs(schema json.RawMessage) (map[string]any, bool) {
	var decoded struct {
		Properties map[string]struct {
			Type string   `json:"type"`
			Enum []string `json:"enum"`
		} `json:"properties"`
		Required []string `json:"required"`
	}

	if err := json.Unmarshal(schema, &decoded); err != nil {
		return nil, false
	}

	_, hasPage := decoded.Properties[pageArg]

	_, hasPageSize := decoded.Properties[pageSizeArg]
	if !hasPage || !hasPageSize {
		return nil, false
	}

	args := map[string]any{pageArg: "x"}

	for _, name := range decoded.Required {
		if name == pageArg || name == pageSizeArg {
			continue
		}

		property := decoded.Properties[name]

		switch {
		case len(property.Enum) > 0:
			args[name] = property.Enum[0]
		case property.Type == "integer" || property.Type == "number":
			args[name] = float64(1)
		case property.Type == "boolean":
			// Never true: a paginated tool that also takes a confirm flag must
			// not be able to execute anything from this test.
			args[name] = false
		default:
			args[name] = "1"
		}
	}

	return args, true
}

// TestEveryPaginatedToolRejectsBadPagination walks the whole registered
// surface rather than naming tools, so a family that adopts pagination later
// is covered without touching this test.
//
// It is the guard the per-family readers used to provide implicitly: 44
// near-identical readers collapsed into one shared reader, and nothing else
// proves each tool still routes through it. A tool that grows a private reader
// again, or wires the shared one in after the client is built, fails here.
//
// Every tool must reject a non-integer page without issuing a request; the
// count that rejects with the shared pagination message specifically must stay
// at or above minToolsRejectingWithPaginationMessage.
func TestEveryPaginatedToolRejectsBadPagination(t *testing.T) {
	t.Parallel()

	var (
		requestMu sync.Mutex
		requests  int
	)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestMu.Lock()
		requests++
		requestMu.Unlock()

		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"reason":"pagination validation was skipped"}]}`))
	}))
	t.Cleanup(apiSrv.Close)

	cfg := fullAccessConfig()
	cfg.Environments[envKeyDefault] = config.EnvironmentConfig{
		Label:  envLabelDefault,
		Linode: config.LinodeConfig{APIURL: apiSrv.URL, Token: tokenShort},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var checked, withPaginationMessage int

	// Sequential on purpose: each case asserts that its own call issued no
	// request, which a shared counter cannot attribute under parallelism.
	for _, info := range srv.AllToolInfos() {
		args, ok := paginatedToolArgs(info.RawInputSchema)
		if !ok {
			continue
		}

		checked++

		before := requests

		text, isError := callPaginatedTool(t, srv, info.Name, args)

		if !isError {
			t.Errorf("%s: page=%q accepted, want a validation error", info.Name, args[pageArg])
		}

		if requests != before {
			t.Errorf("%s: issued a request with an invalid page", info.Name)
		}

		if strings.Contains(text, wantPaginationRejection) {
			withPaginationMessage++
		}
	}

	if checked < minToolsRejectingWithPaginationMessage {
		t.Errorf("checked %d paginated tools, want at least %d",
			checked, minToolsRejectingWithPaginationMessage)
	}

	if withPaginationMessage < minToolsRejectingWithPaginationMessage {
		t.Errorf("%d of %d paginated tools reported the shared pagination message, want at least %d",
			withPaginationMessage, checked, minToolsRejectingWithPaginationMessage)
	}
}

// callPaginatedTool dispatches one tools/call through the server and returns
// the result text with its error flag.
func callPaginatedTool(t *testing.T, srv *server.Server, name string, args map[string]any) (string, bool) {
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

	isError, text := decodeBehaviorResult(t, rawResponse)

	return text, isError
}
