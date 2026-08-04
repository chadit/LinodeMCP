package linode

import (
	"context"
	"encoding/json"
	"net/url"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// Facts shared by the endpoints below, so the per-method docs do not repeat them:
// list endpoints return the standard {data, page, ...} envelope that the
// listProtoElements helpers read; only key create returns secret material, so
// secret_key decodes to its empty default everywhere else; byte counts are int64,
// which protojson serializes as JSON strings.

// httpListObjectStorageBucketsProto retrieves all Object Storage buckets as
// proto messages.
func (c *Client) httpListObjectStorageBucketsProto(ctx context.Context) ([]*linodev1.ObjectStorageBucket, error) {
	return listProtoElementsRouted(ctx, c, "ListObjectStorageBuckets",
		"linode_object_storage_bucket_list", "", nil,
		func() *linodev1.ObjectStorageBucket { return &linodev1.ObjectStorageBucket{} })
}

// httpListObjectStorageBucketsByRegionProto retrieves Object Storage buckets in
// one region as proto messages.
func (c *Client) httpListObjectStorageBucketsByRegionProto(ctx context.Context, region string) ([]*linodev1.ObjectStorageBucket, error) {
	return listProtoElementsRouted(ctx, c, "ListObjectStorageBucketsByRegion",
		"linode_object_storage_bucket_by_region_list", "", []any{region},
		func() *linodev1.ObjectStorageBucket { return &linodev1.ObjectStorageBucket{} })
}

// GetObjectStorageBucket retrieves a specific Object Storage bucket.
func (c *Client) httpGetObjectStorageBucket(ctx context.Context, region, label string) (*ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_bucket_get", nil, region, label)
	if err != nil {
		return nil, wrapRequestError("GetObjectStorageBucket", err)
	}

	defer drainClose(resp)

	var bucket ObjectStorageBucket
	if err := c.handleResponse(resp, &bucket); err != nil {
		return nil, err
	}

	return &bucket, nil
}

// httpGetObjectStorageBucketProto retrieves an Object Storage bucket as a proto
// message.
func (c *Client) httpGetObjectStorageBucketProto(ctx context.Context, region, label string) (*linodev1.ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_bucket_get", nil, region, label)
	if err != nil {
		return nil, wrapRequestError("GetObjectStorageBucket", err)
	}

	defer drainClose(resp)

	bucket := &linodev1.ObjectStorageBucket{}
	if err := c.handleProtoResponse(resp, bucket); err != nil {
		return nil, err
	}

	return bucket, nil
}

// ObjectStorageBucketContentsPage is the decoded body of the S3-style
// object-list endpoint: the object elements plus the marker pagination metadata
// the standard page envelope does not carry.
type ObjectStorageBucketContentsPage struct {
	NextMarker  string
	Objects     []*linodev1.ObjectStorageObject
	IsTruncated bool
}

// httpListObjectStorageBucketContentsProto lists objects in a bucket as proto
// messages. This endpoint pages by marker, not by page number: the body is a
// bespoke {data, is_truncated, next_marker} shape, so the elements are decoded
// here and the truncation metadata is returned alongside them.
func (c *Client) httpListObjectStorageBucketContentsProto(ctx context.Context, region, label string, params map[string]string) (*ObjectStorageBucketContentsPage, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var rawQuery string

	if len(params) > 0 {
		vals := url.Values{}
		for k, v := range params {
			vals.Set(k, v)
		}

		rawQuery = vals.Encode()
	}

	resp, err := c.makeRouteRequestQuery(ctx, "linode_object_storage_bucket_object_list", rawQuery, nil, region, label)
	if err != nil {
		return nil, wrapRequestError("ListObjectStorageBucketContents", err)
	}

	defer drainClose(resp)

	var envelope struct {
		NextMarker  string            `json:"next_marker"`
		Data        []json.RawMessage `json:"data"`
		IsTruncated bool              `json:"is_truncated"`
	}

	if decodeErr := c.handleResponse(resp, &envelope); decodeErr != nil {
		return nil, decodeErr
	}

	objects, err := decodeRawProtoItems(envelope.Data, "ListObjectStorageBucketContents",
		func() *linodev1.ObjectStorageObject { return &linodev1.ObjectStorageObject{} })
	if err != nil {
		return nil, err
	}

	return &ObjectStorageBucketContentsPage{
		Objects:     objects,
		IsTruncated: envelope.IsTruncated,
		NextMarker:  envelope.NextMarker,
	}, nil
}

// httpListObjectStorageEndpointsProto retrieves Object Storage endpoints as
// proto messages. This is the one Object Storage list that takes page numbers.
func (c *Client) httpListObjectStorageEndpointsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ObjectStorageEndpoint, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListObjectStorageEndpoints",
		"linode_object_storage_endpoint_list", "", nil, page, pageSize,
		func() *linodev1.ObjectStorageEndpoint { return &linodev1.ObjectStorageEndpoint{} })
}

