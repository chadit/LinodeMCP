package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// End-to-end verification of LKE cluster listing and filtering.
func TestLinodeLKEClustersListToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKEClusterListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_cluster_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_cluster_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEClustersListToolSuccess(t *testing.T) {
	t.Parallel()

	clusters := []linode.LKECluster{
		{ID: 1, Label: labelProdCluster, Region: regionUSEast, K8sVersion: lkeVersion129, Status: statusReady},
		{ID: 2, Label: "dev-cluster", Region: regionEUWest, K8sVersion: lkeVersion128, Status: statusReady},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    clusters,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEClusterListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, labelProdCluster) {
		t.Errorf("textContent.Text does not contain %v", labelProdCluster)
	}

	if !strings.Contains(textContent.Text, "dev-cluster") {
		t.Errorf("textContent.Text does not contain %v", "dev-cluster")
	}
}

func TestLinodeLKEClustersListToolFilterByLabel(t *testing.T) {
	t.Parallel()

	clusters := []linode.LKECluster{
		{ID: 1, Label: labelProdCluster, Region: regionUSEast, K8sVersion: lkeVersion129, Status: statusReady},
		{ID: 2, Label: "dev-cluster", Region: regionEUWest, K8sVersion: lkeVersion128, Status: statusReady},
		{ID: 3, Label: "staging-prod", Region: regionUSWest, K8sVersion: lkeVersion129, Status: statusReady},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    clusters,
			keyPage:    1,
			keyPages:   1,
			keyResults: 3,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEClusterListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLabel: canRunEnvProd})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, labelProdCluster) {
		t.Errorf("textContent.Text does not contain %v", labelProdCluster)
	}

	if !strings.Contains(textContent.Text, "staging-prod") {
		t.Errorf("textContent.Text does not contain %v", "staging-prod")
	}

	if strings.Contains(textContent.Text, "dev-cluster") {
		t.Errorf("textContent.Text should not contain %v", "dev-cluster")
	}
}

// End-to-end verification of LKE cluster get workflow.
func TestLinodeLKEClusterGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_cluster_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_cluster_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEClusterGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEClusterGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingClusterID, args: map[string]any{}, wantContains: errClusterIDRequired},
		{name: "invalid cluster id", args: map[string]any{keyClusterID: notANumber}, wantContains: "cluster_id must be a valid integer"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeLKEClusterGetToolSuccess(t *testing.T) {
	t.Parallel()

	cluster := linode.LKECluster{
		ID: 123, Label: labelProdCluster, Region: regionUSEast, K8sVersion: lkeVersion129, Status: statusReady,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != lkeClusterGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, lkeClusterGetPath)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(cluster); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEClusterGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, labelProdCluster) {
		t.Errorf("textContent.Text does not contain %v", labelProdCluster)
	}

	if !strings.Contains(textContent.Text, lkeVersion129) {
		t.Errorf("textContent.Text does not contain %v", lkeVersion129)
	}
}

// TestLinodeLKEPoolsListTool verifies the LKE pools list tool
// registers correctly, validates cluster_id, and returns pool data.
func TestLinodeLKEPoolsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_pool_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_pool_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEPoolsListToolCaseMissingClusterID(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEPoolListTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errClusterIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errClusterIDRequired)
	}
}

func TestLinodeLKEPoolsListToolSuccess(t *testing.T) {
	t.Parallel()

	pools := []linode.LKENodePool{
		{ID: 10, ClusterID: 123, Type: typeG6Standard2, Count: 3},
		{ID: 11, ClusterID: 123, Type: "g6-standard-4", Count: 2},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/pools" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/pools")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: pools, keyPage: 1, keyPages: 1, keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEPoolListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, typeG6Standard2) {
		t.Errorf("textContent.Text does not contain %v", typeG6Standard2)
	}

	if !strings.Contains(textContent.Text, "g6-standard-4") {
		t.Errorf("textContent.Text does not contain %v", "g6-standard-4")
	}

	if got := listResponseCount(t, textContent.Text); got != 2 {
		t.Errorf("listResponseCount = %d, want 2", got)
	}
}

