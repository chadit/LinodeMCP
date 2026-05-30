package linode

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
)

const (
	endpointNetworkingIPs         = "/networking/ips"
	endpointNetworkingIPsAssign   = endpointNetworkingIPs + "/assign"
	endpointNetworkingIPsShare    = "/networking/ipv4/share"
	endpointNetworkingIPv4Assign  = "/networking/ipv4/assign"
	endpointFirewalls             = "/networking/firewalls"
	endpointFirewallSettings      = endpointFirewalls + "/settings"
	endpointFirewallTemplates     = endpointFirewalls + "/templates"
	endpointNetworkTransferPrices = "/network-transfer/prices"
	endpointNetworkingIPv6Pools   = "/networking/ipv6/pools"
	endpointNetworkingIPv6Ranges  = "/networking/ipv6/ranges"
	endpointNetworkingVLANs       = "/networking/vlans"
	endpointNodeBalancers         = "/nodebalancers"
	endpointNodeBalancerTypes     = "/nodebalancers/types"
	endpointNodeBalancerConfigs   = endpointNodeBalancers + "/%d/configs"
	endpointNodeBalancerNodes     = endpointNodeBalancerConfigs + "/%d/nodes"
)

// ListFirewalls retrieves all Cloud Firewalls for the authenticated user.
func (c *Client) httpListFirewalls(ctx context.Context) ([]Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointFirewalls, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewalls", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Firewall]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListVLANs retrieves all VLANs for the authenticated user.
func (c *Client) httpListVLANs(ctx context.Context, page, pageSize int) (*PaginatedResponse[VLAN], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointNetworkingVLANs, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVLANs", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[VLAN]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// DeleteVLAN deletes one VLAN by region and label.
func (c *Client) httpDeleteVLAN(ctx context.Context, regionID, label string) error {
	if regionID == "" {
		return ErrRegionIDRequired
	}

	if label == "" {
		return ErrLabelRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointNetworkingVLANs+"/%s/%s", url.PathEscape(regionID), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteVLAN", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ListFirewallRules retrieves the rules for a Cloud Firewall.
func (c *Client) httpListFirewallRules(ctx context.Context, firewallID int) (*FirewallRules, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/rules"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewallRules", Err: err}
	}

	defer drainClose(resp)

	var rules FirewallRules
	if err := c.handleResponse(resp, &rules); err != nil {
		return nil, err
	}

	return &rules, nil
}

// UpdateFirewallRules replaces the rules for a Cloud Firewall.
func (c *Client) httpUpdateFirewallRules(ctx context.Context, firewallID int, req *FirewallRules) (*FirewallRules, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	if req == nil {
		return nil, ErrFirewallRulesRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/rules"

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateFirewallRules", Err: err}
	}

	defer drainClose(resp)

	var rules FirewallRules
	if err := c.handleResponse(resp, &rules); err != nil {
		return nil, err
	}

	return &rules, nil
}

// ListFirewallRuleVersions retrieves the rule-version history payload for a Cloud Firewall.
// The upstream OpenAPI schema for GET /networking/firewalls/{firewallId}/history
// is a firewall object, not a paginated collection; the rule version metadata is
// carried under the rules object.
func (c *Client) httpListFirewallRuleVersions(ctx context.Context, firewallID int) (*Firewall, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/history"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewallRuleVersions", Err: err}
	}

	defer drainClose(resp)

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// GetFirewallRuleVersion retrieves one version of a Cloud Firewall rule set.
func (c *Client) httpGetFirewallRuleVersion(ctx context.Context, firewallID, version int) (*Firewall, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	if version <= 0 {
		return nil, ErrFirewallRuleVersionPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	encodedVersion := url.PathEscape(strconv.Itoa(version))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/history/rules/" + encodedVersion

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetFirewallRuleVersion", Err: err}
	}

	defer drainClose(resp)

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// ListFirewallDevices retrieves devices assigned to a Cloud Firewall.
func (c *Client) httpListFirewallDevices(ctx context.Context, firewallID, page, pageSize int) (*PaginatedResponse[FirewallDevice], error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	endpoint := withPaginationQuery(endpointFirewalls+"/"+encodedFirewallID+"/devices", page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewallDevices", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[FirewallDevice]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateFirewallDevice assigns a device to a Cloud Firewall.
func (c *Client) httpCreateFirewallDevice(ctx context.Context, firewallID int, req *CreateFirewallDeviceRequest) (*FirewallDevice, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	if req == nil {
		return nil, ErrFirewallDeviceIDPositive
	}

	if req.ID <= 0 {
		return nil, ErrFirewallDeviceIDPositive
	}

	if req.Type == "" {
		return nil, ErrFirewallDeviceTypeRequired
	}

	if !isFirewallDeviceType(req.Type) {
		return nil, ErrInvalidFirewallDeviceType
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/devices"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateFirewallDevice", Err: err}
	}

	defer drainClose(resp)

	var device FirewallDevice
	if err := c.handleResponse(resp, &device); err != nil {
		return nil, err
	}

	return &device, nil
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

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	encodedDeviceID := url.PathEscape(strconv.Itoa(deviceID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/devices/" + encodedDeviceID

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetFirewallDevice", Err: err}
	}

	defer drainClose(resp)

	var device FirewallDevice
	if err := c.handleResponse(resp, &device); err != nil {
		return nil, err
	}

	return &device, nil
}

// DeleteFirewallDevice removes one device assignment from a Cloud Firewall.
func (c *Client) httpDeleteFirewallDevice(ctx context.Context, firewallID, deviceID int) error {
	if firewallID <= 0 {
		return ErrFirewallIDPositive
	}

	if deviceID <= 0 {
		return ErrFirewallDeviceIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	encodedDeviceID := url.PathEscape(strconv.Itoa(deviceID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/devices/" + encodedDeviceID

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteFirewallDevice", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

func isFirewallDeviceType(deviceType string) bool {
	switch deviceType {
	case "linode", "nodebalancer", "linode_interface":
		return true
	default:
		return false
	}
}

// ListFirewallSettings retrieves default firewall assignments.
func (c *Client) httpListFirewallSettings(ctx context.Context, page, pageSize int) (*FirewallSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointFirewallSettings, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewallSettings", Err: err}
	}

	defer drainClose(resp)

	var settings FirewallSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// httpUpdateFirewallSettings updates default firewall assignments.
func (c *Client) httpUpdateFirewallSettings(ctx context.Context, req *UpdateFirewallSettingsRequest) (*FirewallSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointFirewallSettings, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateFirewallSettings", Err: err}
	}

	defer drainClose(resp)

	var settings FirewallSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// ListFirewallTemplates retrieves reusable Cloud Firewall templates.
func (c *Client) httpListFirewallTemplates(ctx context.Context, page, pageSize int) (*PaginatedResponse[FirewallTemplate], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointFirewallTemplates, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewallTemplates", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[FirewallTemplate]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
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

func isFirewallTemplateSlug(slug string) bool {
	switch slug {
	case "public", "vpc":
		return true
	default:
		return false
	}
}

// GetFirewallTemplate retrieves a reusable Cloud Firewall template by slug.
func (c *Client) httpGetFirewallTemplate(ctx context.Context, slug string, page, pageSize int) (*PaginatedResponse[FirewallTemplate], error) {
	if !isFirewallTemplateSlug(slug) {
		return nil, ErrInvalidFirewallTemplateSlug
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointFirewallTemplates+"/"+url.PathEscape(slug), page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetFirewallTemplate", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[FirewallTemplate]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetFirewall retrieves a single firewall by its ID.
func (c *Client) httpGetFirewall(ctx context.Context, firewallID int) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointFirewalls+"/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetFirewall", Err: err}
	}

	defer drainClose(resp)

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// CreateFirewall creates a new Cloud Firewall.
func (c *Client) httpCreateFirewall(ctx context.Context, req CreateFirewallRequest) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointFirewalls, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateFirewall", Err: err}
	}

	defer drainClose(resp)

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// UpdateFirewall updates an existing Cloud Firewall.
func (c *Client) httpUpdateFirewall(ctx context.Context, firewallID int, req UpdateFirewallRequest) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointFirewalls+"/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateFirewall", Err: err}
	}

	defer drainClose(resp)

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// DeleteFirewall deletes a Cloud Firewall.
func (c *Client) httpDeleteFirewall(ctx context.Context, firewallID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointFirewalls+"/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteFirewall", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ListNetworkingIPs retrieves all IP addresses on the account.
func (c *Client) httpListNetworkingIPs(ctx context.Context, skipIPv6RDNS bool) (*PaginatedResponse[IPAddress], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointNetworkingIPs

	if skipIPv6RDNS {
		query := url.Values{}
		query.Set("skip_ipv6_rdns", "true")
		endpoint += "?" + query.Encode()
	}

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNetworkingIPs", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[IPAddress]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetNetworkingIP retrieves one account-level IP address.
func (c *Client) httpGetNetworkingIP(ctx context.Context, address string) (*IPAddress, error) {
	if err := validateNetworkingIPAddress(address); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointNetworkingIPs + "/" + url.PathEscape(address)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNetworkingIP", Err: err}
	}

	defer drainClose(resp)

	var ip IPAddress
	if err := c.handleResponse(resp, &ip); err != nil {
		return nil, err
	}

	return &ip, nil
}

// UpdateNetworkingIP updates reverse DNS for an account-level IP address.
func (c *Client) httpUpdateNetworkingIP(ctx context.Context, address string, req UpdateNetworkingIPRequest) (*IPAddress, error) {
	if err := validateNetworkingIPAddress(address); err != nil {
		return nil, err
	}

	if req.RDNS == "" {
		return nil, ErrRDNSRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointNetworkingIPs + "/" + url.PathEscape(address)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNetworkingIP", Err: err}
	}

	defer drainClose(resp)

	var ip IPAddress
	if err := c.handleResponse(resp, &ip); err != nil {
		return nil, err
	}

	return &ip, nil
}

// AllocateNetworkingIP allocates an account-level IP address.
func (c *Client) httpAllocateNetworkingIP(ctx context.Context, req AllocateNetworkingIPRequest) (*IPAddress, error) {
	if req.LinodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointNetworkingIPs, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AllocateNetworkingIP", Err: err}
	}

	defer drainClose(resp)

	var ip IPAddress
	if err := c.handleResponse(resp, &ip); err != nil {
		return nil, err
	}

	return &ip, nil
}

// AssignNetworkingIPs assigns IP addresses to Linodes in a region.
func (c *Client) httpAssignNetworkingIPs(ctx context.Context, req AssignNetworkingIPsRequest) (map[string]any, error) {
	return c.httpAssignNetworkingIPsAtEndpoint(ctx, endpointNetworkingIPsAssign, "AssignNetworkingIPs", req)
}

// AssignNetworkingIPv4s assigns IPv4 addresses to Linodes in a region.
func (c *Client) httpAssignNetworkingIPv4s(ctx context.Context, req AssignNetworkingIPsRequest) (map[string]any, error) {
	if err := validateIPv4Assignments(req.Assignments); err != nil {
		return nil, err
	}

	return c.httpAssignNetworkingIPsAtEndpoint(ctx, endpointNetworkingIPv4Assign, "AssignNetworkingIPv4s", req)
}

func validateIPv4Assignments(assignments []IPAssignment) error {
	for _, assignment := range assignments {
		if assignment.Address == "" {
			continue
		}

		address, err := netip.ParseAddr(assignment.Address)
		if err != nil || !address.Is4() {
			return ErrIPv4AddressInvalid
		}
	}

	return nil
}

func (c *Client) httpAssignNetworkingIPsAtEndpoint(
	ctx context.Context,
	endpoint string,
	operation string,
	req AssignNetworkingIPsRequest,
) (map[string]any, error) {
	if req.Region == "" {
		return nil, ErrRegionRequired
	}

	if len(req.Assignments) == 0 {
		return nil, ErrIPAssignmentsRequired
	}

	for _, assignment := range req.Assignments {
		if assignment.Address == "" {
			return nil, ErrIPAddressRequired
		}

		if assignment.LinodeID <= 0 {
			return nil, ErrLinodeIDPositive
		}
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: operation, Err: err}
	}

	defer drainClose(resp)

	response := map[string]any{}
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response, nil
}

// ShareNetworkingIPs shares IP addresses with a primary Linode.
func (c *Client) httpShareNetworkingIPs(ctx context.Context, req ShareNetworkingIPsRequest) (map[string]any, error) {
	if req.LinodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req.IPs == nil {
		return nil, ErrIPAddressRequired
	}

	if slices.Contains(req.IPs, "") {
		return nil, ErrIPAddressRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointNetworkingIPsShare, req)
	if err != nil {
		return nil, &NetworkError{Operation: "ShareNetworkingIPs", Err: err}
	}

	defer drainClose(resp)

	response := map[string]any{}
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response, nil
}

// ListNetworkTransferPrices retrieves network transfer prices.
func (c *Client) httpListNetworkTransferPrices(ctx context.Context) (*PaginatedResponse[NetworkTransferPrice], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointNetworkTransferPrices, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNetworkTransferPrices", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[NetworkTransferPrice]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListIPv6Pools retrieves IPv6 pools for the authenticated user.
func (c *Client) httpListIPv6Pools(ctx context.Context, page, pageSize int) (*PaginatedResponse[IPv6Pool], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointNetworkingIPv6Pools, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListIPv6Pools", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[IPv6Pool]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListIPv6Ranges retrieves IPv6 ranges for the authenticated user.
func (c *Client) httpListIPv6Ranges(ctx context.Context, page, pageSize int) (*PaginatedResponse[IPv6Range], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointNetworkingIPv6Ranges, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListIPv6Ranges", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[IPv6Range]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// CreateIPv6Range creates an IPv6 range for the authenticated user.
func (c *Client) httpCreateIPv6Range(ctx context.Context, req CreateIPv6RangeRequest) (*IPv6Range, error) {
	if req.PrefixLength < 1 || req.PrefixLength > 128 {
		return nil, ErrIPv6RangePrefixRange
	}

	if req.LinodeID != nil && *req.LinodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req.RouteTarget != "" {
		routeTarget, err := netip.ParseAddr(req.RouteTarget)
		if err != nil || !routeTarget.Is6() || routeTarget.Zone() != "" {
			return nil, ErrIPv6RangeRouteTargetInvalid
		}
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointNetworkingIPv6Ranges, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateIPv6Range", Err: err}
	}

	defer drainClose(resp)

	var ipv6Range IPv6Range
	if err := c.handleResponse(resp, &ipv6Range); err != nil {
		return nil, err
	}

	return &ipv6Range, nil
}

// ListNodeBalancerTypes retrieves available NodeBalancer types.
func (c *Client) httpListNodeBalancerTypes(ctx context.Context) (*PaginatedResponse[NodeBalancerType], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointNodeBalancerTypes, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNodeBalancerTypes", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[NodeBalancerType]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListNodeBalancers retrieves all NodeBalancers for the authenticated user.
func (c *Client) httpListNodeBalancers(ctx context.Context) ([]NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointNodeBalancers, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNodeBalancers", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[NodeBalancer]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetNodeBalancer retrieves a single NodeBalancer by its ID.
func (c *Client) httpGetNodeBalancer(ctx context.Context, nodeBalancerID int) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointNodeBalancers+"/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// ListNodeBalancerConfigs retrieves configs for a NodeBalancer by its ID.
func (c *Client) httpListNodeBalancerConfigs(ctx context.Context, nodeBalancerID int) ([]NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointNodeBalancerConfigs, nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNodeBalancerConfigs", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[NodeBalancerConfig]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
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

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := withPaginationQuery(endpointNodeBalancers+"/"+encodedNodeBalancerID+"/configs/"+encodedConfigID+"/nodes", page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNodeBalancerConfigNodes", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[NodeBalancerConfigNode]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetNodeBalancerConfig retrieves one config for a NodeBalancer by IDs.
func (c *Client) httpGetNodeBalancerConfig(ctx context.Context, nodeBalancerID, configID int) (*NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/configs/" + encodedConfigID

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancerConfig", Err: err}
	}

	defer drainClose(resp)

	var config NodeBalancerConfig
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// CreateNodeBalancerConfig creates a config for a NodeBalancer by its ID.
func (c *Client) httpCreateNodeBalancerConfig(ctx context.Context, nodeBalancerID int, req *CreateNodeBalancerConfigRequest) (*NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if nodeBalancerID < 1 {
		return nil, ErrNodeBalancerIDPositive
	}

	if req == nil {
		return nil, ErrCreateConfigRequestRequired
	}

	endpoint := fmt.Sprintf(endpointNodeBalancerConfigs, nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateNodeBalancerConfig", Err: err}
	}

	defer drainClose(resp)

	var config NodeBalancerConfig
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// CreateNodeBalancerNode creates a node for a NodeBalancer config.
func (c *Client) httpCreateNodeBalancerNode(ctx context.Context, nodeBalancerID, configID int, req *CreateNodeBalancerNodeRequest) (*NodeBalancerNode, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if nodeBalancerID < 1 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID < 1 {
		return nil, ErrConfigIDPositive
	}

	if req == nil {
		return nil, ErrCreateNodeBalancerNodeRequestRequired
	}

	endpoint := fmt.Sprintf(endpointNodeBalancerNodes, nodeBalancerID, configID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateNodeBalancerNode", Err: err}
	}

	defer drainClose(resp)

	var node NodeBalancerNode
	if err := c.handleResponse(resp, &node); err != nil {
		return nil, err
	}

	return &node, nil
}

// CreateNodeBalancer creates a new NodeBalancer.
func (c *Client) httpCreateNodeBalancer(ctx context.Context, req CreateNodeBalancerRequest) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointNodeBalancers, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// UpdateNodeBalancer updates an existing NodeBalancer.
func (c *Client) httpUpdateNodeBalancer(ctx context.Context, nodeBalancerID int, req UpdateNodeBalancerRequest) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointNodeBalancers+"/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// DeleteNodeBalancer deletes a NodeBalancer.
func (c *Client) httpDeleteNodeBalancer(ctx context.Context, nodeBalancerID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointNodeBalancers+"/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}
