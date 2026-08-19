package linode

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

type retryConfig struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	JitterEnabled bool
}

const (
	defaultMaxRetries    = 3
	defaultMaxDelay      = 30 * time.Second
	defaultBackoffFactor = 2.0
	jitterPercent        = 0.1
)

func defaultRetryConfig() retryConfig {
	return retryConfig{
		MaxRetries:    defaultMaxRetries,
		BaseDelay:     time.Second,
		MaxDelay:      defaultMaxDelay,
		BackoffFactor: defaultBackoffFactor,
		JitterEnabled: true,
	}
}

// The Client methods below wrap one raw http* call apiece. Retry is the
// default: executeWithRetry backs off and retries transient failures (network,
// timeout, 429, 5xx) behind the circuit breaker, and the doc comments do not
// repeat it. Methods that route through executeWithoutRetry say so, and name
// the hazard when a replay would cost more than a duplicate request.

// GetProfile retrieves the user profile.
func (c *Client) GetProfile(ctx context.Context) (*Profile, error) {
	var profile *Profile

	err := c.executeWithRetry(ctx, "GetProfile", func() error {
		var err error

		profile, err = c.httpGetProfile(ctx)

		return err
	})

	return profile, err
}

// GetProfileAppProto retrieves one authorized OAuth app as a proto message.
func (c *Client) GetProfileAppProto(ctx context.Context, appID int) (*linodev1.ProfileApp, error) {
	var app *linodev1.ProfileApp

	err := c.executeWithRetry(ctx, "GetProfileApp", func() error {
		var err error

		app, err = c.httpGetProfileAppProto(ctx, appID)

		return err
	})

	return app, err
}

// GetProfileDeviceProto retrieves one trusted device as a proto element.
func (c *Client) GetProfileDeviceProto(ctx context.Context, deviceID int) (*linodev1.TrustedDevice, error) {
	var device *linodev1.TrustedDevice

	err := c.executeWithRetry(ctx, "GetProfileDevice", func() error {
		var err error

		device, err = c.httpGetProfileDeviceProto(ctx, deviceID)

		return err
	})

	return device, err
}

// GetProfileGrants retrieves the /profile/grants response. PATs return an
// empty Grants struct here, so the profile loader must read Profile.Scopes for
// them instead.
func (c *Client) GetProfileGrants(ctx context.Context) (*Grants, error) {
	var grants *Grants

	err := c.executeWithRetry(ctx, "GetProfileGrants", func() error {
		var err error

		grants, err = c.httpGetProfileGrants(ctx)

		return err
	})

	return grants, err
}

// GetProfileTokenProto retrieves one personal access token's metadata as a
// proto element.
func (c *Client) GetProfileTokenProto(ctx context.Context, tokenID int) (*linodev1.PersonalAccessToken, error) {
	var token *linodev1.PersonalAccessToken

	err := c.executeWithRetry(ctx, "GetProfileToken", func() error {
		var err error

		token, err = c.httpGetProfileTokenProto(ctx, tokenID)

		return err
	})

	return token, err
}

// GetInstance retrieves a single instance by ID.
func (c *Client) GetInstance(ctx context.Context, instanceID int) (*Instance, error) {
	var instance *Instance

	err := c.executeWithRetry(ctx, "GetInstance", func() error {
		var err error

		instance, err = c.httpGetInstance(ctx, instanceID)

		return err
	})

	return instance, err
}

// GetAccount retrieves the account information.
func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	var account *Account

	err := c.executeWithRetry(ctx, "GetAccount", func() error {
		var err error

		account, err = c.httpGetAccount(ctx)

		return err
	})

	return account, err
}

// GetAccountSettings retrieves account-wide settings.
func (c *Client) GetAccountSettings(ctx context.Context) (*AccountSettings, error) {
	var settings *AccountSettings

	err := c.executeWithRetry(ctx, "GetAccountSettings", func() error {
		var err error

		settings, err = c.httpGetAccountSettings(ctx)

		return err
	})

	return settings, err
}

// GetLongviewClient retrieves one Longview client.
func (c *Client) GetLongviewClient(ctx context.Context, clientID string) (*LongviewClient, error) {
	var client *LongviewClient

	err := c.executeWithRetry(ctx, "GetLongviewClient", func() error {
		var err error

		client, err = c.httpGetLongviewClient(ctx, clientID)

		return err
	})

	return client, err
}

// GetManagedCredential retrieves one stored managed credential.
func (c *Client) GetManagedCredential(ctx context.Context, credentialID int) (*ManagedCredential, error) {
	var credential *ManagedCredential

	err := c.executeWithRetry(ctx, "GetManagedCredential", func() error {
		var err error

		credential, err = c.httpGetManagedCredential(ctx, credentialID)

		return err
	})

	return credential, err
}

