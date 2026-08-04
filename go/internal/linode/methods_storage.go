package linode

import (
	"context"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// Methods with a Proto suffix decode the API JSON straight into proto messages
// used by the proto-backed read and write paths.

// httpListVolumesProto retrieves one page of block storage volumes.
func (c *Client) httpListVolumesProto(ctx context.Context, page, pageSize int) ([]*linodev1.Volume, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListVolumes",
		"linode_volume_list", "", nil, page, pageSize,
		func() *linodev1.Volume { return &linodev1.Volume{} })
}

// httpListVolumeTypesProto retrieves all block storage volume types.
func (c *Client) httpListVolumeTypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	return listProtoElementsRouted(ctx, c, "ListVolumeTypes",
		"linode_volume_type_list", "", nil,
		func() *linodev1.LinodeType { return &linodev1.LinodeType{} })
}

// GetVolume retrieves a single volume by its ID.
func (c *Client) httpGetVolume(ctx context.Context, volumeID int) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_volume_get", nil, volumeID)
	if err != nil {
		return nil, wrapRequestError("GetVolume", err)
	}

	defer drainClose(resp)

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// httpGetVolumeProto retrieves a single volume by its ID.
func (c *Client) httpGetVolumeProto(ctx context.Context, volumeID int) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_volume_get", nil, volumeID)
	if err != nil {
		return nil, wrapRequestError("GetVolume", err)
	}

	defer drainClose(resp)

	volume := &linodev1.Volume{}
	if err := c.handleProtoResponse(resp, volume); err != nil {
		return nil, err
	}

	return volume, nil
}

// httpCreateVolumeProto creates a block storage volume.
func (c *Client) httpCreateVolumeProto(ctx context.Context, req *CreateVolumeRequest) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_volume_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateVolume", err)
	}

	defer drainClose(resp)

	volume := &linodev1.Volume{}
	if err := c.handleProtoResponse(resp, volume); err != nil {
		return nil, err
	}

	return volume, nil
}

// httpCloneVolumeProto clones a volume into a new one.
func (c *Client) httpCloneVolumeProto(ctx context.Context, volumeID int, req CloneVolumeRequest) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_volume_clone", req, volumeID)
	if err != nil {
		return nil, wrapRequestError("CloneVolume", err)
	}

	defer drainClose(resp)

	volume := &linodev1.Volume{}
	if err := c.handleProtoResponse(resp, volume); err != nil {
		return nil, err
	}

	return volume, nil
}

// httpAttachVolumeProto attaches a volume and decodes the response as a proto
// message for the proto-backed write path.
func (c *Client) httpAttachVolumeProto(ctx context.Context, volumeID int, req AttachVolumeRequest) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_volume_attach", req, volumeID)
	if err != nil {
		return nil, wrapRequestError("AttachVolume", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_volume_detach", nil, volumeID)
	if err != nil {
		return wrapRequestError("DetachVolume", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpResizeVolumeProto resizes a volume and decodes the response as a proto
// message for the proto-backed write path.
func (c *Client) httpResizeVolumeProto(ctx context.Context, volumeID, size int) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	payload := map[string]int{"size": size}

	resp, err := c.makeRouteRequest(ctx, "linode_volume_resize", payload, volumeID)
	if err != nil {
		return nil, wrapRequestError("ResizeVolume", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_volume_delete", nil, volumeID)
	if err != nil {
		return wrapRequestError("DeleteVolume", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpUpdateVolumeProto updates a volume and decodes the response as a proto
// message for the proto-backed write path.
func (c *Client) httpUpdateVolumeProto(ctx context.Context, volumeID int, req *UpdateVolumeRequest) (*linodev1.Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_volume_update", req, volumeID)
	if err != nil {
		return nil, wrapRequestError("UpdateVolume", err)
	}

	defer drainClose(resp)

	volume := &linodev1.Volume{}
	if err := c.handleProtoResponse(resp, volume); err != nil {
		return nil, err
	}

	return volume, nil
}

// httpListSSHKeysProto retrieves one page of SSH keys as proto messages for the
// proto-backed list path, sharing the decode tail with every other proto list.
func (c *Client) httpListSSHKeysProto(ctx context.Context, page, pageSize int) ([]*linodev1.SSHKey, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListSSHKeys",
		"linode_sshkey_list", "", nil, page, pageSize,
		func() *linodev1.SSHKey { return &linodev1.SSHKey{} })
}

// GetSSHKey retrieves a single SSH key by its ID.
func (c *Client) httpGetSSHKey(ctx context.Context, sshKeyID int) (*SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_sshkey_get", nil, sshKeyID)
	if err != nil {
		return nil, wrapRequestError("GetSSHKey", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_sshkey_get", nil, sshKeyID)
	if err != nil {
		return nil, wrapRequestError("GetSSHKey", err)
	}

	defer drainClose(resp)

	sshKey := &linodev1.SSHKey{}
	if err := c.handleProtoResponse(resp, sshKey); err != nil {
		return nil, err
	}

	return sshKey, nil
}

// httpCreateSSHKeyProto creates an SSH key and decodes the created key into a
// proto message for the proto-backed write path.
func (c *Client) httpCreateSSHKeyProto(ctx context.Context, req CreateSSHKeyRequest) (*linodev1.SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_sshkey_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateSSHKey", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_sshkey_update", req, sshKeyID)
	if err != nil {
		return nil, wrapRequestError("UpdateSSHKey", err)
	}

	defer drainClose(resp)

	sshKey := &linodev1.SSHKey{}
	if err := c.handleProtoResponse(resp, sshKey); err != nil {
		return nil, err
	}

	return sshKey, nil
}

// DeleteSSHKey deletes an SSH key from the user's profile.
func (c *Client) httpDeleteSSHKey(ctx context.Context, sshKeyID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_sshkey_delete", nil, sshKeyID)
	if err != nil {
		return wrapRequestError("DeleteSSHKey", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}
