package linode

// LKECluster represents a Linode Kubernetes Engine cluster.
type LKECluster struct {
	Label        string          `json:"label"`
	Region       string          `json:"region"`
	K8sVersion   string          `json:"k8s_version"`
	Status       string          `json:"status"`
	Created      string          `json:"created"`
	Updated      string          `json:"updated"`
	Tags         []string        `json:"tags"`
	ID           int             `json:"id"`
	ControlPlane LKEControlPlane `json:"control_plane"`
}

// LKEControlPlane represents the control plane configuration of an LKE cluster.
type LKEControlPlane struct {
	HighAvailability bool `json:"high_availability"`
}

// LKENodePool represents a node pool within an LKE cluster.
type LKENodePool struct {
	Autoscaler *LKENodePoolAutoscaler `json:"autoscaler"`
	Type       string                 `json:"type"`
	Disks      []LKENodePoolDisk      `json:"disks"`
	Nodes      []LKENode              `json:"nodes"`
	Tags       []string               `json:"tags"`
	ID         int                    `json:"id"`
	ClusterID  int                    `json:"cluster_id"`
	Count      int                    `json:"count"`
}

// LKENodePoolAutoscaler represents autoscaling settings for a node pool.
type LKENodePoolAutoscaler struct {
	Enabled bool `json:"enabled"`
	Min     int  `json:"min"`
	Max     int  `json:"max"`
}

// LKENodePoolDisk represents a disk configuration in a node pool.
type LKENodePoolDisk struct {
	Type string `json:"type"`
	Size int    `json:"size"`
}

// LKENode represents a node within an LKE node pool.
type LKENode struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	InstanceID int    `json:"instance_id"`
}

// LKEKubeconfig holds the base64-encoded kubeconfig for an LKE cluster.
type LKEKubeconfig struct {
	Kubeconfig string `json:"kubeconfig"`
}

// LKEDashboard holds the dashboard URL for an LKE cluster.
type LKEDashboard struct {
	URL string `json:"url"`
}

// LKEAPIEndpoint represents an API endpoint for an LKE cluster.
type LKEAPIEndpoint struct {
	Endpoint string `json:"endpoint"`
}

// LKEVersion represents an available Kubernetes version for LKE.
type LKEVersion struct {
	ID string `json:"id"`
}

// LKEType represents a node type available for LKE clusters.
type LKEType struct {
	ID           string           `json:"id"`
	Label        string           `json:"label"`
	RegionPrices []LKERegionPrice `json:"region_prices"`
	Price        LKETypePrice     `json:"price"`
	Transfer     int              `json:"transfer"`
}

// LKETypePrice represents pricing for an LKE type.
type LKETypePrice struct {
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

// LKERegionPrice represents region-specific pricing for an LKE type.
type LKERegionPrice struct {
	ID      string  `json:"id"`
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

// LKETierVersion represents an LKE tier version.
type LKETierVersion struct {
	ID   string `json:"id"`
	Tier string `json:"tier"`
}

// LKEControlPlaneACL represents the control plane ACL for an LKE cluster.
type LKEControlPlaneACL struct {
	Addresses LKEControlPlaneACLAddresses `json:"addresses"`
	Enabled   bool                        `json:"enabled"`
}

// LKEControlPlaneACLAddresses holds the IP addresses in a control plane ACL.
type LKEControlPlaneACLAddresses struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// CreateLKEClusterRequest represents the request body for creating an LKE cluster.
type CreateLKEClusterRequest struct {
	ControlPlane *LKEControlPlane           `json:"control_plane,omitempty"`
	Label        string                     `json:"label"`
	Region       string                     `json:"region"`
	K8sVersion   string                     `json:"k8s_version"`
	Tags         []string                   `json:"tags,omitempty"`
	NodePools    []CreateLKEClusterNodePool `json:"node_pools"`
}

// CreateLKEClusterNodePool represents a node pool in a create cluster request.
type CreateLKEClusterNodePool struct {
	Autoscaler *LKENodePoolAutoscaler `json:"autoscaler,omitempty"`
	Type       string                 `json:"type"`
	Disks      []LKENodePoolDisk      `json:"disks,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Count      int                    `json:"count"`
}

// UpdateLKEClusterRequest represents the request body for updating an LKE cluster.
type UpdateLKEClusterRequest struct {
	ControlPlane *LKEControlPlane `json:"control_plane,omitempty"`
	Label        string           `json:"label,omitempty"`
	K8sVersion   string           `json:"k8s_version,omitempty"`
	Tags         []string         `json:"tags,omitempty"`
}

// CreateLKENodePoolRequest represents the request body for creating a node pool.
type CreateLKENodePoolRequest struct {
	Autoscaler *LKENodePoolAutoscaler `json:"autoscaler,omitempty"`
	Type       string                 `json:"type"`
	Disks      []LKENodePoolDisk      `json:"disks,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Count      int                    `json:"count"`
}

// UpdateLKENodePoolRequest represents the request body for updating a node pool.
type UpdateLKENodePoolRequest struct {
	Count      *int                   `json:"count,omitempty"`
	Autoscaler *LKENodePoolAutoscaler `json:"autoscaler,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
}

// UpdateLKEControlPlaneACLRequest represents the request body for updating a control plane ACL.
type UpdateLKEControlPlaneACLRequest struct {
	ACL LKEControlPlaneACL `json:"acl"`
}
