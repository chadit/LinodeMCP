package linode

// Profile represents a Linode user profile.
type Profile struct {
	Username           string `json:"username"`
	Email              string `json:"email"`
	Timezone           string `json:"timezone"`
	UID                int    `json:"uid"`
	EmailNotifications bool   `json:"email_notifications"` //nolint:tagliatelle // Linode API snake_case
	Restricted         bool   `json:"restricted"`
	TwoFactorAuth      bool   `json:"two_factor_auth"` //nolint:tagliatelle // Linode API snake_case
}

// Account represents a Linode account.
type Account struct {
	FirstName         string   `json:"first_name"` //nolint:tagliatelle // Linode API snake_case
	LastName          string   `json:"last_name"`  //nolint:tagliatelle // Linode API snake_case
	Email             string   `json:"email"`
	Company           string   `json:"company"`
	Address1          string   `json:"address_1"` //nolint:tagliatelle // Linode API snake_case
	Address2          string   `json:"address_2"` //nolint:tagliatelle // Linode API snake_case
	City              string   `json:"city"`
	State             string   `json:"state"`
	Zip               string   `json:"zip"`
	Country           string   `json:"country"`
	Phone             string   `json:"phone"`
	Balance           float64  `json:"balance"`
	BalanceUninvoiced float64  `json:"balance_uninvoiced"` //nolint:tagliatelle // Linode API snake_case
	Capabilities      []string `json:"capabilities"`
	ActiveSince       string   `json:"active_since"` //nolint:tagliatelle // Linode API snake_case
	EUUID             string   `json:"euuid"`
	BillingSource     string   `json:"billing_source"`    //nolint:tagliatelle // Linode API snake_case
	ActivePromotions  []Promo  `json:"active_promotions"` //nolint:tagliatelle // Linode API snake_case
}

// Promo represents an active promotion on an account.
type Promo struct {
	Description              string `json:"description"`
	Summary                  string `json:"summary"`
	CreditMonthlyCap         string `json:"credit_monthly_cap"`          //nolint:tagliatelle // Linode API snake_case
	CreditRemaining          string `json:"credit_remaining"`            //nolint:tagliatelle // Linode API snake_case
	ExpireDT                 string `json:"expire_dt"`                   //nolint:tagliatelle // Linode API snake_case
	ImageURL                 string `json:"image_url"`                   //nolint:tagliatelle // Linode API snake_case
	ServiceType              string `json:"service_type"`                //nolint:tagliatelle // Linode API snake_case
	ThisMonthCreditRemaining string `json:"this_month_credit_remaining"` //nolint:tagliatelle // Linode API snake_case
}
