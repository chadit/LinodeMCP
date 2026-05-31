package linode

// Firewall represents a Linode Cloud Firewall.
type Firewall struct {
	ID      int           `json:"id"`
	Label   string        `json:"label"`
	Status  string        `json:"status"` // enabled, disabled, deleted
	Rules   FirewallRules `json:"rules"`
	Tags    []string      `json:"tags"`
	Created string        `json:"created"`
	Updated string        `json:"updated"`
}

// FirewallRules represents inbound and outbound firewall rules.
type FirewallRules struct {
	Inbound        []FirewallRule `json:"inbound"`
	InboundPolicy  string         `json:"inbound_policy"`
	Outbound       []FirewallRule `json:"outbound"`
	OutboundPolicy string         `json:"outbound_policy"`
	Fingerprint    string         `json:"fingerprint,omitempty"`
	Version        int            `json:"version,omitempty"`
}

// FirewallRule represents a single firewall rule.
type FirewallRule struct {
	Action      string            `json:"action"`   // ACCEPT, DROP
	Protocol    string            `json:"protocol"` // TCP, UDP, ICMP, IPENCAP
	Ports       string            `json:"ports"`
	Addresses   FirewallAddresses `json:"addresses"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
}

// FirewallAddresses represents IPv4 and IPv6 addresses for a firewall rule.
type FirewallAddresses struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// VLAN represents a Linode VLAN.
type VLAN struct {
	Label   string `json:"label"`
	Region  string `json:"region"`
	Linodes []int  `json:"linodes"`
}

// FirewallDevice represents a device attached to a Cloud Firewall.
type FirewallDevice struct {
	ID      int                  `json:"id"`
	Entity  FirewallDeviceEntity `json:"entity"`
	Created string               `json:"created"`
	Updated string               `json:"updated"`
}

// FirewallDeviceEntity represents the Linode, Linode interface, or NodeBalancer attached to a firewall.
type FirewallDeviceEntity struct {
	ID           int                   `json:"id"`
	Label        string                `json:"label"`
	Type         string                `json:"type"`
	URL          string                `json:"url"`
	ParentEntity *FirewallDeviceEntity `json:"parent_entity"`
}

// FirewallSettings represents the default firewall assignments for resource types.
type FirewallSettings struct {
	DefaultFirewallIDs FirewallDefaultIDs `json:"default_firewall_ids"`
}

// FirewallDefaultIDs contains default firewall IDs by resource type.
type FirewallDefaultIDs struct {
	Linode          int `json:"linode"`
	NodeBalancer    int `json:"nodebalancer"`
	PublicInterface int `json:"public_interface"`
	VPCInterface    int `json:"vpc_interface"`
}

// UpdateFirewallSettingsRequest updates default firewall assignments for resource types.
type UpdateFirewallSettingsRequest struct {
	DefaultFirewallIDs UpdateFirewallDefaultIDs `json:"default_firewall_ids"`
}

// UpdateFirewallDefaultIDs contains optional default firewall IDs by resource type.
type UpdateFirewallDefaultIDs struct {
	Linode          *int `json:"linode,omitempty"`
	NodeBalancer    *int `json:"nodebalancer,omitempty"`
	PublicInterface *int `json:"public_interface,omitempty"`
	VPCInterface    *int `json:"vpc_interface,omitempty"`
}

// FirewallTemplate represents a reusable Cloud Firewall rule template.
type FirewallTemplate struct {
	Slug  string        `json:"slug"`
	Rules FirewallRules `json:"rules"`
}

// NetworkTransferPrice represents a network transfer price entry.
type NetworkTransferPrice struct {
	ID           string                       `json:"id"`
	Label        string                       `json:"label"`
	Price        Price                        `json:"price"`
	RegionPrices []NetworkTransferRegionPrice `json:"region_prices"`
	Transfer     int                          `json:"transfer"`
}

// NetworkTransferRegionPrice represents a region-specific network transfer price.
type NetworkTransferRegionPrice struct {
	ID      string  `json:"id"`
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

// IPv6Pool represents an IPv6 pool on the account.
type IPv6Pool struct {
	Range  string `json:"range"`
	Region string `json:"region"`
	Prefix int    `json:"prefix"`
}

// CreateIPv6RangeRequest represents the request body for creating an IPv6 range.
type CreateIPv6RangeRequest struct {
	LinodeID     *int   `json:"linode_id,omitempty"`
	PrefixLength int    `json:"prefix_length"`
	RouteTarget  string `json:"route_target,omitempty"`
}

// AllocateNetworkingIPRequest represents the request body for allocating an account-level IP address.
type AllocateNetworkingIPRequest struct {
	LinodeID int    `json:"linode_id"`
	Public   bool   `json:"public"`
	Type     string `json:"type"`
}

// UpdateNetworkingIPRequest represents the request body for updating account-level IP reverse DNS.
type UpdateNetworkingIPRequest struct {
	RDNS string `json:"rdns"`
}

// IPAssignment represents one IP-to-Linode assignment.
type IPAssignment struct {
	Address  string `json:"address"`
	LinodeID int    `json:"linode_id"`
}

// AssignNetworkingIPsRequest represents the request body for assigning IP addresses.
type AssignNetworkingIPsRequest struct {
	Region      string         `json:"region"`
	Assignments []IPAssignment `json:"assignments"`
}

// ShareNetworkingIPsRequest represents the request body for sharing IP addresses with a Linode.
type ShareNetworkingIPsRequest struct {
	LinodeID int      `json:"linode_id"`
	IPs      []string `json:"ips"`
}

// NodeBalancerType represents an available NodeBalancer type.
type NodeBalancerType struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Price    Price  `json:"price"`
	Transfer int    `json:"transfer"`
}

// NodeBalancer represents a Linode NodeBalancer (load balancer).
type NodeBalancer struct {
	ID                 int      `json:"id"`
	Label              string   `json:"label"`
	Region             string   `json:"region"`
	Hostname           string   `json:"hostname"`
	IPv4               string   `json:"ipv4"`
	IPv6               string   `json:"ipv6"`
	ClientConnThrottle int      `json:"client_conn_throttle"`
	Transfer           Transfer `json:"transfer"`
	Tags               []string `json:"tags"`
	Created            string   `json:"created"`
	Updated            string   `json:"updated"`
}

// NodeBalancerConfig represents a NodeBalancer frontend configuration.
type NodeBalancerConfig struct {
	ID             int                     `json:"id"`
	Port           int                     `json:"port"`
	Protocol       string                  `json:"protocol"`
	Algorithm      string                  `json:"algorithm"`
	Stickiness     string                  `json:"stickiness"`
	Check          string                  `json:"check"`
	CheckInterval  int                     `json:"check_interval"`
	CheckTimeout   int                     `json:"check_timeout"`
	CheckAttempts  int                     `json:"check_attempts"`
	CheckPath      string                  `json:"check_path"`
	CheckBody      string                  `json:"check_body"`
	CheckPassive   bool                    `json:"check_passive"`
	CipherSuite    string                  `json:"cipher_suite"`
	SSLCommonName  string                  `json:"ssl_commonname"`
	SSLFingerprint string                  `json:"ssl_fingerprint"`
	NodeBalancerID int                     `json:"nodebalancer_id"`
	NodesStatus    NodeBalancerNodesStatus `json:"nodes_status"`
}

// CreateNodeBalancerConfigRequest represents the request body for creating a NodeBalancer config.
type CreateNodeBalancerConfigRequest struct {
	Port          int    `json:"port"`
	Protocol      string `json:"protocol,omitempty"`
	Algorithm     string `json:"algorithm,omitempty"`
	Stickiness    string `json:"stickiness,omitempty"`
	Check         string `json:"check,omitempty"`
	CheckInterval int    `json:"check_interval,omitempty"`
	CheckTimeout  int    `json:"check_timeout,omitempty"`
	CheckAttempts int    `json:"check_attempts,omitempty"`
	CheckPath     string `json:"check_path,omitempty"`
	CheckBody     string `json:"check_body,omitempty"`
	CheckPassive  *bool  `json:"check_passive,omitempty"`
	CipherSuite   string `json:"cipher_suite,omitempty"`
	SSLCert       string `json:"ssl_cert,omitempty"`
	SSLKey        string `json:"ssl_key,omitempty"`
}

// NodeBalancerNode represents a backend node on a NodeBalancer config.
type NodeBalancerNode struct {
	ID             int    `json:"id"`
	Label          string `json:"label"`
	Address        string `json:"address"`
	Status         string `json:"status"`
	Weight         int    `json:"weight"`
	Mode           string `json:"mode"`
	NodeBalancerID int    `json:"nodebalancer_id"`
	ConfigID       int    `json:"config_id"`
}

// CreateNodeBalancerNodeRequest represents the request body for creating a NodeBalancer config node.
type CreateNodeBalancerNodeRequest struct {
	Label   string `json:"label"`
	Address string `json:"address"`
	Weight  int    `json:"weight,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

// UpdateNodeBalancerNodeRequest represents the request body for updating a NodeBalancer config node.
type UpdateNodeBalancerNodeRequest struct {
	Label   string `json:"label,omitempty"`
	Address string `json:"address,omitempty"`
	Weight  int    `json:"weight,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

// UpdateNodeBalancerConfigRequest represents the request body for updating a NodeBalancer config.
type UpdateNodeBalancerConfigRequest struct {
	Port          int    `json:"port,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	Algorithm     string `json:"algorithm,omitempty"`
	Stickiness    string `json:"stickiness,omitempty"`
	Check         string `json:"check,omitempty"`
	CheckInterval int    `json:"check_interval,omitempty"`
	CheckTimeout  int    `json:"check_timeout,omitempty"`
	CheckAttempts int    `json:"check_attempts,omitempty"`
	CheckPath     string `json:"check_path,omitempty"`
	CheckBody     string `json:"check_body,omitempty"`
	CheckPassive  *bool  `json:"check_passive,omitempty"`
	CipherSuite   string `json:"cipher_suite,omitempty"`
	SSLCert       string `json:"ssl_cert,omitempty"`
	SSLKey        string `json:"ssl_key,omitempty"`
}

// NodeBalancerNodesStatus represents the health summary for nodes on a NodeBalancer config.
type NodeBalancerNodesStatus struct {
	Up   int `json:"up"`
	Down int `json:"down"`
}

// NodeBalancerConfigNode represents a backend node attached to a NodeBalancer config.
type NodeBalancerConfigNode struct {
	ID             int    `json:"id"`
	Address        string `json:"address"`
	Label          string `json:"label"`
	Status         string `json:"status"`
	Weight         int    `json:"weight"`
	Mode           string `json:"mode"`
	NodeBalancerID int    `json:"nodebalancer_id"`
	ConfigID       int    `json:"config_id"`
}

// Transfer represents data transfer statistics.
type Transfer struct {
	In    float64 `json:"in"`
	Out   float64 `json:"out"`
	Total float64 `json:"total"`
}

// NodeBalancerStats represents traffic and connection statistics for a NodeBalancer.
type NodeBalancerStats struct {
	Title       string                   `json:"title"`
	Connections [][]float64              `json:"connections"`
	Traffic     NodeBalancerTrafficStats `json:"traffic"`
}

// NodeBalancerTrafficStats contains inbound and outbound traffic graphs for a NodeBalancer.
type NodeBalancerTrafficStats struct {
	In  [][]float64 `json:"in"`
	Out [][]float64 `json:"out"`
}

// CreateFirewallRequest represents the request body for creating a firewall.
type CreateFirewallRequest struct {
	Label   string         `json:"label"`
	Rules   *FirewallRules `json:"rules,omitempty"`
	Tags    []string       `json:"tags,omitempty"`
	Devices []Device       `json:"devices,omitempty"`
}

// Device represents a device attached to a firewall.
type Device struct {
	ID   int    `json:"id"`
	Type string `json:"type"` // linode, nodebalancer
}

// CreateFirewallDeviceRequest represents the request body for assigning a device to a firewall.
type CreateFirewallDeviceRequest struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
}

