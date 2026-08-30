package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

// The cluster and pool read paths the LKE cases assert on. Rehomed here when
// the write dry-run file they lived in was emptied by the migration.
const (
	lkeClusterGetPath = "/lke/clusters/123"
	lkePoolGetPath    = "/lke/clusters/123/pools/10"
)

// End-to-end verification of LKE cluster listing and filtering.
func TestLinodeLKEClustersListToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := gentools.NewLinodeLkeClusterListTool(cfg)

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

	clusters := []LKECluster{
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
	_, _, srvHandler := gentools.NewLinodeLkeClusterListTool(srvCfg)

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

	clusters := []LKECluster{
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
	_, _, srvHandler := gentools.NewLinodeLkeClusterListTool(srvCfg)

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
	tool, _, handler := gentools.NewLinodeLkeClusterGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeLkeClusterGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingClusterID, args: map[string]any{}, wantContains: errClusterIDRequired},
		{name: "invalid cluster id", args: map[string]any{keyClusterID: notANumber}, wantContains: "cluster_id must be a positive integer"},
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

	cluster := LKECluster{
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
	_, _, srvHandler := gentools.NewLinodeLkeClusterGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: 123})

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
	tool, _, handler := gentools.NewLinodeLkePoolListTool(cfg)

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
	_, _, handler := gentools.NewLinodeLkePoolListTool(cfg)

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

	pools := []LKENodePool{
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
	_, _, srvHandler := gentools.NewLinodeLkePoolListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: 123})

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
	tool, _, handler := gentools.NewLinodeLkePoolGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeLkePoolGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingClusterID, args: map[string]any{keyPoolID: 10}, wantContains: errClusterIDRequired},
		{name: "missing pool id", args: map[string]any{keyClusterID: 123}, wantContains: "pool_id is required"},
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

	pool := LKENodePool{ID: 10, ClusterID: 123, Type: typeG6Standard2, Count: 3}

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
	_, _, srvHandler := gentools.NewLinodeLkePoolGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: 123, keyPoolID: 10})

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
	tool, _, handler := gentools.NewLinodeLkeNodeGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeLkeNodeGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingClusterID, args: map[string]any{keyNodeID: idAbc123}, wantContains: errClusterIDRequired},
		{name: "missing node id", args: map[string]any{keyClusterID: 123}, wantContains: "node_id is required"},
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

	node := LKENode{ID: idAbc123, InstanceID: 456, Status: statusReady}

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
	_, _, srvHandler := gentools.NewLinodeLkeNodeGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: 123, keyNodeID: idAbc123})

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
	tool, _, handler := gentools.NewLinodeLkeKubeconfigGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeLkeKubeconfigGetTool(cfg)

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

	kubeconfig := LKEKubeconfig{
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
	_, _, srvHandler := gentools.NewLinodeLkeKubeconfigGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyClusterID: 123})

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
	tool, _, handler := gentools.NewLinodeLkeDashboardGetTool(cfg)

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

		dashboard := LKEDashboard{URL: "https://dashboard.lke.example.com"}

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
		_, _, srvHandler := gentools.NewLinodeLkeDashboardGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: 123})

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
	tool, _, handler := gentools.NewLinodeLkeAPIEndpointListTool(cfg)

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

		endpoints := []LKEAPIEndpoint{
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
		_, _, srvHandler := gentools.NewLinodeLkeAPIEndpointListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyClusterID: 123})

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

// TestLinodeLKEVersionsListTool verifies the LKE versions list tool
// registers correctly and returns available Kubernetes versions.
func TestLinodeLKEVersionsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := gentools.NewLinodeLkeVersionListTool(cfg)

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

		versions := []LKEVersion{{ID: lkeVersion129}, {ID: lkeVersion128}, {ID: "1.27"}}

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
		_, _, srvHandler := gentools.NewLinodeLkeVersionListTool(srvCfg)

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
	tool, _, handler := gentools.NewLinodeLkeVersionGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeLkeVersionGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeLkeVersionGetTool(cfg)

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

	version := LKEVersion{ID: lkeVersion129}

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
	_, _, srvHandler := gentools.NewLinodeLkeVersionGetTool(srvCfg)

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
	tool, _, handler := gentools.NewLinodeLkeTypeListTool(cfg)

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

		types := []LKEType{
			{
				ID: typeG6Standard2, Label: typeLinode4GB, Transfer: 4000,
				Price: LKETypePrice{Hourly: 0.036, Monthly: 24.0},
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
		_, _, srvHandler := gentools.NewLinodeLkeTypeListTool(srvCfg)

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
	tool, _, handler := gentools.NewLinodeLkeTierVersionListTool(cfg)

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

	tierVersions := []LKETierVersion{
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
	_, _, srvHandler := gentools.NewLinodeLkeTierVersionListTool(srvCfg)

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
	_, _, handler := gentools.NewLinodeLkeTierVersionListTool(cfg)

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
