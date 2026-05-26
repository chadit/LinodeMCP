package linode

// CreateManagedContactRequest contains editable fields for POST /managed/contacts.
// Pointer fields distinguish omitted values from explicit contact attributes.
type CreateManagedContactRequest struct {
	Name  *string                           `json:"name,omitempty"`
	Email *string                           `json:"email,omitempty"`
	Group *string                           `json:"group,omitempty"`
	Phone *CreateManagedContactPhoneRequest `json:"phone,omitempty"`
}

// CreateManagedContactPhoneRequest contains editable phone fields for a managed contact create request.
// Pointer fields distinguish omitted numbers from explicit values.
type CreateManagedContactPhoneRequest struct {
	Primary   *string `json:"primary,omitempty"`
	Secondary *string `json:"secondary,omitempty"`
}

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
