package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	nodeBalancerConfigsPath = "/nodebalancers/123/configs"
	protocolHTTP            = "http"
)

func TestClientListNodeBalancerConfigsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyAlgorithm: valueRoundRobin,
				"stickiness": "http_cookie", "check": protocolHTTP, "check_interval": 10,
				"check_timeout": 5, "check_attempts": 3, "check_path": "/health",
				"check_body": "healthy", "check_passive": true, "cipher_suite": "recommended",
				"ssl_commonname": "example.com", "ssl_fingerprint": "fp", keyNodeBalancerID: 123,
				"nodes_status": map[string]int{"up": 2, "down": 1},
			}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerConfigs(t.Context(), 123)

	nbRequireNoError(t, err)
	nbRequireLenOne(t, got)
	nbCheckEqual(t, 456, got[0].ID)
	nbCheckEqual(t, 443, got[0].Port)
	nbCheckEqual(t, protocolHTTPS, got[0].Protocol)
	nbCheckEqual(t, 123, got[0].NodeBalancerID)
	nbCheckEqual(t, 2, got[0].NodesStatus.Up)
	nbCheckEqual(t, 1, got[0].NodesStatus.Down)
}

func TestClientListNodeBalancerConfigsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerConfigs(t.Context(), 123)

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListNodeBalancerConfigsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 456}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListNodeBalancerConfigs(t.Context(), 123)

	nbRequireNoError(t, err)
	nbCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	nbRequireLenOne(t, got)
	nbCheckEqual(t, 456, got[0].ID)
}

func TestClientListNodeBalancerConfigNodesSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes", r.URL.Path, "request path should match")
		nbCheckEqual(t, "2", r.URL.Query().Get(keyPage), "page query should match")
		nbCheckEqual(t, "25", r.URL.Query().Get(keyPageSize), "page_size query should match")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID: 789, keyAddress: "192.0.2.10:80", keyLabel: nodeLabelWeb1, keyStatus: nodeBalancerNodeStatusUP,
				keyWeight: 100, "mode": nodeBalancerNodeModeAccept, keyNodeBalancerID: 123, keyConfigID: 456,
			}},
			keyPage: 2, keyPages: 3, keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerConfigNodes(t.Context(), 123, 456, 2, 25)

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbRequireLenOne(t, got.Data)
	nbCheckEqual(t, 789, got.Data[0].ID)
	nbCheckEqual(t, "192.0.2.10:80", got.Data[0].Address)
	nbCheckEqual(t, 123, got.Data[0].NodeBalancerID)
	nbCheckEqual(t, 456, got.Data[0].ConfigID)
	nbCheckEqual(t, 2, got.Page)
	nbCheckEqual(t, 3, got.Pages)
}

func TestClientListNodeBalancerConfigNodesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerConfigNodes(t.Context(), 123, 456, 0, 0)

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientListNodeBalancerConfigNodesRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for invalid IDs")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerConfigNodes(t.Context(), 0, 456, 0, 0)
	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	nbCheckNil(t, got)

	got, err = client.ListNodeBalancerConfigNodes(t.Context(), 123, 0, 0, 0)
	nbRequireErrorIs(t, err, linode.ErrConfigIDPositive)
	nbCheckNil(t, got)
}

func TestClientListNodeBalancerConfigNodesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes", r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 789}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListNodeBalancerConfigNodes(t.Context(), 123, 456, 0, 0)

	nbRequireNoError(t, err)
	nbCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	nbRequireNotNil(t, got)
	nbRequireLenOne(t, got.Data)
	nbCheckEqual(t, 789, got.Data[0].ID)
}

func TestClientGetNodeBalancerConfigNodeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID: 789, keyAddress: "192.0.2.10:80", keyLabel: nodeLabelWeb1, keyStatus: nodeBalancerNodeStatusUP,
			keyWeight: 100, "mode": "accept", keyNodeBalancerID: 123, keyConfigID: 456,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetNodeBalancerConfigNode(t.Context(), 123, 456, 789)

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbCheckEqual(t, 789, got.ID)
	nbCheckEqual(t, "192.0.2.10:80", got.Address)
	nbCheckEqual(t, 123, got.NodeBalancerID)
	nbCheckEqual(t, 456, got.ConfigID)
}

func TestClientGetNodeBalancerConfigNodeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetNodeBalancerConfigNode(t.Context(), 123, 456, 789)

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetNodeBalancerConfigNodeRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for invalid IDs")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetNodeBalancerConfigNode(t.Context(), 0, 456, 789)
	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	nbCheckNil(t, got)

	got, err = client.GetNodeBalancerConfigNode(t.Context(), 123, 0, 789)
	nbRequireErrorIs(t, err, linode.ErrConfigIDPositive)
	nbCheckNil(t, got)

	got, err = client.GetNodeBalancerConfigNode(t.Context(), 123, 456, 0)
	nbRequireErrorIs(t, err, linode.ErrNodeIDPositive)
	nbCheckNil(t, got)
}

func TestClientGetNodeBalancerConfigNodeRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 789, keyNodeBalancerID: 123, keyConfigID: 456}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetNodeBalancerConfigNode(t.Context(), 123, 456, 789)

	nbRequireNoError(t, err)
	nbCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	nbRequireNotNil(t, got)
	nbCheckEqual(t, 789, got.ID)
}

func TestClientCreateNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		nbCheckEqual(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		nbCheckNoError(t, json.NewDecoder(r.Body).Decode(&body))
		port, ok := body[keyPort].(float64)
		nbCheckTrue(t, ok, "request body port should be numeric")
		nbCheckEqual(t, 80, int(port), "request body should include port")

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID: 456, keyPort: 80, keyProtocol: "http", "algorithm": valueRoundRobin, keyNodeBalancerID: 123,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, &linode.CreateNodeBalancerConfigRequest{Port: 80})

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbCheckEqual(t, 456, got.ID)
	nbCheckEqual(t, 80, got.Port)
	nbCheckEqual(t, "http", got.Protocol)
	nbCheckEqual(t, 123, got.NodeBalancerID)
}

func TestClientCreateNodeBalancerConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		nbCheckEqual(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, &linode.CreateNodeBalancerConfigRequest{Port: 80})

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientCreateNodeBalancerConfigDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		nbCheckEqual(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, &linode.CreateNodeBalancerConfigRequest{Port: 80})

	nbRequireError(t, err)
	nbCheckNil(t, got)
	nbCheckEqual(t, int32(1), calls.Load(), "POST create route must not be retried")
}

func TestClientCreateNodeBalancerConfigRejectsInvalidNodeBalancerID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for invalid nodebalancer_id")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateNodeBalancerConfig(t.Context(), 0, &linode.CreateNodeBalancerConfigRequest{Port: 80})

	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	nbCheckNil(t, got)
}

func TestClientCreateNodeBalancerConfigRejectsNilRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for nil create config request")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, nil)

	nbRequireErrorIs(t, err, linode.ErrCreateConfigRequestRequired)
	nbCheckNil(t, got)
}

func TestClientRebuildNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	var sawBody atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/rebuild", r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		if r.Body != nil {
			body, readErr := io.ReadAll(r.Body)
			nbCheckNoError(t, readErr)
			sawBody.Store(len(body) > 0)
		}

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 456, keyPort: 80, keyProtocol: protocolHTTP, keyNodeBalancerID: 123}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.RebuildNodeBalancerConfig(t.Context(), 123, 456)

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbCheckEqual(t, 456, got.ID)
	nbCheckEqual(t, 80, got.Port)
	nbCheckEqual(t, protocolHTTP, got.Protocol)
	nbCheckEqual(t, 123, got.NodeBalancerID)
	nbCheckEqual(t, false, sawBody.Load(), "rebuild request should not send a body")
}

func TestClientRebuildNodeBalancerConfigDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/rebuild", r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	got, err := client.RebuildNodeBalancerConfig(t.Context(), 123, 456)

	nbRequireError(t, err)
	nbCheckNil(t, got)
	nbCheckEqual(t, int32(1), calls.Load(), "POST rebuild route must not be retried")
}

func TestClientRebuildNodeBalancerConfigValidatesIDs(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.RebuildNodeBalancerConfig(t.Context(), 0, 456)
	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	nbCheckNil(t, got)

	got, err = client.RebuildNodeBalancerConfig(t.Context(), 123, 0)
	nbRequireErrorIs(t, err, linode.ErrConfigIDPositive)
	nbCheckNil(t, got)

	nbCheckEqual(t, int32(0), calls.Load(), "invalid IDs should be rejected before making an HTTP request")
}

func TestClientDeleteNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	var sawBody atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		nbCheckEqual(t, "/nodebalancers/123/configs/456", r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		if r.Body != nil {
			body, readErr := io.ReadAll(r.Body)
			nbCheckNoError(t, readErr)
			sawBody.Store(len(body) > 0)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteNodeBalancerConfig(t.Context(), 123, 456)

	nbRequireNoError(t, err)
	nbCheckEqual(t, false, sawBody.Load(), "DELETE should not send a request body")
}

func TestClientDeleteNodeBalancerConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		nbCheckEqual(t, "/nodebalancers/123/configs/456", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteNodeBalancerConfig(t.Context(), 123, 456)

	nbRequireError(t, err)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientDeleteNodeBalancerConfigDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		nbCheckEqual(t, "/nodebalancers/123/configs/456", r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	err := client.DeleteNodeBalancerConfig(t.Context(), 123, 456)

	nbRequireError(t, err)
	nbCheckEqual(t, int32(1), calls.Load(), "destructive DELETE should not be retried")
}

func TestClientDeleteNodeBalancerConfigValidatesIDs(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	err := client.DeleteNodeBalancerConfig(t.Context(), 0, 456)
	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)

	err = client.DeleteNodeBalancerConfig(t.Context(), 123, 0)
	nbRequireErrorIs(t, err, linode.ErrConfigIDPositive)

	nbCheckEqual(t, int32(0), calls.Load(), "invalid IDs should be rejected before making an HTTP request")
}

func TestClientGetNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerConfigsPath+"/456", r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyAlgorithm: valueRoundRobin,
			"stickiness": "http_cookie", "check": protocolHTTP, "check_interval": 10,
			"check_timeout": 5, "check_attempts": 3, "check_path": "/health",
			"check_body": "healthy", "check_passive": true, "cipher_suite": "recommended",
			"ssl_commonname": domainExample, "ssl_fingerprint": "fp", keyNodeBalancerID: 123,
			"nodes_status": map[string]int{"up": 2, "down": 1},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetNodeBalancerConfig(t.Context(), 123, 456)

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbCheckEqual(t, 456, got.ID)
	nbCheckEqual(t, 443, got.Port)
	nbCheckEqual(t, "https", got.Protocol)
	nbCheckEqual(t, 123, got.NodeBalancerID)
	nbCheckEqual(t, 2, got.NodesStatus.Up)
	nbCheckEqual(t, 1, got.NodesStatus.Down)
}

func TestClientGetNodeBalancerConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerConfigsPath+"/456", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetNodeBalancerConfig(t.Context(), 123, 456)

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientGetNodeBalancerConfigRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		nbCheckEqual(t, nodeBalancerConfigsPath+"/456", r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: 456}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.GetNodeBalancerConfig(t.Context(), 123, 456)

	nbRequireNoError(t, err)
	nbCheckEqual(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	nbRequireNotNil(t, got)
	nbCheckEqual(t, 456, got.ID)
}

func TestClientCreateNodeBalancerNodeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes", r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		nbCheckNoError(t, json.NewDecoder(r.Body).Decode(&body))
		nbCheckEqual(t, accountMaintenanceLabel, body["label"], "request body should include label")
		nbCheckEqual(t, nodeBalancerNodeAddress, body[keyAddress], "request body should include address")
		weight, ok := body["weight"].(float64)
		nbCheckTrue(t, ok, "request body weight should be numeric")
		nbCheckEqual(t, 100, int(weight), "request body should include weight")
		nbCheckEqual(t, nodeBalancerNodeModeAccept, body[keyMode], "request body should include mode")

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID: 789, "label": accountMaintenanceLabel, keyAddress: nodeBalancerNodeAddress, keyStatus: nodeBalancerNodeStatusUP, keyWeight: 100,
			keyMode: nodeBalancerNodeModeAccept, keyNodeBalancerID: 123, keyConfigID: 456,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateNodeBalancerNode(t.Context(), 123, 456, &linode.CreateNodeBalancerNodeRequest{
		Label: accountMaintenanceLabel, Address: nodeBalancerNodeAddress, Weight: 100, Mode: nodeBalancerNodeModeAccept,
	})

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbCheckEqual(t, 789, got.ID)
	nbCheckEqual(t, accountMaintenanceLabel, got.Label)
	nbCheckEqual(t, nodeBalancerNodeAddress, got.Address)
	nbCheckEqual(t, 123, got.NodeBalancerID)
	nbCheckEqual(t, 456, got.ConfigID)
}

func TestClientCreateNodeBalancerNodeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateNodeBalancerNode(t.Context(), 123, 456, &linode.CreateNodeBalancerNodeRequest{Label: accountMaintenanceLabel, Address: nodeBalancerNodeAddress})

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientCreateNodeBalancerNodeDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes", r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	got, err := client.CreateNodeBalancerNode(t.Context(), 123, 456, &linode.CreateNodeBalancerNodeRequest{Label: accountMaintenanceLabel, Address: nodeBalancerNodeAddress})

	nbRequireError(t, err)
	nbCheckNil(t, got)
	nbCheckEqual(t, int32(1), calls.Load(), "POST create route must not be retried")
}

func TestClientCreateNodeBalancerNodeRejectsInvalidIDsAndNilRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for invalid create node arguments")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateNodeBalancerNode(t.Context(), 0, 456, &linode.CreateNodeBalancerNodeRequest{Label: accountMaintenanceLabel, Address: nodeBalancerNodeAddress})
	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	nbCheckNil(t, got)

	got, err = client.CreateNodeBalancerNode(t.Context(), 123, 0, &linode.CreateNodeBalancerNodeRequest{Label: accountMaintenanceLabel, Address: nodeBalancerNodeAddress})
	nbRequireErrorIs(t, err, linode.ErrConfigIDPositive)
	nbCheckNil(t, got)

	got, err = client.CreateNodeBalancerNode(t.Context(), 123, 456, nil)
	nbRequireErrorIs(t, err, linode.ErrCreateNodeBalancerNodeRequestRequired)
	nbCheckNil(t, got)
}

