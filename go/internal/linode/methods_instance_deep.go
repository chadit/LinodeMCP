package linode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const (
	endpointInstanceDeep = "/linode/instances"
)

// ListInstanceBackups retrieves all backups for a Linode instance.
func (c *Client) httpListInstanceBackups(ctx context.Context, linodeID int) (*InstanceBackupsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/backups", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceBackups", Err: err}
	}

	defer drainClose(resp)

	var backups InstanceBackupsResponse
	if err := c.handleResponse(resp, &backups); err != nil {
		return nil, err
	}

	return &backups, nil
}

// httpListInstanceBackupsProto retrieves all backups for a Linode instance as a
// proto message. The /backups endpoint returns a nested object (automatic[] plus
// a snapshot object), so this decodes the whole structure into
// InstanceBackupsResponse.
func (c *Client) httpListInstanceBackupsProto(ctx context.Context, linodeID int) (*linodev1.InstanceBackupsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/backups", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceBackups", Err: err}
	}

	defer drainClose(resp)

	backups := &linodev1.InstanceBackupsResponse{}
	if err := c.handleProtoResponse(resp, backups); err != nil {
		return nil, err
	}

	return backups, nil
}

// GetInstanceStats retrieves daily statistics for a Linode instance.
func (c *Client) httpGetInstanceStats(ctx context.Context, linodeID int) (*InstanceStats, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/stats", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceStats", Err: err}
	}

	defer drainClose(resp)

	var stats InstanceStats
	if err := c.handleResponse(resp, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// GetInstanceTransferByYearMonth retrieves monthly network transfer statistics for a Linode instance.
func (c *Client) httpGetInstanceTransferByYearMonth(ctx context.Context, linodeID, year, month int) (*Transfer, error) {
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedYear := url.PathEscape(strconv.Itoa(year))
	encodedMonth := url.PathEscape(strconv.Itoa(month))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/transfer/%s/%s", encodedLinodeID, encodedYear, encodedMonth)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceTransferByYearMonth", Err: err}
	}

	defer drainClose(resp)

	var transfer Transfer
	if err := c.handleResponse(resp, &transfer); err != nil {
		return nil, err
	}

	return &transfer, nil
}

// GetInstanceBackup retrieves a specific backup for a Linode instance.
func (c *Client) httpGetInstanceBackup(ctx context.Context, linodeID, backupID int) (*InstanceBackup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/backups/%d", linodeID, backupID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceBackup", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/backups/%d", linodeID, backupID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceBackup", Err: err}
	}

	defer drainClose(resp)

	backup := &linodev1.InstanceBackup{}
	if err := c.handleProtoResponse(resp, backup); err != nil {
		return nil, err
	}

	return backup, nil
}

// CreateInstanceBackup creates a manual snapshot for a Linode instance.
func (c *Client) httpCreateInstanceBackup(ctx context.Context, linodeID int, label string) (*InstanceBackup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/backups", linodeID)

	var body any
	if label != "" {
		body = CreateInstanceBackupRequest{Label: label}
	}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstanceBackup", Err: err}
	}

	defer drainClose(resp)

	var backup InstanceBackup
	if err := c.handleResponse(resp, &backup); err != nil {
		return nil, err
	}

	return &backup, nil
}

