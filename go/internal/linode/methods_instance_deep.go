package linode

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

	defer func() { _ = resp.Body.Close() }()

	var backups InstanceBackupsResponse
	if err := c.handleResponse(resp, &backups); err != nil {
		return nil, err
	}

	return &backups, nil
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

	defer func() { _ = resp.Body.Close() }()

	var backup InstanceBackup
	if err := c.handleResponse(resp, &backup); err != nil {
		return nil, err
	}

	return &backup, nil
}

// CreateInstanceBackup creates a manual snapshot for a Linode instance.
func (c *Client) httpCreateInstanceBackup(ctx context.Context, linodeID int) (*InstanceBackup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/backups", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstanceBackup", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
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

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[InstanceDisk]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
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

	defer func() { _ = resp.Body.Close() }()

	var disk InstanceDisk
	if err := c.handleResponse(resp, &disk); err != nil {
		return nil, err
	}

	return &disk, nil
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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/ips/%s", linodeID, url.PathEscape(address))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstanceIP", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var ip IPAddress
	if err := c.handleResponse(resp, &ip); err != nil {
		return nil, err
	}

	return &ip, nil
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

	defer func() { _ = resp.Body.Close() }()

	var ip IPAddress
	if err := c.handleResponse(resp, &ip); err != nil {
		return nil, err
	}

	return &ip, nil
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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
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

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// RebuildInstance rebuilds a Linode instance with a new image.
func (c *Client) httpRebuildInstance(ctx context.Context, linodeID int, req *RebuildInstanceRequest) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointInstanceDeep+"/%d/rebuild", linodeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "RebuildInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}
