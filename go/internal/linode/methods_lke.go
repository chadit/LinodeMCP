package linode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"google.golang.org/protobuf/encoding/protojson"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const (
	endpointLKEClusters     = "/lke/clusters"
	endpointLKEVersions     = "/lke/versions"
	endpointLKETypes        = "/lke/types"
	endpointLKETierVersions = "/lke/tiers"
)

// httpListLKEClustersProto retrieves all LKE clusters as proto messages, decoded
// directly from the API JSON for the proto-backed list path.
func (c *Client) httpListLKEClustersProto(ctx context.Context) ([]*linodev1.LKECluster, error) {
	return listProtoElements(ctx, c, "ListLKEClusters", endpointLKEClusters,
		func() *linodev1.LKECluster { return &linodev1.LKECluster{} })
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

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKECluster", Err: err}
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

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointLKEClusters, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateLKECluster", Err: err}
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

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateLKECluster", Err: err}
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

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKECluster", Err: err}
	}

	defer drainClose(resp)

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

	defer drainClose(resp)

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

	defer drainClose(resp)

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

	defer drainClose(resp)

	var response PaginatedResponse[LKENodePool]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListLKENodePoolsProto retrieves an LKE cluster's node pools as proto
// messages for the proto-backed list path. The endpoint is formatted with the
// same fmt.Sprintf(endpointLKEClusters+"/%d/pools", clusterID) pattern
// httpListLKENodePools uses, so the runtime path matches exactly.
func (c *Client) httpListLKENodePoolsProto(ctx context.Context, clusterID int) ([]*linodev1.LKENodePool, error) {
	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools", clusterID)

	return listProtoElements(ctx, c, "ListLKENodePools", endpoint,
		func() *linodev1.LKENodePool { return &linodev1.LKENodePool{} })
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

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools/%d", clusterID, poolID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKENodePool", Err: err}
	}

	defer drainClose(resp)

	pool := &linodev1.LKENodePool{}
	if err := c.handleProtoResponse(resp, pool); err != nil {
		return nil, err
	}

	return pool, nil
}

// httpCreateLKENodePoolProto creates a node pool and decodes the response into the
// proto element so the write tool emits the same field set as the pool GET/LIST
// path.
func (c *Client) httpCreateLKENodePoolProto(ctx context.Context, clusterID int, req *CreateLKENodePoolRequest) (*linodev1.LKENodePool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateLKENodePool", Err: err}
	}

	defer drainClose(resp)

	pool := &linodev1.LKENodePool{}
	if err := c.handleProtoResponse(resp, pool); err != nil {
		return nil, err
	}

	return pool, nil
}

// httpUpdateLKENodePoolProto updates a node pool and decodes the response into the
// proto element.
func (c *Client) httpUpdateLKENodePoolProto(ctx context.Context, clusterID, poolID int, req UpdateLKENodePoolRequest) (*linodev1.LKENodePool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools/%d", clusterID, poolID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateLKENodePool", Err: err}
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

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/pools/%d", clusterID, poolID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKENodePool", Err: err}
	}

	defer drainClose(resp)

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

	defer drainClose(resp)

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

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/nodes/%s", clusterID, url.PathEscape(nodeID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKENode", Err: err}
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

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/nodes/%s", clusterID, url.PathEscape(nodeID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKENode", Err: err}
	}

	defer drainClose(resp)

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

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpGetLKEKubeconfigProto retrieves an LKE cluster kubeconfig as a proto
// message.
func (c *Client) httpGetLKEKubeconfigProto(ctx context.Context, clusterID int) (*linodev1.LKEKubeconfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/kubeconfig", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKEKubeconfig", Err: err}
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

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/kubeconfig", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKEKubeconfig", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpGetLKEDashboardProto retrieves the LKE dashboard URL as a proto message.
func (c *Client) httpGetLKEDashboardProto(ctx context.Context, clusterID int) (*linodev1.LKEDashboard, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/dashboard", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKEDashboard", Err: err}
	}

	defer drainClose(resp)

	dashboard := &linodev1.LKEDashboard{}
	if err := c.handleProtoResponse(resp, dashboard); err != nil {
		return nil, err
	}

	return dashboard, nil
}

// httpListLKEAPIEndpointsProto retrieves an LKE cluster's API endpoints as proto
// messages for the proto-backed list path. The endpoint is formatted with the
// same fmt.Sprintf(endpointLKEClusters+"/%d/api-endpoints", clusterID) pattern
// httpListLKEAPIEndpoints uses, so the runtime path matches exactly.
func (c *Client) httpListLKEAPIEndpointsProto(ctx context.Context, clusterID int) ([]*linodev1.LKEAPIEndpoint, error) {
	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/api-endpoints", clusterID)

	return listProtoElements(ctx, c, "ListLKEAPIEndpoints", endpoint,
		func() *linodev1.LKEAPIEndpoint { return &linodev1.LKEAPIEndpoint{} })
}

