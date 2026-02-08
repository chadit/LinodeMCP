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
func (rc *RetryableClient) CreateInstance(ctx context.Context, req CreateInstanceRequest) (*Instance, error) {
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
func (rc *RetryableClient) CreateDomain(ctx context.Context, req CreateDomainRequest) (*Domain, error) {
	var domain *Domain

	err := rc.executeWithRetry(ctx, "CreateDomain", func() error {
		var err error

		domain, err = rc.Client.CreateDomain(ctx, req)

		return err
	})

	return domain, err
}

// UpdateDomain updates a domain with automatic retry on transient failures.
func (rc *RetryableClient) UpdateDomain(ctx context.Context, domainID int, req UpdateDomainRequest) (*Domain, error) {
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
func (rc *RetryableClient) CreateDomainRecord(ctx context.Context, domainID int, req CreateDomainRecordRequest) (*DomainRecord, error) {
	var record *DomainRecord

	err := rc.executeWithRetry(ctx, "CreateDomainRecord", func() error {
		var err error

		record, err = rc.Client.CreateDomainRecord(ctx, domainID, req)

		return err
	})

	return record, err
}

// UpdateDomainRecord updates a domain record with automatic retry on transient failures.
func (rc *RetryableClient) UpdateDomainRecord(ctx context.Context, domainID, recordID int, req UpdateDomainRecordRequest) (*DomainRecord, error) {
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
func (rc *RetryableClient) CreateVolume(ctx context.Context, req CreateVolumeRequest) (*Volume, error) {
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
func (rc *RetryableClient) ResizeVolume(ctx context.Context, volumeID int, size int) (*Volume, error) {
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

func (rc *RetryableClient) executeWithRetry(ctx context.Context, _ string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= rc.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := rc.calculateDelay(attempt)
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		err := fn()
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

func (rc *RetryableClient) shouldRetry(err error) bool {
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
