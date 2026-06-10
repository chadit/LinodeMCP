# Deprecated route removals

## Object Storage cluster get

- Removed tool: `linode_object_storage_cluster_get`
- Removed route: `GET /v4/object-storage/clusters/{cluster_id}`
- Replacement: `GET /v4/regions/{region_id}` through the existing `linode_region_get` tool.
