package linode

// Tag represents a tag label returned by the Linode Tags API.
type Tag struct {
	Label string `json:"label"`
}

// TaggedObject represents one resource returned by GET /tags/{tagLabel}.
// The Linode API can return several resource shapes in this list, so keep the
// object flexible while preserving the paginated response envelope.
type TaggedObject map[string]any
