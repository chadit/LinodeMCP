package linode_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientListProfileDevicesProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileDevices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices)
		}

		if r.URL.RawQuery != "page=2&page_size=50" {
			t.Errorf("r.URL.RawQuery = %q, want %q", r.URL.RawQuery, "page=2&page_size=50")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, err := w.Write([]byte(`{"data":[{"id":12345,"user_agent":"test-agent"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

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

	if devices[0].GetUserAgent() != "test-agent" {
		t.Errorf("devices[0].GetUserAgent() = %q, want %q", devices[0].GetUserAgent(), "test-agent")
	}
}

func TestClientListProfileDevicesProtoRejectsMissingOrNullData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "missing", body: `{}`},
		{name: "null", body: `{"data":null}`},
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

			client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

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

func TestClientListProfileDevicesProtoRequestError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("://", "my-token", nil, linode.WithMaxRetries(0))

	devices, err := client.ListProfileDevicesProto(t.Context(), 2, 50)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if devices != nil {
		t.Errorf("devices = %v, want nil", devices)
	}
}
