package linode

import (
	"context"
	"fmt"
	"net/http"
)

const (
	endpointVolumes = "/volumes"
	endpointSSHKeys = "/profile/sshkeys"
)

// ListVolumes retrieves all block storage volumes for the authenticated user.
func (c *Client) httpListVolumes(ctx context.Context) ([]Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointVolumes, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVolumes", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[Volume]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetVolume retrieves a single volume by its ID.
func (c *Client) httpGetVolume(ctx context.Context, volumeID int) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// CreateVolume creates a new block storage volume.
func (c *Client) httpCreateVolume(ctx context.Context, req *CreateVolumeRequest) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointVolumes, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// AttachVolume attaches a volume to a Linode instance.
func (c *Client) httpAttachVolume(ctx context.Context, volumeID int, req AttachVolumeRequest) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d/attach", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AttachVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// DetachVolume detaches a volume from a Linode instance.
func (c *Client) httpDetachVolume(ctx context.Context, volumeID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d/detach", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DetachVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ResizeVolume resizes a volume to a larger size.
func (c *Client) httpResizeVolume(ctx context.Context, volumeID, size int) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d/resize", volumeID)
	payload := map[string]int{"size": size}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, &NetworkError{Operation: "ResizeVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// DeleteVolume deletes a block storage volume.
func (c *Client) httpDeleteVolume(ctx context.Context, volumeID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ListSSHKeys retrieves all SSH keys from the authenticated user's profile.
func (c *Client) httpListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointSSHKeys, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListSSHKeys", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[SSHKey]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// CreateSSHKey creates a new SSH key in the user's profile.
func (c *Client) httpCreateSSHKey(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointSSHKeys, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSSHKey", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var sshKey SSHKey
	if err := c.handleResponse(resp, &sshKey); err != nil {
		return nil, err
	}

	return &sshKey, nil
}

// DeleteSSHKey deletes an SSH key from the user's profile.
func (c *Client) httpDeleteSSHKey(ctx context.Context, sshKeyID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointSSHKeys+"/%d", sshKeyID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteSSHKey", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}
