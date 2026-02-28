package linode

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"
)

// RetryConfig holds configuration for retry behavior.
type RetryConfig struct {
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

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    defaultMaxRetries,
		BaseDelay:     time.Second,
		MaxDelay:      defaultMaxDelay,
		BackoffFactor: defaultBackoffFactor,
		JitterEnabled: true,
	}
}

// RetryableClient wraps the basic Client with retry functionality.
type RetryableClient struct {
	*Client

	retryConfig RetryConfig
}

// NewRetryableClient creates a RetryableClient with the given retry configuration.
func NewRetryableClient(apiURL, token string, retryConfig RetryConfig) *RetryableClient {
	return &RetryableClient{
		Client:      NewClient(apiURL, token),
		retryConfig: retryConfig,
	}
}

// NewRetryableClientWithDefaults creates a RetryableClient with default retry settings.
func NewRetryableClientWithDefaults(apiURL, token string) *RetryableClient {
	return NewRetryableClient(apiURL, token, DefaultRetryConfig())
}

// GetProfile retrieves the user profile with automatic retry on transient failures.
func (rc *RetryableClient) GetProfile(ctx context.Context) (*Profile, error) {
	var profile *Profile

	err := rc.executeWithRetry(ctx, "GetProfile", func() error {
		var err error

		profile, err = rc.Client.GetProfile(ctx)

		return err
	})

	return profile, err
}

// ListInstances retrieves all instances with automatic retry on transient failures.
func (rc *RetryableClient) ListInstances(ctx context.Context) ([]Instance, error) {
	var instances []Instance

	err := rc.executeWithRetry(ctx, "ListInstances", func() error {
		var err error

		instances, err = rc.Client.ListInstances(ctx)

		return err
	})

	return instances, err
}

// GetInstance retrieves a single instance by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetInstance(ctx context.Context, instanceID int) (*Instance, error) {
	var instance *Instance

	err := rc.executeWithRetry(ctx, "GetInstance", func() error {
		var err error

		instance, err = rc.Client.GetInstance(ctx, instanceID)

		return err
	})

	return instance, err
}

// GetAccount retrieves the account information with automatic retry on transient failures.
func (rc *RetryableClient) GetAccount(ctx context.Context) (*Account, error) {
	var account *Account

	err := rc.executeWithRetry(ctx, "GetAccount", func() error {
		var err error

		account, err = rc.Client.GetAccount(ctx)

		return err
	})

	return account, err
}

// ListRegions retrieves all regions with automatic retry on transient failures.
func (rc *RetryableClient) ListRegions(ctx context.Context) ([]Region, error) {
	var regions []Region

	err := rc.executeWithRetry(ctx, "ListRegions", func() error {
		var err error

		regions, err = rc.Client.ListRegions(ctx)

		return err
	})

	return regions, err
}

// ListTypes retrieves all Linode types with automatic retry on transient failures.
func (rc *RetryableClient) ListTypes(ctx context.Context) ([]InstanceType, error) {
	var types []InstanceType

	err := rc.executeWithRetry(ctx, "ListTypes", func() error {
		var err error

		types, err = rc.Client.ListTypes(ctx)

		return err
	})

	return types, err
}

// ListVolumes retrieves all volumes with automatic retry on transient failures.
func (rc *RetryableClient) ListVolumes(ctx context.Context) ([]Volume, error) {
	var volumes []Volume

	err := rc.executeWithRetry(ctx, "ListVolumes", func() error {
		var err error

		volumes, err = rc.Client.ListVolumes(ctx)

		return err
	})

	return volumes, err
}

// ListImages retrieves all images with automatic retry on transient failures.
func (rc *RetryableClient) ListImages(ctx context.Context) ([]Image, error) {
	var images []Image

	err := rc.executeWithRetry(ctx, "ListImages", func() error {
		var err error

		images, err = rc.Client.ListImages(ctx)

		return err
	})

	return images, err
}

// Stage 3: Extended read operations

// ListSSHKeys retrieves all SSH keys with automatic retry on transient failures.
func (rc *RetryableClient) ListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	var keys []SSHKey

	err := rc.executeWithRetry(ctx, "ListSSHKeys", func() error {
		var err error

		keys, err = rc.Client.ListSSHKeys(ctx)

		return err
	})

	return keys, err
}

// ListDomains retrieves all domains with automatic retry on transient failures.
func (rc *RetryableClient) ListDomains(ctx context.Context) ([]Domain, error) {
	var domains []Domain

	err := rc.executeWithRetry(ctx, "ListDomains", func() error {
		var err error

		domains, err = rc.Client.ListDomains(ctx)

		return err
	})

	return domains, err
}

// GetDomain retrieves a single domain by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetDomain(ctx context.Context, domainID int) (*Domain, error) {
	var domain *Domain

	err := rc.executeWithRetry(ctx, "GetDomain", func() error {
		var err error

		domain, err = rc.Client.GetDomain(ctx, domainID)

		return err
	})

	return domain, err
}

// ListDomainRecords retrieves all records for a domain with automatic retry on transient failures.
func (rc *RetryableClient) ListDomainRecords(ctx context.Context, domainID int) ([]DomainRecord, error) {
	var records []DomainRecord

	err := rc.executeWithRetry(ctx, "ListDomainRecords", func() error {
		var err error

		records, err = rc.Client.ListDomainRecords(ctx, domainID)

		return err
	})

	return records, err
}

// ListFirewalls retrieves all firewalls with automatic retry on transient failures.
func (rc *RetryableClient) ListFirewalls(ctx context.Context) ([]Firewall, error) {
	var firewalls []Firewall

	err := rc.executeWithRetry(ctx, "ListFirewalls", func() error {
		var err error

		firewalls, err = rc.Client.ListFirewalls(ctx)

		return err
	})

	return firewalls, err
}