// GetManagedLinodeSettings retrieves Managed settings for one Linode.
func (c *Client) GetManagedLinodeSettings(ctx context.Context, linodeID int) (*ManagedLinodeSettings, error) {
	var settings *ManagedLinodeSettings

	err := c.executeWithRetry(ctx, "GetManagedLinodeSettings", func() error {
		var err error

		settings, err = c.httpGetManagedLinodeSettings(ctx, linodeID)

		return err
	})

	return settings, err
}

// GetManagedContact retrieves one managed contact.
func (c *Client) GetManagedContact(ctx context.Context, contactID int) (*ManagedContact, error) {
	var contact *ManagedContact

	err := c.executeWithRetry(ctx, "GetManagedContact", func() error {
		var err error

		contact, err = c.httpGetManagedContact(ctx, contactID)

		return err
	})

	return contact, err
}

// GetManagedService retrieves one Managed service.
func (c *Client) GetManagedService(ctx context.Context, serviceID int) (*ManagedService, error) {
	var service *ManagedService

	err := c.executeWithRetry(ctx, "GetManagedService", func() error {
		var err error

		service, err = c.httpGetManagedService(ctx, serviceID)

		return err
	})

	return service, err
}

// GetLongviewPlan retrieves the Longview subscription plan.
func (c *Client) GetLongviewPlan(ctx context.Context) (*LongviewSubscription, error) {
	var plan *LongviewSubscription

	err := c.executeWithRetry(ctx, "GetLongviewPlan", func() error {
		var err error

		plan, err = c.httpGetLongviewPlan(ctx)

		return err
	})

	return plan, err
}

// GetMonitorServiceAlertDefinition retrieves one alert definition for one monitoring service type.
func (c *Client) GetMonitorServiceAlertDefinition(ctx context.Context, serviceType string, alertID int) (AlertDefinition, error) {
	var definition AlertDefinition

	err := c.executeWithRetry(ctx, "GetMonitorServiceAlertDefinition", func() error {
		var err error

		definition, err = c.httpGetMonitorServiceAlertDefinition(ctx, serviceType, alertID)

		return err
	})

	return definition, err
}

// GetAccountPaymentMethod retrieves one account payment method.
func (c *Client) GetAccountPaymentMethod(ctx context.Context, paymentMethodID string) (*AccountPaymentMethod, error) {
	var method *AccountPaymentMethod

	err := c.executeWithRetry(ctx, "GetAccountPaymentMethod", func() error {
		var err error

		method, err = c.httpGetAccountPaymentMethod(ctx, paymentMethodID)

		return err
	})

	return method, err
}

// GetAccountOAuthClient retrieves one OAuth client.
func (c *Client) GetAccountOAuthClient(ctx context.Context, clientID string) (*OAuthClient, error) {
	var client *OAuthClient

	err := c.executeWithRetry(ctx, "GetAccountOAuthClient", func() error {
		var err error

		client, err = c.httpGetAccountOAuthClient(ctx, clientID)

		return err
	})

	return client, err
}

// UpdateOAuthClientThumbnail updates an account OAuth client's thumbnail
// without retrying the mutating request.
func (c *Client) UpdateOAuthClientThumbnail(ctx context.Context, clientID string, thumbnailPNG []byte) error {
	return c.httpUpdateOAuthClientThumbnail(ctx, clientID, thumbnailPNG)
}

// GetOAuthClientThumbnail retrieves an OAuth client's thumbnail.
func (c *Client) GetOAuthClientThumbnail(ctx context.Context, clientID string) ([]byte, error) {
	var thumbnailPNG []byte

	err := c.executeWithRetry(ctx, "GetOAuthClientThumbnail", func() error {
		var err error

		thumbnailPNG, err = c.httpGetOAuthClientThumbnail(ctx, clientID)

		return err
	})

	return thumbnailPNG, err
}

// ListTaggedObjects retrieves objects with the supplied tag label.
func (c *Client) ListTaggedObjects(ctx context.Context, tagLabel string, page, pageSize int) (*PaginatedResponse[TaggedObject], error) {
	var taggedObjects *PaginatedResponse[TaggedObject]

	err := c.executeWithRetry(ctx, "ListTaggedObjects", func() error {
		var err error

		taggedObjects, err = c.httpListTaggedObjects(ctx, tagLabel, page, pageSize)

		return err
	})

	return taggedObjects, err
}

// GetAccountUser retrieves one account user.
func (c *Client) GetAccountUser(ctx context.Context, username string) (*AccountUser, error) {
	var user *AccountUser

	err := c.executeWithRetry(ctx, "GetAccountUser", func() error {
		var err error

		user, err = c.httpGetAccountUser(ctx, username)

		return err
	})

	return user, err
}

