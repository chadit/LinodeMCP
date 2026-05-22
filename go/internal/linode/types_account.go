package linode

// Profile represents a Linode user profile. The Scopes field is populated
// for personal access tokens via the /profile response; OAuth tokens leave
// it empty and Phase 6 scope validation falls back to /profile/grants for
// those. Other callers can ignore the field.
type Profile struct {
	Username           string `json:"username"`
	Email              string `json:"email"`
	Timezone           string `json:"timezone"`
	UID                int    `json:"uid"`
	EmailNotifications bool   `json:"email_notifications"`
	Restricted         bool   `json:"restricted"`
	TwoFactorAuth      bool   `json:"two_factor_auth"`
	Scopes             string `json:"scopes,omitempty"`
}

// GrantPermission is one of "read_only", "read_write", or "" (no access).
// The Linode API uses an explicit empty string when the OAuth grant carries
// no permission on a resource, so we keep it as a string rather than an
// enum so unknown future values round-trip cleanly.
type GrantPermission string

// Grant represents the permission an OAuth token has on a single resource
// instance. The Linode /profile/grants response groups grants by resource
// category (linode, domain, nodebalancer, etc); each entry inside the
// category names a specific resource the token has access to.
type Grant struct {
	ID          int             `json:"id"`
	Label       string          `json:"label"`
	Permissions GrantPermission `json:"permissions"`
}

// Grants represents the full /profile/grants response for OAuth tokens.
// Global covers account-level permissions (read/write per resource type);
// the per-resource slices enumerate the specific instances the token can
// touch. The shape mirrors the Linode API directly so future fields are
// additive.
//
// PATs always return an empty Grants object; their scope information is
// on Profile.Scopes instead. Phase 6's profile loader checks both.
type Grants struct {
	Global       GlobalGrants `json:"global"`
	Linode       []Grant      `json:"linode"`
	Domain       []Grant      `json:"domain"`
	NodeBalancer []Grant      `json:"nodebalancer"`
	Image        []Grant      `json:"image"`
	Longview     []Grant      `json:"longview"`
	StackScript  []Grant      `json:"stackscript"`
	Volume       []Grant      `json:"volume"`
	Database     []Grant      `json:"database"`
	Firewall     []Grant      `json:"firewall"`
	VPC          []Grant      `json:"vpc"`
	LKECluster   []Grant      `json:"lkecluster"`
}

// GlobalGrants captures the account-level permission booleans the OAuth
// flow returns. The Linode API exposes each capability as its own bool;
// keeping them as separate fields matches the wire format and avoids
// magic-string lookups in scope-comparison code.
type GlobalGrants struct {
	AccountAccess        GrantPermission `json:"account_access"`
	AddDatabases         bool            `json:"add_databases"`
	AddDomains           bool            `json:"add_domains"`
	AddFirewalls         bool            `json:"add_firewalls"`
	AddImages            bool            `json:"add_images"`
	AddLinodes           bool            `json:"add_linodes"`
	AddLongview          bool            `json:"add_longview"`
	AddNodeBalancers     bool            `json:"add_nodebalancers"`
	AddStackScripts      bool            `json:"add_stackscripts"`
	AddVolumes           bool            `json:"add_volumes"`
	AddVPCs              bool            `json:"add_vpcs"`
	CancelAccount        bool            `json:"cancel_account"`
	ChildAccountAccess   bool            `json:"child_account_access"`
	LongviewSubscription bool            `json:"longview_subscription"`
}

// AccountAgreements represents the acknowledgment status for account agreements.
type AccountAgreements struct {
	BillingAgreement       bool `json:"billing_agreement"`
	EUModel                bool `json:"eu_model"`
	MasterServiceAgreement bool `json:"master_service_agreement"`
	PrivacyPolicy          bool `json:"privacy_policy"`
}

// AccountMaintenance represents one account maintenance record.
type AccountMaintenance struct {
	Entity AccountMaintenanceEntity `json:"entity"`
	Reason string                   `json:"reason"`
	Status string                   `json:"status"`
	Type   string                   `json:"type"`
	When   string                   `json:"when"`
}

// AccountMaintenanceEntity identifies the entity attached to a maintenance record.
type AccountMaintenanceEntity struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

// AccountNotification represents one account notification returned by GET /account/notifications.
type AccountNotification struct {
	Entity   *AccountNotificationEntity `json:"entity"`
	Label    string                     `json:"label"`
	Message  string                     `json:"message"`
	Severity string                     `json:"severity"`
	Type     string                     `json:"type"`
	Until    *string                    `json:"until"`
	When     *string                    `json:"when"`
}

