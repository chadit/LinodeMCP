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

// InstanceConfig represents a Linode configuration profile.
type InstanceConfig struct {
	ID          int                       `json:"id"`
	Label       string                    `json:"label"`
	Kernel      string                    `json:"kernel,omitempty"`
	Comments    string                    `json:"comments,omitempty"`
	MemoryLimit int                       `json:"memory_limit,omitempty"`
	RootDevice  string                    `json:"root_device,omitempty"`
	RunLevel    string                    `json:"run_level,omitempty"`
	VirtMode    string                    `json:"virt_mode,omitempty"`
	Devices     map[string]*ConfigDevice  `json:"devices,omitempty"`
	Helpers     *ConfigHelpers            `json:"helpers,omitempty"`
	Interfaces  []ConfigInterfaceResponse `json:"interfaces,omitempty"`
	Created     string                    `json:"created,omitempty"`
	Updated     string                    `json:"updated,omitempty"`
}

// ConfigDevice assigns a disk or volume to a configuration device slot.
type ConfigDevice struct {
	DiskID   *int `json:"disk_id,omitempty"`
	VolumeID *int `json:"volume_id,omitempty"`
}

// ConfigHelpers contains boot helper settings for a configuration profile.
type ConfigHelpers struct {
	DevtmpfsAutomount *bool `json:"devtmpfs_automount,omitempty"`
	Distro            *bool `json:"distro,omitempty"`
	ModulesDep        *bool `json:"modules_dep,omitempty"`
	Network           *bool `json:"network,omitempty"`
	UpdatedbDisabled  *bool `json:"updatedb_disabled,omitempty"`
}

// ConfigInterface represents a legacy network interface in a configuration profile.
type ConfigInterface struct {
	Purpose     string               `json:"purpose"`
	Label       *string              `json:"label,omitempty"`
	IPAMAddress *string              `json:"ipam_address,omitempty"`
	Primary     *bool                `json:"primary,omitempty"`
	SubnetID    *int                 `json:"subnet_id,omitempty"`
	IPv4        *ConfigInterfaceIPv4 `json:"ipv4,omitempty"`
	IPRanges    []string             `json:"ip_ranges,omitempty"`
}

// UpdateConfigInterfaceRequest represents fields that can update a configuration profile interface.
type UpdateConfigInterfaceRequest struct {
	Primary  *bool                `json:"primary,omitempty"`
	IPv4     *ConfigInterfaceIPv4 `json:"ipv4,omitempty"`
	IPRanges []string             `json:"ip_ranges,omitempty"`
}

// ConfigInterfaceResponse represents a legacy network interface returned by a configuration profile interface list.
type ConfigInterfaceResponse struct {
	ID          int                  `json:"id"`
	Active      bool                 `json:"active"`
	Purpose     string               `json:"purpose"`
	Label       *string              `json:"label"`
	IPAMAddress *string              `json:"ipam_address"`
	Primary     bool                 `json:"primary"`
	SubnetID    *int                 `json:"subnet_id"`
	VPCID       *int                 `json:"vpc_id"`
	IPv4        *ConfigInterfaceIPv4 `json:"ipv4"`
	IPRanges    []string             `json:"ip_ranges,omitempty"`
}

// ConfigInterfaceIPv4 contains IPv4 settings for a configuration interface.
type ConfigInterfaceIPv4 struct {
	NAT1To1 *string `json:"nat_1_1,omitempty"`
	VPC     *string `json:"vpc,omitempty"`
}

// CreateConfigRequest represents the request body for creating an instance configuration profile.
type CreateConfigRequest struct {
	Label       string                   `json:"label"`
	Devices     map[string]*ConfigDevice `json:"devices"`
	Kernel      string                   `json:"kernel,omitempty"`
	Comments    string                   `json:"comments,omitempty"`
	MemoryLimit int                      `json:"memory_limit,omitempty"`
	RootDevice  string                   `json:"root_device,omitempty"`
	RunLevel    string                   `json:"run_level,omitempty"`
	VirtMode    string                   `json:"virt_mode,omitempty"`
	Helpers     *ConfigHelpers           `json:"helpers,omitempty"`
	Interfaces  []ConfigInterface        `json:"interfaces,omitempty"`
}

// UpdateConfigRequest represents the request body for updating an instance configuration profile.
type UpdateConfigRequest struct {
	Label       *string                   `json:"label,omitempty"`
	Devices     *map[string]*ConfigDevice `json:"devices,omitempty"`
	Kernel      *string                   `json:"kernel,omitempty"`
	Comments    *string                   `json:"comments,omitempty"`
	MemoryLimit *int                      `json:"memory_limit,omitempty"`
	RootDevice  *string                   `json:"root_device,omitempty"`
	RunLevel    *string                   `json:"run_level,omitempty"`
	VirtMode    *string                   `json:"virt_mode,omitempty"`
	Helpers     *ConfigHelpers            `json:"helpers,omitempty"`
	Interfaces  *[]ConfigInterface        `json:"interfaces,omitempty"`
}

// ReorderConfigInterfacesRequest represents the request body for reordering configuration profile interfaces.
type ReorderConfigInterfacesRequest struct {
	IDs []int `json:"ids"`
}

// InstanceInterfaceSettings represents interface settings for a Linode instance.
type InstanceInterfaceSettings struct {
	DefaultRoute  *InterfaceDefaultRoute `json:"default_route,omitempty"`
	NetworkHelper *bool                  `json:"network_helper,omitempty"`
}

// UpdateInstanceInterfaceSettingsRequest represents fields that can update Linode interface settings.
type UpdateInstanceInterfaceSettingsRequest struct {
	DefaultRoute  *InterfaceDefaultRoute `json:"default_route,omitempty"`
	NetworkHelper *bool                  `json:"network_helper,omitempty"`
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

// UpdateIPRDNSRequest represents the request body for updating an IP address RDNS.
type UpdateIPRDNSRequest struct {
	RDNS *string `json:"rdns"`
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

// MutateInstanceRequest represents the request body for upgrading a Linode instance.
type MutateInstanceRequest struct {
	AllowAutoDiskResize *bool `json:"allow_auto_disk_resize,omitempty"`
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
