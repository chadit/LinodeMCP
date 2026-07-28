package linode

// CurrentInterfaceGeneration is the Linode Interfaces generation this codebase
// targets. The Linode API rejects POST /linode/instances payloads whose
// interface_generation does not match the account's enabled generation, so this
// constant is the single source of truth for the wire value.
const CurrentInterfaceGeneration = "linode"

// Instance represents a Linode instance.
type Instance struct {
	Backups             Backups             `json:"backups"`
	Hypervisor          string              `json:"hypervisor"`
	Group               string              `json:"group"`
	Type                string              `json:"type"`
	Region              string              `json:"region"`
	Image               string              `json:"image"`
	InterfaceGeneration string              `json:"interface_generation,omitempty"`
	IPv6                string              `json:"ipv6"`
	Updated             string              `json:"updated"`
	Status              string              `json:"status"`
	Label               string              `json:"label"`
	Created             string              `json:"created"`
	Tags                []string            `json:"tags"`
	IPv4                []string            `json:"ipv4"`
	Interfaces          []InstanceInterface `json:"interfaces,omitempty"`
	Alerts              Alerts              `json:"alerts"`
	Specs               Specs               `json:"specs"`
	ID                  int                 `json:"id"`
	WatchdogEnabled     bool                `json:"watchdog_enabled"`
}

// InstanceInterface represents a network interface on a Linode instance under
// the current Interfaces generation. Exactly one of Public, VPC, or VLAN is set
// per interface.
type InstanceInterface struct {
	Public       *InterfacePublicConfig `json:"public,omitempty"`
	VPC          *InterfaceVPCConfig    `json:"vpc,omitempty"`
	VLAN         *InterfaceVLANConfig   `json:"vlan,omitempty"`
	DefaultRoute *InterfaceDefaultRoute `json:"default_route,omitempty"`
	FirewallID   *int                   `json:"firewall_id,omitempty"`
	MACAddress   string                 `json:"mac_address,omitempty"`
	Created      string                 `json:"created,omitempty"`
	Updated      string                 `json:"updated,omitempty"`
	ID           int                    `json:"id,omitempty"`
	Version      int                    `json:"version,omitempty"`
}

// InstanceInterfaceHistory represents a historical version of a Linode interface.
type InstanceInterfaceHistory struct {
	InterfaceData      any    `json:"interface_data"`
	Created            string `json:"created"`
	InterfaceHistoryID int    `json:"interface_history_id"`
	InterfaceID        int    `json:"interface_id"`
	LinodeID           int    `json:"linode_id"`
	Version            int    `json:"version"`
}

// AddInstanceInterfaceRequest represents the request body for adding a Linode
// interface to an existing instance. Exactly one of Public, VPC, or VLAN should
// be set.
type AddInstanceInterfaceRequest struct {
	Public       *InterfacePublicConfig         `json:"public,omitempty"`
	VPC          *AddInstanceInterfaceVPCConfig `json:"vpc,omitempty"`
	VLAN         *InterfaceVLANConfig           `json:"vlan,omitempty"`
	DefaultRoute *AddInterfaceDefaultRoute      `json:"default_route,omitempty"`
	FirewallID   *int                           `json:"firewall_id,omitempty"`
}

// UpdateInstanceInterfaceRequest represents the request body for updating a
// Linode interface. Exactly one of Public, VPC, or VLAN should be set.
type UpdateInstanceInterfaceRequest struct {
	Public       *InterfacePublicConfig            `json:"public,omitempty"`
	VPC          *UpdateInstanceInterfaceVPCConfig `json:"vpc,omitempty"`
	VLAN         *InterfaceVLANConfig              `json:"vlan,omitempty"`
	DefaultRoute *AddInterfaceDefaultRoute         `json:"default_route,omitempty"`
}

// UpgradeLinodeInterfacesRequest represents the request body for upgrading legacy config interfaces.
type UpgradeLinodeInterfacesRequest struct {
	ConfigID *int  `json:"config_id,omitempty"`
	DryRun   *bool `json:"dry_run,omitempty"`
}