// TestLinodeLKEPoolGetTool verifies the LKE pool get tool
// registers correctly, validates required fields, and retrieves pool details.
func TestLinodeLKEPoolGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_pool_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_pool_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEPoolGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEPoolGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingClusterID, args: map[string]any{keyPoolID: "10"}, wantContains: errClusterIDRequired},
		{name: "missing pool id", args: map[string]any{keyClusterID: "123"}, wantContains: "pool_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeLKEPoolGetToolSuccess(t *testing.T) {
	t.Parallel()

	pool := linode.LKENodePool{ID: 10, ClusterID: 123, Type: typeG6Standard2, Count: 3}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != lkePoolGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, lkePoolGetPath)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(pool); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEPoolGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: "123", keyPoolID: "10"})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, typeG6Standard2) {
		t.Errorf("textContent.Text does not contain %v", typeG6Standard2)
	}
}

// TestLinodeLKENodeGetTool verifies the LKE node get tool
// registers correctly, validates required fields, and retrieves node details.
func TestLinodeLKENodeGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKENodeGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_node_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_node_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKENodeGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKENodeGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingClusterID, args: map[string]any{keyNodeID: idAbc123}, wantContains: errClusterIDRequired},
		{name: "missing node id", args: map[string]any{keyClusterID: "123"}, wantContains: "node_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeLKENodeGetToolSuccess(t *testing.T) {
	t.Parallel()

	node := linode.LKENode{ID: idAbc123, InstanceID: 456, Status: statusReady}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/nodes/abc-123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/nodes/abc-123")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(node); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKENodeGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: "123", keyNodeID: idAbc123})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, idAbc123) {
		t.Errorf("textContent.Text does not contain %v", idAbc123)
	}

	if !strings.Contains(textContent.Text, statusReady) {
		t.Errorf("textContent.Text does not contain %v", statusReady)
	}
}

// TestLinodeLKEKubeconfigGetTool verifies the LKE kubeconfig get tool
// registers correctly, validates cluster_id, and returns kubeconfig data.
func TestLinodeLKEKubeconfigGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEKubeconfigGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_kubeconfig_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_kubeconfig_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEKubeconfigGetToolCaseMissingClusterID(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEKubeconfigGetTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errClusterIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errClusterIDRequired)
	}
}

func TestLinodeLKEKubeconfigGetToolSuccess(t *testing.T) {
	t.Parallel()

	kubeconfig := linode.LKEKubeconfig{
		Kubeconfig: "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3Rlcg==",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/kubeconfig" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/kubeconfig")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(kubeconfig); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEKubeconfigGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3Rlcg==") {
		t.Errorf("textContent.Text does not contain %v", "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3Rlcg==")
	}
}

// TestLinodeLKEDashboardGetTool verifies the LKE dashboard get tool
// registers correctly and returns the dashboard URL.
func TestLinodeLKEDashboardGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKEDashboardGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != "linode_lke_dashboard_get" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_dashboard_get")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		dashboard := linode.LKEDashboard{URL: "https://dashboard.lke.example.com"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/lke/clusters/123/dashboard" {
				t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/dashboard")
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(dashboard); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEDashboardGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})

		result, err := srvHandler(t.Context(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Error("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, "https://dashboard.lke.example.com") {
			t.Errorf("textContent.Text does not contain %v", "https://dashboard.lke.example.com")
		}
	})
}

// TestLinodeLKEAPIEndpointsListTool verifies the LKE API endpoints list tool
// registers correctly and returns available API endpoints.
func TestLinodeLKEAPIEndpointsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKEAPIEndpointListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != "linode_lke_api_endpoint_list" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_api_endpoint_list")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		endpoints := []linode.LKEAPIEndpoint{
			{Endpoint: "https://abc123.us-east.lke.example.com:443"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/lke/clusters/123/api-endpoints" {
				t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/api-endpoints")
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyData: endpoints, keyPage: 1, keyPages: 1, keyResults: 1,
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEAPIEndpointListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})

		result, err := srvHandler(t.Context(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Error("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, "abc123.us-east.lke.example.com") {
			t.Errorf("textContent.Text does not contain %v", "abc123.us-east.lke.example.com")
		}
	})
}