// RestoreInstanceBackup restores a backup to a Linode instance.
func (c *Client) httpRestoreInstanceBackup(ctx context.Context, linodeID, backupID int, req RestoreBackupRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/backups/%d/restore", linodeID, backupID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "RestoreInstanceBackup", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// EnableInstanceBackups enables the backup service for a Linode instance.
func (c *Client) httpEnableInstanceBackups(ctx context.Context, linodeID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/backups/enable", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "EnableInstanceBackups", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// CancelInstanceBackups cancels the backup service for a Linode instance.
func (c *Client) httpCancelInstanceBackups(ctx context.Context, linodeID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/backups/cancel", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "CancelInstanceBackups", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/firewalls/apply", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "ApplyInstanceFirewalls", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// CreateInstanceConfig creates a configuration profile for a Linode instance.
func (c *Client) httpCreateInstanceConfig(ctx context.Context, linodeID int, req *CreateConfigRequest) (*InstanceConfig, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req == nil {
		return nil, ErrCreateConfigRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/configs", encodedLinodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstanceConfig", Err: err}
	}

	defer drainClose(resp)

	var config InstanceConfig
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// AddInstanceConfigInterface appends a network interface to a configuration profile.
func (c *Client) httpAddInstanceConfigInterface(ctx context.Context, linodeID, configID int, req *ConfigInterface) (*ConfigInterface, error) {
	if err := validateInstanceConfigMutation(linodeID, configID, req == nil, ErrAddConfigInterfaceRequestRequired); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := instanceConfigEndpoint(linodeID, configID) + "/interfaces"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AddInstanceConfigInterface", Err: err}
	}

	defer drainClose(resp)

	var configInterface ConfigInterface
	if err := c.handleResponse(resp, &configInterface); err != nil {
		return nil, err
	}

	return &configInterface, nil
}

// AddInstanceInterface creates an interface on an existing Linode instance.
func (c *Client) httpAddInstanceInterface(ctx context.Context, linodeID int, req *AddInstanceInterfaceRequest) (*InstanceInterface, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req == nil {
		return nil, ErrAddInstanceInterfaceRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces", encodedLinodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AddInstanceInterface", Err: err}
	}

	defer drainClose(resp)

	var instanceInterface InstanceInterface
	if err := c.handleResponse(resp, &instanceInterface); err != nil {
		return nil, err
	}

	return &instanceInterface, nil
}

// UpdateInstanceInterface updates an interface on an existing Linode instance.
func (c *Client) httpUpdateInstanceInterface(ctx context.Context, linodeID, interfaceID int, req *UpdateInstanceInterfaceRequest) (*InstanceInterface, error) {
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/%s", encodedLinodeID, encodedInterfaceID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceInterface", Err: err}
	}

	defer drainClose(resp)

	var instanceInterface InstanceInterface
	if err := c.handleResponse(resp, &instanceInterface); err != nil {
		return nil, err
	}

	return &instanceInterface, nil
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/%s", encodedLinodeID, encodedInterfaceID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteInstanceInterface", Err: err}
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

	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := instanceConfigEndpoint(linodeID, configID) + "/interfaces/" + encodedInterfaceID

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceConfigInterface", Err: err}
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

	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := instanceConfigEndpoint(linodeID, configID) + "/interfaces/" + encodedInterfaceID

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceConfigInterface", Err: err}
	}

	defer drainClose(resp)

	configInterface := &linodev1.ConfigInterfaceResponse{}
	if err := c.handleProtoResponse(resp, configInterface); err != nil {
		return nil, err
	}

	return configInterface, nil
}

// UpdateInstanceConfigInterface updates a network interface on a configuration profile.
func (c *Client) httpUpdateInstanceConfigInterface(ctx context.Context, linodeID, configID, interfaceID int, req *UpdateConfigInterfaceRequest) (*ConfigInterfaceResponse, error) {
	if err := validateInstanceConfigMutation(linodeID, configID, req == nil, ErrUpdateConfigInterfaceRequestRequired); err != nil {
		return nil, err
	}

	if interfaceID <= 0 {
		return nil, ErrInterfaceIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := instanceConfigEndpoint(linodeID, configID) + "/interfaces/" + encodedInterfaceID

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceConfigInterface", Err: err}
	}

	defer drainClose(resp)

	var configInterface ConfigInterfaceResponse
	if err := c.handleResponse(resp, &configInterface); err != nil {
		return nil, err
	}

	return &configInterface, nil
}

// DeleteInstanceConfigInterface removes a network interface from a configuration profile.
func (c *Client) httpDeleteInstanceConfigInterface(ctx context.Context, linodeID, configID, interfaceID int) error {
	if err := validateInstanceConfigInterfaceIDs(linodeID, configID, interfaceID); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := instanceConfigEndpoint(linodeID, configID) + "/interfaces/" + encodedInterfaceID

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteInstanceConfigInterface", Err: err}
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/settings", encodedLinodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceInterfaceSettings", Err: err}
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/settings", encodedLinodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceInterfaceSettings", Err: err}
	}

	defer drainClose(resp)

	settings := &linodev1.InstanceInterfaceSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// UpdateInstanceInterfaceSettings updates interface settings for a Linode instance.