// UpgradeLinodeInterfacesResponse represents the upgraded Linode interface preview or result.
type UpgradeLinodeInterfacesResponse struct {
	Interfaces []InstanceInterface `json:"interfaces"`
	ConfigID   int                 `json:"config_id"`
	DryRun     bool                `json:"dry_run"`
}

// UpdateInstanceInterfaceVPCConfig holds VPC settings for an existing Linode interface.
type UpdateInstanceInterfaceVPCConfig struct {
	IPv4     *AddInstanceInterfaceIPv4 `json:"ipv4,omitempty"`
	SubnetID int                       `json:"subnet_id"`
}

// AddInstanceInterfaceVPCConfig holds VPC settings for a new Linode interface.
type AddInstanceInterfaceVPCConfig struct {
	IPv4     *AddInstanceInterfaceIPv4 `json:"ipv4,omitempty"`
	SubnetID int                       `json:"subnet_id"`
}

// AddInstanceInterfaceIPv4 holds IPv4 settings for a new VPC interface.
type AddInstanceInterfaceIPv4 struct {
	Addresses []AddInterfaceIPv4Address `json:"addresses,omitempty"`
	Ranges    []InterfaceIPv4Range      `json:"ranges,omitempty"`
}

// AddInterfaceIPv4Address represents an IPv4 address assignment for a new interface.
type AddInterfaceIPv4Address struct {
	NAT11Address *string `json:"nat_1_1_address,omitempty"`
	Primary      *bool   `json:"primary,omitempty"`
	Address      string  `json:"address"`
}

// InterfaceIPv4Range represents an IPv4 range on an interface.
type InterfaceIPv4Range struct {
	Range string `json:"range"`
}

// AddInterfaceDefaultRoute controls default-route assignment for a new interface.
type AddInterfaceDefaultRoute struct {
	IPv4 *bool `json:"ipv4,omitempty"`
	IPv6 *bool `json:"ipv6,omitempty"`
}

// InterfacePublicConfig holds public-interface configuration. Sub-fields are
// derived from BIMHelperScripts reference; live-response field discovery is
// deferred per the linode-interfaces-fix spec.
type InterfacePublicConfig struct {
	IPv4 *InterfacePublicIPv4 `json:"ipv4,omitempty"`
	IPv6 *InterfacePublicIPv6 `json:"ipv6,omitempty"`
}

// InterfacePublicIPv4 is the public IPv4 sub-config. Field set is conservative
// pending live-response capture.
type InterfacePublicIPv4 struct {
	Addresses []InterfaceIPv4Address `json:"addresses,omitempty"`
}

// InterfacePublicIPv6 is the public IPv6 sub-config. Field set is conservative
// pending live-response capture.
type InterfacePublicIPv6 struct {
	Ranges []InterfaceIPv6Range `json:"ranges,omitempty"`
}

// InterfaceIPv4Address represents a single IPv4 address on an interface.
type InterfaceIPv4Address struct {
	Address string `json:"address"`
	Primary bool   `json:"primary,omitempty"`
}

// InterfaceIPv6Range represents an IPv6 range on an interface.
type InterfaceIPv6Range struct {
	Range string `json:"range"`
}

// InterfaceVPCConfig holds VPC-attached-interface configuration.
type InterfaceVPCConfig struct {
	IPv4     *InterfaceVPCIPv4 `json:"ipv4,omitempty"`
	SubnetID int               `json:"subnet_id"`
}

// InterfaceVPCIPv4 is the VPC IPv4 sub-config.
type InterfaceVPCIPv4 struct {
	Addresses []InterfaceIPv4Address `json:"addresses,omitempty"`
}

// InterfaceVLANConfig holds VLAN-attached-interface configuration.
type InterfaceVLANConfig struct {
	Label       string `json:"vlan_label"`
	IPAMAddress string `json:"ipam_address,omitempty"`
}

