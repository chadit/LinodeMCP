package tools

import (
	"fmt"
	"net"
	"regexp"
	"slices"
	"strings"
	"unicode"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
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
	validRegionSlugRegex  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
)

// isPrivateIPv4 reports whether addr falls in a private or reserved range that
// CPython's ipaddress.is_private rejects, minus its two public exceptions inside
// 192.0.0.0/24. Python's DNS A-record validator rejects the same set, so Go
// mirrors it to keep both languages rejecting identically. net.IP.IsPrivate
// covers only RFC 1918, which is why documentation and reserved targets used to
// slip through Go. The list is the IANA special-purpose-address registry: RFC
// 1918 plus loopback, link-local, documentation (RFC 5737), benchmarking, and
// reserved blocks. Domain-record validation is a cold path, so the CIDRs are
// parsed per call rather than held in package-level state.
func isPrivateIPv4(addr net.IP) bool {
	// 192.0.0.9 and 192.0.0.10 sit inside 192.0.0.0/24 but are globally routable
	// (PCP anycast, NAT64/DNS64 discovery), so they stay valid A-record targets.
	exceptions := []string{"192.0.0.9", "192.0.0.10"}
	if slices.ContainsFunc(exceptions, func(exc string) bool { return addr.Equal(net.ParseIP(exc)) }) {
		return false
	}

	cidrs := []string{
		"0.0.0.0/8", "10.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16",
		"172.16.0.0/12", "192.0.0.0/24", "192.0.0.170/31", "192.0.2.0/24",
		"192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
		"240.0.0.0/4", "255.255.255.255/32",
	}

	return slices.ContainsFunc(cidrs, func(cidr string) bool {
		_, network, err := net.ParseCIDR(cidr)

		return err == nil && network.Contains(addr)
	})
}

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

		if isPrivateIPv4(ip) {
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

// validateRegionSlug checks that a region path parameter is a single slug segment.
func validateRegionSlug(region string) error {
	if region == "" {
		return ErrBucketRegionRequired
	}

	if !validRegionSlugRegex.MatchString(region) {
		return fmt.Errorf("got '%s': %w", region, ErrRegionInvalid)
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
		// Return the sentinel unwrapped so the emitted string matches Python
		// byte-for-byte (no "got '<value>':" prefix), pinned by the shared
		// objstorage behavior fixtures.
		return ErrBucketACLInvalid
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

// validateExpiresIn checks that the presigned URL expiration is between 1 and 604800 seconds (7 days).
func validateExpiresIn(expiresIn int) error {
	if expiresIn < minExpiresIn || expiresIn > maxExpiresIn {
		// Suffix form ", got N" matches Python's message byte-for-byte (the
		// same reconcile the bucket-ACL message went through).
		return fmt.Errorf("%w, got %d", ErrPresignedExpiresInvalid, expiresIn)
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

// keyPermissionsError returns "" when a bucket_access permission is a valid
// ObjectStorageKeyPermission value, else the enum-sourced rejection. Each entry
// must carry a permission, so an empty value is rejected too (unlike the
// optional-field helper). The value set and message order come from the proto
// enum, so both languages agree.
func keyPermissionsError(permissions string) string {
	if _, ok := linodev1.ObjectStorageKeyPermission_Value_value[permissions]; ok && permissions != enumSentinel {
		return ""
	}

	return "permissions must be one of: " + strings.Join(enumValueNames(linodev1.ObjectStorageKeyPermission_Value_value), ", ")
}

// grantPermissionValid reports whether an account-user grant level is empty (no
// access) or a valid GrantPermission enum value. Empty is allowed because
// clearing a grant means the user has no access on that resource; the allowed
// non-empty set comes from the proto enum so it stays in sync with the schema.
func grantPermissionValid(permission string) bool {
	if permission == "" {
		return true
	}

	_, ok := linodev1.GrantPermission_Value_value[permission]

	return ok && permission != enumSentinel
}
