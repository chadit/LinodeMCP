package server_test

import (
	"testing"
)

func TestObjectStorageClusterListToolNotRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)
	infos := srv.ToolInfos()
	requireNotEmpty(t, infos, "server must expose registered tools")

	for _, info := range infos {
		assertNotEqual(t, "linode_object_storage_cluster_list", info.Name, "deprecated Object Storage clusters tool must not be registered")
	}

	for _, tool := range srv.Tools() {
		assertNotEqual(t, "linode_object_storage_cluster_list", tool.Name(), "deprecated Object Storage clusters tool must not be dispatchable")
	}
}