// GetAccountUserGrants retrieves one account user's grants.
func (c *Client) GetAccountUserGrants(ctx context.Context, username string) (*Grants, error) {
	var grants *Grants

	err := c.executeWithRetry(ctx, "GetAccountUserGrants", func() error {
		var err error

		grants, err = c.httpGetAccountUserGrants(ctx, username)

		return err
	})

	return grants, err
}

// CreateSupportTicketAttachment creates a support ticket attachment without
// retrying the mutating request.
func (c *Client) CreateSupportTicketAttachment(ctx context.Context, ticketID int, request *CreateSupportTicketAttachmentRequest) (*SupportTicketAttachment, error) {
	return c.httpCreateSupportTicketAttachment(ctx, ticketID, request)
}

// GetAccountServiceTransfer retrieves one account service transfer.
func (c *Client) GetAccountServiceTransfer(ctx context.Context, token string) (*AccountEntityTransfer, error) {
	var transfer *AccountEntityTransfer

	err := c.executeWithRetry(ctx, "GetAccountServiceTransfer", func() error {
		var err error

		transfer, err = c.httpGetAccountServiceTransfer(ctx, token)

		return err
	})

	return transfer, err
}

// GetAccountEvent retrieves one account event.
func (c *Client) GetAccountEvent(ctx context.Context, eventID int) (*AccountEvent, error) {
	var event *AccountEvent

	err := c.executeWithRetry(ctx, "GetAccountEvent", func() error {
		var err error

		event, err = c.httpGetAccountEvent(ctx, eventID)

		return err
	})

	return event, err
}

// GetAccountChildAccount retrieves one child-level account.
func (c *Client) GetAccountChildAccount(ctx context.Context, euuid string) (*ChildAccount, error) {
	var childAccount *ChildAccount

	err := c.executeWithRetry(ctx, "GetAccountChildAccount", func() error {
		var err error

		childAccount, err = c.httpGetAccountChildAccount(ctx, euuid)

		return err
	})

	return childAccount, err
}

// GetType retrieves one Linode type.
func (c *Client) GetType(ctx context.Context, typeID string) (*InstanceType, error) {
	var instanceType *InstanceType

	err := c.executeWithRetry(ctx, "GetType", func() error {
		var err error

		instanceType, err = c.httpGetType(ctx, typeID)

		return err
	})

	return instanceType, err
}

// GetDatabaseInstance retrieves one MySQL Managed Database instance.
func (c *Client) GetDatabaseInstance(ctx context.Context, instanceID int) (*DatabaseInstance, error) {
	var instance *DatabaseInstance

	err := c.executeWithRetry(ctx, "GetDatabaseInstance", func() error {
		var err error

		instance, err = c.httpGetDatabaseInstance(ctx, instanceID)

		return err
	})

	return instance, err
}

// GetDatabasePostgreSQLInstance retrieves one PostgreSQL Managed Database instance.
func (c *Client) GetDatabasePostgreSQLInstance(ctx context.Context, instanceID int) (*DatabaseInstance, error) {
	var instance *DatabaseInstance

	err := c.executeWithRetry(ctx, "GetDatabasePostgreSQLInstance", func() error {
		var err error

		instance, err = c.httpGetDatabasePostgreSQLInstance(ctx, instanceID)

		return err
	})

	return instance, err
}

// GetImage retrieves one image.
func (c *Client) GetImage(ctx context.Context, imageID string) (*Image, error) {
	var image *Image

	err := c.executeWithRetry(ctx, "GetImage", func() error {
		var err error

		image, err = c.httpGetImage(ctx, imageID)

		return err
	})

	return image, err
}

// GetImageShareGroup retrieves a single image share group.
func (c *Client) GetImageShareGroup(ctx context.Context, shareGroupID int) (*ImageShareGroup, error) {
	var shareGroup *ImageShareGroup

	err := c.executeWithRetry(ctx, "GetImageShareGroup", func() error {
		var err error

		shareGroup, err = c.httpGetImageShareGroup(ctx, shareGroupID)

		return err
	})

	return shareGroup, err
}

// GetImageShareGroupByToken retrieves a token's share group.
func (c *Client) GetImageShareGroupByToken(ctx context.Context, tokenUUID string) (*ImageShareGroup, error) {
	var shareGroup *ImageShareGroup

	err := c.executeWithRetry(ctx, "GetImageShareGroupByToken", func() error {
		var err error

		shareGroup, err = c.httpGetImageShareGroupByToken(ctx, tokenUUID)

		return err
	})

	return shareGroup, err
}

// GetDomain retrieves a single domain by ID.
func (c *Client) GetDomain(ctx context.Context, domainID int) (*Domain, error) {
	var domain *Domain

	err := c.executeWithRetry(ctx, "GetDomain", func() error {
		var err error

		domain, err = c.httpGetDomain(ctx, domainID)

		return err
	})

	return domain, err
}

