package tools

import "errors"

// Sentinel errors for Linode instance operations.
var (
	ErrEnvironmentNotFound    = errors.New("environment not found in configuration")
	ErrLinodeConfigIncomplete = errors.New("linode configuration is incomplete: check your API URL and token")
	ErrInstanceIDRequired     = errors.New("instance_id is required")
	ErrInvalidInstanceID      = errors.New("instance_id must be a valid integer")
	ErrLinodeIDRequired       = errors.New("linode_id is required")
	ErrLinodeIDInvalid        = errors.New("linode_id must be a valid integer")
	ErrBackupIDRequired       = errors.New("backup_id is required")
	ErrBackupIDInvalid        = errors.New("backup_id must be a valid integer")
	ErrDiskIDRequired         = errors.New("disk_id is required")
	ErrDiskIDInvalid          = errors.New("disk_id must be a valid integer")
)

// Sentinel errors for SSH key validation.
var (
	ErrSSHKeyRequired      = errors.New("ssh_key is required")
	ErrSSHKeyInvalidFormat = errors.New("invalid SSH key format: must start with ssh-rsa, ssh-ed25519, or ecdsa-sha2-*")
	ErrSSHKeyInvalidLength = errors.New("invalid SSH key length: key appears malformed")
)

// Sentinel errors for password validation.
var (
	ErrPasswordTooShort    = errors.New("root_pass must be at least 12 characters")
	ErrPasswordTooLong     = errors.New("root_pass must not exceed 128 characters")
	ErrPasswordMissingChar = errors.New("root_pass must contain uppercase, lowercase, and digits")
)

// Sentinel errors for DNS validation.
var (
	ErrDNSNameTooLong       = errors.New("DNS record name exceeds maximum length of 253 characters")
	ErrDNSNameInvalid       = errors.New("invalid DNS record name: must contain only alphanumeric characters, hyphens, and dots")
	ErrDNSTargetRequired    = errors.New("target is required")
	ErrDNSTargetInvalidA    = errors.New("a record target must be a valid IPv4 address")
	ErrDNSTargetPrivateIP   = errors.New("a record target cannot be a private IP address")
	ErrDNSTargetInvalidAAAA = errors.New("aaaa record target must be a valid IPv6 address")
)

// Sentinel errors for firewall validation.
var (
	ErrFirewallPolicyInvalid = errors.New("firewall policy must be 'ACCEPT' or 'DROP'")
)

// Sentinel errors for volume validation.
var (
	ErrVolumeSizeTooSmall = errors.New("volume size must be at least 10 GB")
	ErrVolumeSizeTooLarge = errors.New("volume size cannot exceed 10240 GB (10 TB)")
)

// Sentinel errors for bucket validation.
var (
	ErrBucketLabelRequired  = errors.New("label is required")
	ErrBucketLabelTooShort  = errors.New("bucket label must be at least 3 characters")
	ErrBucketLabelTooLong   = errors.New("bucket label must not exceed 63 characters")
	ErrBucketLabelStartEnd  = errors.New("bucket label must start and end with a lowercase letter or number")
	ErrBucketLabelInvalid   = errors.New("bucket label must contain only lowercase letters, numbers, and hyphens")
	ErrBucketLabelIPAddress = errors.New("bucket label must not be formatted as an IP address")
	ErrBucketLabelXNPrefix  = errors.New("bucket label must not use the 'xn--' prefix (reserved for internationalized domain names)")
	ErrBucketACLInvalid     = errors.New("acl must be one of: private, public-read, authenticated-read, public-read-write")
	ErrBucketRegionRequired = errors.New("region is required")
)

// Sentinel errors for access key validation.
var (
	ErrKeyLabelRequired        = errors.New("label is required")
	ErrKeyLabelTooLong         = errors.New("access key label must not exceed 50 characters")
	ErrKeyIDRequired           = errors.New("key_id is required and must be a positive integer")
	ErrKeyPermissionsInvalid   = errors.New("bucket_access permissions must be 'read_only' or 'read_write'")
	ErrKeyBucketNameRequired   = errors.New("bucket_access entries must include bucket_name")
	ErrKeyBucketRegionRequired = errors.New("bucket_access entries must include region")
)

// Sentinel errors for presigned URL validation.
var (
	ErrPresignedMethodInvalid  = errors.New("method must be 'GET' or 'PUT'")
	ErrPresignedExpiresInvalid = errors.New("expires_in must be between 1 and 604800 seconds (7 days)")
	ErrObjectNameRequired      = errors.New("name (object key) is required")
)

// Sentinel errors for LKE validation.
var (
	ErrLKEClusterIDRequired = errors.New("cluster_id is required")
	ErrLKEClusterIDInvalid  = errors.New("cluster_id must be a valid integer")
	ErrLKEPoolIDRequired    = errors.New("pool_id is required")
	ErrLKEPoolIDInvalid     = errors.New("pool_id must be a valid integer")
)

// Sentinel errors for VPC validation.
var (
	ErrVPCIDRequired    = errors.New("vpc_id is required")
	ErrVPCIDInvalid     = errors.New("vpc_id must be a valid integer")
	ErrSubnetIDRequired = errors.New("subnet_id is required")
	ErrSubnetIDInvalid  = errors.New("subnet_id must be a valid integer")
)
