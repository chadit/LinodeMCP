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
	endpointObjTransfer = "/object-storage/transfer"
)

// ListObjectStorageBuckets retrieves all Object Storage buckets.
func (c *Client) httpListObjectStorageBuckets(ctx context.Context) ([]ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointObjBuckets, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageBuckets", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[ObjectStorageType]

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

	var key ObjectStorageKey
	if err := c.handleResponse(resp, &key); err != nil {
		return nil, err
	}

	return &key, nil
}

// GetObjectStorageTransfer retrieves Object Storage outbound data transfer usage.
func (c *Client) httpGetObjectStorageTransfer(ctx context.Context) (*ObjectStorageTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointObjTransfer, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetObjectStorageTransfer", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var transfer ObjectStorageTransfer
	if err := c.handleResponse(resp, &transfer); err != nil {
		return nil, err
	}

	return &transfer, nil
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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

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

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}
