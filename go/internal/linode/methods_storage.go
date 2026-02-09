package linode

import (
	"context"
	"fmt"
	"net/http"
)

// ListVolumes retrieves all block storage volumes for the authenticated user.
func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/volumes", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVolumes", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Volume `json:"data"`
		Page    int      `json:"page"`
		Pages   int      `json:"pages"`
		Results int      `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetVolume retrieves a single volume by its ID.
func (c *Client) GetVolume(ctx context.Context, volumeID int) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/volumes/%d", volumeID)

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
func (c *Client) CreateVolume(ctx context.Context, req CreateVolumeRequest) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/volumes", req)
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
func (c *Client) AttachVolume(ctx context.Context, volumeID int, req AttachVolumeRequest) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/volumes/%d/attach", volumeID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, req)
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
func (c *Client) DetachVolume(ctx context.Context, volumeID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/volumes/%d/detach", volumeID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DetachVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ResizeVolume resizes a volume to a larger size.
func (c *Client) ResizeVolume(ctx context.Context, volumeID int, size int) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/volumes/%d/resize", volumeID)
	payload := map[string]int{"size": size}

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, payload)
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
func (c *Client) DeleteVolume(ctx context.Context, volumeID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/volumes/%d", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ListSSHKeys retrieves all SSH keys from the authenticated user's profile.
func (c *Client) ListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/profile/sshkeys", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListSSHKeys", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []SSHKey `json:"data"`
		Page    int      `json:"page"`
		Pages   int      `json:"pages"`
		Results int      `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// CreateSSHKey creates a new SSH key in the user's profile.
func (c *Client) CreateSSHKey(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/profile/sshkeys", req)
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
func (c *Client) DeleteSSHKey(ctx context.Context, sshKeyID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/profile/sshkeys/%d", sshKeyID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteSSHKey", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}
