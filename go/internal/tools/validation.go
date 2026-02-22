package tools

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"unicode"
)

// Limits for input validation across Linode API tool parameters.
const (
	minPasswordLength    = 12
	maxPasswordLength    = 128
	maxDNSNameLength     = 253
	minVolumeSizeGB      = 10
	maxVolumeSizeGB      = 10240
	minSSHKeyLength      = 80
	maxSSHKeyLength      = 16000
	minBucketLabelLength = 3
	maxBucketLabelLength = 63
	maxKeyLabelLength    = 50
)

// ErrSSHKeyRequired and related errors are returned when SSH key input fails format checks.
var (
	ErrSSHKeyRequired      = errors.New("ssh_key is required")
	ErrSSHKeyInvalidFormat = errors.New("invalid SSH key format: must start with ssh-rsa, ssh-ed25519, or ecdsa-sha2-*")
	ErrSSHKeyInvalidLength = errors.New("invalid SSH key length: key appears malformed")
)

// ErrPasswordTooShort and related errors are returned when root_pass fails complexity checks.
var (
	ErrPasswordTooShort    = errors.New("root_pass must be at least 12 characters")
	ErrPasswordTooLong     = errors.New("root_pass must not exceed 128 characters")
	ErrPasswordMissingChar = errors.New("root_pass must contain uppercase, lowercase, and digits")
)

// ErrDNSNameTooLong and related errors are returned when DNS record input fails format checks.
var (
	ErrDNSNameTooLong       = errors.New("DNS record name exceeds maximum length of 253 characters")
	ErrDNSNameInvalid       = errors.New("invalid DNS record name: must contain only alphanumeric characters, hyphens, and dots")
	ErrDNSTargetRequired    = errors.New("target is required")
	ErrDNSTargetInvalidA    = errors.New("a record target must be a valid IPv4 address")
	ErrDNSTargetPrivateIP   = errors.New("a record target cannot be a private IP address")
	ErrDNSTargetInvalidAAAA = errors.New("aaaa record target must be a valid IPv6 address")
)

// ErrFirewallPolicyInvalid is returned when a firewall inbound/outbound policy is not ACCEPT or DROP.
var (
	ErrFirewallPolicyInvalid = errors.New("firewall policy must be 'ACCEPT' or 'DROP'")
)

// ErrVolumeSizeTooSmall and ErrVolumeSizeTooLarge are returned when volume size is outside Linode's allowed range.
var (
	ErrVolumeSizeTooSmall = errors.New("volume size must be at least 10 GB")
	ErrVolumeSizeTooLarge = errors.New("volume size cannot exceed 10240 GB (10 TB)")
)

// validSSHKeyPrefixes returns the algorithm prefixes accepted by Linode for SSH keys.
func validSSHKeyPrefixes() []string {
	return []string{
		"ssh-rsa",
		"ssh-ed25519",
		"ecdsa-sha2-nistp256",
		"ecdsa-sha2-nistp384",
		"ecdsa-sha2-nistp521",
		"ssh-dss",
	}
}

func validateSSHKey(key string) error {
	if key == "" {
		return ErrSSHKeyRequired
	}

	key = strings.TrimSpace(key)
	validPrefix := false

	for _, prefix := range validSSHKeyPrefixes() {
		if strings.HasPrefix(key, prefix+" ") {
			validPrefix = true

			break
		}
	}

	if !validPrefix {
		return ErrSSHKeyInvalidFormat
	}

	// Basic length check (RSA 2048 is ~400 chars, 4096 is ~800).
	if len(key) < minSSHKeyLength || len(key) > maxSSHKeyLength {
		return ErrSSHKeyInvalidLength
	}

	return nil
}

// validateRootPassword checks length and character complexity requirements for instance root passwords.
func validateRootPassword(password string) error {
	if password == "" {
		return nil // Password is optional.
	}

	if len(password) < minPasswordLength {
		return ErrPasswordTooShort
	}

	if len(password) > maxPasswordLength {
		return ErrPasswordTooLong
	}

	var hasUpper, hasLower, hasDigit bool

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit {
		return ErrPasswordMissingChar
	}

	return nil
}

// validDNSNameRegex matches RFC 1035 hostnames; validBucketLabelRegex matches S3-compatible bucket names.
var (
	validDNSNameRegex     = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$|^@$|^$`)
	validBucketLabelRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]{1,2}$`)
)

func validateDNSRecordName(name string) error {
	if len(name) > maxDNSNameLength {
		return ErrDNSNameTooLong
	}

	if name != "" && name != "@" && !validDNSNameRegex.MatchString(name) {
		return ErrDNSNameInvalid
	}

	return nil
}

