package linode

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// Every method here resolves its path from the tool's declared route, so none of
// them spells a URL or escapes a path segment. The *Proto twins hit the same
// route as their struct-typed counterpart and decode the response into a proto
// message for the proto-backed tool path.

// httpListLKEClustersProto retrieves all LKE clusters as proto messages.
func (c *Client) httpListLKEClustersProto(ctx context.Context) ([]*linodev1.LKECluster, error) {
	return listProtoElementsRouted(ctx, c, "ListLKEClusters",
		"linode_lke_cluster_list", "", nil,
		func() *linodev1.LKECluster { return &linodev1.LKECluster{} })
}

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

// httpGetLKEClusterProto retrieves an LKE cluster as a proto message.
func (c *Client) httpGetLKEClusterProto(ctx context.Context, clusterID int) (*linodev1.LKECluster, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_cluster_get", nil, clusterID)
	if err != nil {
		return nil, wrapRequestError("GetLKECluster", err)
	}

	defer drainClose(resp)

	cluster := &linodev1.LKECluster{}
	if err := c.handleProtoResponse(resp, cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

// httpCreateLKEClusterProto creates an LKE cluster as a proto message.
func (c *Client) httpCreateLKEClusterProto(ctx context.Context, req *CreateLKEClusterRequest) (*linodev1.LKECluster, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_cluster_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateLKECluster", err)
	}

	defer drainClose(resp)

	cluster := &linodev1.LKECluster{}
	if err := c.handleProtoResponse(resp, cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

// httpUpdateLKEClusterProto updates an LKE cluster as a proto message.
func (c *Client) httpUpdateLKEClusterProto(ctx context.Context, clusterID int, req UpdateLKEClusterRequest) (*linodev1.LKECluster, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_cluster_update", req, clusterID)
	if err != nil {
		return nil, wrapRequestError("UpdateLKECluster", err)
	}

	defer drainClose(resp)

	cluster := &linodev1.LKECluster{}
	if err := c.handleProtoResponse(resp, cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

// DeleteLKECluster deletes an LKE cluster.
func (c *Client) httpDeleteLKECluster(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_cluster_delete", nil, clusterID)
	if err != nil {
		return wrapRequestError("DeleteLKECluster", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// RecycleLKECluster recycles all nodes in an LKE cluster.
func (c *Client) httpRecycleLKECluster(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_cluster_recycle", nil, clusterID)
	if err != nil {
		return wrapRequestError("RecycleLKECluster", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// RegenerateLKECluster regenerates the service token for an LKE cluster.
func (c *Client) httpRegenerateLKECluster(ctx context.Context, clusterID int, req RegenerateLKEClusterRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_cluster_regenerate", req.Payload(), clusterID)
	if err != nil {
		return wrapRequestError("RegenerateLKECluster", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
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

// httpListLKENodePoolsProto retrieves an LKE cluster's node pools as proto messages.
func (c *Client) httpListLKENodePoolsProto(ctx context.Context, clusterID int) ([]*linodev1.LKENodePool, error) {
	return listProtoElementsRouted(ctx, c, "ListLKENodePools",
		"linode_lke_pool_list", "", []any{clusterID},
		func() *linodev1.LKENodePool { return &linodev1.LKENodePool{} })
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

// httpGetLKENodePoolProto retrieves one LKE node pool as a proto message.
func (c *Client) httpGetLKENodePoolProto(ctx context.Context, clusterID, poolID int) (*linodev1.LKENodePool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_pool_get", nil, clusterID, poolID)
	if err != nil {
		return nil, wrapRequestError("GetLKENodePool", err)
	}

	defer drainClose(resp)

	pool := &linodev1.LKENodePool{}
	if err := c.handleProtoResponse(resp, pool); err != nil {
		return nil, err
	}

	return pool, nil
}

// httpCreateLKENodePoolProto creates a node pool, emitting the same field set as
// the pool GET and LIST paths.
func (c *Client) httpCreateLKENodePoolProto(ctx context.Context, clusterID int, req *CreateLKENodePoolRequest) (*linodev1.LKENodePool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_pool_create", req, clusterID)
	if err != nil {
		return nil, wrapRequestError("CreateLKENodePool", err)
	}

	defer drainClose(resp)

	pool := &linodev1.LKENodePool{}
	if err := c.handleProtoResponse(resp, pool); err != nil {
		return nil, err
	}

	return pool, nil
}

// httpUpdateLKENodePoolProto updates a node pool as a proto message.
func (c *Client) httpUpdateLKENodePoolProto(ctx context.Context, clusterID, poolID int, req *UpdateLKENodePoolRequest) (*linodev1.LKENodePool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_pool_update", req, clusterID, poolID)
	if err != nil {
		return nil, wrapRequestError("UpdateLKENodePool", err)
	}

	defer drainClose(resp)

	pool := &linodev1.LKENodePool{}
	if err := c.handleProtoResponse(resp, pool); err != nil {
		return nil, err
	}

	return pool, nil
}

// DeleteLKENodePool deletes a node pool from an LKE cluster.
func (c *Client) httpDeleteLKENodePool(ctx context.Context, clusterID, poolID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_pool_delete", nil, clusterID, poolID)
	if err != nil {
		return wrapRequestError("DeleteLKENodePool", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// RecycleLKENodePool recycles all nodes in a specific node pool.
func (c *Client) httpRecycleLKENodePool(ctx context.Context, clusterID, poolID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_pool_recycle", nil, clusterID, poolID)
	if err != nil {
		return wrapRequestError("RecycleLKENodePool", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
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

// httpGetLKENodeProto retrieves one LKE cluster node as a proto message.
func (c *Client) httpGetLKENodeProto(ctx context.Context, clusterID int, nodeID string) (*linodev1.LKENode, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_node_get", nil, clusterID, nodeID)
	if err != nil {
		return nil, wrapRequestError("GetLKENode", err)
	}

	defer drainClose(resp)

	node := &linodev1.LKENode{}
	if err := c.handleProtoResponse(resp, node); err != nil {
		return nil, err
	}

	return node, nil
}

// DeleteLKENode deletes a specific node from an LKE cluster.
func (c *Client) httpDeleteLKENode(ctx context.Context, clusterID int, nodeID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_node_delete", nil, clusterID, nodeID)
	if err != nil {
		return wrapRequestError("DeleteLKENode", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// RecycleLKENode recycles a specific node in an LKE cluster.
func (c *Client) httpRecycleLKENode(ctx context.Context, clusterID int, nodeID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_node_recycle", nil, clusterID, nodeID)
	if err != nil {
		return wrapRequestError("RecycleLKENode", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpGetLKEKubeconfigProto retrieves an LKE cluster kubeconfig as a proto message.
func (c *Client) httpGetLKEKubeconfigProto(ctx context.Context, clusterID int) (*linodev1.LKEKubeconfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_kubeconfig_get", nil, clusterID)
	if err != nil {
		return nil, wrapRequestError("GetLKEKubeconfig", err)
	}

	defer drainClose(resp)

	kubeconfig := &linodev1.LKEKubeconfig{}
	if err := c.handleProtoResponse(resp, kubeconfig); err != nil {
		return nil, err
	}

	return kubeconfig, nil
}

// DeleteLKEKubeconfig deletes and regenerates the kubeconfig for an LKE cluster.
func (c *Client) httpDeleteLKEKubeconfig(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_kubeconfig_delete", nil, clusterID)
	if err != nil {
		return wrapRequestError("DeleteLKEKubeconfig", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpGetLKEDashboardProto retrieves the LKE dashboard URL as a proto message.
func (c *Client) httpGetLKEDashboardProto(ctx context.Context, clusterID int) (*linodev1.LKEDashboard, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_dashboard_get", nil, clusterID)
	if err != nil {
		return nil, wrapRequestError("GetLKEDashboard", err)
	}

	defer drainClose(resp)

	dashboard := &linodev1.LKEDashboard{}
	if err := c.handleProtoResponse(resp, dashboard); err != nil {
		return nil, err
	}

	return dashboard, nil
}

// httpListLKEAPIEndpointsProto retrieves an LKE cluster's API endpoints as proto
// messages.
func (c *Client) httpListLKEAPIEndpointsProto(ctx context.Context, clusterID int) ([]*linodev1.LKEAPIEndpoint, error) {
	return listProtoElementsRouted(ctx, c, "ListLKEAPIEndpoints",
		"linode_lke_api_endpoint_list", "", []any{clusterID},
		func() *linodev1.LKEAPIEndpoint { return &linodev1.LKEAPIEndpoint{} })
}

// DeleteLKEServiceToken deletes and regenerates the service token for an LKE cluster.
func (c *Client) httpDeleteLKEServiceToken(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_service_token_delete", nil, clusterID)
	if err != nil {
		return wrapRequestError("DeleteLKEServiceToken", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
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

// httpUpdateLKEControlPlaneACLProto updates the control plane ACL as a proto message.
func (c *Client) httpUpdateLKEControlPlaneACLProto(ctx context.Context, clusterID int, req UpdateLKEControlPlaneACLRequest) (*linodev1.LKEControlPlaneACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_acl_update", req, clusterID)
	if err != nil {
		return nil, wrapRequestError("UpdateLKEControlPlaneACL", err)
	}

	defer drainClose(resp)

	var envelope struct {
		ACL json.RawMessage `json:"acl"`
	}
	if err := c.handleResponse(resp, &envelope); err != nil {
		return nil, err
	}

	acl := &linodev1.LKEControlPlaneACL{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(envelope.ACL, acl); err != nil {
		return nil, fmt.Errorf("failed to unmarshal control plane ACL element: %w", err)
	}

	return acl, nil
}

// httpGetLKEControlPlaneACLProto retrieves the control plane ACL as a proto message.
func (c *Client) httpGetLKEControlPlaneACLProto(ctx context.Context, clusterID int) (*linodev1.LKEControlPlaneACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_acl_get", nil, clusterID)
	if err != nil {
		return nil, wrapRequestError("GetLKEControlPlaneACL", err)
	}

	defer drainClose(resp)

	var envelope struct {
		ACL json.RawMessage `json:"acl"`
	}
	if err := c.handleResponse(resp, &envelope); err != nil {
		return nil, err
	}

	acl := &linodev1.LKEControlPlaneACL{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(envelope.ACL, acl); err != nil {
		return nil, fmt.Errorf("failed to unmarshal control plane ACL element: %w", err)
	}

	return acl, nil
}

// DeleteLKEControlPlaneACL deletes the control plane ACL for an LKE cluster.
func (c *Client) httpDeleteLKEControlPlaneACL(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_acl_delete", nil, clusterID)
	if err != nil {
		return wrapRequestError("DeleteLKEControlPlaneACL", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpListLKEVersionsProto retrieves all available Kubernetes versions as proto
// messages.
func (c *Client) httpListLKEVersionsProto(ctx context.Context) ([]*linodev1.LKEVersion, error) {
	return listProtoElementsRouted(ctx, c, "ListLKEVersions",
		"linode_lke_version_list", "", nil,
		func() *linodev1.LKEVersion { return &linodev1.LKEVersion{} })
}

// httpGetLKEVersionProto retrieves one LKE Kubernetes version as a proto message.
func (c *Client) httpGetLKEVersionProto(ctx context.Context, versionID string) (*linodev1.LKEVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_version_get", nil, versionID)
	if err != nil {
		return nil, wrapRequestError("GetLKEVersion", err)
	}

	defer drainClose(resp)

	version := &linodev1.LKEVersion{}
	if err := c.handleProtoResponse(resp, version); err != nil {
		return nil, err
	}

	return version, nil
}

// httpListLKETypesProto retrieves all available LKE node types as proto messages.
func (c *Client) httpListLKETypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	return listProtoElementsRouted(ctx, c, "ListLKETypes",
		"linode_lke_type_list", "", nil,
		func() *linodev1.LinodeType { return &linodev1.LinodeType{} })
}

// httpListLKETierVersionsProto retrieves a tier's available LKE versions as proto
// messages. This endpoint returns a {data,page,...} page envelope, so the list
// reads data.
func (c *Client) httpListLKETierVersionsProto(ctx context.Context, tier string) ([]*linodev1.LKETierVersion, error) {
	return listProtoElementsRouted(ctx, c, "ListLKETierVersions",
		"linode_lke_tier_version_list", "", []any{tier},
		func() *linodev1.LKETierVersion { return &linodev1.LKETierVersion{} })
}

// httpGetLKETierVersionProto retrieves one LKE tier Kubernetes version as a proto message.
func (c *Client) httpGetLKETierVersionProto(ctx context.Context, tierID, versionID string) (*linodev1.LKETierVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_lke_tier_version_get", nil, tierID, versionID)
	if err != nil {
		return nil, wrapRequestError("GetLKETierVersion", err)
	}

	defer drainClose(resp)

	version := &linodev1.LKETierVersion{}
	if err := c.handleProtoResponse(resp, version); err != nil {
		return nil, err
	}

	return version, nil
}
