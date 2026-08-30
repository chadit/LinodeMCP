package linode_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// profileAppGetTool is the single-resource read both tests below drive.
const profileAppGetTool = "linode_profile_app_get"

// An empty path value would collapse "/profile/apps/{app_id}" to the
// collection route, so the routed read refuses before anything is sent and
// reports the argument class, not a network failure.
func TestCallProtoRouteQueryEmptyPathValueReachesNoRequest(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		t.Errorf("r.URL.Path = %v, want no request at all", r.URL.Path)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.CallProtoRouteQuery(t.Context(), profileAppGetTool, []any{""}, "", &linodev1.ProfileApp{})
	if !errors.Is(err, linoderoute.ErrEmptyValue) {
		t.Fatalf("CallProtoRouteQuery(\"\") error = %v, want ErrEmptyValue", err)
	}

	argErr, ok := errors.AsType[*linode.ArgumentError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.ArgumentError", err)
	}

	if argErr.Operation != profileAppGetTool {
		t.Errorf("argErr.Operation = %v, want %v", argErr.Operation, profileAppGetTool)
	}

	// The message is the only surface a tool caller sees, so the shape that
	// names the operation before the cause is pinned here.
	wantMessage := "invalid arguments for linode_profile_app_get: " + argErr.Err.Error()
	if argErr.Error() != wantMessage {
		t.Errorf("argErr.Error() = %q, want %q", argErr.Error(), wantMessage)
	}

	if requestCount.Load() != int32(0) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
	}
}

// A body cut short of its declared Content-Length is a read failure, not an
// empty answer, so the proto read reports it rather than decoding a partial
// message.
func TestCallProtoRouteQueryReportsATruncatedBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "500")

		if _, err := w.Write([]byte(`{"id":`)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.CallProtoRouteQuery(t.Context(), profileAppGetTool, []any{1}, "", &linodev1.ProfileApp{})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("CallProtoRouteQuery() error = %v, want an unexpected-EOF read failure", err)
	}
}
