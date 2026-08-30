package linode_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// emptyObjectBody is the error body the API sends when it has no errors[] to
// report, which is what routes the client onto its status-derived messages.
const emptyObjectBody = `{}`

// TestCallRouteBodyRefusesAPayloadItCannotMarshal pins that a body the JSON
// encoder rejects never leaves the process: the caller gets the encoder's
// complaint and the server sees no request, so a half-built mutation cannot
// reach the API with a body the client could not describe.
func TestCallRouteBodyRefusesAPayloadItCannotMarshal(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, fastRetryOpts()...)

	err := client.CallRouteBody(t.Context(), phoneVerifyTool, nil, map[string]any{"otp_code": math.NaN()})

	if _, ok := errors.AsType[*json.UnsupportedValueError](err); !ok {
		t.Errorf("error = %v, want it to wrap the encoder's *json.UnsupportedValueError", err)
	}

	if requestCount.Load() != 0 {
		t.Errorf("server saw %d requests, want none when the body cannot be marshaled", requestCount.Load())
	}
}

// TestCallRouteJSONRefusesAnUnparsableBaseURL covers the configured API URL
// that no request can be built from. The failure has to name the URL rather
// than surface as a transport error, and it must not be replayed, because the
// same string fails the same way on every attempt.
func TestCallRouteJSONRefusesAnUnparsableBaseURL(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://bad host/v4", "token", nil, fastRetryOpts()...)

	_, err := readProfile(t.Context(), client)

	urlErr, ok := errors.AsType[*url.Error](err)
	if !ok {
		t.Fatalf("error = %v, want it to wrap the *url.Error the parser produced", err)
	}

	if urlErr.Op != "parse" {
		t.Errorf("urlErr.Op = %q, want %q: the URL must be refused before any request is built", urlErr.Op, "parse")
	}
}

// TestCallRouteJSONReportsATruncatedBody covers a response cut short by the
// server: the declared Content-Length promises more bytes than arrive. The
// caller must hear that the body could not be read, wrapping io.ErrUnexpectedEOF,
// rather than a decode complaint about a partial document.
func TestCallRouteJSONReportsATruncatedBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write([]byte(`{"username":"cut`)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := readProfile(t.Context(), client)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("error = %v, want it to wrap io.ErrUnexpectedEOF", err)
	}
}

// TestErrorResponsesWithoutAnErrorsArrayUseStatusMessages walks the fallback
// wording the client supplies when the API sends a status but no errors[]
// body. Each row is a distinct status the switch names, plus the generic arm,
// so a reworded or dropped case fails on its own row.
func TestErrorResponsesWithoutAnErrorsArrayUseStatusMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		wantMessage string
		status      int
	}{
		{
			name:        "401 names the token",
			status:      http.StatusUnauthorized,
			wantMessage: "authentication failed, check your API token",
		},
		{
			name:        "404 falls to the generic status message",
			status:      http.StatusNotFound,
			wantMessage: "API request failed with status 404",
		},
		{
			name:        "502 is not read as the 500 case",
			status:      http.StatusBadGateway,
			wantMessage: "API request failed with status 502",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)

				if _, err := w.Write([]byte(emptyObjectBody)); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}))
			defer srv.Close()

			client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

			_, err := readProfile(t.Context(), client)

			apiErr, ok := errors.AsType[*linode.APIError](err)
			if !ok {
				t.Fatalf("error = %v, want an *linode.APIError", err)
			}

			if apiErr.StatusCode != tt.status {
				t.Errorf("apiErr.StatusCode = %d, want %d", apiErr.StatusCode, tt.status)
			}

			if apiErr.Message != tt.wantMessage {
				t.Errorf("apiErr.Message = %q, want %q", apiErr.Message, tt.wantMessage)
			}
		})
	}
}

// TestRateLimitRetryAfterAsAnHTTPDateBecomesADelay covers the second form the
// header may take. RFC 9110 allows an HTTP-date as well as a delay in seconds,
// and the retry loop only honors the hint if it reaches APIError.RetryAfter as
// a duration measured from now.
func TestRateLimitRetryAfterAsAnHTTPDateBecomesADelay(t *testing.T) {
	t.Parallel()

	const hint = 30 * time.Second

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", time.Now().Add(hint).UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusTooManyRequests)

		if _, err := w.Write([]byte(emptyObjectBody)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := readProfile(t.Context(), client)

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want an *linode.APIError", err)
	}

	// HTTP-dates carry whole seconds and the round trip takes wall time, so
	// the parsed delay lands a little under the hint; a delay of zero or one
	// far shorter than the hint means the date was not honored.
	if apiErr.RetryAfter <= hint-5*time.Second || apiErr.RetryAfter > hint {
		t.Errorf("apiErr.RetryAfter = %v, want a delay just under %v from the HTTP-date", apiErr.RetryAfter, hint)
	}

	if !strings.Contains(apiErr.Message, "retry after") {
		t.Errorf("apiErr.Message = %q, want it to carry the retry-after hint", apiErr.Message)
	}
}