func validateDNSRecordTarget(recordType, target string) error {
	if target == "" {
		return ErrDNSTargetRequired
	}

	switch strings.ToUpper(recordType) {
	case "A":
		ip := net.ParseIP(target)
		if ip == nil || ip.To4() == nil {
			return ErrDNSTargetInvalidA
		}

		if ip.IsPrivate() || ip.IsLoopback() {
			return ErrDNSTargetPrivateIP
		}
	case "AAAA":
		ip := net.ParseIP(target)
		if ip == nil || ip.To4() != nil {
			return ErrDNSTargetInvalidAAAA
		}
	case "CNAME", "NS", "MX":
		if !validDNSNameRegex.MatchString(target) && target != "@" {
			return fmt.Errorf("%s record target must be a valid hostname: %w", recordType, ErrDNSNameInvalid)
		}
	}

	return nil
}

// validateFirewallPolicy rejects any policy value other than ACCEPT or DROP.
func validateFirewallPolicy(policy string) error {
	upper := strings.ToUpper(policy)
	if upper != "ACCEPT" && upper != "DROP" {
		return fmt.Errorf("got '%s': %w", policy, ErrFirewallPolicyInvalid)
	}

	return nil
}

// ErrBucketLabelRequired and related errors are returned when bucket label or ACL input is invalid per S3 naming rules.
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

// validateBucketLabel checks that a bucket label conforms to S3 naming rules (3-63 chars, lowercase alphanumeric and hyphens).
func validateBucketLabel(label string) error {
	if label == "" {
		return ErrBucketLabelRequired
	}

	if len(label) < minBucketLabelLength {
		return ErrBucketLabelTooShort
	}

	if len(label) > maxBucketLabelLength {
		return ErrBucketLabelTooLong
	}

	if net.ParseIP(label) != nil {
		return ErrBucketLabelIPAddress
	}

	if strings.HasPrefix(label, "xn--") {
		return ErrBucketLabelXNPrefix
	}

	if !validBucketLabelRegex.MatchString(label) {
		return ErrBucketLabelInvalid
	}

	first := label[0]
	last := label[len(label)-1]

	firstValid := (first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')
	lastValid := (last >= 'a' && last <= 'z') || (last >= '0' && last <= '9')

	if !firstValid || !lastValid {
		return ErrBucketLabelStartEnd
	}

	return nil
}

// validateBucketACL ensures the ACL value is one of the four S3-compatible options supported by Linode.
func validateBucketACL(acl string) error {
	switch acl {
	case "private", "public-read", "authenticated-read", "public-read-write":
		return nil
	default:
		return fmt.Errorf("got '%s': %w", acl, ErrBucketACLInvalid)
	}
}

// validateVolumeSize ensures the requested volume size is within Linode's 10 GB to 10 TB range.
func validateVolumeSize(size int) error {
	if size < minVolumeSizeGB {
		return ErrVolumeSizeTooSmall
	}

	if size > maxVolumeSizeGB {
		return ErrVolumeSizeTooLarge
	}

	return nil
}

// ErrKeyLabelRequired and related errors are returned when Object Storage access key parameters are invalid.
var (
	ErrKeyLabelRequired        = errors.New("label is required")
	ErrKeyLabelTooLong         = errors.New("access key label must not exceed 50 characters")
	ErrKeyIDRequired           = errors.New("key_id is required and must be a positive integer")
	ErrKeyPermissionsInvalid   = errors.New("bucket_access permissions must be 'read_only' or 'read_write'")
	ErrKeyBucketNameRequired   = errors.New("bucket_access entries must include bucket_name")
	ErrKeyBucketRegionRequired = errors.New("bucket_access entries must include region")
)

// ErrPresignedMethodInvalid and related errors are returned when presigned URL parameters are out of range.
var (
	ErrPresignedMethodInvalid  = errors.New("method must be 'GET' or 'PUT'")
	ErrPresignedExpiresInvalid = errors.New("expires_in must be between 1 and 604800 seconds (7 days)")
	ErrObjectNameRequired      = errors.New("name (object key) is required")
)

// Presigned URL expiration limits (1 second to 7 days).
const (
	minExpiresIn = 1
	maxExpiresIn = 604800
)

func validatePresignedMethod(method string) error {
	upper := strings.ToUpper(method)
	if upper != "GET" && upper != "PUT" {
		return fmt.Errorf("got '%s': %w", method, ErrPresignedMethodInvalid)
	}

	return nil
}

func validateExpiresIn(expiresIn int) error {
	if expiresIn < minExpiresIn || expiresIn > maxExpiresIn {
		return fmt.Errorf("got %d: %w", expiresIn, ErrPresignedExpiresInvalid)
	}

	return nil
}

// validateKeyLabel checks that an Object Storage access key label is present and within length limits.
func validateKeyLabel(label string) error {
	if label == "" {
		return ErrKeyLabelRequired
	}

	if len(label) > maxKeyLabelLength {
		return ErrKeyLabelTooLong
	}

	return nil
}

// validateKeyPermissions rejects any permission value other than read_only or read_write.
func validateKeyPermissions(permissions string) error {
	if permissions != "read_only" && permissions != "read_write" {
		return fmt.Errorf("got '%s': %w", permissions, ErrKeyPermissionsInvalid)
	}

	return nil
}