// ListDomainRecords retrieves all records for a domain.
func (c *Client) ListDomainRecords(ctx context.Context, domainID int) ([]DomainRecord, error) {
	var records []DomainRecord

	err := c.executeWithRetry(ctx, "ListDomainRecords", func() error {
		var err error

		records, err = c.httpListDomainRecords(ctx, domainID)

		return err
	})

	return records, err
}

// GetReservedIPRaw retrieves one reserved public IPv4 address while preserving
// its raw JSON shape.
func (c *Client) GetReservedIPRaw(ctx context.Context, address string) (json.RawMessage, error) {
	var reservedIP json.RawMessage

	err := c.executeWithRetry(ctx, "GetReservedIP", func() error {
		var retryErr error

		reservedIP, retryErr = c.httpGetReservedIPRaw(ctx, address)

		return retryErr
	})

	return reservedIP, err
}

// ListVLANs retrieves all VLANs.
func (c *Client) ListVLANs(ctx context.Context, page, pageSize int) (*PaginatedResponse[VLAN], error) {
	var vlans *PaginatedResponse[VLAN]

	err := c.executeWithRetry(ctx, "ListVLANs", func() error {
		var err error

		vlans, err = c.httpListVLANs(ctx, page, pageSize)

		return err
	})

	return vlans, err
}

// ListFirewallRules retrieves firewall rules.
func (c *Client) ListFirewallRules(ctx context.Context, firewallID int) (*FirewallRules, error) {
	var rules *FirewallRules

	err := c.executeWithRetry(ctx, "ListFirewallRules", func() error {
		var err error

		rules, err = c.httpListFirewallRules(ctx, firewallID)

		return err
	})

	return rules, err
}

// ListFirewallDevices retrieves devices assigned to a Cloud Firewall.
func (c *Client) ListFirewallDevices(ctx context.Context, firewallID, page, pageSize int) (*PaginatedResponse[FirewallDevice], error) {
	var devices *PaginatedResponse[FirewallDevice]

	err := c.executeWithRetry(ctx, "ListFirewallDevices", func() error {
		var err error

		devices, err = c.httpListFirewallDevices(ctx, firewallID, page, pageSize)

		return err
	})

	return devices, err
}

// GetFirewallDevice retrieves one device assigned to a Cloud Firewall.
func (c *Client) GetFirewallDevice(ctx context.Context, firewallID, deviceID int) (*FirewallDevice, error) {
	var device *FirewallDevice

	err := c.executeWithRetry(ctx, "GetFirewallDevice", func() error {
		var err error

		device, err = c.httpGetFirewallDevice(ctx, firewallID, deviceID)

		return err
	})

	return device, err
}

// ListFirewallSettings retrieves default firewall assignments.
func (c *Client) ListFirewallSettings(ctx context.Context, page, pageSize int) (*FirewallSettings, error) {
	var settings *FirewallSettings

	err := c.executeWithRetry(ctx, "ListFirewallSettings", func() error {
		var err error

		settings, err = c.httpListFirewallSettings(ctx, page, pageSize)

		return err
	})

	return settings, err
}

// GetNetworkingIP retrieves an account-level IP address.
func (c *Client) GetNetworkingIP(ctx context.Context, address string) (*IPAddress, error) {
	var networkingIPAddr *IPAddress

	err := c.executeWithRetry(ctx, "GetNetworkingIP", func() error {
		var retryErr error

		networkingIPAddr, retryErr = c.httpGetNetworkingIP(ctx, address)

		return retryErr
	})

	return networkingIPAddr, err
}

// GetIPv6Range retrieves one IPv6 range.
func (c *Client) GetIPv6Range(ctx context.Context, ipv6Range string) (*IPv6Range, error) {
	var result *IPv6Range

	err := c.executeWithRetry(ctx, "GetIPv6Range", func() error {
		var err error

		result, err = c.httpGetIPv6Range(ctx, ipv6Range)

		return err
	})

	return result, err
}

// GetNodeBalancer retrieves a single node balancer by ID.
func (c *Client) GetNodeBalancer(ctx context.Context, nodeBalancerID int) (*NodeBalancer, error) {
	var nodeBalancer *NodeBalancer

	err := c.executeWithRetry(ctx, "GetNodeBalancer", func() error {
		var err error

		nodeBalancer, err = c.httpGetNodeBalancer(ctx, nodeBalancerID)

		return err
	})

	return nodeBalancer, err
}

// ListNodeBalancerConfigs retrieves configs for a node balancer by ID.
func (c *Client) ListNodeBalancerConfigs(ctx context.Context, nodeBalancerID, page, pageSize int) ([]NodeBalancerConfig, error) {
	var configs []NodeBalancerConfig

	err := c.executeWithRetry(ctx, "ListNodeBalancerConfigs", func() error {
		var err error

		configs, err = c.httpListNodeBalancerConfigs(ctx, nodeBalancerID, page, pageSize)

		return err
	})

	return configs, err
}

