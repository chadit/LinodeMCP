package linode

// LongviewSubscription represents the current Longview subscription plan.
type LongviewSubscription struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Price           Price  `json:"price"`
	ClientsIncluded int    `json:"clients_included"`
}

// LongviewType represents an available Longview subscription type.
type LongviewType struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Price           Price  `json:"price"`
	ClientsIncluded int    `json:"clients_included"`
}

// CreatedLongviewClient represents the response from creating a Longview client.
// The create response may include setup credentials that are intentionally not
// exposed by the read-only LongviewClient list type.
type CreatedLongviewClient struct {
	APIKey      string       `json:"api_key"`
	Created     string       `json:"created"`
	InstallCode string       `json:"install_code"`
	Label       string       `json:"label"`
	Updated     string       `json:"updated"`
	ID          int          `json:"id"`
	Apps        LongviewApps `json:"apps"`
}

// CreateLongviewClientRequest contains editable fields for POST /longview/clients.
type CreateLongviewClientRequest struct {
	Label string `json:"label"`
}
