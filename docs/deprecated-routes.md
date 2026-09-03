# Deprecated route removals

Removal log for tools and API routes dropped from the surface, each with the
replacement to use instead. One section per removal.

## Object Storage cluster get

- Removed tool: `linode_object_storage_cluster_get`
- Removed route: `GET /v4/object-storage/clusters/{cluster_id}`
- Replacement: `GET /v4/regions/{region_id}` through the existing `linode_region_get` tool.

## Object Storage presigned-URL create

- Removed tool: `linode_object_storage_presigned_url_create`
- Removed route: none. `POST /v4/object-storage/buckets/{region}/{label}/object-url` stays.
- Replacement: `linode_object_storage_object_download_url_create` for a URL signed
  for GET, and `linode_object_storage_object_upload_url_create` for one signed for
  PUT. Neither takes a `method` argument: each pins its own verb, which is what
  lets the GET half stay Read capability while the PUT half takes the confirm
  gate a write credential needs. A caller that was passing `method: "PUT"` from a
  read-only profile now needs a profile that permits writes.
