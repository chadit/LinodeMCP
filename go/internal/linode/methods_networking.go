package linode

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const (
	endpointNetworkingIPs         = "/networking/ips"
	endpointNetworkingIPsAssign   = endpointNetworkingIPs + "/assign"
	endpointNetworkingIPsShare    = endpointNetworkingIPs + "/share"
	endpointNetworkingIPv4Share   = "/networking/ipv4/share"
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
	endpointNodeBalancerVPCs      = endpointNodeBalancers + "/%s/vpcs"
	endpointNodeBalancerConfigs   = endpointNodeBalancers + "/%d/configs"
	endpointNodeBalancerNodes     = endpointNodeBalancerConfigs + "/%d/nodes"
)

// DeleteNodeBalancerConfig deletes one config from a NodeBalancer.
func (c *Client) httpDeleteNodeBalancerConfig(ctx context.Context, nodeBalancerID, configID int) error {
	if nodeBalancerID <= 0 {
		return ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return ErrConfigIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/configs/" + encodedConfigID

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteNodeBalancerConfig", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

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

// httpListFirewallsProto retrieves all Cloud Firewalls as proto messages,
// decoded directly from the API JSON for the proto-backed read path.
func (c *Client) httpListFirewallsProto(ctx context.Context) ([]*linodev1.Firewall, error) {
	return listProtoElements(ctx, c, "ListFirewalls", endpointFirewalls,
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
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

// httpListVLANsProto retrieves VLANs as proto messages for the proto-backed list
// path. page/page_size flow through withPaginationQuery, so the request matches
// the non-proto method.
func (c *Client) httpListVLANsProto(ctx context.Context, page, pageSize int) ([]*linodev1.VLAN, error) {
	return listProtoElementsPaginated(ctx, c, "ListVLANs", endpointNetworkingVLANs, page, pageSize,
		func() *linodev1.VLAN { return &linodev1.VLAN{} })
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

// ListNodeBalancerVPCs retrieves VPC configurations attached to a NodeBalancer.
func (c *Client) httpListNodeBalancerVPCs(ctx context.Context, nodeBalancerID, page, pageSize int) (*PaginatedResponse[NodeBalancerVPCConfig], error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	endpoint := withPaginationQuery(fmt.Sprintf(endpointNodeBalancerVPCs, encodedNodeBalancerID), page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNodeBalancerVPCs", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[NodeBalancerVPCConfig]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// httpListNodeBalancerVPCsProto retrieves a NodeBalancer's VPC configurations as
// proto messages for the proto-backed list path. The endpoint is formatted with
// the same fmt.Sprintf(endpointNodeBalancerVPCs, encodedNodeBalancerID) pattern
// httpListNodeBalancerVPCs uses, then listProtoElementsPaginated adds
// page/page_size via withPaginationQuery, so the runtime request matches exactly.
func (c *Client) httpListNodeBalancerVPCsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.NodeBalancerVPCConfig, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	endpoint := fmt.Sprintf(endpointNodeBalancerVPCs, encodedNodeBalancerID)

	return listProtoElementsPaginated(ctx, c, "ListNodeBalancerVPCs", endpoint, page, pageSize,
		func() *linodev1.NodeBalancerVPCConfig { return &linodev1.NodeBalancerVPCConfig{} })
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

// httpListFirewallRulesProto retrieves a firewall's ruleset as a proto message.
func (c *Client) httpListFirewallRulesProto(ctx context.Context, firewallID int) (*linodev1.FirewallRules, error) {
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

	rules := &linodev1.FirewallRules{}
	if err := c.handleProtoResponse(resp, rules); err != nil {
		return nil, err
	}

	return rules, nil
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

	body := firewallRulesReplaceBody{Inbound: req.Inbound, Outbound: req.Outbound}

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, body)
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

// httpListFirewallRuleVersionsProto retrieves a Cloud Firewall's rule-version
// history as proto messages for the proto-backed list path. GET
// /networking/firewalls/{firewallId}/history returns a paginated {data:[...]}
// list of version snapshots (each a firewall-shaped object plus a top-level
// version), so listProtoElements decodes the data[] envelope element-for-element
// the same way the Python client does.
func (c *Client) httpListFirewallRuleVersionsProto(ctx context.Context, firewallID int) ([]*linodev1.FirewallRuleVersion, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/history"

	return listProtoElements(ctx, c, "ListFirewallRuleVersions", endpoint,
		func() *linodev1.FirewallRuleVersion { return &linodev1.FirewallRuleVersion{} })
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

// httpGetFirewallRuleVersionProto retrieves one rule-version snapshot and decodes
// the response into the FirewallRuleVersion proto element, the same element the
// rule-version LIST path emits. The endpoint returns a bare firewall-shaped
// object (not a {data:[...]} page), so it decodes directly.
func (c *Client) httpGetFirewallRuleVersionProto(ctx context.Context, firewallID, version int) (*linodev1.FirewallRuleVersion, error) {
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

	ruleVersion := &linodev1.FirewallRuleVersion{}
	if err := c.handleProtoResponse(resp, ruleVersion); err != nil {
		return nil, err
	}

	return ruleVersion, nil
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

// httpListFirewallDevicesProto retrieves a Cloud Firewall's assigned devices as
// proto messages for the proto-backed list path. The endpoint is formatted with
// the same encoded-firewall-id path httpListFirewallDevices uses, then
// listProtoElementsPaginated adds page/page_size via withPaginationQuery, so the
// runtime request matches exactly.
func (c *Client) httpListFirewallDevicesProto(ctx context.Context, firewallID, page, pageSize int) ([]*linodev1.FirewallDevice, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	encodedFirewallID := url.PathEscape(strconv.Itoa(firewallID))
	endpoint := endpointFirewalls + "/" + encodedFirewallID + "/devices"

	return listProtoElementsPaginated(ctx, c, "ListFirewallDevices", endpoint, page, pageSize,
		func() *linodev1.FirewallDevice { return &linodev1.FirewallDevice{} })
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

// httpCreateFirewallDeviceProto assigns a device to a Cloud Firewall and decodes
// the response into the FirewallDevice proto element so the write tool emits the
// same shape as the firewall device read path.
func (c *Client) httpCreateFirewallDeviceProto(ctx context.Context, firewallID int, req *CreateFirewallDeviceRequest) (*linodev1.FirewallDevice, error) {
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

	device := &linodev1.FirewallDevice{}
	if err := c.handleProtoResponse(resp, device); err != nil {
		return nil, err
	}

	return device, nil
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

// httpGetFirewallDeviceProto retrieves one firewall device as a proto message.
func (c *Client) httpGetFirewallDeviceProto(ctx context.Context, firewallID, deviceID int) (*linodev1.FirewallDevice, error) {
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

	device := &linodev1.FirewallDevice{}
	if err := c.handleProtoResponse(resp, device); err != nil {
		return nil, err
	}

	return device, nil
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

// httpListFirewallSettingsProto retrieves default firewall assignments and
// decodes the single-object response into the FirewallSettings proto element so
// the read tool emits the same shape as the firewall settings write path.
func (c *Client) httpListFirewallSettingsProto(ctx context.Context, page, pageSize int) (*linodev1.FirewallSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointFirewallSettings, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewallSettings", Err: err}
	}

	defer drainClose(resp)

	settings := &linodev1.FirewallSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
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

// httpUpdateFirewallSettingsProto updates default firewall assignments and
// decodes the response into the FirewallSettings proto element so the write tool
// emits the same shape as the firewall settings read path.
func (c *Client) httpUpdateFirewallSettingsProto(ctx context.Context, req *UpdateFirewallSettingsRequest) (*linodev1.FirewallSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointFirewallSettings, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateFirewallSettings", Err: err}
	}

	defer drainClose(resp)

	settings := &linodev1.FirewallSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// httpListFirewallTemplatesProto retrieves reusable Cloud Firewall templates as
// proto FirewallTemplate messages for the proto-backed list path. page/page_size
// flow through withPaginationQuery, so the request matches httpListFirewallTemplates.
func (c *Client) httpListFirewallTemplatesProto(ctx context.Context, page, pageSize int) ([]*linodev1.FirewallTemplate, error) {
	return listProtoElementsPaginated(ctx, c, "ListFirewallTemplates", endpointFirewallTemplates, page, pageSize,
		func() *linodev1.FirewallTemplate { return &linodev1.FirewallTemplate{} })
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

// httpGetFirewallTemplateProto retrieves a reusable Cloud Firewall template by
// slug and decodes the response into the FirewallTemplate proto element, the same
// element the template LIST path emits. The by-slug endpoint returns a single
// bare template object, so it decodes directly.
func (c *Client) httpGetFirewallTemplateProto(ctx context.Context, slug string, page, pageSize int) (*linodev1.FirewallTemplate, error) {
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

	template := &linodev1.FirewallTemplate{}
	if err := c.handleProtoResponse(resp, template); err != nil {
		return nil, err
	}

	return template, nil
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

// httpGetFirewallProto retrieves one Cloud Firewall as a proto message.
func (c *Client) httpGetFirewallProto(ctx context.Context, firewallID int) (*linodev1.Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointFirewalls+"/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetFirewall", Err: err}
	}

	defer drainClose(resp)

	firewall := &linodev1.Firewall{}
	if err := c.handleProtoResponse(resp, firewall); err != nil {
		return nil, err
	}

	return firewall, nil
}

// CreateFirewall creates a new Cloud Firewall.
func (c *Client) httpCreateFirewall(ctx context.Context, req CreateFirewallRequest) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointFirewalls, firewallCreateBodyFromRequest(req))
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

// httpCreateFirewallProto creates a Cloud Firewall and decodes the response into
// the Firewall proto element so the write tool emits the same field set as the
// firewall GET/LIST path.
func (c *Client) httpCreateFirewallProto(ctx context.Context, req CreateFirewallRequest) (*linodev1.Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointFirewalls, firewallCreateBodyFromRequest(req))
	if err != nil {
		return nil, &NetworkError{Operation: "CreateFirewall", Err: err}
	}

	defer drainClose(resp)

	firewall := &linodev1.Firewall{}
	if err := c.handleProtoResponse(resp, firewall); err != nil {
		return nil, err
	}

	return firewall, nil
}

// httpUpdateFirewallProto updates a Cloud Firewall and decodes the response into
// the Firewall proto element.
func (c *Client) httpUpdateFirewallProto(ctx context.Context, firewallID int, req UpdateFirewallRequest) (*linodev1.Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointFirewalls+"/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateFirewall", Err: err}
	}

	defer drainClose(resp)

	firewall := &linodev1.Firewall{}
	if err := c.handleProtoResponse(resp, firewall); err != nil {
		return nil, err
	}

	return firewall, nil
}

// httpUpdateFirewallRulesProto replaces a Cloud Firewall's rules and decodes the
// response into the FirewallRules proto element so the write tool emits the same
// ruleset shape as the rules GET path. The request carries the caller's rule
// objects verbatim (see FirewallRulesReplaceRequest) so Go and the Python client
// put identical bytes on the wire.
func (c *Client) httpUpdateFirewallRulesProto(ctx context.Context, firewallID int, req *FirewallRulesReplaceRequest) (*linodev1.FirewallRules, error) {
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

	body := firewallRulesRawReplaceBody{Inbound: req.Inbound, Outbound: req.Outbound}

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, body)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateFirewallRules", Err: err}
	}

	defer drainClose(resp)

	rules := &linodev1.FirewallRules{}
	if err := c.handleProtoResponse(resp, rules); err != nil {
		return nil, err
	}

	return rules, nil
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

// httpListNetworkingIPsProto retrieves account IP addresses as proto messages
// for the proto-backed list path. skip_ipv6_rdns flows through the same query
// param as httpListNetworkingIPs, so the request matches.
func (c *Client) httpListNetworkingIPsProto(ctx context.Context, skipIPv6RDNS bool) ([]*linodev1.IPAddress, error) {
	endpoint := endpointNetworkingIPs

	if skipIPv6RDNS {
		query := url.Values{}
		query.Set("skip_ipv6_rdns", "true")
		endpoint += "?" + query.Encode()
	}

	return listProtoElements(ctx, c, "ListNetworkingIPs", endpoint,
		func() *linodev1.IPAddress { return &linodev1.IPAddress{} })
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

// httpGetNetworkingIPProto retrieves a networking IP as a proto message.
func (c *Client) httpGetNetworkingIPProto(ctx context.Context, address string) (*linodev1.IPAddress, error) {
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

	ip := &linodev1.IPAddress{}
	if err := c.handleProtoResponse(resp, ip); err != nil {
		return nil, err
	}

	return ip, nil
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

// httpUpdateNetworkingIPProto updates a networking IP and decodes the response as
// a proto message.
func (c *Client) httpUpdateNetworkingIPProto(ctx context.Context, address string, req UpdateNetworkingIPRequest) (*linodev1.IPAddress, error) {
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

	ip := &linodev1.IPAddress{}
	if err := c.handleProtoResponse(resp, ip); err != nil {
		return nil, err
	}

	return ip, nil
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

// httpAllocateNetworkingIPProto allocates an account-level IP address and
// decodes the response into the proto IPAddress element.
func (c *Client) httpAllocateNetworkingIPProto(ctx context.Context, req AllocateNetworkingIPRequest) (*linodev1.IPAddress, error) {
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

	ip := &linodev1.IPAddress{}
	if err := c.handleProtoResponse(resp, ip); err != nil {
		return nil, err
	}

	return ip, nil
}

// AssignNetworkingIPs assigns IP addresses to Linodes in a region.
func (c *Client) httpAssignNetworkingIPs(ctx context.Context, req AssignNetworkingIPsRequest) (map[string]any, error) {
	if err := validateAssignNetworkingIPsRequest(req); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointNetworkingIPsAssign, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AssignNetworkingIPs", Err: err}
	}

	defer drainClose(resp)

	response := map[string]any{}
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response, nil
}

// AssignNetworkingIPv4s assigns IPv4 addresses to Linodes in a region.
func (c *Client) httpAssignNetworkingIPv4s(ctx context.Context, req AssignNetworkingIPsRequest) (map[string]any, error) {
	if err := validateIPv4Assignments(req.Assignments); err != nil {
		return nil, err
	}

	if err := validateAssignNetworkingIPsRequest(req); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointNetworkingIPv4Assign, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AssignNetworkingIPv4s", Err: err}
	}

	defer drainClose(resp)

	response := map[string]any{}
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response, nil
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

func validateAssignNetworkingIPsRequest(req AssignNetworkingIPsRequest) error {
	if req.Region == "" {
		return ErrRegionRequired
	}

	if len(req.Assignments) == 0 {
		return ErrIPAssignmentsRequired
	}

	for _, assignment := range req.Assignments {
		if assignment.Address == "" {
			return ErrIPAddressRequired
		}

		if assignment.LinodeID <= 0 {
			return ErrLinodeIDPositive
		}
	}

	return nil
}

// ShareNetworkingIPv4s shares IP addresses with a primary Linode.
func (c *Client) httpShareNetworkingIPv4s(ctx context.Context, req ShareNetworkingIPsRequest) (map[string]any, error) {
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

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointNetworkingIPv4Share, req)
	if err != nil {
		return nil, &NetworkError{Operation: "ShareNetworkingIPv4s", Err: err}
	}

	defer drainClose(resp)

	response := map[string]any{}
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response, nil
}

// ShareNetworkingIPs shares IP addresses with a primary Linode via the
// generic /networking/ips/share endpoint (the IPv4-specific variant above
// uses /networking/ipv4/share).
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

// httpListNetworkTransferPricesProto retrieves network transfer prices as proto
// LinodeType messages for the proto-backed list path. The element shares the
// LinodeType shape (id, label, price, region_prices[], transfer).
func (c *Client) httpListNetworkTransferPricesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	return listProtoElements(ctx, c, "ListNetworkTransferPrices", endpointNetworkTransferPrices,
		func() *linodev1.LinodeType { return &linodev1.LinodeType{} })
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

// httpListIPv6PoolsProto retrieves IPv6 pools as proto IPv6Pool messages for the
// proto-backed list path. page/page_size flow through withPaginationQuery, so the
// request matches httpListIPv6Pools.
func (c *Client) httpListIPv6PoolsProto(ctx context.Context, page, pageSize int) ([]*linodev1.IPv6Pool, error) {
	return listProtoElementsPaginated(ctx, c, "ListIPv6Pools", endpointNetworkingIPv6Pools, page, pageSize,
		func() *linodev1.IPv6Pool { return &linodev1.IPv6Pool{} })
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

// httpListIPv6RangesProto retrieves IPv6 ranges as proto IPv6Range messages for
// the proto-backed list path. page/page_size flow through withPaginationQuery, so
// the request matches httpListIPv6Ranges.
func (c *Client) httpListIPv6RangesProto(ctx context.Context, page, pageSize int) ([]*linodev1.IPv6Range, error) {
	return listProtoElementsPaginated(ctx, c, "ListIPv6Ranges", endpointNetworkingIPv6Ranges, page, pageSize,
		func() *linodev1.IPv6Range { return &linodev1.IPv6Range{} })
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

// httpCreateIPv6RangeProto creates an IPv6 range and decodes the response into
// the IPv6Range proto element so the write tool emits the same shape as the
// IPv6 range read path.
func (c *Client) httpCreateIPv6RangeProto(ctx context.Context, req CreateIPv6RangeRequest) (*linodev1.IPv6Range, error) {
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

	ipv6Range := &linodev1.IPv6Range{}
	if err := c.handleProtoResponse(resp, ipv6Range); err != nil {
		return nil, err
	}

	return ipv6Range, nil
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

// httpListNodeBalancerTypesProto retrieves available NodeBalancer types as proto
// messages, decoded directly from the API JSON for the proto-backed list path.
func (c *Client) httpListNodeBalancerTypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	return listProtoElements(ctx, c, "ListNodeBalancerTypes", endpointNodeBalancerTypes,
		func() *linodev1.LinodeType { return &linodev1.LinodeType{} })
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

// httpListNodeBalancersProto retrieves all NodeBalancers as proto messages,
// decoded directly from the API JSON for the proto-backed read path.
func (c *Client) httpListNodeBalancersProto(ctx context.Context) ([]*linodev1.NodeBalancer, error) {
	return listProtoElements(ctx, c, "ListNodeBalancers", endpointNodeBalancers,
		func() *linodev1.NodeBalancer { return &linodev1.NodeBalancer{} })
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

// GetNodeBalancerVPCConfig retrieves a NodeBalancer VPC configuration by ID.
func (c *Client) httpGetNodeBalancerVPCConfig(ctx context.Context, nodeBalancerID, vpcConfigID int) (*NodeBalancerVPCConfig, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	if vpcConfigID <= 0 {
		return nil, ErrConfigIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedVPCConfigID := url.PathEscape(strconv.Itoa(vpcConfigID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/vpcs/" + encodedVPCConfigID

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancerVPCConfig", Err: err}
	}

	defer drainClose(resp)

	var config NodeBalancerVPCConfig
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// httpGetNodeBalancerVPCConfigProto retrieves one NodeBalancer VPC config as a
// proto message.
func (c *Client) httpGetNodeBalancerVPCConfigProto(ctx context.Context, nodeBalancerID, vpcConfigID int) (*linodev1.NodeBalancerVPCConfig, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	if vpcConfigID <= 0 {
		return nil, ErrConfigIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedVPCConfigID := url.PathEscape(strconv.Itoa(vpcConfigID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/vpcs/" + encodedVPCConfigID

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancerVPCConfig", Err: err}
	}

	defer drainClose(resp)

	config := &linodev1.NodeBalancerVPCConfig{}
	if err := c.handleProtoResponse(resp, config); err != nil {
		return nil, err
	}

	return config, nil
}

// ListNodeBalancerConfigs retrieves configs for a NodeBalancer by its ID.
func (c *Client) httpListNodeBalancerConfigs(ctx context.Context, nodeBalancerID, page, pageSize int) ([]NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(fmt.Sprintf(endpointNodeBalancerConfigs, nodeBalancerID), page, pageSize)

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

// httpListNodeBalancerConfigsProto retrieves a NodeBalancer's configs as proto
// messages for the proto-backed list path. The endpoint is formatted with the
// same fmt.Sprintf(endpointNodeBalancerConfigs, nodeBalancerID) pattern
// httpListNodeBalancerConfigs uses, then listProtoElementsPaginated adds
// page/page_size via withPaginationQuery, so the runtime request matches exactly.
func (c *Client) httpListNodeBalancerConfigsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.NodeBalancerConfig, error) {
	endpoint := fmt.Sprintf(endpointNodeBalancerConfigs, nodeBalancerID)

	return listProtoElementsPaginated(ctx, c, "ListNodeBalancerConfigs", endpoint, page, pageSize,
		func() *linodev1.NodeBalancerConfig { return &linodev1.NodeBalancerConfig{} })
}

// ListNodeBalancerFirewalls retrieves Cloud Firewalls assigned to a NodeBalancer.
func (c *Client) httpListNodeBalancerFirewalls(ctx context.Context, nodeBalancerID, page, pageSize int) ([]Firewall, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	endpoint := withPaginationQuery(endpointNodeBalancers+"/"+encodedNodeBalancerID+"/firewalls", page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNodeBalancerFirewalls", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Firewall]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListNodeBalancerFirewallsProto retrieves the Cloud Firewalls assigned to a
// NodeBalancer as proto messages for the proto-backed list path. The endpoint is
// formatted with the same encoded nodebalancer-id path
// httpListNodeBalancerFirewalls uses, then listProtoElementsPaginated adds
// page/page_size via withPaginationQuery, so the runtime request matches exactly.
func (c *Client) httpListNodeBalancerFirewallsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.Firewall, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/firewalls"

	return listProtoElementsPaginated(ctx, c, "ListNodeBalancerFirewalls", endpoint, page, pageSize,
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
}

// UpdateNodeBalancerFirewalls replaces firewall assignments for a NodeBalancer.
func (c *Client) httpUpdateNodeBalancerFirewalls(ctx context.Context, nodeBalancerID, page, pageSize int, req *UpdateNodeBalancerFirewallsRequest) ([]Firewall, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	if req == nil {
		return nil, ErrUpdateNodeBalancerFirewallsRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	endpoint := withPaginationQuery(endpointNodeBalancers+"/"+encodedNodeBalancerID+"/firewalls", page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancerFirewalls", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Firewall]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpUpdateNodeBalancerFirewallsProto replaces firewall assignments for a
// NodeBalancer and decodes the returned page into Firewall proto elements so the
// write tool emits the same shape as the NodeBalancer firewall list path.
func (c *Client) httpUpdateNodeBalancerFirewallsProto(ctx context.Context, nodeBalancerID, page, pageSize int, req *UpdateNodeBalancerFirewallsRequest) ([]*linodev1.Firewall, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	if req == nil {
		return nil, ErrUpdateNodeBalancerFirewallsRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	endpoint := withPaginationQuery(endpointNodeBalancers+"/"+encodedNodeBalancerID+"/firewalls", page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancerFirewalls", Err: err}
	}

	defer drainClose(resp)

	return decodeProtoElementsBareOrData(resp, c, "UpdateNodeBalancerFirewalls",
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

// httpListNodeBalancerConfigNodesProto retrieves the backend nodes of one
// NodeBalancer config as proto messages. It formats both path ids into the
// endpoint exactly like httpListNodeBalancerConfigNodes, then reuses
// listProtoElementsPaginated (which adds the page/page_size query) to decode the
// {data:[...]} page envelope.
func (c *Client) httpListNodeBalancerConfigNodesProto(ctx context.Context, nodeBalancerID, configID, page, pageSize int) ([]*linodev1.NodeBalancerConfigNode, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/configs/" + encodedConfigID + "/nodes"

	return listProtoElementsPaginated(ctx, c, "ListNodeBalancerConfigNodes", endpoint, page, pageSize,
		func() *linodev1.NodeBalancerConfigNode { return &linodev1.NodeBalancerConfigNode{} })
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

// httpGetNodeBalancerConfigProto retrieves one NodeBalancer config as a proto
// message.
func (c *Client) httpGetNodeBalancerConfigProto(ctx context.Context, nodeBalancerID, configID int) (*linodev1.NodeBalancerConfig, error) {
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

	config := &linodev1.NodeBalancerConfig{}
	if err := c.handleProtoResponse(resp, config); err != nil {
		return nil, err
	}

	return config, nil
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

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	encodedNodeID := url.PathEscape(strconv.Itoa(nodeID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/configs/" + encodedConfigID + "/nodes/" + encodedNodeID

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancerConfigNode", Err: err}
	}

	defer drainClose(resp)

	var node NodeBalancerConfigNode
	if err := c.handleResponse(resp, &node); err != nil {
		return nil, err
	}

	return &node, nil
}

// httpGetNodeBalancerConfigNodeProto retrieves one NodeBalancer config node as a
// proto message.
func (c *Client) httpGetNodeBalancerConfigNodeProto(ctx context.Context, nodeBalancerID, configID, nodeID int) (*linodev1.NodeBalancerConfigNode, error) {
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

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	encodedNodeID := url.PathEscape(strconv.Itoa(nodeID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/configs/" + encodedConfigID + "/nodes/" + encodedNodeID

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancerConfigNode", Err: err}
	}

	defer drainClose(resp)

	node := &linodev1.NodeBalancerConfigNode{}
	if err := c.handleProtoResponse(resp, node); err != nil {
		return nil, err
	}

	return node, nil
}

// DeleteNodeBalancerConfigNode deletes a node from a NodeBalancer config.
func (c *Client) httpDeleteNodeBalancerConfigNode(ctx context.Context, nodeBalancerID, configID, nodeID int) error {
	if nodeBalancerID <= 0 {
		return ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return ErrConfigIDPositive
	}

	if nodeID <= 0 {
		return ErrNodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	encodedNodeID := url.PathEscape(strconv.Itoa(nodeID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/configs/" + encodedConfigID + "/nodes/" + encodedNodeID

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteNodeBalancerConfigNode", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
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

// UpdateNodeBalancerConfig updates a config for a NodeBalancer by ID.
func (c *Client) httpUpdateNodeBalancerConfig(ctx context.Context, nodeBalancerID, configID int, req *UpdateNodeBalancerConfigRequest) (*NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if nodeBalancerID < 1 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	if req == nil {
		return nil, ErrUpdateConfigRequestRequired
	}

	endpoint := fmt.Sprintf(endpointNodeBalancerConfigs+"/%d", nodeBalancerID, configID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancerConfig", Err: err}
	}

	defer drainClose(resp)

	var config NodeBalancerConfig
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// UpdateNodeBalancerNode updates a node for a NodeBalancer config.
func (c *Client) httpUpdateNodeBalancerNode(ctx context.Context, nodeBalancerID, configID, nodeID int, req *UpdateNodeBalancerNodeRequest) (*NodeBalancerNode, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if nodeBalancerID < 1 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	if nodeID <= 0 {
		return nil, ErrNodeIDPositive
	}

	if req == nil {
		return nil, ErrUpdateNodeBalancerNodeRequestRequired
	}

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	encodedNodeID := url.PathEscape(strconv.Itoa(nodeID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/configs/" + encodedConfigID + "/nodes/" + encodedNodeID

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancerNode", Err: err}
	}

	defer drainClose(resp)

	var node NodeBalancerNode
	if err := c.handleResponse(resp, &node); err != nil {
		return nil, err
	}

	return &node, nil
}

// RebuildNodeBalancerConfig rebuilds a config for a NodeBalancer by ID.
func (c *Client) httpRebuildNodeBalancerConfig(ctx context.Context, nodeBalancerID, configID int) (*NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if nodeBalancerID < 1 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/configs/" + encodedConfigID + "/rebuild"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "RebuildNodeBalancerConfig", Err: err}
	}

	defer drainClose(resp)

	var config NodeBalancerConfig
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// httpCreateNodeBalancerConfigProto creates a NodeBalancer config and decodes the
// response into the proto element so the write tool emits the same field set as
// the config GET/LIST path.
func (c *Client) httpCreateNodeBalancerConfigProto(ctx context.Context, nodeBalancerID int, req *CreateNodeBalancerConfigRequest) (*linodev1.NodeBalancerConfig, error) {
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

	config := &linodev1.NodeBalancerConfig{}
	if err := c.handleProtoResponse(resp, config); err != nil {
		return nil, err
	}

	return config, nil
}

// httpUpdateNodeBalancerConfigProto updates a NodeBalancer config and decodes the
// response into the proto element.
func (c *Client) httpUpdateNodeBalancerConfigProto(ctx context.Context, nodeBalancerID, configID int, req *UpdateNodeBalancerConfigRequest) (*linodev1.NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if nodeBalancerID < 1 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	if req == nil {
		return nil, ErrUpdateConfigRequestRequired
	}

	endpoint := fmt.Sprintf(endpointNodeBalancerConfigs+"/%d", nodeBalancerID, configID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancerConfig", Err: err}
	}

	defer drainClose(resp)

	config := &linodev1.NodeBalancerConfig{}
	if err := c.handleProtoResponse(resp, config); err != nil {
		return nil, err
	}

	return config, nil
}

// httpRebuildNodeBalancerConfigProto rebuilds a NodeBalancer config and decodes
// the response into the proto element.
func (c *Client) httpRebuildNodeBalancerConfigProto(ctx context.Context, nodeBalancerID, configID int) (*linodev1.NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if nodeBalancerID < 1 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/configs/" + encodedConfigID + "/rebuild"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "RebuildNodeBalancerConfig", Err: err}
	}

	defer drainClose(resp)

	config := &linodev1.NodeBalancerConfig{}
	if err := c.handleProtoResponse(resp, config); err != nil {
		return nil, err
	}

	return config, nil
}

// httpCreateNodeBalancerNodeProto creates a NodeBalancer config node and decodes
// the response into the proto element so the write tool emits the same field set
// as the node GET/LIST path.
func (c *Client) httpCreateNodeBalancerNodeProto(ctx context.Context, nodeBalancerID, configID int, req *CreateNodeBalancerNodeRequest) (*linodev1.NodeBalancerConfigNode, error) {
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

	node := &linodev1.NodeBalancerConfigNode{}
	if err := c.handleProtoResponse(resp, node); err != nil {
		return nil, err
	}

	return node, nil
}

// httpUpdateNodeBalancerNodeProto updates a NodeBalancer config node and decodes
// the response into the proto element.
func (c *Client) httpUpdateNodeBalancerNodeProto(ctx context.Context, nodeBalancerID, configID, nodeID int, req *UpdateNodeBalancerNodeRequest) (*linodev1.NodeBalancerConfigNode, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if nodeBalancerID < 1 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	if nodeID <= 0 {
		return nil, ErrNodeIDPositive
	}

	if req == nil {
		return nil, ErrUpdateNodeBalancerNodeRequestRequired
	}

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	encodedNodeID := url.PathEscape(strconv.Itoa(nodeID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/configs/" + encodedConfigID + "/nodes/" + encodedNodeID

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancerNode", Err: err}
	}

	defer drainClose(resp)

	node := &linodev1.NodeBalancerConfigNode{}
	if err := c.handleProtoResponse(resp, node); err != nil {
		return nil, err
	}

	return node, nil
}

// httpGetNodeBalancerStatsProto retrieves statistics for a NodeBalancer by its
// ID as a proto message. The API nests the graphs under a top-level "data"
// object modeled by NodeBalancerStats.
func (c *Client) httpGetNodeBalancerStatsProto(ctx context.Context, nodeBalancerID int) (*linodev1.NodeBalancerStats, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedNodeBalancerID := url.PathEscape(strconv.Itoa(nodeBalancerID))
	endpoint := endpointNodeBalancers + "/" + encodedNodeBalancerID + "/stats"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancerStats", Err: err}
	}

	defer drainClose(resp)

	stats := &linodev1.NodeBalancerStats{}
	if err := c.handleProtoResponse(resp, stats); err != nil {
		return nil, err
	}

	return stats, nil
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

// httpGetNodeBalancerProto retrieves a NodeBalancer as a proto message.
func (c *Client) httpGetNodeBalancerProto(ctx context.Context, nodeBalancerID int) (*linodev1.NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointNodeBalancers+"/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	nodeBalancer := &linodev1.NodeBalancer{}
	if err := c.handleProtoResponse(resp, nodeBalancer); err != nil {
		return nil, err
	}

	return nodeBalancer, nil
}

// httpCreateNodeBalancerProto creates a NodeBalancer as a proto message.
func (c *Client) httpCreateNodeBalancerProto(ctx context.Context, req CreateNodeBalancerRequest) (*linodev1.NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointNodeBalancers, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	nodeBalancer := &linodev1.NodeBalancer{}
	if err := c.handleProtoResponse(resp, nodeBalancer); err != nil {
		return nil, err
	}

	return nodeBalancer, nil
}

// httpUpdateNodeBalancerProto updates a NodeBalancer as a proto message.
func (c *Client) httpUpdateNodeBalancerProto(ctx context.Context, nodeBalancerID int, req UpdateNodeBalancerRequest) (*linodev1.NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointNodeBalancers+"/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancer", Err: err}
	}

	defer drainClose(resp)

	nodeBalancer := &linodev1.NodeBalancer{}
	if err := c.handleProtoResponse(resp, nodeBalancer); err != nil {
		return nil, err
	}

	return nodeBalancer, nil
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
