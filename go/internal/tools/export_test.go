package tools

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// ExportedSelectEnvironment exposes selectEnvironment for testing.
func ExportedSelectEnvironment(cfg *config.Config, environment string) (*config.EnvironmentConfig, error) {
	return selectEnvironment(cfg, environment)
}

// ExportedValidateLinodeConfig exposes validateLinodeConfig for testing.
func ExportedValidateLinodeConfig(env *config.EnvironmentConfig) error {
	return validateLinodeConfig(env)
}

// ExportedFilterInstancesByStatus exposes the instance status filter for testing.
func ExportedFilterInstancesByStatus(instances []linode.Instance, statusFilter string) []linode.Instance {
	return filterByField(instances, statusFilter, func(inst linode.Instance) string {
		return inst.Status
	})
}

// ExportedFormatInstancesResponse exposes formatInstancesResponse for testing.
func ExportedFormatInstancesResponse(instances []linode.Instance, statusFilter string) (*mcp.CallToolResult, error) {
	return formatInstancesResponse(instances, statusFilter)
}

// ExportedValidateBucketLabel exposes validateBucketLabel for testing.
func ExportedValidateBucketLabel(label string) error {
	return validateBucketLabel(label)
}

// ExportedValidateDNSRecordTarget exposes validateDNSRecordTarget for testing.
func ExportedValidateDNSRecordTarget(recordType, target string) error {
	return validateDNSRecordTarget(recordType, target)
}

// ExportedValidateDNSRecordName exposes validateDNSRecordName for testing.
func ExportedValidateDNSRecordName(name string) error {
	return validateDNSRecordName(name)
}
