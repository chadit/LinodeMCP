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

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	nodeBalancerConfigsPath = "/nodebalancers/123/configs"
	protocolHTTP            = "http"
)

func TestClientListNodeBalancerConfigsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerConfigsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerConfigsPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyAlgorithm: valueRoundRobin,
				"stickiness": "http_cookie", "check": protocolHTTP, "check_interval": 10,
				"check_timeout": 5, "check_attempts": 3, "check_path": "/health",
				"check_body": "healthy", "check_passive": true, "cipher_suite": "recommended",
				"ssl_commonname": "example.com", "ssl_fingerprint": "fp", keyNodeBalancerID: 123,
				"nodes_status": map[string]int{"up": 2, "down": 1},
			}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerConfigs(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if got[0].ID != 456 {
		t.Errorf("got[0].ID = %v, want %v", got[0].ID, 456)
	}

	if got[0].Port != 443 {
		t.Errorf("got[0].Port = %v, want %v", got[0].Port, 443)
	}

	if got[0].Protocol != protocolHTTPS {
		t.Errorf("got[0].Protocol = %v, want %v", got[0].Protocol, protocolHTTPS)
	}

	if got[0].NodeBalancerID != 123 {
		t.Errorf("got[0].NodeBalancerID = %v, want %v", got[0].NodeBalancerID, 123)
	}

	if got[0].NodesStatus.Up != 2 {
		t.Errorf("got[0].NodesStatus.Up = %v, want %v", got[0].NodesStatus.Up, 2)
	}

	if got[0].NodesStatus.Down != 1 {
		t.Errorf("got[0].NodesStatus.Down = %v, want %v", got[0].NodesStatus.Down, 1)
	}
}

func TestClientListNodeBalancerConfigsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerConfigsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerConfigsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerConfigs(t.Context(), 123)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
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

func TestClientListNodeBalancerConfigsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerConfigsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerConfigsPath)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 456}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListNodeBalancerConfigs(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if got[0].ID != 456 {
		t.Errorf("got[0].ID = %v, want %v", got[0].ID, 456)
	}
}

func TestClientListNodeBalancerConfigNodesSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes)
		}

		if r.URL.Query().Get(keyPage) != "2" {
			t.Errorf("r.URL.Query().Get(keyPage) = %v, want %v", r.URL.Query().Get(keyPage), "2")
		}

		if r.URL.Query().Get(keyPageSize) != "25" {
			t.Errorf("r.URL.Query().Get(keyPageSize) = %v, want %v", r.URL.Query().Get(keyPageSize), "25")
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID: 789, keyAddress: "192.0.2.10:80", keyLabel: nodeLabelWeb1, keyStatus: nodeBalancerNodeStatusUP,
				keyWeight: 100, "mode": nodeBalancerNodeModeAccept, keyNodeBalancerID: 123, keyConfigID: 456,
			}},
			keyPage: 2, keyPages: 3, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerConfigNodes(t.Context(), 123, 456, 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if got.Data[0].ID != 789 {
		t.Errorf("got.Data[0].ID = %v, want %v", got.Data[0].ID, 789)
	}

	if got.Data[0].Address != nodeBalancerNodeAddress {
		t.Errorf("got.Data[0].Address = %v, want %v", got.Data[0].Address, nodeBalancerNodeAddress)
	}

	if got.Data[0].NodeBalancerID != 123 {
		t.Errorf("got.Data[0].NodeBalancerID = %v, want %v", got.Data[0].NodeBalancerID, 123)
	}

	if got.Data[0].ConfigID != 456 {
		t.Errorf("got.Data[0].ConfigID = %v, want %v", got.Data[0].ConfigID, 456)
	}

	if got.Page != 2 {
		t.Errorf("got.Page = %v, want %v", got.Page, 2)
	}

	if got.Pages != 3 {
		t.Errorf("got.Pages = %v, want %v", got.Pages, 3)
	}
}

func TestClientListNodeBalancerConfigNodesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerConfigNodes(t.Context(), 123, 456, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
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

func TestClientListNodeBalancerConfigNodesRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for invalid IDs")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerConfigNodes(t.Context(), 0, 456, 0, 0)
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	got, err = client.ListNodeBalancerConfigNodes(t.Context(), 123, 0, 0, 0)
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientListNodeBalancerConfigNodesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 789}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListNodeBalancerConfigNodes(t.Context(), 123, 456, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if len(got.Data) != 1 {
		t.Fatalf("len(got.Data) = %d, want 1", len(got.Data))
	}

	if got.Data[0].ID != 789 {
		t.Errorf("got.Data[0].ID = %v, want %v", got.Data[0].ID, 789)
	}
}

func TestClientGetNodeBalancerConfigNodeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 789, keyAddress: "192.0.2.10:80", keyLabel: nodeLabelWeb1, keyStatus: nodeBalancerNodeStatusUP,
			keyWeight: 100, "mode": "accept", keyNodeBalancerID: 123, keyConfigID: 456,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetNodeBalancerConfigNode(t.Context(), 123, 456, 789)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 789 {
		t.Errorf("got.ID = %v, want %v", got.ID, 789)
	}

	if got.Address != nodeBalancerNodeAddress {
		t.Errorf("got.Address = %v, want %v", got.Address, nodeBalancerNodeAddress)
	}

	if got.NodeBalancerID != 123 {
		t.Errorf("got.NodeBalancerID = %v, want %v", got.NodeBalancerID, 123)
	}

	if got.ConfigID != 456 {
		t.Errorf("got.ConfigID = %v, want %v", got.ConfigID, 456)
	}
}

func TestClientGetNodeBalancerConfigNodeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetNodeBalancerConfigNode(t.Context(), 123, 456, 789)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
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

func TestClientGetNodeBalancerConfigNodeRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for invalid IDs")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetNodeBalancerConfigNode(t.Context(), 0, 456, 789)
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	got, err = client.GetNodeBalancerConfigNode(t.Context(), 123, 0, 789)
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	got, err = client.GetNodeBalancerConfigNode(t.Context(), 123, 456, 0)
	if !errors.Is(err, linode.ErrNodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientGetNodeBalancerConfigNodeRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: 789, keyNodeBalancerID: 123, keyConfigID: 456}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetNodeBalancerConfigNode(t.Context(), 123, 456, 789)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 789 {
		t.Errorf("got.ID = %v, want %v", got.ID, 789)
	}
}

func TestClientCreateNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != nodeBalancerConfigsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerConfigsPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		port, ok := body[keyPort].(float64)
		if !ok {
			t.Error("ok = false, want true")
		}

		if int(port) != 80 {
			t.Errorf("int(port) = %v, want %v", int(port), 80)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 456, keyPort: 80, keyProtocol: "http", "algorithm": valueRoundRobin, keyNodeBalancerID: 123,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, &linode.CreateNodeBalancerConfigRequest{Port: 80})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 456 {
		t.Errorf("got.ID = %v, want %v", got.ID, 456)
	}

	if got.Port != 80 {
		t.Errorf("got.Port = %v, want %v", got.Port, 80)
	}

	if got.Protocol != "http" {
		t.Errorf("got.Protocol = %v, want %v", got.Protocol, "http")
	}

	if got.NodeBalancerID != 123 {
		t.Errorf("got.NodeBalancerID = %v, want %v", got.NodeBalancerID, 123)
	}
}

func TestClientCreateNodeBalancerConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != nodeBalancerConfigsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerConfigsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, &linode.CreateNodeBalancerConfigRequest{Port: 80})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
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

func TestClientCreateNodeBalancerConfigDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != nodeBalancerConfigsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerConfigsPath)
		}

		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, &linode.CreateNodeBalancerConfigRequest{Port: 80})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
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

	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
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

	if !errors.Is(err, linode.ErrCreateConfigRequestRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrCreateConfigRequestRequired)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientRebuildNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	var sawBody atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/nodebalancers/123/configs/456/rebuild" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/nodebalancers/123/configs/456/rebuild")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if r.Body != nil {
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				t.Errorf("unexpected error: %v", readErr)
			}

			sawBody.Store(len(body) > 0)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: 456, keyPort: 80, keyProtocol: protocolHTTP, keyNodeBalancerID: 123}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.RebuildNodeBalancerConfig(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 456 {
		t.Errorf("got.ID = %v, want %v", got.ID, 456)
	}

	if got.Port != 80 {
		t.Errorf("got.Port = %v, want %v", got.Port, 80)
	}

	if got.Protocol != protocolHTTP {
		t.Errorf("got.Protocol = %v, want %v", got.Protocol, protocolHTTP)
	}

	if got.NodeBalancerID != 123 {
		t.Errorf("got.NodeBalancerID = %v, want %v", got.NodeBalancerID, 123)
	}

	if sawBody.Load() != false {
		t.Errorf("sawBody.Load() = %v, want %v", sawBody.Load(), false)
	}
}

func TestClientRebuildNodeBalancerConfigDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/nodebalancers/123/configs/456/rebuild" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/nodebalancers/123/configs/456/rebuild")
		}

		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	got, err := client.RebuildNodeBalancerConfig(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
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
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	got, err = client.RebuildNodeBalancerConfig(t.Context(), 123, 0)
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}
}

func TestClientDeleteNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	var sawBody atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcNodebalancers123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if r.Body != nil {
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				t.Errorf("unexpected error: %v", readErr)
			}

			sawBody.Store(len(body) > 0)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteNodeBalancerConfig(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sawBody.Load() != false {
		t.Errorf("sawBody.Load() = %v, want %v", sawBody.Load(), false)
	}
}

func TestClientDeleteNodeBalancerConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcNodebalancers123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteNodeBalancerConfig(t.Context(), 123, 456)
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

func TestClientDeleteNodeBalancerConfigDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcNodebalancers123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456)
		}

		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	err := client.DeleteNodeBalancerConfig(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
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
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	err = client.DeleteNodeBalancerConfig(t.Context(), 123, 0)
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}
}

func TestClientGetNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerConfigsPath+"/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerConfigsPath+"/456")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyAlgorithm: valueRoundRobin,
			"stickiness": "http_cookie", "check": protocolHTTP, "check_interval": 10,
			"check_timeout": 5, "check_attempts": 3, "check_path": "/health",
			"check_body": "healthy", "check_passive": true, "cipher_suite": "recommended",
			"ssl_commonname": domainExample, "ssl_fingerprint": "fp", keyNodeBalancerID: 123,
			"nodes_status": map[string]int{"up": 2, "down": 1},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetNodeBalancerConfig(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 456 {
		t.Errorf("got.ID = %v, want %v", got.ID, 456)
	}

	if got.Port != 443 {
		t.Errorf("got.Port = %v, want %v", got.Port, 443)
	}

	if got.Protocol != "https" {
		t.Errorf("got.Protocol = %v, want %v", got.Protocol, "https")
	}

	if got.NodeBalancerID != 123 {
		t.Errorf("got.NodeBalancerID = %v, want %v", got.NodeBalancerID, 123)
	}

	if got.NodesStatus.Up != 2 {
		t.Errorf("got.NodesStatus.Up = %v, want %v", got.NodesStatus.Up, 2)
	}

	if got.NodesStatus.Down != 1 {
		t.Errorf("got.NodesStatus.Down = %v, want %v", got.NodesStatus.Down, 1)
	}
}

func TestClientGetNodeBalancerConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerConfigsPath+"/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerConfigsPath+"/456")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetNodeBalancerConfig(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
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

func TestClientGetNodeBalancerConfigRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerConfigsPath+"/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerConfigsPath+"/456")
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: 456}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetNodeBalancerConfig(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 456 {
		t.Errorf("got.ID = %v, want %v", got.ID, 456)
	}
}

func TestClientCreateNodeBalancerNodeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["label"], accountMaintenanceLabel) {
			t.Errorf("got %v, want %v", body["label"], accountMaintenanceLabel)
		}

		if !reflect.DeepEqual(body[keyAddress], nodeBalancerNodeAddress) {
			t.Errorf("body[keyAddress] = %v, want %v", body[keyAddress], nodeBalancerNodeAddress)
		}

		weight, ok := body["weight"].(float64)
		if !ok {
			t.Error("ok = false, want true")
		}

		if int(weight) != 100 {
			t.Errorf("int(weight) = %v, want %v", int(weight), 100)
		}

		if !reflect.DeepEqual(body[keyMode], nodeBalancerNodeModeAccept) {
			t.Errorf("body[keyMode] = %v, want %v", body[keyMode], nodeBalancerNodeModeAccept)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 789, "label": accountMaintenanceLabel, keyAddress: nodeBalancerNodeAddress, keyStatus: nodeBalancerNodeStatusUP, keyWeight: 100,
			keyMode: nodeBalancerNodeModeAccept, keyNodeBalancerID: 123, keyConfigID: 456,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateNodeBalancerNode(t.Context(), 123, 456, &linode.CreateNodeBalancerNodeRequest{
		Label: accountMaintenanceLabel, Address: nodeBalancerNodeAddress, Weight: 100, Mode: nodeBalancerNodeModeAccept,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 789 {
		t.Errorf("got.ID = %v, want %v", got.ID, 789)
	}

	if got.Label != accountMaintenanceLabel {
		t.Errorf("got.Label = %v, want %v", got.Label, accountMaintenanceLabel)
	}

	if got.Address != nodeBalancerNodeAddress {
		t.Errorf("got.Address = %v, want %v", got.Address, nodeBalancerNodeAddress)
	}

	if got.NodeBalancerID != 123 {
		t.Errorf("got.NodeBalancerID = %v, want %v", got.NodeBalancerID, 123)
	}

	if got.ConfigID != 456 {
		t.Errorf("got.ConfigID = %v, want %v", got.ConfigID, 456)
	}
}

func TestClientCreateNodeBalancerNodeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateNodeBalancerNode(t.Context(), 123, 456, &linode.CreateNodeBalancerNodeRequest{Label: accountMaintenanceLabel, Address: nodeBalancerNodeAddress})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
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

func TestClientCreateNodeBalancerNodeDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes)
		}

		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	got, err := client.CreateNodeBalancerNode(t.Context(), 123, 456, &linode.CreateNodeBalancerNodeRequest{Label: accountMaintenanceLabel, Address: nodeBalancerNodeAddress})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
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
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	got, err = client.CreateNodeBalancerNode(t.Context(), 123, 0, &linode.CreateNodeBalancerNodeRequest{Label: accountMaintenanceLabel, Address: nodeBalancerNodeAddress})
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	got, err = client.CreateNodeBalancerNode(t.Context(), 123, 456, nil)
	if !errors.Is(err, linode.ErrCreateNodeBalancerNodeRequestRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrCreateNodeBalancerNodeRequestRequired)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientDeleteNodeBalancerConfigNodeSuccess(t *testing.T) {
	t.Parallel()

	var sawBody atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if r.Body != nil {
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				t.Errorf("unexpected error: %v", readErr)
			}

			sawBody.Store(len(body) > 0)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteNodeBalancerConfigNode(t.Context(), 123, 456, 789)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sawBody.Load() != false {
		t.Errorf("sawBody.Load() = %v, want %v", sawBody.Load(), false)
	}
}

