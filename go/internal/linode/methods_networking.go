package linode

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// Most routes here have two decoders: httpXxx fills the hand-written struct and
// httpXxxProto fills the generated proto message. Both name the same tool, so
// they resolve one declared route, and the paginated helpers append
// page/page_size, which keeps the two runtime requests identical.

// IsObjectBody reports whether a decoded API body is a JSON object. Raw routes
// pass bodies through untouched so documented explicit nulls survive, and the
// bytes have already parsed as JSON, so the opening token settles the check.
func IsObjectBody(raw json.RawMessage) bool {
	opening := bytes.TrimLeft(raw, " \t\r\n")

	return len(opening) > 0 && opening[0] == '{'
}

// httpGetReservedIPRaw retrieves one reserved public IPv4 address, keeping the
// documented explicit nulls and empty arrays the API returns.
func (c *Client) httpGetReservedIPRaw(ctx context.Context, address string) (json.RawMessage, error) {
	addr, err := netip.ParseAddr(address)
	if err != nil || !addr.Is4() {
		return nil, ErrIPv4AddressInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_networking_reserved_ip_get", nil, address)
	if err != nil {
		return nil, wrapRequestError("GetReservedIP", err)
	}

	defer drainClose(resp)

	var reservedIP json.RawMessage
	if err := c.handleResponse(resp, &reservedIP); err != nil {
		return nil, err
	}

	return reservedIP, nil
}

// ListVLANs retrieves all VLANs for the authenticated user.
func (c *Client) httpListVLANs(ctx context.Context, page, pageSize int) (*PaginatedResponse[VLAN], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_vlan_list", pageQuery(page, pageSize), nil)
	if err != nil {
		return nil, wrapRequestError("ListVLANs", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[VLAN]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListFirewallRules retrieves the rules for a Cloud Firewall.
func (c *Client) httpListFirewallRules(ctx context.Context, firewallID int) (*FirewallRules, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_rules_get", nil, firewallID)
	if err != nil {
		return nil, wrapRequestError("ListFirewallRules", err)
	}

	defer drainClose(resp)

	var rules FirewallRules
	if err := c.handleResponse(resp, &rules); err != nil {
		return nil, err
	}

	return &rules, nil
}

// ListFirewallDevices retrieves devices assigned to a Cloud Firewall.
func (c *Client) httpListFirewallDevices(ctx context.Context, firewallID, page, pageSize int) (*PaginatedResponse[FirewallDevice], error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_firewall_device_list", pageQuery(page, pageSize), nil, firewallID)
	if err != nil {
		return nil, wrapRequestError("ListFirewallDevices", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[FirewallDevice]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetFirewallDevice retrieves one device assigned to a Cloud Firewall.
func (c *Client) httpGetFirewallDevice(ctx context.Context, firewallID, deviceID int) (*FirewallDevice, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	if deviceID <= 0 {
		return nil, ErrFirewallDeviceIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_device_get", nil, firewallID, deviceID)
	if err != nil {
		return nil, wrapRequestError("GetFirewallDevice", err)
	}

	defer drainClose(resp)

	var device FirewallDevice
	if err := c.handleResponse(resp, &device); err != nil {
		return nil, err
	}

	return &device, nil
}

// ListFirewallSettings retrieves default firewall assignments.
func (c *Client) httpListFirewallSettings(ctx context.Context, page, pageSize int) (*FirewallSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_firewall_settings_get", pageQuery(page, pageSize), nil)
	if err != nil {
		return nil, wrapRequestError("ListFirewallSettings", err)
	}

	defer drainClose(resp)

	var settings FirewallSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func validateNetworkingIPAddress(address string) error {
	if address == "" {
		return ErrIPAddressRequired
	}

	addr, err := netip.ParseAddr(address)
	if err != nil || addr.Zone() != "" {
		return ErrIPAddressInvalid
	}

	return nil
}

// GetFirewall retrieves a single firewall by its ID.
func (c *Client) httpGetFirewall(ctx context.Context, firewallID int) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_get", nil, firewallID)
	if err != nil {
		return nil, wrapRequestError("GetFirewall", err)
	}

	defer drainClose(resp)

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// GetNetworkingIP retrieves one account-level IP address.
func (c *Client) httpGetNetworkingIP(ctx context.Context, address string) (*IPAddress, error) {
	if err := validateNetworkingIPAddress(address); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_networking_ip_get", nil, address)
	if err != nil {
		return nil, wrapRequestError("GetNetworkingIP", err)
	}

	defer drainClose(resp)

	var ip IPAddress
	if err := c.handleResponse(resp, &ip); err != nil {
		return nil, err
	}

	return &ip, nil
}

// GetNodeBalancer retrieves a single NodeBalancer by its ID.
func (c *Client) httpGetNodeBalancer(ctx context.Context, nodeBalancerID int) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_get", nil, nodeBalancerID)
	if err != nil {
		return nil, wrapRequestError("GetNodeBalancer", err)
	}

	defer drainClose(resp)

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// ListNodeBalancerConfigs retrieves configs for a NodeBalancer by its ID.
func (c *Client) httpListNodeBalancerConfigs(ctx context.Context, nodeBalancerID, page, pageSize int) ([]NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_nodebalancer_config_list", pageQuery(page, pageSize), nil, nodeBalancerID)
	if err != nil {
		return nil, wrapRequestError("ListNodeBalancerConfigs", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[NodeBalancerConfig]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListNodeBalancerFirewallsProto retrieves the Cloud Firewalls assigned to a
// NodeBalancer as proto messages for the proto-backed list path.
func (c *Client) httpListNodeBalancerFirewallsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.Firewall, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	return listProtoElementsPaginatedRouted(ctx, c, "ListNodeBalancerFirewalls",
		"linode_nodebalancer_firewall_list", "", []any{nodeBalancerID}, page, pageSize,
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
}

// ListNodeBalancerConfigNodes retrieves nodes for a NodeBalancer config.
func (c *Client) httpListNodeBalancerConfigNodes(ctx context.Context, nodeBalancerID, configID, page, pageSize int) (*PaginatedResponse[NodeBalancerConfigNode], error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_nodebalancer_config_node_list", pageQuery(page, pageSize), nil, nodeBalancerID, configID)
	if err != nil {
		return nil, wrapRequestError("ListNodeBalancerConfigNodes", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[NodeBalancerConfigNode]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// httpGetNodeBalancerConfigProto retrieves one NodeBalancer config as a proto
// message.
// httpGetNodeBalancerConfig retrieves one NodeBalancer config as the struct the
// dry-run previews report, which serializes the API's own field set.
func (c *Client) httpGetNodeBalancerConfig(ctx context.Context, nodeBalancerID, configID int) (*NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_get", nil, nodeBalancerID, configID)
	if err != nil {
		return nil, wrapRequestError("GetNodeBalancerConfig", err)
	}

	defer drainClose(resp)

	var config NodeBalancerConfig
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// GetNodeBalancerConfigNode retrieves a single node for a NodeBalancer config.
func (c *Client) httpGetNodeBalancerConfigNode(ctx context.Context, nodeBalancerID, configID, nodeID int) (*NodeBalancerConfigNode, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	if nodeID <= 0 {
		return nil, ErrNodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_node_get", nil, nodeBalancerID, configID, nodeID)
	if err != nil {
		return nil, wrapRequestError("GetNodeBalancerConfigNode", err)
	}

	defer drainClose(resp)

	var node NodeBalancerConfigNode
	if err := c.handleResponse(resp, &node); err != nil {
		return nil, err
	}

	return &node, nil
}
