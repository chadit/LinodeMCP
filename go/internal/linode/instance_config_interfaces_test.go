package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
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
			Purpose:  "vpc",
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/configs/456/interfaces", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(interfaces), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceConfigInterfaces(t.Context(), 123, 456)

	require.NoError(t, err, "ListInstanceConfigInterfaces should succeed on 200 response")
	require.Len(t, got, 1)
	assert.Equal(t, 103, got[0].ID)
	assert.Equal(t, "vpc", got[0].Purpose)
	assert.True(t, got[0].Active)
	require.NotNil(t, got[0].VPCID)
	assert.Equal(t, 111, *got[0].VPCID)
	require.NotNil(t, got[0].IPv4)
	assert.Equal(t, "203.0.113.2", *got[0].IPv4.NAT1To1)
}

func TestClientListInstanceConfigInterfacesAcceptsPaginatedEnvelope(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/configs/456/interfaces", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    []linode.ConfigInterfaceResponse{{ID: 101, Purpose: purposePublic}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceConfigInterfaces(t.Context(), 123, 456)

	require.NoError(t, err, "ListInstanceConfigInterfaces should tolerate a paginated envelope")
	require.Len(t, got, 1)
	assert.Equal(t, 101, got[0].ID)
}

func TestClientListInstanceConfigInterfacesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/configs/456/interfaces", r.URL.Path, "request path should match")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}), "encoding error response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListInstanceConfigInterfaces(t.Context(), 123, 456)

	require.Error(t, err, "ListInstanceConfigInterfaces should fail on API error")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
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

			require.Error(t, err, "ListInstanceConfigInterfaces should reject invalid IDs before request")
			assert.False(t, called.Load(), "invalid IDs should not reach upstream server")
			assert.ErrorIs(t, err, tt.wantErr, "error should expose invalid ID sentinel")
		})
	}
}