func TestClientDeleteNodeBalancerConfigNodeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteNodeBalancerConfigNode(t.Context(), 123, 456, 789)
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

func TestClientDeleteNodeBalancerConfigNodeDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcNodebalancers123Configs456Nodes789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456Nodes789)
		}

		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	err := client.DeleteNodeBalancerConfigNode(t.Context(), 123, 456, 789)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
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
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	err = client.DeleteNodeBalancerConfigNode(t.Context(), 123, 0, 789)
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	err = client.DeleteNodeBalancerConfigNode(t.Context(), 123, 456, 0)
	if !errors.Is(err, linode.ErrNodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeIDPositive)
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}
}

func TestClientUpdateNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNodebalancers123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		port, ok := body[keyPort].(float64)
		if !ok {
			t.Error("ok = false, want true")
		}

		if int(port) != 443 {
			t.Errorf("int(port) = %v, want %v", int(port), 443)
		}

		if !reflect.DeepEqual(body[keyProtocol], protocolHTTPS) {
			t.Errorf("body[keyProtocol] = %v, want %v", body[keyProtocol], protocolHTTPS)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 456, keyPort: 443, keyProtocol: protocolHTTPS, keyAlgorithm: valueRoundRobin, keyNodeBalancerID: 123,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateNodeBalancerConfig(t.Context(), 123, 456, &linode.UpdateNodeBalancerConfigRequest{Port: 443, Protocol: protocolHTTPS})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 456 {
		t.Errorf("got.ID = %v, want %v", got.ID, 456)
	}

	if got.Port != 443 {
		t.Errorf("got.Port = %v, want %v", got.Port, 443)
	}

	if got.Protocol != protocolHTTPS {
		t.Errorf("got.Protocol = %v, want %v", got.Protocol, protocolHTTPS)
	}

	if got.NodeBalancerID != 123 {
		t.Errorf("got.NodeBalancerID = %v, want %v", got.NodeBalancerID, 123)
	}
}

func TestClientUpdateNodeBalancerConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNodebalancers123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateNodeBalancerConfig(t.Context(), 123, 456, &linode.UpdateNodeBalancerConfigRequest{Port: 443})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
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

func TestClientUpdateNodeBalancerConfigDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNodebalancers123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456)
		}

		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	got, err := client.UpdateNodeBalancerConfig(t.Context(), 123, 456, &linode.UpdateNodeBalancerConfigRequest{Port: 443})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
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
	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	got, err = client.UpdateNodeBalancerConfig(t.Context(), 123, 0, &linode.UpdateNodeBalancerConfigRequest{Port: 443})
	if !errors.Is(err, linode.ErrConfigIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
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

	if !errors.Is(err, linode.ErrUpdateConfigRequestRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrUpdateConfigRequestRequired)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}
