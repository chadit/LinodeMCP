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

func TestClientListInstanceConfigInterfacesSuccess(t *testing.T) {
	t.Parallel()

	primary := true
	subnetID := 101
	vpcID := 111
	label := "vpc-interface"
	natIPv4 := "203.0.113.2"
	vpcIPv4 := "10.0.1.2"
	interfaces := []linode.ConfigInterfaceResponse{
		{
			ID:       103,
			Active:   true,
			Purpose:  purposeVPC,
			Label:    &label,
			Primary:  primary,
			SubnetID: &subnetID,
			VPCID:    &vpcID,
			IPv4: &linode.ConfigInterfaceIPv4{
				NAT1To1: &natIPv4,
				VPC:     &vpcIPv4,
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLinodeInstances123Configs456Interfaces {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs456Interfaces)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(interfaces); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceConfigInterfaces(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if got[0].ID != 103 {
		t.Errorf("got[0].ID = %v, want %v", got[0].ID, 103)
	}

	if got[0].Purpose != purposeVPC {
		t.Errorf("got[0].Purpose = %v, want %v", got[0].Purpose, purposeVPC)
	}

	if !got[0].Active {
		t.Error("got[0].Active = false, want true")
	}

	if got[0].VPCID == nil {
		t.Fatal("got[0].VPCID is nil")
	}

	if *got[0].VPCID != 111 {
		t.Errorf("*got[0].VPCID = %v, want %v", *got[0].VPCID, 111)
	}

	if got[0].IPv4 == nil {
		t.Fatal("got[0].IPv4 is nil")
	}

	if got[0].IPv4.NAT1To1 == nil {
		t.Fatal("got[0].IPv4.NAT1To1 is nil")
	}

	if *got[0].IPv4.NAT1To1 != "203.0.113.2" {
		t.Errorf("*got[0].IPv4.NAT1To1 = %v, want %v", *got[0].IPv4.NAT1To1, "203.0.113.2")
	}
}

func TestClientListInstanceConfigInterfacesAcceptsPaginatedEnvelope(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLinodeInstances123Configs456Interfaces {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs456Interfaces)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    []linode.ConfigInterfaceResponse{{ID: 101, Purpose: purposePublic}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceConfigInterfaces(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if got[0].ID != 101 {
		t.Errorf("got[0].ID = %v, want %v", got[0].ID, 101)
	}
}

func TestClientListInstanceConfigInterfacesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs456Interfaces {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs456Interfaces)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.ListInstanceConfigInterfaces(t.Context(), 123, 456)
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
}

func TestClientListInstanceConfigInterfacesRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		linodeID int
		configID int
		wantErr  error
	}{
		{name: "zero linode id", linodeID: 0, configID: 456, wantErr: linode.ErrLinodeIDPositive},
		{name: "negative linode id", linodeID: -1, configID: 456, wantErr: linode.ErrLinodeIDPositive},
		{name: "zero config id", linodeID: 123, configID: 0, wantErr: linode.ErrConfigIDPositive},
		{name: "negative config id", linodeID: 123, configID: -1, wantErr: linode.ErrConfigIDPositive},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called.Store(true)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

			_, err := client.ListInstanceConfigInterfaces(t.Context(), tt.linodeID, tt.configID)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if called.Load() {
				t.Fatalf("invalid IDs should not reach upstream server")
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