// TestLinodeLKEACLGetTool verifies the LKE ACL get tool
// registers correctly and returns control plane ACL data.
func TestLinodeLKEACLGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKEACLGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != "linode_lke_acl_get" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_acl_get")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		// The Linode API wraps the ACL under a top-level "acl" key; the
		// handler must emit the bare ACL object. The extra field the proto
		// does not model must be dropped by the DiscardUnknown decode,
		// proving the output routes through the proto serializer.
		wrapped := map[string]any{
			keyACL: struct {
				linode.LKEControlPlaneACL

				NotInProto string `json:"not_in_proto"`
			}{
				LKEControlPlaneACL: linode.LKEControlPlaneACL{
					Enabled: true,
					Addresses: linode.LKEControlPlaneACLAddresses{
						IPv4: []string{cidrV4},
						IPv6: []string{cidrV6},
					},
				},
				NotInProto: valNotInProto,
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != lkeACLGetPath {
				t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, lkeACLGetPath)
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(wrapped); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEACLGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: "123"})

		result, err := srvHandler(t.Context(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Error("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, cidrV4) {
			t.Errorf("textContent.Text does not contain %v", cidrV4)
		}

		if !strings.Contains(textContent.Text, cidrV6) {
			t.Errorf("textContent.Text does not contain %v", cidrV6)
		}

		if strings.Contains(textContent.Text, "not_in_proto") {
			t.Error("unknown field not_in_proto leaked into proto-canonical output")
		}
	})
}

// TestLinodeLKEVersionsListTool verifies the LKE versions list tool
// registers correctly and returns available Kubernetes versions.
func TestLinodeLKEVersionsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKEVersionListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != "linode_lke_version_list" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_version_list")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		versions := []linode.LKEVersion{{ID: lkeVersion129}, {ID: lkeVersion128}, {ID: "1.27"}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/lke/versions" {
				t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/versions")
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyData: versions, keyPage: 1, keyPages: 1, keyResults: 3,
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKEVersionListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})

		result, err := srvHandler(t.Context(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Error("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, lkeVersion129) {
			t.Errorf("textContent.Text does not contain %v", lkeVersion129)
		}

		if !strings.Contains(textContent.Text, lkeVersion128) {
			t.Errorf("textContent.Text does not contain %v", lkeVersion128)
		}
	})
}

// TestLinodeLKEVersionGetTool verifies the LKE version get tool
// registers correctly, validates the version parameter, and returns version details.
func TestLinodeLKEVersionGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEVersionGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_version_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_version_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEVersionGetToolMissingVersion(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEVersionGetTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "version is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "version is required")
	}
}

func TestLinodeLKEVersionGetToolInvalidVersionPathParameter(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEVersionGetTool(cfg)

	t.Parallel()

	for _, versionCase := range []struct {
		name  string
		value string
	}{
		{name: "separator version", value: lkeVersionWithSlash},
		{name: "query version", value: lkeVersionWithQuery},
		{name: "traversal version", value: lkeVersionTraversal},
	} {
		t.Run(versionCase.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, map[string]any{keyVersion: versionCase.value})

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "version must be a Kubernetes version ID") {
				t.Errorf("error text %q does not contain %q", text.Text, "version must be a Kubernetes version ID")
			}
		})
	}
}

func TestLinodeLKEVersionGetToolSuccess(t *testing.T) {
	t.Parallel()

	version := linode.LKEVersion{ID: lkeVersion129}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/versions/1.29" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/versions/1.29")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(version); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEVersionGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyVersion: lkeVersion129})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, lkeVersion129) {
		t.Errorf("textContent.Text does not contain %v", lkeVersion129)
	}
}

// TestLinodeLKETypesListTool verifies the LKE types list tool
// registers correctly and returns available LKE node types.
func TestLinodeLKETypesListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKETypeListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != "linode_lke_type_list" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_type_list")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		types := []linode.LKEType{
			{
				ID: typeG6Standard2, Label: typeLinode4GB, Transfer: 4000,
				Price: linode.LKETypePrice{Hourly: 0.036, Monthly: 24.0},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/lke/types" {
				t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/types")
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyData: types, keyPage: 1, keyPages: 1, keyResults: 1,
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKETypeListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})

		result, err := srvHandler(t.Context(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Error("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, typeG6Standard2) {
			t.Errorf("textContent.Text does not contain %v", typeG6Standard2)
		}

		if !strings.Contains(textContent.Text, `"lke_types"`) {
			t.Errorf("textContent.Text does not contain the lke_types key: %s", textContent.Text)
		}

		if count := listResponseCount(t, textContent.Text); count != 1 {
			t.Errorf("listResponseCount = %d, want 1", count)
		}
	})
}

// TestLinodeLKETierVersionsListTool verifies the LKE tier versions list tool
// registers correctly and returns tier version data.
const (
	keyLKETier              = "tier"
	errLKETierInvalidChoice = "tier must be one of: standard, enterprise"
)

func TestLinodeLKETierVersionsListToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeLKETierVersionListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_tier_version_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_tier_version_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyLKETier) {
		t.Errorf("tool.RawInputSchema missing key %v", keyLKETier)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKETierVersionsListToolSuccess(t *testing.T) {
	t.Parallel()

	tierVersions := []linode.LKETierVersion{
		{ID: lkeVersion129, Tier: classStandard},
		{ID: lkeVersion128, Tier: classStandard},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/tiers/standard/versions" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/tiers/standard/versions")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: tierVersions, keyPage: 1, keyPages: 1, keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKETierVersionListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLKETier: classStandard})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, classStandard) {
		t.Errorf("textContent.Text does not contain %v", classStandard)
	}
}

func TestLinodeLKETierVersionsListToolInvalid(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeLKETierVersionListTool(cfg)

	invalidCases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing tier", args: map[string]any{}, want: "tier is required"},
		{name: "slash tier", args: map[string]any{keyLKETier: "standard/enterprise"}, want: errLKETierInvalidChoice},
		{name: "query tier", args: map[string]any{keyLKETier: "standard?x=1"}, want: errLKETierInvalidChoice},
		{name: "traversal tier", args: map[string]any{keyLKETier: pathTraversalValue}, want: errLKETierInvalidChoice},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

// End-to-end verification of LKE cluster creation workflow.
func TestLinodeLKEClusterCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_cluster_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_cluster_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{monitorAlertDefinitionLabelParam, keySupportTicketRegion, keyK8sVersion, keyNodePools, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeLKEClusterCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEClusterCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseMissingConfirm,
			args:         map[string]any{keyLabel: labelTestCluster, keyRegion: regionUSEast, keyK8sVersion: lkeVersion129, keyNodePools: lkePoolSnapshot},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingLabel,
			args:         map[string]any{keyRegion: regionUSEast, keyK8sVersion: lkeVersion129, keyNodePools: lkePoolSnapshot, keyConfirm: true},
			wantContains: errLabelRequired,
		},
		{
			name:         caseMissingRegion,
			args:         map[string]any{keyLabel: labelTestCluster, keyK8sVersion: lkeVersion129, keyNodePools: lkePoolSnapshot, keyConfirm: true},
			wantContains: errRegionRequired,
		},
		{
			name:         "invalid node pools type",
			args:         map[string]any{keyLabel: labelTestCluster, keyRegion: regionUSEast, keyK8sVersion: lkeVersion129, keyNodePools: "not-valid-json", keyConfirm: true},
			wantContains: "node_pools must be an array of objects",
		},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeLKEClusterCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	cluster := linode.LKECluster{
		ID: 999, Label: labelTestCluster, Region: regionUSEast, K8sVersion: lkeVersion129, Status: statusReady,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(cluster); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEClusterCreateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLabel: labelTestCluster, keyRegion: regionUSEast, keyK8sVersion: lkeVersion129,
		keyNodePools: lkePoolSnapshot, keyConfirm: true,
	})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, labelTestCluster) {
		t.Errorf("textContent.Text does not contain %v", labelTestCluster)
	}

	if !strings.Contains(textContent.Text, "999") {
		t.Errorf("textContent.Text does not contain %v", "999")
	}
}

// TestLinodeLKEClusterUpdateTool verifies the LKE cluster update tool
// registers correctly, validates required fields, and updates clusters.
func TestLinodeLKEClusterUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_cluster_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_cluster_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyClusterID, monitorAlertDefinitionLabelParam, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeLKEClusterUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEClusterUpdateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyClusterID: float64(123), keyLabel: labelNew}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingClusterID, args: map[string]any{keyLabel: labelNew, keyConfirm: true}, wantContains: errClusterIDRequired},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeLKEClusterUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	cluster := linode.LKECluster{
		ID: 123, Label: "updated-cluster", Region: regionUSEast, K8sVersion: lkeVersion129, Status: statusReady,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != lkeClusterGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, lkeClusterGetPath)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(cluster); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEClusterUpdateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyLabel: "updated-cluster", keyConfirm: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

// TestLinodeLKEClusterDeleteTool verifies the LKE cluster delete tool
// registers correctly, validates required fields, and deletes clusters.
func TestLinodeLKEClusterDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_cluster_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_cluster_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}
}

func TestLinodeLKEClusterDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyClusterID: float64(123)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingClusterID, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errClusterIDRequired},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeLKEClusterDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != lkeClusterGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, lkeClusterGetPath)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEClusterDeleteTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