func (c *Client) httpUpdateInstanceInterfaceSettings(ctx context.Context, linodeID int, req *UpdateInstanceInterfaceSettingsRequest) (*InstanceInterfaceSettings, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req == nil {
		return nil, ErrUpdateInterfaceSettingsRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/settings", encodedLinodeID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceInterfaceSettings", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceDisks", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[InstanceDisk]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListInstanceDisksProto retrieves a Linode instance's disks as proto
// messages for the proto-backed list path. The endpoint is formatted with the
// same fmt.Sprintf(endpointInstanceDeep+"/%d/disks", linodeID) pattern
// httpListInstanceDisks uses, so the runtime path matches exactly.
func (c *Client) httpListInstanceDisksProto(ctx context.Context, linodeID int) ([]*linodev1.InstanceDisk, error) {
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks", linodeID)

	return listProtoElements(ctx, c, "ListInstanceDisks", endpoint,
		func() *linodev1.InstanceDisk { return &linodev1.InstanceDisk{} })
}

// UpdateInstanceConfig updates a configuration profile for a Linode instance.
func (c *Client) httpUpdateInstanceConfig(ctx context.Context, linodeID, configID int, req *UpdateConfigRequest) (*InstanceConfig, error) {
	if err := validateInstanceConfigMutation(linodeID, configID, req == nil, ErrUpdateConfigRequestRequired); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := instanceConfigEndpoint(linodeID, configID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceConfig", Err: err}
	}

	defer drainClose(resp)

	var config InstanceConfig
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return &config, nil
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

func instanceConfigEndpoint(linodeID, configID int) string {
	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))

	return fmt.Sprintf(endpointInstanceDeep+"/%s/configs/%s", encodedLinodeID, encodedConfigID)
}

// ListInstanceConfigs retrieves all configuration profiles for a Linode instance.
func (c *Client) httpListInstanceConfigs(ctx context.Context, linodeID, page, pageSize int) ([]InstanceConfig, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := withPaginationQuery(fmt.Sprintf(endpointInstanceDeep+"/%s/configs", encodedLinodeID), page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceConfigs", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[InstanceConfig]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListInstanceConfigsProto retrieves a Linode instance's configuration
// profiles as proto messages for the proto-backed list path. The endpoint is
// formatted with the same encoded-linode-id path httpListInstanceConfigs uses,
// then listProtoElementsPaginated adds page/page_size via withPaginationQuery, so
// the runtime request matches exactly.
func (c *Client) httpListInstanceConfigsProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.InstanceConfig, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/configs", encodedLinodeID)

	return listProtoElementsPaginated(ctx, c, "ListInstanceConfigs", endpoint, page, pageSize,
		func() *linodev1.InstanceConfig { return &linodev1.InstanceConfig{} })
}

// ListInstanceVolumes retrieves all volumes attached to a Linode instance.
func (c *Client) httpListInstanceVolumes(ctx context.Context, linodeID, page, pageSize int) ([]Volume, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := withPaginationQuery(fmt.Sprintf(endpointInstanceDeep+"/%s/volumes", encodedLinodeID), page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceVolumes", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Volume]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListInstanceVolumesProto retrieves a Linode instance's attached volumes as
// proto messages for the proto-backed list path. The endpoint is formatted with
// the same encoded-linode-id path httpListInstanceVolumes uses, then
// listProtoElementsPaginated adds page/page_size via withPaginationQuery, so the
// runtime request matches exactly.
func (c *Client) httpListInstanceVolumesProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.Volume, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/volumes", encodedLinodeID)

	return listProtoElementsPaginated(ctx, c, "ListInstanceVolumes", endpoint, page, pageSize,
		func() *linodev1.Volume { return &linodev1.Volume{} })
}

// ListInstanceNodeBalancers retrieves NodeBalancers assigned to a Linode instance.
func (c *Client) httpListInstanceNodeBalancers(ctx context.Context, linodeID int) ([]NodeBalancer, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/nodebalancers", encodedLinodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceNodeBalancers", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[NodeBalancer]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// UpdateInstanceFirewalls replaces firewall assignments for a Linode instance.
func (c *Client) httpUpdateInstanceFirewalls(ctx context.Context, linodeID, page, pageSize int, req *UpdateInstanceFirewallsRequest) ([]Firewall, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req == nil {
		return nil, ErrUpdateInstanceFirewallsRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := withPaginationQuery(fmt.Sprintf(endpointInstanceDeep+"/%s/firewalls", encodedLinodeID), page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceFirewalls", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Firewall]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListInstanceInterfaces retrieves all interfaces for a Linode instance.
func (c *Client) httpListInstanceInterfaces(ctx context.Context, linodeID int) ([]InstanceInterface, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces", encodedLinodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceInterfaces", Err: err}
	}

	defer drainClose(resp)

	var payload struct {
		Interfaces []InstanceInterface `json:"interfaces"`
	}
	if err := c.handleResponse(resp, &payload); err != nil {
		return nil, err
	}

	return payload.Interfaces, nil
}

// httpListInstanceInterfacesProto retrieves the current-generation interfaces for
// a Linode instance as proto messages. The endpoint wraps elements under the
// "interfaces" key (not the usual "data" page envelope), so it reads through
// listProtoElementsKeyed with that key.
func (c *Client) httpListInstanceInterfacesProto(ctx context.Context, linodeID int) ([]*linodev1.InstanceInterface, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces", encodedLinodeID)

	return listProtoElementsKeyed(ctx, c, "ListInstanceInterfaces", endpoint, "interfaces",
		func() *linodev1.InstanceInterface { return &linodev1.InstanceInterface{} })
}

// UpgradeLinodeInterfaces upgrades legacy config interfaces to Linode interfaces.
func (c *Client) httpUpgradeLinodeInterfaces(ctx context.Context, linodeID int, req *UpgradeLinodeInterfacesRequest) (*UpgradeLinodeInterfacesResponse, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/upgrade-interfaces", encodedLinodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpgradeLinodeInterfaces", Err: err}
	}

	defer drainClose(resp)

	var result UpgradeLinodeInterfacesResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/%s", encodedLinodeID, encodedInterfaceID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceInterface", Err: err}
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/%s", encodedLinodeID, encodedInterfaceID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceInterface", Err: err}
	}

	defer drainClose(resp)

	instanceInterface := &linodev1.InstanceInterface{}
	if err := c.handleProtoResponse(resp, instanceInterface); err != nil {
		return nil, err
	}

	return instanceInterface, nil
}