// InterfaceDefaultRoute controls whether the interface owns the default route
// for each address family. A field is sent only when true; false values are
// omitted from the wire so the API treats them as unset.
type InterfaceDefaultRoute struct {
	IPv4 bool `json:"ipv4,omitempty"`
	IPv6 bool `json:"ipv6,omitempty"`
}

// Specs represents instance hardware specifications.
type Specs struct {
	Disk     int `json:"disk"`
	Memory   int `json:"memory"`
	VCPUs    int `json:"vcpus"`
	GPUs     int `json:"gpus"`
	Transfer int `json:"transfer"`
}

// Alerts represents alert settings for an instance.
type Alerts struct {
	CPU           int `json:"cpu"`
	NetworkIn     int `json:"network_in"`
	NetworkOut    int `json:"network_out"`
	TransferQuota int `json:"transfer_quota"`
	IO            int `json:"io"`
}

// Backups represents backup settings.
type Backups struct {
	Last      *Backup  `json:"last_successful"`
	Schedule  Schedule `json:"schedule"`
	Enabled   bool     `json:"enabled"`
	Available bool     `json:"available"`
}

// Schedule represents backup schedule settings.
type Schedule struct {
	Day    string `json:"day"`
	Window string `json:"window"`
}

// Backup represents a backup snapshot.
type Backup struct {
	Label    string `json:"label"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Region   string `json:"region"`
	Created  string `json:"created"`
	Updated  string `json:"updated"`
	Finished string `json:"finished"`
	ID       int    `json:"id"`
}

// Region represents a Linode region (datacenter).
type Region struct {
	Resolvers    Resolver `json:"resolvers"`
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Country      string   `json:"country"`
	Status       string   `json:"status"`
	SiteType     string   `json:"site_type"`
	Capabilities []string `json:"capabilities"`
}

// RegionAvailability represents compute instance type availability for a region.
type RegionAvailability struct {
	Region    string `json:"region"`
	Plan      string `json:"plan"`
	Available bool   `json:"available"`
}

// Resolver represents DNS resolvers for a region.
type Resolver struct {
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6"`
}

// Kernel represents a Linode kernel.
type Kernel struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
	Built        string `json:"built"`
	KVM          bool   `json:"kvm"`
	PVOPS        bool   `json:"pvops"`
	Deprecated   bool   `json:"deprecated"`
}

// InstanceType represents a Linode instance type (plan).
type InstanceType struct {
	Successor  *string `json:"successor"`
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Class      string  `json:"class"`
	Price      Price   `json:"price"`
	Addons     Addons  `json:"addons"`
	Disk       int     `json:"disk"`
	Memory     int     `json:"memory"`
	VCPUs      int     `json:"vcpus"`
	GPUs       int     `json:"gpus"`
	NetworkOut int     `json:"network_out"`
	Transfer   int     `json:"transfer"`
}

// Price represents pricing for a Linode type.
type Price struct {
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

// Addons represents add-on pricing for a Linode type.
type Addons struct {
	Backups BackupsAddon `json:"backups"`
}

// BackupsAddon represents backup add-on pricing.
type BackupsAddon struct {
	Price Price `json:"price"`
}

// Image represents a Linode image (OS image or custom image).
type Image struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	Vendor       string   `json:"vendor"`
	Status       string   `json:"status"`
	Created      string   `json:"created"`
	CreatedBy    string   `json:"created_by"`
	Expiry       *string  `json:"expiry"`
	EOL          *string  `json:"eol"`
	Capabilities []string `json:"capabilities"`
	Tags         []string `json:"tags"`
	Size         int      `json:"size"`
	IsPublic     bool     `json:"is_public"`
	Deprecated   bool     `json:"deprecated"`
}

// ReplicateImageRequest represents the request body for replicating an image to regions.
type ReplicateImageRequest struct {
	Regions []string `json:"regions"`
}

