package linode

// ObjectStorageBucket represents a Linode Object Storage bucket.
type ObjectStorageBucket struct {
	Label    string `json:"label"`
	Region   string `json:"region"`
	Hostname string `json:"hostname"`
	Created  string `json:"created"`
	Objects  int    `json:"objects"`
	Size     int    `json:"size"`
	Cluster  string `json:"cluster"`
}

// ObjectStorageObject represents an object within a bucket.
type ObjectStorageObject struct {
	Name         string `json:"name"`
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified"` //nolint:tagliatelle // Linode API snake_case
	Owner        string `json:"owner"`
	Size         int    `json:"size"`
	IsPrefix     bool   `json:"is_prefix"` //nolint:tagliatelle // Linode API snake_case
}

// ObjectStorageCluster represents an Object Storage cluster/region.
type ObjectStorageCluster struct {
	ID         string `json:"id"`
	Region     string `json:"region"`
	Domain     string `json:"domain"`
	StaticSite struct {
		Domain string `json:"domain"`
	} `json:"static_site"` //nolint:tagliatelle // Linode API snake_case
	Status string `json:"status"`
}

// ObjectStorageType represents Object Storage pricing and type info.
type ObjectStorageType struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Price    Price  `json:"price"`
	Transfer int    `json:"transfer"`
	Region   string `json:"region_prices,omitempty"` //nolint:tagliatelle // Linode API snake_case
}

// CreateObjectStorageBucketRequest represents the request body for creating an Object Storage bucket.
type CreateObjectStorageBucketRequest struct {
	Label       string `json:"label"`
	Region      string `json:"region"`
	ACL         string `json:"acl,omitempty"`
	CORSEnabled *bool  `json:"cors_enabled,omitempty"` //nolint:tagliatelle // Linode API snake_case
}

// UpdateObjectStorageBucketAccessRequest represents the request body for updating bucket access.
type UpdateObjectStorageBucketAccessRequest struct {
	ACL         string `json:"acl,omitempty"`
	CORSEnabled *bool  `json:"cors_enabled,omitempty"` //nolint:tagliatelle // Linode API snake_case
}

// ObjectStorageKeyBucketAccess represents bucket-level permissions for an access key.
type ObjectStorageKeyBucketAccess struct {
	BucketName  string `json:"bucket_name"` //nolint:tagliatelle // Linode API snake_case
	Region      string `json:"region"`
	Permissions string `json:"permissions"`
}

// CreateObjectStorageKeyRequest represents the request body for creating an Object Storage key.
type CreateObjectStorageKeyRequest struct {
	Label        string                         `json:"label"`
	BucketAccess []ObjectStorageKeyBucketAccess `json:"bucket_access,omitempty"` //nolint:tagliatelle // Linode API snake_case
}

// UpdateObjectStorageKeyRequest represents the request body for updating an Object Storage key.
type UpdateObjectStorageKeyRequest struct {
	Label        string                         `json:"label,omitempty"`
	BucketAccess []ObjectStorageKeyBucketAccess `json:"bucket_access,omitempty"` //nolint:tagliatelle // Linode API snake_case
}

// PresignedURLRequest represents the request body for generating a presigned URL.
type PresignedURLRequest struct {
	Method    string `json:"method"`
	Name      string `json:"name"`
	ExpiresIn int    `json:"expires_in"` //nolint:tagliatelle // Linode API snake_case
}

// PresignedURLResponse represents the response from generating a presigned URL.
type PresignedURLResponse struct {
	URL string `json:"url"`
}

// ObjectACL represents the ACL of an object in Object Storage.
type ObjectACL struct {
	ACL    string `json:"acl"`
	ACLXML string `json:"acl_xml"` //nolint:tagliatelle // Linode API snake_case
}

// ObjectACLUpdateRequest represents the request body for updating an object's ACL.
type ObjectACLUpdateRequest struct {
	ACL  string `json:"acl"`
	Name string `json:"name"`
}

// BucketSSL represents the SSL/TLS certificate status for an Object Storage bucket.
type BucketSSL struct {
	SSL bool `json:"ssl"`
}

// ObjectStorageKey represents a Linode Object Storage access key.
type ObjectStorageKey struct {
	Label        string                         `json:"label"`
	AccessKey    string                         `json:"access_key"`    //nolint:tagliatelle // Linode API snake_case
	SecretKey    string                         `json:"secret_key"`    //nolint:tagliatelle // Linode API snake_case
	BucketAccess []ObjectStorageKeyBucketAccess `json:"bucket_access"` //nolint:tagliatelle // Linode API snake_case
	Regions      []ObjectStorageKeyRegion       `json:"regions"`
	ID           int                            `json:"id"`
	Limited      bool                           `json:"limited"`
}

// ObjectStorageKeyRegion represents a region associated with an Object Storage key.
type ObjectStorageKeyRegion struct {
	ID         string `json:"id"`
	S3Endpoint string `json:"s3_endpoint"` //nolint:tagliatelle // Linode API snake_case
}

// ObjectStorageTransfer represents Object Storage transfer usage.
type ObjectStorageTransfer struct {
	UsedBytes int `json:"used"`
}

// ObjectStorageBucketAccess represents bucket ACL and CORS settings.
type ObjectStorageBucketAccess struct {
	ACL         string `json:"acl"`
	CORSEnabled bool   `json:"cors_enabled"` //nolint:tagliatelle // Linode API snake_case
}
