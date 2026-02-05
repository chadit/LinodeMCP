// Package linode provides a client for interacting with the Linode API v4.
package linode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/chadit/LinodeMCP/internal/version"
)

// Client represents a Linode API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// ClientInterface defines the interface for Linode API operations.
//
//nolint:interfacebloat // API client interface grows with supported operations
type ClientInterface interface {
	// Read operations.
	GetProfile(ctx context.Context) (*Profile, error)
	GetAccount(ctx context.Context) (*Account, error)
	ListInstances(ctx context.Context) ([]Instance, error)
	GetInstance(ctx context.Context, instanceID int) (*Instance, error)
	ListRegions(ctx context.Context) ([]Region, error)
	ListTypes(ctx context.Context) ([]InstanceType, error)
	ListVolumes(ctx context.Context) ([]Volume, error)
	ListImages(ctx context.Context) ([]Image, error)
	ListSSHKeys(ctx context.Context) ([]SSHKey, error)
	ListDomains(ctx context.Context) ([]Domain, error)
	GetDomain(ctx context.Context, domainID int) (*Domain, error)
	ListDomainRecords(ctx context.Context, domainID int) ([]DomainRecord, error)
	ListFirewalls(ctx context.Context) ([]Firewall, error)
	GetFirewall(ctx context.Context, firewallID int) (*Firewall, error)
	ListNodeBalancers(ctx context.Context) ([]NodeBalancer, error)
	GetNodeBalancer(ctx context.Context, nodeBalancerID int) (*NodeBalancer, error)
	ListStackScripts(ctx context.Context) ([]StackScript, error)
	GetVolume(ctx context.Context, volumeID int) (*Volume, error)

	// Stage 4: Write operations - SSH Keys.
	CreateSSHKey(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error)
	DeleteSSHKey(ctx context.Context, sshKeyID int) error

	// Stage 4: Write operations - Instances.
	BootInstance(ctx context.Context, instanceID int, configID *int) error
	RebootInstance(ctx context.Context, instanceID int, configID *int) error
	ShutdownInstance(ctx context.Context, instanceID int) error
	CreateInstance(ctx context.Context, req CreateInstanceRequest) (*Instance, error)
	DeleteInstance(ctx context.Context, instanceID int) error
	ResizeInstance(ctx context.Context, instanceID int, req ResizeInstanceRequest) error

	// Stage 4: Write operations - Firewalls.
	CreateFirewall(ctx context.Context, req CreateFirewallRequest) (*Firewall, error)
	UpdateFirewall(ctx context.Context, firewallID int, req UpdateFirewallRequest) (*Firewall, error)
	DeleteFirewall(ctx context.Context, firewallID int) error

	// Stage 4: Write operations - Domains.
	CreateDomain(ctx context.Context, req CreateDomainRequest) (*Domain, error)
	UpdateDomain(ctx context.Context, domainID int, req UpdateDomainRequest) (*Domain, error)
	DeleteDomain(ctx context.Context, domainID int) error
	CreateDomainRecord(ctx context.Context, domainID int, req CreateDomainRecordRequest) (*DomainRecord, error)
	UpdateDomainRecord(ctx context.Context, domainID, recordID int, req UpdateDomainRecordRequest) (*DomainRecord, error)
	DeleteDomainRecord(ctx context.Context, domainID, recordID int) error

	// Stage 4: Write operations - Volumes.
	CreateVolume(ctx context.Context, req CreateVolumeRequest) (*Volume, error)
	AttachVolume(ctx context.Context, volumeID int, req AttachVolumeRequest) (*Volume, error)
	DetachVolume(ctx context.Context, volumeID int) error
	ResizeVolume(ctx context.Context, volumeID int, size int) (*Volume, error)
	DeleteVolume(ctx context.Context, volumeID int) error

	// Stage 4: Write operations - NodeBalancers.
	CreateNodeBalancer(ctx context.Context, req CreateNodeBalancerRequest) (*NodeBalancer, error)
	UpdateNodeBalancer(ctx context.Context, nodeBalancerID int, req UpdateNodeBalancerRequest) (*NodeBalancer, error)
	DeleteNodeBalancer(ctx context.Context, nodeBalancerID int) error
}

