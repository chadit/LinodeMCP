package tools

import (
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

// validateSSHKey checks that the key is non-empty, starts with a recognized algorithm prefix
// (ssh-rsa, ssh-ed25519, ecdsa-sha2-*, ssh-dss), and falls within the 80-16000 character length range.
func validateSSHKey(key string) error {
	if key == "" {
		return ErrSSHKeyRequired
	}

	key = strings.TrimSpace(key)

	var validPrefix bool

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

// validateDNSRecordName checks that the DNS record name does not exceed 253 characters
// and contains only alphanumeric characters, hyphens, and dots per RFC 1035. Empty and "@" are allowed.
func validateDNSRecordName(name string) error {
	if len(name) > maxDNSNameLength {
		return ErrDNSNameTooLong
	}

	if name != "" && name != "@" && !validDNSNameRegex.MatchString(name) {
		return ErrDNSNameInvalid
	}

	return nil
}

// validateDNSRecordTarget checks that the target is non-empty and valid for the given record type.
// A records require a public IPv4 address, AAAA records require an IPv6 address,
// and CNAME/NS/MX records require a valid hostname.
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

// Presigned URL expiration limits (1 second to 7 days).
const (
	minExpiresIn = 1
	maxExpiresIn = 604800
)

// validatePresignedMethod checks that the HTTP method is GET or PUT (case-insensitive),
// which are the only methods supported for S3-compatible presigned URLs.
func validatePresignedMethod(method string) error {
	upper := strings.ToUpper(method)
	if upper != "GET" && upper != "PUT" {
		return fmt.Errorf("got '%s': %w", method, ErrPresignedMethodInvalid)
	}

	return nil
}

// validateExpiresIn checks that the presigned URL expiration is between 1 and 604800 seconds (7 days).
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
