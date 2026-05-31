package linode

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

const (
	endpointObjBuckets  = "/object-storage/buckets"
	endpointObjClusters = "/object-storage/clusters"
	endpointObjTypes    = "/object-storage/types"
	endpointObjKeys     = "/object-storage/keys"
	endpointObjQuotas   = "/object-storage/quotas"
	endpointObjTransfer = "/object-storage/transfer"
	endpointObjCancel   = "/object-storage/cancel"
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

// ListObjectStorageBucketsByRegion retrieves Object Storage buckets in a region.
func (c *Client) httpListObjectStorageBucketsByRegion(ctx context.Context, region string) ([]ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s", url.PathEscape(region))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageBucketsByRegion", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[ObjectStorageBucket]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
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

// ListObjectStorageBucketContents lists objects in a bucket.
// Returns objects, isTruncated flag, and nextMarker for pagination.
func (c *Client) httpListObjectStorageBucketContents(ctx context.Context, region, label string, params map[string]string) ([]ObjectStorageObject, bool, string, error) {
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
		return nil, false, "", &NetworkError{Operation: "ListObjectStorageBucketContents", Err: err}
	}

	defer drainClose(resp)

	var response struct {
		Data        []ObjectStorageObject `json:"data"`
		IsTruncated bool                  `json:"is_truncated"`
		NextMarker  string                `json:"next_marker"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, false, "", err
	}

	return response.Data, response.IsTruncated, response.NextMarker, nil
}

// ListObjectStorageClusters retrieves available Object Storage clusters.
func (c *Client) httpListObjectStorageClusters(ctx context.Context) ([]ObjectStorageCluster, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointObjClusters, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageClusters", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[ObjectStorageCluster]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
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

// GetObjectStorageQuotaUsage retrieves usage data for a specific Object Storage quota.
func (c *Client) httpGetObjectStorageQuotaUsage(ctx context.Context, quotaID string) (*ObjectStorageQuotaUsage, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjQuotas+"/%s/usage", url.PathEscape(quotaID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageQuotaUsage", Err: err}
	}

	defer drainClose(resp)

	var usage ObjectStorageQuotaUsage
	if err := c.handleResponse(resp, &usage); err != nil {
		return nil, err
	}

	return &usage, nil
}

// GetObjectStorageTransfer retrieves Object Storage outbound data transfer usage.
func (c *Client) httpGetObjectStorageTransfer(ctx context.Context) (*ObjectStorageTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointObjTransfer, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageTransfer", Err: err}
	}

	defer drainClose(resp)

	var transfer ObjectStorageTransfer
	if err := c.handleResponse(resp, &transfer); err != nil {
		return nil, err
	}

	return &transfer, nil
}

// GetObjectStorageQuota retrieves a single Object Storage quota.
func (c *Client) httpGetObjectStorageQuota(ctx context.Context, objQuotaID string) (*ObjectStorageQuota, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjQuotas+"/%s", url.PathEscape(objQuotaID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageQuota", Err: err}
	}

	defer drainClose(resp)

	var quota ObjectStorageQuota
	if err := c.handleResponse(resp, &quota); err != nil {
		return nil, err
	}

	return &quota, nil
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

// CreatePresignedURL generates a presigned URL for an object in Object Storage.
func (c *Client) httpCreatePresignedURL(ctx context.Context, region, label string, req PresignedURLRequest) (*PresignedURLResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointObjBuckets+"/%s/%s/object-url", url.PathEscape(region), url.PathEscape(label))

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreatePresignedURL", Err: err}
	}

	defer drainClose(resp)

	var result PresignedURLResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
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
