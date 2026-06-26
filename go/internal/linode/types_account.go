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

// CreateProfileTokenRequest contains optional fields for POST /profile/tokens.
type CreateProfileTokenRequest struct {
	Expiry string `json:"expiry,omitempty"`
	Label  string `json:"label,omitempty"`
	Scopes string `json:"scopes,omitempty"`
}

// ProfileDevice represents a trusted device on the authenticated profile.
type ProfileDevice map[string]any

// ProfileToken represents a personal access token on the authenticated profile.
type ProfileToken map[string]any

// UpdateProfileTokenRequest contains editable fields for PUT /profile/tokens/{tokenID}.
type UpdateProfileTokenRequest map[string]any

// ProfileSecurityQuestions represents the response from GET /profile/security-questions.
type ProfileSecurityQuestions map[string]any

// ProfileTFAEnableResponse represents the response from POST /profile/tfa-enable.
type ProfileTFAEnableResponse map[string]any

// ProfilePhoneNumberRequest contains fields for POST /profile/phone-number.
type ProfilePhoneNumberRequest struct {
	ISOCode     string `json:"iso_code"`
	PhoneNumber string `json:"phone_number"`
}

// ProfilePhoneNumberVerifyRequest contains fields for POST /profile/phone-number/verify.
type ProfilePhoneNumberVerifyRequest struct {
	OTPCode string `json:"otp_code"`
}

// ProfileTFAEnableConfirmRequest contains fields for POST /profile/tfa-enable-confirm.
type ProfileTFAEnableConfirmRequest struct {
	TFACode string `json:"tfa_code,omitempty"`
}

// ProfileTFAEnableConfirmResponse contains the response from POST /profile/tfa-enable-confirm.
type ProfileTFAEnableConfirmResponse map[string]any

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

// AnswerProfileSecurityQuestionsRequest contains the body for POST /profile/security-questions.
type AnswerProfileSecurityQuestionsRequest struct {
	SecurityQuestions string `json:"security_questions,omitempty"`
}

// UpdateAccountUserGrantsRequest contains editable grant sections for
// PUT /account/users/{username}/grants. Pointer fields preserve the caller's
// intent so omitted sections are not serialized, while empty arrays can still
// be sent when a grant category needs to be cleared.
type UpdateAccountUserGrantsRequest struct {
	Global       *UpdateAccountUserGlobalGrants `json:"global,omitempty"`
	Linode       *[]UpdateAccountUserGrant      `json:"linode,omitempty"`
	Domain       *[]UpdateAccountUserGrant      `json:"domain,omitempty"`
	NodeBalancer *[]UpdateAccountUserGrant      `json:"nodebalancer,omitempty"`
	Image        *[]UpdateAccountUserGrant      `json:"image,omitempty"`
	Longview     *[]UpdateAccountUserGrant      `json:"longview,omitempty"`
	StackScript  *[]UpdateAccountUserGrant      `json:"stackscript,omitempty"`
	Volume       *[]UpdateAccountUserGrant      `json:"volume,omitempty"`
	Database     *[]UpdateAccountUserGrant      `json:"database,omitempty"`
	Firewall     *[]UpdateAccountUserGrant      `json:"firewall,omitempty"`
	VPC          *[]UpdateAccountUserGrant      `json:"vpc,omitempty"`
	LKECluster   *[]UpdateAccountUserGrant      `json:"lkecluster,omitempty"`
}

// UpdateAccountUserGrant contains one resource grant update. It intentionally
// excludes Grant.Label because labels are returned by read APIs but are not part
// of the update payload.
type UpdateAccountUserGrant struct {
	ID          int              `json:"id"`
	Permissions *GrantPermission `json:"permissions"`
}

