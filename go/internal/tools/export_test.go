package tools

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// SelectEnvironment exposes selectEnvironment for testing.
func SelectEnvironment(cfg *config.Config, environment string) (*config.EnvironmentConfig, error) {
	return selectEnvironment(cfg, environment)
}

// ValidateLinodeConfig exposes validateLinodeConfig for testing.
func ValidateLinodeConfig(env *config.EnvironmentConfig) error {
	return validateLinodeConfig(env)
}

// FilterInstancesByStatus exposes filterInstancesByStatus for testing.
func FilterInstancesByStatus(instances []linode.Instance, statusFilter string) []linode.Instance {
	return filterInstancesByStatus(instances, statusFilter)
}

// FormatInstancesResponse exposes formatInstancesResponse for testing.
func FormatInstancesResponse(instances []linode.Instance, statusFilter string) (*mcp.CallToolResult, error) {
	return formatInstancesResponse(instances, statusFilter)
}

// ValidateBucketLabel exposes validateBucketLabel for testing.
func ValidateBucketLabel(label string) error {
	return validateBucketLabel(label)
}

// ValidateDNSRecordTarget exposes validateDNSRecordTarget for testing.
func ValidateDNSRecordTarget(recordType, target string) error {
	return validateDNSRecordTarget(recordType, target)
}

// ValidateDNSRecordName exposes validateDNSRecordName for testing.
func ValidateDNSRecordName(name string) error {
	return validateDNSRecordName(name)
}
