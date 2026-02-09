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
	InboundPolicy  string         `json:"inbound_policy"` //nolint:tagliatelle // Linode API snake_case
	Outbound       []FirewallRule `json:"outbound"`
	OutboundPolicy string         `json:"outbound_policy"` //nolint:tagliatelle // Linode API snake_case
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

// NodeBalancer represents a Linode NodeBalancer (load balancer).
type NodeBalancer struct {
	ID                 int      `json:"id"`
	Label              string   `json:"label"`
	Region             string   `json:"region"`
	Hostname           string   `json:"hostname"`
	IPv4               string   `json:"ipv4"`
	IPv6               string   `json:"ipv6"`
	ClientConnThrottle int      `json:"client_conn_throttle"` //nolint:tagliatelle // Linode API snake_case
	Transfer           Transfer `json:"transfer"`
	Tags               []string `json:"tags"`
	Created            string   `json:"created"`
	Updated            string   `json:"updated"`
}

// Transfer represents data transfer statistics.
type Transfer struct {
	In    float64 `json:"in"`
	Out   float64 `json:"out"`
	Total float64 `json:"total"`
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

// UpdateFirewallRequest represents the request body for updating a firewall.
type UpdateFirewallRequest struct {
	Label  string         `json:"label,omitempty"`
	Status string         `json:"status,omitempty"` // enabled, disabled
	Rules  *FirewallRules `json:"rules,omitempty"`
	Tags   []string       `json:"tags,omitempty"`
}

// CreateNodeBalancerRequest represents the request body for creating a NodeBalancer.
type CreateNodeBalancerRequest struct {
	Region             string   `json:"region"`
	Label              string   `json:"label,omitempty"`
	ClientConnThrottle int      `json:"client_conn_throttle,omitempty"` //nolint:tagliatelle // Linode API snake_case
	Tags               []string `json:"tags,omitempty"`
}

// UpdateNodeBalancerRequest represents the request body for updating a NodeBalancer.
type UpdateNodeBalancerRequest struct {
	Label              string   `json:"label,omitempty"`
	ClientConnThrottle *int     `json:"client_conn_throttle,omitempty"` //nolint:tagliatelle // Linode API snake_case
	Tags               []string `json:"tags,omitempty"`
}