// ListNodeBalancerFirewallsProto retrieves Cloud Firewalls assigned to a
// NodeBalancer as proto messages.
func (c *Client) ListNodeBalancerFirewallsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.Firewall, error) {
	var firewalls []*linodev1.Firewall

	err := c.executeWithRetry(ctx, "ListNodeBalancerFirewalls", func() error {
		var retryErr error

		firewalls, retryErr = c.httpListNodeBalancerFirewallsProto(ctx, nodeBalancerID, page, pageSize)

		return retryErr
	})

	return firewalls, err
}

// ListNodeBalancerConfigNodes retrieves nodes for a node balancer config.
func (c *Client) ListNodeBalancerConfigNodes(ctx context.Context, nodeBalancerID, configID, page, pageSize int) (*PaginatedResponse[NodeBalancerConfigNode], error) {
	var nodes *PaginatedResponse[NodeBalancerConfigNode]

	err := c.executeWithRetry(ctx, "ListNodeBalancerConfigNodes", func() error {
		var retryErr error

		nodes, retryErr = c.httpListNodeBalancerConfigNodes(ctx, nodeBalancerID, configID, page, pageSize)

		return retryErr
	})

	return nodes, err
}

// GetNodeBalancerConfig retrieves one NodeBalancer config as the struct the
// dry-run previews report.
func (c *Client) GetNodeBalancerConfig(ctx context.Context, nodeBalancerID, configID int) (*NodeBalancerConfig, error) {
	var config *NodeBalancerConfig

	err := c.executeWithRetry(ctx, "GetNodeBalancerConfig", func() error {
		var retryErr error

		config, retryErr = c.httpGetNodeBalancerConfig(ctx, nodeBalancerID, configID)

		return retryErr
	})

	return config, err
}

// GetNodeBalancerConfigNode retrieves one node for a node balancer config.
func (c *Client) GetNodeBalancerConfigNode(ctx context.Context, nodeBalancerID, configID, nodeID int) (*NodeBalancerConfigNode, error) {
	var node *NodeBalancerConfigNode

	err := c.executeWithRetry(ctx, "GetNodeBalancerConfigNode", func() error {
		var retryErr error

		node, retryErr = c.httpGetNodeBalancerConfigNode(ctx, nodeBalancerID, configID, nodeID)

		return retryErr
	})

	return node, err
}

// GetStackScript retrieves one StackScript.
func (c *Client) GetStackScript(ctx context.Context, stackScriptID int) (*StackScript, error) {
	var script *StackScript

	err := c.executeWithRetry(ctx, "GetStackScript", func() error {
		var err error

		script, err = c.httpGetStackScript(ctx, stackScriptID)

		return err
	})

	return script, err
}

// GetFirewall retrieves a single firewall by ID.
func (c *Client) GetFirewall(ctx context.Context, firewallID int) (*Firewall, error) {
	var firewall *Firewall

	err := c.executeWithRetry(ctx, "GetFirewall", func() error {
		var err error

		firewall, err = c.httpGetFirewall(ctx, firewallID)

		return err
	})

	return firewall, err
}

// GetVolume retrieves a single volume by ID.
func (c *Client) GetVolume(ctx context.Context, volumeID int) (*Volume, error) {
	var volume *Volume

	err := c.executeWithRetry(ctx, "GetVolume", func() error {
		var err error

		volume, err = c.httpGetVolume(ctx, volumeID)

		return err
	})

	return volume, err
}

// GetSSHKey retrieves a single SSH key by ID.
func (c *Client) GetSSHKey(ctx context.Context, sshKeyID int) (*SSHKey, error) {
	var sshKey *SSHKey

	err := c.executeWithRetry(ctx, "GetSSHKey", func() error {
		var err error

		sshKey, err = c.httpGetSSHKey(ctx, sshKeyID)

		return err
	})

	return sshKey, err
}

// GetDomainRecord gets a domain record.
func (c *Client) GetDomainRecord(ctx context.Context, domainID, recordID int) (*DomainRecord, error) {
	var record *DomainRecord

	err := c.executeWithRetry(ctx, "GetDomainRecord", func() error {
		var err error

		record, err = c.httpGetDomainRecord(ctx, domainID, recordID)

		return err
	})

	return record, err
}

// GetObjectStorageBucket retrieves a specific bucket.
func (c *Client) GetObjectStorageBucket(ctx context.Context, region, label string) (*ObjectStorageBucket, error) {
	var bucket *ObjectStorageBucket

	err := c.executeWithRetry(ctx, "GetObjectStorageBucket", func() error {
		var err error

		bucket, err = c.httpGetObjectStorageBucket(ctx, region, label)

		return err
	})

	return bucket, err
}