// AccountNotificationEntity identifies the entity attached to an account notification.
type AccountNotificationEntity struct {
	ID    any    `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

// AccountAvailability represents the account service availability for a region.
type AccountAvailability struct {
	Available   []string `json:"available"`
	Region      string   `json:"region"`
	Unavailable []string `json:"unavailable"`
}

// AccountBetaProgram represents a beta program that the account is enrolled in.
type AccountBetaProgram struct {
	Description *string `json:"description"`
	Ended       *string `json:"ended"`
	Enrolled    string  `json:"enrolled"`
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Started     string  `json:"started"`
}

// AccountEvent represents an account event returned by GET /account/events.
type AccountEvent struct {
	Action          string              `json:"action"`
	Created         string              `json:"created"`
	Duration        *float64            `json:"duration"`
	Entity          *AccountEventEntity `json:"entity"`
	ID              int                 `json:"id"`
	Message         string              `json:"message"`
	PercentComplete *int                `json:"percent_complete"`
	Rate            *string             `json:"rate"`
	SecondaryEntity *AccountEventEntity `json:"secondary_entity"`
	Seen            bool                `json:"seen"`
	Status          string              `json:"status"`
	TimeRemaining   *string             `json:"time_remaining"`
	Username        string              `json:"username"`
}

// AccountEventEntity identifies the primary or secondary entity attached to an account event.
type AccountEventEntity struct {
	ID    any    `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

// AccountLogin represents one user login returned by GET /account/logins.
type AccountLogin struct {
	Datetime   string `json:"datetime"`
	ID         int    `json:"id"`
	IP         string `json:"ip"`
	Restricted bool   `json:"restricted"`
	Status     string `json:"status"`
	Username   string `json:"username"`
}

// AccountInvoice represents one account invoice.
type AccountInvoice struct {
	ID    int     `json:"id"`
	Date  string  `json:"date"`
	Label string  `json:"label"`
	Total float64 `json:"total"`
}

// AccountPayment represents one account payment.
type AccountPayment struct {
	ID   int     `json:"id"`
	Date string  `json:"date"`
	USD  float64 `json:"usd"`
}

// AccountInvoiceItem represents one line item on an account invoice.
type AccountInvoiceItem struct {
	Amount    float64 `json:"amount"`
	From      string  `json:"from"`
	Label     string  `json:"label"`
	Quantity  int     `json:"quantity"`
	Tax       float64 `json:"tax"`
	To        string  `json:"to"`
	Total     float64 `json:"total"`
	Type      string  `json:"type"`
	UnitPrice float64 `json:"unit_price"`
}

// OAuthClient represents an OAuth client registered on the account.
type OAuthClient struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Public       bool   `json:"public"`
	RedirectURI  string `json:"redirect_uri"`
	Status       string `json:"status"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// AccountPaymentMethod represents a payment method available on the account.
type AccountPaymentMethod struct {
	ID        int            `json:"id"`
	Type      string         `json:"type"`
	IsDefault bool           `json:"is_default"`
	Data      map[string]any `json:"data"`
}

// CreateAccountPaymentMethodRequest contains the required fields for POST /account/payment-methods.
type CreateAccountPaymentMethodRequest struct {
	Type      string         `json:"type"`
	Data      map[string]any `json:"data"`
	IsDefault bool           `json:"is_default"`
}

// CreatedOAuthClient represents the response from creating an OAuth client.
type CreatedOAuthClient struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	RedirectURI string `json:"redirect_uri"`
	Secret      string `json:"secret"`
}

// OAuthClientSecret represents the response from resetting an OAuth client secret.
type OAuthClientSecret struct {
	Secret string `json:"secret"`
}

// CreateOAuthClientRequest contains the required fields for POST /account/oauth-clients.
type CreateOAuthClientRequest struct {
	Label       string `json:"label"`
	RedirectURI string `json:"redirect_uri"`
}

// UpdateOAuthClientRequest contains fields for PUT /account/oauth-clients/{clientId}.
type UpdateOAuthClientRequest struct {
	Label       *string `json:"label,omitempty"`
	Public      *bool   `json:"public,omitempty"`
	RedirectURI *string `json:"redirect_uri,omitempty"`
}

// ChildAccount represents a child-level account available to a parent account.
type ChildAccount struct {
	ActiveSince       string                 `json:"active_since"`
	Address1          string                 `json:"address_1"`
	Address2          string                 `json:"address_2"`
	Balance           float64                `json:"balance"`
	BalanceUninvoiced float64                `json:"balance_uninvoiced"`
	BillingSource     string                 `json:"billing_source"`
	Capabilities      []string               `json:"capabilities"`
	City              string                 `json:"city"`
	Company           string                 `json:"company"`
	Country           string                 `json:"country"`
	CreditCard        ChildAccountCreditCard `json:"credit_card"`
	Email             string                 `json:"email"`
	EUUID             string                 `json:"euuid"`
	FirstName         string                 `json:"first_name"`
	LastName          string                 `json:"last_name"`
	Phone             string                 `json:"phone"`
	State             string                 `json:"state"`
	TaxID             string                 `json:"tax_id"`
	Zip               string                 `json:"zip"`
}

// ChildAccountCreditCard contains masked credit card details for a child account.
type ChildAccountCreditCard struct {
	Expiry   string `json:"expiry"`
	LastFour string `json:"last_four"`
}

// ProxyUserToken contains a short-lived proxy user token for a child account.
type ProxyUserToken struct {
	Created string `json:"created"`
	Expiry  string `json:"expiry"`
	ID      int    `json:"id"`
	Label   string `json:"label"`
	Scopes  string `json:"scopes"`
	Token   string `json:"token"`
}

// AccountEntityTransfer represents an account entity transfer request.
type AccountEntityTransfer struct {
	Created  string                        `json:"created"`
	Entities AccountEntityTransferEntities `json:"entities"`
	Expiry   string                        `json:"expiry"`
	IsSender bool                          `json:"is_sender"`
	Status   string                        `json:"status"`
	Token    string                        `json:"token"`
	Updated  string                        `json:"updated"`
}

// AccountEntityTransferEntities groups transferred entities by type.
type AccountEntityTransferEntities struct {
	Linodes []int `json:"linodes"`
}

// CreateAccountEntityTransferRequest contains the entities to transfer for POST /account/entity-transfers.
type CreateAccountEntityTransferRequest struct {
	Entities AccountEntityTransferEntities `json:"entities"`
}

// EnrollAccountBetaRequest contains the beta program identifier for POST /account/betas.
type EnrollAccountBetaRequest struct {
	ID string `json:"id"`
}

// AcknowledgeAccountAgreementsRequest contains the optional agreement flags for
// POST /account/agreements. Pointer booleans distinguish omitted fields from
// explicit false values.
type AcknowledgeAccountAgreementsRequest struct {
	BillingAgreement       *bool `json:"billing_agreement,omitempty"`
	EUModel                *bool `json:"eu_model,omitempty"`
	MasterServiceAgreement *bool `json:"master_service_agreement,omitempty"`
	PrivacyPolicy          *bool `json:"privacy_policy,omitempty"`
}

// CancelAccountRequest contains optional feedback for POST /account/cancel.
type CancelAccountRequest struct {
	Comments *string `json:"comments,omitempty"`
}

// CancelAccountResponse contains the account cancellation response.
type CancelAccountResponse struct {
	SurveyLink string `json:"survey_link"`
}

// Account represents a Linode account.
type Account struct {
	FirstName         string   `json:"first_name"`
	LastName          string   `json:"last_name"`
	Email             string   `json:"email"`
	Company           string   `json:"company"`
	Address1          string   `json:"address_1"`
	Address2          string   `json:"address_2"`
	City              string   `json:"city"`
	State             string   `json:"state"`
	Zip               string   `json:"zip"`
	Country           string   `json:"country"`
	Phone             string   `json:"phone"`
	Balance           float64  `json:"balance"`
	BalanceUninvoiced float64  `json:"balance_uninvoiced"`
	Capabilities      []string `json:"capabilities"`
	ActiveSince       string   `json:"active_since"`
	EUUID             string   `json:"euuid"`
	BillingSource     string   `json:"billing_source"`
	ActivePromotions  []Promo  `json:"active_promotions"`
}

// UpdateAccountRequest contains the editable fields for PUT /v4/account.
// All fields are pointers so omitted fields are not sent in the request body.
type UpdateAccountRequest struct {
	Address1  *string `json:"address_1,omitempty"`
	Address2  *string `json:"address_2,omitempty"`
	City      *string `json:"city,omitempty"`
	Company   *string `json:"company,omitempty"`
	Country   *string `json:"country,omitempty"`
	Email     *string `json:"email,omitempty"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	State     *string `json:"state,omitempty"`
	TaxID     *string `json:"tax_id,omitempty"`
	Zip       *string `json:"zip,omitempty"`
}

// Promo represents an active promotion on an account.
type Promo struct {
	Description              string `json:"description"`
	Summary                  string `json:"summary"`
	CreditMonthlyCap         string `json:"credit_monthly_cap"`
	CreditRemaining          string `json:"credit_remaining"`
	ExpireDT                 string `json:"expire_dt"`
	ImageURL                 string `json:"image_url"`
	ServiceType              string `json:"service_type"`
	ThisMonthCreditRemaining string `json:"this_month_credit_remaining"`
}

// UpdateProfileRequest contains the updatable fields for PUT /v4/profile.
// All fields are pointers so omitted fields are not sent in the request body.
type UpdateProfileRequest struct {
	AuthorizedKeys     *[]string `json:"authorized_keys,omitempty"`
	Email              *string   `json:"email,omitempty"`
	EmailNotifications *bool     `json:"email_notifications,omitempty"`
	LishAuthMethod     *string   `json:"lish_auth_method,omitempty"`
	Restricted         *bool     `json:"restricted,omitempty"`
	Timezone           *string   `json:"timezone,omitempty"`
	TwoFactorAuth      *bool     `json:"two_factor_auth,omitempty"`
}
