package linode

// VPC represents a Linode Virtual Private Cloud.
type VPC struct {
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Region      string      `json:"region"`
	Created     string      `json:"created"`
	Updated     string      `json:"updated"`
	Subnets     []VPCSubnet `json:"subnets"`
	ID          int         `json:"id"`
}

// VPCSubnet represents a subnet within a VPC.
type VPCSubnet struct {
	Label   string            `json:"label"`
	IPv4    string            `json:"ipv4"`
	Created string            `json:"created"`
	Updated string            `json:"updated"`
	Linodes []VPCSubnetLinode `json:"linodes"`
	ID      int               `json:"id"`
}

// VPCSubnetLinode represents a Linode assigned to a VPC subnet.
type VPCSubnetLinode struct {
	Interfaces []VPCSubnetLinodeInterface `json:"interfaces"`
	ID         int                        `json:"id"`
}

// VPCSubnetLinodeInterface represents a network interface on a Linode within a VPC subnet.
type VPCSubnetLinodeInterface struct {
	ID       int  `json:"id"`
	Active   bool `json:"active"`
	ConfigID int  `json:"config_id"`
}

// VPCIP represents an IP address associated with a VPC.
type VPCIP struct {
	ConfigID     *int    `json:"config_id"`
	AddressRange *string `json:"address_range"`
	Prefix       *int    `json:"prefix"`
	Gateway      *string `json:"gateway"`
	Address      *string `json:"address"`
	LinodeID     *int    `json:"linode_id"`
	NAT1To1      string  `json:"nat_1_1"`
	Region       string  `json:"region"`
	SubnetMask   string  `json:"subnet_mask"`
	InterfaceID  int     `json:"interface_id"`
	SubnetID     int     `json:"subnet_id"`
	VPCID        int     `json:"vpc_id"`
	Active       bool    `json:"active"`
}

// NodeBalancerVPCConfig represents a VPC configuration for a NodeBalancer.
type NodeBalancerVPCConfig struct {
	VPCID               *int   `json:"vpc_id"`
	IPv4RangeID         *int   `json:"ipv4_range_id,omitempty"`
	IPv6RangeID         *int   `json:"ipv6_range_id,omitempty"`
	IPv4RangeAutoAssign *bool  `json:"ipv4_range_auto_assign,omitempty"`
	IPv4Range           string `json:"ipv4_range,omitempty"`
	IPv6Range           string `json:"ipv6_range,omitempty"`
	ID                  int    `json:"id"`
	SubnetID            int    `json:"subnet_id"`
	NodeBalancerID      int    `json:"nodebalancer_id,omitempty"`
}

// CreateVPCRequest represents the request body for creating a VPC.
type CreateVPCRequest struct {
	Label       string                `json:"label"`
	Description string                `json:"description,omitempty"`
	Region      string                `json:"region"`
	Subnets     []CreateSubnetRequest `json:"subnets,omitempty"`
}

// UpdateVPCRequest represents the request body for updating a VPC.
type UpdateVPCRequest struct {
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// CreateSubnetRequest represents the request body for creating a VPC subnet.
type CreateSubnetRequest struct {
	Label string `json:"label"`
	IPv4  string `json:"ipv4,omitempty"`
}

// UpdateSubnetRequest represents the request body for updating a VPC subnet.
type UpdateSubnetRequest struct {
	Label string `json:"label"`
}
