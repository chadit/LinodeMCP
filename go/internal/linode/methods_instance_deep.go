package linode

import (
	"context"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// GetInstanceBackup retrieves a specific backup for a Linode instance.
func (c *Client) httpGetInstanceBackup(ctx context.Context, linodeID, backupID int) (*InstanceBackup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_backup_get", nil, linodeID, backupID)
	if err != nil {
		return nil, wrapRequestError("GetInstanceBackup", err)
	}

	defer drainClose(resp)

	var backup InstanceBackup
	if err := c.handleResponse(resp, &backup); err != nil {
		return nil, err
	}

	return &backup, nil
}

// GetInstanceConfigInterface retrieves a specific network interface from a configuration profile.
func (c *Client) httpGetInstanceConfigInterface(ctx context.Context, linodeID, configID, interfaceID int) (*ConfigInterfaceResponse, error) {
	if err := validateInstanceConfigInterfaceIDs(linodeID, configID, interfaceID); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_config_interface_get", nil, linodeID, configID, interfaceID)
	if err != nil {
		return nil, wrapRequestError("GetInstanceConfigInterface", err)
	}

	defer drainClose(resp)

	var configInterface ConfigInterfaceResponse
	if err := c.handleResponse(resp, &configInterface); err != nil {
		return nil, err
	}

	return &configInterface, nil
}

// GetInstanceInterfaceSettings retrieves interface settings for a Linode instance.
func (c *Client) httpGetInstanceInterfaceSettings(ctx context.Context, linodeID int) (*InstanceInterfaceSettings, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_interface_settings_get", nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("GetInstanceInterfaceSettings", err)
	}

	defer drainClose(resp)

	var settings InstanceInterfaceSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func validateInstanceConfigInterfaceIDs(linodeID, configID, interfaceID int) error {
	if linodeID <= 0 {
		return ErrLinodeIDPositive
	}

	if configID <= 0 {
		return ErrConfigIDPositive
	}

	if interfaceID <= 0 {
		return ErrInterfaceIDPositive
	}

	return nil
}

// ListInstanceDisks retrieves all disks for a Linode instance.
func (c *Client) httpListInstanceDisks(ctx context.Context, linodeID int) ([]InstanceDisk, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_disk_list", nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("ListInstanceDisks", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[InstanceDisk]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListInstanceConfigs retrieves all configuration profiles for a Linode instance.
func (c *Client) httpListInstanceConfigs(ctx context.Context, linodeID, page, pageSize int) ([]InstanceConfig, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_instance_config_list", pageQuery(page, pageSize), nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("ListInstanceConfigs", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[InstanceConfig]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListInstanceVolumes retrieves all volumes attached to a Linode instance.
func (c *Client) httpListInstanceVolumes(ctx context.Context, linodeID, page, pageSize int) ([]Volume, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_instance_volume_list", pageQuery(page, pageSize), nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("ListInstanceVolumes", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[Volume]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetInstanceInterface retrieves a specific interface for a Linode instance.
func (c *Client) httpGetInstanceInterface(ctx context.Context, linodeID, interfaceID int) (*InstanceInterface, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if interfaceID <= 0 {
		return nil, ErrInterfaceIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_interface_get", nil, linodeID, interfaceID)
	if err != nil {
		return nil, wrapRequestError("GetInstanceInterface", err)
	}

	defer drainClose(resp)

	var instanceInterface InstanceInterface
	if err := c.handleResponse(resp, &instanceInterface); err != nil {
		return nil, err
	}

	return &instanceInterface, nil
}

// GetInstanceConfig retrieves a specific configuration profile for a Linode instance.
func (c *Client) httpGetInstanceConfig(ctx context.Context, linodeID, configID int) (*InstanceConfig, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_config_get", nil, linodeID, configID)
	if err != nil {
		return nil, wrapRequestError("GetInstanceConfig", err)
	}

	defer drainClose(resp)

	var config InstanceConfig
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ListInstanceFirewalls retrieves all Cloud Firewalls assigned to a Linode instance.
func (c *Client) httpListInstanceFirewalls(ctx context.Context, linodeID, page, pageSize int) ([]Firewall, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_instance_firewall_list", pageQuery(page, pageSize), nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("ListInstanceFirewalls", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[Firewall]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListInstanceFirewallsProto retrieves a Linode instance's assigned Cloud
// Firewalls as proto messages.
func (c *Client) httpListInstanceFirewallsProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.Firewall, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	return listProtoElementsPaginatedRouted(ctx, c, "ListInstanceFirewalls",
		"linode_instance_firewall_list", "", []any{linodeID}, page, pageSize,
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
}

// GetInstanceDisk retrieves a specific disk for a Linode instance.
func (c *Client) httpGetInstanceDisk(ctx context.Context, linodeID, diskID int) (*InstanceDisk, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_disk_get", nil, linodeID, diskID)
	if err != nil {
		return nil, wrapRequestError("GetInstanceDisk", err)
	}

	defer drainClose(resp)

	var disk InstanceDisk
	if err := c.handleResponse(resp, &disk); err != nil {
		return nil, err
	}

	return &disk, nil
}

// ListInstanceIPs retrieves all IP addresses for a Linode instance.
func (c *Client) httpListInstanceIPs(ctx context.Context, linodeID int) (*InstanceIPAddresses, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_ip_list", nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("ListInstanceIPs", err)
	}

	defer drainClose(resp)

	var ips InstanceIPAddresses
	if err := c.handleResponse(resp, &ips); err != nil {
		return nil, err
	}

	return &ips, nil
}

// GetInstanceIP retrieves a specific IP address for a Linode instance.
func (c *Client) httpGetInstanceIP(ctx context.Context, linodeID int, address string) (*IPAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_ip_get", nil, linodeID, address)
	if err != nil {
		return nil, wrapRequestError("GetInstanceIP", err)
	}

	defer drainClose(resp)

	var ip IPAddress
	if err := c.handleResponse(resp, &ip); err != nil {
		return nil, err
	}

	return &ip, nil
}
