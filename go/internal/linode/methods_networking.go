package linode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"net/url"
	"slices"

	"google.golang.org/protobuf/encoding/protojson"

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

// httpListReservedIPsProto retrieves reserved public IPv4 addresses, keeping the
// raw API objects so documented explicit nulls survive.
func (c *Client) httpListReservedIPsProto(ctx context.Context, page, pageSize int) (*ReservedIPListPage, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_networking_reserved_ip_list", pageQuery(page, pageSize), nil)
	if err != nil {
		return nil, wrapRequestError("ListReservedIPs", err)
	}

	defer drainClose(resp)

	var body json.RawMessage
	if decodeErr := c.handleResponse(resp, &body); decodeErr != nil {
		return nil, decodeErr
	}

	data, err := reservedIPListData(body)
	if err != nil {
		return nil, err
	}

	reservedIPs, err := decodeRawProtoItems(data, "ListReservedIPs",
		func() *linodev1.ReservedIPAddress { return &linodev1.ReservedIPAddress{} })
	if err != nil {
		return nil, err
	}

	return &ReservedIPListPage{ReservedIPs: reservedIPs, RawReservedIPs: data}, nil
}

// reservedIPListData pulls the data[] page out of a list body, rejecting a
// non-object body first so the failure names the shape, not a Go type.
func reservedIPListData(body json.RawMessage) ([]json.RawMessage, error) {
	if !IsObjectBody(body) {
		return nil, ErrReservedIPListNotObject
	}

	var envelope struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return envelope.Data, nil
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

// httpCreateReservedIPRaw reserves a public IPv4 address in one region,
// returning the raw API object so explicit nulls survive as they do on the
// list and get paths.
func (c *Client) httpCreateReservedIPRaw(ctx context.Context, region string, tags []string) (json.RawMessage, error) {
	if region == "" {
		return nil, ErrRegionRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	body := map[string]any{"region": region}
	// Absent and empty are different to the API here: omitting tags leaves the
	// reservation untagged, so only a supplied list is sent.
	if tags != nil {
		body["tags"] = tags
	}

	resp, err := c.makeRouteRequest(ctx, "linode_networking_reserved_ip_create", body)
	if err != nil {
		return nil, wrapRequestError("CreateReservedIP", err)
	}

	defer drainClose(resp)

	var reservedIP json.RawMessage
	if err := c.handleResponse(resp, &reservedIP); err != nil {
		return nil, err
	}

	return reservedIP, nil
}

// httpUpdateReservedIPRaw replaces one reserved address's tags, returning the
// raw API object for the same null-preserving reason as the create path.
func (c *Client) httpUpdateReservedIPRaw(ctx context.Context, address string, tags []string) (json.RawMessage, error) {
	addr, err := netip.ParseAddr(address)
	if err != nil || !addr.Is4() {
		return nil, ErrIPv4AddressInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	// The update replaces the whole tag set, so an empty list is a meaningful
	// request (clear the tags) rather than an omission.
	if tags == nil {
		tags = []string{}
	}

	resp, err := c.makeRouteRequest(ctx, "linode_networking_reserved_ip_update", map[string]any{"tags": tags}, address)
	if err != nil {
		return nil, wrapRequestError("UpdateReservedIP", err)
	}

	defer drainClose(resp)

	var reservedIP json.RawMessage
	if err := c.handleResponse(resp, &reservedIP); err != nil {
		return nil, err
	}

	return reservedIP, nil
}

// httpListReservedIPTypesProto retrieves reserved IPv4 pricing types. Unlike the
// address routes it needs no raw copy: the nullable price fields are optional in
// the proto, so protojson keeps them.
func (c *Client) httpListReservedIPTypesProto(ctx context.Context) ([]*linodev1.ReservedIPType, error) {
	return listProtoElementsRouted(ctx, c, "ListReservedIPTypes",
		"linode_networking_reserved_ip_type_list", "", nil,
		func() *linodev1.ReservedIPType { return &linodev1.ReservedIPType{} })
}

// httpDeleteReservedIP permanently unreserves one public IPv4 address.
func (c *Client) httpDeleteReservedIP(ctx context.Context, address string) error {
	addr, err := netip.ParseAddr(address)
	if err != nil || !addr.Is4() {
		return ErrIPv4AddressInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_networking_reserved_ip_delete", nil, address)
	if err != nil {
		return wrapRequestError("DeleteReservedIP", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

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

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_delete", nil, nodeBalancerID, configID)
	if err != nil {
		return wrapRequestError("DeleteNodeBalancerConfig", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpListFirewallsProto retrieves one page of Cloud Firewalls as proto messages.
func (c *Client) httpListFirewallsProto(ctx context.Context, page, pageSize int) ([]*linodev1.Firewall, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListFirewalls",
		"linode_firewall_list", "", nil, page, pageSize,
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
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

// httpListVLANsProto retrieves one page of VLANs as proto messages.
func (c *Client) httpListVLANsProto(ctx context.Context, page, pageSize int) ([]*linodev1.VLAN, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListVLANs",
		"linode_vlan_list", "", nil, page, pageSize,
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

	resp, err := c.makeRouteRequest(ctx, "linode_vlan_delete", nil, regionID, label)
	if err != nil {
		return wrapRequestError("DeleteVLAN", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpListNodeBalancerVPCsProto retrieves a NodeBalancer's VPC configurations as
// proto messages.
func (c *Client) httpListNodeBalancerVPCsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.NodeBalancerVPCConfig, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	return listProtoElementsPaginatedRouted(ctx, c, "ListNodeBalancerVPCs",
		"linode_nodebalancer_vpc_config_list", "", []any{nodeBalancerID}, page, pageSize,
		func() *linodev1.NodeBalancerVPCConfig { return &linodev1.NodeBalancerVPCConfig{} })
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

// httpListFirewallRulesProto retrieves a firewall's ruleset as a proto message.
func (c *Client) httpListFirewallRulesProto(ctx context.Context, firewallID int) (*linodev1.FirewallRules, error) {
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

	rules := &linodev1.FirewallRules{}
	if err := c.handleProtoResponse(resp, rules); err != nil {
		return nil, err
	}

	return rules, nil
}

// httpListFirewallRuleVersionsProto retrieves a Cloud Firewall's rule-version
// history. The history route answers with one firewall-shaped object, not a
// {data:[...]} page, so the decode reads that object as the single snapshot.
// Its top-level version is lifted out of rules.version because the proto
// FirewallRules message has no such field; the Python handler lifts the same way.
func (c *Client) httpListFirewallRuleVersionsProto(ctx context.Context, firewallID int) ([]*linodev1.FirewallRuleVersion, error) {
	const operation = "ListFirewallRuleVersions"

	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_rule_version_list", nil, firewallID)
	if err != nil {
		return nil, wrapRequestError(operation, err)
	}

	defer drainClose(resp)

	var body json.RawMessage
	if err := c.handleResponse(resp, &body); err != nil {
		return nil, err
	}

	snapshot := &linodev1.FirewallRuleVersion{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s object: %w", operation, err)
	}

	// A firewall id is always positive, so a zero id means the body was some other
	// shape that DiscardUnknown would otherwise swallow into an empty snapshot.
	if snapshot.GetId() == 0 {
		return nil, fmt.Errorf("failed to unmarshal %s object: %w", operation, ErrFirewallHistoryNotObject)
	}

	var probe struct {
		Rules struct {
			Version int32 `json:"version"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s object: %w", operation, err)
	}

	snapshot.Version = probe.Rules.Version

	return []*linodev1.FirewallRuleVersion{snapshot}, nil
}

// httpGetFirewallRuleVersionProto retrieves one rule-version snapshot. The route
// answers with a bare firewall-shaped object, not a {data:[...]} page, so it
// decodes straight into the element the LIST path emits.
func (c *Client) httpGetFirewallRuleVersionProto(ctx context.Context, firewallID, version int) (*linodev1.FirewallRuleVersion, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	if version <= 0 {
		return nil, ErrFirewallRuleVersionPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_rule_version_get", nil, firewallID, version)
	if err != nil {
		return nil, wrapRequestError("GetFirewallRuleVersion", err)
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

// httpListFirewallDevicesProto retrieves a Cloud Firewall's assigned devices as
// proto messages.
func (c *Client) httpListFirewallDevicesProto(ctx context.Context, firewallID, page, pageSize int) ([]*linodev1.FirewallDevice, error) {
	if firewallID <= 0 {
		return nil, ErrFirewallIDPositive
	}

	return listProtoElementsPaginatedRouted(ctx, c, "ListFirewallDevices",
		"linode_firewall_device_list", "", []any{firewallID}, page, pageSize,
		func() *linodev1.FirewallDevice { return &linodev1.FirewallDevice{} })
}

// httpCreateFirewallDeviceProto assigns a device to a Cloud Firewall.
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

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_device_create", req, firewallID)
	if err != nil {
		return nil, wrapRequestError("CreateFirewallDevice", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_device_get", nil, firewallID, deviceID)
	if err != nil {
		return nil, wrapRequestError("GetFirewallDevice", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_device_delete", nil, firewallID, deviceID)
	if err != nil {
		return wrapRequestError("DeleteFirewallDevice", err)
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

// httpListFirewallSettingsProto retrieves default firewall assignments. The
// route answers with a single object, not a page.
func (c *Client) httpListFirewallSettingsProto(ctx context.Context, page, pageSize int) (*linodev1.FirewallSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_firewall_settings_get", pageQuery(page, pageSize), nil)
	if err != nil {
		return nil, wrapRequestError("ListFirewallSettings", err)
	}

	defer drainClose(resp)

	settings := &linodev1.FirewallSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// httpUpdateFirewallSettingsProto updates default firewall assignments.
func (c *Client) httpUpdateFirewallSettingsProto(ctx context.Context, req *UpdateFirewallSettingsRequest) (*linodev1.FirewallSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_settings_update", req)
	if err != nil {
		return nil, wrapRequestError("UpdateFirewallSettings", err)
	}

	defer drainClose(resp)

	settings := &linodev1.FirewallSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// httpListFirewallTemplatesProto retrieves reusable Cloud Firewall templates.
func (c *Client) httpListFirewallTemplatesProto(ctx context.Context, page, pageSize int) ([]*linodev1.FirewallTemplate, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListFirewallTemplates",
		"linode_firewall_template_list", "", nil, page, pageSize,
		func() *linodev1.FirewallTemplate { return &linodev1.FirewallTemplate{} })
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

// httpGetFirewallTemplateProto retrieves a Cloud Firewall template by slug. The
// by-slug route answers with a single bare template object, so it decodes
// directly into the element the LIST path emits.
func (c *Client) httpGetFirewallTemplateProto(ctx context.Context, slug string, page, pageSize int) (*linodev1.FirewallTemplate, error) {
	if !isFirewallTemplateSlug(slug) {
		return nil, ErrInvalidFirewallTemplateSlug
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_firewall_template_get", pageQuery(page, pageSize), nil, slug)
	if err != nil {
		return nil, wrapRequestError("GetFirewallTemplate", err)
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

// httpGetFirewallProto retrieves one Cloud Firewall as a proto message.
func (c *Client) httpGetFirewallProto(ctx context.Context, firewallID int) (*linodev1.Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_get", nil, firewallID)
	if err != nil {
		return nil, wrapRequestError("GetFirewall", err)
	}

	defer drainClose(resp)

	firewall := &linodev1.Firewall{}
	if err := c.handleProtoResponse(resp, firewall); err != nil {
		return nil, err
	}

	return firewall, nil
}

// DeleteFirewall deletes a Cloud Firewall.
func (c *Client) httpDeleteFirewall(ctx context.Context, firewallID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_delete", nil, firewallID)
	if err != nil {
		return wrapRequestError("DeleteFirewall", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateFirewall", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_update", req, firewallID)
	if err != nil {
		return nil, wrapRequestError("UpdateFirewall", err)
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

	body := firewallRulesRawReplaceBody{
		InboundPolicy:  req.InboundPolicy,
		OutboundPolicy: req.OutboundPolicy,
		Inbound:        req.Inbound,
		Outbound:       req.Outbound,
	}

	resp, err := c.makeRouteRequest(ctx, "linode_firewall_rules_update", body, firewallID)
	if err != nil {
		return nil, wrapRequestError("UpdateFirewallRules", err)
	}

	defer drainClose(resp)

	rules := &linodev1.FirewallRules{}
	if err := c.handleProtoResponse(resp, rules); err != nil {
		return nil, err
	}

	return rules, nil
}

// httpListNetworkingIPsProto retrieves account IP addresses as proto messages
// for the proto-backed list path. skip_ipv6_rdns flows through the same query
// param as httpListNetworkingIPs, so the request matches.
func (c *Client) httpListNetworkingIPsProto(ctx context.Context, skipIPv6RDNS bool) ([]*linodev1.IPAddress, error) {
	var rawQuery string

	if skipIPv6RDNS {
		query := url.Values{}
		query.Set("skip_ipv6_rdns", "true")
		rawQuery = query.Encode()
	}

	return listProtoElementsRouted(ctx, c, "ListNetworkingIPs", "linode_networking_ip_list", rawQuery, nil,
		func() *linodev1.IPAddress { return &linodev1.IPAddress{} })
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

// httpGetNetworkingIPProto retrieves a networking IP as a proto message.
func (c *Client) httpGetNetworkingIPProto(ctx context.Context, address string) (*linodev1.IPAddress, error) {
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

	ip := &linodev1.IPAddress{}
	if err := c.handleProtoResponse(resp, ip); err != nil {
		return nil, err
	}

	return ip, nil
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

	resp, err := c.makeRouteRequest(ctx, "linode_networking_ip_update", req, address)
	if err != nil {
		return nil, wrapRequestError("UpdateNetworkingIP", err)
	}

	defer drainClose(resp)

	ip := &linodev1.IPAddress{}
	if err := c.handleProtoResponse(resp, ip); err != nil {
		return nil, err
	}

	return ip, nil
}

// httpAllocateNetworkingIPProto allocates an account-level IP address and
// decodes the response into the proto IPAddress element.
func (c *Client) httpAllocateNetworkingIPProto(ctx context.Context, req AllocateNetworkingIPRequest) (*linodev1.IPAddress, error) {
	if req.LinodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_networking_ip_allocate", req)
	if err != nil {
		return nil, wrapRequestError("AllocateNetworkingIP", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_networking_ip_assign", req)
	if err != nil {
		return nil, wrapRequestError("AssignNetworkingIPs", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_networking_ipv4_assign", req)
	if err != nil {
		return nil, wrapRequestError("AssignNetworkingIPv4s", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_networking_ipv4_share", req)
	if err != nil {
		return nil, wrapRequestError("ShareNetworkingIPv4s", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_networking_ip_share", req)
	if err != nil {
		return nil, wrapRequestError("ShareNetworkingIPs", err)
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
	return listProtoElementsRouted(ctx, c, "ListNetworkTransferPrices",
		"linode_network_transfer_price_list", "", nil,
		func() *linodev1.LinodeType { return &linodev1.LinodeType{} })
}

// httpListIPv6PoolsProto retrieves IPv6 pools as proto IPv6Pool messages for the
// proto-backed list path. page/page_size flow through withPaginationQuery, so the
// request matches httpListIPv6Pools.
func (c *Client) httpListIPv6PoolsProto(ctx context.Context, page, pageSize int) ([]*linodev1.IPv6Pool, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListIPv6Pools",
		"linode_ipv6_pool_list", "", nil, page, pageSize,
		func() *linodev1.IPv6Pool { return &linodev1.IPv6Pool{} })
}

// httpListIPv6RangesProto retrieves IPv6 ranges as proto IPv6Range messages for
// the proto-backed list path. page/page_size flow through withPaginationQuery, so
// the request matches httpListIPv6Ranges.
func (c *Client) httpListIPv6RangesProto(ctx context.Context, page, pageSize int) ([]*linodev1.IPv6Range, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListIPv6Ranges",
		"linode_ipv6_range_list", "", nil, page, pageSize,
		func() *linodev1.IPv6Range { return &linodev1.IPv6Range{} })
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

	resp, err := c.makeRouteRequest(ctx, "linode_ipv6_range_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateIPv6Range", err)
	}

	defer drainClose(resp)

	ipv6Range := &linodev1.IPv6Range{}
	if err := c.handleProtoResponse(resp, ipv6Range); err != nil {
		return nil, err
	}

	return ipv6Range, nil
}

// httpListNodeBalancerTypesProto retrieves available NodeBalancer types as proto
// messages, decoded directly from the API JSON for the proto-backed list path.
func (c *Client) httpListNodeBalancerTypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	return listProtoElementsRouted(ctx, c, "ListNodeBalancerTypes",
		"linode_nodebalancer_type_list", "", nil,
		func() *linodev1.LinodeType { return &linodev1.LinodeType{} })
}

// httpListNodeBalancersProto retrieves one page of NodeBalancers as proto
// messages, decoded directly from the API JSON for the proto-backed read path.
func (c *Client) httpListNodeBalancersProto(ctx context.Context, page, pageSize int) ([]*linodev1.NodeBalancer, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListNodeBalancers",
		"linode_nodebalancer_list", "", nil, page, pageSize,
		func() *linodev1.NodeBalancer { return &linodev1.NodeBalancer{} })
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

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_vpc_config_get", nil, nodeBalancerID, vpcConfigID)
	if err != nil {
		return nil, wrapRequestError("GetNodeBalancerVPCConfig", err)
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

// httpListNodeBalancerConfigsProto retrieves a NodeBalancer's configs as proto
// messages for the proto-backed list path. It names the same tool as
// httpListNodeBalancerConfigs, so both resolve one declared route, then the
// paginated twin adds page/page_size and the runtime request matches exactly.
func (c *Client) httpListNodeBalancerConfigsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.NodeBalancerConfig, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListNodeBalancerConfigs",
		"linode_nodebalancer_config_list", "", []any{nodeBalancerID}, page, pageSize,
		func() *linodev1.NodeBalancerConfig { return &linodev1.NodeBalancerConfig{} })
}

// ListNodeBalancerFirewalls retrieves Cloud Firewalls assigned to a NodeBalancer.
func (c *Client) httpListNodeBalancerFirewalls(ctx context.Context, nodeBalancerID, page, pageSize int) ([]Firewall, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_nodebalancer_firewall_list", pageQuery(page, pageSize), nil, nodeBalancerID)
	if err != nil {
		return nil, wrapRequestError("ListNodeBalancerFirewalls", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[Firewall]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListNodeBalancerFirewallsProto retrieves the Cloud Firewalls assigned to a
// NodeBalancer as proto messages for the proto-backed list path. It names the
// same tool as httpListNodeBalancerFirewalls, so both resolve one declared
// route, then the paginated twin adds page/page_size and the runtime request
// matches exactly.
func (c *Client) httpListNodeBalancerFirewallsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.Firewall, error) {
	if nodeBalancerID <= 0 {
		return nil, ErrNodeBalancerIDPositive
	}

	return listProtoElementsPaginatedRouted(ctx, c, "ListNodeBalancerFirewalls",
		"linode_nodebalancer_firewall_list", "", []any{nodeBalancerID}, page, pageSize,
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
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

	resp, err := c.makeRouteRequestQuery(ctx, "linode_nodebalancer_firewall_update", pageQuery(page, pageSize), req, nodeBalancerID)
	if err != nil {
		return nil, wrapRequestError("UpdateNodeBalancerFirewalls", err)
	}

	defer drainClose(resp)

	return decodeProtoElements(resp, c, "UpdateNodeBalancerFirewalls",
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

	return listProtoElementsPaginatedRouted(ctx, c, "ListNodeBalancerConfigNodes",
		"linode_nodebalancer_config_node_list", "", []any{nodeBalancerID, configID}, page, pageSize,
		func() *linodev1.NodeBalancerConfigNode { return &linodev1.NodeBalancerConfigNode{} })
}

// httpGetNodeBalancerConfigProto retrieves one NodeBalancer config as a proto
// message.
func (c *Client) httpGetNodeBalancerConfigProto(ctx context.Context, nodeBalancerID, configID int) (*linodev1.NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_get", nil, nodeBalancerID, configID)
	if err != nil {
		return nil, wrapRequestError("GetNodeBalancerConfig", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_node_get", nil, nodeBalancerID, configID, nodeID)
	if err != nil {
		return nil, wrapRequestError("GetNodeBalancerConfigNode", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_node_delete", nil, nodeBalancerID, configID, nodeID)
	if err != nil {
		return wrapRequestError("DeleteNodeBalancerConfigNode", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
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

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_create", req, nodeBalancerID)
	if err != nil {
		return nil, wrapRequestError("CreateNodeBalancerConfig", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_update", req, nodeBalancerID, configID)
	if err != nil {
		return nil, wrapRequestError("UpdateNodeBalancerConfig", err)
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
func (c *Client) httpRebuildNodeBalancerConfigProto(ctx context.Context, nodeBalancerID, configID int, req *RebuildNodeBalancerConfigRequest) (*linodev1.NodeBalancerConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if nodeBalancerID < 1 {
		return nil, ErrNodeBalancerIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	if req == nil {
		return nil, ErrRebuildConfigRequestRequired
	}

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_rebuild", req, nodeBalancerID, configID)
	if err != nil {
		return nil, wrapRequestError("RebuildNodeBalancerConfig", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_node_create", req, nodeBalancerID, configID)
	if err != nil {
		return nil, wrapRequestError("CreateNodeBalancerNode", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_config_node_update", req, nodeBalancerID, configID, nodeID)
	if err != nil {
		return nil, wrapRequestError("UpdateNodeBalancerNode", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_stats_get", nil, nodeBalancerID)
	if err != nil {
		return nil, wrapRequestError("GetNodeBalancerStats", err)
	}

	defer drainClose(resp)

	stats := &linodev1.NodeBalancerStats{}
	if err := c.handleProtoResponse(resp, stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// httpGetNodeBalancerProto retrieves a NodeBalancer as a proto message.
func (c *Client) httpGetNodeBalancerProto(ctx context.Context, nodeBalancerID int) (*linodev1.NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_get", nil, nodeBalancerID)
	if err != nil {
		return nil, wrapRequestError("GetNodeBalancer", err)
	}

	defer drainClose(resp)

	nodeBalancer := &linodev1.NodeBalancer{}
	if err := c.handleProtoResponse(resp, nodeBalancer); err != nil {
		return nil, err
	}

	return nodeBalancer, nil
}

// httpCreateNodeBalancerProto creates a NodeBalancer as a proto message.
func (c *Client) httpCreateNodeBalancerProto(ctx context.Context, req *CreateNodeBalancerRequest) (*linodev1.NodeBalancer, error) {
	if err := validateCreateNodeBalancerRequest(req); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateNodeBalancer", err)
	}

	defer drainClose(resp)

	nodeBalancer := &linodev1.NodeBalancer{}
	if err := c.handleProtoResponse(resp, nodeBalancer); err != nil {
		return nil, err
	}

	return nodeBalancer, nil
}

func validateCreateNodeBalancerRequest(req *CreateNodeBalancerRequest) error {
	if req.IPv4 == nil {
		return nil
	}

	address, err := netip.ParseAddr(*req.IPv4)
	if err != nil || !address.Is4() {
		return ErrIPv4AddressInvalid
	}

	return nil
}

// httpUpdateNodeBalancerProto updates a NodeBalancer as a proto message.
func (c *Client) httpUpdateNodeBalancerProto(ctx context.Context, nodeBalancerID int, req UpdateNodeBalancerRequest) (*linodev1.NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_update", req, nodeBalancerID)
	if err != nil {
		return nil, wrapRequestError("UpdateNodeBalancer", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_nodebalancer_delete", nil, nodeBalancerID)
	if err != nil {
		return wrapRequestError("DeleteNodeBalancer", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}
