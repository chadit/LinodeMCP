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
