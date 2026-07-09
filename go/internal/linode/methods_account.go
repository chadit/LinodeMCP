package linode

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url" // path parameter escaping
	"strconv"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const (
	endpointProfile                      = "/profile"
	endpointProfilePreferences           = endpointProfile + "/preferences"
	endpointProfileTokens                = endpointProfile + "/tokens"
	endpointProfileTFAEnable             = endpointProfile + "/tfa-enable"
	endpointProfilePhoneNumber           = "/profile/phone-number"
	endpointProfilePhoneNumberVerify     = endpointProfilePhoneNumber + "/verify"
	endpointProfileGrants                = "/profile/grants"
	endpointProfileSecurityQuestions     = "/profile/security-questions"
	endpointProfileLogins                = "/profile/logins"
	endpointProfileApps                  = "/profile/apps"
	endpointProfileDevices               = "/profile/devices"
	endpointProfileTFADisable            = "/profile/tfa-disable"
	endpointProfileTFAEnableConfirm      = "/profile/tfa-enable-confirm"
	endpointAccount                      = "/account"
	endpointAccountTransfer              = "/account/transfer"
	endpointAccountSettings              = "/account/settings"
	endpointAccountSettingsManagedEnable = "/account/settings/managed-enable"
	endpointManagedCredentials           = "/managed/" + "credentials"
	endpointManagedCredentialsSSHKey     = endpointManagedCredentials + "/sshkey"
	endpointAccountCancel                = "/account/cancel"
	endpointAccountAgreements            = "/account/agreements"
	endpointAccountMaintenance           = "/account/maintenance"
	endpointMaintenancePolicies          = "/maintenance/policies"
	endpointAccountNotifications         = "/account/notifications"
	endpointAccountAvailability          = "/account/availability"
	endpointBetas                        = "/betas"
	endpointAccountBetas                 = "/account/betas"
	endpointAccountOAuthClients          = "/account/oauth-clients"
	endpointLongviewClients              = "/longview/clients"
	endpointLongviewPlan                 = "/longview/plan"
	endpointLongviewTypes                = "/longview/types"
	endpointLongviewSubscriptions        = "/longview/subscriptions"
	endpointMonitorAlertChannels         = "/monitor/alert-channels"
	endpointMonitorAlertDefinitions      = "/monitor/alert-definitions"
	endpointMonitorDashboards            = "/monitor/dashboards"
	endpointMonitorServices              = "/monitor/services"
	endpointAccountPaymentMethods        = "/account/payment-methods"
	endpointAccountEvents                = "/account/events"
	endpointAccountUsers                 = "/account/users"
	endpointAccountLogins                = "/account/logins"
	endpointAccountInvoices              = "/account/invoices"
	endpointAccountPayments              = "/account/payments"
	endpointAccountPromoCodes            = "/account/promo-codes"
	endpointAccountChildAccounts         = "/account/child-accounts"
	endpointAccountEntityTransfers       = "/account/entity-transfers"
	endpointAccountServiceTransfers      = "/account/service-transfers"
)