// TestLinodeLKEClusterRecycleTool verifies the LKE cluster recycle tool
// registers correctly, validates confirm, and recycles cluster nodes.
func TestLinodeLKEClusterRecycleToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterRecycleTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_cluster_recycle" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_cluster_recycle")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEClusterRecycleToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEClusterRecycleTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeLKEClusterRecycleToolSuccessfulRecycle(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/recycle" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/recycle")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEClusterRecycleTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "recycle initiated successfully") {
		t.Errorf("textContent.Text does not contain %v", "recycle initiated successfully")
	}
}

// TestLinodeLKEClusterRegenerateTool verifies the LKE cluster regenerate tool
// registers correctly, validates confirm, and regenerates cluster credentials.
func TestLinodeLKEClusterRegenerateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEClusterRegenerateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_cluster_regenerate" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_cluster_regenerate")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEClusterRegenerateToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEClusterRegenerateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeLKEClusterRegenerateToolSuccessfulRegenerate(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/regenerate" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/regenerate")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEClusterRegenerateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "regenerated successfully") {
		t.Errorf("textContent.Text does not contain %v", "regenerated successfully")
	}
}

// End-to-end verification of LKE pool creation workflow.
func TestLinodeLKEPoolCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_pool_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_pool_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyClusterID, keyType, keyCount, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeLKEPoolCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEPoolCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyClusterID: float64(123), keyType: typeG6Standard2, keyCount: float64(3)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingType, args: map[string]any{keyClusterID: float64(123), keyCount: float64(3), keyConfirm: true}, wantContains: errTypeRequired},
		{name: "missing count", args: map[string]any{keyClusterID: float64(123), keyType: typeG6Standard2, keyConfirm: true}, wantContains: "count is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeLKEPoolCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	pool := linode.LKENodePool{ID: 50, ClusterID: 123, Type: typeG6Standard2, Count: 3}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/pools" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/pools")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(pool); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEPoolCreateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyClusterID: float64(123), keyType: typeG6Standard2, keyCount: float64(3), keyConfirm: true,
	})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, typeG6Standard2) {
		t.Errorf("textContent.Text does not contain %v", typeG6Standard2)
	}

	var payload struct {
		Message string `json:"message"`
		Pool    struct {
			ID        int    `json:"id"`
			ClusterID int    `json:"cluster_id"`
			Type      string `json:"type"`
			Count     int    `json:"count"`
		} `json:"pool"`
	}
	if err := json.Unmarshal([]byte(textContent.Text), &payload); err != nil {
		t.Fatalf("unmarshal write response: %v", err)
	}

	if payload.Pool.ID != 50 || payload.Pool.ClusterID != 123 ||
		payload.Pool.Type != typeG6Standard2 || payload.Pool.Count != 3 {
		t.Errorf("pool element = %+v, want full proto element with id 50, cluster 123", payload.Pool)
	}
}

// TestLinodeLKEPoolUpdateTool verifies the LKE pool update tool
// registers correctly, validates confirm, and updates node pools.
func TestLinodeLKEPoolUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_pool_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_pool_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEPoolUpdateToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEPoolUpdateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyPoolID: float64(10), keyCount: float64(5)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeLKEPoolUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	pool := linode.LKENodePool{ID: 10, ClusterID: 123, Type: typeG6Standard2, Count: 5}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != lkePoolGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, lkePoolGetPath)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(pool); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEPoolUpdateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyClusterID: float64(123), keyPoolID: float64(10), keyCount: float64(5), keyConfirm: true,
	})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

// TestLinodeLKEPoolDeleteTool verifies the LKE pool delete tool
// registers correctly, validates confirm, and deletes node pools.
func TestLinodeLKEPoolDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_pool_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_pool_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEPoolDeleteToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEPoolDeleteTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyPoolID: float64(10)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeLKEPoolDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != lkePoolGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, lkePoolGetPath)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEPoolDeleteTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyPoolID: float64(10), keyConfirm: true, keyConfirmedDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "deleted") {
		t.Errorf("textContent.Text does not contain %v", "deleted")
	}
}

// TestLinodeLKEPoolRecycleTool verifies the LKE pool recycle tool
// registers correctly, validates confirm, and recycles pool nodes.
func TestLinodeLKEPoolRecycleToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEPoolRecycleTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_pool_recycle" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_pool_recycle")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEPoolRecycleToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEPoolRecycleTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyPoolID: float64(10)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeLKEPoolRecycleToolSuccessfulRecycle(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/pools/10/recycle" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/pools/10/recycle")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEPoolRecycleTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyPoolID: float64(10), keyConfirm: true, keyConfirmedDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "recycle initiated successfully") {
		t.Errorf("textContent.Text does not contain %v", "recycle initiated successfully")
	}
}