// ListInstanceInterfaceFirewalls retrieves Cloud Firewalls assigned to a Linode interface.
func (c *Client) httpListInstanceInterfaceFirewalls(ctx context.Context, linodeID, interfaceID int) ([]Firewall, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if interfaceID <= 0 {
		return nil, ErrInterfaceIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/%s/firewalls", encodedLinodeID, encodedInterfaceID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceInterfaceFirewalls", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Firewall]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListInstanceInterfaceFirewallsProto retrieves the Cloud Firewalls assigned
// to a Linode interface as proto messages for the proto-backed list path. The
// endpoint is formatted with the same encoded linode-id/interface-id path
// httpListInstanceInterfaceFirewalls uses; this list is not paginated, so it uses
// listProtoElements directly.
func (c *Client) httpListInstanceInterfaceFirewallsProto(ctx context.Context, linodeID, interfaceID int) ([]*linodev1.Firewall, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if interfaceID <= 0 {
		return nil, ErrInterfaceIDPositive
	}

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/%s/firewalls", encodedLinodeID, encodedInterfaceID)

	return listProtoElements(ctx, c, "ListInstanceInterfaceFirewalls", endpoint,
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
}

// ListInstanceInterfaceHistory retrieves historical interface versions for a Linode instance.
func (c *Client) httpListInstanceInterfaceHistory(ctx context.Context, linodeID, page, pageSize int) (*PaginatedResponse[InstanceInterfaceHistory], error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := withPaginationQuery(fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/history", encodedLinodeID), page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceInterfaceHistory", Err: err}
	}

	defer drainClose(resp)

	var history PaginatedResponse[InstanceInterfaceHistory]
	if err := c.handleResponse(resp, &history); err != nil {
		return nil, err
	}

	return &history, nil
}

// httpListInstanceConfigInterfacesProto retrieves the legacy config-profile
// network interfaces of one configuration profile as proto messages. It formats
// both path ids into the endpoint exactly like httpListInstanceConfigInterfaces,
// then reads through listProtoElementsBareOrData because the endpoint returns
// either a bare array or a {data:[...]} page envelope.
func (c *Client) httpListInstanceConfigInterfacesProto(ctx context.Context, linodeID, configID int) ([]*linodev1.ConfigInterfaceResponse, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/configs/%s/interfaces", encodedLinodeID, encodedConfigID)

	return listProtoElementsBareOrData(ctx, c, "ListInstanceConfigInterfaces", endpoint,
		func() *linodev1.ConfigInterfaceResponse { return &linodev1.ConfigInterfaceResponse{} })
}

// httpListInstanceInterfaceHistoryProto retrieves the historical interface
// versions of one Linode instance as proto messages. It formats the path id into
// the endpoint exactly like httpListInstanceInterfaceHistory, then reuses
// listProtoElementsPaginated (which adds the page/page_size query) to decode the
// {data:[...]} page envelope.
func (c *Client) httpListInstanceInterfaceHistoryProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.InstanceInterfaceHistory, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces/history", encodedLinodeID)

	return listProtoElementsPaginated(ctx, c, "ListInstanceInterfaceHistory", endpoint, page, pageSize,
		func() *linodev1.InstanceInterfaceHistory { return &linodev1.InstanceInterfaceHistory{} })
}