// DeleteLKEServiceToken deletes and regenerates the service token for an LKE cluster.
func (c *Client) httpDeleteLKEServiceToken(ctx context.Context, clusterID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/servicetoken", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKEServiceToken", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// GetLKEControlPlaneACL retrieves the control plane ACL for an LKE cluster.
func (c *Client) httpGetLKEControlPlaneACL(ctx context.Context, clusterID int) (*LKEControlPlaneACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/control_plane_acl", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKEControlPlaneACL", Err: err}
	}

	defer drainClose(resp)

	// The Linode API wraps the ACL under a top-level "acl" key. Decode the
	// wrapper and return the bare ACL so the handler emits the unwrapped object.
	var wrapper struct {
		ACL LKEControlPlaneACL `json:"acl"`
	}
	if err := c.handleResponse(resp, &wrapper); err != nil {
		return nil, err
	}

	return &wrapper.ACL, nil
}

// httpUpdateLKEControlPlaneACLProto updates the control plane ACL and decodes the
// response into the LKEControlPlaneACL proto element. The Linode API wraps the
// ACL under a top-level "acl" key, so the acl sub-object is protojson-decoded
// (DiscardUnknown) into the proto element to keep the full {enabled, addresses}
// shape the API returns.
func (c *Client) httpUpdateLKEControlPlaneACLProto(ctx context.Context, clusterID int, req UpdateLKEControlPlaneACLRequest) (*linodev1.LKEControlPlaneACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/control_plane_acl", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateLKEControlPlaneACL", Err: err}
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

// httpGetLKEControlPlaneACLProto retrieves the control plane ACL and decodes it
// into the LKEControlPlaneACL proto element. The Linode API wraps the ACL under
// a top-level "acl" key, so the acl sub-object is protojson-decoded
// (DiscardUnknown) into the proto element to keep the full {enabled, addresses}
// shape the API returns.
func (c *Client) httpGetLKEControlPlaneACLProto(ctx context.Context, clusterID int) (*linodev1.LKEControlPlaneACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/control_plane_acl", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKEControlPlaneACL", Err: err}
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

	endpoint := fmt.Sprintf(endpointLKEClusters+"/%d/control_plane_acl", clusterID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLKEControlPlaneACL", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpListLKEVersionsProto retrieves all available Kubernetes versions as proto
// messages, decoded directly from the API JSON for the proto-backed list path.
func (c *Client) httpListLKEVersionsProto(ctx context.Context) ([]*linodev1.LKEVersion, error) {
	return listProtoElements(ctx, c, "ListLKEVersions", endpointLKEVersions,
		func() *linodev1.LKEVersion { return &linodev1.LKEVersion{} })
}

// httpGetLKEVersionProto retrieves one LKE Kubernetes version as a proto message.
func (c *Client) httpGetLKEVersionProto(ctx context.Context, versionID string) (*linodev1.LKEVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("%s/%s", endpointLKEVersions, url.PathEscape(versionID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKEVersion", Err: err}
	}

	defer drainClose(resp)

	version := &linodev1.LKEVersion{}
	if err := c.handleProtoResponse(resp, version); err != nil {
		return nil, err
	}

	return version, nil
}

// httpListLKETypesProto retrieves all available LKE node types as proto
// messages, decoded directly from the API JSON for the proto-backed list path.
func (c *Client) httpListLKETypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	return listProtoElements(ctx, c, "ListLKETypes", endpointLKETypes,
		func() *linodev1.LinodeType { return &linodev1.LinodeType{} })
}

// httpListLKETierVersionsProto retrieves available LKE tier versions for a tier
// as proto messages for the proto-backed list path. The endpoint returns a
// {data,page,...} page envelope, so listProtoElements reads data. The tier string
// is path-escaped into the endpoint exactly like httpListLKETierVersions.
func (c *Client) httpListLKETierVersionsProto(ctx context.Context, tier string) ([]*linodev1.LKETierVersion, error) {
	endpoint := endpointLKETierVersions + "/" + url.PathEscape(tier) + "/versions"

	return listProtoElements(ctx, c, "ListLKETierVersions", endpoint,
		func() *linodev1.LKETierVersion { return &linodev1.LKETierVersion{} })
}

// httpGetLKETierVersionProto retrieves one LKE tier Kubernetes version as a proto
// message.
func (c *Client) httpGetLKETierVersionProto(ctx context.Context, tierID, versionID string) (*linodev1.LKETierVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("%s/%s/versions/%s", endpointLKETierVersions, url.PathEscape(tierID), url.PathEscape(versionID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLKETierVersion", Err: err}
	}

	defer drainClose(resp)

	version := &linodev1.LKETierVersion{}
	if err := c.handleProtoResponse(resp, version); err != nil {
		return nil, err
	}

	return version, nil
}