// ListNodeBalancers retrieves all node balancers with automatic retry on transient failures.
func (rc *RetryableClient) ListNodeBalancers(ctx context.Context) ([]NodeBalancer, error) {
	var nodeBalancers []NodeBalancer

	err := rc.executeWithRetry(ctx, "ListNodeBalancers", func() error {
		var err error

		nodeBalancers, err = rc.Client.ListNodeBalancers(ctx)

		return err
	})

	return nodeBalancers, err
}

// GetNodeBalancer retrieves a single node balancer by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetNodeBalancer(ctx context.Context, nodeBalancerID int) (*NodeBalancer, error) {
	var nodeBalancer *NodeBalancer

	err := rc.executeWithRetry(ctx, "GetNodeBalancer", func() error {
		var err error

		nodeBalancer, err = rc.Client.GetNodeBalancer(ctx, nodeBalancerID)

		return err
	})

	return nodeBalancer, err
}

// ListStackScripts retrieves all stack scripts with automatic retry on transient failures.
func (rc *RetryableClient) ListStackScripts(ctx context.Context) ([]StackScript, error) {
	var scripts []StackScript

	err := rc.executeWithRetry(ctx, "ListStackScripts", func() error {
		var err error

		scripts, err = rc.Client.ListStackScripts(ctx)

		return err
	})

	return scripts, err
}

// GetFirewall retrieves a single firewall by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetFirewall(ctx context.Context, firewallID int) (*Firewall, error) {
	var firewall *Firewall

	err := rc.executeWithRetry(ctx, "GetFirewall", func() error {
		var err error

		firewall, err = rc.Client.GetFirewall(ctx, firewallID)

		return err
	})

	return firewall, err
}

// GetVolume retrieves a single volume by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetVolume(ctx context.Context, volumeID int) (*Volume, error) {
	var volume *Volume

	err := rc.executeWithRetry(ctx, "GetVolume", func() error {
		var err error

		volume, err = rc.Client.GetVolume(ctx, volumeID)

		return err
	})

	return volume, err
}

// Stage 4: Write operations with retry.

// CreateSSHKey creates a new SSH key with automatic retry on transient failures.
func (rc *RetryableClient) CreateSSHKey(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error) {
	var sshKey *SSHKey

	err := rc.executeWithRetry(ctx, "CreateSSHKey", func() error {
		var err error

		sshKey, err = rc.Client.CreateSSHKey(ctx, req)

		return err
	})

	return sshKey, err
}

// DeleteSSHKey deletes an SSH key with automatic retry on transient failures.
func (rc *RetryableClient) DeleteSSHKey(ctx context.Context, sshKeyID int) error {
	return rc.executeWithRetry(ctx, "DeleteSSHKey", func() error {
		return rc.Client.DeleteSSHKey(ctx, sshKeyID)
	})
}

// BootInstance boots a Linode instance with automatic retry on transient failures.
func (rc *RetryableClient) BootInstance(ctx context.Context, instanceID int, configID *int) error {
	return rc.executeWithRetry(ctx, "BootInstance", func() error {
		return rc.Client.BootInstance(ctx, instanceID, configID)
	})
}

// RebootInstance reboots a Linode instance with automatic retry on transient failures.
func (rc *RetryableClient) RebootInstance(ctx context.Context, instanceID int, configID *int) error {
	return rc.executeWithRetry(ctx, "RebootInstance", func() error {
		return rc.Client.RebootInstance(ctx, instanceID, configID)
	})
}

// ShutdownInstance shuts down a Linode instance with automatic retry on transient failures.
func (rc *RetryableClient) ShutdownInstance(ctx context.Context, instanceID int) error {
	return rc.executeWithRetry(ctx, "ShutdownInstance", func() error {
		return rc.Client.ShutdownInstance(ctx, instanceID)
	})
}

// CreateInstance creates a new Linode instance with automatic retry on transient failures.
func (rc *RetryableClient) CreateInstance(ctx context.Context, req *CreateInstanceRequest) (*Instance, error) {
	var instance *Instance

	err := rc.executeWithRetry(ctx, "CreateInstance", func() error {
		var err error

		instance, err = rc.Client.CreateInstance(ctx, req)

		return err
	})

	return instance, err
}

// DeleteInstance deletes a Linode instance with automatic retry on transient failures.
func (rc *RetryableClient) DeleteInstance(ctx context.Context, instanceID int) error {
	return rc.executeWithRetry(ctx, "DeleteInstance", func() error {
		return rc.Client.DeleteInstance(ctx, instanceID)
	})
}

// ResizeInstance resizes a Linode instance with automatic retry on transient failures.
func (rc *RetryableClient) ResizeInstance(ctx context.Context, instanceID int, req ResizeInstanceRequest) error {
	return rc.executeWithRetry(ctx, "ResizeInstance", func() error {
		return rc.Client.ResizeInstance(ctx, instanceID, req)
	})
}

// CreateFirewall creates a new firewall with automatic retry on transient failures.
func (rc *RetryableClient) CreateFirewall(ctx context.Context, req CreateFirewallRequest) (*Firewall, error) {
	var firewall *Firewall

	err := rc.executeWithRetry(ctx, "CreateFirewall", func() error {
		var err error

		firewall, err = rc.Client.CreateFirewall(ctx, req)

		return err
	})

	return firewall, err
}

