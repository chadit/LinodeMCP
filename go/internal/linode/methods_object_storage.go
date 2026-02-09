package linode

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ListObjectStorageBuckets retrieves all Object Storage buckets.
func (c *Client) ListObjectStorageBuckets(ctx context.Context) ([]ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/object-storage/buckets", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageBuckets", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []ObjectStorageBucket `json:"data"`
		Page    int                   `json:"page"`
		Pages   int                   `json:"pages"`
		Results int                   `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetObjectStorageBucket retrieves a specific Object Storage bucket.
func (c *Client) GetObjectStorageBucket(ctx context.Context, region, label string) (*ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/buckets/%s/%s", region, label)

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
func (c *Client) ListObjectStorageBucketContents(ctx context.Context, region, label string, params map[string]string) ([]ObjectStorageObject, bool, string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/buckets/%s/%s/object-list", region, label)

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
		IsTruncated bool                  `json:"is_truncated"` //nolint:tagliatelle // Linode API snake_case
		NextMarker  string                `json:"next_marker"`  //nolint:tagliatelle // Linode API snake_case
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, false, "", err
	}

	return response.Data, response.IsTruncated, response.NextMarker, nil
}

// ListObjectStorageClusters retrieves available Object Storage clusters.
func (c *Client) ListObjectStorageClusters(ctx context.Context) ([]ObjectStorageCluster, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/object-storage/clusters", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageClusters", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []ObjectStorageCluster `json:"data"`
		Page    int                    `json:"page"`
		Pages   int                    `json:"pages"`
		Results int                    `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListObjectStorageTypes retrieves Object Storage types and pricing.
func (c *Client) ListObjectStorageTypes(ctx context.Context) ([]ObjectStorageType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/object-storage/types", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageTypes", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []ObjectStorageType `json:"data"`
		Page    int                 `json:"page"`
		Pages   int                 `json:"pages"`
		Results int                 `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListObjectStorageKeys retrieves all Object Storage access keys.
func (c *Client) ListObjectStorageKeys(ctx context.Context) ([]ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/object-storage/keys", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListObjectStorageKeys", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []ObjectStorageKey `json:"data"`
		Page    int                `json:"page"`
		Pages   int                `json:"pages"`
		Results int                `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetObjectStorageKey retrieves a specific Object Storage access key by ID.
func (c *Client) GetObjectStorageKey(ctx context.Context, keyID int) (*ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/keys/%d", keyID)

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
func (c *Client) GetObjectStorageTransfer(ctx context.Context) (*ObjectStorageTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/object-storage/transfer", nil)
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
func (c *Client) GetObjectStorageBucketAccess(ctx context.Context, region, label string) (*ObjectStorageBucketAccess, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/buckets/%s/%s/access", region, label)

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
func (c *Client) CreateObjectStorageBucket(ctx context.Context, req CreateObjectStorageBucketRequest) (*ObjectStorageBucket, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/object-storage/buckets", req)
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
func (c *Client) DeleteObjectStorageBucket(ctx context.Context, region, label string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/buckets/%s/%s", region, label)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteObjectStorageBucket", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// UpdateObjectStorageBucketAccess updates bucket ACL and CORS settings.
func (c *Client) UpdateObjectStorageBucketAccess(ctx context.Context, region, label string, req UpdateObjectStorageBucketAccessRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/buckets/%s/%s/access", region, label)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "UpdateObjectStorageBucketAccess", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreateObjectStorageKey creates a new Object Storage access key.
func (c *Client) CreateObjectStorageKey(ctx context.Context, req CreateObjectStorageKeyRequest) (*ObjectStorageKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/object-storage/keys", req)
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
func (c *Client) UpdateObjectStorageKey(ctx context.Context, keyID int, req UpdateObjectStorageKeyRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/keys/%d", keyID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return &NetworkError{Operation: "UpdateObjectStorageKey", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// DeleteObjectStorageKey revokes an Object Storage access key.
func (c *Client) DeleteObjectStorageKey(ctx context.Context, keyID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/keys/%d", keyID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteObjectStorageKey", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreatePresignedURL generates a presigned URL for an object in Object Storage.
func (c *Client) CreatePresignedURL(ctx context.Context, region, label string, req PresignedURLRequest) (*PresignedURLResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/buckets/%s/%s/object-url", region, label)

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, req)
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
func (c *Client) GetObjectACL(ctx context.Context, region, label, name string) (*ObjectACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/buckets/%s/%s/object-acl?name=%s", region, label, name)

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
func (c *Client) UpdateObjectACL(ctx context.Context, region, label string, req ObjectACLUpdateRequest) (*ObjectACL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/buckets/%s/%s/object-acl", region, label)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
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
func (c *Client) GetBucketSSL(ctx context.Context, region, label string) (*BucketSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/buckets/%s/%s/ssl", region, label)

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
func (c *Client) DeleteBucketSSL(ctx context.Context, region, label string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/object-storage/buckets/%s/%s/ssl", region, label)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteBucketSSL", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}
