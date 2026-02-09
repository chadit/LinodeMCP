package linode

// Volume represents a Linode block storage volume.
type Volume struct {
	ID             int      `json:"id"`
	Label          string   `json:"label"`
	Status         string   `json:"status"`
	Size           int      `json:"size"`
	Region         string   `json:"region"`
	LinodeID       *int     `json:"linode_id"`       //nolint:tagliatelle // Linode API snake_case
	LinodeLabel    *string  `json:"linode_label"`    //nolint:tagliatelle // Linode API snake_case
	FilesystemPath string   `json:"filesystem_path"` //nolint:tagliatelle // Linode API snake_case
	Tags           []string `json:"tags"`
	Created        string   `json:"created"`
	Updated        string   `json:"updated"`
	HardwareType   string   `json:"hardware_type"` //nolint:tagliatelle // Linode API snake_case
}

// SSHKey represents an SSH key in a user's profile.
type SSHKey struct {
	ID      int    `json:"id"`
	Label   string `json:"label"`
	SSHKey  string `json:"ssh_key"` //nolint:tagliatelle // Linode API snake_case
	Created string `json:"created"`
}

// CreateVolumeRequest represents the request body for creating a volume.
type CreateVolumeRequest struct {
	Label    string   `json:"label"`
	Region   string   `json:"region,omitempty"`
	Size     int      `json:"size,omitempty"`
	LinodeID *int     `json:"linode_id,omitempty"` //nolint:tagliatelle // Linode API snake_case
	ConfigID *int     `json:"config_id,omitempty"` //nolint:tagliatelle // Linode API snake_case
	Tags     []string `json:"tags,omitempty"`
}

// AttachVolumeRequest represents the request body for attaching a volume to a Linode.
type AttachVolumeRequest struct {
	LinodeID           int  `json:"linode_id"`                      //nolint:tagliatelle // Linode API snake_case
	ConfigID           *int `json:"config_id,omitempty"`            //nolint:tagliatelle // Linode API snake_case
	PersistAcrossBoots bool `json:"persist_across_boots,omitempty"` //nolint:tagliatelle // Linode API snake_case
}

// CreateSSHKeyRequest represents the request body for creating an SSH key.
type CreateSSHKeyRequest struct {
	Label  string `json:"label"`
	SSHKey string `json:"ssh_key"` //nolint:tagliatelle // Linode API snake_case
}
