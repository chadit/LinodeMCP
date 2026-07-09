package linode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const (
	endpointObjBuckets   = "/object-storage/buckets"
	endpointObjEndpoints = "/object-storage/endpoints"
	endpointObjTypes     = "/object-storage/types"
	endpointObjKeys      = "/object-storage/keys"
	endpointObjQuotas    = "/object-storage/quotas"
	endpointObjTransfer  = "/object-storage/transfer"
	endpointObjCancel    = "/object-storage/cancel"
)

// ListObjectStorageBuckets retrieves all Object Storage buckets.
func (c *Client) httpListObjectStorageBuckets(ctx context.Context) ([]ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointObjBuckets, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageBuckets", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[ObjectStorageBucket]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListObjectStorageBucketsProto retrieves all Object Storage buckets as
// proto messages for the proto-backed list path. The endpoint returns a
// {data, page, ...} page envelope, so listProtoElements reads the data field.
func (c *Client) httpListObjectStorageBucketsProto(ctx context.Context) ([]*linodev1.ObjectStorageBucket, error) {
	return listProtoElements(ctx, c, "ListObjectStorageBuckets", endpointObjBuckets,
		func() *linodev1.ObjectStorageBucket { return &linodev1.ObjectStorageBucket{} })
}

// httpListObjectStorageBucketsByRegionProto retrieves Object Storage buckets in
// a region as proto messages for the proto-backed read path. The region is
// path-escaped into the endpoint before the call, matching the non-region list;
// the endpoint returns the {data,page,...} page envelope listProtoElements reads.
func (c *Client) httpListObjectStorageBucketsByRegionProto(ctx context.Context, region string) ([]*linodev1.ObjectStorageBucket, error) {
	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s", url.PathEscape(region))

	return listProtoElements(ctx, c, "ListObjectStorageBucketsByRegion", endpoint,
		func() *linodev1.ObjectStorageBucket { return &linodev1.ObjectStorageBucket{} })
}

// GetObjectStorageBucket retrieves a specific Object Storage bucket.
func (c *Client) httpGetObjectStorageBucket(ctx context.Context, region, label string) (*ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageBucket", Err: err}
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

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageBucket", Err: err}
	}

	defer drainClose(resp)

	bucket := &linodev1.ObjectStorageBucket{}
	if err := c.handleProtoResponse(resp, bucket); err != nil {
		return nil, err
	}

	return bucket, nil
}

// ObjectStorageBucketContentsPage is the decoded body of the S3-style
// object-list endpoint: the proto object elements plus the S3 pagination
// metadata (is_truncated and next_marker) the standard {data,page,...} envelope
// does not carry.
type ObjectStorageBucketContentsPage struct {
	Objects     []*linodev1.ObjectStorageObject
	IsTruncated bool
	NextMarker  string
}

