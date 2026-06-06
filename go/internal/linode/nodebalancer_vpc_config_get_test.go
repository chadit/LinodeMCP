package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestGetNodeBalancerVPCConfig(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
			nbCheckEqual(t, "/nodebalancers/123/vpcs/456", r.URL.Path, "request path should include both IDs")
			nbCheckEmpty(t, r.URL.RawQuery, "request should not include query parameters")
			nbCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
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
		nbRequireNoError(t, err)
		nbRequireNotNil(t, config)
		nbCheckEqual(t, 456, config.ID)
		nbRequireNotNil(t, config.VPCID)
		nbCheckEqual(t, 789, *config.VPCID)
		nbCheckEqual(t, 123, config.NodeBalancerID)
		nbCheckEqual(t, 321, config.SubnetID)
		nbCheckEqual(t, "10.100.5.100/30", config.IPv4Range)
		nbRequireNotNil(t, config.IPv4RangeAutoAssign)
		nbCheckEqual(t, false, *config.IPv4RangeAutoAssign)
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
			nbCheckEqual(t, "/nodebalancers/123/vpcs/456", r.URL.Path, "request path should include both IDs")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errNotFound}},
			}))
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

		_, err := client.GetNodeBalancerVPCConfig(t.Context(), 123, 456)
		nbRequireError(t, err)
	})

	t.Run("id validation", func(t *testing.T) {
		t.Parallel()

		client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

		_, err := client.GetNodeBalancerVPCConfig(t.Context(), 0, 456)
		nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)

		_, err = client.GetNodeBalancerVPCConfig(t.Context(), 123, 0)
		nbRequireErrorIs(t, err, linode.ErrConfigIDPositive)
	})
}
