package linode

// Instance represents a Linode instance.
type Instance struct {
	ID              int      `json:"id"`
	Label           string   `json:"label"`
	Status          string   `json:"status"`
	Type            string   `json:"type"`
	Region          string   `json:"region"`
	Image           string   `json:"image"`
	IPv4            []string `json:"ipv4"`
	IPv6            string   `json:"ipv6"`
	Hypervisor      string   `json:"hypervisor"`
	Specs           Specs    `json:"specs"`
	Alerts          Alerts   `json:"alerts"`
	Backups         Backups  `json:"backups"`
	Created         string   `json:"created"`
	Updated         string   `json:"updated"`
	Group           string   `json:"group"`
	Tags            []string `json:"tags"`
	WatchdogEnabled bool     `json:"watchdog_enabled"`
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
	Schedule  Schedule `json:"schedule"`
	Last      *Backup  `json:"last_successful"`
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
	ID       int    `json:"id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Region   string `json:"region"`
	Created  string `json:"created"`
	Updated  string `json:"updated"`
	Finished string `json:"finished"`
}

// Region represents a Linode region (datacenter).
type Region struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Country      string   `json:"country"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
	Resolvers    Resolver `json:"resolvers"`
	SiteType     string   `json:"site_type"`
}

// Resolver represents DNS resolvers for a region.
type Resolver struct {
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6"`
}

// InstanceType represents a Linode instance type (plan).
type InstanceType struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Class      string  `json:"class"`
	Disk       int     `json:"disk"`
	Memory     int     `json:"memory"`
	VCPUs      int     `json:"vcpus"`
	GPUs       int     `json:"gpus"`
	NetworkOut int     `json:"network_out"`
	Transfer   int     `json:"transfer"`
	Price      Price   `json:"price"`
	Addons     Addons  `json:"addons"`
	Successor  *string `json:"successor"`
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

// CreateInstanceRequest represents the request body for creating a Linode instance.
type CreateInstanceRequest struct {
	Region          string   `json:"region"`
	Type            string   `json:"type"`
	Label           string   `json:"label,omitempty"`
	Image           string   `json:"image,omitempty"`
	RootPass        string   `json:"root_pass,omitempty"`
	AuthorizedKeys  []string `json:"authorized_keys,omitempty"`
	AuthorizedUsers []string `json:"authorized_users,omitempty"`
	StackScriptID   *int     `json:"stackscript_id,omitempty"`
	StackScriptData any      `json:"stackscript_data,omitempty"`
	BackupsEnabled  bool     `json:"backups_enabled,omitempty"`
	SwapSize        *int     `json:"swap_size,omitempty"`
	PrivateIP       bool     `json:"private_ip,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Booted          *bool    `json:"booted,omitempty"`
}

// ResizeInstanceRequest represents the request body for resizing a Linode instance.
type ResizeInstanceRequest struct {
	Type          string `json:"type"`
	MigrationType string `json:"migration_type,omitempty"`
	AllowAutoDisk bool   `json:"allow_auto_disk,omitempty"`
}