// TestLinodeLKENodeDeleteTool verifies the LKE node delete tool
// registers correctly, validates required fields, and deletes nodes.
func TestLinodeLKENodeDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_node_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_node_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKENodeDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyClusterID: float64(123), keyNodeID: idAbc123}, wantContains: errConfirmEqualsTrue},
		{name: "missing node id", args: map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: "node_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeLKENodeDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/nodes/abc-123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/nodes/abc-123")
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKENodeDeleteTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyNodeID: idAbc123, keyConfirm: true, keyConfirmedDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "deleted") {
		t.Errorf("textContent.Text does not contain %v", "deleted")
	}

	if !strings.Contains(textContent.Text, idAbc123) {
		t.Errorf("textContent.Text does not contain %v", idAbc123)
	}
}

// TestLinodeLKENodeRecycleTool verifies the LKE node recycle tool
// registers correctly, validates confirm, and recycles individual nodes.
func TestLinodeLKENodeRecycleToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKENodeRecycleTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_node_recycle" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_node_recycle")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKENodeRecycleToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKENodeRecycleTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyNodeID: idAbc123})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeLKENodeRecycleToolSuccessfulRecycle(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/nodes/abc-123/recycle" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/nodes/abc-123/recycle")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKENodeRecycleTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyNodeID: idAbc123, keyConfirm: true, keyConfirmedDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "recycle initiated successfully") {
		t.Errorf("textContent.Text does not contain %v", "recycle initiated successfully")
	}
}

// TestLinodeLKEKubeconfigDeleteTool verifies the LKE kubeconfig delete tool
// registers correctly, validates confirm, and regenerates the kubeconfig.
func TestLinodeLKEKubeconfigDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEKubeconfigDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_kubeconfig_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_kubeconfig_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEKubeconfigDeleteToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEKubeconfigDeleteTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeLKEKubeconfigDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/kubeconfig" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/kubeconfig")
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEKubeconfigDeleteTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "regenerated successfully") {
		t.Errorf("textContent.Text does not contain %v", "regenerated successfully")
	}
}

// TestLinodeLKEServiceTokenDeleteTool verifies the LKE service token delete tool
// registers correctly, validates confirm, and regenerates the service token.
func TestLinodeLKEServiceTokenDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEServiceTokenDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_service_token_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_service_token_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEServiceTokenDeleteToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEServiceTokenDeleteTool(cfg)

	confirmCases := []struct {
		name  string
		value any
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirmRejected, value: false},
		{name: caseStringConfirmRejected, value: boolStringTrue},
		{name: caseNumericConfirmRejected, value: float64(1)},
	}
	for i := range confirmCases {
		confirmCase := confirmCases[i]
		t.Run(confirmCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyClusterID: float64(123)}
			if confirmCase.value != nil {
				args[keyConfirm] = confirmCase.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeLKEServiceTokenDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/clusters/123/servicetoken" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/clusters/123/servicetoken")
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEServiceTokenDeleteTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "regenerated successfully") {
		t.Errorf("textContent.Text does not contain %v", "regenerated successfully")
	}
}

