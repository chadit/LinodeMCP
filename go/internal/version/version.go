// Package version provides build-time version information for LinodeMCP.
package version

import (
	"fmt"
	"runtime"
)

// Version is the current LinodeMCP version.
const Version = "0.1.0"

// APIVersion is the current MCP API version.
const APIVersion = "0.1.0"

const (
	defaultBuildDate = "unknown"

	featureKeyTools    = "tools"
	featureKeyLogging  = "logging"
	featureKeyProtocol = "protocol"

	featureToolsList = "hello,version,linode_profile,linode_account,linode_instances_list,linode_instance_get,linode_regions_list,linode_types_list,linode_volumes_list,linode_images_list,linode_sshkeys_list,linode_domains_list,linode_domain_get,linode_domain_records_list,linode_firewalls_list,linode_nodebalancers_list,linode_nodebalancer_get,linode_stackscripts_list,linode_sshkey_create,linode_sshkey_delete,linode_instance_boot,linode_instance_reboot,linode_instance_shutdown,linode_instance_create,linode_instance_delete,linode_instance_resize,linode_firewall_create,linode_firewall_update,linode_firewall_delete,linode_domain_create,linode_domain_update,linode_domain_delete,linode_domain_record_create,linode_domain_record_update,linode_domain_record_delete,linode_volume_create,linode_volume_attach,linode_volume_detach,linode_volume_resize,linode_volume_delete,linode_nodebalancer_create,linode_nodebalancer_update,linode_nodebalancer_delete,linode_object_storage_buckets_list,linode_object_storage_bucket_get,linode_object_storage_bucket_contents,linode_object_storage_clusters_list,linode_object_storage_type_list,linode_object_storage_keys_list,linode_object_storage_key_get,linode_object_storage_transfer,linode_object_storage_bucket_access_get,linode_object_storage_bucket_create,linode_object_storage_bucket_delete,linode_object_storage_bucket_access_update,linode_object_storage_key_create,linode_object_storage_key_update,linode_object_storage_key_delete,linode_object_storage_presigned_url,linode_object_storage_object_acl_get,linode_object_storage_object_acl_update,linode_object_storage_ssl_get,linode_object_storage_ssl_delete"
	featureLogging   = "basic"
	featureProtocol  = "mcp"
)

// Build-time variables set via ldflags.
var (
	BuildDate = ""     //nolint:gochecknoglobals // Build-time variable set via ldflags
	GitCommit = "dev"  //nolint:gochecknoglobals // Build-time variable set via ldflags
	GitBranch = "main" //nolint:gochecknoglobals // Build-time variable set via ldflags
)

// Info holds build and version metadata for the LinodeMCP server.
type Info struct {
	Version    string            `json:"version"`
	APIVersion string            `json:"api_version"` //nolint:tagliatelle // snake_case for JSON compatibility
	BuildDate  string            `json:"build_date"`  //nolint:tagliatelle // snake_case for JSON compatibility
	GitCommit  string            `json:"git_commit"`  //nolint:tagliatelle // snake_case for JSON compatibility
	GitBranch  string            `json:"git_branch"`  //nolint:tagliatelle // snake_case for JSON compatibility
	GoVersion  string            `json:"go_version"`  //nolint:tagliatelle // snake_case for JSON compatibility
	Platform   string            `json:"platform"`
	Features   map[string]string `json:"features"`
}

// Get returns the current version information.
func Get() Info {
	buildDate := BuildDate
	if buildDate == "" {
		buildDate = defaultBuildDate
	}

	return Info{
		Version:    Version,
		APIVersion: APIVersion,
		BuildDate:  buildDate,
		GitCommit:  GitCommit,
		GitBranch:  GitBranch,
		GoVersion:  runtime.Version(),
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		Features: map[string]string{
			featureKeyTools:    featureToolsList,
			featureKeyLogging:  featureLogging,
			featureKeyProtocol: featureProtocol,
		},
	}
}

// String returns a human-readable version string.
func (i Info) String() string {
	return fmt.Sprintf("LinodeMCP v%s (MCP: v%s, %s, %s)",
		i.Version, i.APIVersion, i.Platform, i.GitCommit)
}