// GetObjectStorageKey retrieves a specific access key.
func (c *Client) GetObjectStorageKey(ctx context.Context, keyID int) (*ObjectStorageKey, error) {
	var key *ObjectStorageKey

	err := c.executeWithRetry(ctx, "GetObjectStorageKey", func() error {
		var err error

		key, err = c.httpGetObjectStorageKey(ctx, keyID)

		return err
	})

	return key, err
}

// GetObjectStorageBucketAccess retrieves bucket ACL/CORS settings.
func (c *Client) GetObjectStorageBucketAccess(ctx context.Context, region, label string) (*ObjectStorageBucketAccess, error) {
	var access *ObjectStorageBucketAccess

	err := c.executeWithRetry(ctx, "GetObjectStorageBucketAccess", func() error {
		var err error

		access, err = c.httpGetObjectStorageBucketAccess(ctx, region, label)

		return err
	})

	return access, err
}

// GetObjectACL retrieves an object's ACL.
func (c *Client) GetObjectACL(ctx context.Context, region, label, name string) (*ObjectACL, error) {
	var result *ObjectACL

	err := c.executeWithRetry(ctx, "GetObjectACL", func() error {
		var retryErr error

		result, retryErr = c.httpGetObjectACL(ctx, region, label, name)

		return retryErr
	})

	return result, err
}

// GetBucketSSL retrieves a bucket's SSL status.
func (c *Client) GetBucketSSL(ctx context.Context, region, label string) (*BucketSSL, error) {
	var result *BucketSSL

	err := c.executeWithRetry(ctx, "GetBucketSSL", func() error {
		var retryErr error

		result, retryErr = c.httpGetBucketSSL(ctx, region, label)

		return retryErr
	})

	return result, err
}

// LKE (Kubernetes Engine) operations

// GetLKECluster retrieves a single LKE cluster by ID.
func (c *Client) GetLKECluster(ctx context.Context, clusterID int) (*LKECluster, error) {
	var cluster *LKECluster

	err := c.executeWithRetry(ctx, "GetLKECluster", func() error {
		var err error

		cluster, err = c.httpGetLKECluster(ctx, clusterID)

		return err
	})

	return cluster, err
}

// ListLKENodePools retrieves all node pools for an LKE cluster.
func (c *Client) ListLKENodePools(ctx context.Context, clusterID int) ([]LKENodePool, error) {
	var pools []LKENodePool

	err := c.executeWithRetry(ctx, "ListLKENodePools", func() error {
		var err error

		pools, err = c.httpListLKENodePools(ctx, clusterID)

		return err
	})

	return pools, err
}

// GetLKENodePool retrieves a single node pool by ID.
func (c *Client) GetLKENodePool(ctx context.Context, clusterID, poolID int) (*LKENodePool, error) {
	var pool *LKENodePool

	err := c.executeWithRetry(ctx, "GetLKENodePool", func() error {
		var err error

		pool, err = c.httpGetLKENodePool(ctx, clusterID, poolID)

		return err
	})

	return pool, err
}

// GetLKENode retrieves a single node by ID.
func (c *Client) GetLKENode(ctx context.Context, clusterID int, nodeID string) (*LKENode, error) {
	var node *LKENode

	err := c.executeWithRetry(ctx, "GetLKENode", func() error {
		var err error

		node, err = c.httpGetLKENode(ctx, clusterID, nodeID)

		return err
	})

	return node, err
}

// GetLKEControlPlaneACL retrieves the control plane ACL.
func (c *Client) GetLKEControlPlaneACL(ctx context.Context, clusterID int) (*LKEControlPlaneACL, error) {
	var acl *LKEControlPlaneACL

	err := c.executeWithRetry(ctx, "GetLKEControlPlaneACL", func() error {
		var err error

		acl, err = c.httpGetLKEControlPlaneACL(ctx, clusterID)

		return err
	})

	return acl, err
}

// VPC operations

// GetVPC retrieves a single VPC by ID.
func (c *Client) GetVPC(ctx context.Context, vpcID int) (*VPC, error) {
	var vpc *VPC

	err := c.executeWithRetry(ctx, "GetVPC", func() error {
		var retryErr error

		vpc, retryErr = c.httpGetVPC(ctx, vpcID)

		return retryErr
	})

	return vpc, err
}

// GetPlacementGroup retrieves a single placement group by ID.
func (c *Client) GetPlacementGroup(ctx context.Context, groupID int) (*PlacementGroup, error) {
	var group *PlacementGroup

	err := c.executeWithRetry(ctx, "GetPlacementGroup", func() error {
		var retryErr error

		group, retryErr = c.httpGetPlacementGroup(ctx, groupID)

		return retryErr
	})

	return group, err
}

// ListVPCSubnets retrieves all subnets for a VPC.
func (c *Client) ListVPCSubnets(ctx context.Context, vpcID int) ([]VPCSubnet, error) {
	var subnets []VPCSubnet

	err := c.executeWithRetry(ctx, "ListVPCSubnets", func() error {
		var retryErr error

		subnets, retryErr = c.httpListVPCSubnets(ctx, vpcID)

		return retryErr
	})

	return subnets, err
}