func (c *Client) httpListInstanceConfigInterfaces(ctx context.Context, linodeID, configID int) ([]ConfigInterfaceResponse, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if configID <= 0 {
		return nil, ErrConfigIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/configs/%s/interfaces", encodedLinodeID, encodedConfigID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceConfigInterfaces", Err: err}
	}

	defer drainClose(resp)

	var interfaces configInterfaceListResponse
	if err := c.handleResponse(resp, &interfaces); err != nil {
		return nil, err
	}

	return []ConfigInterfaceResponse(interfaces), nil
}

type configInterfaceListResponse []ConfigInterfaceResponse

func (r *configInterfaceListResponse) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var interfaces []ConfigInterfaceResponse
		if err := json.Unmarshal(trimmed, &interfaces); err != nil {
			return fmt.Errorf("decode configuration profile interfaces array: %w", err)
		}

		*r = interfaces

		return nil
	}

	var response PaginatedResponse[ConfigInterfaceResponse]
	if err := json.Unmarshal(trimmed, &response); err != nil {
		return fmt.Errorf("decode configuration profile interfaces envelope: %w", err)
	}

	*r = response.Data

	return nil
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/configs/%s", encodedLinodeID, encodedConfigID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceConfig", Err: err}
	}

	defer drainClose(resp)

	var config InstanceConfig
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return &config, nil
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/configs/%s", encodedLinodeID, encodedConfigID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteInstanceConfig", Err: err}
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedConfigID := url.PathEscape(strconv.Itoa(configID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/configs/%s/interfaces/order", encodedLinodeID, encodedConfigID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "ReorderInstanceConfigInterfaces", Err: err}
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := withPaginationQuery(fmt.Sprintf(endpointInstanceDeep+"/%s/firewalls", encodedLinodeID), page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceFirewalls", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Firewall]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListInstanceFirewallsProto retrieves a Linode instance's assigned Cloud
// Firewalls as proto messages for the proto-backed list path. The endpoint is
// formatted with the same encoded-linode-id path httpListInstanceFirewalls uses,
// then listProtoElementsPaginated adds page/page_size via withPaginationQuery, so
// the runtime request matches exactly.
func (c *Client) httpListInstanceFirewallsProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.Firewall, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/firewalls", encodedLinodeID)

	return listProtoElementsPaginated(ctx, c, "ListInstanceFirewalls", endpoint, page, pageSize,
		func() *linodev1.Firewall { return &linodev1.Firewall{} })
}

// httpListInstanceNodeBalancersProto retrieves the NodeBalancers assigned to a
// Linode instance as proto messages for the proto-backed list path. The endpoint
// is formatted with the same encoded-linode-id path httpListInstanceNodeBalancers
// uses; this list is not paginated, so it uses listProtoElements directly.
func (c *Client) httpListInstanceNodeBalancersProto(ctx context.Context, linodeID int) ([]*linodev1.NodeBalancer, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/nodebalancers", encodedLinodeID)

	return listProtoElements(ctx, c, "ListInstanceNodeBalancers", endpoint,
		func() *linodev1.NodeBalancer { return &linodev1.NodeBalancer{} })
}

// GetInstanceDisk retrieves a specific disk for a Linode instance.
func (c *Client) httpGetInstanceDisk(ctx context.Context, linodeID, diskID int) (*InstanceDisk, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks/%d", linodeID, diskID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceDisk", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks/%d", linodeID, diskID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceDisk", Err: err}
	}

	defer drainClose(resp)

	disk := &linodev1.InstanceDisk{}
	if err := c.handleProtoResponse(resp, disk); err != nil {
		return nil, err
	}

	return disk, nil
}

