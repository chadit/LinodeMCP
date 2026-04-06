package linode

// InstanceBackup represents a detailed backup of a Linode instance.
type InstanceBackup struct {
	ID        int                  `json:"id"`
	Label     string               `json:"label"`
	Status    string               `json:"status"`
	Type      string               `json:"type"`
	Created   string               `json:"created"`
	Updated   string               `json:"updated"`
	Finished  string               `json:"finished"`
	Region    string               `json:"region"`
	Available bool                 `json:"available"`
	Configs   []string             `json:"configs"`
	Disks     []InstanceBackupDisk `json:"disks"`
}

// InstanceBackupDisk represents a disk within an instance backup.
type InstanceBackupDisk struct {
	Label      string `json:"label"`
	Size       int    `json:"size"`
	Filesystem string `json:"filesystem"`
}

// InstanceBackupsResponse represents the response from the instance backups endpoint.
// The API returns automatic backups as an array and snapshots as a nested object.
type InstanceBackupsResponse struct {
	Automatic []InstanceBackup        `json:"automatic"`
	Snapshot  InstanceBackupSnapshots `json:"snapshot"`
}

// InstanceBackupSnapshots holds the current and in-progress snapshot references.
type InstanceBackupSnapshots struct {
	Current    *InstanceBackup `json:"current"`
	InProgress *InstanceBackup `json:"in_progress"`
}

// RestoreBackupRequest represents the request body for restoring an instance backup.
type RestoreBackupRequest struct {
	LinodeID  int  `json:"linode_id"`
	Overwrite bool `json:"overwrite"`
}

// InstanceDisk represents a disk attached to a Linode instance.
type InstanceDisk struct {
	ID         int    `json:"id"`
	Label      string `json:"label"`
	Status     string `json:"status"`
	Size       int    `json:"size"`
	Filesystem string `json:"filesystem"`
	Created    string `json:"created"`
	Updated    string `json:"updated"`
}

// CreateDiskRequest represents the request body for creating an instance disk.
type CreateDiskRequest struct {
	Label           string   `json:"label"`
	Size            int      `json:"size"`
	Filesystem      string   `json:"filesystem,omitempty"`
	Image           string   `json:"image,omitempty"`
	RootPass        string   `json:"root_pass,omitempty"`
	AuthorizedKeys  []string `json:"authorized_keys,omitempty"`
	AuthorizedUsers []string `json:"authorized_users,omitempty"`
}

// UpdateDiskRequest represents the request body for updating an instance disk.
type UpdateDiskRequest struct {
	Label string `json:"label"`
}

// ResizeDiskRequest represents the request body for resizing an instance disk.
type ResizeDiskRequest struct {
	Size int `json:"size"`
}

// InstanceIPAddresses represents the full IP address configuration for an instance.
type InstanceIPAddresses struct {
	IPv4 *InstanceIPv4 `json:"ipv4"`
	IPv6 *InstanceIPv6 `json:"ipv6"`
}

// InstanceIPv4 holds the IPv4 address categories for an instance.
type InstanceIPv4 struct {
	Public   []IPAddress `json:"public"`
	Private  []IPAddress `json:"private"`
	Shared   []IPAddress `json:"shared"`
	Reserved []IPAddress `json:"reserved"`
}

// InstanceIPv6 holds the IPv6 address information for an instance.
type InstanceIPv6 struct {
	SLAAC     *IPv6SLAAC  `json:"slaac"`
	LinkLocal *IPv6SLAAC  `json:"link_local"`
	Global    []IPv6Range `json:"global"`
	Ranges    []IPv6Range `json:"ranges,omitempty"`
	Pools     []IPv6Range `json:"pools,omitempty"`
}

// IPAddress represents an IPv4 address assigned to a Linode instance.
type IPAddress struct {
	Address    string  `json:"address"`
	Gateway    string  `json:"gateway"`
	SubnetMask string  `json:"subnet_mask"`
	Prefix     int     `json:"prefix"`
	Type       string  `json:"type"`
	Public     bool    `json:"public"`
	RDNS       string  `json:"rdns"`
	LinodeID   int     `json:"linode_id"`
	Region     string  `json:"region"`
	VPCNAT1To1 *string `json:"vpc_nat_1_1,omitempty"`
}

// IPv6SLAAC represents an IPv6 SLAAC or link-local address.
type IPv6SLAAC struct {
	Address    string `json:"address"`
	Gateway    string `json:"gateway"`
	SubnetMask string `json:"subnet_mask"`
	Prefix     int    `json:"prefix"`
	Type       string `json:"type"`
	RDNS       string `json:"rdns"`
	Region     string `json:"region"`
}

// IPv6Range represents an IPv6 range or pool assigned to an instance.
type IPv6Range struct {
	Range       string `json:"range"`
	Region      string `json:"region"`
	Prefix      int    `json:"prefix"`
	RouteTarget string `json:"route_target"`
}

// AllocateIPRequest represents the request body for allocating an IP address to an instance.
type AllocateIPRequest struct {
	Type   string `json:"type"`
	Public bool   `json:"public"`
}

// CloneInstanceRequest represents the request body for cloning a Linode instance.
type CloneInstanceRequest struct {
	Region         string `json:"region,omitempty"`
	Type           string `json:"type,omitempty"`
	Label          string `json:"label,omitempty"`
	Group          string `json:"group,omitempty"`
	BackupsEnabled bool   `json:"backups_enabled,omitempty"`
	Disks          []int  `json:"disks,omitempty"`
	Configs        []int  `json:"configs,omitempty"`
}

// RebuildInstanceRequest represents the request body for rebuilding a Linode instance.
type RebuildInstanceRequest struct {
	Image           string   `json:"image"`
	RootPass        string   `json:"root_pass"`
	AuthorizedKeys  []string `json:"authorized_keys,omitempty"`
	AuthorizedUsers []string `json:"authorized_users,omitempty"`
	Booted          *bool    `json:"booted,omitempty"`
}

// RescueInstanceRequest represents the request body for booting an instance into rescue mode.
type RescueInstanceRequest struct {
	Devices map[string]*RescueDeviceAssignment `json:"devices"`
}

// RescueDeviceAssignment represents a device assignment for rescue mode.
type RescueDeviceAssignment struct {
	DiskID   *int `json:"disk_id,omitempty"`
	VolumeID *int `json:"volume_id,omitempty"`
}
