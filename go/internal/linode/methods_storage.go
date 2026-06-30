package linode

import (
	"context"
	"fmt"
	"net/http"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const (
	endpointVolumes     = "/volumes"
	endpointVolumeTypes = "/volumes/types"
	endpointSSHKeys     = "/profile/sshkeys"
)

// ListVolumes retrieves all block storage volumes for the authenticated user.
func (c *Client) httpListVolumes(ctx context.Context) ([]Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointVolumes, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVolumes", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[Volume]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListVolumesProto retrieves all block storage volumes as proto messages,
// decoded directly from the API JSON for the proto-backed read path.
func (c *Client) httpListVolumesProto(ctx context.Context) ([]*linodev1.Volume, error) {
	return listProtoElements(ctx, c, "ListVolumes", endpointVolumes,
		func() *linodev1.Volume { return &linodev1.Volume{} })
}

// ListVolumeTypes retrieves all block storage volume types.
func (c *Client) httpListVolumeTypes(ctx context.Context) ([]VolumeType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointVolumeTypes, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVolumeTypes", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[VolumeType]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListVolumeTypesProto retrieves all block storage volume types as proto
// messages, decoded directly from the API JSON for the proto-backed list path.
func (c *Client) httpListVolumeTypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	return listProtoElements(ctx, c, "ListVolumeTypes", endpointVolumeTypes,
		func() *linodev1.LinodeType { return &linodev1.LinodeType{} })
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

	defer drainClose(resp)

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// httpGetVolumeProto retrieves a single volume and decodes it as a proto message
// for the proto-backed read path.
func (c *Client) httpGetVolumeProto(ctx context.Context, volumeID int) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetVolume", Err: err}
	}

	defer drainClose(resp)

	volume := &linodev1.Volume{}
	if err := c.handleProtoResponse(resp, volume); err != nil {
		return nil, err
	}

	return volume, nil
}

// CreateVolume creates a new block storage volume.
func (c *Client) httpCreateVolume(ctx context.Context, req *CreateVolumeRequest) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointVolumes, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateVolume", Err: err}
	}

	defer drainClose(resp)

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// httpCreateVolumeProto creates a volume and decodes the response as a proto
// message for the proto-backed write path.
func (c *Client) httpCreateVolumeProto(ctx context.Context, req *CreateVolumeRequest) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointVolumes, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateVolume", Err: err}
	}

	defer drainClose(resp)

	volume := &linodev1.Volume{}
	if err := c.handleProtoResponse(resp, volume); err != nil {
		return nil, err
	}

	return volume, nil
}

// CloneVolume clones an existing block storage volume.
func (c *Client) httpCloneVolume(ctx context.Context, volumeID int, req CloneVolumeRequest) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d/clone", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CloneVolume", Err: err}
	}

	defer drainClose(resp)

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// httpCloneVolumeProto clones a volume and decodes the response as a proto
// message for the proto-backed write path.
func (c *Client) httpCloneVolumeProto(ctx context.Context, volumeID int, req CloneVolumeRequest) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d/clone", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CloneVolume", Err: err}
	}

	defer drainClose(resp)

	volume := &linodev1.Volume{}
	if err := c.handleProtoResponse(resp, volume); err != nil {
		return nil, err
	}

	return volume, nil
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

	defer drainClose(resp)

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// httpAttachVolumeProto attaches a volume and decodes the response as a proto
// message for the proto-backed write path.
func (c *Client) httpAttachVolumeProto(ctx context.Context, volumeID int, req AttachVolumeRequest) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d/attach", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AttachVolume", Err: err}
	}

	defer drainClose(resp)

	volume := &linodev1.Volume{}
	if err := c.handleProtoResponse(resp, volume); err != nil {
		return nil, err
	}

	return volume, nil
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

	defer drainClose(resp)

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

	defer drainClose(resp)

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// httpResizeVolumeProto resizes a volume and decodes the response as a proto
// message for the proto-backed write path.
func (c *Client) httpResizeVolumeProto(ctx context.Context, volumeID, size int) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d/resize", volumeID)
	payload := map[string]int{"size": size}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, &NetworkError{Operation: "ResizeVolume", Err: err}
	}

	defer drainClose(resp)

	volume := &linodev1.Volume{}
	if err := c.handleProtoResponse(resp, volume); err != nil {
		return nil, err
	}

	return volume, nil
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

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// UpdateVolume updates a volume's label or tags.
func (c *Client) httpUpdateVolume(ctx context.Context, volumeID int, req *UpdateVolumeRequest) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateVolume", Err: err}
	}

	defer drainClose(resp)

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// httpUpdateVolumeProto updates a volume and decodes the response as a proto
// message for the proto-backed write path.
func (c *Client) httpUpdateVolumeProto(ctx context.Context, volumeID int, req *UpdateVolumeRequest) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointVolumes+"/%d", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateVolume", Err: err}
	}

	defer drainClose(resp)

	volume := &linodev1.Volume{}
	if err := c.handleProtoResponse(resp, volume); err != nil {
		return nil, err
	}

	return volume, nil
}