// CreateInstanceDisk creates a new disk for a Linode instance.
func (c *Client) httpCreateInstanceDisk(ctx context.Context, linodeID int, req *CreateDiskRequest) (*InstanceDisk, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstanceDisk", Err: err}
	}

	defer drainClose(resp)

	var disk InstanceDisk
	if err := c.handleResponse(resp, &disk); err != nil {
		return nil, err
	}

	return &disk, nil
}

// UpdateInstanceDisk updates a disk for a Linode instance.
func (c *Client) httpUpdateInstanceDisk(ctx context.Context, linodeID, diskID int, req UpdateDiskRequest) (*InstanceDisk, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks/%d", linodeID, diskID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceDisk", Err: err}
	}

	defer drainClose(resp)

	var disk InstanceDisk
	if err := c.handleResponse(resp, &disk); err != nil {
		return nil, err
	}

	return &disk, nil
}

// DeleteInstanceDisk deletes a disk from a Linode instance.
func (c *Client) httpDeleteInstanceDisk(ctx context.Context, linodeID, diskID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks/%d", linodeID, diskID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteInstanceDisk", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// CloneInstanceDisk clones a disk on a Linode instance.
func (c *Client) httpCloneInstanceDisk(ctx context.Context, linodeID, diskID int) (*InstanceDisk, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks/%d/clone", linodeID, diskID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "CloneInstanceDisk", Err: err}
	}

	defer drainClose(resp)

	var disk InstanceDisk
	if err := c.handleResponse(resp, &disk); err != nil {
		return nil, err
	}

	return &disk, nil
}

// ResizeInstanceDisk resizes a disk on a Linode instance.
func (c *Client) httpResizeInstanceDisk(ctx context.Context, linodeID, diskID int, req ResizeDiskRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks/%d/resize", linodeID, diskID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "ResizeInstanceDisk", Err: err}
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

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	encodedDiskID := url.PathEscape(strconv.Itoa(diskID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/disks/%s/password", encodedLinodeID, encodedDiskID)

	payload := map[string]string{"password": password}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return &NetworkError{Operation: "ResetInstanceDiskPassword", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ListInstanceIPs retrieves all IP addresses for a Linode instance.
func (c *Client) httpListInstanceIPs(ctx context.Context, linodeID int) (*InstanceIPAddresses, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/ips", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceIPs", Err: err}
	}

	defer drainClose(resp)

	var ips InstanceIPAddresses
	if err := c.handleResponse(resp, &ips); err != nil {
		return nil, err
	}

	return &ips, nil
}

// httpListInstanceIPsProto retrieves the full IPv4/IPv6 address configuration for
// a Linode instance as a proto message. The /ips endpoint returns a nested
// object, so this decodes the whole structure into InstanceIPsResponse.
func (c *Client) httpListInstanceIPsProto(ctx context.Context, linodeID int) (*linodev1.InstanceIPsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/ips", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstanceIPs", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/ips/%s", linodeID, url.PathEscape(address))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceIP", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/ips/%s", linodeID, url.PathEscape(address))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceIP", Err: err}
	}

	defer drainClose(resp)

	ip := &linodev1.IPAddress{}
	if err := c.handleProtoResponse(resp, ip); err != nil {
		return nil, err
	}

	return ip, nil
}

