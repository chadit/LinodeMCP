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

// ManagedLinodeSettings represents Managed service settings for a Linode.
type ManagedLinodeSettings struct {
	ID    int                      `json:"id"`
	Label string                   `json:"label"`
	Group string                   `json:"group"`
	SSH   ManagedLinodeSettingsSSH `json:"ssh"`
}

// ManagedLinodeSettingsSSH contains SSH access settings for Managed service responders.
type ManagedLinodeSettingsSSH struct {
	Access bool    `json:"access"`
	IP     string  `json:"ip"`
	Port   *int    `json:"port"`
	User   *string `json:"user"`
}

// UpdateManagedLinodeSettingsRequest contains mutable Managed Linode settings fields.
type UpdateManagedLinodeSettingsRequest struct {
	SSH *UpdateManagedLinodeSettingsSSH `json:"ssh,omitempty"`
}

// UpdateManagedLinodeSettingsSSH contains mutable SSH settings for a Managed Linode.
type UpdateManagedLinodeSettingsSSH struct {
	Access *bool   `json:"access,omitempty"`
	IP     *string `json:"ip,omitempty"`
	Port   *int    `json:"port,omitempty"`
	User   *string `json:"user,omitempty"`
}

// ManagedService represents a service monitored by Linode Managed.
type ManagedService struct {
	ID                int     `json:"id"`
	Label             string  `json:"label"`
	ServiceType       string  `json:"service_type"`
	Status            string  `json:"status"`
	Address           string  `json:"address"`
	Body              *string `json:"body"`
	ConsultationGroup string  `json:"consultation_group"`
	Created           string  `json:"created"`
	Credentials       []int   `json:"credentials"`
	Notes             *string `json:"notes"`
	Region            *string `json:"region"`
	Timeout           int     `json:"timeout"`
	Updated           string  `json:"updated"`
}

// ManagedIssue represents an issue detected by Linode Managed service monitors.
type ManagedIssue struct {
	ID       int                `json:"id"`
	Created  string             `json:"created"`
	Services []int              `json:"services"`
	Entity   ManagedIssueEntity `json:"entity"`
}

// ManagedIssueEntity identifies the support ticket opened for a Managed issue.
type ManagedIssueEntity struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

// UpdateManagedContactRequest contains mutable Managed contact fields.
type UpdateManagedContactRequest struct {
	Name  *string                    `json:"name,omitempty"`
	Email *string                    `json:"email,omitempty"`
	Group *string                    `json:"group,omitempty"`
	Phone *UpdateManagedContactPhone `json:"phone,omitempty"`
}

// UpdateManagedContactPhone contains mutable phone fields for a Managed contact.
type UpdateManagedContactPhone struct {
	Primary   *string `json:"primary,omitempty"`
	Secondary *string `json:"secondary,omitempty"`
}