// GetVPCSubnet retrieves a single subnet by ID.
func (c *Client) GetVPCSubnet(ctx context.Context, vpcID, subnetID int) (*VPCSubnet, error) {
	var subnet *VPCSubnet

	err := c.executeWithRetry(ctx, "GetVPCSubnet", func() error {
		var retryErr error

		subnet, retryErr = c.httpGetVPCSubnet(ctx, vpcID, subnetID)

		return retryErr
	})

	return subnet, err
}

// Instance deep operations

// GetInstanceBackup retrieves a specific backup.
func (c *Client) GetInstanceBackup(ctx context.Context, linodeID, backupID int) (*InstanceBackup, error) {
	var backup *InstanceBackup

	err := c.executeWithRetry(ctx, "GetInstanceBackup", func() error {
		var retryErr error

		backup, retryErr = c.httpGetInstanceBackup(ctx, linodeID, backupID)

		return retryErr
	})

	return backup, err
}

// GetInstanceConfigInterface retrieves an interface.
func (c *Client) GetInstanceConfigInterface(ctx context.Context, linodeID, configID, interfaceID int) (*ConfigInterfaceResponse, error) {
	var configInterface *ConfigInterfaceResponse

	err := c.executeWithRetry(ctx, "GetInstanceConfigInterface", func() error {
		var retryErr error

		configInterface, retryErr = c.httpGetInstanceConfigInterface(ctx, linodeID, configID, interfaceID)

		return retryErr
	})

	return configInterface, err
}

// ListInstanceDisks retrieves all disks for an instance.
func (c *Client) ListInstanceDisks(ctx context.Context, linodeID int) ([]InstanceDisk, error) {
	var disks []InstanceDisk

	err := c.executeWithRetry(ctx, "ListInstanceDisks", func() error {
		var retryErr error

		disks, retryErr = c.httpListInstanceDisks(ctx, linodeID)

		return retryErr
	})

	return disks, err
}