// GetProfile retrieves the authenticated user's profile from the Linode API.
func (c *Client) httpGetProfile(ctx context.Context) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointProfile, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfile", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var profile Profile
	if err := c.handleResponse(resp, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// httpGetProfileProto retrieves the profile as a proto message.
func (c *Client) httpGetProfileProto(ctx context.Context) (*linodev1.Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointProfile, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfile", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	profile := &linodev1.Profile{}
	if err := c.handleProtoResponse(resp, profile); err != nil {
		return nil, err
	}

	return profile, nil
}

// httpCreateProfileToken creates a personal access token for the authenticated profile.
func (c *Client) httpCreateProfileToken(ctx context.Context, req CreateProfileTokenRequest) (*ProfileToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointProfileTokens, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateProfileToken", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var token ProfileToken
	if err := c.handleResponse(resp, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// httpCreateProfileTokenProto creates a personal access token and decodes the
// created token (including the one-time secret) into a proto message for the
// proto-backed write path.
func (c *Client) httpCreateProfileTokenProto(ctx context.Context, req CreateProfileTokenRequest) (*linodev1.CreatedPersonalAccessToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointProfileTokens, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateProfileToken", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	token := &linodev1.CreatedPersonalAccessToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpUpdateProfilePreferences updates the authenticated user's preferences.
func (c *Client) httpUpdateProfilePreferences(ctx context.Context, req ProfilePreferences) (ProfilePreferences, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if req == nil {
		req = ProfilePreferences{}
	}

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointProfilePreferences, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateProfilePreferences", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var preferences ProfilePreferences
	if err := c.handleResponse(resp, &preferences); err != nil {
		return nil, err
	}

	return preferences, nil
}

// httpEnableProfileTFA generates a two-factor authentication secret for the authenticated profile.
func (c *Client) httpEnableProfileTFA(ctx context.Context) (ProfileTFAEnableResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointProfileTFAEnable, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "EnableProfileTFA", Err: err}
	}

	defer drainClose(resp)

	var result ProfileTFAEnableResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// httpEnableProfileTFAProto generates a two-factor authentication secret and
// decodes the {secret, expiry} body into a proto message for the proto-backed
// write path. The handler sets the one-time-secret warning; the API does not
// return it. The secret is returned to the user by design (it must be confirmed
// to activate two-factor auth), so it is not output-redacted.
func (c *Client) httpEnableProfileTFAProto(ctx context.Context) (*linodev1.ProfileTfaEnableResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointProfileTFAEnable, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "EnableProfileTFA", Err: err}
	}

	defer drainClose(resp)

	result := &linodev1.ProfileTfaEnableResponse{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
}

// httpSendProfilePhoneNumberVerificationCode sends a profile phone verification code.
func (c *Client) httpSendProfilePhoneNumberVerificationCode(ctx context.Context, req *ProfilePhoneNumberRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointProfilePhoneNumber, req)
	if err != nil {
		return &NetworkError{Operation: "SendProfilePhoneNumberVerificationCode", Err: err}
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpDeleteProfilePhoneNumber deletes the authenticated profile's phone number.
func (c *Client) httpDeleteProfilePhoneNumber(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpointProfilePhoneNumber, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteProfilePhoneNumber", Err: err}
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpVerifyProfilePhoneNumber verifies a profile phone number using an OTP code.
func (c *Client) httpVerifyProfilePhoneNumber(ctx context.Context, req *ProfilePhoneNumberVerifyRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointProfilePhoneNumberVerify, req)
	if err != nil {
		return &NetworkError{Operation: "VerifyProfilePhoneNumber", Err: err}
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpDisableProfileTFA disables two-factor authentication for the profile.
func (c *Client) httpDisableProfileTFA(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointProfileTFADisable, nil)
	if err != nil {
		return &NetworkError{Operation: "DisableProfileTFA", Err: err}
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpConfirmProfileTFAEnable confirms two-factor authentication enablement for the profile.
func (c *Client) httpConfirmProfileTFAEnable(ctx context.Context, req *ProfileTFAEnableConfirmRequest) (ProfileTFAEnableConfirmResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if req == nil {
		req = &ProfileTFAEnableConfirmRequest{}
	}

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointProfileTFAEnableConfirm, req)
	if err != nil {
		return nil, &NetworkError{Operation: "ConfirmProfileTFAEnable", Err: err}
	}

	defer drainClose(resp)

	var result ProfileTFAEnableConfirmResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// httpListProfileSecurityQuestions lists available profile security questions.
func (c *Client) httpListProfileSecurityQuestions(ctx context.Context) (*ProfileSecurityQuestions, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointProfileSecurityQuestions, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListProfileSecurityQuestions", Err: err}
	}

	defer drainClose(resp)

	var questions ProfileSecurityQuestions
	if err := c.handleResponse(resp, &questions); err != nil {
		return nil, err
	}

	return &questions, nil
}

// httpListProfileLogins retrieves login history for the authenticated profile.
func (c *Client) httpListProfileLogins(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountLogin], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointProfileLogins, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListProfileLogins", Err: err}
	}

	defer drainClose(resp)

	var logins PaginatedResponse[AccountLogin]
	if err := c.handleResponse(resp, &logins); err != nil {
		return nil, err
	}

	return &logins, nil
}

// httpListProfileLoginsProto retrieves profile login history as proto messages
// for the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListProfileLogins.
func (c *Client) httpListProfileLoginsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountLogin, error) {
	return listProtoElementsPaginated(ctx, c, "ListProfileLogins", endpointProfileLogins, page, pageSize,
		func() *linodev1.AccountLogin { return &linodev1.AccountLogin{} })
}

// httpListProfileTokens retrieves personal access tokens for the authenticated profile.
func (c *Client) httpListProfileTokens(ctx context.Context, page, pageSize int) (*PaginatedResponse[ProfileToken], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointProfileTokens, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListProfileTokens", Err: err}
	}

	defer drainClose(resp)

	var tokens PaginatedResponse[ProfileToken]
	if err := c.handleResponse(resp, &tokens); err != nil {
		return nil, err
	}

	return &tokens, nil
}

// httpDeleteProfileToken revokes a personal access token for the authenticated profile.
func (c *Client) httpDeleteProfileToken(ctx context.Context, tokenID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointProfileTokens + "/" + url.PathEscape(strconv.Itoa(tokenID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteProfileToken", Err: err}
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpUpdateProfileToken updates one personal access token on the authenticated profile.
func (c *Client) httpUpdateProfileToken(ctx context.Context, tokenID string, req UpdateProfileTokenRequest) (*ProfileToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if req == nil {
		req = UpdateProfileTokenRequest{}
	}

	endpoint := endpointProfileTokens + "/" + url.PathEscape(tokenID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateProfileToken", Err: err}
	}

	defer drainClose(resp)

	var token ProfileToken
	if err := c.handleResponse(resp, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// httpUpdateProfileTokenProto updates one personal access token and decodes the
// updated metadata into a proto message for the proto-backed write path. An
// update never returns the token secret, so the metadata element carries no
// secret field.
func (c *Client) httpUpdateProfileTokenProto(ctx context.Context, tokenID string, req UpdateProfileTokenRequest) (*linodev1.PersonalAccessToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if req == nil {
		req = UpdateProfileTokenRequest{}
	}

	endpoint := endpointProfileTokens + "/" + url.PathEscape(tokenID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateProfileToken", Err: err}
	}

	defer drainClose(resp)

	token := &linodev1.PersonalAccessToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpListProfileDevices retrieves trusted devices for the authenticated profile.
func (c *Client) httpListProfileDevices(ctx context.Context, page, pageSize int) (*PaginatedResponse[ProfileDevice], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointProfileDevices, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListProfileDevices", Err: err}
	}

	defer drainClose(resp)

	var devices PaginatedResponse[ProfileDevice]
	if err := c.handleResponse(resp, &devices); err != nil {
		return nil, err
	}

	return &devices, nil
}

// httpGetProfileLogin retrieves one login from the authenticated profile.
func (c *Client) httpGetProfileLogin(ctx context.Context, loginID int) (*AccountLogin, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointProfileLogins + "/" + url.PathEscape(strconv.Itoa(loginID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfileLogin", Err: err}
	}

	defer drainClose(resp)

	var login AccountLogin
	if err := c.handleResponse(resp, &login); err != nil {
		return nil, err
	}

	return &login, nil
}

// httpGetProfileLoginProto retrieves one profile login as a proto message (shared
// AccountLogin shape).
func (c *Client) httpGetProfileLoginProto(ctx context.Context, loginID int) (*linodev1.AccountLogin, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointProfileLogins + "/" + url.PathEscape(strconv.Itoa(loginID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfileLogin", Err: err}
	}

	defer drainClose(resp)

	login := &linodev1.AccountLogin{}
	if err := c.handleProtoResponse(resp, login); err != nil {
		return nil, err
	}

	return login, nil
}

// httpGetProfileApp retrieves one authorized OAuth app from the profile.
func (c *Client) httpGetProfileApp(ctx context.Context, appID int) (*ProfileApp, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointProfileApps + "/" + url.PathEscape(strconv.Itoa(appID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfileApp", Err: err}
	}

	defer drainClose(resp)

	var app ProfileApp
	if err := c.handleResponse(resp, &app); err != nil {
		return nil, err
	}

	return &app, nil
}

// httpGetProfileAppProto retrieves one authorized OAuth app as a proto message.
func (c *Client) httpGetProfileAppProto(ctx context.Context, appID int) (*linodev1.ProfileApp, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointProfileApps + "/" + url.PathEscape(strconv.Itoa(appID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfileApp", Err: err}
	}

	defer drainClose(resp)

	app := &linodev1.ProfileApp{}
	if err := c.handleProtoResponse(resp, app); err != nil {
		return nil, err
	}

	return app, nil
}

// httpDeleteProfileApp revokes access for one OAuth app authorized on the profile.
func (c *Client) httpDeleteProfileApp(ctx context.Context, appID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointProfileApps + "/" + url.PathEscape(strconv.Itoa(appID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteProfileApp", Err: err}
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpGetProfileDevice retrieves one trusted device from the profile.
func (c *Client) httpGetProfileDevice(ctx context.Context, deviceID int) (*ProfileDevice, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointProfileDevices + "/" + url.PathEscape(strconv.Itoa(deviceID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfileDevice", Err: err}
	}

	defer drainClose(resp)

	var device ProfileDevice
	if err := c.handleResponse(resp, &device); err != nil {
		return nil, err
	}

	return &device, nil
}

// httpGetProfileDeviceProto retrieves one trusted device from the profile and
// decodes it into the TrustedDevice proto element for the proto-backed read path.
func (c *Client) httpGetProfileDeviceProto(ctx context.Context, deviceID int) (*linodev1.TrustedDevice, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointProfileDevices + "/" + url.PathEscape(strconv.Itoa(deviceID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfileDevice", Err: err}
	}

	defer drainClose(resp)

	device := &linodev1.TrustedDevice{}
	if err := c.handleProtoResponse(resp, device); err != nil {
		return nil, err
	}

	return device, nil
}

// httpDeleteProfileDevice revokes one trusted device from the profile.
func (c *Client) httpDeleteProfileDevice(ctx context.Context, deviceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointProfileDevices + "/" + url.PathEscape(strconv.Itoa(deviceID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteProfileDevice", Err: err}
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpGetProfileGrants retrieves /profile/grants. Returns a Grants
// struct for OAuth tokens; PATs return an empty payload by design (the
// Linode API still returns 200 with zero-valued fields). Callers
// distinguish PAT vs OAuth by checking Profile.Scopes != "" first; this
// method does not need to know which token type the caller has.
func (c *Client) httpGetProfileGrants(ctx context.Context) (*Grants, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointProfileGrants, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfileGrants", Err: err}
	}

	defer drainClose(resp)

	var grants Grants
	if err := c.handleResponse(resp, &grants); err != nil {
		return nil, err
	}

	return &grants, nil
}

// httpGetProfileTokenProto retrieves one personal access token's metadata and
// decodes it into the PersonalAccessToken proto element for the proto-backed
// read path. The element models metadata only (id, label, scopes, created,
// expiry) and no secret field, so any token value the API returns is dropped by
// the DiscardUnknown decode.
func (c *Client) httpGetProfileTokenProto(ctx context.Context, tokenID int) (*linodev1.PersonalAccessToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointProfileTokens + "/" + url.PathEscape(strconv.Itoa(tokenID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfileToken", Err: err}
	}

	defer drainClose(resp)

	token := &linodev1.PersonalAccessToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpAnswerProfileSecurityQuestions answers the authenticated user's security questions via POST /v4/profile/security-questions.
func (c *Client) httpAnswerProfileSecurityQuestions(ctx context.Context, req *AnswerProfileSecurityQuestionsRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointProfileSecurityQuestions, req)
	if err != nil {
		return &NetworkError{Operation: "AnswerProfileSecurityQuestions", Err: err}
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpGetProfilePreferences retrieves /profile/preferences for the authenticated user.
func (c *Client) httpGetProfilePreferences(ctx context.Context) (*ProfilePreferences, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointProfilePreferences, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfilePreferences", Err: err}
	}

	defer drainClose(resp)

	preferences := ProfilePreferences{}
	if err := c.handleResponse(resp, &preferences); err != nil {
		return nil, err
	}

	return &preferences, nil
}

// httpListProfileApps retrieves OAuth app authorizations for the authenticated profile.
func (c *Client) httpListProfileApps(ctx context.Context, page, pageSize int) (*PaginatedResponse[AuthorizedApp], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointProfileApps, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListProfileApps", Err: err}
	}

	defer drainClose(resp)

	var apps PaginatedResponse[AuthorizedApp]
	if err := c.handleResponse(resp, &apps); err != nil {
		return nil, err
	}

	return &apps, nil
}

// httpListProfileAppsProto retrieves OAuth app authorizations as proto messages
// for the proto-backed list path. page/page_size flow through withPaginationQuery,
// so the request matches httpListProfileApps.
func (c *Client) httpListProfileAppsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ProfileApp, error) {
	return listProtoElementsPaginated(ctx, c, "ListProfileApps", endpointProfileApps, page, pageSize,
		func() *linodev1.ProfileApp { return &linodev1.ProfileApp{} })
}

// GetAccount retrieves the authenticated user's account information from the Linode API.
func (c *Client) httpGetAccount(ctx context.Context) (*Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccount, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccount", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var account Account
	if err := c.handleResponse(resp, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// httpGetAccountProto retrieves the account as a proto message.
func (c *Client) httpGetAccountProto(ctx context.Context) (*linodev1.Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccount, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccount", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	account := &linodev1.Account{}
	if err := c.handleProtoResponse(resp, account); err != nil {
		return nil, err
	}

	return account, nil
}

// httpUpdateAccountProto updates the account as a proto message.
func (c *Client) httpUpdateAccountProto(ctx context.Context, req *UpdateAccountRequest) (*linodev1.Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointAccount, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateAccount", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	account := &linodev1.Account{}
	if err := c.handleProtoResponse(resp, account); err != nil {
		return nil, err
	}

	return account, nil
}

// httpGetAccountTransfer retrieves the authenticated account's network transfer usage.
func (c *Client) httpGetAccountTransfer(ctx context.Context) (*AccountTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccountTransfer, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var transfer AccountTransfer
	if err := c.handleResponse(resp, &transfer); err != nil {
		return nil, err
	}

	return &transfer, nil
}

// httpGetAccountTransferProto retrieves account transfer usage as a proto message.
func (c *Client) httpGetAccountTransferProto(ctx context.Context) (*linodev1.AccountTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccountTransfer, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	transfer := &linodev1.AccountTransfer{}
	if err := c.handleProtoResponse(resp, transfer); err != nil {
		return nil, err
	}

	return transfer, nil
}

// httpGetAccountSettings retrieves account-wide settings from the Linode API.
func (c *Client) httpGetAccountSettings(ctx context.Context) (*AccountSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccountSettings, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountSettings", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var settings AccountSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// httpGetAccountSettingsProto retrieves the account settings as a proto message.
func (c *Client) httpGetAccountSettingsProto(ctx context.Context) (*linodev1.AccountSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccountSettings, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountSettings", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	settings := &linodev1.AccountSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// httpUpdateAccountSettings updates account-wide settings via PUT /v4/account/settings.
func (c *Client) httpUpdateAccountSettings(ctx context.Context, req *UpdateAccountSettingsRequest) (*AccountSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointAccountSettings, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateAccountSettings", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var settings AccountSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// httpUpdateAccountSettingsProto updates account settings and decodes the
// response into the proto AccountSettings element.
func (c *Client) httpUpdateAccountSettingsProto(ctx context.Context, req *UpdateAccountSettingsRequest) (*linodev1.AccountSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointAccountSettings, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateAccountSettings", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	settings := &linodev1.AccountSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// httpEnableAccountManaged enables Linode Managed for the account via POST /v4/account/settings/managed-enable.
func (c *Client) httpEnableAccountManaged(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountSettingsManagedEnable, nil)
	if err != nil {
		return &NetworkError{Operation: "EnableAccountManaged", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpListManagedCredentials retrieves stored managed credentials.
func (c *Client) httpListManagedCredentials(ctx context.Context, page, pageSize int) (*PaginatedResponse[ManagedCredential], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointManagedCredentials, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListManagedCredentials", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var credentials PaginatedResponse[ManagedCredential]
	if err := c.handleResponse(resp, &credentials); err != nil {
		return nil, err
	}

	return &credentials, nil
}

// httpListManagedCredentialsProto retrieves stored managed credentials as proto
// messages for the proto-backed list path. page/page_size flows through
// withPaginationQuery, so the request matches httpListManagedCredentials. The
// secret material is write-only and never present in the list body.
func (c *Client) httpListManagedCredentialsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedCredential, error) {
	return listProtoElementsPaginated(ctx, c, "ListManagedCredentials", endpointManagedCredentials, page, pageSize,
		func() *linodev1.ManagedCredential { return &linodev1.ManagedCredential{} })
}

// httpUpdateManagedCredential updates one stored Managed credential.
func (c *Client) httpUpdateManagedCredential(ctx context.Context, credentialID int, req UpdateManagedCredentialRequest) (*ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedCredentials + "/" + url.PathEscape(strconv.Itoa(credentialID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateManagedCredential", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var credential ManagedCredential
	if err := c.handleResponse(resp, &credential); err != nil {
		return nil, err
	}

	return &credential, nil
}

// httpUpdateManagedCredentialProto updates one stored Managed credential's label
// and decodes the response into the proto element so the write tool emits the
// same field set as the credential GET/LIST path.
func (c *Client) httpUpdateManagedCredentialProto(ctx context.Context, credentialID int, req UpdateManagedCredentialRequest) (*linodev1.ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedCredentials + "/" + url.PathEscape(strconv.Itoa(credentialID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateManagedCredential", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	credential := &linodev1.ManagedCredential{}
	if err := c.handleProtoResponse(resp, credential); err != nil {
		return nil, err
	}

	return credential, nil
}

// httpUpdateManagedCredentialUsernamePassword updates one stored Managed credential's username and password.
func (c *Client) httpUpdateManagedCredentialUsernamePassword(ctx context.Context, credentialID int, req *UpdateManagedCredentialUsernamePasswordRequest) (*ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedCredentials + "/" + url.PathEscape(strconv.Itoa(credentialID)) + "/update"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateManagedCredentialUsernamePassword", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var credential ManagedCredential
	if err := c.handleResponse(resp, &credential); err != nil {
		return nil, err
	}

	return &credential, nil
}

// httpGetManagedSSHKeyProto retrieves the Managed SSH public key assigned to the
// account and decodes it into the ManagedSSHKey proto element.
func (c *Client) httpGetManagedSSHKeyProto(ctx context.Context) (*linodev1.ManagedSSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointManagedCredentialsSSHKey, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetManagedSSHKey", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	sshKey := &linodev1.ManagedSSHKey{}
	if err := c.handleProtoResponse(resp, sshKey); err != nil {
		return nil, err
	}

	return sshKey, nil
}

// httpCreateManagedCredential creates a stored Managed credential.
func (c *Client) httpCreateManagedCredential(ctx context.Context, request *CreateManagedCredentialRequest) (*ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointManagedCredentials, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateManagedCredential", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var credential ManagedCredential
	if err := c.handleResponse(resp, &credential); err != nil {
		return nil, err
	}

	return &credential, nil
}

// httpCreateManagedCredentialProto creates a stored Managed credential and decodes
// the response into the proto element so the write tool emits the same field set
// as the credential GET/LIST path (the secret is never echoed).
func (c *Client) httpCreateManagedCredentialProto(ctx context.Context, request *CreateManagedCredentialRequest) (*linodev1.ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointManagedCredentials, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateManagedCredential", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	credential := &linodev1.ManagedCredential{}
	if err := c.handleProtoResponse(resp, credential); err != nil {
		return nil, err
	}

	return credential, nil
}

// httpGetManagedCredential retrieves one stored managed credential.
func (c *Client) httpGetManagedCredential(ctx context.Context, credentialID int) (*ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedCredentials + "/" + url.PathEscape(strconv.Itoa(credentialID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetManagedCredential", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var credential ManagedCredential
	if err := c.handleResponse(resp, &credential); err != nil {
		return nil, err
	}

	return &credential, nil
}

// httpGetManagedCredentialProto retrieves a Managed credential as a proto message.
func (c *Client) httpGetManagedCredentialProto(ctx context.Context, credentialID int) (*linodev1.ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedCredentials + "/" + url.PathEscape(strconv.Itoa(credentialID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetManagedCredential", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	credential := &linodev1.ManagedCredential{}
	if err := c.handleProtoResponse(resp, credential); err != nil {
		return nil, err
	}

	return credential, nil
}

// httpRevokeManagedCredential revokes one stored managed credential.
func (c *Client) httpRevokeManagedCredential(ctx context.Context, credentialID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointManagedCredentials + "/" + url.PathEscape(strconv.Itoa(credentialID)) + "/revoke"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "RevokeManagedCredential", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpGetAccountAgreements retrieves account agreement acknowledgment status.
func (c *Client) httpGetAccountAgreements(ctx context.Context) (*AccountAgreements, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccountAgreements, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountAgreements", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var agreements AccountAgreements
	if err := c.handleResponse(resp, &agreements); err != nil {
		return nil, err
	}

	return &agreements, nil
}

// httpGetAccountAgreementsProto retrieves account agreement acknowledgment status
// as a proto message. The endpoint returns a flat object of bool flags, decoded
// into AccountAgreements.
func (c *Client) httpGetAccountAgreementsProto(ctx context.Context) (*linodev1.AccountAgreements, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccountAgreements, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountAgreements", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	agreements := &linodev1.AccountAgreements{}
	if err := c.handleProtoResponse(resp, agreements); err != nil {
		return nil, err
	}

	return agreements, nil
}

// httpListAccountMaintenance retrieves account maintenance records.
func (c *Client) httpListAccountMaintenance(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountMaintenance], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountMaintenance, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountMaintenance", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var maintenance PaginatedResponse[AccountMaintenance]
	if err := c.handleResponse(resp, &maintenance); err != nil {
		return nil, err
	}

	return &maintenance, nil
}

// httpListAccountMaintenanceProto retrieves account maintenance records as proto
// messages for the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListAccountMaintenance.
func (c *Client) httpListAccountMaintenanceProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountMaintenance, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountMaintenance", endpointAccountMaintenance, page, pageSize,
		func() *linodev1.AccountMaintenance { return &linodev1.AccountMaintenance{} })
}

// httpListMaintenancePolicies retrieves available Linode maintenance policies.
func (c *Client) httpListMaintenancePolicies(ctx context.Context, page, pageSize int) (*PaginatedResponse[MaintenancePolicy], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointMaintenancePolicies, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListMaintenancePolicies", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var policies PaginatedResponse[MaintenancePolicy]
	if err := c.handleResponse(resp, &policies); err != nil {
		return nil, err
	}

	return &policies, nil
}

// httpListMaintenancePoliciesProto retrieves maintenance policies as proto
// messages for the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListMaintenancePolicies.
func (c *Client) httpListMaintenancePoliciesProto(ctx context.Context, page, pageSize int) ([]*linodev1.MaintenancePolicy, error) {
	return listProtoElementsPaginated(ctx, c, "ListMaintenancePolicies", endpointMaintenancePolicies, page, pageSize,
		func() *linodev1.MaintenancePolicy { return &linodev1.MaintenancePolicy{} })
}

// httpListAccountNotifications retrieves account notifications.
func (c *Client) httpListAccountNotifications(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountNotification], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountNotifications, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountNotifications", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var notifications PaginatedResponse[AccountNotification]
	if err := c.handleResponse(resp, &notifications); err != nil {
		return nil, err
	}

	return &notifications, nil
}

// httpListAccountNotificationsProto retrieves account notifications as proto
// messages for the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListAccountNotifications.
func (c *Client) httpListAccountNotificationsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountNotification, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountNotifications", endpointAccountNotifications, page, pageSize,
		func() *linodev1.AccountNotification { return &linodev1.AccountNotification{} })
}

// httpListProfileDevicesProto retrieves trusted devices as proto messages for the
// proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListProfileDevices.
func (c *Client) httpListProfileDevicesProto(ctx context.Context, page, pageSize int) ([]*linodev1.TrustedDevice, error) {
	return listProtoElementsPaginated(ctx, c, "ListProfileDevices", endpointProfileDevices, page, pageSize,
		func() *linodev1.TrustedDevice { return &linodev1.TrustedDevice{} })
}

// httpListProfileTokensProto retrieves personal access token metadata as proto
// messages for the proto-backed list path. The proto PersonalAccessToken models
// no secret field, so any token value the API returns is dropped on decode and
// the list never leaks a token. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListProfileTokens.
func (c *Client) httpListProfileTokensProto(ctx context.Context, page, pageSize int) ([]*linodev1.PersonalAccessToken, error) {
	return listProtoElementsPaginated(ctx, c, "ListProfileTokens", endpointProfileTokens, page, pageSize,
		func() *linodev1.PersonalAccessToken { return &linodev1.PersonalAccessToken{} })
}

// httpListProfileSecurityQuestionsProto retrieves the profile security questions
// as proto messages. The endpoint is not paginated and wraps its elements under
// "security_questions" rather than the usual {data} page envelope, so this reads
// that key via listProtoElementsKeyed.
func (c *Client) httpListProfileSecurityQuestionsProto(ctx context.Context) ([]*linodev1.SecurityQuestion, error) {
	return listProtoElementsKeyed(ctx, c, "ListProfileSecurityQuestions", endpointProfileSecurityQuestions, "security_questions",
		func() *linodev1.SecurityQuestion { return &linodev1.SecurityQuestion{} })
}

// httpListAccountAvailability retrieves account service availability by region.
func (c *Client) httpGetAccountAvailability(ctx context.Context, regionID string) (*AccountAvailability, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountAvailability + "/" + url.PathEscape(regionID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountAvailability", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var availability AccountAvailability
	if err := c.handleResponse(resp, &availability); err != nil {
		return nil, err
	}

	return &availability, nil
}

// httpGetAccountAvailabilityProto retrieves one region's account availability as a
// proto message.
func (c *Client) httpGetAccountAvailabilityProto(ctx context.Context, regionID string) (*linodev1.AccountAvailability, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountAvailability + "/" + url.PathEscape(regionID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountAvailability", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	availability := &linodev1.AccountAvailability{}
	if err := c.handleProtoResponse(resp, availability); err != nil {
		return nil, err
	}

	return availability, nil
}

func (c *Client) httpListAccountAvailability(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountAvailability], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountAvailability, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountAvailability", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var availability PaginatedResponse[AccountAvailability]
	if err := c.handleResponse(resp, &availability); err != nil {
		return nil, err
	}

	return &availability, nil
}

// httpListAccountAvailabilityProto retrieves account service availability as
// proto messages for the proto-backed list path. The page/page_size pair flows
// through withPaginationQuery, so the request matches httpListAccountAvailability.
func (c *Client) httpListAccountAvailabilityProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountAvailability, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountAvailability", endpointAccountAvailability, page, pageSize,
		func() *linodev1.AccountAvailability { return &linodev1.AccountAvailability{} })
}

// httpListBetas retrieves available beta programs.
func (c *Client) httpListBetas(ctx context.Context, page, pageSize int) (*PaginatedResponse[BetaProgram], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointBetas, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListBetas", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var betas PaginatedResponse[BetaProgram]
	if err := c.handleResponse(resp, &betas); err != nil {
		return nil, err
	}

	return &betas, nil
}

// httpListBetasProto retrieves available beta programs as proto messages for the
// proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListBetas.
func (c *Client) httpListBetasProto(ctx context.Context, page, pageSize int) ([]*linodev1.BetaProgram, error) {
	return listProtoElementsPaginated(ctx, c, "ListBetas", endpointBetas, page, pageSize,
		func() *linodev1.BetaProgram { return &linodev1.BetaProgram{} })
}

// httpGetBeta retrieves one available beta program.
func (c *Client) httpGetBeta(ctx context.Context, betaID string) (*BetaProgram, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointBetas + "/" + url.PathEscape(betaID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetBeta", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var beta BetaProgram
	if err := c.handleResponse(resp, &beta); err != nil {
		return nil, err
	}

	return &beta, nil
}

// httpGetBetaProto retrieves one available beta program as a proto message.
func (c *Client) httpGetBetaProto(ctx context.Context, betaID string) (*linodev1.BetaProgram, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointBetas + "/" + url.PathEscape(betaID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetBeta", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	beta := &linodev1.BetaProgram{}
	if err := c.handleProtoResponse(resp, beta); err != nil {
		return nil, err
	}

	return beta, nil
}

// httpListAccountBetas retrieves enrolled account beta programs.
func (c *Client) httpListAccountBetas(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountBetaProgram], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountBetas, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountBetas", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var betas PaginatedResponse[AccountBetaProgram]
	if err := c.handleResponse(resp, &betas); err != nil {
		return nil, err
	}

	return &betas, nil
}

// httpListAccountBetasProto retrieves enrolled account beta programs as proto
// messages for the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListAccountBetas.
func (c *Client) httpListAccountBetasProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountBetaProgram, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountBetas", endpointAccountBetas, page, pageSize,
		func() *linodev1.AccountBetaProgram { return &linodev1.AccountBetaProgram{} })
}

// httpListAccountOAuthClients retrieves OAuth clients for the account.
func (c *Client) httpListAccountOAuthClients(ctx context.Context, page, pageSize int) (*PaginatedResponse[OAuthClient], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountOAuthClients, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountOAuthClients", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var clients PaginatedResponse[OAuthClient]
	if err := c.handleResponse(resp, &clients); err != nil {
		return nil, err
	}

	return &clients, nil
}

// httpListAccountOAuthClientsProto retrieves OAuth clients for the account as
// proto messages for the proto-backed list path. The page/page_size pair flows
// through withPaginationQuery, so the request matches httpListAccountOAuthClients.
func (c *Client) httpListAccountOAuthClientsProto(ctx context.Context, page, pageSize int) ([]*linodev1.OAuthClient, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountOAuthClients", endpointAccountOAuthClients, page, pageSize,
		func() *linodev1.OAuthClient { return &linodev1.OAuthClient{} })
}

// httpListLongviewClients retrieves Longview clients for the account.
func (c *Client) httpListLongviewClients(ctx context.Context, page, pageSize int) (*PaginatedResponse[LongviewClient], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointLongviewClients, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListLongviewClients", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var clients PaginatedResponse[LongviewClient]
	if err := c.handleResponse(resp, &clients); err != nil {
		return nil, err
	}

	return &clients, nil
}

// httpListLongviewClientsProto retrieves Longview clients as proto messages for
// the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListLongviewClients.
func (c *Client) httpListLongviewClientsProto(ctx context.Context, page, pageSize int) ([]*linodev1.LongviewClient, error) {
	return listProtoElementsPaginated(ctx, c, "ListLongviewClients", endpointLongviewClients, page, pageSize,
		func() *linodev1.LongviewClient { return &linodev1.LongviewClient{} })
}

// httpUpdateLongviewClient updates one Longview client's editable fields.
func (c *Client) httpUpdateLongviewClient(ctx context.Context, clientID int, req *UpdateLongviewClientRequest) (*LongviewClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointLongviewClients + "/" + url.PathEscape(strconv.Itoa(clientID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateLongviewClient", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var client LongviewClient
	if err := c.handleResponse(resp, &client); err != nil {
		return nil, err
	}

	return &client, nil
}

// UpdateLongviewClientProto updates one Longview client and returns the proto
// LongviewClient metadata element.
func (c *Client) UpdateLongviewClientProto(ctx context.Context, clientID int, req *UpdateLongviewClientRequest) (*linodev1.LongviewClient, error) {
	return c.httpUpdateLongviewClientProto(ctx, clientID, req)
}

// httpUpdateLongviewClientProto updates one Longview client and decodes the
// response into the proto LongviewClient element.
func (c *Client) httpUpdateLongviewClientProto(ctx context.Context, clientID int, req *UpdateLongviewClientRequest) (*linodev1.LongviewClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointLongviewClients + "/" + url.PathEscape(strconv.Itoa(clientID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateLongviewClient", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	client := &linodev1.LongviewClient{}
	if err := c.handleProtoResponse(resp, client); err != nil {
		return nil, err
	}

	return client, nil
}

// httpDeleteLongviewClient deletes one Longview client.
func (c *Client) httpDeleteLongviewClient(ctx context.Context, clientID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointLongviewClients + "/" + url.PathEscape(strconv.Itoa(clientID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteLongviewClient", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpListAccountPaymentMethods retrieves payment methods for the account.
func (c *Client) httpListAccountPaymentMethods(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountPaymentMethod], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountPaymentMethods, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountPaymentMethods", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var methods PaginatedResponse[AccountPaymentMethod]
	if err := c.handleResponse(resp, &methods); err != nil {
		return nil, err
	}

	return &methods, nil
}

// httpGetAccountPaymentMethod retrieves one payment method by ID.
func (c *Client) httpGetAccountPaymentMethod(ctx context.Context, paymentMethodID string) (*AccountPaymentMethod, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountPaymentMethods + "/" + url.PathEscape(paymentMethodID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountPaymentMethod", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var method AccountPaymentMethod
	if err := c.handleResponse(resp, &method); err != nil {
		return nil, err
	}

	return &method, nil
}

// httpGetAccountPaymentMethodProto retrieves one account payment method and
// decodes it into the proto AccountPaymentMethod element for the proto-backed
// read path. The polymorphic data object rides through the element's Struct field.
func (c *Client) httpGetAccountPaymentMethodProto(ctx context.Context, paymentMethodID string) (*linodev1.AccountPaymentMethod, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountPaymentMethods + "/" + url.PathEscape(paymentMethodID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountPaymentMethod", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	method := &linodev1.AccountPaymentMethod{}
	if err := c.handleProtoResponse(resp, method); err != nil {
		return nil, err
	}

	return method, nil
}

// httpCreateAccountPaymentMethod adds a payment method to the account.
func (c *Client) httpCreateAccountPaymentMethod(ctx context.Context, req *CreateAccountPaymentMethodRequest) (*AccountPaymentMethod, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountPaymentMethods, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountPaymentMethod", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var method AccountPaymentMethod
	if err := c.handleResponse(resp, &method); err != nil {
		return nil, err
	}

	return &method, nil
}

// CreateAccountPaymentMethodProto adds a payment method and returns the proto
// AccountPaymentMethod element.
func (c *Client) CreateAccountPaymentMethodProto(ctx context.Context, req *CreateAccountPaymentMethodRequest) (*linodev1.AccountPaymentMethod, error) {
	return c.httpCreateAccountPaymentMethodProto(ctx, req)
}

// httpCreateAccountPaymentMethodProto adds a payment method and decodes the
// response into the proto AccountPaymentMethod element.
func (c *Client) httpCreateAccountPaymentMethodProto(ctx context.Context, req *CreateAccountPaymentMethodRequest) (*linodev1.AccountPaymentMethod, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountPaymentMethods, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountPaymentMethod", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	method := &linodev1.AccountPaymentMethod{}
	if err := c.handleProtoResponse(resp, method); err != nil {
		return nil, err
	}

	return method, nil
}

// httpDeleteAccountPaymentMethod deletes one payment method by ID.
func (c *Client) httpDeleteAccountPaymentMethod(ctx context.Context, paymentMethodID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountPaymentMethods + "/" + url.PathEscape(paymentMethodID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteAccountPaymentMethod", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

func (c *Client) httpMakeAccountPaymentMethodDefault(ctx context.Context, paymentMethodID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountPaymentMethods + "/" + url.PathEscape(paymentMethodID) + "/make-default"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "MakeAccountPaymentMethodDefault", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpGetAccountOAuthClient retrieves one OAuth client by ID.
func (c *Client) httpGetAccountOAuthClient(ctx context.Context, clientID string) (*OAuthClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountOAuthClients + "/" + url.PathEscape(clientID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountOAuthClient", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var client OAuthClient
	if err := c.handleResponse(resp, &client); err != nil {
		return nil, err
	}

	return &client, nil
}

// httpGetAccountOAuthClientProto retrieves one OAuth client as a proto message.
func (c *Client) httpGetAccountOAuthClientProto(ctx context.Context, clientID string) (*linodev1.OAuthClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountOAuthClients + "/" + url.PathEscape(clientID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountOAuthClient", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	oauthClient := &linodev1.OAuthClient{}
	if err := c.handleProtoResponse(resp, oauthClient); err != nil {
		return nil, err
	}

	return oauthClient, nil
}

// httpUpdateOAuthClient updates one OAuth client by ID.
func (c *Client) httpUpdateOAuthClient(ctx context.Context, clientID string, req *UpdateOAuthClientRequest) (*OAuthClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountOAuthClients + "/" + url.PathEscape(clientID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateOAuthClient", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var client OAuthClient
	if err := c.handleResponse(resp, &client); err != nil {
		return nil, err
	}

	return &client, nil
}

// httpUpdateOAuthClientProto updates one OAuth client and decodes the response
// into the proto OAuthClient element (the metadata element, no secret).
func (c *Client) httpUpdateOAuthClientProto(ctx context.Context, clientID string, req *UpdateOAuthClientRequest) (*linodev1.OAuthClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountOAuthClients + "/" + url.PathEscape(clientID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateOAuthClient", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	client := &linodev1.OAuthClient{}
	if err := c.handleProtoResponse(resp, client); err != nil {
		return nil, err
	}

	return client, nil
}

// httpUpdateOAuthClientThumbnail updates one OAuth client's thumbnail by ID.
func (c *Client) httpUpdateOAuthClientThumbnail(ctx context.Context, clientID string, thumbnailPNG []byte) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountOAuthClients + "/" + url.PathEscape(clientID) + "/thumbnail"

	resp, err := c.makeRequestWithContentType(ctx, http.MethodPut, endpoint, bytes.NewReader(thumbnailPNG), contentTypePNG)
	if err != nil {
		return &NetworkError{Operation: "UpdateOAuthClientThumbnail", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpGetOAuthClientThumbnail retrieves one OAuth client's thumbnail by ID as raw PNG bytes.
func (c *Client) httpGetOAuthClientThumbnail(ctx context.Context, clientID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountOAuthClients + "/" + url.PathEscape(clientID) + "/thumbnail"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetOAuthClientThumbnail", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	// Read the body first since handleResponse would consume it
	thumbnailPNG, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &NetworkError{Operation: "GetOAuthClientThumbnail", Err: err}
	}

	// Check for error status codes after reading the body
	if resp.StatusCode >= http.StatusBadRequest {
		apiErr := c.handleErrorResponse(resp.StatusCode, thumbnailPNG, resp)

		// Stamp the request method onto the API error so the retry layer can
		// decide whether a 5xx is safe to replay.
		if typedErr, ok := errors.AsType[*APIError](apiErr); ok && resp.Request != nil {
			typedErr.Method = resp.Request.Method
		}

		return nil, apiErr
	}

	return thumbnailPNG, nil
}

// httpDeleteAccountOAuthClient deletes one OAuth client by ID.
func (c *Client) httpDeleteAccountOAuthClient(ctx context.Context, clientID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountOAuthClients + "/" + url.PathEscape(clientID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteAccountOAuthClient", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpResetOAuthClientSecret resets one OAuth client secret by ID.
func (c *Client) httpResetOAuthClientSecret(ctx context.Context, clientID string) (*OAuthClientSecret, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountOAuthClients + "/" + url.PathEscape(clientID) + "/reset-secret"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ResetOAuthClientSecret", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var secret OAuthClientSecret
	if err := c.handleResponse(resp, &secret); err != nil {
		return nil, err
	}

	return &secret, nil
}

// httpResetOAuthClientSecretProto resets an OAuth client secret and decodes the
// response into the proto OAuthClientSecret element.
func (c *Client) httpResetOAuthClientSecretProto(ctx context.Context, clientID string) (*linodev1.OAuthClientSecret, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountOAuthClients + "/" + url.PathEscape(clientID) + "/reset-secret"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ResetOAuthClientSecret", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	secret := &linodev1.OAuthClientSecret{}
	if err := c.handleProtoResponse(resp, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

// httpListAccountEvents retrieves account events.
func (c *Client) httpListAccountEvents(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountEvent], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountEvents, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountEvents", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var events PaginatedResponse[AccountEvent]
	if err := c.handleResponse(resp, &events); err != nil {
		return nil, err
	}

	return &events, nil
}

// httpListAccountEventsProto retrieves account events as proto messages for the
// proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListAccountEvents.
func (c *Client) httpListAccountEventsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountEvent, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountEvents", endpointAccountEvents, page, pageSize,
		func() *linodev1.AccountEvent { return &linodev1.AccountEvent{} })
}

// httpListAccountUsers retrieves users for the account.
func (c *Client) httpListAccountUsers(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountUser], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountUsers, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountUsers", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var users PaginatedResponse[AccountUser]
	if err := c.handleResponse(resp, &users); err != nil {
		return nil, err
	}

	return &users, nil
}

// httpListAccountUsersProto retrieves account users as proto messages for the
// proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListAccountUsers.
func (c *Client) httpListAccountUsersProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountUser, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountUsers", endpointAccountUsers, page, pageSize,
		func() *linodev1.AccountUser { return &linodev1.AccountUser{} })
}

// httpGetAccountUser retrieves one account user by username.
func (c *Client) httpGetAccountUser(ctx context.Context, username string) (*AccountUser, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountUsers + "/" + url.PathEscape(username)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountUser", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var user AccountUser
	if err := c.handleResponse(resp, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// httpGetAccountUserProto retrieves one account user as a proto message.
func (c *Client) httpGetAccountUserProto(ctx context.Context, username string) (*linodev1.AccountUser, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountUsers + "/" + url.PathEscape(username)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountUser", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	user := &linodev1.AccountUser{}
	if err := c.handleProtoResponse(resp, user); err != nil {
		return nil, err
	}

	return user, nil
}

// httpGetAccountUserGrants retrieves one account user's grants by username.
func (c *Client) httpGetAccountUserGrants(ctx context.Context, username string) (*Grants, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountUsers + "/" + url.PathEscape(username) + "/grants"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountUserGrants", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var grants Grants
	if err := c.handleResponse(resp, &grants); err != nil {
		return nil, err
	}

	return &grants, nil
}

// httpGetAccountUserGrantsProto retrieves one account user's grants and decodes
// them into the proto AccountUserGrants element for the proto-backed read path.
// The API omits grant sections the user has none of; protojson leaves those
// repeated fields empty so the canonical output normalizes them to [].
func (c *Client) httpGetAccountUserGrantsProto(ctx context.Context, username string) (*linodev1.AccountUserGrants, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountUsers + "/" + url.PathEscape(username) + "/grants"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountUserGrants", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	grants := &linodev1.AccountUserGrants{}
	if err := c.handleProtoResponse(resp, grants); err != nil {
		return nil, err
	}

	return grants, nil
}

// httpUpdateAccountUserGrants updates one account user's grants by username.
func (c *Client) httpUpdateAccountUserGrants(ctx context.Context, username string, request *UpdateAccountUserGrantsRequest) (*Grants, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountUsers + "/" + url.PathEscape(username) + "/grants"

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateAccountUserGrants", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var grants Grants
	if err := c.handleResponse(resp, &grants); err != nil {
		return nil, err
	}

	return &grants, nil
}

// httpUpdateAccountUser updates one account user by username.
func (c *Client) httpUpdateAccountUser(ctx context.Context, username string, request *UpdateAccountUserRequest) (*AccountUser, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountUsers + "/" + url.PathEscape(username)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateAccountUser", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var user AccountUser
	if err := c.handleResponse(resp, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// httpCreateAccountUserProto creates a user and decodes the response into the
// proto AccountUser element.
func (c *Client) httpCreateAccountUserProto(ctx context.Context, request *CreateAccountUserRequest) (*linodev1.AccountUser, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountUsers, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountUser", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	user := &linodev1.AccountUser{}
	if err := c.handleProtoResponse(resp, user); err != nil {
		return nil, err
	}

	return user, nil
}

// httpUpdateAccountUserProto updates one account user by username and decodes the
// response into the proto AccountUser element.
func (c *Client) httpUpdateAccountUserProto(ctx context.Context, username string, request *UpdateAccountUserRequest) (*linodev1.AccountUser, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountUsers + "/" + url.PathEscape(username)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateAccountUser", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	user := &linodev1.AccountUser{}
	if err := c.handleProtoResponse(resp, user); err != nil {
		return nil, err
	}

	return user, nil
}

// httpUpdateAccountUserGrantsProto updates one account user's grants and decodes
// the response into the proto AccountUserGrants element.
func (c *Client) httpUpdateAccountUserGrantsProto(ctx context.Context, username string, request *UpdateAccountUserGrantsRequest) (*linodev1.AccountUserGrants, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountUsers + "/" + url.PathEscape(username) + "/grants"

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateAccountUserGrants", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	grants := &linodev1.AccountUserGrants{}
	if err := c.handleProtoResponse(resp, grants); err != nil {
		return nil, err
	}

	return grants, nil
}

// httpDeleteAccountUser deletes one account user by username.
func (c *Client) httpDeleteAccountUser(ctx context.Context, username string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountUsers + "/" + url.PathEscape(username)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteAccountUser", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpCreateAccountUser creates a user on the account.
func (c *Client) httpCreateAccountUser(ctx context.Context, request *CreateAccountUserRequest) (*AccountUser, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountUsers, request)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountUser", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var user AccountUser
	if err := c.handleResponse(resp, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// httpListAccountLogins retrieves user logins for the account.
func (c *Client) httpListAccountLogins(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountLogin], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountLogins, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountLogins", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var logins PaginatedResponse[AccountLogin]
	if err := c.handleResponse(resp, &logins); err != nil {
		return nil, err
	}

	return &logins, nil
}

// httpListAccountLoginsProto retrieves account logins as proto messages for the
// proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListAccountLogins.
func (c *Client) httpListAccountLoginsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountLogin, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountLogins", endpointAccountLogins, page, pageSize,
		func() *linodev1.AccountLogin { return &linodev1.AccountLogin{} })
}

// httpGetAccountLogin retrieves one account login by ID.
func (c *Client) httpGetAccountLogin(ctx context.Context, loginID int) (*AccountLogin, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountLogins + "/" + url.PathEscape(strconv.Itoa(loginID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountLogin", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var login AccountLogin
	if err := c.handleResponse(resp, &login); err != nil {
		return nil, err
	}

	return &login, nil
}

// httpGetAccountLoginProto retrieves one account login as a proto message.
func (c *Client) httpGetAccountLoginProto(ctx context.Context, loginID int) (*linodev1.AccountLogin, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountLogins + "/" + url.PathEscape(strconv.Itoa(loginID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountLogin", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	login := &linodev1.AccountLogin{}
	if err := c.handleProtoResponse(resp, login); err != nil {
		return nil, err
	}

	return login, nil
}

func (c *Client) httpListAccountInvoices(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountInvoice], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountInvoices, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountInvoices", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var invoices PaginatedResponse[AccountInvoice]
	if err := c.handleResponse(resp, &invoices); err != nil {
		return nil, err
	}

	return &invoices, nil
}

// httpListAccountInvoicesProto retrieves account invoices as proto messages for
// the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListAccountInvoices.
func (c *Client) httpListAccountInvoicesProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountInvoice, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountInvoices", endpointAccountInvoices, page, pageSize,
		func() *linodev1.AccountInvoice { return &linodev1.AccountInvoice{} })
}

// httpListAccountPayments retrieves account payments.
func (c *Client) httpListAccountPayments(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountPayment], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountPayments, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountPayments", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var payments PaginatedResponse[AccountPayment]
	if err := c.handleResponse(resp, &payments); err != nil {
		return nil, err
	}

	return &payments, nil
}

// httpListAccountPaymentsProto retrieves account payments as proto messages for
// the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListAccountPayments.
func (c *Client) httpListAccountPaymentsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountPayment, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountPayments", endpointAccountPayments, page, pageSize,
		func() *linodev1.AccountPayment { return &linodev1.AccountPayment{} })
}

// httpGetAccountPayment retrieves one account payment by ID.
func (c *Client) httpGetAccountPayment(ctx context.Context, paymentID int) (*AccountPayment, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountPayments + "/" + url.PathEscape(strconv.Itoa(paymentID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountPayment", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var payment AccountPayment
	if err := c.handleResponse(resp, &payment); err != nil {
		return nil, err
	}

	return &payment, nil
}

// httpGetAccountPaymentProto retrieves one account payment as a proto message.
func (c *Client) httpGetAccountPaymentProto(ctx context.Context, paymentID int) (*linodev1.AccountPayment, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountPayments + "/" + url.PathEscape(strconv.Itoa(paymentID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountPayment", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	payment := &linodev1.AccountPayment{}
	if err := c.handleProtoResponse(resp, payment); err != nil {
		return nil, err
	}

	return payment, nil
}

// httpCreateAccountPayment makes a payment on the account.
func (c *Client) httpCreateAccountPayment(ctx context.Context, req *CreateAccountPaymentRequest) (*AccountPayment, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountPayments, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountPayment", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var payment AccountPayment
	if err := c.handleResponse(resp, &payment); err != nil {
		return nil, err
	}

	return &payment, nil
}

// CreateAccountPaymentProto makes an account payment and returns the proto
// AccountPayment element.
func (c *Client) CreateAccountPaymentProto(ctx context.Context, req *CreateAccountPaymentRequest) (*linodev1.AccountPayment, error) {
	return c.httpCreateAccountPaymentProto(ctx, req)
}

// httpCreateAccountPaymentProto makes an account payment and decodes the
// response into the proto AccountPayment element.
func (c *Client) httpCreateAccountPaymentProto(ctx context.Context, req *CreateAccountPaymentRequest) (*linodev1.AccountPayment, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountPayments, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountPayment", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	payment := &linodev1.AccountPayment{}
	if err := c.handleProtoResponse(resp, payment); err != nil {
		return nil, err
	}

	return payment, nil
}

// httpGetAccountInvoice retrieves one account invoice by ID.
func (c *Client) httpGetAccountInvoice(ctx context.Context, invoiceID int) (*AccountInvoice, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountInvoices + "/" + strconv.Itoa(invoiceID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountInvoice", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var invoice AccountInvoice
	if err := c.handleResponse(resp, &invoice); err != nil {
		return nil, err
	}

	return &invoice, nil
}

// httpGetAccountInvoiceProto retrieves one account invoice as a proto message.
func (c *Client) httpGetAccountInvoiceProto(ctx context.Context, invoiceID int) (*linodev1.AccountInvoice, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountInvoices + "/" + strconv.Itoa(invoiceID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountInvoice", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	invoice := &linodev1.AccountInvoice{}
	if err := c.handleProtoResponse(resp, invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

// httpListAccountInvoiceItems retrieves items for one account invoice.
func (c *Client) httpListAccountInvoiceItems(ctx context.Context, invoiceID, page, pageSize int) (*PaginatedResponse[AccountInvoiceItem], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountInvoices+"/"+strconv.Itoa(invoiceID)+"/items", page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountInvoiceItems", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var items PaginatedResponse[AccountInvoiceItem]
	if err := c.handleResponse(resp, &items); err != nil {
		return nil, err
	}

	return &items, nil
}

// httpListAccountPaymentMethodsProto retrieves account payment methods as proto
// messages for the proto-backed list path. The payment method data sub-object is
// modeled as a google.protobuf.Struct, so whatever object the API returns per
// payment method type round-trips intact. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListAccountPaymentMethods.
func (c *Client) httpListAccountPaymentMethodsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountPaymentMethod, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountPaymentMethods", endpointAccountPaymentMethods, page, pageSize,
		func() *linodev1.AccountPaymentMethod { return &linodev1.AccountPaymentMethod{} })
}

// httpListAccountInvoiceItemsProto retrieves an invoice's line items as proto
// messages for the proto-backed list path. The invoice id is formatted into the
// endpoint the same way httpListAccountInvoiceItems does, then
// listProtoElementsPaginated adds page/page_size via withPaginationQuery, so the
// runtime request matches exactly.
func (c *Client) httpListAccountInvoiceItemsProto(ctx context.Context, invoiceID, page, pageSize int) ([]*linodev1.AccountInvoiceItem, error) {
	endpoint := endpointAccountInvoices + "/" + strconv.Itoa(invoiceID) + "/items"

	return listProtoElementsPaginated(ctx, c, "ListAccountInvoiceItems", endpoint, page, pageSize,
		func() *linodev1.AccountInvoiceItem { return &linodev1.AccountInvoiceItem{} })
}

// httpListAccountChildAccountsProto retrieves child-level accounts as proto
// messages for the proto-backed list path. The page/page_size pair flows through
// withPaginationQuery, so the request matches httpListAccountChildAccounts.
func (c *Client) httpListAccountChildAccountsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ChildAccount, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountChildAccounts", endpointAccountChildAccounts, page, pageSize,
		func() *linodev1.ChildAccount { return &linodev1.ChildAccount{} })
}

// httpListAccountChildAccounts retrieves child-level accounts.
func (c *Client) httpListAccountChildAccounts(ctx context.Context, page, pageSize int) (*PaginatedResponse[ChildAccount], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountChildAccounts, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountChildAccounts", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var childAccounts PaginatedResponse[ChildAccount]
	if err := c.handleResponse(resp, &childAccounts); err != nil {
		return nil, err
	}

	return &childAccounts, nil
}

func (c *Client) httpListAccountServiceTransfers(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountEntityTransfer], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountServiceTransfers, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountServiceTransfers", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var transfers PaginatedResponse[AccountEntityTransfer]
	if err := c.handleResponse(resp, &transfers); err != nil {
		return nil, err
	}

	return &transfers, nil
}

// httpListAccountServiceTransfersProto retrieves account service transfers as
// proto messages for the proto-backed list path. The page/page_size pair flows
// through withPaginationQuery, so the request matches
// httpListAccountServiceTransfers.
func (c *Client) httpListAccountServiceTransfersProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountEntityTransfer, error) {
	return listProtoElementsPaginated(ctx, c, "ListAccountServiceTransfers", endpointAccountServiceTransfers, page, pageSize,
		func() *linodev1.AccountEntityTransfer { return &linodev1.AccountEntityTransfer{} })
}

// httpGetAccountServiceTransfer retrieves one account service transfer by token.
func (c *Client) httpGetAccountServiceTransfer(ctx context.Context, token string) (*AccountEntityTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountServiceTransfers + "/" + url.PathEscape(token)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountServiceTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var transfer AccountEntityTransfer
	if err := c.handleResponse(resp, &transfer); err != nil {
		return nil, err
	}

	return &transfer, nil
}

// httpGetAccountServiceTransferProto retrieves one account service transfer as a
// proto message.
func (c *Client) httpGetAccountServiceTransferProto(ctx context.Context, token string) (*linodev1.AccountEntityTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountServiceTransfers + "/" + url.PathEscape(token)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountServiceTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	transfer := &linodev1.AccountEntityTransfer{}
	if err := c.handleProtoResponse(resp, transfer); err != nil {
		return nil, err
	}

	return transfer, nil
}

// httpDeleteAccountServiceTransfer cancels one account service transfer by token.
func (c *Client) httpDeleteAccountServiceTransfer(ctx context.Context, token string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountServiceTransfers + "/" + url.PathEscape(token)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteAccountServiceTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpAcceptAccountServiceTransfer accepts one account service transfer by token.
func (c *Client) httpAcceptAccountServiceTransfer(ctx context.Context, token string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountServiceTransfers + "/" + url.PathEscape(token) + "/accept"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "AcceptAccountServiceTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpCreateAccountServiceTransfer creates an account service transfer.
func (c *Client) httpCreateAccountServiceTransfer(ctx context.Context, req *CreateAccountServiceTransferRequest) (*AccountEntityTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountServiceTransfers, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountServiceTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var transfer AccountEntityTransfer
	if err := c.handleResponse(resp, &transfer); err != nil {
		return nil, err
	}

	return &transfer, nil
}

// httpCreateAccountServiceTransferProto creates an account service transfer and
// decodes the response into the proto AccountEntityTransfer element.
func (c *Client) httpCreateAccountServiceTransferProto(ctx context.Context, req *CreateAccountServiceTransferRequest) (*linodev1.AccountEntityTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountServiceTransfers, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountServiceTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	transfer := &linodev1.AccountEntityTransfer{}
	if err := c.handleProtoResponse(resp, transfer); err != nil {
		return nil, err
	}

	return transfer, nil
}

// httpGetAccountEvent retrieves one account event by ID.
func (c *Client) httpGetAccountEvent(ctx context.Context, eventID int) (*AccountEvent, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountEvents + "/" + url.PathEscape(strconv.Itoa(eventID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountEvent", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var event AccountEvent
	if err := c.handleResponse(resp, &event); err != nil {
		return nil, err
	}

	return &event, nil
}

// httpGetAccountEventProto retrieves one account event as a proto message.
func (c *Client) httpGetAccountEventProto(ctx context.Context, eventID int) (*linodev1.AccountEvent, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountEvents + "/" + url.PathEscape(strconv.Itoa(eventID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountEvent", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	event := &linodev1.AccountEvent{}
	if err := c.handleProtoResponse(resp, event); err != nil {
		return nil, err
	}

	return event, nil
}

// httpMarkAccountEventSeen marks one account event as seen by ID.
func (c *Client) httpMarkAccountEventSeen(ctx context.Context, eventID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountEvents + "/" + url.PathEscape(strconv.Itoa(eventID)) + "/seen"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "MarkAccountEventSeen", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpGetAccountChildAccount retrieves one child-level account by EUUID.
func (c *Client) httpGetAccountChildAccount(ctx context.Context, euuid string) (*ChildAccount, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountChildAccounts + "/" + url.PathEscape(euuid)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountChildAccount", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var childAccount ChildAccount
	if err := c.handleResponse(resp, &childAccount); err != nil {
		return nil, err
	}

	return &childAccount, nil
}

// httpGetAccountChildAccountProto retrieves one child-level account by EUUID and
// decodes it into the proto ChildAccount element for the proto-backed read path.
// credit_card is a message field, so a null credit_card from the API is omitted
// rather than rendered as empty strings.
func (c *Client) httpGetAccountChildAccountProto(ctx context.Context, euuid string) (*linodev1.ChildAccount, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountChildAccounts + "/" + url.PathEscape(euuid)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountChildAccount", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	childAccount := &linodev1.ChildAccount{}
	if err := c.handleProtoResponse(resp, childAccount); err != nil {
		return nil, err
	}

	return childAccount, nil
}

// httpCreateAccountChildAccountToken creates a proxy user token for one child-level account.
func (c *Client) httpCreateAccountChildAccountToken(ctx context.Context, euuid string) (*ProxyUserToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountChildAccounts + "/" + url.PathEscape(euuid) + "/token"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountChildAccountToken", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var token ProxyUserToken
	if err := c.handleResponse(resp, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// httpCreateAccountChildAccountTokenProto creates a proxy user token for one
// child-level account and decodes the response into the proto ProxyUserToken
// element. The token it carries is returned to the user by design.
func (c *Client) httpCreateAccountChildAccountTokenProto(ctx context.Context, euuid string) (*linodev1.ProxyUserToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountChildAccounts + "/" + url.PathEscape(euuid) + "/token"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountChildAccountToken", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	token := &linodev1.ProxyUserToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpCreateOAuthClient creates an account OAuth client.
func (c *Client) httpCreateOAuthClient(ctx context.Context, req *CreateOAuthClientRequest) (*CreatedOAuthClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountOAuthClients, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateOAuthClient", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var client CreatedOAuthClient
	if err := c.handleResponse(resp, &client); err != nil {
		return nil, err
	}

	return &client, nil
}

// httpCreateOAuthClientProto creates an OAuth client and decodes the response
// into the proto CreatedOAuthClient element (which carries the one-time secret).
func (c *Client) httpCreateOAuthClientProto(ctx context.Context, req *CreateOAuthClientRequest) (*linodev1.CreatedOAuthClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountOAuthClients, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateOAuthClient", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	client := &linodev1.CreatedOAuthClient{}
	if err := c.handleProtoResponse(resp, client); err != nil {
		return nil, err
	}

	return client, nil
}

// httpGetAccountBeta retrieves one enrolled account beta program.
func (c *Client) httpGetAccountBeta(ctx context.Context, betaID string) (*AccountBetaProgram, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountBetas + "/" + url.PathEscape(betaID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountBeta", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var beta AccountBetaProgram
	if err := c.handleResponse(resp, &beta); err != nil {
		return nil, err
	}

	return &beta, nil
}

// httpGetAccountBetaProto retrieves one enrolled account beta program as a proto
// message.
func (c *Client) httpGetAccountBetaProto(ctx context.Context, betaID string) (*linodev1.AccountBetaProgram, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountBetas + "/" + url.PathEscape(betaID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountBeta", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	beta := &linodev1.AccountBetaProgram{}
	if err := c.handleProtoResponse(resp, beta); err != nil {
		return nil, err
	}

	return beta, nil
}

// httpEnrollAccountBeta enrolls the account in a beta program via POST /v4/account/betas.
func (c *Client) httpEnrollAccountBeta(ctx context.Context, req *EnrollAccountBetaRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountBetas, req)
	if err != nil {
		return &NetworkError{Operation: "EnrollAccountBeta", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpAddAccountPromoCredit applies a promo credit to the account via POST /v4/account/promo-codes.
func (c *Client) httpAddAccountPromoCredit(ctx context.Context, req *AddAccountPromoCreditRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountPromoCodes, req)
	if err != nil {
		return &NetworkError{Operation: "AddAccountPromoCredit", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

func withPaginationQuery(endpoint string, page, pageSize int) string {
	query := url.Values{}

	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}

	if pageSize > 0 {
		query.Set("page_size", strconv.Itoa(pageSize))
	}

	if encoded := query.Encode(); encoded != "" {
		return endpoint + "?" + encoded
	}

	return endpoint
}

// httpAcknowledgeAccountAgreements acknowledges account agreements via POST /v4/account/agreements.
func (c *Client) httpAcknowledgeAccountAgreements(ctx context.Context, req *AcknowledgeAccountAgreementsRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountAgreements, req)
	if err != nil {
		return &NetworkError{Operation: "AcknowledgeAccountAgreements", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpCancelAccount cancels the account via POST /v4/account/cancel.
func (c *Client) httpCancelAccount(ctx context.Context, req *CancelAccountRequest) (*CancelAccountResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountCancel, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CancelAccount", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var cancelResponse CancelAccountResponse
	if err := c.handleResponse(resp, &cancelResponse); err != nil {
		return nil, err
	}

	return &cancelResponse, nil
}

// httpCancelAccountProto cancels the account and decodes the response into the
// proto AccountCancelResponse element.
func (c *Client) httpCancelAccountProto(ctx context.Context, req *CancelAccountRequest) (*linodev1.AccountCancelResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountCancel, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CancelAccount", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	cancelResponse := &linodev1.AccountCancelResponse{}
	if err := c.handleProtoResponse(resp, cancelResponse); err != nil {
		return nil, err
	}

	return cancelResponse, nil
}

// httpUpdateAccount updates account billing/contact fields via PUT /v4/account.
func (c *Client) httpUpdateAccount(ctx context.Context, req *UpdateAccountRequest) (*Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointAccount, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateAccount", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var account Account
	if err := c.handleResponse(resp, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// httpUpdateProfile updates the authenticated user's profile via PUT /v4/profile.
func (c *Client) httpUpdateProfile(ctx context.Context, req *UpdateProfileRequest) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointProfile, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateProfile", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var profile Profile
	if err := c.handleResponse(resp, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}