// Profile represents a Linode user profile.
type Profile struct {
	Username           string `json:"username"`
	Email              string `json:"email"`
	Timezone           string `json:"timezone"`
	EmailNotifications bool   `json:"email_notifications"` //nolint:tagliatelle // Linode API snake_case
	Restricted         bool   `json:"restricted"`
	TwoFactorAuth      bool   `json:"two_factor_auth"` //nolint:tagliatelle // Linode API snake_case
	UID                int    `json:"uid"`
}

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
	WatchdogEnabled bool     `json:"watchdog_enabled"` //nolint:tagliatelle // Linode API snake_case
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
	NetworkIn     int `json:"network_in"`     //nolint:tagliatelle // Linode API snake_case
	NetworkOut    int `json:"network_out"`    //nolint:tagliatelle // Linode API snake_case
	TransferQuota int `json:"transfer_quota"` //nolint:tagliatelle // Linode API snake_case
	IO            int `json:"io"`
}

// Backups represents backup settings.
type Backups struct {
	Enabled   bool     `json:"enabled"`
	Available bool     `json:"available"`
	Schedule  Schedule `json:"schedule"`
	Last      *Backup  `json:"last_successful"` //nolint:tagliatelle // Linode API snake_case
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

// Account represents a Linode account.
type Account struct {
	FirstName         string   `json:"first_name"` //nolint:tagliatelle // Linode API snake_case
	LastName          string   `json:"last_name"`  //nolint:tagliatelle // Linode API snake_case
	Email             string   `json:"email"`
	Company           string   `json:"company"`
	Address1          string   `json:"address_1"` //nolint:tagliatelle // Linode API snake_case
	Address2          string   `json:"address_2"` //nolint:tagliatelle // Linode API snake_case
	City              string   `json:"city"`
	State             string   `json:"state"`
	Zip               string   `json:"zip"`
	Country           string   `json:"country"`
	Phone             string   `json:"phone"`
	Balance           float64  `json:"balance"`
	BalanceUninvoiced float64  `json:"balance_uninvoiced"` //nolint:tagliatelle // Linode API snake_case
	Capabilities      []string `json:"capabilities"`
	ActiveSince       string   `json:"active_since"` //nolint:tagliatelle // Linode API snake_case
	EUUID             string   `json:"euuid"`
	BillingSource     string   `json:"billing_source"`    //nolint:tagliatelle // Linode API snake_case
	ActivePromotions  []Promo  `json:"active_promotions"` //nolint:tagliatelle // Linode API snake_case
}

// Promo represents an active promotion on an account.
type Promo struct {
	Description              string `json:"description"`
	Summary                  string `json:"summary"`
	CreditMonthlyCap         string `json:"credit_monthly_cap"`          //nolint:tagliatelle // Linode API snake_case
	CreditRemaining          string `json:"credit_remaining"`            //nolint:tagliatelle // Linode API snake_case
	ExpireDT                 string `json:"expire_dt"`                   //nolint:tagliatelle // Linode API snake_case
	ImageURL                 string `json:"image_url"`                   //nolint:tagliatelle // Linode API snake_case
	ServiceType              string `json:"service_type"`                //nolint:tagliatelle // Linode API snake_case
	ThisMonthCreditRemaining string `json:"this_month_credit_remaining"` //nolint:tagliatelle // Linode API snake_case
}

// Region represents a Linode region (datacenter).
type Region struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Country      string   `json:"country"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
	Resolvers    Resolver `json:"resolvers"`
	SiteType     string   `json:"site_type"` //nolint:tagliatelle // Linode API snake_case
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
	NetworkOut int     `json:"network_out"` //nolint:tagliatelle // Linode API snake_case
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

// Image represents a Linode image (OS image or custom image).
type Image struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	IsPublic     bool     `json:"is_public"` //nolint:tagliatelle // Linode API snake_case
	Deprecated   bool     `json:"deprecated"`
	Size         int      `json:"size"`
	Vendor       string   `json:"vendor"`
	Status       string   `json:"status"`
	Created      string   `json:"created"`
	CreatedBy    string   `json:"created_by"` //nolint:tagliatelle // Linode API snake_case
	Expiry       *string  `json:"expiry"`
	EOL          *string  `json:"eol"`
	Capabilities []string `json:"capabilities"`
	Tags         []string `json:"tags"`
}

// SSHKey represents an SSH key in a user's profile.
type SSHKey struct {
	ID      int    `json:"id"`
	Label   string `json:"label"`
	SSHKey  string `json:"ssh_key"` //nolint:tagliatelle // Linode API snake_case
	Created string `json:"created"`
}

// Domain represents a Linode DNS domain.
type Domain struct {
	ID          int      `json:"id"`
	Domain      string   `json:"domain"`
	Type        string   `json:"type"`      // master, slave
	Status      string   `json:"status"`    // active, disabled, edit_mode
	SOAEmail    string   `json:"soa_email"` //nolint:tagliatelle // Linode API snake_case
	Description string   `json:"description"`
	RetrySec    int      `json:"retry_sec"`   //nolint:tagliatelle // Linode API snake_case
	MasterIPs   []string `json:"master_ips"`  //nolint:tagliatelle // Linode API snake_case
	AXFRIPs     []string `json:"axfr_ips"`    //nolint:tagliatelle // Linode API snake_case
	ExpireSec   int      `json:"expire_sec"`  //nolint:tagliatelle // Linode API snake_case
	RefreshSec  int      `json:"refresh_sec"` //nolint:tagliatelle // Linode API snake_case
	TTLSec      int      `json:"ttl_sec"`     //nolint:tagliatelle // Linode API snake_case
	Tags        []string `json:"tags"`
	Created     string   `json:"created"`
	Updated     string   `json:"updated"`
	Group       string   `json:"group"`
}

// DomainRecord represents a DNS record within a domain.
type DomainRecord struct {
	ID       int    `json:"id"`
	Type     string `json:"type"` // A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, PTR
	Name     string `json:"name"`
	Target   string `json:"target"`
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
	Port     int    `json:"port"`
	Service  string `json:"service"`
	Protocol string `json:"protocol"`
	TTLSec   int    `json:"ttl_sec"` //nolint:tagliatelle // Linode API snake_case
	Tag      string `json:"tag"`
	Created  string `json:"created"`
	Updated  string `json:"updated"`
}

// Firewall represents a Linode Cloud Firewall.
type Firewall struct {
	ID      int           `json:"id"`
	Label   string        `json:"label"`
	Status  string        `json:"status"` // enabled, disabled, deleted
	Rules   FirewallRules `json:"rules"`
	Tags    []string      `json:"tags"`
	Created string        `json:"created"`
	Updated string        `json:"updated"`
}

// FirewallRules represents inbound and outbound firewall rules.
type FirewallRules struct {
	Inbound        []FirewallRule `json:"inbound"`
	InboundPolicy  string         `json:"inbound_policy"` //nolint:tagliatelle // Linode API snake_case
	Outbound       []FirewallRule `json:"outbound"`
	OutboundPolicy string         `json:"outbound_policy"` //nolint:tagliatelle // Linode API snake_case
}

// FirewallRule represents a single firewall rule.
type FirewallRule struct {
	Action      string            `json:"action"`   // ACCEPT, DROP
	Protocol    string            `json:"protocol"` // TCP, UDP, ICMP, IPENCAP
	Ports       string            `json:"ports"`
	Addresses   FirewallAddresses `json:"addresses"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
}

