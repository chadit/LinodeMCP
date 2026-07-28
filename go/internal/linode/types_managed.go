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

// CreateManagedServiceRequest contains fields for POST /managed/services.
type CreateManagedServiceRequest struct {
	Body              *string `json:"body,omitempty"`
	ConsultationGroup *string `json:"consultation_group,omitempty"`
	Notes             *string `json:"notes,omitempty"`
	Region            *string `json:"region,omitempty"`
	Label             string  `json:"label"`
	ServiceType       string  `json:"service_type"`
	Address           string  `json:"address"`
	Credentials       []int   `json:"credentials,omitempty"`
	Timeout           int     `json:"timeout"`
}

// UpdateManagedServiceRequest contains mutable fields for PUT /managed/services/{serviceID}.
// Pointer fields distinguish omitted values from explicit updates.
type UpdateManagedServiceRequest struct {
	Label             *string `json:"label,omitempty"`
	ServiceType       *string `json:"service_type,omitempty"`
	Address           *string `json:"address,omitempty"`
	Timeout           *int    `json:"timeout,omitempty"`
	Body              *string `json:"body,omitempty"`
	ConsultationGroup *string `json:"consultation_group,omitempty"`
	Credentials       *[]int  `json:"credentials,omitempty"`
	Notes             *string `json:"notes,omitempty"`
	Region            *string `json:"region,omitempty"`
}

// ManagedContact represents a contact for Linode Managed service alerts.
type ManagedContact struct {
	Phone   ManagedContactPhone `json:"phone"`
	Group   *string             `json:"group"`
	Name    string              `json:"name"`
	Email   string              `json:"email"`
	Updated string              `json:"updated"`
	ID      int                 `json:"id"`
}

// ManagedContactPhone contains primary and secondary phone numbers for a Managed contact.
type ManagedContactPhone struct {
	Primary   *string `json:"primary"`
	Secondary *string `json:"secondary"`
}

// ManagedLinodeSettings represents Managed service settings for a Linode.
type ManagedLinodeSettings struct {
	Label string                   `json:"label"`
	Group string                   `json:"group"`
	SSH   ManagedLinodeSettingsSSH `json:"ssh"`
	ID    int                      `json:"id"`
}

// ManagedLinodeSettingsSSH contains SSH access settings for Managed service responders.
type ManagedLinodeSettingsSSH struct {
	Port   *int    `json:"port"`
	User   *string `json:"user"`
	IP     string  `json:"ip"`
	Access bool    `json:"access"`
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
	Body              *string `json:"body"`
	Region            *string `json:"region"`
	Notes             *string `json:"notes"`
	Status            string  `json:"status"`
	Address           string  `json:"address"`
	ConsultationGroup string  `json:"consultation_group"`
	Created           string  `json:"created"`
	ServiceType       string  `json:"service_type"`
	Label             string  `json:"label"`
	Updated           string  `json:"updated"`
	Credentials       []int   `json:"credentials"`
	ID                int     `json:"id"`
	Timeout           int     `json:"timeout"`
}

// ManagedIssue represents an issue detected by Linode Managed service monitors.
type ManagedIssue struct {
	Entity   ManagedIssueEntity `json:"entity"`
	Created  string             `json:"created"`
	Services []int              `json:"services"`
	ID       int                `json:"id"`
}

// ManagedIssueEntity identifies the support ticket opened for a Managed issue.
type ManagedIssueEntity struct {
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
	ID    int    `json:"id"`
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
