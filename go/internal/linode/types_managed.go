package linode

// ManagedContact represents a contact for Linode Managed service alerts.
type ManagedContact struct {
	ID      int                 `json:"id"`
	Name    string              `json:"name"`
	Email   string              `json:"email"`
	Group   *string             `json:"group"`
	Phone   ManagedContactPhone `json:"phone"`
	Updated string              `json:"updated"`
}

// ManagedContactPhone contains primary and secondary phone numbers for a Managed contact.
type ManagedContactPhone struct {
	Primary   *string `json:"primary"`
	Secondary *string `json:"secondary"`
}
