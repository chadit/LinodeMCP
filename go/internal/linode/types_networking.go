package linode

// Firewall represents a Linode Cloud Firewall.
type Firewall struct {
	Label   string        `json:"label"`
	Status  string        `json:"status"` // enabled, disabled, deleted
	Created string        `json:"created"`
	Updated string        `json:"updated"`
	Tags    []string      `json:"tags"`
	Rules   FirewallRules `json:"rules"`
	ID      int           `json:"id"`
}

// FirewallRules represents inbound and outbound firewall rules.
type FirewallRules struct {
	InboundPolicy  string         `json:"inbound_policy"`
	OutboundPolicy string         `json:"outbound_policy"`
	Fingerprint    string         `json:"fingerprint,omitempty"`
	Inbound        []FirewallRule `json:"inbound"`
	Outbound       []FirewallRule `json:"outbound"`
	Version        int            `json:"version,omitempty"`
}

// FirewallRule represents a single firewall rule.
type FirewallRule struct {
	Action      string            `json:"action"`   // ACCEPT, DROP
	Protocol    string            `json:"protocol"` // TCP, UDP, ICMP, IPENCAP
	Ports       string            `json:"ports"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Addresses   FirewallAddresses `json:"addresses"`
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
	Created string               `json:"created"`
	Updated string               `json:"updated"`
	Entity  FirewallDeviceEntity `json:"entity"`
	ID      int                  `json:"id"`
}

// FirewallDeviceEntity represents the Linode, Linode interface, or NodeBalancer attached to a firewall.
type FirewallDeviceEntity struct {
	ParentEntity *FirewallDeviceEntity `json:"parent_entity"`
	Label        string                `json:"label"`
	Type         string                `json:"type"`
	URL          string                `json:"url"`
	ID           int                   `json:"id"`
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
	RegionPrices []NetworkTransferRegionPrice `json:"region_prices"`
	Price        Price                        `json:"price"`
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
	RouteTarget  string `json:"route_target,omitempty"`
	PrefixLength int    `json:"prefix_length"`
}

// AllocateNetworkingIPRequest represents the request body for allocating an account-level IP address.
type AllocateNetworkingIPRequest struct {
	Type     string `json:"type"`
	LinodeID int    `json:"linode_id"`
	Public   bool   `json:"public"`
}

// UpdateNetworkingIPRequest represents the request body for updating account-level IP reverse DNS.
// Reserved is a pointer because an absent flag leaves the reservation alone,
// while an explicit false unreserves the address.
type UpdateNetworkingIPRequest struct {
	Reserved *bool  `json:"reserved,omitempty"`
	RDNS     string `json:"rdns"`
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
	IPs      []string `json:"ips"`
	LinodeID int      `json:"linode_id"`
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
	Label              string   `json:"label"`
	Region             string   `json:"region"`
	Hostname           string   `json:"hostname"`
	IPv4               string   `json:"ipv4"`
	IPv6               string   `json:"ipv6"`
	Created            string   `json:"created"`
	Updated            string   `json:"updated"`
	Tags               []string `json:"tags"`
	Transfer           Transfer `json:"transfer"`
	ID                 int      `json:"id"`
	ClientConnThrottle int      `json:"client_conn_throttle"`
}

// NodeBalancerConfig represents a NodeBalancer frontend configuration.
type NodeBalancerConfig struct {
	CipherSuite    string                  `json:"cipher_suite"`
	CheckPath      string                  `json:"check_path"`
	Protocol       string                  `json:"protocol"`
	Algorithm      string                  `json:"algorithm"`
	Stickiness     string                  `json:"stickiness"`
	Check          string                  `json:"check"`
	SSLFingerprint string                  `json:"ssl_fingerprint"`
	SSLCommonName  string                  `json:"ssl_commonname"`
	CheckBody      string                  `json:"check_body"`
	NodesStatus    NodeBalancerNodesStatus `json:"nodes_status"`
	Port           int                     `json:"port"`
	CheckAttempts  int                     `json:"check_attempts"`
	ID             int                     `json:"id"`
	CheckTimeout   int                     `json:"check_timeout"`
	CheckInterval  int                     `json:"check_interval"`
	NodeBalancerID int                     `json:"nodebalancer_id"`
	CheckPassive   bool                    `json:"check_passive"`
}

// CreateNodeBalancerConfigRequest represents the request body for creating a NodeBalancer config.
type CreateNodeBalancerConfigRequest struct {
	CheckPassive  *bool                           `json:"check_passive,omitempty"`
	CheckPath     string                          `json:"check_path,omitempty"`
	CipherSuite   string                          `json:"cipher_suite,omitempty"`
	Stickiness    string                          `json:"stickiness,omitempty"`
	Check         string                          `json:"check,omitempty"`
	ProxyProtocol string                          `json:"proxy_protocol,omitempty"`
	SSLKey        string                          `json:"ssl_key,omitempty"`
	Algorithm     string                          `json:"algorithm,omitempty"`
	Protocol      string                          `json:"protocol,omitempty"`
	CheckBody     string                          `json:"check_body,omitempty"`
	SSLCert       string                          `json:"ssl_cert,omitempty"`
	Nodes         []CreateNodeBalancerNodeRequest `json:"nodes,omitempty"`
	CheckAttempts int                             `json:"check_attempts,omitempty"`
	Port          int                             `json:"port"`
	CheckTimeout  int                             `json:"check_timeout,omitempty"`
	CheckInterval int                             `json:"check_interval,omitempty"`
	UDPCheckPort  int                             `json:"udp_check_port,omitempty"`
}

// RebuildNodeBalancerConfigRequest represents the request body for rebuilding a
// NodeBalancer config. Nodes carries no omitempty because rebuild replaces the
// whole backend set: an empty list means drop every node, not leave them alone.
// The rebuild endpoint documents no ssl, cipher, proxy, or check_passive options.
type RebuildNodeBalancerConfigRequest struct {
	CheckPath     string                          `json:"check_path,omitempty"`
	Stickiness    string                          `json:"stickiness,omitempty"`
	Check         string                          `json:"check,omitempty"`
	Algorithm     string                          `json:"algorithm,omitempty"`
	Protocol      string                          `json:"protocol,omitempty"`
	CheckBody     string                          `json:"check_body,omitempty"`
	Nodes         []CreateNodeBalancerNodeRequest `json:"nodes"`
	CheckAttempts int                             `json:"check_attempts,omitempty"`
	Port          int                             `json:"port,omitempty"`
	CheckTimeout  int                             `json:"check_timeout,omitempty"`
	CheckInterval int                             `json:"check_interval,omitempty"`
	UDPCheckPort  int                             `json:"udp_check_port,omitempty"`
}

// NodeBalancerNode represents a backend node on a NodeBalancer config.
type NodeBalancerNode struct {
	Label          string `json:"label"`
	Address        string `json:"address"`
	Status         string `json:"status"`
	Mode           string `json:"mode"`
	ID             int    `json:"id"`
	Weight         int    `json:"weight"`
	NodeBalancerID int    `json:"nodebalancer_id"`
	ConfigID       int    `json:"config_id"`
}

// CreateNodeBalancerNodeRequest represents the request body for creating a NodeBalancer config node.
type CreateNodeBalancerNodeRequest struct {
	Label    string `json:"label"`
	Address  string `json:"address"`
	Mode     string `json:"mode,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	SubnetID int    `json:"subnet_id,omitempty"`
}