// httpListObjectStorageBucketContentsProto lists objects in a bucket as proto
// messages for the proto-backed list path. The endpoint returns a bespoke
// {data:[...], is_truncated, next_marker} body (not the standard page envelope),
// so this decodes the data[] elements with protojson and returns the truncation
// metadata alongside them.
func (c *Client) httpListObjectStorageBucketContentsProto(ctx context.Context, region, label string, params map[string]string) (*ObjectStorageBucketContentsPage, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/object-list", url.PathEscape(region), url.PathEscape(label))

	if len(params) > 0 {
		vals := url.Values{}
		for k, v := range params {
			vals.Set(k, v)
		}

		endpoint += "?" + vals.Encode()
	}

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageBucketContents", Err: err}
	}

	defer drainClose(resp)

	var envelope struct {
		Data        []json.RawMessage `json:"data"`
		IsTruncated bool              `json:"is_truncated"`
		NextMarker  string            `json:"next_marker"`
	}

	if err := c.handleResponse(resp, &envelope); err != nil {
		return nil, err
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

// ListObjectStorageEndpoints retrieves Object Storage endpoints.
func (c *Client) httpListObjectStorageEndpoints(ctx context.Context) ([]ObjectStorageEndpoint, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointObjEndpoints, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageEndpoints", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[ObjectStorageEndpoint]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListObjectStorageEndpointsProto retrieves Object Storage endpoints as proto
// ObjectStorageEndpoint messages for the proto-backed list path.
func (c *Client) httpListObjectStorageEndpointsProto(ctx context.Context) ([]*linodev1.ObjectStorageEndpoint, error) {
	return listProtoElements(ctx, c, "ListObjectStorageEndpoints", endpointObjEndpoints,
		func() *linodev1.ObjectStorageEndpoint { return &linodev1.ObjectStorageEndpoint{} })
}

// httpListObjectStorageTypesProto retrieves Object Storage types and pricing as
// proto LinodeType messages for the proto-backed list path. The element shares
// the LinodeType shape (id, label, price, region_prices[], transfer), so
// region_prices decodes as a repeated message, not a string.
func (c *Client) httpListObjectStorageTypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	return listProtoElements(ctx, c, "ListObjectStorageTypes", endpointObjTypes,
		func() *linodev1.LinodeType { return &linodev1.LinodeType{} })
}

// ListObjectStorageTypes retrieves Object Storage types and pricing.
func (c *Client) httpListObjectStorageTypes(ctx context.Context) ([]ObjectStorageType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointObjTypes, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageTypes", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[ObjectStorageType]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListObjectStorageQuotas retrieves Object Storage quotas for the account.
func (c *Client) httpListObjectStorageQuotas(ctx context.Context) ([]ObjectStorageQuota, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointObjQuotas, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageQuotas", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[ObjectStorageQuota]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListObjectStorageQuotasProto retrieves Object Storage quotas as proto
// ObjectStorageQuota messages for the proto-backed list path.
func (c *Client) httpListObjectStorageQuotasProto(ctx context.Context) ([]*linodev1.ObjectStorageQuota, error) {
	return listProtoElements(ctx, c, "ListObjectStorageQuotas", endpointObjQuotas,
		func() *linodev1.ObjectStorageQuota { return &linodev1.ObjectStorageQuota{} })
}

// ListObjectStorageKeys retrieves all Object Storage access keys.
func (c *Client) httpListObjectStorageKeys(ctx context.Context) ([]ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointObjKeys, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageKeys", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[ObjectStorageKey]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListObjectStorageKeysProto retrieves Object Storage keys as proto messages
// for the proto-backed list path. The endpoint returns a {data, page, ...} page
// envelope, so listProtoElements reads the data field. The list endpoint returns
// keys without secret material, so each element's secret_key decodes to its empty
// default.
func (c *Client) httpListObjectStorageKeysProto(ctx context.Context) ([]*linodev1.ObjectStorageKey, error) {
	return listProtoElements(ctx, c, "ListObjectStorageKeys", endpointObjKeys,
		func() *linodev1.ObjectStorageKey { return &linodev1.ObjectStorageKey{} })
}

// GetObjectStorageKey retrieves a specific Object Storage access key by ID.
func (c *Client) httpGetObjectStorageKey(ctx context.Context, keyID int) (*ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjKeys+"/%d", keyID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageKey", Err: err}
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

	endpoint := fmt.Sprintf(endpointObjKeys+"/%d", keyID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageKey", Err: err}
	}

	defer drainClose(resp)

	key := &linodev1.ObjectStorageKey{}
	if err := c.handleProtoResponse(resp, key); err != nil {
		return nil, err
	}

	return key, nil
}

// httpGetObjectStorageQuotaUsageProto retrieves usage data for a specific Object
// Storage quota and decodes it into the ObjectStorageQuotaUsage proto element. The
// byte counts are int64, so protojson serializes them as JSON strings; usage is
// optional and omitted when the API returns null (before any usage is recorded).
func (c *Client) httpGetObjectStorageQuotaUsageProto(ctx context.Context, quotaID string) (*linodev1.ObjectStorageQuotaUsage, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjQuotas+"/%s/usage", url.PathEscape(quotaID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageQuotaUsage", Err: err}
	}

	defer drainClose(resp)

	usage := &linodev1.ObjectStorageQuotaUsage{}
	if err := c.handleProtoResponse(resp, usage); err != nil {
		return nil, err
	}

	return usage, nil
}

// httpGetObjectStorageTransferProto retrieves Object Storage outbound data
// transfer usage and decodes it into the ObjectStorageTransfer proto element. The
// used byte count is int64, so protojson serializes it as a JSON string.
func (c *Client) httpGetObjectStorageTransferProto(ctx context.Context) (*linodev1.ObjectStorageTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointObjTransfer, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageTransfer", Err: err}
	}

	defer drainClose(resp)

	transfer := &linodev1.ObjectStorageTransfer{}
	if err := c.handleProtoResponse(resp, transfer); err != nil {
		return nil, err
	}

	return transfer, nil
}

// httpGetObjectStorageQuotaProto retrieves a single Object Storage quota and
// decodes it into the ObjectStorageQuota proto element for the proto-backed read
// path. The quota GET returns the bare quota object (quota usage is a separate
// endpoint), so the body decodes straight into the element with DiscardUnknown.
func (c *Client) httpGetObjectStorageQuotaProto(ctx context.Context, objQuotaID string) (*linodev1.ObjectStorageQuota, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjQuotas+"/%s", url.PathEscape(objQuotaID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageQuota", Err: err}
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

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointObjCancel, nil)
	if err != nil {
		return &NetworkError{Operation: "CancelObjectStorage", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// GetObjectStorageBucketAccess retrieves ACL and CORS settings for a bucket.
func (c *Client) httpGetObjectStorageBucketAccess(ctx context.Context, region, label string) (*ObjectStorageBucketAccess, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/access", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageBucketAccess", Err: err}
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

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/access", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageBucketAccess", Err: err}
	}

	defer drainClose(resp)

	access := &linodev1.ObjectStorageBucketAccess{}
	if err := c.handleProtoResponse(resp, access); err != nil {
		return nil, err
	}

	return access, nil
}

// CreateObjectStorageBucket creates a new Object Storage bucket.
func (c *Client) httpCreateObjectStorageBucket(ctx context.Context, req CreateObjectStorageBucketRequest) (*ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointObjBuckets, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateObjectStorageBucket", Err: err}
	}

	defer drainClose(resp)

	var bucket ObjectStorageBucket
	if err := c.handleResponse(resp, &bucket); err != nil {
		return nil, err
	}

	return &bucket, nil
}

// httpCreateObjectStorageBucketProto creates an Object Storage bucket as a proto
// message.
func (c *Client) httpCreateObjectStorageBucketProto(ctx context.Context, req CreateObjectStorageBucketRequest) (*linodev1.ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointObjBuckets, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateObjectStorageBucket", Err: err}
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

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteObjectStorageBucket", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// UpdateObjectStorageBucketAccess updates bucket ACL and CORS settings.
func (c *Client) httpUpdateObjectStorageBucketAccess(ctx context.Context, region, label string, req UpdateObjectStorageBucketAccessRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/access", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "UpdateObjectStorageBucketAccess", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// AllowObjectStorageBucketAccess applies bucket ACL and CORS settings.
func (c *Client) httpAllowObjectStorageBucketAccess(ctx context.Context, region, label string, req AllowObjectStorageBucketAccessRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/access", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "AllowObjectStorageBucketAccess", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// CreateObjectStorageKey creates a new Object Storage access key.
func (c *Client) httpCreateObjectStorageKey(ctx context.Context, req CreateObjectStorageKeyRequest) (*ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointObjKeys, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateObjectStorageKey", Err: err}
	}

	defer drainClose(resp)

	var key ObjectStorageKey
	if err := c.handleResponse(resp, &key); err != nil {
		return nil, err
	}

	return &key, nil
}

// httpCreateObjectStorageKeyProto creates an Object Storage key as a proto message.
func (c *Client) httpCreateObjectStorageKeyProto(ctx context.Context, req CreateObjectStorageKeyRequest) (*linodev1.ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointObjKeys, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateObjectStorageKey", Err: err}
	}

	defer drainClose(resp)

	key := &linodev1.ObjectStorageKey{}
	if err := c.handleProtoResponse(resp, key); err != nil {
		return nil, err
	}

	return key, nil
}

// UpdateObjectStorageKey updates an Object Storage access key.
func (c *Client) httpUpdateObjectStorageKey(ctx context.Context, keyID int, req UpdateObjectStorageKeyRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjKeys+"/%d", keyID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "UpdateObjectStorageKey", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpUpdateObjectStorageKeyProto updates a key and decodes the echoed key as a
// proto message. The update endpoint returns the full key without secret
// material, so the element's secret_key serializes as its empty default.
func (c *Client) httpUpdateObjectStorageKeyProto(ctx context.Context, keyID int, req UpdateObjectStorageKeyRequest) (*linodev1.ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjKeys+"/%d", keyID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateObjectStorageKey", Err: err}
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

	endpoint := fmt.Sprintf(endpointObjKeys+"/%d", keyID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteObjectStorageKey", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCreatePresignedURLProto generates a presigned URL for an object in Object
// Storage and decodes the response into the PresignedURLResponse proto element.
func (c *Client) httpCreatePresignedURLProto(ctx context.Context, region, label string, req PresignedURLRequest) (*linodev1.PresignedURLResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/object-url", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreatePresignedURL", Err: err}
	}

	defer drainClose(resp)

	result := &linodev1.PresignedURLResponse{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetObjectACL retrieves the ACL of an object in Object Storage.
func (c *Client) httpGetObjectACL(ctx context.Context, region, label, name string) (*ObjectACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/object-acl?name=%s", url.PathEscape(region), url.PathEscape(label), url.QueryEscape(name))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectACL", Err: err}
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

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/object-acl?name=%s", url.PathEscape(region), url.PathEscape(label), url.QueryEscape(name))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectACL", Err: err}
	}

	defer drainClose(resp)

	result := &linodev1.ObjectACL{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
}

// UpdateObjectACL updates the ACL of an object in Object Storage.
func (c *Client) httpUpdateObjectACL(ctx context.Context, region, label string, req ObjectACLUpdateRequest) (*ObjectACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/object-acl", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateObjectACL", Err: err}
	}

	defer drainClose(resp)

	var result ObjectACL
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// httpUpdateObjectACLProto updates an object's ACL and decodes the echoed ACL
// as a proto message.
func (c *Client) httpUpdateObjectACLProto(ctx context.Context, region, label string, req ObjectACLUpdateRequest) (*linodev1.ObjectACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/object-acl", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateObjectACL", Err: err}
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

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/ssl", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetBucketSSL", Err: err}
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

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/ssl", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetBucketSSL", Err: err}
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

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/ssl", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteBucketSSL", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpUploadBucketSSLProto uploads a certificate and decodes the echoed TLS
// status as a proto message.
func (c *Client) httpUploadBucketSSLProto(ctx context.Context, region, label string, req UploadBucketSSLRequest) (*linodev1.BucketSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/ssl", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UploadBucketSSL", Err: err}
	}

	defer drainClose(resp)

	result := &linodev1.BucketSSL{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
}

// UploadBucketSSL uploads an SSL/TLS certificate to an Object Storage bucket.
func (c *Client) httpUploadBucketSSL(ctx context.Context, region, label string, req UploadBucketSSLRequest) (*BucketSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/ssl", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UploadBucketSSL", Err: err}
	}

	defer drainClose(resp)

	var result BucketSSL
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
