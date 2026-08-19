package linode_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// An empty type ID would collapse "/linode/types/{type_id}" to the collection
// route, so the typed getter refuses before anything is sent and reports the
// argument class, not a network failure.
func TestClientGetTypeEmptyIDReachesNoRequest(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		t.Errorf("r.URL.Path = %v, want no request at all", r.URL.Path)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetType(t.Context(), "")
	if !errors.Is(err, linoderoute.ErrEmptyValue) {
		t.Fatalf("GetType(\"\") error = %v, want ErrEmptyValue", err)
	}

	argErr, ok := errors.AsType[*linode.ArgumentError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.ArgumentError", err)
	}

	if argErr.Operation != "GetType" {
		t.Errorf("argErr.Operation = %v, want GetType", argErr.Operation)
	}

	// The message is the only surface a tool caller sees, so the shape that
	// names the operation before the cause is pinned here.
	wantMessage := "invalid arguments for GetType: " + argErr.Err.Error()
	if argErr.Error() != wantMessage {
		t.Errorf("argErr.Error() = %q, want %q", argErr.Error(), wantMessage)
	}

	if requestCount.Load() != int32(0) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
	}
}

// A body that ends before its declared length is a transport failure the proto
// read path has to surface, not a decode complaint over half a message.
func TestClientGetProfileAppProtoReportsATruncatedBody(t *testing.T) {
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

	_, err := client.GetProfileAppProto(t.Context(), 1)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("GetProfileAppProto() error = %v, want an unexpected-EOF read failure", err)
	}
}