// AllocateInstanceIP allocates a new IP address for a Linode instance.
func (c *Client) httpAllocateInstanceIP(ctx context.Context, linodeID int, req AllocateIPRequest) (*IPAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/ips", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AllocateInstanceIP", Err: err}
	}

	defer drainClose(resp)

	var ip IPAddress
	if err := c.handleResponse(resp, &ip); err != nil {
		return nil, err
	}

	return &ip, nil
}

// UpdateInstanceIP updates the reverse DNS for a Linode instance IP address.
func (c *Client) httpUpdateInstanceIP(ctx context.Context, linodeID int, address string, req UpdateIPRDNSRequest) (*IPAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/ips/%s", linodeID, url.PathEscape(address))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceIP", Err: err}
	}

	// Ignore close errors after handleResponse consumes the response body.
	defer drainClose(resp)

	var ip IPAddress
	if err := c.handleResponse(resp, &ip); err != nil {
		return nil, err
	}

	return &ip, nil
}

// AllocateInstanceIPProto allocates an instance IP and returns the proto
// IPAddress element. The POST is non-idempotent, so it is not retried.
func (c *Client) AllocateInstanceIPProto(ctx context.Context, linodeID int, req AllocateIPRequest) (*linodev1.IPAddress, error) {
	var ipAddr *linodev1.IPAddress

	err := c.executeWithoutRetry(ctx, "AllocateInstanceIP", func() error {
		var err error

		ipAddr, err = c.httpAllocateInstanceIPProto(ctx, linodeID, req)

		return err
	})

	return ipAddr, err
}

// httpAllocateInstanceIPProto allocates an instance IP and decodes the response
// into the proto IPAddress element.
func (c *Client) httpAllocateInstanceIPProto(ctx context.Context, linodeID int, req AllocateIPRequest) (*linodev1.IPAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/ips", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AllocateInstanceIP", Err: err}
	}

	defer drainClose(resp)

	ip := &linodev1.IPAddress{}
	if err := c.handleProtoResponse(resp, ip); err != nil {
		return nil, err
	}

	return ip, nil
}

// UpdateInstanceIPProto updates an instance IP's RDNS and returns the proto
// IPAddress element.
func (c *Client) UpdateInstanceIPProto(ctx context.Context, linodeID int, address string, req UpdateIPRDNSRequest) (*linodev1.IPAddress, error) {
	var ipAddr *linodev1.IPAddress

	err := c.executeWithRetry(ctx, "UpdateInstanceIP", func() error {
		var retryErr error

		ipAddr, retryErr = c.httpUpdateInstanceIPProto(ctx, linodeID, address, req)

		return retryErr
	})

	return ipAddr, err
}