// UpdateFirewall updates a firewall with automatic retry on transient failures.
func (rc *RetryableClient) UpdateFirewall(ctx context.Context, firewallID int, req UpdateFirewallRequest) (*Firewall, error) {
	var firewall *Firewall

	err := rc.executeWithRetry(ctx, "UpdateFirewall", func() error {
		var err error

		firewall, err = rc.Client.UpdateFirewall(ctx, firewallID, req)

		return err
	})

	return firewall, err
}

// DeleteFirewall deletes a firewall with automatic retry on transient failures.
func (rc *RetryableClient) DeleteFirewall(ctx context.Context, firewallID int) error {
	return rc.executeWithRetry(ctx, "DeleteFirewall", func() error {
		return rc.Client.DeleteFirewall(ctx, firewallID)
	})
}

// CreateDomain creates a new domain with automatic retry on transient failures.
func (rc *RetryableClient) CreateDomain(ctx context.Context, req *CreateDomainRequest) (*Domain, error) {
	var domain *Domain

	err := rc.executeWithRetry(ctx, "CreateDomain", func() error {
		var err error

		domain, err = rc.Client.CreateDomain(ctx, req)

		return err
	})

	return domain, err
}

// UpdateDomain updates a domain with automatic retry on transient failures.
func (rc *RetryableClient) UpdateDomain(ctx context.Context, domainID int, req *UpdateDomainRequest) (*Domain, error) {
	var domain *Domain

	err := rc.executeWithRetry(ctx, "UpdateDomain", func() error {
		var err error

		domain, err = rc.Client.UpdateDomain(ctx, domainID, req)

		return err
	})

	return domain, err
}

// DeleteDomain deletes a domain with automatic retry on transient failures.
func (rc *RetryableClient) DeleteDomain(ctx context.Context, domainID int) error {
	return rc.executeWithRetry(ctx, "DeleteDomain", func() error {
		return rc.Client.DeleteDomain(ctx, domainID)
	})
}

// CreateDomainRecord creates a domain record with automatic retry on transient failures.
func (rc *RetryableClient) CreateDomainRecord(ctx context.Context, domainID int, req *CreateDomainRecordRequest) (*DomainRecord, error) {
	var record *DomainRecord

	err := rc.executeWithRetry(ctx, "CreateDomainRecord", func() error {
		var err error

		record, err = rc.Client.CreateDomainRecord(ctx, domainID, req)

		return err
	})

	return record, err
}

// UpdateDomainRecord updates a domain record with automatic retry on transient failures.
func (rc *RetryableClient) UpdateDomainRecord(ctx context.Context, domainID, recordID int, req *UpdateDomainRecordRequest) (*DomainRecord, error) {
	var record *DomainRecord

	err := rc.executeWithRetry(ctx, "UpdateDomainRecord", func() error {
		var err error

		record, err = rc.Client.UpdateDomainRecord(ctx, domainID, recordID, req)

		return err
	})

	return record, err
}

// DeleteDomainRecord deletes a domain record with automatic retry on transient failures.
func (rc *RetryableClient) DeleteDomainRecord(ctx context.Context, domainID, recordID int) error {
	return rc.executeWithRetry(ctx, "DeleteDomainRecord", func() error {
		return rc.Client.DeleteDomainRecord(ctx, domainID, recordID)
	})
}

// CreateVolume creates a new volume with automatic retry on transient failures.
func (rc *RetryableClient) CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*Volume, error) {
	var volume *Volume

	err := rc.executeWithRetry(ctx, "CreateVolume", func() error {
		var err error

		volume, err = rc.Client.CreateVolume(ctx, req)

		return err
	})

	return volume, err
}

// AttachVolume attaches a volume to a Linode with automatic retry on transient failures.
func (rc *RetryableClient) AttachVolume(ctx context.Context, volumeID int, req AttachVolumeRequest) (*Volume, error) {
	var volume *Volume

	err := rc.executeWithRetry(ctx, "AttachVolume", func() error {
		var err error

		volume, err = rc.Client.AttachVolume(ctx, volumeID, req)

		return err
	})

	return volume, err
}

// DetachVolume detaches a volume from a Linode with automatic retry on transient failures.
func (rc *RetryableClient) DetachVolume(ctx context.Context, volumeID int) error {
	return rc.executeWithRetry(ctx, "DetachVolume", func() error {
		return rc.Client.DetachVolume(ctx, volumeID)
	})
}

// ResizeVolume resizes a volume with automatic retry on transient failures.
func (rc *RetryableClient) ResizeVolume(ctx context.Context, volumeID, size int) (*Volume, error) {
	var volume *Volume

	err := rc.executeWithRetry(ctx, "ResizeVolume", func() error {
		var err error

		volume, err = rc.Client.ResizeVolume(ctx, volumeID, size)

		return err
	})

	return volume, err
}

// DeleteVolume deletes a volume with automatic retry on transient failures.
func (rc *RetryableClient) DeleteVolume(ctx context.Context, volumeID int) error {
	return rc.executeWithRetry(ctx, "DeleteVolume", func() error {
		return rc.Client.DeleteVolume(ctx, volumeID)
	})
}

// CreateNodeBalancer creates a new NodeBalancer with automatic retry on transient failures.
func (rc *RetryableClient) CreateNodeBalancer(ctx context.Context, req CreateNodeBalancerRequest) (*NodeBalancer, error) {
	var nodeBalancer *NodeBalancer

	err := rc.executeWithRetry(ctx, "CreateNodeBalancer", func() error {
		var err error

		nodeBalancer, err = rc.Client.CreateNodeBalancer(ctx, req)

		return err
	})

	return nodeBalancer, err
}

