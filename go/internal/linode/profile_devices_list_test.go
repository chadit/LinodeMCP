package linode_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientListProfileDevicesProtoRoute(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileDevices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices)
		}

		if r.URL.RawQuery != tcPage2PageSize50 {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, tcPage2PageSize50)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(
			`{"data":[{"id":12345,"user_agent":"Mozilla/5.0"}],"page":2,"pages":2,"results":2}`,
		)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	devices, err := client.ListProfileDevicesProto(t.Context(), 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 1 {
		t.Fatalf("len(devices) = %d, want %d", len(devices), 1)
	}

	if devices[0].GetId() != 12345 {
		t.Errorf("devices[0].GetId() = %d, want %d", devices[0].GetId(), 12345)
	}

	if devices[0].GetUserAgent() != "Mozilla/5.0" {
		t.Errorf("devices[0].GetUserAgent() = %q, want %q", devices[0].GetUserAgent(), "Mozilla/5.0")
	}
}

// TestClientListProfileDevicesProtoAcceptsEmptyData pins the narrow half of the
// required-data contract: an account with no trusted devices answers with an
// empty data array, and that is a real answer rather than a malformed body.
func TestClientListProfileDevicesProtoAcceptsEmptyData(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(`{"data":[],"page":1,"pages":1,"results":0}`)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	devices, err := client.ListProfileDevicesProto(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 0 {
		t.Errorf("len(devices) = %d, want %d", len(devices), 0)
	}
}

// TestClientListProfileDevicesProtoRejectsMissingData covers the bodies the
// shared decode path would otherwise report as zero trusted devices. Answering
// "no remembered browser sessions" from a body that never listed any is the
// wrong answer on this surface, so the call fails instead.
func TestClientListProfileDevicesProtoRejectsMissingData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "absent", body: `{"page":1,"pages":1,"results":0}`},
		{name: "null", body: `{"data":null,"page":1,"pages":1,"results":0}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tcApplicationJSON)

				if _, err := w.Write([]byte(test.body)); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}))
			t.Cleanup(srv.Close)

			client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

			devices, err := client.ListProfileDevicesProto(t.Context(), 2, 50)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if devices != nil {
				t.Errorf("devices = %v, want nil", devices)
			}
		})
	}
}

func TestClientListProfileDevicesProtoRejectsMalformedEnvelope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "top level array", body: `[]`},
		{name: "top level null", body: `null`},
		{name: "top level string", body: `"invalid"`},
		{name: "object data member", body: `{"data":{},"page":1,"pages":1,"results":0}`},
		{name: "non object element", body: `{"data":[1],"page":1,"pages":1,"results":1}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tcApplicationJSON)

				if _, err := w.Write([]byte(test.body)); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}))
			t.Cleanup(srv.Close)

			client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

			devices, err := client.ListProfileDevicesProto(t.Context(), 0, 0)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if devices != nil {
				t.Errorf("devices = %v, want nil", devices)
			}
		})
	}
}

func TestClientListProfileDevicesProtoWrapsRequestError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("://", "test-token", nil, linode.WithMaxRetries(0))

	devices, err := client.ListProfileDevicesProto(t.Context(), 2, 50)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if devices != nil {
		t.Errorf("devices = %v, want nil", devices)
	}
}