func TestClientDeleteNodeBalancerConfigNodeSuccess(t *testing.T) {
	t.Parallel()

	var sawBody atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		if r.Body != nil {
			body, readErr := io.ReadAll(r.Body)
			nbCheckNoError(t, readErr)
			sawBody.Store(len(body) > 0)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteNodeBalancerConfigNode(t.Context(), 123, 456, 789)

	nbRequireNoError(t, err)
	nbCheckEqual(t, false, sawBody.Load(), "DELETE should not send a request body")
}

func TestClientDeleteNodeBalancerConfigNodeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteNodeBalancerConfigNode(t.Context(), 123, 456, 789)

	nbRequireError(t, err)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientDeleteNodeBalancerConfigNodeDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		nbCheckEqual(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	err := client.DeleteNodeBalancerConfigNode(t.Context(), 123, 456, 789)

	nbRequireError(t, err)
	nbCheckEqual(t, int32(1), calls.Load(), "destructive DELETE should not be retried")
}

func TestClientDeleteNodeBalancerConfigNodeValidatesIDs(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteNodeBalancerConfigNode(t.Context(), 0, 456, 789)
	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)

	err = client.DeleteNodeBalancerConfigNode(t.Context(), 123, 0, 789)
	nbRequireErrorIs(t, err, linode.ErrConfigIDPositive)

	err = client.DeleteNodeBalancerConfigNode(t.Context(), 123, 456, 0)
	nbRequireErrorIs(t, err, linode.ErrNodeIDPositive)

	nbCheckEqual(t, int32(0), calls.Load(), "invalid IDs should be rejected before making an HTTP request")
}

func TestClientUpdateNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		nbCheckEqual(t, "/nodebalancers/123/configs/456", r.URL.Path, "request path should match")
		nbCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		nbCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		nbCheckNoError(t, json.NewDecoder(r.Body).Decode(&body))
		port, ok := body[keyPort].(float64)
		nbCheckTrue(t, ok, "request body port should be numeric")
		nbCheckEqual(t, 443, int(port), "request body should include port")
		nbCheckEqual(t, protocolHTTPS, body[keyProtocol], "request body should include protocol")

		w.Header().Set("Content-Type", "application/json")
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyAlgorithm: valueRoundRobin, keyNodeBalancerID: 123,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerConfig(t.Context(), 123, 456, &linode.UpdateNodeBalancerConfigRequest{Port: 443, Protocol: protocolHTTPS})

	nbRequireNoError(t, err)
	nbRequireNotNil(t, got)
	nbCheckEqual(t, 456, got.ID)
	nbCheckEqual(t, 443, got.Port)
	nbCheckEqual(t, protocolHTTPS, got.Protocol)
	nbCheckEqual(t, 123, got.NodeBalancerID)
}

func TestClientUpdateNodeBalancerConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		nbCheckEqual(t, "/nodebalancers/123/configs/456", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		nbCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerConfig(t.Context(), 123, 456, &linode.UpdateNodeBalancerConfigRequest{Port: 443})

	nbRequireError(t, err)
	nbCheckNil(t, got)

	apiErr := nbRequireAPIError(t, err)
	nbCheckEqual(t, http.StatusForbidden, apiErr.StatusCode)
	nbCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientUpdateNodeBalancerConfigDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nbCheckEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		nbCheckEqual(t, "/nodebalancers/123/configs/456", r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	got, err := client.UpdateNodeBalancerConfig(t.Context(), 123, 456, &linode.UpdateNodeBalancerConfigRequest{Port: 443})

	nbRequireError(t, err)
	nbCheckNil(t, got)
	nbCheckEqual(t, int32(1), calls.Load(), "PUT update route must not be retried")
}

func TestClientUpdateNodeBalancerConfigRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for invalid IDs")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateNodeBalancerConfig(t.Context(), 0, 456, &linode.UpdateNodeBalancerConfigRequest{Port: 443})
	nbRequireErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	nbCheckNil(t, got)

	got, err = client.UpdateNodeBalancerConfig(t.Context(), 123, 0, &linode.UpdateNodeBalancerConfigRequest{Port: 443})
	nbRequireErrorIs(t, err, linode.ErrConfigIDPositive)
	nbCheckNil(t, got)
}

func TestClientUpdateNodeBalancerConfigRejectsNilRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for nil update config request")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerConfig(t.Context(), 123, 456, nil)

	nbRequireErrorIs(t, err, linode.ErrUpdateConfigRequestRequired)
	nbCheckNil(t, got)
}
