package linode

import (
	"context"
)

// Every method here resolves its path from the tool's declared route, so none of
// them spells a URL or escapes a path segment.

// GetLKECluster retrieves a single LKE cluster by its ID.
func (c *Client) httpGetLKECluster(ctx context.Context, clusterID int) (*LKECluster, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_cluster_get", nil, clusterID)
	if err != nil {
		return nil, wrapRequestError("GetLKECluster", err)
	}

	defer drainClose(resp)

	var cluster LKECluster
	if err := c.handleResponse(resp, &cluster); err != nil {
		return nil, err
	}

	return &cluster, nil
}

// ListLKENodePools retrieves all node pools for an LKE cluster.
func (c *Client) httpListLKENodePools(ctx context.Context, clusterID int) ([]LKENodePool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_pool_list", nil, clusterID)
	if err != nil {
		return nil, wrapRequestError("ListLKENodePools", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_lke_pool_get", nil, clusterID, poolID)
	if err != nil {
		return nil, wrapRequestError("GetLKENodePool", err)
	}

	defer drainClose(resp)

	var pool LKENodePool
	if err := c.handleResponse(resp, &pool); err != nil {
		return nil, err
	}

	return &pool, nil
}

// GetLKENode retrieves a single node by its ID within an LKE cluster.
func (c *Client) httpGetLKENode(ctx context.Context, clusterID int, nodeID string) (*LKENode, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_node_get", nil, clusterID, nodeID)
	if err != nil {
		return nil, wrapRequestError("GetLKENode", err)
	}

	defer drainClose(resp)

	var node LKENode
	if err := c.handleResponse(resp, &node); err != nil {
		return nil, err
	}

	return &node, nil
}

// The Linode API wraps the control plane ACL under a top-level "acl" key, so the
// read and update methods below unwrap it and return the bare {enabled,
// addresses} object. The proto twins decode that sub-object with DiscardUnknown.

// GetLKEControlPlaneACL retrieves the control plane ACL for an LKE cluster.
func (c *Client) httpGetLKEControlPlaneACL(ctx context.Context, clusterID int) (*LKEControlPlaneACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_acl_get", nil, clusterID)
	if err != nil {
		return nil, wrapRequestError("GetLKEControlPlaneACL", err)
	}

	defer drainClose(resp)

	var wrapper struct {
		ACL LKEControlPlaneACL `json:"acl"`
	}
	if err := c.handleResponse(resp, &wrapper); err != nil {
		return nil, err
	}

	return &wrapper.ACL, nil
}