// httpListObjectStorageTypesProto retrieves Object Storage types and pricing.
// The elements share the LinodeType shape, so region_prices decodes as a
// repeated message, not a string.
func (c *Client) httpListObjectStorageTypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	return listProtoElementsRouted(ctx, c, "ListObjectStorageTypes",
		"linode_object_storage_type_list", "", nil,
		func() *linodev1.LinodeType { return &linodev1.LinodeType{} })
}

// httpListObjectStorageQuotasProto retrieves Object Storage quotas as proto
// messages.
func (c *Client) httpListObjectStorageQuotasProto(ctx context.Context) ([]*linodev1.ObjectStorageQuota, error) {
	return listProtoElementsRouted(ctx, c, "ListObjectStorageQuotas",
		"linode_object_storage_quota_list", "", nil,
		func() *linodev1.ObjectStorageQuota { return &linodev1.ObjectStorageQuota{} })
}

// httpListObjectStorageKeysProto retrieves Object Storage keys as proto
// messages.
func (c *Client) httpListObjectStorageKeysProto(ctx context.Context) ([]*linodev1.ObjectStorageKey, error) {
	return listProtoElementsRouted(ctx, c, "ListObjectStorageKeys",
		"linode_object_storage_key_list", "", nil,
		func() *linodev1.ObjectStorageKey { return &linodev1.ObjectStorageKey{} })
}

// GetObjectStorageKey retrieves a specific Object Storage access key by ID.
func (c *Client) httpGetObjectStorageKey(ctx context.Context, keyID int) (*ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_key_get", nil, keyID)
	if err != nil {
		return nil, wrapRequestError("GetObjectStorageKey", err)
	}

	defer drainClose(resp)

	var key ObjectStorageKey
	if err := c.handleResponse(resp, &key); err != nil {
		return nil, err
	}

	return &key, nil
}

// httpGetObjectStorageKeyProto retrieves an Object Storage key as a proto message.
func (c *Client) httpGetObjectStorageKeyProto(ctx context.Context, keyID int) (*linodev1.ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_key_get", nil, keyID)
	if err != nil {
		return nil, wrapRequestError("GetObjectStorageKey", err)
	}

	defer drainClose(resp)

	key := &linodev1.ObjectStorageKey{}
	if err := c.handleProtoResponse(resp, key); err != nil {
		return nil, err
	}

	return key, nil
}

// httpGetObjectStorageQuotaUsageProto retrieves usage for one Object Storage
// quota. Usage is optional and omitted when the API returns null, which it does
// until the first usage is recorded.
func (c *Client) httpGetObjectStorageQuotaUsageProto(ctx context.Context, quotaID string) (*linodev1.ObjectStorageQuotaUsage, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_quota_usage_get", nil, quotaID)
	if err != nil {
		return nil, wrapRequestError("GetObjectStorageQuotaUsage", err)
	}

	defer drainClose(resp)

	usage := &linodev1.ObjectStorageQuotaUsage{}
	if err := c.handleProtoResponse(resp, usage); err != nil {
		return nil, err
	}

	return usage, nil
}

// httpGetObjectStorageTransferProto retrieves outbound data transfer usage.
func (c *Client) httpGetObjectStorageTransferProto(ctx context.Context) (*linodev1.ObjectStorageTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_transfer_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetObjectStorageTransfer", err)
	}

	defer drainClose(resp)

	transfer := &linodev1.ObjectStorageTransfer{}
	if err := c.handleProtoResponse(resp, transfer); err != nil {
		return nil, err
	}

	return transfer, nil
}

