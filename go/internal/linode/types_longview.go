package linode

// LongviewSubscription represents the current Longview subscription plan.
type LongviewSubscription struct {
	ClientsIncluded int    `json:"clients_included"`
	ID              string `json:"id"`
	Label           string `json:"label"`
	Price           Price  `json:"price"`
}

// LongviewType represents an available Longview subscription type.
type LongviewType struct {
	ClientsIncluded int    `json:"clients_included"`
	ID              string `json:"id"`
	Label           string `json:"label"`
	Price           Price  `json:"price"`
}

// CreatedLongviewClient represents the response from creating a Longview client.
// The create response may include setup credentials that are intentionally not
// exposed by the read-only LongviewClient list type.
type CreatedLongviewClient struct {
	APIKey      string       `json:"api_key"`
	Apps        LongviewApps `json:"apps"`
	Created     string       `json:"created"`
	ID          int          `json:"id"`
	InstallCode string       `json:"install_code"`
	Label       string       `json:"label"`
	Updated     string       `json:"updated"`
}

// CreateLongviewClientRequest contains editable fields for POST /longview/clients.
type CreateLongviewClientRequest struct {
	Label string `json:"label"`
}
