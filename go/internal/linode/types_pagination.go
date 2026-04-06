package linode

// PaginatedResponse represents a standard Linode API paginated response.
type PaginatedResponse[T any] struct {
	Data    []T `json:"data"`
	Page    int `json:"page"`
	Pages   int `json:"pages"`
	Results int `json:"results"`
}
