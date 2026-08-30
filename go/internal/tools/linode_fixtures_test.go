package tools_test

// The Linode API response and request shapes these tests build fixtures from.
// Handlers decode every tool answer through the generated proto messages, so
// the client has no production declaration of these shapes for a test to reuse.

// Account represents a Linode account.
type Account struct {
	Zip               string   `json:"zip"`
	Phone             string   `json:"phone"`
	Email             string   `json:"email"`
	Company           string   `json:"company"`
	Address1          string   `json:"address_1"`
	Address2          string   `json:"address_2"`
	City              string   `json:"city"`
	State             string   `json:"state"`
	LastName          string   `json:"last_name"`
	FirstName         string   `json:"first_name"`
	Country           string   `json:"country"`
	BillingSource     string   `json:"billing_source"`
	EUUID             string   `json:"euuid"`
	ActiveSince       string   `json:"active_since"`
	Capabilities      []string `json:"capabilities"`
	ActivePromotions  []Promo  `json:"active_promotions"`
	BalanceUninvoiced float64  `json:"balance_uninvoiced"`
	Balance           float64  `json:"balance"`
}

// AccountAgreements represents the acknowledgment status for account agreements.
type AccountAgreements struct {
	BillingAgreement       bool `json:"billing_agreement"`
	EUModel                bool `json:"eu_model"`
	MasterServiceAgreement bool `json:"master_service_agreement"`
	PrivacyPolicy          bool `json:"privacy_policy"`
}

// AccountAvailability represents the account service availability for a region.
type AccountAvailability struct {
	Available   []string `json:"available"`
	Region      string   `json:"region"`
	Unavailable []string `json:"unavailable"`
}

// AccountBetaProgram represents a beta program that the account is enrolled in.
type AccountBetaProgram struct {
	Description *string `json:"description"`
	Ended       *string `json:"ended"`
	Enrolled    string  `json:"enrolled"`
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Started     string  `json:"started"`
}

// AccountEntityTransfer represents an account entity transfer request.
type AccountEntityTransfer struct {
	Created  string                        `json:"created"`
	Expiry   string                        `json:"expiry"`
	Status   string                        `json:"status"`
	Token    string                        `json:"token"`
	Updated  string                        `json:"updated"`
	Entities AccountEntityTransferEntities `json:"entities"`
	IsSender bool                          `json:"is_sender"`
}

// AccountEntityTransferEntities groups transferred entities by type.
type AccountEntityTransferEntities struct {
	Linodes []int `json:"linodes"`
}

// AccountEvent represents an account event returned by GET /account/events.
type AccountEvent struct {
	PercentComplete *int                `json:"percent_complete"`
	Duration        *float64            `json:"duration"`
	Entity          *AccountEventEntity `json:"entity"`
	Rate            *string             `json:"rate"`
	SecondaryEntity *AccountEventEntity `json:"secondary_entity"`
	TimeRemaining   *string             `json:"time_remaining"`
	Created         string              `json:"created"`
	Message         string              `json:"message"`
	Action          string              `json:"action"`
	Status          string              `json:"status"`
	Username        string              `json:"username"`
	ID              int                 `json:"id"`
	Seen            bool                `json:"seen"`
}