// UpdateNodeBalancerNodeRequest represents the request body for updating a NodeBalancer config node.
type UpdateNodeBalancerNodeRequest struct {
	Label    string `json:"label,omitempty"`
	Address  string `json:"address,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	SubnetID int    `json:"subnet_id,omitempty"`
}

// UpdateNodeBalancerConfigRequest represents the request body for updating a NodeBalancer config.
type UpdateNodeBalancerConfigRequest struct {
	CheckPassive  *bool  `json:"check_passive,omitempty"`
	CheckPath     string `json:"check_path,omitempty"`
	SSLCert       string `json:"ssl_cert,omitempty"`
	Stickiness    string `json:"stickiness,omitempty"`
	Check         string `json:"check,omitempty"`
	CheckBody     string `json:"check_body,omitempty"`
	ProxyProtocol string `json:"proxy_protocol,omitempty"`
	Algorithm     string `json:"algorithm,omitempty"`
	CipherSuite   string `json:"cipher_suite,omitempty"`
	SSLKey        string `json:"ssl_key,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	CheckInterval int    `json:"check_interval,omitempty"`
	Port          int    `json:"port,omitempty"`
	CheckAttempts int    `json:"check_attempts,omitempty"`
	CheckTimeout  int    `json:"check_timeout,omitempty"`
	UDPCheckPort  int    `json:"udp_check_port,omitempty"`
}

// NodeBalancerNodesStatus represents the health summary for nodes on a NodeBalancer config.
type NodeBalancerNodesStatus struct {
	Up   int `json:"up"`
	Down int `json:"down"`
}

// Transfer represents data transfer statistics.
type Transfer struct {
	In    float64 `json:"in"`
	Out   float64 `json:"out"`
	Total float64 `json:"total"`
}

// CreateFirewallRequest is the wire form of a firewall-create request. Rules and
// Devices stay raw maps for the reason FirewallRulesReplaceRequest documents, so
// a create with only default policies sends a rules object holding just
// inbound_policy and outbound_policy.
type CreateFirewallRequest struct {
	Rules   map[string]any `json:"rules,omitempty"`
	Devices map[string]any `json:"devices,omitempty"`
	Label   string         `json:"label"`
	Tags    []string       `json:"tags,omitempty"`
}

// FirewallRulesReplaceRequest carries caller-supplied rule objects verbatim for a
// PUT /networking/firewalls/{id}/rules call. Rules stay raw maps because
// FirewallRule is a response-decode type with no omitempty tags: re-marshaling
// through it pads every rule with empty action/protocol/ports/label/description
// and a null ipv6 the caller never sent. The API owns rule field defaults.
type FirewallRulesReplaceRequest struct {
	InboundPolicy  string
	OutboundPolicy string
	Inbound        []map[string]any
	Outbound       []map[string]any
}

// Device represents a device attached to a firewall.
type Device struct {
	Type string `json:"type"` // linode, nodebalancer
	ID   int    `json:"id"`
}

// CreateFirewallDeviceRequest represents the request body for assigning a device to a firewall.
type CreateFirewallDeviceRequest struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
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
	IPv4               *string          `json:"ipv4,omitempty"`
	Region             string           `json:"region"`
	Label              string           `json:"label,omitempty"`
	Tags               []string         `json:"tags,omitempty"`
	Configs            []map[string]any `json:"configs,omitempty"`
	VPCs               []map[string]any `json:"vpcs,omitempty"`
	ClientConnThrottle int              `json:"client_conn_throttle,omitempty"`
	FirewallID         int              `json:"firewall_id,omitempty"`
}

// UpdateNodeBalancerRequest represents the request body for updating a NodeBalancer.
type UpdateNodeBalancerRequest struct {
	Label              string   `json:"label,omitempty"`
	ClientConnThrottle *int     `json:"client_conn_throttle,omitempty"`
	Tags               []string `json:"tags,omitempty"`
}
