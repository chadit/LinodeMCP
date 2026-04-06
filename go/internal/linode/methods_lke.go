package linode

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

const (
	endpointLKEClusters     = "/lke/clusters"
	endpointLKEVersions     = "/lke/versions"
	endpointLKETypes        = "/lke/types"
	endpointLKETierVersions = "/lke/tiers/versions"
)

// ListLKEClusters retrieves all LKE clusters for the authenticated user.
func (c *Client) httpListLKEClusters(ctx context.Context) ([]LKECluster, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointLKEClusters, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListLKEClusters", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[LKECluster]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetLKECluster retrieves a single LKE cluster by its ID.
func (c *Client) httpGetLKECluster(ctx context.Context, clusterID int) (*LKECluster, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKECluster", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var cluster LKECluster
	if err := c.handleResponse(resp, &cluster); err != nil {
		return nil, err
	}

	return &cluster, nil
}

// CreateLKECluster creates a new LKE cluster.
func (c *Client) httpCreateLKECluster(ctx context.Context, req *CreateLKEClusterRequest) (*LKECluster, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointLKEClusters, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateLKECluster", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var cluster LKECluster
	if err := c.handleResponse(resp, &cluster); err != nil {
		return nil, err
	}

	return &cluster, nil
}

// UpdateLKECluster updates an existing LKE cluster.
func (c *Client) httpUpdateLKECluster(ctx context.Context, clusterID int, req UpdateLKEClusterRequest) (*LKECluster, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateLKECluster", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var cluster LKECluster
	if err := c.handleResponse(resp, &cluster); err != nil {
		return nil, err
	}

	return &cluster, nil
}

// DeleteLKECluster deletes an LKE cluster.
func (c *Client) httpDeleteLKECluster(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKECluster", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// RecycleLKECluster recycles all nodes in an LKE cluster.
func (c *Client) httpRecycleLKECluster(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/recycle", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "RecycleLKECluster", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// RegenerateLKECluster regenerates the service token for an LKE cluster.
func (c *Client) httpRegenerateLKECluster(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/regenerate", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "RegenerateLKECluster", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ListLKENodePools retrieves all node pools for an LKE cluster.
func (c *Client) httpListLKENodePools(ctx context.Context, clusterID int) ([]LKENodePool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListLKENodePools", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[LKENodePool]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetLKENodePool retrieves a single node pool by its ID.
func (c *Client) httpGetLKENodePool(ctx context.Context, clusterID, poolID int) (*LKENodePool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools/%d", clusterID, poolID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKENodePool", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var pool LKENodePool
	if err := c.handleResponse(resp, &pool); err != nil {
		return nil, err
	}

	return &pool, nil
}

// CreateLKENodePool creates a new node pool for an LKE cluster.
func (c *Client) httpCreateLKENodePool(ctx context.Context, clusterID int, req *CreateLKENodePoolRequest) (*LKENodePool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateLKENodePool", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var pool LKENodePool
	if err := c.handleResponse(resp, &pool); err != nil {
		return nil, err
	}

	return &pool, nil
}

// UpdateLKENodePool updates an existing node pool.
func (c *Client) httpUpdateLKENodePool(ctx context.Context, clusterID, poolID int, req UpdateLKENodePoolRequest) (*LKENodePool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools/%d", clusterID, poolID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateLKENodePool", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var pool LKENodePool
	if err := c.handleResponse(resp, &pool); err != nil {
		return nil, err
	}

	return &pool, nil
}

// DeleteLKENodePool deletes a node pool from an LKE cluster.
func (c *Client) httpDeleteLKENodePool(ctx context.Context, clusterID, poolID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools/%d", clusterID, poolID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKENodePool", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// RecycleLKENodePool recycles all nodes in a specific node pool.
func (c *Client) httpRecycleLKENodePool(ctx context.Context, clusterID, poolID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools/%d/recycle", clusterID, poolID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "RecycleLKENodePool", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// GetLKENode retrieves a single node by its ID within an LKE cluster.
func (c *Client) httpGetLKENode(ctx context.Context, clusterID int, nodeID string) (*LKENode, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/nodes/%s", clusterID, url.PathEscape(nodeID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKENode", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var node LKENode
	if err := c.handleResponse(resp, &node); err != nil {
		return nil, err
	}

	return &node, nil
}

// DeleteLKENode deletes a specific node from an LKE cluster.
func (c *Client) httpDeleteLKENode(ctx context.Context, clusterID int, nodeID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/nodes/%s", clusterID, url.PathEscape(nodeID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKENode", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// RecycleLKENode recycles a specific node in an LKE cluster.
func (c *Client) httpRecycleLKENode(ctx context.Context, clusterID int, nodeID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/nodes/%s/recycle", clusterID, url.PathEscape(nodeID))

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "RecycleLKENode", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// GetLKEKubeconfig retrieves the kubeconfig for an LKE cluster.
func (c *Client) httpGetLKEKubeconfig(ctx context.Context, clusterID int) (*LKEKubeconfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/kubeconfig", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKEKubeconfig", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var kubeconfig LKEKubeconfig
	if err := c.handleResponse(resp, &kubeconfig); err != nil {
		return nil, err
	}

	return &kubeconfig, nil
}

// DeleteLKEKubeconfig deletes and regenerates the kubeconfig for an LKE cluster.
func (c *Client) httpDeleteLKEKubeconfig(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/kubeconfig", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKEKubeconfig", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// GetLKEDashboard retrieves the dashboard URL for an LKE cluster.
func (c *Client) httpGetLKEDashboard(ctx context.Context, clusterID int) (*LKEDashboard, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/dashboard", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKEDashboard", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var dashboard LKEDashboard
	if err := c.handleResponse(resp, &dashboard); err != nil {
		return nil, err
	}

	return &dashboard, nil
}

// ListLKEAPIEndpoints retrieves the API endpoints for an LKE cluster.
func (c *Client) httpListLKEAPIEndpoints(ctx context.Context, clusterID int) ([]LKEAPIEndpoint, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/api-endpoints", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListLKEAPIEndpoints", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[LKEAPIEndpoint]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// DeleteLKEServiceToken deletes and regenerates the service token for an LKE cluster.
func (c *Client) httpDeleteLKEServiceToken(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/service-token", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKEServiceToken", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// GetLKEControlPlaneACL retrieves the control plane ACL for an LKE cluster.
func (c *Client) httpGetLKEControlPlaneACL(ctx context.Context, clusterID int) (*LKEControlPlaneACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/control-plane-acl", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKEControlPlaneACL", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var acl LKEControlPlaneACL
	if err := c.handleResponse(resp, &acl); err != nil {
		return nil, err
	}

	return &acl, nil
}

// UpdateLKEControlPlaneACL updates the control plane ACL for an LKE cluster.
func (c *Client) httpUpdateLKEControlPlaneACL(ctx context.Context, clusterID int, req UpdateLKEControlPlaneACLRequest) (*LKEControlPlaneACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/control-plane-acl", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateLKEControlPlaneACL", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var acl LKEControlPlaneACL
	if err := c.handleResponse(resp, &acl); err != nil {
		return nil, err
	}

	return &acl, nil
}

// DeleteLKEControlPlaneACL deletes the control plane ACL for an LKE cluster.
func (c *Client) httpDeleteLKEControlPlaneACL(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/control-plane-acl", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKEControlPlaneACL", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ListLKEVersions retrieves all available Kubernetes versions for LKE.
func (c *Client) httpListLKEVersions(ctx context.Context) ([]LKEVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointLKEVersions, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListLKEVersions", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[LKEVersion]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetLKEVersion retrieves a specific Kubernetes version for LKE.
func (c *Client) httpGetLKEVersion(ctx context.Context, versionID string) (*LKEVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("%s/%s", endpointLKEVersions, url.PathEscape(versionID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKEVersion", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var version LKEVersion
	if err := c.handleResponse(resp, &version); err != nil {
		return nil, err
	}

	return &version, nil
}

// ListLKETypes retrieves all available node types for LKE clusters.
func (c *Client) httpListLKETypes(ctx context.Context) ([]LKEType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointLKETypes, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListLKETypes", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[LKEType]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListLKETierVersions retrieves all available LKE tier versions.
func (c *Client) httpListLKETierVersions(ctx context.Context) ([]LKETierVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointLKETierVersions, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListLKETierVersions", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[LKETierVersion]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}