// AccountEventEntity identifies the primary or secondary entity attached to an account event.
type AccountEventEntity struct {
	ID    any    `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

// AccountInvoice represents one account invoice.
type AccountInvoice struct {
	Date  string  `json:"date"`
	Label string  `json:"label"`
	ID    int     `json:"id"`
	Total float64 `json:"total"`
}

// AccountInvoiceItem represents one line item on an account invoice.
type AccountInvoiceItem struct {
	From      string  `json:"from"`
	Label     string  `json:"label"`
	To        string  `json:"to"`
	Type      string  `json:"type"`
	Amount    float64 `json:"amount"`
	Quantity  int     `json:"quantity"`
	Tax       float64 `json:"tax"`
	Total     float64 `json:"total"`
	UnitPrice float64 `json:"unit_price"`
}

// AccountLogin represents one user login returned by GET /account/logins.
type AccountLogin struct {
	Datetime   string `json:"datetime"`
	IP         string `json:"ip"`
	Status     string `json:"status"`
	Username   string `json:"username"`
	ID         int    `json:"id"`
	Restricted bool   `json:"restricted"`
}

// AccountMaintenance represents one account maintenance record.
type AccountMaintenance struct {
	Reason string                   `json:"reason"`
	Status string                   `json:"status"`
	Type   string                   `json:"type"`
	When   string                   `json:"when"`
	Entity AccountMaintenanceEntity `json:"entity"`
}

// AccountMaintenanceEntity identifies the entity attached to a maintenance record.
type AccountMaintenanceEntity struct {
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
	ID    int    `json:"id"`
}

// AccountNotification represents one account notification returned by GET /account/notifications.
type AccountNotification struct {
	Entity   *AccountNotificationEntity `json:"entity"`
	Until    *string                    `json:"until"`
	When     *string                    `json:"when"`
	Label    string                     `json:"label"`
	Message  string                     `json:"message"`
	Severity string                     `json:"severity"`
	Type     string                     `json:"type"`
}

// AccountNotificationEntity identifies the entity attached to an account notification.
type AccountNotificationEntity struct {
	ID    any    `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

// AccountPayment represents one account payment.
type AccountPayment struct {
	Date string  `json:"date"`
	ID   int     `json:"id"`
	USD  float64 `json:"usd"`
}

// AccountPaymentMethod represents a payment method available on the account.
type AccountPaymentMethod struct {
	Data      map[string]any `json:"data"`
	Type      string         `json:"type"`
	ID        int            `json:"id"`
	IsDefault bool           `json:"is_default"`
}

// AccountRegionTransfer represents network transfer usage for a region.
type AccountRegionTransfer struct {
	ID       string `json:"id"`
	Billable int    `json:"billable"`
	Quota    int    `json:"quota"`
	Used     int    `json:"used"`
}

// AccountSettings represents account-wide settings returned by GET /account/settings.
type AccountSettings struct {
	LongviewSubscription    *string `json:"longview_subscription"`
	ObjectStorage           *string `json:"object_storage"`
	InterfacesForNewLinodes string  `json:"interfaces_for_new_linodes"`
	MaintenancePolicy       string  `json:"maintenance_policy"`
	BackupsEnabled          bool    `json:"backups_enabled"`
	Managed                 bool    `json:"managed"`
	NetworkHelper           bool    `json:"network_helper"`
}

// AccountTransfer represents account network transfer usage returned by GET /account/transfer.
type AccountTransfer struct {
	RegionTransfers []AccountRegionTransfer `json:"region_transfers"`
	Billable        int                     `json:"billable"`
	Quota           int                     `json:"quota"`
	Used            int                     `json:"used"`
}

// AccountUser represents one user returned by account user endpoints.
type AccountUser struct {
	LastLogin           *AccountUserLastLogin `json:"last_login"`
	PasswordCreated     *string               `json:"password_created"`
	VerifiedPhoneNumber *string               `json:"verified_phone_number"`
	Email               string                `json:"email"`
	UserType            string                `json:"user_type"`
	Username            string                `json:"username"`
	SSHKeys             []string              `json:"ssh_keys"`
	Restricted          bool                  `json:"restricted"`
	TFAEnabled          bool                  `json:"tfa_enabled"`
}

// AccountUserLastLogin contains the most recent login attempt for an account user.
type AccountUserLastLogin struct {
	LoginDatetime string `json:"login_datetime"`
	Status        string `json:"status"`
}

// Addons represents add-on pricing for a Linode type.
type Addons struct {
	Backups BackupsAddon `json:"backups"`
}

// Alerts represents alert settings for an instance.
type Alerts struct {
	CPU           int `json:"cpu"`
	NetworkIn     int `json:"network_in"`
	NetworkOut    int `json:"network_out"`
	TransferQuota int `json:"transfer_quota"`
	IO            int `json:"io"`
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

// Backups represents backup settings.
type Backups struct {
	Last      *Backup  `json:"last_successful"`
	Schedule  Schedule `json:"schedule"`
	Enabled   bool     `json:"enabled"`
	Available bool     `json:"available"`
}

// BackupsAddon represents backup add-on pricing.
type BackupsAddon struct {
	Price Price `json:"price"`
}

// BetaProgram represents a beta program available for account enrollment.
type BetaProgram struct {
	Description    *string `json:"description"`
	Ended          *string `json:"ended"`
	BetaClass      string  `json:"class"`
	ID             string  `json:"id"`
	Label          string  `json:"label"`
	MoreInfo       string  `json:"more_info"`
	Started        string  `json:"started"`
	GreenlightOnly bool    `json:"greenlight_only"`
}

// BucketSSL represents the SSL/TLS certificate status for an Object Storage bucket.
type BucketSSL struct {
	SSL bool `json:"ssl"`
}

// ChildAccount represents a child-level account available to a parent account.
type ChildAccount struct {
	CreditCard        ChildAccountCreditCard `json:"credit_card"`
	EUUID             string                 `json:"euuid"`
	LastName          string                 `json:"last_name"`
	Zip               string                 `json:"zip"`
	TaxID             string                 `json:"tax_id"`
	BillingSource     string                 `json:"billing_source"`
	State             string                 `json:"state"`
	City              string                 `json:"city"`
	Company           string                 `json:"company"`
	Address2          string                 `json:"address_2"`
	Email             string                 `json:"email"`
	Address1          string                 `json:"address_1"`
	ActiveSince       string                 `json:"active_since"`
	FirstName         string                 `json:"first_name"`
	Country           string                 `json:"country"`
	Phone             string                 `json:"phone"`
	Capabilities      []string               `json:"capabilities"`
	BalanceUninvoiced float64                `json:"balance_uninvoiced"`
	Balance           float64                `json:"balance"`
}

// ChildAccountCreditCard contains masked credit card details for a child account.
type ChildAccountCreditCard struct {
	Expiry   string `json:"expiry"`
	LastFour string `json:"last_four"`
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

// ConfigInterfaceIPv4 contains IPv4 settings for a configuration interface.
type ConfigInterfaceIPv4 struct {
	NAT1To1 *string `json:"nat_1_1,omitempty"`
	VPC     *string `json:"vpc,omitempty"`
}

// ConfigInterfaceResponse represents a legacy network interface returned by a configuration profile interface list.
type ConfigInterfaceResponse struct {
	Label       *string              `json:"label"`
	IPAMAddress *string              `json:"ipam_address"`
	SubnetID    *int                 `json:"subnet_id"`
	VPCID       *int                 `json:"vpc_id"`
	IPv4        *ConfigInterfaceIPv4 `json:"ipv4"`
	Purpose     string               `json:"purpose"`
	IPRanges    []string             `json:"ip_ranges,omitempty"`
	ID          int                  `json:"id"`
	Active      bool                 `json:"active"`
	Primary     bool                 `json:"primary"`
}

// CreateOAuthClientRequest contains the required fields for POST /account/oauth-clients.
type CreateOAuthClientRequest struct {
	Label       string `json:"label"`
	RedirectURI string `json:"redirect_uri"`
	Public      bool   `json:"public"`
}

// CreatePlacementGroupRequest represents the request body for creating a placement group.
type CreatePlacementGroupRequest struct {
	Label                string `json:"label"`
	Region               string `json:"region"`
	PlacementGroupType   string `json:"placement_group_type"`
	PlacementGroupPolicy string `json:"placement_group_policy"`
}

// CreatedOAuthClient represents the response from creating an OAuth client.
type CreatedOAuthClient struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	RedirectURI string `json:"redirect_uri"`
	Secret      string `json:"secret"`
}

// DatabaseEngine represents a Managed Database engine.
type DatabaseEngine struct {
	ID      string `json:"id"`
	Engine  string `json:"engine"`
	Version string `json:"version"`
}

// DatabaseInstance represents a Managed Database instance.
type DatabaseInstance struct {
	Version         string   `json:"version"`
	Created         string   `json:"created"`
	Label           string   `json:"label"`
	Region          string   `json:"region"`
	Type            string   `json:"type"`
	Engine          string   `json:"engine"`
	ReplicationType string   `json:"replication_type"`
	Updated         string   `json:"updated"`
	Status          string   `json:"status"`
	AllowList       []string `json:"allow_list"`
	ClusterSize     int      `json:"cluster_size"`
	ID              int      `json:"id"`
	SSLConnection   bool     `json:"ssl_connection"`
	Encrypted       bool     `json:"encrypted"`
}

// DatabaseSSL contains the SSL CA certificate for a MySQL Managed Database.
type DatabaseSSL struct {
	CACertificate string `json:"ca_certificate"`
}

// DatabaseType represents a Managed Database node plan type.
type DatabaseType struct {
	ID         string              `json:"id"`
	Label      string              `json:"label"`
	Class      string              `json:"class"`
	Engines    DatabaseTypeEngines `json:"engines"`
	Disk       int                 `json:"disk"`
	Memory     int                 `json:"memory"`
	VCPUs      int                 `json:"vcpus"`
	Deprecated bool                `json:"deprecated"`
}

// DatabaseTypeEngine represents pricing for one Managed Database node quantity.
type DatabaseTypeEngine struct {
	Quantity int   `json:"quantity"`
	Price    Price `json:"price"`
}

// DatabaseTypeEngines contains engine-specific node quantities and prices.
type DatabaseTypeEngines struct {
	MySQL      []DatabaseTypeEngine `json:"mysql"`
	PostgreSQL []DatabaseTypeEngine `json:"postgresql"`
}

// Domain represents a Linode DNS domain.
type Domain struct {
	Created     string   `json:"created"`
	Domain      string   `json:"domain"`
	Type        string   `json:"type"`   // master, slave
	Status      string   `json:"status"` // active, disabled, edit_mode
	SOAEmail    string   `json:"soa_email"`
	Description string   `json:"description"`
	Group       string   `json:"group"`
	Updated     string   `json:"updated"`
	AXFRIPs     []string `json:"axfr_ips"`
	Tags        []string `json:"tags"`
	MasterIPs   []string `json:"master_ips"`
	ExpireSec   int      `json:"expire_sec"`
	RefreshSec  int      `json:"refresh_sec"`
	TTLSec      int      `json:"ttl_sec"`
	ID          int      `json:"id"`
	RetrySec    int      `json:"retry_sec"`
}

// DomainRecord represents a DNS record within a domain.
type DomainRecord struct {
	Protocol string `json:"protocol"`
	Type     string `json:"type"` // A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, PTR
	Name     string `json:"name"`
	Target   string `json:"target"`
	Updated  string `json:"updated"`
	Created  string `json:"created"`
	Tag      string `json:"tag"`
	Service  string `json:"service"`
	Port     int    `json:"port"`
	TTLSec   int    `json:"ttl_sec"`
	ID       int    `json:"id"`
	Weight   int    `json:"weight"`
	Priority int    `json:"priority"`
}

// Firewall represents a Linode Cloud Firewall.
type Firewall struct {
	Label   string        `json:"label"`
	Status  string        `json:"status"` // enabled, disabled, deleted
	Created string        `json:"created"`
	Updated string        `json:"updated"`
	Tags    []string      `json:"tags"`
	Rules   FirewallRules `json:"rules"`
	ID      int           `json:"id"`
}

// FirewallAddresses represents IPv4 and IPv6 addresses for a firewall rule.
type FirewallAddresses struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// FirewallDefaultIDs contains default firewall IDs by resource type.
type FirewallDefaultIDs struct {
	Linode          int `json:"linode"`
	NodeBalancer    int `json:"nodebalancer"`
	PublicInterface int `json:"public_interface"`
	VPCInterface    int `json:"vpc_interface"`
}

// FirewallDevice represents a device attached to a Cloud Firewall.
type FirewallDevice struct {
	Created string               `json:"created"`
	Updated string               `json:"updated"`
	Entity  FirewallDeviceEntity `json:"entity"`
	ID      int                  `json:"id"`
}

// FirewallDeviceEntity represents the Linode, Linode interface, or NodeBalancer attached to a firewall.
type FirewallDeviceEntity struct {
	ParentEntity *FirewallDeviceEntity `json:"parent_entity"`
	Label        string                `json:"label"`
	Type         string                `json:"type"`
	URL          string                `json:"url"`
	ID           int                   `json:"id"`
}

// FirewallRule represents a single firewall rule.
type FirewallRule struct {
	Action      string            `json:"action"`   // ACCEPT, DROP
	Protocol    string            `json:"protocol"` // TCP, UDP, ICMP, IPENCAP
	Ports       string            `json:"ports"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Addresses   FirewallAddresses `json:"addresses"`
}

// FirewallRules represents inbound and outbound firewall rules.
type FirewallRules struct {
	InboundPolicy  string         `json:"inbound_policy"`
	OutboundPolicy string         `json:"outbound_policy"`
	Fingerprint    string         `json:"fingerprint,omitempty"`
	Inbound        []FirewallRule `json:"inbound"`
	Outbound       []FirewallRule `json:"outbound"`
	Version        int            `json:"version,omitempty"`
}

// FirewallSettings represents the default firewall assignments for resource types.
type FirewallSettings struct {
	DefaultFirewallIDs FirewallDefaultIDs `json:"default_firewall_ids"`
}

// FirewallTemplate represents a reusable Cloud Firewall rule template.
type FirewallTemplate struct {
	Slug  string        `json:"slug"`
	Rules FirewallRules `json:"rules"`
}

// IPAddress represents an IPv4 address assigned to a Linode instance.
type IPAddress struct {
	VPCNAT1To1 *string `json:"vpc_nat_1_1,omitempty"`
	Address    string  `json:"address"`
	Gateway    string  `json:"gateway"`
	SubnetMask string  `json:"subnet_mask"`
	Type       string  `json:"type"`
	RDNS       string  `json:"rdns"`
	Region     string  `json:"region"`
	Prefix     int     `json:"prefix"`
	LinodeID   int     `json:"linode_id"`
	Public     bool    `json:"public"`
}

// IPv6Pool represents an IPv6 pool on the account.
type IPv6Pool struct {
	Range  string `json:"range"`
	Region string `json:"region"`
	Prefix int    `json:"prefix"`
}

// IPv6Range represents an IPv6 range or pool assigned to an instance.
type IPv6Range struct {
	Range       string `json:"range"`
	Region      string `json:"region"`
	RouteTarget string `json:"route_target"`
	Prefix      int    `json:"prefix"`
}

// IPv6SLAAC represents an IPv6 SLAAC or link-local address.
type IPv6SLAAC struct {
	Address    string `json:"address"`
	Gateway    string `json:"gateway"`
	SubnetMask string `json:"subnet_mask"`
	Type       string `json:"type"`
	RDNS       string `json:"rdns"`
	Region     string `json:"region"`
	Prefix     int    `json:"prefix"`
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

// InstanceBackup represents a detailed backup of a Linode instance.
type InstanceBackup struct {
	Label     string               `json:"label"`
	Status    string               `json:"status"`
	Type      string               `json:"type"`
	Created   string               `json:"created"`
	Updated   string               `json:"updated"`
	Finished  string               `json:"finished"`
	Region    string               `json:"region"`
	Configs   []string             `json:"configs"`
	Disks     []InstanceBackupDisk `json:"disks"`
	ID        int                  `json:"id"`
	Available bool                 `json:"available"`
}

// InstanceBackupDisk represents a disk within an instance backup.
type InstanceBackupDisk struct {
	Label      string `json:"label"`
	Filesystem string `json:"filesystem"`
	Size       int    `json:"size"`
}

// InstanceBackupSnapshots holds the current and in-progress snapshot references.
type InstanceBackupSnapshots struct {
	Current    *InstanceBackup `json:"current"`
	InProgress *InstanceBackup `json:"in_progress"`
}

// InstanceBackupsResponse represents the response from the instance backups endpoint.
// The API returns automatic backups as an array and snapshots as a nested object.
type InstanceBackupsResponse struct {
	Snapshot  InstanceBackupSnapshots `json:"snapshot"`
	Automatic []InstanceBackup        `json:"automatic"`
}

// InstanceConfig represents a Linode configuration profile.
type InstanceConfig struct {
	Devices     map[string]*ConfigDevice  `json:"devices,omitempty"`
	Helpers     *ConfigHelpers            `json:"helpers,omitempty"`
	Comments    string                    `json:"comments,omitempty"`
	RootDevice  string                    `json:"root_device,omitempty"`
	RunLevel    string                    `json:"run_level,omitempty"`
	VirtMode    string                    `json:"virt_mode,omitempty"`
	Kernel      string                    `json:"kernel,omitempty"`
	Label       string                    `json:"label"`
	Created     string                    `json:"created,omitempty"`
	Updated     string                    `json:"updated,omitempty"`
	Interfaces  []ConfigInterfaceResponse `json:"interfaces,omitempty"`
	ID          int                       `json:"id"`
	MemoryLimit int                       `json:"memory_limit,omitempty"`
}

// InstanceDisk represents a disk attached to a Linode instance.
type InstanceDisk struct {
	Label      string `json:"label"`
	Status     string `json:"status"`
	Filesystem string `json:"filesystem"`
	Created    string `json:"created"`
	Updated    string `json:"updated"`
	ID         int    `json:"id"`
	Size       int    `json:"size"`
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

// InterfaceDefaultRoute controls whether the interface owns the default route
// for each address family. A field is sent only when true; false values are
// omitted from the wire so the API treats them as unset.
type InterfaceDefaultRoute struct {
	IPv4 bool `json:"ipv4,omitempty"`
	IPv6 bool `json:"ipv6,omitempty"`
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

// InterfaceVLANConfig holds VLAN-attached-interface configuration.
type InterfaceVLANConfig struct {
	Label       string `json:"vlan_label"`
	IPAMAddress string `json:"ipam_address,omitempty"`
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

// LKEAPIEndpoint represents an API endpoint for an LKE cluster.
type LKEAPIEndpoint struct {
	Endpoint string `json:"endpoint"`
}

// LKECluster represents a Linode Kubernetes Engine cluster.
type LKECluster struct {
	Label        string          `json:"label"`
	Region       string          `json:"region"`
	K8sVersion   string          `json:"k8s_version"`
	Status       string          `json:"status"`
	Created      string          `json:"created"`
	Updated      string          `json:"updated"`
	Tags         []string        `json:"tags"`
	ID           int             `json:"id"`
	ControlPlane LKEControlPlane `json:"control_plane"`
}

// LKEControlPlane represents the control plane configuration of an LKE cluster.
type LKEControlPlane struct {
	HighAvailability bool `json:"high_availability"`
}

// LKEDashboard holds the dashboard URL for an LKE cluster.
type LKEDashboard struct {
	URL string `json:"url"`
}

// LKEKubeconfig holds the base64-encoded kubeconfig for an LKE cluster.
type LKEKubeconfig struct {
	Kubeconfig string `json:"kubeconfig"`
}

// LKENode represents a node within an LKE node pool.
type LKENode struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	InstanceID int    `json:"instance_id"`
}

// LKENodePool represents a node pool within an LKE cluster.
type LKENodePool struct {
	Autoscaler *LKENodePoolAutoscaler `json:"autoscaler"`
	Type       string                 `json:"type"`
	Disks      []LKENodePoolDisk      `json:"disks"`
	Nodes      []LKENode              `json:"nodes"`
	Tags       []string               `json:"tags"`
	ID         int                    `json:"id"`
	ClusterID  int                    `json:"cluster_id"`
	Count      int                    `json:"count"`
}

// LKENodePoolAutoscaler represents autoscaling settings for a node pool.
type LKENodePoolAutoscaler struct {
	Enabled bool `json:"enabled"`
	Min     int  `json:"min"`
	Max     int  `json:"max"`
}

// LKENodePoolDisk represents a disk configuration in a node pool.
type LKENodePoolDisk struct {
	Type string `json:"type"`
	Size int    `json:"size"`
}

// LKERegionPrice represents region-specific pricing for an LKE type.
type LKERegionPrice struct {
	ID      string  `json:"id"`
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

// LKETierVersion represents an LKE tier version.
type LKETierVersion struct {
	ID   string `json:"id"`
	Tier string `json:"tier"`
}

// LKEType represents a node type available for LKE clusters.
type LKEType struct {
	ID           string           `json:"id"`
	Label        string           `json:"label"`
	RegionPrices []LKERegionPrice `json:"region_prices"`
	Price        LKETypePrice     `json:"price"`
	Transfer     int              `json:"transfer"`
}

// LKETypePrice represents pricing for an LKE type.
type LKETypePrice struct {
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

// LKEVersion represents an available Kubernetes version for LKE.
type LKEVersion struct {
	ID string `json:"id"`
}

// LongviewApps describes the application monitors enabled for a Longview client.
type LongviewApps struct {
	Apache bool `json:"apache"`
	MySQL  bool `json:"mysql"`
	Nginx  bool `json:"nginx"`
}

// LongviewClient represents a Longview client monitor.
type LongviewClient struct {
	Created string       `json:"created"`
	Label   string       `json:"label"`
	Updated string       `json:"updated"`
	ID      int          `json:"id"`
	Apps    LongviewApps `json:"apps"`
}

// LongviewSubscription represents the current Longview subscription plan.
type LongviewSubscription struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Price           Price  `json:"price"`
	ClientsIncluded int    `json:"clients_included"`
}

// MaintenancePolicy represents one available Linode maintenance policy.
type MaintenancePolicy struct {
	Slug                  string `json:"slug"`
	Label                 string `json:"label"`
	Description           string `json:"description"`
	Type                  string `json:"type"`
	NotificationPeriodSec int    `json:"notification_period_sec"`
	IsDefault             bool   `json:"is_default"`
}

// ManagedContact represents a contact for Linode Managed service alerts.
type ManagedContact struct {
	Phone   ManagedContactPhone `json:"phone"`
	Group   *string             `json:"group"`
	Name    string              `json:"name"`
	Email   string              `json:"email"`
	Updated string              `json:"updated"`
	ID      int                 `json:"id"`
}

// ManagedContactPhone contains primary and secondary phone numbers for a Managed contact.
type ManagedContactPhone struct {
	Primary   *string `json:"primary"`
	Secondary *string `json:"secondary"`
}

// ManagedCredential represents one stored credential returned by GET /managed/credentials.
type ManagedCredential struct {
	Label         string `json:"label"`
	LastDecrypted string `json:"last_decrypted"`
	ID            int    `json:"id"`
}

// ManagedIssue represents an issue detected by Linode Managed service monitors.
type ManagedIssue struct {
	Entity   ManagedIssueEntity `json:"entity"`
	Created  string             `json:"created"`
	Services []int              `json:"services"`
	ID       int                `json:"id"`
}

// ManagedIssueEntity identifies the support ticket opened for a Managed issue.
type ManagedIssueEntity struct {
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
	ID    int    `json:"id"`
}

// ManagedLinodeSettings represents Managed service settings for a Linode.
type ManagedLinodeSettings struct {
	Label string                   `json:"label"`
	Group string                   `json:"group"`
	SSH   ManagedLinodeSettingsSSH `json:"ssh"`
	ID    int                      `json:"id"`
}

// ManagedLinodeSettingsSSH contains SSH access settings for Managed service responders.
type ManagedLinodeSettingsSSH struct {
	Port   *int    `json:"port"`
	User   *string `json:"user"`
	IP     string  `json:"ip"`
	Access bool    `json:"access"`
}

// ManagedService represents a service monitored by Linode Managed.
type ManagedService struct {
	Body              *string `json:"body"`
	Region            *string `json:"region"`
	Notes             *string `json:"notes"`
	Status            string  `json:"status"`
	Address           string  `json:"address"`
	ConsultationGroup string  `json:"consultation_group"`
	Created           string  `json:"created"`
	ServiceType       string  `json:"service_type"`
	Label             string  `json:"label"`
	Updated           string  `json:"updated"`
	Credentials       []int   `json:"credentials"`
	ID                int     `json:"id"`
	Timeout           int     `json:"timeout"`
}

// NetworkTransferPrice represents a network transfer price entry.
type NetworkTransferPrice struct {
	ID           string                       `json:"id"`
	Label        string                       `json:"label"`
	RegionPrices []NetworkTransferRegionPrice `json:"region_prices"`
	Price        Price                        `json:"price"`
	Transfer     int                          `json:"transfer"`
}

// NetworkTransferRegionPrice represents a region-specific network transfer price.
type NetworkTransferRegionPrice struct {
	ID      string  `json:"id"`
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

// NodeBalancer represents a Linode NodeBalancer (load balancer).
type NodeBalancer struct {
	Label              string   `json:"label"`
	Region             string   `json:"region"`
	Hostname           string   `json:"hostname"`
	IPv4               string   `json:"ipv4"`
	IPv6               string   `json:"ipv6"`
	Created            string   `json:"created"`
	Updated            string   `json:"updated"`
	Tags               []string `json:"tags"`
	Transfer           Transfer `json:"transfer"`
	ID                 int      `json:"id"`
	ClientConnThrottle int      `json:"client_conn_throttle"`
}

// NodeBalancerConfig represents a NodeBalancer frontend configuration.
type NodeBalancerConfig struct {
	CipherSuite    string                  `json:"cipher_suite"`
	CheckPath      string                  `json:"check_path"`
	Protocol       string                  `json:"protocol"`
	Algorithm      string                  `json:"algorithm"`
	Stickiness     string                  `json:"stickiness"`
	Check          string                  `json:"check"`
	SSLFingerprint string                  `json:"ssl_fingerprint"`
	SSLCommonName  string                  `json:"ssl_commonname"`
	CheckBody      string                  `json:"check_body"`
	NodesStatus    NodeBalancerNodesStatus `json:"nodes_status"`
	Port           int                     `json:"port"`
	CheckAttempts  int                     `json:"check_attempts"`
	ID             int                     `json:"id"`
	CheckTimeout   int                     `json:"check_timeout"`
	CheckInterval  int                     `json:"check_interval"`
	NodeBalancerID int                     `json:"nodebalancer_id"`
	CheckPassive   bool                    `json:"check_passive"`
}

// NodeBalancerNodesStatus represents the health summary for nodes on a NodeBalancer config.
type NodeBalancerNodesStatus struct {
	Up   int `json:"up"`
	Down int `json:"down"`
}

// OAuthClient represents an OAuth client registered on the account.
type OAuthClient struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	RedirectURI  string `json:"redirect_uri"`
	Status       string `json:"status"`
	ThumbnailURL string `json:"thumbnail_url"`
	Public       bool   `json:"public"`
}

// OAuthClientSecret represents the response from resetting an OAuth client secret.
type OAuthClientSecret struct {
	Secret string `json:"secret"`
}

// ObjectACL represents the ACL of an object in Object Storage.
type ObjectACL struct {
	ACL    string `json:"acl"`
	ACLXML string `json:"acl_xml"`
}

// ObjectStorageBucket represents a Linode Object Storage bucket.
type ObjectStorageBucket struct {
	Label    string `json:"label"`
	Region   string `json:"region"`
	Hostname string `json:"hostname"`
	Created  string `json:"created"`
	Cluster  string `json:"cluster"`
	Objects  int    `json:"objects"`
	Size     int    `json:"size"`
}

// ObjectStorageBucketAccess represents bucket ACL and CORS settings.
type ObjectStorageBucketAccess struct {
	ACL         string `json:"acl"`
	CORSEnabled bool   `json:"cors_enabled"`
}

// ObjectStorageEndpoint represents an Object Storage endpoint.
type ObjectStorageEndpoint struct {
	Region       string  `json:"region"`
	S3Endpoint   *string `json:"s3_endpoint"`
	EndpointType string  `json:"endpoint_type"`
}

// ObjectStorageKey represents a Linode Object Storage access key.
type ObjectStorageKey struct {
	Label        string                         `json:"label"`
	AccessKey    string                         `json:"access_key"`
	SecretKey    string                         `json:"secret_key"`
	BucketAccess []ObjectStorageKeyBucketAccess `json:"bucket_access"`
	Regions      []ObjectStorageKeyRegion       `json:"regions"`
	ID           int                            `json:"id"`
	Limited      bool                           `json:"limited"`
}

// ObjectStorageKeyBucketAccess represents bucket-level permissions for an access key.
type ObjectStorageKeyBucketAccess struct {
	BucketName  string `json:"bucket_name"`
	Region      string `json:"region"`
	Permissions string `json:"permissions"`
}

// ObjectStorageKeyRegion represents a region associated with an Object Storage key.
type ObjectStorageKeyRegion struct {
	ID         string `json:"id"`
	S3Endpoint string `json:"s3_endpoint"`
}

// ObjectStorageQuota represents Object Storage quota metadata.
type ObjectStorageQuota map[string]any

// ObjectStorageRegionPrice represents region-specific Object Storage pricing.
type ObjectStorageRegionPrice struct {
	ID      string  `json:"id"`
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

// ObjectStorageType represents Object Storage pricing and type info.
type ObjectStorageType struct {
	ID           string                     `json:"id"`
	Label        string                     `json:"label"`
	RegionPrices []ObjectStorageRegionPrice `json:"region_prices"`
	Price        Price                      `json:"price"`
	Transfer     int                        `json:"transfer"`
}

// PaginatedResponse represents a standard Linode API paginated response.
type PaginatedResponse[T any] struct {
	Data    []T `json:"data"`
	Page    int `json:"page"`
	Pages   int `json:"pages"`
	Results int `json:"results"`
}

// PlacementGroup represents a Linode placement group.
type PlacementGroup struct {
	Migrations           *PlacementGroupMigrations `json:"migrations,omitempty"`
	Label                string                    `json:"label"`
	Region               string                    `json:"region"`
	PlacementGroupType   string                    `json:"placement_group_type"`
	PlacementGroupPolicy string                    `json:"placement_group_policy"`
	Members              []PlacementGroupMember    `json:"members,omitempty"`
	ID                   int                       `json:"id"`
	IsCompliant          bool                      `json:"is_compliant"`
}

// PlacementGroupMember represents one Linode instance assigned to a placement group.
type PlacementGroupMember struct {
	LinodeID    int  `json:"linode_id"`
	IsCompliant bool `json:"is_compliant"`
}

// PlacementGroupMigration represents a compute instance migrating into or out of a placement group.
type PlacementGroupMigration struct {
	LinodeID int `json:"linode_id"`
}

// PlacementGroupMigrations represents placement group migration state.
type PlacementGroupMigrations struct {
	Inbound  []PlacementGroupMigration `json:"inbound"`
	Outbound []PlacementGroupMigration `json:"outbound"`
}

// PlacementGroupUnassignRequest represents the request body for unassigning Linodes from a placement group.
type PlacementGroupUnassignRequest struct {
	Linodes []int `json:"linodes,omitempty"`
}

// Price represents pricing for a Linode type.
type Price struct {
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

// ProfileToken represents a personal access token on the authenticated profile.
type ProfileToken map[string]any

// Promo represents an active promotion on an account.
type Promo struct {
	Description              string `json:"description"`
	Summary                  string `json:"summary"`
	CreditMonthlyCap         string `json:"credit_monthly_cap"`
	CreditRemaining          string `json:"credit_remaining"`
	ExpireDT                 string `json:"expire_dt"`
	ImageURL                 string `json:"image_url"`
	ServiceType              string `json:"service_type"`
	ThisMonthCreditRemaining string `json:"this_month_credit_remaining"`
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

// Resolver represents DNS resolvers for a region.
type Resolver struct {
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6"`
}

// SSHKey represents an SSH key in a user's profile.
type SSHKey struct {
	Label   string `json:"label"`
	SSHKey  string `json:"ssh_key"`
	Created string `json:"created"`
	ID      int    `json:"id"`
}

// Schedule represents backup schedule settings.
type Schedule struct {
	Day    string `json:"day"`
	Window string `json:"window"`
}

// Specs represents instance hardware specifications.
type Specs struct {
	Disk     int `json:"disk"`
	Memory   int `json:"memory"`
	VCPUs    int `json:"vcpus"`
	GPUs     int `json:"gpus"`
	Transfer int `json:"transfer"`
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

// SupportTicket represents one support ticket returned by GET /support/tickets.
type SupportTicket struct {
	Closed      *string                   `json:"closed"`
	Entity      *SupportTicketEntity      `json:"entity"`
	Status      string                    `json:"status"`
	Description string                    `json:"description"`
	GravatarID  string                    `json:"gravatar_id"`
	Opened      string                    `json:"opened"`
	OpenedBy    string                    `json:"opened_by"`
	Summary     string                    `json:"summary"`
	Updated     string                    `json:"updated"`
	UpdatedBy   string                    `json:"updated_by"`
	Attachments []SupportTicketAttachment `json:"attachments"`
	ID          int                       `json:"id"`
	Closable    bool                      `json:"closable"`
}

// SupportTicketAttachment represents one attachment on a support ticket.
type SupportTicketAttachment struct {
	Filename string `json:"filename"`
	ID       int    `json:"id"`
	Size     int    `json:"size"`
}

// SupportTicketEntity identifies the API entity attached to a support ticket.
type SupportTicketEntity struct {
	ID    any    `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

// SupportTicketReply represents one reply returned by GET /support/tickets/{ticket_id}/replies.
type SupportTicketReply struct {
	Created     string `json:"created"`
	CreatedBy   string `json:"created_by"`
	Description string `json:"description"`
	GravatarID  string `json:"gravatar_id"`
	Updated     string `json:"updated"`
	UpdatedBy   string `json:"updated_by"`
	ID          int    `json:"id"`
}

// Tag represents a Linode tag.
type Tag struct {
	Label         string `json:"label"`
	Domains       []int  `json:"domains,omitempty"`
	Linodes       []int  `json:"linodes,omitempty"`
	NodeBalancers []int  `json:"nodebalancers,omitempty"`
	Volumes       []int  `json:"volumes,omitempty"`
}

// TaggedObject represents one resource returned by GET /tags/{tagLabel}.
// The Linode API can return several resource shapes in this list, so keep the
// object flexible while preserving the paginated response envelope.
type TaggedObject map[string]any

// Transfer represents data transfer statistics.
type Transfer struct {
	In    float64 `json:"in"`
	Out   float64 `json:"out"`
	Total float64 `json:"total"`
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

// UpdateSSHKeyRequest represents the request body for updating an SSH key.
type UpdateSSHKeyRequest struct {
	Label string `json:"label"`
}

// VLAN represents a Linode VLAN.
type VLAN struct {
	Label   string `json:"label"`
	Region  string `json:"region"`
	Linodes []int  `json:"linodes"`
}

// VPC represents a Linode Virtual Private Cloud.
type VPC struct {
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Region      string      `json:"region"`
	Created     string      `json:"created"`
	Updated     string      `json:"updated"`
	Subnets     []VPCSubnet `json:"subnets"`
	ID          int         `json:"id"`
}

// VPCIP represents an IP address associated with a VPC.
type VPCIP struct {
	ConfigID     *int    `json:"config_id"`
	AddressRange *string `json:"address_range"`
	Prefix       *int    `json:"prefix"`
	Gateway      *string `json:"gateway"`
	Address      *string `json:"address"`
	LinodeID     *int    `json:"linode_id"`
	NAT1To1      string  `json:"nat_1_1"`
	Region       string  `json:"region"`
	SubnetMask   string  `json:"subnet_mask"`
	InterfaceID  int     `json:"interface_id"`
	SubnetID     int     `json:"subnet_id"`
	VPCID        int     `json:"vpc_id"`
	Active       bool    `json:"active"`
}

// VPCSubnet represents a subnet within a VPC.
type VPCSubnet struct {
	Label   string            `json:"label"`
	IPv4    string            `json:"ipv4"`
	Created string            `json:"created"`
	Updated string            `json:"updated"`
	Linodes []VPCSubnetLinode `json:"linodes"`
	ID      int               `json:"id"`
}

// VPCSubnetLinode represents a Linode assigned to a VPC subnet.
type VPCSubnetLinode struct {
	Interfaces []VPCSubnetLinodeInterface `json:"interfaces"`
	ID         int                        `json:"id"`
}

// VPCSubnetLinodeInterface represents a network interface on a Linode within a VPC subnet.
type VPCSubnetLinodeInterface struct {
	ID       int  `json:"id"`
	Active   bool `json:"active"`
	ConfigID int  `json:"config_id"`
}

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