// TestRateLimitWithAnUnparsableRetryAfterCarriesNoHint pins the safe
// fallback: a Retry-After the client cannot read as seconds or as a date
// leaves RetryAfter at zero, so the retry loop uses its own backoff instead
// of a garbage delay, and the message stops short of quoting a hint.
func TestRateLimitWithAnUnparsableRetryAfterCarriesNoHint(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "soon")
		w.WriteHeader(http.StatusTooManyRequests)

		if _, err := w.Write([]byte(emptyObjectBody)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := readProfile(t.Context(), client)

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want an *linode.APIError", err)
	}

	if apiErr.RetryAfter != 0 {
		t.Errorf("apiErr.RetryAfter = %v, want 0 for a Retry-After that parses as neither seconds nor a date", apiErr.RetryAfter)
	}

	if apiErr.Message != "rate limit exceeded, try again later" {
		t.Errorf("apiErr.Message = %q, want the hint-free rate limit message", apiErr.Message)
	}
}

// apiRecord is one round trip as the client reported it to the recorder.
type apiRecord struct {
	endpoint string
	method   string
	duration float64
	status   int
}

// recordingAPIRecorder collects what the client reports so a test can read it
// back; it stands in for the observability instance the server injects.
type recordingAPIRecorder struct {
	records []apiRecord
	mu      sync.Mutex
}

func (r *recordingAPIRecorder) RecordAPIRequest(_ context.Context, endpoint, method string, status int, duration float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.records = append(r.records, apiRecord{endpoint: endpoint, method: method, status: status, duration: duration})
}

func (r *recordingAPIRecorder) snapshot() []apiRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]apiRecord(nil), r.records...)
}

// TestRecorderInTheContextSeesEachRoundTrip pins the seam the server uses to
// count API calls: a recorder placed in the context receives the path with
// its query stripped, the method, the status, and a measured duration.
func TestRecorderInTheContextSeesEachRoundTrip(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != tcPage2PageSize50 {
			t.Errorf("r.URL.RawQuery = %q, want %q", r.URL.RawQuery, tcPage2PageSize50)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(`{"id":5,"domain":"example.com"}`)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	recorder := &recordingAPIRecorder{}
	ctx := linode.WithAPIRecorder(t.Context(), recorder)
	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	if err := client.CallProtoRouteQuery(ctx, domainGetTool, []any{5}, tcPage2PageSize50, &linodev1.Domain{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	records := recorder.snapshot()
	if len(records) != 1 {
		t.Fatalf("recorder saw %d round trips, want 1", len(records))
	}

	got := records[0]

	if got.endpoint != "/domains/5" {
		t.Errorf("recorded endpoint = %q, want %q with the query stripped", got.endpoint, "/domains/5")
	}

	if got.method != http.MethodGet {
		t.Errorf("recorded method = %q, want %q", got.method, http.MethodGet)
	}

	if got.status != http.StatusOK {
		t.Errorf("recorded status = %d, want %d", got.status, http.StatusOK)
	}

	if got.duration <= 0 {
		t.Errorf("recorded duration = %v, want a positive measurement", got.duration)
	}
}

// TestRecorderSeesAFailedRoundTripAsStatusZero covers the transport failure
// the recorder must still hear about: with no response there is no status,
// so the round trip is reported with zero, and the caller still gets the
// network error.
func TestRecorderSeesAFailedRoundTripAsStatusZero(t *testing.T) {
	t.Parallel()

	// A server that has already closed guarantees a released loopback port
	// that refuses the connection, without guessing at a free port number.
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL

	closed.Close()

	recorder := &recordingAPIRecorder{}
	ctx := linode.WithAPIRecorder(t.Context(), recorder)
	client := linode.NewClient(closedURL, "token", nil, linode.WithMaxRetries(0))

	_, err := readProfile(ctx, client)
	if err == nil {
		t.Fatal("expected a network error against a closed port, got nil")
	}

	records := recorder.snapshot()
	if len(records) != 1 {
		t.Fatalf("recorder saw %d round trips, want 1", len(records))
	}

	if records[0].status != 0 {
		t.Errorf("recorded status = %d, want 0 when no response arrived", records[0].status)
	}

	if records[0].endpoint != "/profile" {
		t.Errorf("recorded endpoint = %q, want %q", records[0].endpoint, "/profile")
	}
}

// TestWithAPIRecorderIgnoresANilRecorder pins the convenience the server
// relies on before metrics are wired: a nil recorder leaves the context as
// it was, so the client records nothing rather than calling through nil.
func TestWithAPIRecorderIgnoresANilRecorder(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(`{"username":"probe"}`)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	base := t.Context()

	ctx := linode.WithAPIRecorder(base, nil)
	if ctx != base {
		t.Error("WithAPIRecorder(nil) derived a new context, want the original handed back")
	}

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	if _, err := readProfile(ctx, client); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount.Load() != 1 {
		t.Errorf("server saw %d requests, want 1", requestCount.Load())
	}
}
