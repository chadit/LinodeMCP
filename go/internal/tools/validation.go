package tools

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"unicode"
)

// Validation constants.
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

// SSH key validation errors.
var (
	ErrSSHKeyRequired      = errors.New("ssh_key is required")
	ErrSSHKeyInvalidFormat = errors.New("invalid SSH key format: must start with ssh-rsa, ssh-ed25519, or ecdsa-sha2-*")
	ErrSSHKeyInvalidLength = errors.New("invalid SSH key length: key appears malformed")
)

// Password validation errors.
var (
	ErrPasswordTooShort    = errors.New("root_pass must be at least 12 characters")
	ErrPasswordTooLong     = errors.New("root_pass must not exceed 128 characters")
	ErrPasswordMissingChar = errors.New("root_pass must contain uppercase, lowercase, and digits")
)

// DNS validation errors.
var (
	ErrDNSNameTooLong       = errors.New("DNS record name exceeds maximum length of 253 characters")
	ErrDNSNameInvalid       = errors.New("invalid DNS record name: must contain only alphanumeric characters, hyphens, and dots")
	ErrDNSTargetRequired    = errors.New("target is required")
	ErrDNSTargetInvalidA    = errors.New("a record target must be a valid IPv4 address")
	ErrDNSTargetPrivateIP   = errors.New("a record target cannot be a private IP address")
	ErrDNSTargetInvalidAAAA = errors.New("aaaa record target must be a valid IPv6 address")
)

// Firewall validation errors.
var (
	ErrFirewallPolicyInvalid = errors.New("firewall policy must be 'ACCEPT' or 'DROP'")
)

// Volume validation errors.
var (
	ErrVolumeSizeTooSmall = errors.New("volume size must be at least 10 GB")
	ErrVolumeSizeTooLarge = errors.New("volume size cannot exceed 10240 GB (10 TB)")
)

// SSH key validation.
//
//nolint:gochecknoglobals // Read-only slice of valid SSH key prefixes.
var validSSHKeyPrefixes = []string{
	"ssh-rsa",
	"ssh-ed25519",
	"ecdsa-sha2-nistp256",
	"ecdsa-sha2-nistp384",
	"ecdsa-sha2-nistp521",
	"ssh-dss",
}

func validateSSHKey(key string) error {
	if key == "" {
		return ErrSSHKeyRequired
	}

	key = strings.TrimSpace(key)
	validPrefix := false

	for _, prefix := range validSSHKeyPrefixes {
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

// Password validation.
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

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit {
		return ErrPasswordMissingChar
	}

	return nil
}

// DNS record validation.
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

// Firewall policy validation.
func validateFirewallPolicy(policy string) error {
	upper := strings.ToUpper(policy)
	if upper != "ACCEPT" && upper != "DROP" {
		return fmt.Errorf("got '%s': %w", policy, ErrFirewallPolicyInvalid)
	}

	return nil
}

// Object Storage bucket label validation errors.
var (
	ErrBucketLabelRequired  = errors.New("label is required")
	ErrBucketLabelTooShort  = errors.New("bucket label must be at least 3 characters")
	ErrBucketLabelTooLong   = errors.New("bucket label must not exceed 63 characters")
	ErrBucketLabelStartEnd  = errors.New("bucket label must start and end with a lowercase letter or number")
	ErrBucketLabelInvalid   = errors.New("bucket label must contain only lowercase letters, numbers, and hyphens")
	ErrBucketACLInvalid     = errors.New("acl must be one of: private, public-read, authenticated-read, public-read-write")
	ErrBucketRegionRequired = errors.New("region is required")
)

// Object Storage bucket label validation (S3 naming rules).
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

// Object Storage ACL validation.
func validateBucketACL(acl string) error {
	switch acl {
	case "private", "public-read", "authenticated-read", "public-read-write":
		return nil
	default:
		return fmt.Errorf("got '%s': %w", acl, ErrBucketACLInvalid)
	}
}

// Volume size validation.
func validateVolumeSize(size int) error {
	if size < minVolumeSizeGB {
		return ErrVolumeSizeTooSmall
	}

	if size > maxVolumeSizeGB {
		return ErrVolumeSizeTooLarge
	}

	return nil
}

// Object Storage access key validation errors.
var (
	ErrKeyLabelRequired        = errors.New("label is required")
	ErrKeyLabelTooLong         = errors.New("access key label must not exceed 50 characters")
	ErrKeyIDRequired           = errors.New("key_id is required and must be a positive integer")
	ErrKeyPermissionsInvalid   = errors.New("bucket_access permissions must be 'read_only' or 'read_write'")
	ErrKeyBucketNameRequired   = errors.New("bucket_access entries must include bucket_name")
	ErrKeyBucketRegionRequired = errors.New("bucket_access entries must include region")
)

// Presigned URL validation errors.
var (
	ErrPresignedMethodInvalid  = errors.New("method must be 'GET' or 'PUT'")
	ErrPresignedExpiresInvalid = errors.New("expires_in must be between 1 and 604800 seconds (7 days)")
	ErrObjectNameRequired      = errors.New("name (object key) is required")
)

// Presigned URL validation constants.
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

// Object Storage access key label validation.
func validateKeyLabel(label string) error {
	if label == "" {
		return ErrKeyLabelRequired
	}

	if len(label) > maxKeyLabelLength {
		return ErrKeyLabelTooLong
	}

	return nil
}

// Object Storage access key permissions validation.
func validateKeyPermissions(permissions string) error {
	if permissions != "read_only" && permissions != "read_write" {
		return fmt.Errorf("got '%s': %w", permissions, ErrKeyPermissionsInvalid)
	}

	return nil
}