// httpUpdateInstanceIPProto updates an instance IP's RDNS and decodes the
// response into the proto IPAddress element.
func (c *Client) httpUpdateInstanceIPProto(ctx context.Context, linodeID int, address string, req UpdateIPRDNSRequest) (*linodev1.IPAddress, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/ips/%s", linodeID, url.PathEscape(address))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceIP", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/ips/%s", linodeID, url.PathEscape(address))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteInstanceIP", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// CloneInstance clones a Linode instance.
func (c *Client) httpCloneInstance(ctx context.Context, linodeID int, req *CloneInstanceRequest) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/clone", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CloneInstance", Err: err}
	}

	defer drainClose(resp)

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// httpCloneInstanceProto clones a Linode instance and decodes the response as a
// proto message for the proto-backed write path.
func (c *Client) httpCloneInstanceProto(ctx context.Context, linodeID int, req *CloneInstanceRequest) (*linodev1.Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/clone", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CloneInstance", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/migrate", linodeID)

	var payload any
	if region != "" {
		payload = map[string]string{"region": region}
	}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return &NetworkError{Operation: "MigrateInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpMutateInstance upgrades a Linode instance to the latest generation type.
func (c *Client) httpMutateInstance(ctx context.Context, linodeID int, req *MutateInstanceRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/mutate", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "MutateInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

func (c *Client) httpRebuildInstance(ctx context.Context, linodeID int, req *RebuildInstanceRequest) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/rebuild", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "RebuildInstance", Err: err}
	}

	defer drainClose(resp)

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// RescueInstance boots a Linode instance into rescue mode.
func (c *Client) httpRescueInstance(ctx context.Context, linodeID int, req RescueInstanceRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/rescue", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "RescueInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ResetInstancePassword resets the root password on a Linode instance.
func (c *Client) httpResetInstancePassword(ctx context.Context, linodeID int, rootPass string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/password", linodeID)

	payload := map[string]string{"root_pass": rootPass}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return &NetworkError{Operation: "ResetInstancePassword", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCreateInstanceConfigProto creates a configuration profile and decodes the
// response into the proto element so the tool emits proto-canonical output.
func (c *Client) httpCreateInstanceConfigProto(ctx context.Context, linodeID int, req *CreateConfigRequest) (*linodev1.InstanceConfig, error) {
	if linodeID <= 0 {
		return nil, ErrLinodeIDPositive
	}

	if req == nil {
		return nil, ErrCreateConfigRequestRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedLinodeID := url.PathEscape(strconv.Itoa(linodeID))
	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%s/configs", encodedLinodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstanceConfig", Err: err}
	}

	defer drainClose(resp)

	config := &linodev1.InstanceConfig{}
	if err := c.handleProtoResponse(resp, config); err != nil {
		return nil, err
	}

	return config, nil
}

// httpUpdateInstanceConfigProto updates a configuration profile and decodes the
// response into the proto element.
func (c *Client) httpUpdateInstanceConfigProto(ctx context.Context, linodeID, configID int, req *UpdateConfigRequest) (*linodev1.InstanceConfig, error) {
	if err := validateInstanceConfigMutation(linodeID, configID, req == nil, ErrUpdateConfigRequestRequired); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := instanceConfigEndpoint(linodeID, configID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceConfig", Err: err}
	}

	defer drainClose(resp)

	config := &linodev1.InstanceConfig{}
	if err := c.handleProtoResponse(resp, config); err != nil {
		return nil, err
	}

	return config, nil
}

// httpAddInstanceConfigInterfaceProto appends a network interface to a
// configuration profile and decodes the response into the proto element.
func (c *Client) httpAddInstanceConfigInterfaceProto(ctx context.Context, linodeID, configID int, req *ConfigInterface) (*linodev1.ConfigInterfaceResponse, error) {
	if err := validateInstanceConfigMutation(linodeID, configID, req == nil, ErrAddConfigInterfaceRequestRequired); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := instanceConfigEndpoint(linodeID, configID) + "/interfaces"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AddInstanceConfigInterface", Err: err}
	}

	defer drainClose(resp)

	configInterface := &linodev1.ConfigInterfaceResponse{}
	if err := c.handleProtoResponse(resp, configInterface); err != nil {
		return nil, err
	}

	return configInterface, nil
}

// httpUpdateInstanceConfigInterfaceProto updates a configuration profile
// interface and decodes the response into the proto element.
func (c *Client) httpUpdateInstanceConfigInterfaceProto(ctx context.Context, linodeID, configID, interfaceID int, req *UpdateConfigInterfaceRequest) (*linodev1.ConfigInterfaceResponse, error) {
	if err := validateInstanceConfigMutation(linodeID, configID, req == nil, ErrUpdateConfigInterfaceRequestRequired); err != nil {
		return nil, err
	}

	if interfaceID <= 0 {
		return nil, ErrInterfaceIDPositive
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	encodedInterfaceID := url.PathEscape(strconv.Itoa(interfaceID))
	endpoint := instanceConfigEndpoint(linodeID, configID) + "/interfaces/" + encodedInterfaceID

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceConfigInterface", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstanceDisk", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks/%d", linodeID, diskID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateInstanceDisk", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/disks/%d/clone", linodeID, diskID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "CloneInstanceDisk", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/backups", linodeID)

	var body any
	if label != "" {
		body = CreateInstanceBackupRequest{Label: label}
	}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstanceBackup", Err: err}
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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/rebuild", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "RebuildInstance", Err: err}
	}

	defer drainClose(resp)

	instance := &linodev1.Instance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}