// TestLinodeLKEACLUpdateTool verifies the LKE ACL update tool
// registers correctly, validates confirm, and updates control plane ACLs.
func TestLinodeLKEACLUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEACLUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_acl_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_acl_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyClusterID, keyACL, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeLKEACLUpdateToolRejectsNonTrueConfirmBeforeClientCall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		confirm any
		include bool
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, confirm: false, include: true},
		{name: caseStringConfirm, confirm: boolStringTrue, include: true},
		{name: caseNumericConfirm, confirm: 1, include: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := make(chan struct{}, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called <- struct{}{}

				w.WriteHeader(http.StatusTeapot)
			}))
			t.Cleanup(srv.Close)

			srvCfg := &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				},
			}
			_, _, srvHandler := tools.NewLinodeLKEACLUpdateTool(srvCfg)

			args := map[string]any{keyClusterID: float64(123), statusEnabled: true}
			if tt.include {
				args[keyConfirm] = tt.confirm
			}

			req := createRequestWithArgs(t, args)

			result, err := srvHandler(t.Context(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			select {
			case <-called:
				t.Error("handler should reject confirm before client call")
			default:
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeLKEACLUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	acl := linode.LKEControlPlaneACL{
		Enabled: true,
		Addresses: linode.LKEControlPlaneACLAddresses{
			IPv4: []string{cidrV4, cidrV4Secondary},
			IPv6: []string{cidrV6},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != lkeACLGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, lkeACLGetPath)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		var got linode.UpdateLKEControlPlaneACLRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got.ACL, acl) {
			t.Errorf("got.ACL = %v, want %v", got.ACL, acl)
		}

		w.Header().Set("Content-Type", "application/json")

		// The Linode API wraps the updated ACL under a top-level "acl" key.
		if err := json.NewEncoder(w).Encode(map[string]any{"acl": acl}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEACLUpdateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyClusterID: float64(123),
		"acl": map[string]any{
			statusEnabled: true,
			"addresses": map[string]any{
				keyIPv4: []any{cidrV4, cidrV4Secondary},
				tcIpv6:  []any{cidrV6},
			},
		},
		keyConfirm: true,
	})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}

	// The output must carry the full acl object, not the stripped/empty ACL the
	// old decode-bug produced: {message, acl:{enabled, addresses:{ipv4,ipv6}}}.
	var envelope struct {
		Message string `json:"message"`
		ACL     struct {
			Enabled   bool `json:"enabled"`
			Addresses struct {
				IPv4 []string `json:"ipv4"`
				IPv6 []string `json:"ipv6"`
			} `json:"addresses"`
		} `json:"acl"`
	}
	if err := json.Unmarshal([]byte(textContent.Text), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !envelope.ACL.Enabled {
		t.Error("envelope.ACL.Enabled = false, want true")
	}

	if !reflect.DeepEqual(envelope.ACL.Addresses.IPv4, []string{cidrV4, cidrV4Secondary}) {
		t.Errorf("envelope.ACL.Addresses.IPv4 = %v, want %v", envelope.ACL.Addresses.IPv4, []string{cidrV4, cidrV4Secondary})
	}

	if !reflect.DeepEqual(envelope.ACL.Addresses.IPv6, []string{cidrV6}) {
		t.Errorf("envelope.ACL.Addresses.IPv6 = %v, want %v", envelope.ACL.Addresses.IPv6, []string{cidrV6})
	}
}

// TestLinodeLKEACLDeleteTool verifies the LKE ACL delete tool
// registers correctly, validates confirm, and deletes control plane ACLs.
func TestLinodeLKEACLDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeLKEACLDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_acl_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_acl_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeLKEACLDeleteToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKEACLDeleteTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeLKEACLDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != lkeACLGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, lkeACLGetPath)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKEACLDeleteTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

// Dry-run coverage for LKE cluster delete. Sibling function keeps the
// main test's subtest count below maintidx's threshold.
func TestLinodeLKEClusterDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeLKEClusterDeleteTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeLKEClusterDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	clusterBody := `{"id":123,"label":"prod-cluster","region":"us-east","k8s_version":"1.29","status":"ready"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)

		if r.Method != http.MethodGet {
			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == lkeClusterGetPath {
			_, _ = w.Write([]byte(clusterBody))

			return
		}

		// The Tier A walk also lists node pools; an empty page keeps
		// this subtest on the no-mutation and preview-shape contract.
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyClusterID: float64(123),
		keyDryRun:    true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Error("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_lke_cluster_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_lke_cluster_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Error("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], lkeClusterGetPath) {
		t.Errorf("got %v, want %v", would["path"], lkeClusterGetPath)
	}

	if len(methodsSeen) == 0 {
		t.Error("methodsSeen is empty")
	}

	if slices.Contains(methodsSeen, http.MethodDelete) {
		t.Errorf("methodsSeen should not contain %v", http.MethodDelete)
	}
}

func TestLinodeLKEClusterDeleteToolDryRunStillValidatesClusterId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeLKEClusterDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errClusterIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errClusterIDRequired)
	}
}

// Dry-run coverage for LKE pool delete via the ByTwoIDs helper.
func TestLinodeLKEPoolDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeLKEPoolDeleteTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeLKEPoolDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	poolBody := `{"id":10,"count":3,"type":"g6-standard-2","nodes":[]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != lkePoolGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, lkePoolGetPath)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(poolBody))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeLKEPoolDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyClusterID: float64(123),
		keyPoolID:    float64(10),
		keyDryRun:    true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Error("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_lke_pool_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_lke_pool_delete")
	}

	would, wouldOK := body["would_execute"].(map[string]any)
	if !wouldOK {
		t.Error("wouldOK = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], lkePoolGetPath) {
		t.Errorf("got %v, want %v", would["path"], lkePoolGetPath)
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeLKEPoolDeleteToolDryRunStillValidatesClusterId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeLKEPoolDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyPoolID: float64(10),
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errClusterIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errClusterIDRequired)
	}
}

