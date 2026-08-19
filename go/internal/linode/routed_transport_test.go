package linode_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// routedTransportToken never leaves the process, so it only has to be non-empty.
const routedTransportToken = "routed-transport-token"

// routedTransportCase pins one Client method to the error it reports when the
// connection fails. A method that skips wrapRequestError returns a bare
// *url.Error carrying no operation name, and the retry layer loses its only
// signal for whether a replay is safe. The happy path never notices, so every
// routed method gets a row here.
type routedTransportCase struct {
	call func(ctx context.Context, client *linode.Client) error
	name string

	// operation is set only when the error label differs from the case name, as
	// happens when a proto variant shares the plain method's label.
	operation string
}

func (tt routedTransportCase) wantOperation() string {
	if tt.operation != "" {
		return tt.operation
	}

	return tt.name
}

// runRoutedTransportCases points each case at a closed listener and checks that
// the call comes back as a *linode.NetworkError naming its operation.
func runRoutedTransportCases(t *testing.T, cases []routedTransportCase) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// A client per case: a shared circuit breaker would trip partway
			// through and later cases would see an open-circuit error instead
			// of the transport one.
			err := tt.call(t.Context(), newUnreachableRoutedClient(t))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			netErr, ok := errors.AsType[*linode.NetworkError](err)
			if !ok {
				t.Fatalf("err = %T (%v), want *linode.NetworkError", err, err)
			}

			if netErr.Operation != tt.wantOperation() {
				t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, tt.wantOperation())
			}

			if netErr.Unwrap() == nil {
				t.Error("netErr.Unwrap() = nil, want the transport error")
			}
		})
	}
}

// newUnreachableRoutedClient points a client at an already-closed server so the
// next connection attempt fails at the transport layer. Retries are off so the
// failure surfaces on the first try.
func newUnreachableRoutedClient(t *testing.T) *linode.Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	return linode.NewClient(baseURL, routedTransportToken, nil, linode.WithMaxRetries(0))
}

// Path values chosen to pass each method's own argument guard. A rejected value
// short-circuits ahead of the transport, so the case would assert a validation
// error instead of the wrap.
const (
	testIPv4      = "198.51.100.7"
	testIPv6Range = "2001:db8::/64"
)