// UpdateNodeBalancer updates a NodeBalancer with automatic retry on transient failures.
func (rc *RetryableClient) UpdateNodeBalancer(ctx context.Context, nodeBalancerID int, req UpdateNodeBalancerRequest) (*NodeBalancer, error) {
	var nodeBalancer *NodeBalancer

	err := rc.executeWithRetry(ctx, "UpdateNodeBalancer", func() error {
		var err error

		nodeBalancer, err = rc.Client.UpdateNodeBalancer(ctx, nodeBalancerID, req)

		return err
	})

	return nodeBalancer, err
}

// DeleteNodeBalancer deletes a NodeBalancer with automatic retry on transient failures.
func (rc *RetryableClient) DeleteNodeBalancer(ctx context.Context, nodeBalancerID int) error {
	return rc.executeWithRetry(ctx, "DeleteNodeBalancer", func() error {
		return rc.Client.DeleteNodeBalancer(ctx, nodeBalancerID)
	})
}

// Stage 5: Object Storage read operations with retry.

// ListObjectStorageBuckets retrieves all Object Storage buckets with automatic retry.
func (rc *RetryableClient) ListObjectStorageBuckets(ctx context.Context) ([]ObjectStorageBucket, error) {
	var buckets []ObjectStorageBucket

	err := rc.executeWithRetry(ctx, "ListObjectStorageBuckets", func() error {
		var err error

		buckets, err = rc.Client.ListObjectStorageBuckets(ctx)

		return err
	})

	return buckets, err
}

// GetObjectStorageBucket retrieves a specific bucket with automatic retry.
func (rc *RetryableClient) GetObjectStorageBucket(ctx context.Context, region, label string) (*ObjectStorageBucket, error) {
	var bucket *ObjectStorageBucket

	err := rc.executeWithRetry(ctx, "GetObjectStorageBucket", func() error {
		var err error

		bucket, err = rc.Client.GetObjectStorageBucket(ctx, region, label)

		return err
	})

	return bucket, err
}

// ListObjectStorageBucketContents lists objects in a bucket with automatic retry.
func (rc *RetryableClient) ListObjectStorageBucketContents(ctx context.Context, region, label string, params map[string]string) ([]ObjectStorageObject, bool, string, error) {
	var objects []ObjectStorageObject

	var isTruncated bool

	var nextMarker string

	err := rc.executeWithRetry(ctx, "ListObjectStorageBucketContents", func() error {
		var err error

		objects, isTruncated, nextMarker, err = rc.Client.ListObjectStorageBucketContents(ctx, region, label, params)

		return err
	})

	return objects, isTruncated, nextMarker, err
}

// ListObjectStorageClusters retrieves Object Storage clusters with automatic retry.
func (rc *RetryableClient) ListObjectStorageClusters(ctx context.Context) ([]ObjectStorageCluster, error) {
	var clusters []ObjectStorageCluster

	err := rc.executeWithRetry(ctx, "ListObjectStorageClusters", func() error {
		var err error

		clusters, err = rc.Client.ListObjectStorageClusters(ctx)

		return err
	})

	return clusters, err
}

// ListObjectStorageTypes retrieves Object Storage types with automatic retry.
func (rc *RetryableClient) ListObjectStorageTypes(ctx context.Context) ([]ObjectStorageType, error) {
	var types []ObjectStorageType

	err := rc.executeWithRetry(ctx, "ListObjectStorageTypes", func() error {
		var err error

		types, err = rc.Client.ListObjectStorageTypes(ctx)

		return err
	})

	return types, err
}

// ListObjectStorageKeys retrieves all Object Storage access keys with automatic retry.
func (rc *RetryableClient) ListObjectStorageKeys(ctx context.Context) ([]ObjectStorageKey, error) {
	var keys []ObjectStorageKey

	err := rc.executeWithRetry(ctx, "ListObjectStorageKeys", func() error {
		var err error

		keys, err = rc.Client.ListObjectStorageKeys(ctx)

		return err
	})

	return keys, err
}

// GetObjectStorageKey retrieves a specific access key with automatic retry.
func (rc *RetryableClient) GetObjectStorageKey(ctx context.Context, keyID int) (*ObjectStorageKey, error) {
	var key *ObjectStorageKey

	err := rc.executeWithRetry(ctx, "GetObjectStorageKey", func() error {
		var err error

		key, err = rc.Client.GetObjectStorageKey(ctx, keyID)

		return err
	})

	return key, err
}

// GetObjectStorageTransfer retrieves Object Storage transfer usage with automatic retry.
func (rc *RetryableClient) GetObjectStorageTransfer(ctx context.Context) (*ObjectStorageTransfer, error) {
	var transfer *ObjectStorageTransfer

	err := rc.executeWithRetry(ctx, "GetObjectStorageTransfer", func() error {
		var err error

		transfer, err = rc.Client.GetObjectStorageTransfer(ctx)

		return err
	})

	return transfer, err
}

// GetObjectStorageBucketAccess retrieves bucket ACL/CORS settings with automatic retry.
func (rc *RetryableClient) GetObjectStorageBucketAccess(ctx context.Context, region, label string) (*ObjectStorageBucketAccess, error) {
	var access *ObjectStorageBucketAccess

	err := rc.executeWithRetry(ctx, "GetObjectStorageBucketAccess", func() error {
		var err error

		access, err = rc.Client.GetObjectStorageBucketAccess(ctx, region, label)

		return err
	})

	return access, err
}

// CreateObjectStorageBucket creates a new Object Storage bucket with automatic retry.
func (rc *RetryableClient) CreateObjectStorageBucket(ctx context.Context, req CreateObjectStorageBucketRequest) (*ObjectStorageBucket, error) {
	var bucket *ObjectStorageBucket

	err := rc.executeWithRetry(ctx, "CreateObjectStorageBucket", func() error {
		var err error

		bucket, err = rc.Client.CreateObjectStorageBucket(ctx, req)

		return err
	})

	return bucket, err
}

