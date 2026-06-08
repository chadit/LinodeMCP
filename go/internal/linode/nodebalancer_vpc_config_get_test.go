package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestGetNodeBalancerVPCConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/nodebalancers/123/vpcs/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/nodebalancers/123/vpcs/456")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:                    456,
			keyVPCID:                 789,
			keyNodeBalancerID:        123,
			"subnet_id":              321,
			"ipv4_range":             "10.100.5.100/30",
			"ipv4_range_auto_assign": false,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	config, err := client.GetNodeBalancerVPCConfig(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config == nil {
		t.Fatal("config is nil")
	}

	if config.ID != 456 {
		t.Errorf("config.ID = %v, want %v", config.ID, 456)
	}

	if config.VPCID == nil {
		t.Fatal("config.VPCID is nil")
	}

	if *config.VPCID != 789 {
		t.Errorf("*config.VPCID = %v, want %v", *config.VPCID, 789)
	}

	if config.NodeBalancerID != 123 {
		t.Errorf("config.NodeBalancerID = %v, want %v", config.NodeBalancerID, 123)
	}

	if config.SubnetID != 321 {
		t.Errorf("config.SubnetID = %v, want %v", config.SubnetID, 321)
	}

	if config.IPv4Range != "10.100.5.100/30" {
		t.Errorf("config.IPv4Range = %v, want %v", config.IPv4Range, "10.100.5.100/30")
	}

	if config.IPv4RangeAutoAssign == nil {
		t.Fatal("config.IPv4RangeAutoAssign is nil")
	}

	if *config.IPv4RangeAutoAssign != false {
		t.Errorf("*config.IPv4RangeAutoAssign = %v, want %v", *config.IPv4RangeAutoAssign, false)
	}
}

func TestGetNodeBalancerVPCConfigApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/nodebalancers/123/vpcs/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/nodebalancers/123/vpcs/456")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetNodeBalancerVPCConfig(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestGetNodeBalancerVPCConfigIdValidation(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetNodeBalancerVPCConfig(t.Context(), 0, 456)
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	_, err = client.GetNodeBalancerVPCConfig(t.Context(), 123, 0)
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}
}
