package appinfo_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/appinfo"
)

func TestGet_ReturnsCorrectVersionInfo(t *testing.T) {
	t.Parallel()

	info := appinfo.Get()

	assert.Equal(t, appinfo.Version, info.Version, "Version should match the constant.")
	assert.Equal(t, appinfo.APIVersion, info.APIVersion, "APIVersion should match the constant.")
	assert.Equal(t, runtime.Version(), info.GoVersion, "GoVersion should match runtime.")

	expectedPlatform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	assert.Equal(t, expectedPlatform, info.Platform, "Platform should match runtime GOOS/GOARCH.")
}

func TestGet_BuildDateDefault(t *testing.T) {
	t.Parallel()

	info := appinfo.Get()
	assert.Equal(t, "unknown", info.BuildDate, "BuildDate should be 'unknown' by default.")
}

func TestGet_GitMetadataDefaults(t *testing.T) {
	t.Parallel()

	info := appinfo.Get()
	assert.Equal(t, "dev", info.GitCommit, "GitCommit should default to 'dev'.")
	assert.Equal(t, "main", info.GitBranch, "GitBranch should default to 'main'.")
}

func TestGet_FeaturesPopulated(t *testing.T) {
	t.Parallel()

	info := appinfo.Get()

	require.NotNil(t, info.Features, "Features map should not be nil.")

	expectedTools := "hello,version,linode_profile,linode_account,linode_instances_list,linode_instance_get,linode_regions_list,linode_types_list,linode_volumes_list,linode_images_list,linode_sshkeys_list,linode_domains_list,linode_domain_get,linode_domain_records_list,linode_firewalls_list,linode_nodebalancers_list,linode_nodebalancer_get,linode_stackscripts_list,linode_sshkey_create,linode_sshkey_delete,linode_instance_boot,linode_instance_reboot,linode_instance_shutdown,linode_instance_create,linode_instance_delete,linode_instance_resize,linode_firewall_create,linode_firewall_update,linode_firewall_delete,linode_domain_create,linode_domain_update,linode_domain_delete,linode_domain_record_create,linode_domain_record_update,linode_domain_record_delete,linode_volume_create,linode_volume_attach,linode_volume_detach,linode_volume_resize,linode_volume_delete,linode_nodebalancer_create,linode_nodebalancer_update,linode_nodebalancer_delete,linode_object_storage_buckets_list,linode_object_storage_bucket_get,linode_object_storage_bucket_contents,linode_object_storage_clusters_list,linode_object_storage_type_list,linode_object_storage_keys_list,linode_object_storage_key_get,linode_object_storage_transfer,linode_object_storage_bucket_access_get,linode_object_storage_bucket_create,linode_object_storage_bucket_delete,linode_object_storage_bucket_access_update,linode_object_storage_key_create,linode_object_storage_key_update,linode_object_storage_key_delete,linode_object_storage_presigned_url,linode_object_storage_object_acl_get,linode_object_storage_object_acl_update,linode_object_storage_ssl_get,linode_object_storage_ssl_delete"
	assert.Equal(t, expectedTools, info.Features["tools"])
	assert.Equal(t, "basic", info.Features["logging"])
	assert.Equal(t, "mcp", info.Features["protocol"])
}

func TestInfo_String(t *testing.T) {
	t.Parallel()

	info := appinfo.Get()
	s := info.String()

	assert.True(t, strings.HasPrefix(s, "LinodeMCP v"), "String should start with 'LinodeMCP v'.")
	assert.Contains(t, s, info.Version, "String should contain the appinfo.")
	assert.Contains(t, s, info.APIVersion, "String should contain the API appinfo.")
	assert.Contains(t, s, info.Platform, "String should contain the platform.")
}
