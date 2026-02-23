package linode

// LKECluster represents a Linode Kubernetes Engine cluster.
type LKECluster struct {
	ID           int             `json:"id"`
	Label        string          `json:"label"`
	Region       string          `json:"region"`
	K8sVersion   string          `json:"k8s_version"`
	Status       string          `json:"status"`
	Tags         []string        `json:"tags"`
	Created      string          `json:"created"`
	Updated      string          `json:"updated"`
	ControlPlane LKEControlPlane `json:"control_plane"`
}

// LKEControlPlane represents the control plane configuration of an LKE cluster.
type LKEControlPlane struct {
	HighAvailability bool `json:"high_availability"`
}

// LKENodePool represents a node pool within an LKE cluster.
type LKENodePool struct {
	ID         int                    `json:"id"`
	ClusterID  int                    `json:"cluster_id"`
	Type       string                 `json:"type"`
	Count      int                    `json:"count"`
	Disks      []LKENodePoolDisk      `json:"disks"`
	Autoscaler *LKENodePoolAutoscaler `json:"autoscaler"`
	Nodes      []LKENode              `json:"nodes"`
	Tags       []string               `json:"tags"`
}

// LKENodePoolAutoscaler represents autoscaling settings for a node pool.
type LKENodePoolAutoscaler struct {
	Enabled bool `json:"enabled"`
	Min     int  `json:"min"`
	Max     int  `json:"max"`
}

// LKENodePoolDisk represents a disk configuration in a node pool.
type LKENodePoolDisk struct {
	Size int    `json:"size"`
	Type string `json:"type"`
}

// LKENode represents a node within an LKE node pool.
type LKENode struct {
	ID         string `json:"id"`
	InstanceID int    `json:"instance_id"`
	Status     string `json:"status"`
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
	Price        LKETypePrice     `json:"price"`
	RegionPrices []LKERegionPrice `json:"region_prices"`
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
	Enabled   bool                        `json:"enabled"`
	Addresses LKEControlPlaneACLAddresses `json:"addresses"`
}

// LKEControlPlaneACLAddresses holds the IP addresses in a control plane ACL.
type LKEControlPlaneACLAddresses struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// CreateLKEClusterRequest represents the request body for creating an LKE cluster.
type CreateLKEClusterRequest struct {
	Label        string                     `json:"label"`
	Region       string                     `json:"region"`
	K8sVersion   string                     `json:"k8s_version"`
	Tags         []string                   `json:"tags,omitempty"`
	NodePools    []CreateLKEClusterNodePool `json:"node_pools"`
	ControlPlane *LKEControlPlane           `json:"control_plane,omitempty"`
}

// CreateLKEClusterNodePool represents a node pool in a create cluster request.
type CreateLKEClusterNodePool struct {
	Type       string                 `json:"type"`
	Count      int                    `json:"count"`
	Autoscaler *LKENodePoolAutoscaler `json:"autoscaler,omitempty"`
	Disks      []LKENodePoolDisk      `json:"disks,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
}

// UpdateLKEClusterRequest represents the request body for updating an LKE cluster.
type UpdateLKEClusterRequest struct {
	Label        string           `json:"label,omitempty"`
	K8sVersion   string           `json:"k8s_version,omitempty"`
	Tags         []string         `json:"tags,omitempty"`
	ControlPlane *LKEControlPlane `json:"control_plane,omitempty"`
}

// CreateLKENodePoolRequest represents the request body for creating a node pool.
type CreateLKENodePoolRequest struct {
	Type       string                 `json:"type"`
	Count      int                    `json:"count"`
	Autoscaler *LKENodePoolAutoscaler `json:"autoscaler,omitempty"`
	Disks      []LKENodePoolDisk      `json:"disks,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
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
