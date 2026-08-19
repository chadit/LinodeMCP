package linode

import (
	"context"
)

// httpGetVolume retrieves a single volume by its ID.
func (c *Client) httpGetVolume(ctx context.Context, volumeID int) (*Volume, error) {
	return routedGet[Volume](ctx, c, "GetVolume", "linode_volume_get", volumeID)
}

// httpGetSSHKey retrieves a single SSH key by its ID.
func (c *Client) httpGetSSHKey(ctx context.Context, sshKeyID int) (*SSHKey, error) {
	return routedGet[SSHKey](ctx, c, "GetSSHKey", "linode_sshkey_get", sshKeyID)
}
