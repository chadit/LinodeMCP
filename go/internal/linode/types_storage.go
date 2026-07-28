package linode

// Volume represents a Linode block storage volume.
type Volume struct {
	LinodeID       *int     `json:"linode_id"`
	LinodeLabel    *string  `json:"linode_label"`
	Label          string   `json:"label"`
	Status         string   `json:"status"`
	Region         string   `json:"region"`
	FilesystemPath string   `json:"filesystem_path"`
	Created        string   `json:"created"`
	Updated        string   `json:"updated"`
	HardwareType   string   `json:"hardware_type"`
	Tags           []string `json:"tags"`
	ID             int      `json:"id"`
	Size           int      `json:"size"`
}

// VolumeType represents a Linode block storage volume type.
type VolumeType map[string]any

// SSHKey represents an SSH key in a user's profile.
type SSHKey struct {
	Label   string `json:"label"`
	SSHKey  string `json:"ssh_key"`
	Created string `json:"created"`
	ID      int    `json:"id"`
}

// CreateVolumeRequest represents the request body for creating a volume.
type CreateVolumeRequest struct {
	LinodeID *int     `json:"linode_id,omitempty"`
	ConfigID *int     `json:"config_id,omitempty"`
	Label    string   `json:"label"`
	Region   string   `json:"region,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Size     int      `json:"size,omitempty"`
}

// CloneVolumeRequest represents the request body for cloning a volume.
type CloneVolumeRequest struct {
	Label string `json:"label"`
}

// AttachVolumeRequest represents the request body for attaching a volume to a Linode.
type AttachVolumeRequest struct {
	ConfigID           *int `json:"config_id,omitempty"`
	LinodeID           int  `json:"linode_id"`
	PersistAcrossBoots bool `json:"persist_across_boots,omitempty"`
}

// UpdateVolumeRequest represents the request body for updating a volume.
type UpdateVolumeRequest struct {
	Label *string  `json:"label,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

// CreateSSHKeyRequest represents the request body for creating an SSH key.
type CreateSSHKeyRequest struct {
	Label  string `json:"label"`
	SSHKey string `json:"ssh_key"`
}

// UpdateSSHKeyRequest represents the request body for updating an SSH key.
type UpdateSSHKeyRequest struct {
	Label string `json:"label"`
}