// FirewallAddresses represents IPv4 and IPv6 addresses for a firewall rule.
type FirewallAddresses struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// NodeBalancer represents a Linode NodeBalancer (load balancer).
type NodeBalancer struct {
	ID                 int      `json:"id"`
	Label              string   `json:"label"`
	Region             string   `json:"region"`
	Hostname           string   `json:"hostname"`
	IPv4               string   `json:"ipv4"`
	IPv6               string   `json:"ipv6"`
	ClientConnThrottle int      `json:"client_conn_throttle"` //nolint:tagliatelle // Linode API snake_case
	Transfer           Transfer `json:"transfer"`
	Tags               []string `json:"tags"`
	Created            string   `json:"created"`
	Updated            string   `json:"updated"`
}

// Transfer represents data transfer statistics.
type Transfer struct {
	In    float64 `json:"in"`
	Out   float64 `json:"out"`
	Total float64 `json:"total"`
}

// StackScript represents a Linode StackScript for automated deployments.
type StackScript struct {
	ID                int      `json:"id"`
	Username          string   `json:"username"`
	UserGravatarID    string   `json:"user_gravatar_id"` //nolint:tagliatelle // Linode API snake_case
	Label             string   `json:"label"`
	Description       string   `json:"description"`
	Images            []string `json:"images"`
	DeploymentsTotal  int      `json:"deployments_total"`  //nolint:tagliatelle // Linode API snake_case
	DeploymentsActive int      `json:"deployments_active"` //nolint:tagliatelle // Linode API snake_case
	IsPublic          bool     `json:"is_public"`          //nolint:tagliatelle // Linode API snake_case
	Mine              bool     `json:"mine"`
	Created           string   `json:"created"`
	Updated           string   `json:"updated"`
	RevNote           string   `json:"rev_note"` //nolint:tagliatelle // Linode API snake_case
	Script            string   `json:"script"`
	UserDefinedFields []UDF    `json:"user_defined_fields"` //nolint:tagliatelle // Linode API snake_case
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

// Stage 4: Request types for write operations.

// CreateSSHKeyRequest represents the request body for creating an SSH key.
type CreateSSHKeyRequest struct {
	Label  string `json:"label"`
	SSHKey string `json:"ssh_key"` //nolint:tagliatelle // Linode API snake_case
}

// CreateInstanceRequest represents the request body for creating a Linode instance.
type CreateInstanceRequest struct {
	Region          string   `json:"region"`
	Type            string   `json:"type"`
	Label           string   `json:"label,omitempty"`
	Image           string   `json:"image,omitempty"`
	RootPass        string   `json:"root_pass,omitempty"`        //nolint:tagliatelle // Linode API snake_case
	AuthorizedKeys  []string `json:"authorized_keys,omitempty"`  //nolint:tagliatelle // Linode API snake_case
	AuthorizedUsers []string `json:"authorized_users,omitempty"` //nolint:tagliatelle // Linode API snake_case
	StackScriptID   *int     `json:"stackscript_id,omitempty"`   //nolint:tagliatelle // Linode API snake_case
	StackScriptData any      `json:"stackscript_data,omitempty"` //nolint:tagliatelle // Linode API snake_case
	BackupsEnabled  bool     `json:"backups_enabled,omitempty"`  //nolint:tagliatelle // Linode API snake_case
	SwapSize        *int     `json:"swap_size,omitempty"`        //nolint:tagliatelle // Linode API snake_case
	PrivateIP       bool     `json:"private_ip,omitempty"`       //nolint:tagliatelle // Linode API snake_case
	Tags            []string `json:"tags,omitempty"`
	Booted          *bool    `json:"booted,omitempty"`
}

// ResizeInstanceRequest represents the request body for resizing a Linode instance.
type ResizeInstanceRequest struct {
	Type          string `json:"type"`
	AllowAutoDisk bool   `json:"allow_auto_disk,omitempty"` //nolint:tagliatelle // Linode API snake_case
	MigrationType string `json:"migration_type,omitempty"`  //nolint:tagliatelle // Linode API snake_case (cold, warm)
}

// CreateFirewallRequest represents the request body for creating a firewall.
type CreateFirewallRequest struct {
	Label   string         `json:"label"`
	Rules   *FirewallRules `json:"rules,omitempty"`
	Tags    []string       `json:"tags,omitempty"`
	Devices []Device       `json:"devices,omitempty"`
}

// Device represents a device attached to a firewall.
type Device struct {
	ID   int    `json:"id"`
	Type string `json:"type"` // linode, nodebalancer
}

// UpdateFirewallRequest represents the request body for updating a firewall.
type UpdateFirewallRequest struct {
	Label  string         `json:"label,omitempty"`
	Status string         `json:"status,omitempty"` // enabled, disabled
	Rules  *FirewallRules `json:"rules,omitempty"`
	Tags   []string       `json:"tags,omitempty"`
}

// CreateDomainRequest represents the request body for creating a domain.
type CreateDomainRequest struct {
	Domain      string   `json:"domain"`
	Type        string   `json:"type"`                // master, slave
	SOAEmail    string   `json:"soa_email,omitempty"` //nolint:tagliatelle // Linode API snake_case
	Description string   `json:"description,omitempty"`
	RetrySec    int      `json:"retry_sec,omitempty"`   //nolint:tagliatelle // Linode API snake_case
	MasterIPs   []string `json:"master_ips,omitempty"`  //nolint:tagliatelle // Linode API snake_case
	AXFRIPs     []string `json:"axfr_ips,omitempty"`    //nolint:tagliatelle // Linode API snake_case
	ExpireSec   int      `json:"expire_sec,omitempty"`  //nolint:tagliatelle // Linode API snake_case
	RefreshSec  int      `json:"refresh_sec,omitempty"` //nolint:tagliatelle // Linode API snake_case
	TTLSec      int      `json:"ttl_sec,omitempty"`     //nolint:tagliatelle // Linode API snake_case
	Tags        []string `json:"tags,omitempty"`
	Group       string   `json:"group,omitempty"`
}

// UpdateDomainRequest represents the request body for updating a domain.
type UpdateDomainRequest struct {
	Domain      string   `json:"domain,omitempty"`
	Status      string   `json:"status,omitempty"`    // active, disabled, edit_mode
	SOAEmail    string   `json:"soa_email,omitempty"` //nolint:tagliatelle // Linode API snake_case
	Description string   `json:"description,omitempty"`
	RetrySec    int      `json:"retry_sec,omitempty"`   //nolint:tagliatelle // Linode API snake_case
	MasterIPs   []string `json:"master_ips,omitempty"`  //nolint:tagliatelle // Linode API snake_case
	AXFRIPs     []string `json:"axfr_ips,omitempty"`    //nolint:tagliatelle // Linode API snake_case
	ExpireSec   int      `json:"expire_sec,omitempty"`  //nolint:tagliatelle // Linode API snake_case
	RefreshSec  int      `json:"refresh_sec,omitempty"` //nolint:tagliatelle // Linode API snake_case
	TTLSec      int      `json:"ttl_sec,omitempty"`     //nolint:tagliatelle // Linode API snake_case
	Tags        []string `json:"tags,omitempty"`
	Group       string   `json:"group,omitempty"`
}

// CreateDomainRecordRequest represents the request body for creating a domain record.
type CreateDomainRecordRequest struct {
	Type     string `json:"type"` // A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, PTR
	Name     string `json:"name,omitempty"`
	Target   string `json:"target"`
	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Port     int    `json:"port,omitempty"`
	Service  string `json:"service,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	TTLSec   int    `json:"ttl_sec,omitempty"` //nolint:tagliatelle // Linode API snake_case
	Tag      string `json:"tag,omitempty"`
}

// UpdateDomainRecordRequest represents the request body for updating a domain record.
type UpdateDomainRecordRequest struct {
	Name     string `json:"name,omitempty"`
	Target   string `json:"target,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Port     int    `json:"port,omitempty"`
	Service  string `json:"service,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	TTLSec   int    `json:"ttl_sec,omitempty"` //nolint:tagliatelle // Linode API snake_case
	Tag      string `json:"tag,omitempty"`
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

// CreateNodeBalancerRequest represents the request body for creating a NodeBalancer.
type CreateNodeBalancerRequest struct {
	Region             string   `json:"region"`
	Label              string   `json:"label,omitempty"`
	ClientConnThrottle int      `json:"client_conn_throttle,omitempty"` //nolint:tagliatelle // Linode API snake_case
	Tags               []string `json:"tags,omitempty"`
}

// UpdateNodeBalancerRequest represents the request body for updating a NodeBalancer.
type UpdateNodeBalancerRequest struct {
	Label              string   `json:"label,omitempty"`
	ClientConnThrottle *int     `json:"client_conn_throttle,omitempty"` //nolint:tagliatelle // Linode API snake_case
	Tags               []string `json:"tags,omitempty"`
}

const (
	defaultTimeout     = 30 * time.Second
	defaultIdleTimeout = 30 * time.Second
	maxIdleConns       = 10
	httpBadRequest     = 400
	httpUnauthorized   = 401
	httpForbidden      = 403
	httpTooManyReqs    = 429
	httpServerError    = 500
	authHeaderPrefix   = "Bearer "
	contentTypeJSON    = "application/json"
	// requestTimeout is the per-request context timeout for API calls.
	requestTimeout = 30 * time.Second

	errMsgAuthentication = "Authentication failed. Please check your API token."
	errMsgForbidden      = "Access forbidden. Your API token may not have sufficient permissions."
	errMsgRateLimit      = "Rate limit exceeded. Please try again later."
	errMsgServerError    = "Internal server error. Please try again later."
)

// NewClient creates a new Linode API client.
func NewClient(apiURL, token string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        maxIdleConns,
				MaxIdleConnsPerHost: maxIdleConns,
				IdleConnTimeout:     defaultIdleTimeout,
			},
		},
		baseURL: apiURL,
		token:   token,
	}
}

// GetProfile retrieves the authenticated user's profile from the Linode API.
func (c *Client) GetProfile(ctx context.Context) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/profile", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfile", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var profile Profile
	if err := c.handleResponse(resp, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// ListInstances retrieves all Linode instances for the authenticated user.
func (c *Client) ListInstances(ctx context.Context) ([]Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/linode/instances", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListInstances", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Instance `json:"data"`
		Page    int        `json:"page"`
		Pages   int        `json:"pages"`
		Results int        `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetInstance retrieves a single Linode instance by its ID.
func (c *Client) GetInstance(ctx context.Context, instanceID int) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/linode/instances/%d", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// GetAccount retrieves the authenticated user's account information from the Linode API.
func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/account", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccount", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var account Account
	if err := c.handleResponse(resp, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// ListRegions retrieves all available Linode regions.
func (c *Client) ListRegions(ctx context.Context) ([]Region, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/regions", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListRegions", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Region `json:"data"`
		Page    int      `json:"page"`
		Pages   int      `json:"pages"`
		Results int      `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListTypes retrieves all available Linode instance types.
func (c *Client) ListTypes(ctx context.Context) ([]InstanceType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/linode/types", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListTypes", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []InstanceType `json:"data"`
		Page    int            `json:"page"`
		Pages   int            `json:"pages"`
		Results int            `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListVolumes retrieves all block storage volumes for the authenticated user.
func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/volumes", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListVolumes", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Volume `json:"data"`
		Page    int      `json:"page"`
		Pages   int      `json:"pages"`
		Results int      `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListImages retrieves all available Linode images.
func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/images", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListImages", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Image `json:"data"`
		Page    int     `json:"page"`
		Pages   int     `json:"pages"`
		Results int     `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListSSHKeys retrieves all SSH keys from the authenticated user's profile.
func (c *Client) ListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/profile/sshkeys", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListSSHKeys", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []SSHKey `json:"data"`
		Page    int      `json:"page"`
		Pages   int      `json:"pages"`
		Results int      `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListDomains retrieves all DNS domains for the authenticated user.
func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/domains", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDomains", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Domain `json:"data"`
		Page    int      `json:"page"`
		Pages   int      `json:"pages"`
		Results int      `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetDomain retrieves a single DNS domain by its ID.
func (c *Client) GetDomain(ctx context.Context, domainID int) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d", domainID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDomain", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var domain Domain
	if err := c.handleResponse(resp, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// ListDomainRecords retrieves all DNS records for a specific domain.
func (c *Client) ListDomainRecords(ctx context.Context, domainID int) ([]DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d/records", domainID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDomainRecords", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []DomainRecord `json:"data"`
		Page    int            `json:"page"`
		Pages   int            `json:"pages"`
		Results int            `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListFirewalls retrieves all Cloud Firewalls for the authenticated user.
func (c *Client) ListFirewalls(ctx context.Context) ([]Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/networking/firewalls", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListFirewalls", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Firewall `json:"data"`
		Page    int        `json:"page"`
		Pages   int        `json:"pages"`
		Results int        `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListNodeBalancers retrieves all NodeBalancers for the authenticated user.
func (c *Client) ListNodeBalancers(ctx context.Context) ([]NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/nodebalancers", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListNodeBalancers", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []NodeBalancer `json:"data"`
		Page    int            `json:"page"`
		Pages   int            `json:"pages"`
		Results int            `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetNodeBalancer retrieves a single NodeBalancer by its ID.
func (c *Client) GetNodeBalancer(ctx context.Context, nodeBalancerID int) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/nodebalancers/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetNodeBalancer", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// ListStackScripts retrieves StackScripts available to the authenticated user.
func (c *Client) ListStackScripts(ctx context.Context) ([]StackScript, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/linode/stackscripts", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListStackScripts", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []StackScript `json:"data"`
		Page    int           `json:"page"`
		Pages   int           `json:"pages"`
		Results int           `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// Stage 4: Write operations.

// GetFirewall retrieves a single firewall by its ID.
func (c *Client) GetFirewall(ctx context.Context, firewallID int) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/networking/firewalls/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetFirewall", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// GetVolume retrieves a single volume by its ID.
func (c *Client) GetVolume(ctx context.Context, volumeID int) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/volumes/%d", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// CreateSSHKey creates a new SSH key in the user's profile.
func (c *Client) CreateSSHKey(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/profile/sshkeys", req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateSSHKey", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var sshKey SSHKey
	if err := c.handleResponse(resp, &sshKey); err != nil {
		return nil, err
	}

	return &sshKey, nil
}

// DeleteSSHKey deletes an SSH key from the user's profile.
func (c *Client) DeleteSSHKey(ctx context.Context, sshKeyID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/profile/sshkeys/%d", sshKeyID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteSSHKey", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// BootInstance boots a Linode instance.
func (c *Client) BootInstance(ctx context.Context, instanceID int, configID *int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/linode/instances/%d/boot", instanceID)

	var payload any
	if configID != nil {
		payload = map[string]int{"config_id": *configID}
	}

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return &NetworkError{Operation: "BootInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// RebootInstance reboots a Linode instance.
func (c *Client) RebootInstance(ctx context.Context, instanceID int, configID *int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/linode/instances/%d/reboot", instanceID)

	var payload any
	if configID != nil {
		payload = map[string]int{"config_id": *configID}
	}

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return &NetworkError{Operation: "RebootInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ShutdownInstance shuts down a Linode instance.
func (c *Client) ShutdownInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/linode/instances/%d/shutdown", instanceID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "ShutdownInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreateInstance creates a new Linode instance.
func (c *Client) CreateInstance(ctx context.Context, req CreateInstanceRequest) (*Instance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/linode/instances", req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var instance Instance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// DeleteInstance deletes a Linode instance.
func (c *Client) DeleteInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/linode/instances/%d", instanceID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ResizeInstance resizes a Linode instance to a new plan.
func (c *Client) ResizeInstance(ctx context.Context, instanceID int, req ResizeInstanceRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/linode/instances/%d/resize", instanceID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "ResizeInstance", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreateFirewall creates a new Cloud Firewall.
func (c *Client) CreateFirewall(ctx context.Context, req CreateFirewallRequest) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/networking/firewalls", req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateFirewall", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// UpdateFirewall updates an existing Cloud Firewall.
func (c *Client) UpdateFirewall(ctx context.Context, firewallID int, req UpdateFirewallRequest) (*Firewall, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/networking/firewalls/%d", firewallID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateFirewall", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var firewall Firewall
	if err := c.handleResponse(resp, &firewall); err != nil {
		return nil, err
	}

	return &firewall, nil
}

// DeleteFirewall deletes a Cloud Firewall.
func (c *Client) DeleteFirewall(ctx context.Context, firewallID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/networking/firewalls/%d", firewallID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteFirewall", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreateDomain creates a new DNS domain.
func (c *Client) CreateDomain(ctx context.Context, req CreateDomainRequest) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/domains", req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateDomain", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var domain Domain
	if err := c.handleResponse(resp, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// UpdateDomain updates an existing DNS domain.
func (c *Client) UpdateDomain(ctx context.Context, domainID int, req UpdateDomainRequest) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d", domainID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateDomain", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var domain Domain
	if err := c.handleResponse(resp, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// DeleteDomain deletes a DNS domain and all its records.
func (c *Client) DeleteDomain(ctx context.Context, domainID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d", domainID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteDomain", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreateDomainRecord creates a new DNS record within a domain.
func (c *Client) CreateDomainRecord(ctx context.Context, domainID int, req CreateDomainRecordRequest) (*DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d/records", domainID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateDomainRecord", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var record DomainRecord
	if err := c.handleResponse(resp, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// UpdateDomainRecord updates an existing DNS record.
func (c *Client) UpdateDomainRecord(ctx context.Context, domainID, recordID int, req UpdateDomainRecordRequest) (*DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d/records/%d", domainID, recordID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateDomainRecord", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var record DomainRecord
	if err := c.handleResponse(resp, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// DeleteDomainRecord deletes a DNS record from a domain.
func (c *Client) DeleteDomainRecord(ctx context.Context, domainID, recordID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d/records/%d", domainID, recordID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteDomainRecord", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreateVolume creates a new block storage volume.
func (c *Client) CreateVolume(ctx context.Context, req CreateVolumeRequest) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/volumes", req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// AttachVolume attaches a volume to a Linode instance.
func (c *Client) AttachVolume(ctx context.Context, volumeID int, req AttachVolumeRequest) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/volumes/%d/attach", volumeID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AttachVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// DetachVolume detaches a volume from a Linode instance.
func (c *Client) DetachVolume(ctx context.Context, volumeID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/volumes/%d/detach", volumeID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DetachVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// ResizeVolume resizes a volume to a larger size.
func (c *Client) ResizeVolume(ctx context.Context, volumeID int, size int) (*Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/volumes/%d/resize", volumeID)
	payload := map[string]int{"size": size}

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, &NetworkError{Operation: "ResizeVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var volume Volume
	if err := c.handleResponse(resp, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// DeleteVolume deletes a block storage volume.
func (c *Client) DeleteVolume(ctx context.Context, volumeID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/volumes/%d", volumeID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteVolume", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreateNodeBalancer creates a new NodeBalancer.
func (c *Client) CreateNodeBalancer(ctx context.Context, req CreateNodeBalancerRequest) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/nodebalancers", req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateNodeBalancer", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// UpdateNodeBalancer updates an existing NodeBalancer.
func (c *Client) UpdateNodeBalancer(ctx context.Context, nodeBalancerID int, req UpdateNodeBalancerRequest) (*NodeBalancer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/nodebalancers/%d", nodeBalancerID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateNodeBalancer", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var nodeBalancer NodeBalancer
	if err := c.handleResponse(resp, &nodeBalancer); err != nil {
		return nil, err
	}

	return &nodeBalancer, nil
}

// DeleteNodeBalancer deletes a NodeBalancer.
func (c *Client) DeleteNodeBalancer(ctx context.Context, nodeBalancerID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/nodebalancers/%d", nodeBalancerID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteNodeBalancer", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// Private helper methods

func (c *Client) makeRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", authHeaderPrefix+c.token)
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("User-Agent", "LinodeMCP/"+version.Version)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func (c *Client) makeJSONRequest(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	var body io.Reader

	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}

		body = bytes.NewReader(jsonData)
	}

	return c.makeRequest(ctx, method, endpoint, body)
}

func (c *Client) handleResponse(resp *http.Response, target any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= httpBadRequest {
		return c.handleErrorResponse(resp.StatusCode, body, resp)
	}

	if target != nil {
		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

func (c *Client) handleErrorResponse(statusCode int, body []byte, resp *http.Response) error {
	var apiError struct {
		Errors []struct {
			Field  string `json:"field"`
			Reason string `json:"reason"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &apiError); err == nil && len(apiError.Errors) > 0 {
		return &APIError{
			StatusCode: statusCode,
			Message:    apiError.Errors[0].Reason,
			Field:      apiError.Errors[0].Field,
		}
	}

	switch statusCode {
	case httpUnauthorized:
		return &APIError{StatusCode: statusCode, Message: errMsgAuthentication}
	case httpForbidden:
		return &APIError{StatusCode: statusCode, Message: errMsgForbidden}
	case httpTooManyReqs:
		retryAfter := c.parseRetryAfter(resp)

		message := errMsgRateLimit
		if retryAfter > 0 {
			message = fmt.Sprintf("Rate limit exceeded. Retry after %v.", retryAfter)
		}

		return &APIError{StatusCode: statusCode, Message: message}
	case httpServerError:
		return &APIError{StatusCode: statusCode, Message: errMsgServerError}
	default:
		return &APIError{StatusCode: statusCode, Message: fmt.Sprintf("API request failed with status %d", statusCode)}
	}
}

func (c *Client) parseRetryAfter(resp *http.Response) time.Duration {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	if t, err := http.ParseTime(retryAfter); err == nil {
		return time.Until(t)
	}

	return 0
}
