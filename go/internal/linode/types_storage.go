package linode

// Volume represents a Linode block storage volume.
type Volume struct {
	ID             int      `json:"id"`
	Label          string   `json:"label"`
	Status         string   `json:"status"`
	Size           int      `json:"size"`
	Region         string   `json:"region"`
	LinodeID       *int     `json:"linode_id"`
	LinodeLabel    *string  `json:"linode_label"`
	FilesystemPath string   `json:"filesystem_path"`
	Tags           []string `json:"tags"`
	Created        string   `json:"created"`
	Updated        string   `json:"updated"`
	HardwareType   string   `json:"hardware_type"`
}

// SSHKey represents an SSH key in a user's profile.
type SSHKey struct {
	ID      int    `json:"id"`
	Label   string `json:"label"`
	SSHKey  string `json:"ssh_key"`
	Created string `json:"created"`
}

// CreateVolumeRequest represents the request body for creating a volume.
type CreateVolumeRequest struct {
	Label    string   `json:"label"`
	Region   string   `json:"region,omitempty"`
	Size     int      `json:"size,omitempty"`
	LinodeID *int     `json:"linode_id,omitempty"`
	ConfigID *int     `json:"config_id,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// AttachVolumeRequest represents the request body for attaching a volume to a Linode.
type AttachVolumeRequest struct {
	LinodeID           int  `json:"linode_id"`
	ConfigID           *int `json:"config_id,omitempty"`
	PersistAcrossBoots bool `json:"persist_across_boots,omitempty"`
}

// CreateSSHKeyRequest represents the request body for creating an SSH key.
type CreateSSHKeyRequest struct {
	Label  string `json:"label"`
	SSHKey string `json:"ssh_key"`
}