// UpdateFirewallRequest represents the request body for updating a firewall.
type UpdateFirewallRequest struct {
	Label  string         `json:"label,omitempty"`
	Status string         `json:"status,omitempty"` // enabled, disabled
	Rules  *FirewallRules `json:"rules,omitempty"`
	Tags   []string       `json:"tags,omitempty"`
}

// UpdateInstanceFirewallsRequest represents the request body for replacing
// firewall assignments on a Linode instance.
type UpdateInstanceFirewallsRequest struct {
	FirewallIDs []int `json:"firewall_ids"`
}

// UpdateNodeBalancerFirewallsRequest represents the request body for replacing
// firewall assignments on a NodeBalancer.
type UpdateNodeBalancerFirewallsRequest struct {
	FirewallIDs []int `json:"firewall_ids"`
}

// CreateNodeBalancerRequest represents the request body for creating a NodeBalancer.
type CreateNodeBalancerRequest struct {
	Region             string   `json:"region"`
	Label              string   `json:"label,omitempty"`
	ClientConnThrottle int      `json:"client_conn_throttle,omitempty"`
	Tags               []string `json:"tags,omitempty"`
}

// UpdateNodeBalancerRequest represents the request body for updating a NodeBalancer.
type UpdateNodeBalancerRequest struct {
	Label              string   `json:"label,omitempty"`
	ClientConnThrottle *int     `json:"client_conn_throttle,omitempty"`
	Tags               []string `json:"tags,omitempty"`
}
