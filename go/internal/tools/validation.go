package tools

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Validation constants.
const (
	minPasswordLength = 12
	maxPasswordLength = 128
	maxDNSNameLength  = 253
	minVolumeSizeGB   = 10
	maxVolumeSizeGB   = 10240
	minSSHKeyLength   = 80
	maxSSHKeyLength   = 16000
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
	validDNSNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$|^@$|^$`)
	validIPv4Regex    = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	validIPv6Regex    = regexp.MustCompile(`^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$`)
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
		if !validIPv4Regex.MatchString(target) {
			return ErrDNSTargetInvalidA
		}
		// Check for private IP ranges.
		if strings.HasPrefix(target, "10.") ||
			strings.HasPrefix(target, "192.168.") ||
			strings.HasPrefix(target, "127.") {
			return ErrDNSTargetPrivateIP
		}
	case "AAAA":
		if !validIPv6Regex.MatchString(target) {
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
