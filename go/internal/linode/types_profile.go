package linode

// Profile represents a Linode user profile. Scopes is populated only for
// personal access tokens; OAuth tokens leave it empty, so scope validation
// falls back to /profile/grants for those.
type Profile struct {
	Username           string `json:"username"`
	Email              string `json:"email"`
	Timezone           string `json:"timezone"`
	Scopes             string `json:"scopes,omitempty"`
	UID                int    `json:"uid"`
	EmailNotifications bool   `json:"email_notifications"`
	Restricted         bool   `json:"restricted"`
	TwoFactorAuth      bool   `json:"two_factor_auth"`
}

// GrantPermission is one of "read_only", "read_write", or "" (no access).
// The API sends an explicit empty string for no permission. It stays a string
// rather than an enum so unknown future values round-trip.
type GrantPermission string

// Grant represents the permission an OAuth token has on a single resource
// instance within one /profile/grants category.
type Grant struct {
	Label       string          `json:"label"`
	Permissions GrantPermission `json:"permissions"`
	ID          int             `json:"id"`
}

// Grants represents the full /profile/grants response for OAuth tokens.
// PATs always return an empty Grants object; their scope information lives on
// Profile.Scopes instead, so the profile loader checks both.
type Grants struct {
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
	Global       GlobalGrants `json:"global"`
}

// GlobalGrants captures the account-level permission booleans returned in the
// global object of /profile/grants.
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