// httpGetObjectStorageQuotaProto retrieves a single Object Storage quota. The
// body carries the bare quota, since usage lives behind its own endpoint.
func (c *Client) httpGetObjectStorageQuotaProto(ctx context.Context, objQuotaID string) (*linodev1.ObjectStorageQuota, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_quota_get", nil, objQuotaID)
	if err != nil {
		return nil, wrapRequestError("GetObjectStorageQuota", err)
	}

	defer drainClose(resp)

	quota := &linodev1.ObjectStorageQuota{}
	if err := c.handleProtoResponse(resp, quota); err != nil {
		return nil, err
	}

	return quota, nil
}

// CancelObjectStorage cancels Object Storage service for the account.
func (c *Client) httpCancelObjectStorage(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_cancel", nil)
	if err != nil {
		return wrapRequestError("CancelObjectStorage", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// GetObjectStorageBucketAccess retrieves ACL and CORS settings for a bucket.
func (c *Client) httpGetObjectStorageBucketAccess(ctx context.Context, region, label string) (*ObjectStorageBucketAccess, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_bucket_access_get", nil, region, label)
	if err != nil {
		return nil, wrapRequestError("GetObjectStorageBucketAccess", err)
	}

	defer drainClose(resp)

	var access ObjectStorageBucketAccess
	if err := c.handleResponse(resp, &access); err != nil {
		return nil, err
	}

	return &access, nil
}

// httpGetObjectStorageBucketAccessProto retrieves a bucket's access config as a
// proto message.
func (c *Client) httpGetObjectStorageBucketAccessProto(ctx context.Context, region, label string) (*linodev1.ObjectStorageBucketAccess, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_bucket_access_get", nil, region, label)
	if err != nil {
		return nil, wrapRequestError("GetObjectStorageBucketAccess", err)
	}

	defer drainClose(resp)

	access := &linodev1.ObjectStorageBucketAccess{}
	if err := c.handleProtoResponse(resp, access); err != nil {
		return nil, err
	}

	return access, nil
}

// httpCreateObjectStorageBucketProto creates an Object Storage bucket as a proto
// message.
func (c *Client) httpCreateObjectStorageBucketProto(ctx context.Context, req *CreateObjectStorageBucketRequest) (*linodev1.ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_bucket_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateObjectStorageBucket", err)
	}

	defer drainClose(resp)

	bucket := &linodev1.ObjectStorageBucket{}
	if err := c.handleProtoResponse(resp, bucket); err != nil {
		return nil, err
	}

	return bucket, nil
}

// DeleteObjectStorageBucket deletes an Object Storage bucket.
func (c *Client) httpDeleteObjectStorageBucket(ctx context.Context, region, label string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_bucket_delete", nil, region, label)
	if err != nil {
		return wrapRequestError("DeleteObjectStorageBucket", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// UpdateObjectStorageBucketAccess updates bucket ACL and CORS settings.
func (c *Client) httpUpdateObjectStorageBucketAccess(ctx context.Context, region, label string, req UpdateObjectStorageBucketAccessRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_bucket_access_update", req, region, label)
	if err != nil {
		return wrapRequestError("UpdateObjectStorageBucketAccess", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// AllowObjectStorageBucketAccess applies bucket ACL and CORS settings.
func (c *Client) httpAllowObjectStorageBucketAccess(ctx context.Context, region, label string, req AllowObjectStorageBucketAccessRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_bucket_access_allow", req, region, label)
	if err != nil {
		return wrapRequestError("AllowObjectStorageBucketAccess", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCreateObjectStorageKeyProto creates an Object Storage key as a proto message.
func (c *Client) httpCreateObjectStorageKeyProto(ctx context.Context, req CreateObjectStorageKeyRequest) (*linodev1.ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_key_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateObjectStorageKey", err)
	}

	defer drainClose(resp)

	key := &linodev1.ObjectStorageKey{}
	if err := c.handleProtoResponse(resp, key); err != nil {
		return nil, err
	}

	return key, nil
}

// httpUpdateObjectStorageKeyProto updates a key and decodes the echoed key as a
// proto message. The update endpoint returns the full key without secret
// material, so the element's secret_key serializes as its empty default.
func (c *Client) httpUpdateObjectStorageKeyProto(ctx context.Context, keyID int, req UpdateObjectStorageKeyRequest) (*linodev1.ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_key_update", req, keyID)
	if err != nil {
		return nil, wrapRequestError("UpdateObjectStorageKey", err)
	}

	defer drainClose(resp)

	key := &linodev1.ObjectStorageKey{}
	if err := c.handleProtoResponse(resp, key); err != nil {
		return nil, err
	}

	return key, nil
}

// DeleteObjectStorageKey revokes an Object Storage access key.
func (c *Client) httpDeleteObjectStorageKey(ctx context.Context, keyID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_key_delete", nil, keyID)
	if err != nil {
		return wrapRequestError("DeleteObjectStorageKey", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCreatePresignedURLProto generates a presigned URL for an object in Object
// Storage and decodes the response into the PresignedURLResponse proto element.
func (c *Client) httpCreatePresignedURLProto(ctx context.Context, region, label string, req PresignedURLRequest) (*linodev1.PresignedURLResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_presigned_url_create", req, region, label)
	if err != nil {
		return nil, wrapRequestError("CreatePresignedURL", err)
	}

	defer drainClose(resp)

	result := &linodev1.PresignedURLResponse{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
}

// objectACLNameQuery renders the object-acl name filter.
//
// The object name is a query parameter rather than a path slot, so it stays
// here: the route contract declares the bucket path, and both readers of it
// send the same encoded bytes url.QueryEscape produced before the route moved.
func objectACLNameQuery(name string) string {
	return "name=" + url.QueryEscape(name)
}

// GetObjectACL retrieves the ACL of an object in Object Storage.
func (c *Client) httpGetObjectACL(ctx context.Context, region, label, name string) (*ObjectACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_object_storage_object_acl_get",
		objectACLNameQuery(name), nil, region, label)
	if err != nil {
		return nil, wrapRequestError("GetObjectACL", err)
	}

	defer drainClose(resp)

	var result ObjectACL
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// httpGetObjectACLProto retrieves an object's ACL as a proto message.
func (c *Client) httpGetObjectACLProto(ctx context.Context, region, label, name string) (*linodev1.ObjectACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_object_storage_object_acl_get",
		objectACLNameQuery(name), nil, region, label)
	if err != nil {
		return nil, wrapRequestError("GetObjectACL", err)
	}

	defer drainClose(resp)

	result := &linodev1.ObjectACL{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
}

// httpUpdateObjectACLProto updates an object's ACL and decodes the echoed ACL
// as a proto message.
func (c *Client) httpUpdateObjectACLProto(ctx context.Context, region, label string, req ObjectACLUpdateRequest) (*linodev1.ObjectACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_object_acl_update", req, region, label)
	if err != nil {
		return nil, wrapRequestError("UpdateObjectACL", err)
	}

	defer drainClose(resp)

	result := &linodev1.ObjectACL{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetBucketSSL retrieves the SSL/TLS certificate status for an Object Storage bucket.
func (c *Client) httpGetBucketSSL(ctx context.Context, region, label string) (*BucketSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_ssl_get", nil, region, label)
	if err != nil {
		return nil, wrapRequestError("GetBucketSSL", err)
	}

	defer drainClose(resp)

	var result BucketSSL
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// httpGetBucketSSLProto retrieves a bucket's TLS status as a proto message.
func (c *Client) httpGetBucketSSLProto(ctx context.Context, region, label string) (*linodev1.BucketSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_ssl_get", nil, region, label)
	if err != nil {
		return nil, wrapRequestError("GetBucketSSL", err)
	}

	defer drainClose(resp)

	result := &linodev1.BucketSSL{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
}

// DeleteBucketSSL deletes the SSL/TLS certificate from an Object Storage bucket.
func (c *Client) httpDeleteBucketSSL(ctx context.Context, region, label string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_ssl_delete", nil, region, label)
	if err != nil {
		return wrapRequestError("DeleteBucketSSL", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpUploadBucketSSLProto uploads a certificate and decodes the echoed TLS
// status as a proto message.
func (c *Client) httpUploadBucketSSLProto(ctx context.Context, region, label string, req UploadBucketSSLRequest) (*linodev1.BucketSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_object_storage_ssl_upload", req, region, label)
	if err != nil {
		return nil, wrapRequestError("UploadBucketSSL", err)
	}

	defer drainClose(resp)

	result := &linodev1.BucketSSL{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
}
