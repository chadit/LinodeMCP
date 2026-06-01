package linode

// TaggedObject represents one resource returned by GET /tags/{tagLabel}.
// The Linode API can return several resource shapes in this list, so keep the
// object flexible while preserving the paginated response envelope.
type TaggedObject map[string]any

// Tag represents a Linode tag.
type Tag struct {
	Label         string `json:"label"`
	Domains       []int  `json:"domains,omitempty"`
	Linodes       []int  `json:"linodes,omitempty"`
	NodeBalancers []int  `json:"nodebalancers,omitempty"`
	Volumes       []int  `json:"volumes,omitempty"`
}

// CreateTagRequest represents the request body for creating a tag.
type CreateTagRequest struct {
	Label         string `json:"label"`
	Domains       []int  `json:"domains,omitempty"`
	Linodes       []int  `json:"linodes,omitempty"`
	NodeBalancers []int  `json:"nodebalancers,omitempty"`
	Volumes       []int  `json:"volumes,omitempty"`
}
