package server_test

import (
	"testing"
)

func TestObjectStorageClusterListToolNotRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)

	infos := srv.ToolInfos()
	if len(infos) == 0 {
		t.Fatal("infos is empty")
	}

	for _, info := range infos {
		if info.Name == "linode_object_storage_cluster_list" {
			t.Errorf("info.Name = %v, do not want %v", info.Name, "linode_object_storage_cluster_list")
		}
	}

	for _, tool := range srv.Tools() {
		if tool.Name() == "linode_object_storage_cluster_list" {
			t.Errorf("tool.Name() = %v, do not want %v", tool.Name(), "linode_object_storage_cluster_list")
		}
	}
}