// Dry-run coverage for LKE node delete (mixed int + string IDs).
func TestLinodeLKENodeDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeLKENodeDeleteTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeLKENodeDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	nodeBody := `{"id":"123-abc","instance_id":456,"status":"ready"}`
	expectedPath := "/lke/clusters/123/nodes/123-abc"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != expectedPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, expectedPath)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(nodeBody))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyClusterID: float64(123),
		keyNodeID:    "123-abc",
		keyDryRun:    true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Error("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_lke_node_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_lke_node_delete")
	}

	would, wouldOK := body["would_execute"].(map[string]any)
	if !wouldOK {
		t.Error("wouldOK = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], expectedPath) {
		t.Errorf("got %v, want %v", would["path"], expectedPath)
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeLKENodeDeleteToolDryRunStillValidatesNodeId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeLKENodeDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyClusterID: float64(123),
		keyDryRun:    true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "node_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "node_id is required")
	}
}

// Dry-run coverage for LKE kubeconfig delete. The fetch returns the
// CLUSTER state (not the kubeconfig contents) so dry-run never surfaces
// credential material to the model. Locks that design choice.
func TestLinodeLKEKubeconfigDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEKubeconfigDeleteTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview fetches cluster, never kubeconfig", func(t *testing.T) {
		t.Parallel()

		var pathsSeen []string

		clusterBody := `{"id":123,"label":"prod-cluster","region":"us-east"}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pathsSeen = append(pathsSeen, r.Method+" "+r.URL.Path)

			if r.Method == http.MethodGet && r.URL.Path == lkeClusterGetPath {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(clusterBody))

				return
			}

			t.Errorf("dry_run must only GET cluster metadata, never touch kubeconfig; got %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeLKEKubeconfigDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		})

		result, err := handler(t.Context(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, isText := result.Content[0].(mcp.TextContent)
		if !isText {
			t.Error("isText = false, want true")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_lke_kubeconfig_delete") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_kubeconfig_delete")
		}

		would, wouldOK := body["would_execute"].(map[string]any)
		if !wouldOK {
			t.Error("wouldOK = false, want true")
		}

		if !reflect.DeepEqual(would["method"], "DELETE") {
			t.Errorf("got %v, want %v", would["method"], "DELETE")
		}

		if !reflect.DeepEqual(would["path"], "/lke/clusters/123/kubeconfig") {
			t.Errorf("got %v, want %v", would["path"], "/lke/clusters/123/kubeconfig")
		}

		if !reflect.DeepEqual(pathsSeen, []string{tcGETLkeClusters123}) {
			t.Errorf("pathsSeen = %v, want %v", pathsSeen, []string{tcGETLkeClusters123})
		}

		state, stateOK := body["current_state"].(map[string]any)
		if !stateOK {
			t.Error("stateOK = false, want true")
		}

		if _, ok := state["kubeconfig"]; ok {
			t.Errorf("state has unexpected key %v", "kubeconfig")
		}
	})
}

// Dry-run coverage for LKE service token delete. Same safety design as
// kubeconfig delete: fetch the cluster, not the token.
func TestLinodeLKEServiceTokenDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEServiceTokenDeleteTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview fetches cluster, never service token", func(t *testing.T) {
		t.Parallel()

		var pathsSeen []string

		clusterBody := `{"id":123,"label":"prod-cluster"}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pathsSeen = append(pathsSeen, r.Method+" "+r.URL.Path)

			if r.Method == http.MethodGet && r.URL.Path == lkeClusterGetPath {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(clusterBody))

				return
			}

			t.Errorf("dry_run must only GET cluster metadata; got %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeLKEServiceTokenDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		})

		result, err := handler(t.Context(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, isText := result.Content[0].(mcp.TextContent)
		if !isText {
			t.Error("isText = false, want true")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_lke_service_token_delete") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_service_token_delete")
		}

		would, wouldOK := body["would_execute"].(map[string]any)
		if !wouldOK {
			t.Error("wouldOK = false, want true")
		}

		if !reflect.DeepEqual(would["path"], "/lke/clusters/123/servicetoken") {
			t.Errorf("got %v, want %v", would["path"], "/lke/clusters/123/servicetoken")
		}

		if !reflect.DeepEqual(pathsSeen, []string{tcGETLkeClusters123}) {
			t.Errorf("pathsSeen = %v, want %v", pathsSeen, []string{tcGETLkeClusters123})
		}
	})
}
