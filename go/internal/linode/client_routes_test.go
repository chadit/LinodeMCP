package linode_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Response bodies the route cases serve. Each one carries a single probe value
// so the case can prove the client decoded the payload rather than handing back
// a zero value.
const (
	clientRouteEmptyObject = "{}"
	clientRouteObjUsername = "{\"username\":\"probe-value\"}"
)

// Request paths and probe results the cases below share.
const (
	clientRoutePathProfile                  = "/profile"
	clientRoutePathProfilePhoneNumberVerify = "/profile/phone-number/verify"
	clientRouteProbeValue                   = "probe-value"
)

// clientRouteCase pins one linode.Client method to the request it puts on the
// wire and the value it decodes back out of the response.
//
// Every Client method funnels through the same makeRequest/handleResponse pair,
// so covering them one at a time says little that the next method does not
// repeat. What is worth pinning is the part that differs: the verb and path a
// method sends, and whether it decodes the body it gets back. A method that
// quietly moves to another path, flips its verb, or stops decoding its payload
// fails here instead of against the real API.
type clientRouteCase struct {
	call     func(ctx context.Context, client *linode.Client) (any, error)
	want     any
	name     string
	wantVerb string
	wantPath string
	response string
}

// runClientRouteCases serves each case's response and checks the verb, path,
// and decoded probe value its call reports back.
func runClientRouteCases(t *testing.T, cases []clientRouteCase) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.wantVerb {
					t.Errorf("request verb = %v, want %v", r.Method, tt.wantVerb)
				}

				if r.URL.Path != tt.wantPath {
					t.Errorf("request path = %v, want %v", r.URL.Path, tt.wantPath)
				}

				if r.Header.Get("Authorization") != authHeaderTestToken {
					t.Errorf("authorization header = %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
				}

				w.Header().Set("Content-Type", tcApplicationJSON)

				if _, err := io.WriteString(w, tt.response); err != nil {
					t.Errorf("write response body: %v", err)
				}
			}))
			defer srv.Close()

			client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

			got, err := tt.call(t.Context(), client)
			if err != nil {
				t.Fatalf("call returned error: %v", err)
			}

			if got != tt.want {
				t.Errorf("decoded probe = %v, want %v", got, tt.want)
			}
		})
	}
}

// clientRouteError adds the harness's own context to a Client error so a case
// body reports the failed call instead of handing the upstream error straight
// back to the runner.
func clientRouteError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("linode client call failed: %w", err)
}

// clientRouteProbe reads probe only once the call that produced it succeeded,
// so a failed call never reaches a field access on a nil result.
func clientRouteProbe(err error, probe func() any) (any, error) {
	if err != nil {
		return nil, clientRouteError(err)
	}

	return probe(), nil
}
