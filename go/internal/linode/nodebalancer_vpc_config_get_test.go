package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestGetNodeBalancerVPCConfig(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/nodebalancers/123/vpcs/456", r.URL.Path, "request path should include both IDs")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:                    456,
				keyVPCID:                 789,
				keyNodeBalancerID:        123,
				"subnet_id":              321,
				"ipv4_range":             "10.100.5.100/30",
				"ipv4_range_auto_assign": false,
			}))
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

		config, err := client.GetNodeBalancerVPCConfig(t.Context(), 123, 456)
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, 456, config.ID)
		require.NotNil(t, config.VPCID)
		assert.Equal(t, 789, *config.VPCID)
		assert.Equal(t, 123, config.NodeBalancerID)
		assert.Equal(t, 321, config.SubnetID)
		assert.Equal(t, "10.100.5.100/30", config.IPv4Range)
		require.NotNil(t, config.IPv4RangeAutoAssign)
		assert.False(t, *config.IPv4RangeAutoAssign)
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/nodebalancers/123/vpcs/456", r.URL.Path, "request path should include both IDs")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errNotFound}},
			}))
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

		_, err := client.GetNodeBalancerVPCConfig(t.Context(), 123, 456)
		require.Error(t, err)
	})

	t.Run("id validation", func(t *testing.T) {
		t.Parallel()

		client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

		_, err := client.GetNodeBalancerVPCConfig(t.Context(), 0, 456)
		require.ErrorIs(t, err, linode.ErrNodeBalancerIDPositive)

		_, err = client.GetNodeBalancerVPCConfig(t.Context(), 123, 0)
		require.ErrorIs(t, err, linode.ErrConfigIDPositive)
	})
}