// DeleteObjectStorageBucket deletes an Object Storage bucket with automatic retry.
func (rc *RetryableClient) DeleteObjectStorageBucket(ctx context.Context, region, label string) error {
	return rc.executeWithRetry(ctx, "DeleteObjectStorageBucket", func() error {
		return rc.Client.DeleteObjectStorageBucket(ctx, region, label)
	})
}

// UpdateObjectStorageBucketAccess updates bucket access settings with automatic retry.
func (rc *RetryableClient) UpdateObjectStorageBucketAccess(ctx context.Context, region, label string, req UpdateObjectStorageBucketAccessRequest) error {
	return rc.executeWithRetry(ctx, "UpdateObjectStorageBucketAccess", func() error {
		return rc.Client.UpdateObjectStorageBucketAccess(ctx, region, label, req)
	})
}

// CreateObjectStorageKey creates a new Object Storage access key with automatic retry.
func (rc *RetryableClient) CreateObjectStorageKey(ctx context.Context, req CreateObjectStorageKeyRequest) (*ObjectStorageKey, error) {
	var key *ObjectStorageKey

	err := rc.executeWithRetry(ctx, "CreateObjectStorageKey", func() error {
		var err error

		key, err = rc.Client.CreateObjectStorageKey(ctx, req)

		return err
	})

	return key, err
}

// UpdateObjectStorageKey updates an Object Storage access key with automatic retry.
func (rc *RetryableClient) UpdateObjectStorageKey(ctx context.Context, keyID int, req UpdateObjectStorageKeyRequest) error {
	return rc.executeWithRetry(ctx, "UpdateObjectStorageKey", func() error {
		return rc.Client.UpdateObjectStorageKey(ctx, keyID, req)
	})
}

// DeleteObjectStorageKey revokes an Object Storage access key with automatic retry.
func (rc *RetryableClient) DeleteObjectStorageKey(ctx context.Context, keyID int) error {
	return rc.executeWithRetry(ctx, "DeleteObjectStorageKey", func() error {
		return rc.Client.DeleteObjectStorageKey(ctx, keyID)
	})
}

// CreatePresignedURL generates a presigned URL with automatic retry.
func (rc *RetryableClient) CreatePresignedURL(ctx context.Context, region, label string, req PresignedURLRequest) (*PresignedURLResponse, error) {
	var result *PresignedURLResponse

	err := rc.executeWithRetry(ctx, "CreatePresignedURL", func() error {
		var retryErr error

		result, retryErr = rc.Client.CreatePresignedURL(ctx, region, label, req)

		return retryErr
	})

	return result, err
}

// GetObjectACL retrieves an object's ACL with automatic retry.
func (rc *RetryableClient) GetObjectACL(ctx context.Context, region, label, name string) (*ObjectACL, error) {
	var result *ObjectACL

	err := rc.executeWithRetry(ctx, "GetObjectACL", func() error {
		var retryErr error

		result, retryErr = rc.Client.GetObjectACL(ctx, region, label, name)

		return retryErr
	})

	return result, err
}

// UpdateObjectACL updates an object's ACL with automatic retry.
func (rc *RetryableClient) UpdateObjectACL(ctx context.Context, region, label string, req ObjectACLUpdateRequest) (*ObjectACL, error) {
	var result *ObjectACL

	err := rc.executeWithRetry(ctx, "UpdateObjectACL", func() error {
		var retryErr error

		result, retryErr = rc.Client.UpdateObjectACL(ctx, region, label, req)

		return retryErr
	})

	return result, err
}

// GetBucketSSL retrieves a bucket's SSL status with automatic retry.
func (rc *RetryableClient) GetBucketSSL(ctx context.Context, region, label string) (*BucketSSL, error) {
	var result *BucketSSL

	err := rc.executeWithRetry(ctx, "GetBucketSSL", func() error {
		var retryErr error

		result, retryErr = rc.Client.GetBucketSSL(ctx, region, label)

		return retryErr
	})

	return result, err
}

// DeleteBucketSSL removes a bucket's SSL certificate with automatic retry.
func (rc *RetryableClient) DeleteBucketSSL(ctx context.Context, region, label string) error {
	return rc.executeWithRetry(ctx, "DeleteBucketSSL", func() error {
		return rc.Client.DeleteBucketSSL(ctx, region, label)
	})
}

// LKE (Kubernetes Engine) operations

// ListLKEClusters retrieves all LKE clusters with automatic retry on transient failures.
func (rc *RetryableClient) ListLKEClusters(ctx context.Context) ([]LKECluster, error) {
	var clusters []LKECluster

	err := rc.executeWithRetry(ctx, "ListLKEClusters", func() error {
		var err error

		clusters, err = rc.Client.ListLKEClusters(ctx)

		return err
	})

	return clusters, err
}

// GetLKECluster retrieves a single LKE cluster by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetLKECluster(ctx context.Context, clusterID int) (*LKECluster, error) {
	var cluster *LKECluster

	err := rc.executeWithRetry(ctx, "GetLKECluster", func() error {
		var err error

		cluster, err = rc.Client.GetLKECluster(ctx, clusterID)

		return err
	})

	return cluster, err
}

// CreateLKECluster creates a new LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) CreateLKECluster(ctx context.Context, req *CreateLKEClusterRequest) (*LKECluster, error) {
	var cluster *LKECluster

	err := rc.executeWithRetry(ctx, "CreateLKECluster", func() error {
		var err error

		cluster, err = rc.Client.CreateLKECluster(ctx, req)

		return err
	})

	return cluster, err
}

