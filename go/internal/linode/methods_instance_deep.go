package linode

import (
	"context"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// httpListInstanceBackupsProto retrieves all backups for a Linode instance. The
// /backups endpoint returns a nested object (automatic[] plus a snapshot
// object), not a page envelope.
func (c *Client) httpListInstanceBackupsProto(ctx context.Context, linodeID int) (*linodev1.InstanceBackupsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_backup_list", nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("ListInstanceBackups", err)
	}

	defer drainClose(resp)

	backups := &linodev1.InstanceBackupsResponse{}
	if err := c.handleProtoResponse(resp, backups); err != nil {
		return nil, err
	}

	return backups, nil
}

// httpGetInstanceStatsProto retrieves daily statistics for a Linode instance.
// The API nests the graphs under a top-level "data" object, which InstanceStats
// models (see instance_stats.proto).
func (c *Client) httpGetInstanceStatsProto(ctx context.Context, linodeID int) (*linodev1.InstanceStats, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_stats_get", nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("GetInstanceStats", err)
	}

	defer drainClose(resp)

	stats := &linodev1.InstanceStats{}
	if err := c.handleProtoResponse(resp, stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// httpGetInstanceTransferByYearMonthProto retrieves one past month's network
// transfer totals. This endpoint returns bytes_in/bytes_out/bytes_total, a
// different shape from the current month's billable/quota/used.
func (c *Client) httpGetInstanceTransferByYearMonthProto(ctx context.Context, linodeID, year, month int) (*linodev1.InstanceTransferMonth, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if year <= 0 {
		return nil, ErrTransferYearPositive
	}

	if month < 1 || month > 12 {
		return nil, ErrTransferMonthRange
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_transfer_month_get", nil, linodeID, year, month)
	if err != nil {
		return nil, wrapRequestError("GetInstanceTransferByYearMonth", err)
	}

	defer drainClose(resp)

	transfer := &linodev1.InstanceTransferMonth{}
	if err := c.handleProtoResponse(resp, transfer); err != nil {
		return nil, err
	}

	return transfer, nil
}

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

// httpGetInstanceBackupProto retrieves one instance backup as a proto message.
func (c *Client) httpGetInstanceBackupProto(ctx context.Context, linodeID, backupID int) (*linodev1.InstanceBackup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_backup_get", nil, linodeID, backupID)
	if err != nil {
		return nil, wrapRequestError("GetInstanceBackup", err)
	}

	defer drainClose(resp)

	backup := &linodev1.InstanceBackup{}
	if err := c.handleProtoResponse(resp, backup); err != nil {
		return nil, err
	}

	return backup, nil
}

// RestoreInstanceBackup restores a backup to a Linode instance.
func (c *Client) httpRestoreInstanceBackup(ctx context.Context, linodeID, backupID int, req RestoreBackupRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_backup_restore", req, linodeID, backupID)
	if err != nil {
		return wrapRequestError("RestoreInstanceBackup", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// EnableInstanceBackups enables the backup service for a Linode instance.
func (c *Client) httpEnableInstanceBackups(ctx context.Context, linodeID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_backups_enable", nil, linodeID)
	if err != nil {
		return wrapRequestError("EnableInstanceBackups", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// CancelInstanceBackups cancels the backup service for a Linode instance.
func (c *Client) httpCancelInstanceBackups(ctx context.Context, linodeID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_backups_cancel", nil, linodeID)
	if err != nil {
		return wrapRequestError("CancelInstanceBackups", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ApplyInstanceFirewalls reapplies assigned firewalls to a Linode instance.
func (c *Client) httpApplyInstanceFirewalls(ctx context.Context, linodeID int) error {
	if linodeID <= 0 {
		return ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_firewall_apply", nil, linodeID)
	if err != nil {
		return wrapRequestError("ApplyInstanceFirewalls", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpAddInstanceInterfaceProto appends an interface to a Linode instance.
func (c *Client) httpAddInstanceInterfaceProto(ctx context.Context, linodeID int, req *AddInstanceInterfaceRequest) (*linodev1.InstanceInterface, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req == nil {
		return nil, ErrAddInstanceInterfaceRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_interface_add", req, linodeID)
	if err != nil {
		return nil, wrapRequestError("AddInstanceInterface", err)
	}

	defer drainClose(resp)

	instanceInterface := &linodev1.InstanceInterface{}
	if err := c.handleProtoResponse(resp, instanceInterface); err != nil {
		return nil, err
	}

	return instanceInterface, nil
}

// httpUpdateInstanceInterfaceProto updates an interface on a Linode instance.
func (c *Client) httpUpdateInstanceInterfaceProto(ctx context.Context, linodeID, interfaceID int, req *UpdateInstanceInterfaceRequest) (*linodev1.InstanceInterface, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if interfaceID <= 0 {
		return nil, ErrInterfaceIDPositive
	}

	if req == nil {
		return nil, ErrUpdateInstanceInterfaceRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_interface_update", req, linodeID, interfaceID)
	if err != nil {
		return nil, wrapRequestError("UpdateInstanceInterface", err)
	}

	defer drainClose(resp)

	instanceInterface := &linodev1.InstanceInterface{}
	if err := c.handleProtoResponse(resp, instanceInterface); err != nil {
		return nil, err
	}

	return instanceInterface, nil
}

// DeleteInstanceInterface deletes an interface from a Linode instance.
func (c *Client) httpDeleteInstanceInterface(ctx context.Context, linodeID, interfaceID int) error {
	if linodeID <= 0 {
		return ErrLinodeIDPositive
	}

	if interfaceID <= 0 {
		return ErrInterfaceIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_interface_delete", nil, linodeID, interfaceID)
	if err != nil {
		return wrapRequestError("DeleteInstanceInterface", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
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

// httpGetInstanceConfigInterfaceProto retrieves one config interface as a proto
// message.
func (c *Client) httpGetInstanceConfigInterfaceProto(ctx context.Context, linodeID, configID, interfaceID int) (*linodev1.ConfigInterfaceResponse, error) {
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

	configInterface := &linodev1.ConfigInterfaceResponse{}
	if err := c.handleProtoResponse(resp, configInterface); err != nil {
		return nil, err
	}

	return configInterface, nil
}

// DeleteInstanceConfigInterface removes a network interface from a configuration profile.
func (c *Client) httpDeleteInstanceConfigInterface(ctx context.Context, linodeID, configID, interfaceID int) error {
	if err := validateInstanceConfigInterfaceIDs(linodeID, configID, interfaceID); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_config_interface_delete", nil, linodeID, configID, interfaceID)
	if err != nil {
		return wrapRequestError("DeleteInstanceConfigInterface", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
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

// httpGetInstanceInterfaceSettingsProto retrieves a Linode's interface settings
// as a proto message.
func (c *Client) httpGetInstanceInterfaceSettingsProto(ctx context.Context, linodeID int) (*linodev1.InstanceInterfaceSettings, error) {
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

	settings := &linodev1.InstanceInterfaceSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// httpUpdateInstanceInterfaceSettingsProto updates a Linode's interface settings.
func (c *Client) httpUpdateInstanceInterfaceSettingsProto(ctx context.Context, linodeID int, req *UpdateInstanceInterfaceSettingsRequest) (*linodev1.InstanceInterfaceSettings, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req == nil {
		return nil, ErrUpdateInterfaceSettingsRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_interface_settings_update", req, linodeID)
	if err != nil {
		return nil, wrapRequestError("UpdateInstanceInterfaceSettings", err)
	}

	defer drainClose(resp)

	settings := &linodev1.InstanceInterfaceSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
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

// httpListInstanceDisksProto retrieves a Linode instance's disks as proto
// messages.
func (c *Client) httpListInstanceDisksProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.InstanceDisk, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListInstanceDisks",
		"linode_instance_disk_list", "", []any{linodeID}, page, pageSize,
		func() *linodev1.InstanceDisk { return &linodev1.InstanceDisk{} })
}

func validateInstanceConfigMutation(linodeID, configID int, requestMissing bool, missingReqErr error) error {
	if linodeID <= 0 {
		return ErrLinodeIDPositive
	}

	if configID <= 0 {
		return ErrConfigIDPositive
	}

	if requestMissing {
		return missingReqErr
	}

	return nil
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

// httpListInstanceConfigsProto retrieves a Linode instance's configuration
// profiles as proto messages.
func (c *Client) httpListInstanceConfigsProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.InstanceConfig, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	return listProtoElementsPaginatedRouted(ctx, c, "ListInstanceConfigs",
		"linode_instance_config_list", "", []any{linodeID}, page, pageSize,
		func() *linodev1.InstanceConfig { return &linodev1.InstanceConfig{} })
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

// httpListInstanceVolumesProto retrieves a Linode instance's attached volumes as
// proto messages.
func (c *Client) httpListInstanceVolumesProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.Volume, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	return listProtoElementsPaginatedRouted(ctx, c, "ListInstanceVolumes",
		"linode_instance_volume_list", "", []any{linodeID}, page, pageSize,
		func() *linodev1.Volume { return &linodev1.Volume{} })
}

// httpUpdateInstanceFirewallsProto replaces firewall assignments for a Linode
// instance. The response is a page of firewalls, so the write tool emits the
// same shape as the instance firewall list path.
func (c *Client) httpUpdateInstanceFirewallsProto(ctx context.Context, linodeID, page, pageSize int, req *UpdateInstanceFirewallsRequest) ([]*linodev1.Firewall, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req == nil {
		return nil, ErrUpdateInstanceFirewallsRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_instance_firewall_update", pageQuery(page, pageSize), req, linodeID)
	if err != nil {
		return nil, wrapRequestError("UpdateInstanceFirewalls", err)
	}

	defer drainClose(resp)

	return decodeProtoElements(resp, c, "UpdateInstanceFirewalls",
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
}

// httpListInstanceInterfacesProto retrieves the current-generation interfaces for
// a Linode instance. The endpoint wraps elements under an "interfaces" key, not
// the usual "data" page envelope.
func (c *Client) httpListInstanceInterfacesProto(ctx context.Context, linodeID int) ([]*linodev1.InstanceInterface, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	return listProtoElementsKeyedRouted(ctx, c, "ListInstanceInterfaces",
		"linode_instance_interface_list", "", "interfaces", []any{linodeID},
		func() *linodev1.InstanceInterface { return &linodev1.InstanceInterface{} })
}

// httpUpgradeLinodeInterfacesProto upgrades a Linode's legacy config interfaces.
// The API returns config_id, dry_run and interfaces; the message field on the
// proto envelope is not in the API body, the handler fills it.
func (c *Client) httpUpgradeLinodeInterfacesProto(ctx context.Context, linodeID int, req *UpgradeLinodeInterfacesRequest) (*linodev1.InstanceInterfaceUpgradeWriteResponse, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_interface_upgrade", req, linodeID)
	if err != nil {
		return nil, wrapRequestError("UpgradeLinodeInterfaces", err)
	}

	defer drainClose(resp)

	result := &linodev1.InstanceInterfaceUpgradeWriteResponse{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
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

// httpGetInstanceInterfaceProto retrieves a specific interface for a Linode
// instance as a proto message.
func (c *Client) httpGetInstanceInterfaceProto(ctx context.Context, linodeID, interfaceID int) (*linodev1.InstanceInterface, error) {
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

	instanceInterface := &linodev1.InstanceInterface{}
	if err := c.handleProtoResponse(resp, instanceInterface); err != nil {
		return nil, err
	}

	return instanceInterface, nil
}

// httpListInstanceInterfaceFirewallsProto retrieves the Cloud Firewalls assigned
// to a Linode interface as proto messages.
func (c *Client) httpListInstanceInterfaceFirewallsProto(ctx context.Context, linodeID, interfaceID int) ([]*linodev1.Firewall, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if interfaceID <= 0 {
		return nil, ErrInterfaceIDPositive
	}

	return listProtoElementsRouted(ctx, c, "ListInstanceInterfaceFirewalls",
		"linode_instance_interface_firewall_list", "", []any{linodeID, interfaceID},
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
}

// httpListInstanceConfigInterfacesProto retrieves the legacy config-profile
// network interfaces of one configuration profile. This endpoint returns a bare
// top-level array, not a page envelope.
func (c *Client) httpListInstanceConfigInterfacesProto(ctx context.Context, linodeID, configID int) ([]*linodev1.ConfigInterfaceResponse, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	return listProtoElementsBareRouted(ctx, c, "ListInstanceConfigInterfaces",
		"linode_instance_config_interface_list", "", []any{linodeID, configID},
		func() *linodev1.ConfigInterfaceResponse { return &linodev1.ConfigInterfaceResponse{} })
}

// httpListInstanceInterfaceHistoryProto retrieves the historical interface
// versions of one Linode instance as proto messages.
func (c *Client) httpListInstanceInterfaceHistoryProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.InstanceInterfaceHistory, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	return listProtoElementsPaginatedRouted(ctx, c, "ListInstanceInterfaceHistory",
		"linode_instance_interface_history_list", "", []any{linodeID}, page, pageSize,
		func() *linodev1.InstanceInterfaceHistory { return &linodev1.InstanceInterfaceHistory{} })
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

// httpGetInstanceConfigProto retrieves a specific configuration profile for a
// Linode instance.
func (c *Client) httpGetInstanceConfigProto(ctx context.Context, linodeID, configID int) (*linodev1.InstanceConfig, error) {
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

	config := &linodev1.InstanceConfig{}
	if err := c.handleProtoResponse(resp, config); err != nil {
		return nil, err
	}

	return config, nil
}

// DeleteInstanceConfig deletes a configuration profile from a Linode instance.
func (c *Client) httpDeleteInstanceConfig(ctx context.Context, linodeID, configID int) error {
	if linodeID <= 0 {
		return ErrLinodeIDPositive
	}

	if configID <= 0 {
		return ErrConfigIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_config_delete", nil, linodeID, configID)
	if err != nil {
		return wrapRequestError("DeleteInstanceConfig", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ReorderInstanceConfigInterfaces reorders the interfaces for a Linode instance configuration profile.
func (c *Client) httpReorderInstanceConfigInterfaces(ctx context.Context, linodeID, configID int, req *ReorderConfigInterfacesRequest) error {
	if linodeID <= 0 {
		return ErrLinodeIDPositive
	}

	if configID <= 0 {
		return ErrConfigIDPositive
	}

	if req == nil {
		return ErrReorderConfigInterfacesRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_config_interface_reorder", req, linodeID, configID)
	if err != nil {
		return wrapRequestError("ReorderInstanceConfigInterfaces", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
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

// httpListInstanceNodeBalancersProto retrieves the NodeBalancers assigned to a
// Linode instance as proto messages.
func (c *Client) httpListInstanceNodeBalancersProto(ctx context.Context, linodeID int) ([]*linodev1.NodeBalancer, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	return listProtoElementsRouted(ctx, c, "ListInstanceNodeBalancers",
		"linode_instance_nodebalancer_list", "", []any{linodeID},
		func() *linodev1.NodeBalancer { return &linodev1.NodeBalancer{} })
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

// httpGetInstanceDiskProto retrieves one instance disk as a proto message.
func (c *Client) httpGetInstanceDiskProto(ctx context.Context, linodeID, diskID int) (*linodev1.InstanceDisk, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_disk_get", nil, linodeID, diskID)
	if err != nil {
		return nil, wrapRequestError("GetInstanceDisk", err)
	}

	defer drainClose(resp)

	disk := &linodev1.InstanceDisk{}
	if err := c.handleProtoResponse(resp, disk); err != nil {
		return nil, err
	}

	return disk, nil
}

// DeleteInstanceDisk deletes a disk from a Linode instance.
func (c *Client) httpDeleteInstanceDisk(ctx context.Context, linodeID, diskID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_disk_delete", nil, linodeID, diskID)
	if err != nil {
		return wrapRequestError("DeleteInstanceDisk", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ResizeInstanceDisk resizes a disk on a Linode instance.
func (c *Client) httpResizeInstanceDisk(ctx context.Context, linodeID, diskID int, req ResizeDiskRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_disk_resize", req, linodeID, diskID)
	if err != nil {
		return wrapRequestError("ResizeInstanceDisk", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ResetInstanceDiskPassword resets the root password for a disk on a Linode instance.
func (c *Client) httpResetInstanceDiskPassword(ctx context.Context, linodeID, diskID int, password string) error {
	if linodeID <= 0 {
		return ErrLinodeIDPositive
	}

	if diskID <= 0 {
		return ErrDiskIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	payload := map[string]string{"password": password}

	resp, err := c.makeRouteRequest(ctx, "linode_instance_disk_password_reset", payload, linodeID, diskID)
	if err != nil {
		return wrapRequestError("ResetInstanceDiskPassword", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
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

// httpListInstanceIPsProto retrieves the full IPv4/IPv6 address configuration for
// a Linode instance. The /ips endpoint returns a nested object, not a page
// envelope.
func (c *Client) httpListInstanceIPsProto(ctx context.Context, linodeID int) (*linodev1.InstanceIPsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_ip_list", nil, linodeID)
	if err != nil {
		return nil, wrapRequestError("ListInstanceIPs", err)
	}

	defer drainClose(resp)

	ips := &linodev1.InstanceIPsResponse{}
	if err := c.handleProtoResponse(resp, ips); err != nil {
		return nil, err
	}

	return ips, nil
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

// httpGetInstanceIPProto retrieves one instance IP address as a proto message.
func (c *Client) httpGetInstanceIPProto(ctx context.Context, linodeID int, address string) (*linodev1.IPAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_ip_get", nil, linodeID, address)
	if err != nil {
		return nil, wrapRequestError("GetInstanceIP", err)
	}

	defer drainClose(resp)

	ip := &linodev1.IPAddress{}
	if err := c.handleProtoResponse(resp, ip); err != nil {
		return nil, err
	}

	return ip, nil
}

// AllocateInstanceIPProto allocates an instance IP. The POST is non-idempotent,
// so it is not retried.
func (c *Client) AllocateInstanceIPProto(ctx context.Context, linodeID int, req AllocateIPRequest) (*linodev1.IPAddress, error) {
	var ipAddr *linodev1.IPAddress

	err := c.executeWithoutRetry(ctx, "AllocateInstanceIP", func() error {
		var err error

		ipAddr, err = c.httpAllocateInstanceIPProto(ctx, linodeID, req)

		return err
	})

	return ipAddr, err
}

// httpAllocateInstanceIPProto allocates an instance IP.
func (c *Client) httpAllocateInstanceIPProto(ctx context.Context, linodeID int, req AllocateIPRequest) (*linodev1.IPAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_ip_allocate", req, linodeID)
	if err != nil {
		return nil, wrapRequestError("AllocateInstanceIP", err)
	}

	defer drainClose(resp)

	ip := &linodev1.IPAddress{}
	if err := c.handleProtoResponse(resp, ip); err != nil {
		return nil, err
	}

	return ip, nil
}

// UpdateInstanceIPProto updates an instance IP's RDNS.
func (c *Client) UpdateInstanceIPProto(ctx context.Context, linodeID int, address string, req UpdateIPRDNSRequest) (*linodev1.IPAddress, error) {
	var ipAddr *linodev1.IPAddress

	err := c.executeWithRetry(ctx, "UpdateInstanceIP", func() error {
		var retryErr error

		ipAddr, retryErr = c.httpUpdateInstanceIPProto(ctx, linodeID, address, req)

		return retryErr
	})

	return ipAddr, err
}

// httpUpdateInstanceIPProto updates an instance IP's RDNS.
func (c *Client) httpUpdateInstanceIPProto(ctx context.Context, linodeID int, address string, req UpdateIPRDNSRequest) (*linodev1.IPAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_ip_update", req, linodeID, address)
	if err != nil {
		return nil, wrapRequestError("UpdateInstanceIP", err)
	}

	defer drainClose(resp)

	ip := &linodev1.IPAddress{}
	if err := c.handleProtoResponse(resp, ip); err != nil {
		return nil, err
	}

	return ip, nil
}

// DeleteInstanceIP removes an IP address from a Linode instance.
func (c *Client) httpDeleteInstanceIP(ctx context.Context, linodeID int, address string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_ip_delete", nil, linodeID, address)
	if err != nil {
		return wrapRequestError("DeleteInstanceIP", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCloneInstanceProto clones a Linode instance.
func (c *Client) httpCloneInstanceProto(ctx context.Context, linodeID int, req *CloneInstanceRequest) (*linodev1.Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_clone", req, linodeID)
	if err != nil {
		return nil, wrapRequestError("CloneInstance", err)
	}

	defer drainClose(resp)

	instance := &linodev1.Instance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}

// MigrateInstance migrates a Linode instance to a new region.
func (c *Client) httpMigrateInstance(ctx context.Context, linodeID int, region string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var payload any
	if region != "" {
		payload = map[string]string{"region": region}
	}

	resp, err := c.makeRouteRequest(ctx, "linode_instance_migrate", payload, linodeID)
	if err != nil {
		return wrapRequestError("MigrateInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpMutateInstance upgrades a Linode instance to the latest generation type.
func (c *Client) httpMutateInstance(ctx context.Context, linodeID int, req *MutateInstanceRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_mutate", req, linodeID)
	if err != nil {
		return wrapRequestError("MutateInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// RescueInstance boots a Linode instance into rescue mode.
func (c *Client) httpRescueInstance(ctx context.Context, linodeID int, req RescueInstanceRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_rescue", req, linodeID)
	if err != nil {
		return wrapRequestError("RescueInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ResetInstancePassword resets the root password on a Linode instance.
func (c *Client) httpResetInstancePassword(ctx context.Context, linodeID int, rootPass string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	payload := map[string]string{"root_pass": rootPass}

	resp, err := c.makeRouteRequest(ctx, "linode_instance_password_reset", payload, linodeID)
	if err != nil {
		return wrapRequestError("ResetInstancePassword", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCreateInstanceConfigProto creates a configuration profile.
func (c *Client) httpCreateInstanceConfigProto(ctx context.Context, linodeID int, req *CreateConfigRequest) (*linodev1.InstanceConfig, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req == nil {
		return nil, ErrCreateConfigRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_config_create", req, linodeID)
	if err != nil {
		return nil, wrapRequestError("CreateInstanceConfig", err)
	}

	defer drainClose(resp)

	config := &linodev1.InstanceConfig{}
	if err := c.handleProtoResponse(resp, config); err != nil {
		return nil, err
	}

	return config, nil
}

// httpUpdateInstanceConfigProto updates a configuration profile.
func (c *Client) httpUpdateInstanceConfigProto(ctx context.Context, linodeID, configID int, req *UpdateConfigRequest) (*linodev1.InstanceConfig, error) {
	if err := validateInstanceConfigMutation(linodeID, configID, req == nil, ErrUpdateConfigRequestRequired); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_config_update", req, linodeID, configID)
	if err != nil {
		return nil, wrapRequestError("UpdateInstanceConfig", err)
	}

	defer drainClose(resp)

	config := &linodev1.InstanceConfig{}
	if err := c.handleProtoResponse(resp, config); err != nil {
		return nil, err
	}

	return config, nil
}

// httpAddInstanceConfigInterfaceProto appends a network interface to a
// configuration profile.
func (c *Client) httpAddInstanceConfigInterfaceProto(ctx context.Context, linodeID, configID int, req *ConfigInterface) (*linodev1.ConfigInterfaceResponse, error) {
	if err := validateInstanceConfigMutation(linodeID, configID, req == nil, ErrAddConfigInterfaceRequestRequired); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_config_interface_add", req, linodeID, configID)
	if err != nil {
		return nil, wrapRequestError("AddInstanceConfigInterface", err)
	}

	defer drainClose(resp)

	configInterface := &linodev1.ConfigInterfaceResponse{}
	if err := c.handleProtoResponse(resp, configInterface); err != nil {
		return nil, err
	}

	return configInterface, nil
}

// httpUpdateInstanceConfigInterfaceProto updates a configuration profile
// interface.
func (c *Client) httpUpdateInstanceConfigInterfaceProto(ctx context.Context, linodeID, configID, interfaceID int, req *UpdateConfigInterfaceRequest) (*linodev1.ConfigInterfaceResponse, error) {
	if err := validateInstanceConfigMutation(linodeID, configID, req == nil, ErrUpdateConfigInterfaceRequestRequired); err != nil {
		return nil, err
	}

	if interfaceID <= 0 {
		return nil, ErrInterfaceIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_config_interface_update", req, linodeID, configID, interfaceID)
	if err != nil {
		return nil, wrapRequestError("UpdateInstanceConfigInterface", err)
	}

	defer drainClose(resp)

	configInterface := &linodev1.ConfigInterfaceResponse{}
	if err := c.handleProtoResponse(resp, configInterface); err != nil {
		return nil, err
	}

	return configInterface, nil
}

// httpCreateInstanceDiskProto creates a disk and decodes the response into the
// proto element.
func (c *Client) httpCreateInstanceDiskProto(ctx context.Context, linodeID int, req *CreateDiskRequest) (*linodev1.InstanceDisk, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_disk_create", req, linodeID)
	if err != nil {
		return nil, wrapRequestError("CreateInstanceDisk", err)
	}

	defer drainClose(resp)

	disk := &linodev1.InstanceDisk{}
	if err := c.handleProtoResponse(resp, disk); err != nil {
		return nil, err
	}

	return disk, nil
}

// httpUpdateInstanceDiskProto updates a disk and decodes the response into the
// proto element.
func (c *Client) httpUpdateInstanceDiskProto(ctx context.Context, linodeID, diskID int, req UpdateDiskRequest) (*linodev1.InstanceDisk, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_disk_update", req, linodeID, diskID)
	if err != nil {
		return nil, wrapRequestError("UpdateInstanceDisk", err)
	}

	defer drainClose(resp)

	disk := &linodev1.InstanceDisk{}
	if err := c.handleProtoResponse(resp, disk); err != nil {
		return nil, err
	}

	return disk, nil
}

// httpCloneInstanceDiskProto clones a disk and decodes the response into the
// proto element.
func (c *Client) httpCloneInstanceDiskProto(ctx context.Context, linodeID, diskID int) (*linodev1.InstanceDisk, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_disk_clone", nil, linodeID, diskID)
	if err != nil {
		return nil, wrapRequestError("CloneInstanceDisk", err)
	}

	defer drainClose(resp)

	disk := &linodev1.InstanceDisk{}
	if err := c.handleProtoResponse(resp, disk); err != nil {
		return nil, err
	}

	return disk, nil
}

// httpCreateInstanceBackupProto takes a manual snapshot and decodes the response
// into the proto element.
func (c *Client) httpCreateInstanceBackupProto(ctx context.Context, linodeID int, label string) (*linodev1.InstanceBackup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var body any
	if label != "" {
		body = CreateInstanceBackupRequest{Label: label}
	}

	resp, err := c.makeRouteRequest(ctx, "linode_instance_backup_create", body, linodeID)
	if err != nil {
		return nil, wrapRequestError("CreateInstanceBackup", err)
	}

	defer drainClose(resp)

	backup := &linodev1.InstanceBackup{}
	if err := c.handleProtoResponse(resp, backup); err != nil {
		return nil, err
	}

	return backup, nil
}

// httpRebuildInstanceProto rebuilds a Linode instance and decodes the response
// into the proto element.
func (c *Client) httpRebuildInstanceProto(ctx context.Context, linodeID int, req *RebuildInstanceRequest) (*linodev1.Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_instance_rebuild", req, linodeID)
	if err != nil {
		return nil, wrapRequestError("RebuildInstance", err)
	}

	defer drainClose(resp)

	instance := &linodev1.Instance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}
