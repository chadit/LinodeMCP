package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientListInstanceConfigInterfacesProtoContract(t *testing.T) {
	t.Parallel()

	const natAddress = "192.0.2.10"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/configs/456/interfaces" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/456/interfaces")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)

			return
		}

		if len(body) != 0 {
			t.Errorf("request body = %q, want empty", body)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("Authorization = %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		_, err = w.Write([]byte(`[
			{"id":202,"active":true,"purpose":"vpc","label":"eth1","ipam_address":"10.0.0.1/24","primary":true,"subnet_id":55,"vpc_id":77,"ipv4":{"nat_1_1":"192.0.2.10","vpc":"10.0.0.5"},"ip_ranges":["2001:db8::/64"]},
			{"id":101,"active":false,"purpose":"public","primary":false,"ip_ranges":[]}
		]`))
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceConfigInterfacesProto(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %v, want %v", len(got), 2)
	}

	if got[0].GetId() != 202 || got[1].GetId() != 101 {
		t.Errorf("interface order = [%d, %d], want [202, 101]", got[0].GetId(), got[1].GetId())
	}

	if got[0].GetLabel() != "eth1" || got[0].GetIpamAddress() != "10.0.0.1/24" {
		t.Errorf("optional strings = (%q, %q), want (%q, %q)", got[0].GetLabel(), got[0].GetIpamAddress(), "eth1", "10.0.0.1/24")
	}

	if got[0].GetSubnetId() != 55 || got[0].GetVpcId() != 77 {
		t.Errorf("network ids = (%d, %d), want (%d, %d)", got[0].GetSubnetId(), got[0].GetVpcId(), 55, 77)
	}

	if got[0].GetIpv4().GetNat_1_1() != natAddress || got[0].GetIpv4().GetVpc() != "10.0.0.5" {
		t.Errorf("ipv4 = %v, want populated NAT and VPC addresses", got[0].GetIpv4())
	}

	if !reflect.DeepEqual(got[0].GetIpRanges(), []string{"2001:db8::/64"}) {
		t.Errorf("ip_ranges = %v, want %v", got[0].GetIpRanges(), []string{"2001:db8::/64"})
	}
}

func TestClientListInstanceConfigInterfacesProtoRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	// No server is needed: validation must return its sentinel before attempting
	// the deliberately unusable upstream URL.
	client := linode.NewClient("invalid://upstream", "test-token", nil, linode.WithMaxRetries(0))

	for _, tt := range []struct {
		name     string
		linodeID int
		configID int
		wantErr  error
	}{
		{name: "non-positive linode id", linodeID: 0, configID: 456, wantErr: linode.ErrLinodeIDPositive},
		{name: "negative linode id", linodeID: -1, configID: 456, wantErr: linode.ErrLinodeIDPositive},
		{name: "non-positive config id", linodeID: 123, configID: 0, wantErr: linode.ErrConfigIDPositive},
		{name: "negative config id", linodeID: 123, configID: -1, wantErr: linode.ErrConfigIDPositive},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := client.ListInstanceConfigInterfacesProto(t.Context(), tt.linodeID, tt.configID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestClientListInstanceConfigInterfacesProtoAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListInstanceConfigInterfacesProto(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientListInstanceConfigInterfacesProtoRetriesTransientGET(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))

			return
		}

		_, _ = w.Write([]byte(`[{"id":101,"purpose":"public"}]`))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"test-token",
		nil,
		linode.WithMaxRetries(1),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithMaxDelay(time.Millisecond),
		linode.WithJitter(false),
	)

	got, err := client.ListInstanceConfigInterfacesProto(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 || got[0].GetId() != 101 {
		t.Errorf("got = %v, want one interface with id 101", got)
	}

	if calls.Load() != 2 {
		t.Errorf("calls = %v, want %v", calls.Load(), 2)
	}
}