// UpdateLKECluster updates an LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) UpdateLKECluster(ctx context.Context, clusterID int, req UpdateLKEClusterRequest) (*LKECluster, error) {
	var cluster *LKECluster

	err := rc.executeWithRetry(ctx, "UpdateLKECluster", func() error {
		var err error

		cluster, err = rc.Client.UpdateLKECluster(ctx, clusterID, req)

		return err
	})

	return cluster, err
}

// DeleteLKECluster deletes an LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) DeleteLKECluster(ctx context.Context, clusterID int) error {
	return rc.executeWithRetry(ctx, "DeleteLKECluster", func() error {
		return rc.Client.DeleteLKECluster(ctx, clusterID)
	})
}

// RecycleLKECluster recycles all nodes in an LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) RecycleLKECluster(ctx context.Context, clusterID int) error {
	return rc.executeWithRetry(ctx, "RecycleLKECluster", func() error {
		return rc.Client.RecycleLKECluster(ctx, clusterID)
	})
}

// RegenerateLKECluster regenerates the service token for an LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) RegenerateLKECluster(ctx context.Context, clusterID int) error {
	return rc.executeWithRetry(ctx, "RegenerateLKECluster", func() error {
		return rc.Client.RegenerateLKECluster(ctx, clusterID)
	})
}

// ListLKENodePools retrieves all node pools for an LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) ListLKENodePools(ctx context.Context, clusterID int) ([]LKENodePool, error) {
	var pools []LKENodePool

	err := rc.executeWithRetry(ctx, "ListLKENodePools", func() error {
		var err error

		pools, err = rc.Client.ListLKENodePools(ctx, clusterID)

		return err
	})

	return pools, err
}

// GetLKENodePool retrieves a single node pool by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetLKENodePool(ctx context.Context, clusterID, poolID int) (*LKENodePool, error) {
	var pool *LKENodePool

	err := rc.executeWithRetry(ctx, "GetLKENodePool", func() error {
		var err error

		pool, err = rc.Client.GetLKENodePool(ctx, clusterID, poolID)

		return err
	})

	return pool, err
}

// CreateLKENodePool creates a new node pool with automatic retry on transient failures.
func (rc *RetryableClient) CreateLKENodePool(ctx context.Context, clusterID int, req *CreateLKENodePoolRequest) (*LKENodePool, error) {
	var pool *LKENodePool

	err := rc.executeWithRetry(ctx, "CreateLKENodePool", func() error {
		var err error

		pool, err = rc.Client.CreateLKENodePool(ctx, clusterID, req)

		return err
	})

	return pool, err
}

// UpdateLKENodePool updates a node pool with automatic retry on transient failures.
func (rc *RetryableClient) UpdateLKENodePool(ctx context.Context, clusterID, poolID int, req UpdateLKENodePoolRequest) (*LKENodePool, error) {
	var pool *LKENodePool

	err := rc.executeWithRetry(ctx, "UpdateLKENodePool", func() error {
		var err error

		pool, err = rc.Client.UpdateLKENodePool(ctx, clusterID, poolID, req)

		return err
	})

	return pool, err
}

// DeleteLKENodePool deletes a node pool with automatic retry on transient failures.
func (rc *RetryableClient) DeleteLKENodePool(ctx context.Context, clusterID, poolID int) error {
	return rc.executeWithRetry(ctx, "DeleteLKENodePool", func() error {
		return rc.Client.DeleteLKENodePool(ctx, clusterID, poolID)
	})
}

// RecycleLKENodePool recycles all nodes in a node pool with automatic retry on transient failures.
func (rc *RetryableClient) RecycleLKENodePool(ctx context.Context, clusterID, poolID int) error {
	return rc.executeWithRetry(ctx, "RecycleLKENodePool", func() error {
		return rc.Client.RecycleLKENodePool(ctx, clusterID, poolID)
	})
}

// GetLKENode retrieves a single node by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetLKENode(ctx context.Context, clusterID int, nodeID string) (*LKENode, error) {
	var node *LKENode

	err := rc.executeWithRetry(ctx, "GetLKENode", func() error {
		var err error

		node, err = rc.Client.GetLKENode(ctx, clusterID, nodeID)

		return err
	})

	return node, err
}

// DeleteLKENode deletes a node with automatic retry on transient failures.
func (rc *RetryableClient) DeleteLKENode(ctx context.Context, clusterID int, nodeID string) error {
	return rc.executeWithRetry(ctx, "DeleteLKENode", func() error {
		return rc.Client.DeleteLKENode(ctx, clusterID, nodeID)
	})
}

// RecycleLKENode recycles a specific node with automatic retry on transient failures.
func (rc *RetryableClient) RecycleLKENode(ctx context.Context, clusterID int, nodeID string) error {
	return rc.executeWithRetry(ctx, "RecycleLKENode", func() error {
		return rc.Client.RecycleLKENode(ctx, clusterID, nodeID)
	})
}

// GetLKEKubeconfig retrieves the kubeconfig for an LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) GetLKEKubeconfig(ctx context.Context, clusterID int) (*LKEKubeconfig, error) {
	var kubeconfig *LKEKubeconfig

	err := rc.executeWithRetry(ctx, "GetLKEKubeconfig", func() error {
		var err error

		kubeconfig, err = rc.Client.GetLKEKubeconfig(ctx, clusterID)

		return err
	})

	return kubeconfig, err
}

// DeleteLKEKubeconfig deletes the kubeconfig for an LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) DeleteLKEKubeconfig(ctx context.Context, clusterID int) error {
	return rc.executeWithRetry(ctx, "DeleteLKEKubeconfig", func() error {
		return rc.Client.DeleteLKEKubeconfig(ctx, clusterID)
	})
}

