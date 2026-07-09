package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientGetInstanceTransferSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != endpointInstanceTransferPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), endpointInstanceTransferPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{"billable": 0, "quota": 2000, "used": 22956600198}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetInstanceTransferProto(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.GetBillable() != 0 {
		t.Errorf("result.GetBillable() = %v, want %v", result.GetBillable(), 0)
	}

	if result.GetQuota() != 2000 {
		t.Errorf("result.GetQuota() = %v, want %v", result.GetQuota(), 2000)
	}

	if result.GetUsed() != int64(22956600198) {
		t.Errorf("result.GetUsed() = %v, want %v", result.GetUsed(), int64(22956600198))
	}
}

func TestClientGetInstanceTransferRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://example.invalid", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceTransferProto(t.Context(), 0)

	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}
}

func TestClientGetInstanceTransferAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceTransferPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceTransferPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceTransferProto(t.Context(), 123)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientGetInstanceTransferRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call == 1 {
			http.Error(w, "temporary", http.StatusInternalServerError)

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceTransferPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceTransferPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{"used": 123}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetInstanceTransferProto(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.GetUsed() != int64(123) {
		t.Errorf("result.GetUsed() = %v, want %v", result.GetUsed(), int64(123))
	}

	if calls.Load() < int32(2) {
		t.Errorf("calls.Load() = %v, want >= %v", calls.Load(), int32(2))
	}
}

func TestClientGetInstanceTransferByYearMonthSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != endpointInstanceTransferYearMonth {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), endpointInstanceTransferYearMonth)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"bytes_in":    30471077120,
			"bytes_out":   22956600198,
			"bytes_total": 53427677318,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetInstanceTransferByYearMonthProto(t.Context(), 123, 2024, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.GetBytesIn() != int64(30471077120) {
		t.Errorf("result.GetBytesIn() = %v, want %v", result.GetBytesIn(), int64(30471077120))
	}

	if result.GetBytesOut() != int64(22956600198) {
		t.Errorf("result.GetBytesOut() = %v, want %v", result.GetBytesOut(), int64(22956600198))
	}

	if result.GetBytesTotal() != int64(53427677318) {
		t.Errorf("result.GetBytesTotal() = %v, want %v", result.GetBytesTotal(), int64(53427677318))
	}
}

func TestClientGetInstanceTransferByYearMonthAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceTransferYearMonth {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceTransferYearMonth)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceTransferByYearMonthProto(t.Context(), 123, 2024, 1)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientGetInstanceTransferByYearMonthRetriesTransientError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response writer should support hijacking")

				return
			}

			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("hijack should succeed: %v", err)

				return
			}

			if err := conn.Close(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceTransferYearMonth {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceTransferYearMonth)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{"bytes_total": 4}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetInstanceTransferByYearMonthProto(t.Context(), 123, 2024, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.GetBytesTotal() != int64(4) {
		t.Errorf("result.GetBytesTotal() = %v, want %v", result.GetBytesTotal(), int64(4))
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}

func TestClientGetInstanceTransferByYearMonthValidatesPathParams(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://example.invalid", "my-token", nil, linode.WithMaxRetries(0))

	cases := []struct {
		name    string
		id      int
		year    int
		mon     int
		wantErr error
	}{
		{name: "zero linode id", id: 0, year: 2024, mon: 1, wantErr: linode.ErrLinodeIDPositive},
		{name: "zero year", id: 123, year: 0, mon: 1, wantErr: linode.ErrTransferYearPositive},
		{name: "zero month", id: 123, year: 2024, mon: 0, wantErr: linode.ErrTransferMonthRange},
		{name: "month too large", id: 123, year: 2024, mon: 13, wantErr: linode.ErrTransferMonthRange},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := client.GetInstanceTransferByYearMonthProto(t.Context(), tt.id, tt.year, tt.mon)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