// ListInstanceConfigs retrieves all configuration profiles for an instance.
func (c *Client) ListInstanceConfigs(ctx context.Context, linodeID, page, pageSize int) ([]InstanceConfig, error) {
	var configs []InstanceConfig

	err := c.executeWithRetry(ctx, "ListInstanceConfigs", func() error {
		var retryErr error

		configs, retryErr = c.httpListInstanceConfigs(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return configs, err
}

// ListInstanceVolumes retrieves all volumes attached to an instance.
func (c *Client) ListInstanceVolumes(ctx context.Context, linodeID, page, pageSize int) ([]Volume, error) {
	var volumes []Volume

	err := c.executeWithRetry(ctx, "ListInstanceVolumes", func() error {
		var retryErr error

		volumes, retryErr = c.httpListInstanceVolumes(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return volumes, err
}

// GetInstanceInterface retrieves a Linode interface.
func (c *Client) GetInstanceInterface(ctx context.Context, linodeID, interfaceID int) (*InstanceInterface, error) {
	var instanceInterface *InstanceInterface

	err := c.executeWithRetry(ctx, "GetInstanceInterface", func() error {
		var retryErr error

		instanceInterface, retryErr = c.httpGetInstanceInterface(ctx, linodeID, interfaceID)

		return retryErr
	})

	return instanceInterface, err
}

// GetInstanceInterfaceSettings retrieves Linode interface settings.
func (c *Client) GetInstanceInterfaceSettings(ctx context.Context, linodeID int) (*InstanceInterfaceSettings, error) {
	var settings *InstanceInterfaceSettings

	err := c.executeWithRetry(ctx, "GetInstanceInterfaceSettings", func() error {
		var retryErr error

		settings, retryErr = c.httpGetInstanceInterfaceSettings(ctx, linodeID)

		return retryErr
	})

	return settings, err
}

// GetInstanceConfig retrieves a specific configuration profile.
func (c *Client) GetInstanceConfig(ctx context.Context, linodeID, configID int) (*InstanceConfig, error) {
	var config *InstanceConfig

	err := c.executeWithRetry(ctx, "GetInstanceConfig", func() error {
		var retryErr error

		config, retryErr = c.httpGetInstanceConfig(ctx, linodeID, configID)

		return retryErr
	})

	return config, err
}

// ListInstanceFirewalls retrieves all Cloud Firewalls assigned to an instance.
func (c *Client) ListInstanceFirewalls(ctx context.Context, linodeID, page, pageSize int) ([]Firewall, error) {
	var firewalls []Firewall

	err := c.executeWithRetry(ctx, "ListInstanceFirewalls", func() error {
		var retryErr error

		firewalls, retryErr = c.httpListInstanceFirewalls(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return firewalls, err
}

// ListInstanceFirewallsProto retrieves an instance's assigned Cloud Firewalls
// as proto messages.
func (c *Client) ListInstanceFirewallsProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.Firewall, error) {
	var firewalls []*linodev1.Firewall

	err := c.executeWithRetry(ctx, "ListInstanceFirewalls", func() error {
		var retryErr error

		firewalls, retryErr = c.httpListInstanceFirewallsProto(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return firewalls, err
}

// GetInstanceDisk retrieves a specific disk.
func (c *Client) GetInstanceDisk(ctx context.Context, linodeID, diskID int) (*InstanceDisk, error) {
	var disk *InstanceDisk

	err := c.executeWithRetry(ctx, "GetInstanceDisk", func() error {
		var retryErr error

		disk, retryErr = c.httpGetInstanceDisk(ctx, linodeID, diskID)

		return retryErr
	})

	return disk, err
}

// ListInstanceIPs retrieves all IP addresses for an instance.
func (c *Client) ListInstanceIPs(ctx context.Context, linodeID int) (*InstanceIPAddresses, error) {
	var ips *InstanceIPAddresses

	err := c.executeWithRetry(ctx, "ListInstanceIPs", func() error {
		var retryErr error

		ips, retryErr = c.httpListInstanceIPs(ctx, linodeID)

		return retryErr
	})

	return ips, err
}

// GetInstanceIP retrieves a specific IP address.
func (c *Client) GetInstanceIP(ctx context.Context, linodeID int, address string) (*IPAddress, error) {
	var ipAddr *IPAddress

	err := c.executeWithRetry(ctx, "GetInstanceIP", func() error {
		var retryErr error

		ipAddr, retryErr = c.httpGetInstanceIP(ctx, linodeID, address)

		return retryErr
	})

	return ipAddr, err
}

func (c *Client) executeWithoutRetry(ctx context.Context, operation string, run func() error) error {
	if err := c.circuit.Allow(); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}

	err := run()
	if err == nil {
		c.circuit.RecordSuccess()

		return nil
	}

	if c.shouldRecordCircuitFailure(err) {
		c.circuit.RecordFailure()
	}

	return err
}

func (*Client) shouldRecordCircuitFailure(err error) bool {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.IsRateLimitError() || apiErr.IsServerError()
	}

	return isNetworkError(err) || isTimeoutError(err)
}

func (c *Client) executeWithRetry(ctx context.Context, operation string, retryFunc func() error) error {
	if err := c.circuit.Allow(); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	var lastErr error

	var attempt int

	for attempt <= c.retryCfg.MaxRetries {
		if attempt > 0 {
			delay := c.delayForAttempt(attempt, lastErr)
			select {
			case <-ctx.Done():
				// Caller canceled; not an upstream-health signal.
				return fmt.Errorf("context canceled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		err := retryFunc()
		if err == nil {
			c.circuit.RecordSuccess()

			return nil
		}

		lastErr = err
		attempt++

		if !c.shouldRetry(err) {
			// Non-retryable (e.g. auth), not the breaker's concern.
			return err
		}
	}

	// Retries exhausted on a retryable failure: the signal the breaker tracks.
	c.circuit.RecordFailure()

	return fmt.Errorf("%s: %w", operation, lastErr)
}

// delayForAttempt waits out the upstream's Retry-After hint (typically on 429)
// when there is one, otherwise falls back to exponential backoff with jitter.
// The hint is clamped to MaxDelay so a hostile or buggy server can't ask us to
// wait an hour.
func (c *Client) delayForAttempt(attempt int, lastErr error) time.Duration {
	if apiErr, ok := errors.AsType[*APIError](lastErr); ok && apiErr.RetryAfter > 0 {
		hint := apiErr.RetryAfter
		if hint > c.retryCfg.MaxDelay {
			return c.retryCfg.MaxDelay
		}

		return hint
	}

	return c.calculateDelay(attempt)
}

func (c *Client) calculateDelay(attempt int) time.Duration {
	delay := float64(c.retryCfg.BaseDelay) * math.Pow(c.retryCfg.BackoffFactor, float64(attempt-1))

	if c.retryCfg.JitterEnabled {
		jitterMax := big.NewInt(int64(delay * jitterPercent))
		if jitterMax.Int64() > 0 {
			jitterBig, err := rand.Int(rand.Reader, jitterMax)
			if err != nil {
				return c.retryCfg.BaseDelay
			}

			jitter := float64(jitterBig.Int64())
			delay += jitter
		}
	}

	maxDelay := float64(c.retryCfg.MaxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}

	return time.Duration(delay)
}

func (*Client) shouldRetry(err error) bool {
	// Cheaper than falling through to isRetryable, which reaches the same
	// answer only after extra type assertions.
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		if apiErr.IsAuthenticationError() || apiErr.IsForbiddenError() {
			return false
		}
	}

	return isRetryable(err)
}