// GetLKEDashboard retrieves the dashboard URL for an LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) GetLKEDashboard(ctx context.Context, clusterID int) (*LKEDashboard, error) {
	var dashboard *LKEDashboard

	err := rc.executeWithRetry(ctx, "GetLKEDashboard", func() error {
		var err error

		dashboard, err = rc.Client.GetLKEDashboard(ctx, clusterID)

		return err
	})

	return dashboard, err
}

// ListLKEAPIEndpoints retrieves API endpoints for an LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) ListLKEAPIEndpoints(ctx context.Context, clusterID int) ([]LKEAPIEndpoint, error) {
	var endpoints []LKEAPIEndpoint

	err := rc.executeWithRetry(ctx, "ListLKEAPIEndpoints", func() error {
		var err error

		endpoints, err = rc.Client.ListLKEAPIEndpoints(ctx, clusterID)

		return err
	})

	return endpoints, err
}

// DeleteLKEServiceToken deletes the service token for an LKE cluster with automatic retry on transient failures.
func (rc *RetryableClient) DeleteLKEServiceToken(ctx context.Context, clusterID int) error {
	return rc.executeWithRetry(ctx, "DeleteLKEServiceToken", func() error {
		return rc.Client.DeleteLKEServiceToken(ctx, clusterID)
	})
}

// GetLKEControlPlaneACL retrieves the control plane ACL with automatic retry on transient failures.
func (rc *RetryableClient) GetLKEControlPlaneACL(ctx context.Context, clusterID int) (*LKEControlPlaneACL, error) {
	var acl *LKEControlPlaneACL

	err := rc.executeWithRetry(ctx, "GetLKEControlPlaneACL", func() error {
		var err error

		acl, err = rc.Client.GetLKEControlPlaneACL(ctx, clusterID)

		return err
	})

	return acl, err
}

// UpdateLKEControlPlaneACL updates the control plane ACL with automatic retry on transient failures.
func (rc *RetryableClient) UpdateLKEControlPlaneACL(ctx context.Context, clusterID int, req UpdateLKEControlPlaneACLRequest) (*LKEControlPlaneACL, error) {
	var acl *LKEControlPlaneACL

	err := rc.executeWithRetry(ctx, "UpdateLKEControlPlaneACL", func() error {
		var err error

		acl, err = rc.Client.UpdateLKEControlPlaneACL(ctx, clusterID, req)

		return err
	})

	return acl, err
}

// DeleteLKEControlPlaneACL deletes the control plane ACL with automatic retry on transient failures.
func (rc *RetryableClient) DeleteLKEControlPlaneACL(ctx context.Context, clusterID int) error {
	return rc.executeWithRetry(ctx, "DeleteLKEControlPlaneACL", func() error {
		return rc.Client.DeleteLKEControlPlaneACL(ctx, clusterID)
	})
}

// ListLKEVersions retrieves all LKE versions with automatic retry on transient failures.
func (rc *RetryableClient) ListLKEVersions(ctx context.Context) ([]LKEVersion, error) {
	var versions []LKEVersion

	err := rc.executeWithRetry(ctx, "ListLKEVersions", func() error {
		var err error

		versions, err = rc.Client.ListLKEVersions(ctx)

		return err
	})

	return versions, err
}

// GetLKEVersion retrieves a specific LKE version with automatic retry on transient failures.
func (rc *RetryableClient) GetLKEVersion(ctx context.Context, versionID string) (*LKEVersion, error) {
	var version *LKEVersion

	err := rc.executeWithRetry(ctx, "GetLKEVersion", func() error {
		var err error

		version, err = rc.Client.GetLKEVersion(ctx, versionID)

		return err
	})

	return version, err
}

// ListLKETypes retrieves all LKE types with automatic retry on transient failures.
func (rc *RetryableClient) ListLKETypes(ctx context.Context) ([]LKEType, error) {
	var types []LKEType

	err := rc.executeWithRetry(ctx, "ListLKETypes", func() error {
		var err error

		types, err = rc.Client.ListLKETypes(ctx)

		return err
	})

	return types, err
}

// ListLKETierVersions retrieves all LKE tier versions with automatic retry on transient failures.
func (rc *RetryableClient) ListLKETierVersions(ctx context.Context) ([]LKETierVersion, error) {
	var versions []LKETierVersion

	err := rc.executeWithRetry(ctx, "ListLKETierVersions", func() error {
		var err error

		versions, err = rc.Client.ListLKETierVersions(ctx)

		return err
	})

	return versions, err
}

// VPC operations

// ListVPCs retrieves all VPCs with automatic retry on transient failures.
func (rc *RetryableClient) ListVPCs(ctx context.Context) ([]VPC, error) {
	var vpcs []VPC

	err := rc.executeWithRetry(ctx, "ListVPCs", func() error {
		var retryErr error

		vpcs, retryErr = rc.Client.ListVPCs(ctx)

		return retryErr
	})

	return vpcs, err
}

// GetVPC retrieves a single VPC by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetVPC(ctx context.Context, vpcID int) (*VPC, error) {
	var vpc *VPC

	err := rc.executeWithRetry(ctx, "GetVPC", func() error {
		var retryErr error

		vpc, retryErr = rc.Client.GetVPC(ctx, vpcID)

		return retryErr
	})

	return vpc, err
}

// CreateVPC creates a new VPC with automatic retry on transient failures.
func (rc *RetryableClient) CreateVPC(ctx context.Context, req CreateVPCRequest) (*VPC, error) {
	var vpc *VPC

	err := rc.executeWithRetry(ctx, "CreateVPC", func() error {
		var retryErr error

		vpc, retryErr = rc.Client.CreateVPC(ctx, req)

		return retryErr
	})

	return vpc, err
}