// UpdateAccountUserGlobalGrants contains optional global grant fields for
// account user grants updates. Pointers preserve partial update intent so an
// omitted permission is not serialized as false.
type UpdateAccountUserGlobalGrants struct {
	AccountAccess        *GrantPermission `json:"account_access,omitempty"`
	AddDatabases         *bool            `json:"add_databases,omitempty"`
	AddDomains           *bool            `json:"add_domains,omitempty"`
	AddFirewalls         *bool            `json:"add_firewalls,omitempty"`
	AddImages            *bool            `json:"add_images,omitempty"`
	AddLinodes           *bool            `json:"add_linodes,omitempty"`
	AddLongview          *bool            `json:"add_longview,omitempty"`
	AddNodeBalancers     *bool            `json:"add_nodebalancers,omitempty"`
	AddStackScripts      *bool            `json:"add_stackscripts,omitempty"`
	AddVolumes           *bool            `json:"add_volumes,omitempty"`
	AddVPCs              *bool            `json:"add_vpcs,omitempty"`
	CancelAccount        *bool            `json:"cancel_account,omitempty"`
	ChildAccountAccess   *bool            `json:"child_account_access,omitempty"`
	LongviewSubscription *bool            `json:"longview_subscription,omitempty"`
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

// AccountTransfer represents account network transfer usage returned by GET /account/transfer.
type AccountTransfer struct {
	Billable        int                     `json:"billable"`
	Quota           int                     `json:"quota"`
	Used            int                     `json:"used"`
	RegionTransfers []AccountRegionTransfer `json:"region_transfers"`
}

// AccountRegionTransfer represents network transfer usage for a region.
type AccountRegionTransfer struct {
	ID       string `json:"id"`
	Billable int    `json:"billable"`
	Quota    int    `json:"quota"`
	Used     int    `json:"used"`
}

// AccountSettings represents account-wide settings returned by GET /account/settings.
type AccountSettings struct {
	BackupsEnabled          bool    `json:"backups_enabled"`
	Managed                 bool    `json:"managed"`
	NetworkHelper           bool    `json:"network_helper"`
	LongviewSubscription    *string `json:"longview_subscription"`
	ObjectStorage           *string `json:"object_storage"`
	InterfacesForNewLinodes string  `json:"interfaces_for_new_linodes"`
	MaintenancePolicy       string  `json:"maintenance_policy"`
}

// UpdateAccountSettingsRequest contains editable fields for PUT /v4/account/settings.
// All fields are pointers so omitted fields are not sent in the request body.
type UpdateAccountSettingsRequest struct {
	BackupsEnabled          *bool   `json:"backups_enabled,omitempty"`
	Managed                 *bool   `json:"managed,omitempty"`
	NetworkHelper           *bool   `json:"network_helper,omitempty"`
	LongviewSubscription    *string `json:"longview_subscription,omitempty"`
	ObjectStorage           *string `json:"object_storage,omitempty"`
	InterfacesForNewLinodes *string `json:"interfaces_for_new_linodes,omitempty"`
	MaintenancePolicy       *string `json:"maintenance_policy,omitempty"`
}

// AccountAgreements represents the acknowledgment status for account agreements.
type AccountAgreements struct {
	BillingAgreement       bool `json:"billing_agreement"`
	EUModel                bool `json:"eu_model"`
	MasterServiceAgreement bool `json:"master_service_agreement"`
	PrivacyPolicy          bool `json:"privacy_policy"`
}

// ManagedCredential represents one stored credential returned by GET /managed/credentials.
type ManagedCredential struct {
	ID            int    `json:"id"`
	Label         string `json:"label"`
	LastDecrypted string `json:"last_decrypted"`
}

// UpdateManagedCredentialRequest contains mutable fields for PUT /managed/credentials/{credentialID}.
// Pointer fields distinguish omitted values from explicit empty strings.
type UpdateManagedCredentialRequest struct {
	Label *string `json:"label,omitempty"`
}

// UpdateManagedCredentialUsernamePasswordRequest contains the fields for POST /managed/credentials/{credential_id}/update.
type UpdateManagedCredentialUsernamePasswordRequest struct {
	Password string  `json:"password"`
	Username *string `json:"username,omitempty"`
}

// ManagedSSHKey represents the Managed SSH public key assigned to an account.
type ManagedSSHKey struct {
	SSHKey string `json:"ssh_key"`
}

// CreateManagedCredentialRequest contains the fields for POST /managed/credentials.
type CreateManagedCredentialRequest struct {
	Label    string  `json:"label"`
	Password string  `json:"password"`
	Username *string `json:"username,omitempty"`
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

// MaintenancePolicy represents one available Linode maintenance policy.
type MaintenancePolicy struct {
	Slug                  string `json:"slug"`
	Label                 string `json:"label"`
	Description           string `json:"description"`
	Type                  string `json:"type"`
	NotificationPeriodSec int    `json:"notification_period_sec"`
	IsDefault             bool   `json:"is_default"`
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

// BetaProgram represents a beta program available for account enrollment.
type BetaProgram struct {
	BetaClass      string  `json:"class"`
	Description    *string `json:"description"`
	Ended          *string `json:"ended"`
	GreenlightOnly bool    `json:"greenlight_only"`
	ID             string  `json:"id"`
	Label          string  `json:"label"`
	MoreInfo       string  `json:"more_info"`
	Started        string  `json:"started"`
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

// CreateAccountUserRequest contains the fields for POST /account/users.
// Restricted is a pointer so an omitted value is distinguishable from an
// explicit false.
type CreateAccountUserRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Restricted *bool  `json:"restricted,omitempty"`
}

// UpdateAccountUserRequest contains editable fields for PUT /account/users/{username}.
// Pointer fields distinguish omitted values from explicit updates.
type UpdateAccountUserRequest struct {
	Email      *string   `json:"email,omitempty"`
	Restricted *bool     `json:"restricted,omitempty"`
	SSHKeys    *[]string `json:"ssh_keys,omitempty"`
	Username   *string   `json:"username,omitempty"`
}

// AccountUser represents one user returned by account user endpoints.
type AccountUser struct {
	Email               string                `json:"email"`
	LastLogin           *AccountUserLastLogin `json:"last_login"`
	PasswordCreated     *string               `json:"password_created"`
	Restricted          bool                  `json:"restricted"`
	SSHKeys             []string              `json:"ssh_keys"`
	TFAEnabled          bool                  `json:"tfa_enabled"`
	UserType            string                `json:"user_type"`
	Username            string                `json:"username"`
	VerifiedPhoneNumber *string               `json:"verified_phone_number"`
}

// AccountUserLastLogin contains the most recent login attempt for an account user.
type AccountUserLastLogin struct {
	LoginDatetime string `json:"login_datetime"`
	Status        string `json:"status"`
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

// CreateAccountPaymentRequest contains the request body for POST /account/payments.
type CreateAccountPaymentRequest struct {
	PaymentMethodID int    `json:"payment_method_id,omitempty"`
	USD             string `json:"usd,omitempty"`
}

// AddAccountPromoCreditRequest contains the request body for POST /account/promo-codes.
type AddAccountPromoCreditRequest struct {
	PromoCode string `json:"promo_code"`
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

// ProfileApp represents an OAuth app authorized for the current profile.
type ProfileApp struct {
	ID      int    `json:"id"`
	Label   string `json:"label"`
	Scopes  string `json:"scopes"`
	Website string `json:"website"`
}

// AuthorizedApp represents an OAuth app authorization for the authenticated profile.
type AuthorizedApp struct {
	ID           int     `json:"id"`
	Label        string  `json:"label"`
	Scopes       string  `json:"scopes"`
	Website      string  `json:"website"`
	Created      string  `json:"created"`
	Expiry       *string `json:"expiry"`
	ThumbnailURL *string `json:"thumbnail_url"`
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

// LongviewApps describes the application monitors enabled for a Longview client.
type LongviewApps struct {
	Apache bool `json:"apache"`
	MySQL  bool `json:"mysql"`
	Nginx  bool `json:"nginx"`
}

// LongviewClient represents a Longview client monitor.
type LongviewClient struct {
	Apps    LongviewApps `json:"apps"`
	Created string       `json:"created"`
	ID      int          `json:"id"`
	Label   string       `json:"label"`
	Updated string       `json:"updated"`
}

// UpdateLongviewClientRequest contains editable fields for PUT /v4/longview/clients/{clientId}.
type UpdateLongviewClientRequest struct {
	Label *string `json:"label,omitempty"`
}

// UpdateLongviewPlanRequest contains editable fields for PUT /v4/longview/plan.
type UpdateLongviewPlanRequest struct {
	LongviewSubscription string `json:"longview_subscription"`
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

// CreateAccountServiceTransferRequest contains the entities to transfer for POST /account/service-transfers.
type CreateAccountServiceTransferRequest struct {
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

// ProfilePreferences contains the authenticated user's dashboard preferences.
// The API can add preference keys over time, so keep the response map-backed.
type ProfilePreferences map[string]any

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
