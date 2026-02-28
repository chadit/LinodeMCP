package linode

// VPC represents a Linode Virtual Private Cloud.
type VPC struct {
	ID          int         `json:"id"`
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Region      string      `json:"region"`
	Subnets     []VPCSubnet `json:"subnets"`
	Created     string      `json:"created"`
	Updated     string      `json:"updated"`
}

// VPCSubnet represents a subnet within a VPC.
type VPCSubnet struct {
	ID      int               `json:"id"`
	Label   string            `json:"label"`
	IPv4    string            `json:"ipv4"`
	Linodes []VPCSubnetLinode `json:"linodes"`
	Created string            `json:"created"`
	Updated string            `json:"updated"`
}

// VPCSubnetLinode represents a Linode assigned to a VPC subnet.
type VPCSubnetLinode struct {
	ID         int                        `json:"id"`
	Interfaces []VPCSubnetLinodeInterface `json:"interfaces"`
}

// VPCSubnetLinodeInterface represents a network interface on a Linode within a VPC subnet.
type VPCSubnetLinodeInterface struct {
	ID       int  `json:"id"`
	Active   bool `json:"active"`
	ConfigID int  `json:"config_id"`
}

// VPCIP represents an IP address associated with a VPC.
type VPCIP struct {
	Address      *string `json:"address"`
	AddressRange *string `json:"address_range"`
	VPCID        int     `json:"vpc_id"`
	SubnetID     int     `json:"subnet_id"`
	Region       string  `json:"region"`
	LinodeID     *int    `json:"linode_id"`
	ConfigID     *int    `json:"config_id"`
	InterfaceID  int     `json:"interface_id"`
	Active       bool    `json:"active"`
	NAT1To1      string  `json:"nat_1_1"`
	Gateway      *string `json:"gateway"`
	Prefix       *int    `json:"prefix"`
	SubnetMask   string  `json:"subnet_mask"`
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