// UpdateVPC updates a VPC with automatic retry on transient failures.
func (rc *RetryableClient) UpdateVPC(ctx context.Context, vpcID int, req UpdateVPCRequest) (*VPC, error) {
	var vpc *VPC

	err := rc.executeWithRetry(ctx, "UpdateVPC", func() error {
		var retryErr error

		vpc, retryErr = rc.Client.UpdateVPC(ctx, vpcID, req)

		return retryErr
	})

	return vpc, err
}

// DeleteVPC deletes a VPC with automatic retry on transient failures.
func (rc *RetryableClient) DeleteVPC(ctx context.Context, vpcID int) error {
	return rc.executeWithRetry(ctx, "DeleteVPC", func() error {
		return rc.Client.DeleteVPC(ctx, vpcID)
	})
}

// ListVPCIPs retrieves all VPC IP addresses with automatic retry on transient failures.
func (rc *RetryableClient) ListVPCIPs(ctx context.Context) ([]VPCIP, error) {
	var ips []VPCIP

	err := rc.executeWithRetry(ctx, "ListVPCIPs", func() error {
		var retryErr error

		ips, retryErr = rc.Client.ListVPCIPs(ctx)

		return retryErr
	})

	return ips, err
}

// ListVPCIPAddresses retrieves IP addresses for a specific VPC with automatic retry on transient failures.
func (rc *RetryableClient) ListVPCIPAddresses(ctx context.Context, vpcID int) ([]VPCIP, error) {
	var ips []VPCIP

	err := rc.executeWithRetry(ctx, "ListVPCIPAddresses", func() error {
		var retryErr error

		ips, retryErr = rc.Client.ListVPCIPAddresses(ctx, vpcID)

		return retryErr
	})

	return ips, err
}

// ListVPCSubnets retrieves all subnets for a VPC with automatic retry on transient failures.
func (rc *RetryableClient) ListVPCSubnets(ctx context.Context, vpcID int) ([]VPCSubnet, error) {
	var subnets []VPCSubnet

	err := rc.executeWithRetry(ctx, "ListVPCSubnets", func() error {
		var retryErr error

		subnets, retryErr = rc.Client.ListVPCSubnets(ctx, vpcID)

		return retryErr
	})

	return subnets, err
}

// GetVPCSubnet retrieves a single subnet by ID with automatic retry on transient failures.
func (rc *RetryableClient) GetVPCSubnet(ctx context.Context, vpcID, subnetID int) (*VPCSubnet, error) {
	var subnet *VPCSubnet

	err := rc.executeWithRetry(ctx, "GetVPCSubnet", func() error {
		var retryErr error

		subnet, retryErr = rc.Client.GetVPCSubnet(ctx, vpcID, subnetID)

		return retryErr
	})

	return subnet, err
}

// CreateVPCSubnet creates a new subnet in a VPC with automatic retry on transient failures.
func (rc *RetryableClient) CreateVPCSubnet(ctx context.Context, vpcID int, req CreateSubnetRequest) (*VPCSubnet, error) {
	var subnet *VPCSubnet

	err := rc.executeWithRetry(ctx, "CreateVPCSubnet", func() error {
		var retryErr error

		subnet, retryErr = rc.Client.CreateVPCSubnet(ctx, vpcID, req)

		return retryErr
	})

	return subnet, err
}

// UpdateVPCSubnet updates a subnet in a VPC with automatic retry on transient failures.
func (rc *RetryableClient) UpdateVPCSubnet(ctx context.Context, vpcID, subnetID int, req UpdateSubnetRequest) (*VPCSubnet, error) {
	var subnet *VPCSubnet

	err := rc.executeWithRetry(ctx, "UpdateVPCSubnet", func() error {
		var retryErr error

		subnet, retryErr = rc.Client.UpdateVPCSubnet(ctx, vpcID, subnetID, req)

		return retryErr
	})

	return subnet, err
}

// DeleteVPCSubnet deletes a subnet from a VPC with automatic retry on transient failures.
func (rc *RetryableClient) DeleteVPCSubnet(ctx context.Context, vpcID, subnetID int) error {
	return rc.executeWithRetry(ctx, "DeleteVPCSubnet", func() error {
		return rc.Client.DeleteVPCSubnet(ctx, vpcID, subnetID)
	})
}

func (rc *RetryableClient) executeWithRetry(ctx context.Context, _ string, retryFunc func() error) error {
	var lastErr error

	for attempt := 0; attempt <= rc.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := rc.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return fmt.Errorf("context canceled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		err := retryFunc()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt == rc.retryConfig.MaxRetries {
			break
		}

		if !rc.shouldRetry(err) {
			return err
		}
	}

	return lastErr
}

func (rc *RetryableClient) calculateDelay(attempt int) time.Duration {
	delay := float64(rc.retryConfig.BaseDelay) * math.Pow(rc.retryConfig.BackoffFactor, float64(attempt-1))

	if rc.retryConfig.JitterEnabled {
		jitterMax := big.NewInt(int64(delay * jitterPercent))
		if jitterMax.Int64() > 0 {
			jitterBig, _ := rand.Int(rand.Reader, jitterMax)
			jitter := float64(jitterBig.Int64())
			delay += jitter
		}
	}

	maxDelay := float64(rc.retryConfig.MaxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}

	return time.Duration(delay)
}

func (*RetryableClient) shouldRetry(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.IsRateLimitError() || apiErr.IsServerError() {
			return true
		}

		if apiErr.IsAuthenticationError() || apiErr.IsForbiddenError() {
			return false
		}
	}

	if IsNetworkError(err) || IsTimeoutError(err) {
		return true
	}

	return false
}