// UpdateImageRequest represents editable fields for a Linode image.
type UpdateImageRequest struct {
	Label       *string   `json:"label,omitempty"`
	Description *string   `json:"description,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
}

// ImageShareGroup represents an owned image share group.
type ImageShareGroup struct {
	Description  *string `json:"description"`
	Updated      *string `json:"updated"`
	Expiry       *string `json:"expiry"`
	UUID         string  `json:"uuid"`
	Label        string  `json:"label"`
	Created      string  `json:"created"`
	ID           int     `json:"id"`
	ImagesCount  int     `json:"images_count"`
	MembersCount int     `json:"members_count"`
	IsSuspended  bool    `json:"is_suspended"`
}

// ImageShareGroupMember represents a member linked to an image share group.
type ImageShareGroupMember struct {
	Updated   *string `json:"updated"`
	Expiry    *string `json:"expiry"`
	TokenUUID string  `json:"token_uuid"`
	Status    string  `json:"status"`
	Label     string  `json:"label"`
	Created   string  `json:"created"`
}

// ImageShareGroupImage represents an image to add when creating an image share group.
type ImageShareGroupImage struct {
	ID          string `json:"id"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// CreateImageShareGroupRequest represents the request body for creating an image share group.
type CreateImageShareGroupRequest struct {
	Label       string                 `json:"label"`
	Description string                 `json:"description,omitempty"`
	Images      []ImageShareGroupImage `json:"images,omitempty"`
}

// AddImageShareGroupImagesRequest represents the request body for adding images to a share group.
type AddImageShareGroupImagesRequest struct {
	Images []ImageShareGroupImage `json:"images"`
}

// AddImageShareGroupMembersRequest represents the request body for adding members to a share group.
type AddImageShareGroupMembersRequest struct {
	Label string `json:"label"`
	Token string `json:"token"`
}

// UpdateImageShareGroupRequest represents the request body for updating an image share group.
type UpdateImageShareGroupRequest struct {
	Label       *string `json:"label,omitempty"`
	Description *string `json:"description,omitempty"`
}

// UpdateImageShareGroupImageRequest represents the request body for updating a shared image.
type UpdateImageShareGroupImageRequest struct {
	Label       *string `json:"label,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ImageShareGroupToken represents a token associated with an image share group.
type ImageShareGroupToken struct {
	Token                  string  `json:"token,omitempty"`
	TokenUUID              string  `json:"token_uuid"`
	Status                 string  `json:"status"`
	Label                  string  `json:"label"`
	Created                string  `json:"created"`
	Updated                *string `json:"updated"`
	Expiry                 *string `json:"expiry"`
	ValidForShareGroupUUID string  `json:"valid_for_sharegroup_uuid"`
	ShareGroupUUID         string  `json:"sharegroup_uuid"`
	ShareGroupLabel        string  `json:"sharegroup_label"`
}

// CreateImageShareGroupTokenRequest represents the request body for creating an image share group membership token.
type CreateImageShareGroupTokenRequest struct {
	Label                  string `json:"label,omitempty"`
	ValidForShareGroupUUID string `json:"valid_for_sharegroup_uuid"`
}

// UpdateImageShareGroupTokenRequest represents the request body for updating an image share group membership token.
type UpdateImageShareGroupTokenRequest struct {
	Label string `json:"label"`
}

// UpdateImageShareGroupMemberRequest represents the request body for updating an image share group member token.
type UpdateImageShareGroupMemberRequest struct {
	Label string `json:"label"`
}

// CreateImageRequest represents the request body for creating a private image from a Linode disk.
type CreateImageRequest struct {
	Label       string   `json:"label,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	DiskID      int      `json:"disk_id"`
	CloudInit   bool     `json:"cloud_init,omitempty"`
}

// UploadImageRequest represents the request body for uploading a custom image.
type UploadImageRequest struct {
	Label       string   `json:"label"`
	Region      string   `json:"region"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	CloudInit   bool     `json:"cloud_init,omitempty"`
}

// UploadImageResponse represents the image upload target returned by the API.
type UploadImageResponse struct {
	UploadTo string `json:"upload_to"`
	Image    Image  `json:"image"`
}

// StackScript represents a Linode StackScript for automated deployments.
type StackScript struct {
	Username          string   `json:"username"`
	UserGravatarID    string   `json:"user_gravatar_id"`
	Label             string   `json:"label"`
	Description       string   `json:"description"`
	Images            []string `json:"images"`
	Created           string   `json:"created"`
	Updated           string   `json:"updated"`
	RevNote           string   `json:"rev_note"`
	Script            string   `json:"script"`
	UserDefinedFields []UDF    `json:"user_defined_fields"`
	ID                int      `json:"id"`
	DeploymentsTotal  int      `json:"deployments_total"`
	DeploymentsActive int      `json:"deployments_active"`
	IsPublic          bool     `json:"is_public"`
	Mine              bool     `json:"mine"`
}

// UDF represents a user-defined field in a StackScript.
type UDF struct {
	Label   string `json:"label"`
	Name    string `json:"name"`
	Example string `json:"example"`
	OneOf   string `json:"oneof"`
	Default string `json:"default"`
	ManyOf  string `json:"manyof"`
}

// CreateStackScriptRequest represents the request body for creating a StackScript.
type CreateStackScriptRequest struct {
	Label       string   `json:"label"`
	Script      string   `json:"script"`
	Description string   `json:"description,omitempty"`
	RevNote     string   `json:"rev_note,omitempty"`
	Images      []string `json:"images"`
	IsPublic    bool     `json:"is_public,omitempty"`
}

// UpdateStackScriptRequest represents the request body for updating a StackScript.
type UpdateStackScriptRequest struct {
	Label       *string  `json:"label,omitempty"`
	Script      *string  `json:"script,omitempty"`
	Description *string  `json:"description,omitempty"`
	IsPublic    *bool    `json:"is_public,omitempty"`
	RevNote     *string  `json:"rev_note,omitempty"`
	Images      []string `json:"images,omitempty"`
}

// CreateInstanceRequest represents the request body for creating a Linode
// instance under the current Linode Interfaces generation. InterfaceGeneration
// and Interfaces are required on the wire; the Linode API rejects with
// "must have at least 1 interface defined to boot" when Interfaces is empty.
type CreateInstanceRequest struct {
	StackScriptData     any                 `json:"stackscript_data,omitempty"`
	SwapSize            *int                `json:"swap_size,omitempty"`
	StackScriptID       *int                `json:"stackscript_id,omitempty"`
	Booted              *bool               `json:"booted,omitempty"`
	Type                string              `json:"type"`
	Label               string              `json:"label,omitempty"`
	Image               string              `json:"image,omitempty"`
	RootPass            string              `json:"root_pass,omitempty"`
	InterfaceGeneration string              `json:"interface_generation"`
	Region              string              `json:"region"`
	AuthorizedUsers     []string            `json:"authorized_users,omitempty"`
	Tags                []string            `json:"tags,omitempty"`
	AuthorizedKeys      []string            `json:"authorized_keys,omitempty"`
	Interfaces          []InstanceInterface `json:"interfaces"`
	BackupsEnabled      bool                `json:"backups_enabled,omitempty"`
}

// ResizeInstanceRequest represents the request body for resizing a Linode instance.
type ResizeInstanceRequest struct {
	Type                string `json:"type"`
	MigrationType       string `json:"migration_type,omitempty"`
	AllowAutoDiskResize bool   `json:"allow_auto_disk_resize,omitempty"`
}

// UpdateInstanceRequest represents the request body for updating a Linode
// instance. All fields are optional; only provided fields are updated.
type UpdateInstanceRequest struct {
	WatchdogEnabled   *bool    `json:"watchdog_enabled,omitempty"`
	Alerts            *Alerts  `json:"alerts,omitempty"`
	Label             string   `json:"label,omitempty"`
	Group             string   `json:"group,omitempty"`
	MaintenancePolicy string   `json:"maintenance_policy,omitempty"`
	Tags              []string `json:"tags,omitempty"`
}
