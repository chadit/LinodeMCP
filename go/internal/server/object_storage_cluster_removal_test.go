package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectStorageClusterListToolNotRegistered(t *testing.T) {
	t.Parallel()

	srv := newCapabilityTestServer(t)
	infos := srv.ToolInfos()
	require.NotEmpty(t, infos, "server must expose registered tools")

	for _, info := range infos {
		assert.NotEqual(t, "linode_object_storage_cluster_list", info.Name, "deprecated Object Storage clusters tool must not be registered")
	}

	for _, tool := range srv.Tools() {
		assert.NotEqual(t, "linode_object_storage_cluster_list", tool.Name(), "deprecated Object Storage clusters tool must not be dispatchable")
	}
}