// ListSSHKeys retrieves all SSH keys from the authenticated user's profile.
func (c *Client) httpListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointSSHKeys, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListSSHKeys", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[SSHKey]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListSSHKeysProto retrieves all SSH keys as proto messages for the
// proto-backed list path, sharing the decode tail with every other proto list.
func (c *Client) httpListSSHKeysProto(ctx context.Context) ([]*linodev1.SSHKey, error) {
	return listProtoElements(ctx, c, "ListSSHKeys", endpointSSHKeys,
		func() *linodev1.SSHKey { return &linodev1.SSHKey{} })
}

// GetSSHKey retrieves a single SSH key by its ID.
func (c *Client) httpGetSSHKey(ctx context.Context, sshKeyID int) (*SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointSSHKeys+"/%d", sshKeyID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetSSHKey", Err: err}
	}

	defer drainClose(resp)

	var sshKey SSHKey
	if err := c.handleResponse(resp, &sshKey); err != nil {
		return nil, err
	}

	return &sshKey, nil
}

// httpGetSSHKeyProto retrieves one SSH key as a proto message.
func (c *Client) httpGetSSHKeyProto(ctx context.Context, sshKeyID int) (*linodev1.SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointSSHKeys+"/%d", sshKeyID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetSSHKey", Err: err}
	}

	defer drainClose(resp)

	sshKey := &linodev1.SSHKey{}
	if err := c.handleProtoResponse(resp, sshKey); err != nil {
		return nil, err
	}

	return sshKey, nil
}

// CreateSSHKey creates a new SSH key in the user's profile.
func (c *Client) httpCreateSSHKey(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointSSHKeys, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSSHKey", Err: err}
	}

	defer drainClose(resp)

	var sshKey SSHKey
	if err := c.handleResponse(resp, &sshKey); err != nil {
		return nil, err
	}

	return &sshKey, nil
}

// httpCreateSSHKeyProto creates an SSH key and decodes the created key into a
// proto message for the proto-backed write path.
func (c *Client) httpCreateSSHKeyProto(ctx context.Context, req CreateSSHKeyRequest) (*linodev1.SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointSSHKeys, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSSHKey", Err: err}
	}

	defer drainClose(resp)

	sshKey := &linodev1.SSHKey{}
	if err := c.handleProtoResponse(resp, sshKey); err != nil {
		return nil, err
	}

	return sshKey, nil
}

// httpUpdateSSHKeyProto updates an SSH key and decodes the updated key into a
// proto message for the proto-backed write path.
func (c *Client) httpUpdateSSHKeyProto(ctx context.Context, sshKeyID int, req UpdateSSHKeyRequest) (*linodev1.SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointSSHKeys+"/%d", sshKeyID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateSSHKey", Err: err}
	}

	defer drainClose(resp)

	sshKey := &linodev1.SSHKey{}
	if err := c.handleProtoResponse(resp, sshKey); err != nil {
		return nil, err
	}

	return sshKey, nil
}

// UpdateSSHKey updates an SSH key in the user's profile.
func (c *Client) httpUpdateSSHKey(ctx context.Context, sshKeyID int, req UpdateSSHKeyRequest) (*SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointSSHKeys+"/%d", sshKeyID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateSSHKey", Err: err}
	}

	// The response body is fully consumed by handleResponse; close errors do not change the API result.
	defer drainClose(resp)

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

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}
