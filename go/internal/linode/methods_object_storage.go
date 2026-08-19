package linode

import (
	"context"
	"net/url"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

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

// ObjectStorageBucketContentsPage is the decoded body of the S3-style
// object-list endpoint: the object elements plus the marker pagination metadata
// the standard page envelope does not carry.
type ObjectStorageBucketContentsPage struct {
	NextMarker  string
	Objects     []*linodev1.ObjectStorageObject
	IsTruncated bool
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
