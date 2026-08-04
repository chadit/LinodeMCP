package linode

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// Each method below calls one Linode API endpoint through its tool's declared
// proto route, so paths live in the proto rather than here; endpointProfile is
// the one exception. The *Proto variants decode into generated proto elements,
// and the paginated list helpers encode page/page_size via withPaginationQuery.

// endpointProfile is the last hand-built path in this package: PUT /profile
// has no tool in front of it, so no proto route declares it.
const endpointProfile = "/profile"

// httpGetProfile retrieves the authenticated user's profile.
func (c *Client) httpGetProfile(ctx context.Context) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetProfile", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_profile_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetProfile", err)
	}

	defer drainClose(resp)

	profile := &linodev1.Profile{}
	if err := c.handleProtoResponse(resp, profile); err != nil {
		return nil, err
	}

	return profile, nil
}

// httpCreateProfileTokenProto creates a personal access token. The created
// element carries the one-time secret, which no later read returns.
func (c *Client) httpCreateProfileTokenProto(ctx context.Context, req CreateProfileTokenRequest) (*linodev1.CreatedPersonalAccessToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_token_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateProfileToken", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_profile_preferences_update", req)
	if err != nil {
		return nil, wrapRequestError("UpdateProfilePreferences", err)
	}

	defer drainClose(resp)

	var preferences ProfilePreferences
	if err := c.handleResponse(resp, &preferences); err != nil {
		return nil, err
	}

	return preferences, nil
}

// httpEnableProfileTFAProto generates a two-factor secret. The secret must be
// confirmed to activate two-factor auth, so it is returned rather than
// output-redacted; the handler adds the one-time warning the API omits.
func (c *Client) httpEnableProfileTFAProto(ctx context.Context) (*linodev1.ProfileTfaEnableResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_tfa_enable", nil)
	if err != nil {
		return nil, wrapRequestError("EnableProfileTFA", err)
	}

	defer drainClose(resp)

	result := &linodev1.ProfileTfaEnableResponse{}
	if err := c.handleProtoResponse(resp, result); err != nil {
		return nil, err
	}

	return result, nil
}

// httpSendProfilePhoneNumberVerificationCode sends a phone verification code.
func (c *Client) httpSendProfilePhoneNumberVerificationCode(ctx context.Context, req *ProfilePhoneNumberRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_phone_number_send", req)
	if err != nil {
		return wrapRequestError("SendProfilePhoneNumberVerificationCode", err)
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpDeleteProfilePhoneNumber deletes the profile's phone number.
func (c *Client) httpDeleteProfilePhoneNumber(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_phone_number_delete", nil)
	if err != nil {
		return wrapRequestError("DeleteProfilePhoneNumber", err)
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpVerifyProfilePhoneNumber verifies a phone number with an OTP code.
func (c *Client) httpVerifyProfilePhoneNumber(ctx context.Context, req *ProfilePhoneNumberVerifyRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_phone_number_verify", req)
	if err != nil {
		return wrapRequestError("VerifyProfilePhoneNumber", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_profile_tfa_disable", nil)
	if err != nil {
		return wrapRequestError("DisableProfileTFA", err)
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpConfirmProfileTFAEnable confirms two-factor authentication enablement.
func (c *Client) httpConfirmProfileTFAEnable(ctx context.Context, req *ProfileTFAEnableConfirmRequest) (ProfileTFAEnableConfirmResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if req == nil {
		req = &ProfileTFAEnableConfirmRequest{}
	}

	resp, err := c.makeRouteRequest(ctx, "linode_profile_tfa_enable_confirm", req)
	if err != nil {
		return nil, wrapRequestError("ConfirmProfileTFAEnable", err)
	}

	defer drainClose(resp)

	var result ProfileTFAEnableConfirmResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// httpListProfileLoginsProto lists profile login history.
func (c *Client) httpListProfileLoginsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountLogin, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListProfileLogins",
		"linode_profile_login_list", "", nil, page, pageSize,
		func() *linodev1.AccountLogin { return &linodev1.AccountLogin{} })
}

// httpDeleteProfileToken revokes one personal access token.
func (c *Client) httpDeleteProfileToken(ctx context.Context, tokenID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_token_delete", nil, tokenID)
	if err != nil {
		return wrapRequestError("DeleteProfileToken", err)
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpUpdateProfileTokenProto updates one personal access token. An update
// never returns the secret, so the element carries metadata only.
func (c *Client) httpUpdateProfileTokenProto(ctx context.Context, tokenID string, req UpdateProfileTokenRequest) (*linodev1.PersonalAccessToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if req == nil {
		req = UpdateProfileTokenRequest{}
	}

	resp, err := c.makeRouteRequest(ctx, "linode_profile_token_update", req, tokenID)
	if err != nil {
		return nil, wrapRequestError("UpdateProfileToken", err)
	}

	defer drainClose(resp)

	token := &linodev1.PersonalAccessToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpGetProfileLoginProto retrieves one profile login (AccountLogin shape).
func (c *Client) httpGetProfileLoginProto(ctx context.Context, loginID int) (*linodev1.AccountLogin, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_login_get", nil, loginID)
	if err != nil {
		return nil, wrapRequestError("GetProfileLogin", err)
	}

	defer drainClose(resp)

	login := &linodev1.AccountLogin{}
	if err := c.handleProtoResponse(resp, login); err != nil {
		return nil, err
	}

	return login, nil
}

// httpGetProfileApp retrieves one authorized OAuth app.
func (c *Client) httpGetProfileApp(ctx context.Context, appID int) (*ProfileApp, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_app_get", nil, appID)
	if err != nil {
		return nil, wrapRequestError("GetProfileApp", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_profile_app_get", nil, appID)
	if err != nil {
		return nil, wrapRequestError("GetProfileApp", err)
	}

	defer drainClose(resp)

	app := &linodev1.ProfileApp{}
	if err := c.handleProtoResponse(resp, app); err != nil {
		return nil, err
	}

	return app, nil
}

// httpDeleteProfileApp revokes access for one authorized OAuth app.
func (c *Client) httpDeleteProfileApp(ctx context.Context, appID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_app_delete", nil, appID)
	if err != nil {
		return wrapRequestError("DeleteProfileApp", err)
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpGetProfileDevice retrieves one trusted device.
func (c *Client) httpGetProfileDevice(ctx context.Context, deviceID int) (*ProfileDevice, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_device_get", nil, deviceID)
	if err != nil {
		return nil, wrapRequestError("GetProfileDevice", err)
	}

	defer drainClose(resp)

	var device ProfileDevice
	if err := c.handleResponse(resp, &device); err != nil {
		return nil, err
	}

	return &device, nil
}

// httpGetProfileDeviceProto retrieves one trusted device as a proto message.
func (c *Client) httpGetProfileDeviceProto(ctx context.Context, deviceID int) (*linodev1.TrustedDevice, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_device_get", nil, deviceID)
	if err != nil {
		return nil, wrapRequestError("GetProfileDevice", err)
	}

	defer drainClose(resp)

	device := &linodev1.TrustedDevice{}
	if err := c.handleProtoResponse(resp, device); err != nil {
		return nil, err
	}

	return device, nil
}

// httpDeleteProfileDevice revokes one trusted device.
func (c *Client) httpDeleteProfileDevice(ctx context.Context, deviceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_device_revoke", nil, deviceID)
	if err != nil {
		return wrapRequestError("DeleteProfileDevice", err)
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpGetProfileGrants retrieves the token's grants. A PAT gets a 200 with a
// zero-valued payload by design; callers tell PAT from OAuth by checking
// Profile.Scopes first, so this method does not need to know the token type.
func (c *Client) httpGetProfileGrants(ctx context.Context) (*Grants, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_grants_get", nil)
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

// httpGetProfileTokenProto retrieves one personal access token. The element
// models metadata only, so any token value the API returns is dropped on decode.
func (c *Client) httpGetProfileTokenProto(ctx context.Context, tokenID int) (*linodev1.PersonalAccessToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_token_get", nil, tokenID)
	if err != nil {
		return nil, wrapRequestError("GetProfileToken", err)
	}

	defer drainClose(resp)

	token := &linodev1.PersonalAccessToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpAnswerProfileSecurityQuestions answers the profile's security questions.
func (c *Client) httpAnswerProfileSecurityQuestions(ctx context.Context, req *AnswerProfileSecurityQuestionsRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_security_question_answer", req)
	if err != nil {
		return wrapRequestError("AnswerProfileSecurityQuestions", err)
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpGetProfilePreferences retrieves the profile's preferences.
func (c *Client) httpGetProfilePreferences(ctx context.Context) (*ProfilePreferences, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_profile_preferences_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetProfilePreferences", err)
	}

	defer drainClose(resp)

	preferences := ProfilePreferences{}
	if err := c.handleResponse(resp, &preferences); err != nil {
		return nil, err
	}

	return &preferences, nil
}

// httpListProfileAppsProto lists OAuth app authorizations.
func (c *Client) httpListProfileAppsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ProfileApp, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListProfileApps",
		"linode_profile_app_list", "", nil, page, pageSize,
		func() *linodev1.ProfileApp { return &linodev1.ProfileApp{} })
}

// httpGetAccount retrieves the authenticated user's account.
func (c *Client) httpGetAccount(ctx context.Context) (*Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetAccount", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_account_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetAccount", err)
	}

	defer drainClose(resp)

	account := &linodev1.Account{}
	if err := c.handleProtoResponse(resp, account); err != nil {
		return nil, err
	}

	return account, nil
}

// httpUpdateAccountProto updates the account and returns it as a proto message.
func (c *Client) httpUpdateAccountProto(ctx context.Context, req *UpdateAccountRequest) (*linodev1.Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_update", req)
	if err != nil {
		return nil, wrapRequestError("UpdateAccount", err)
	}

	defer drainClose(resp)

	account := &linodev1.Account{}
	if err := c.handleProtoResponse(resp, account); err != nil {
		return nil, err
	}

	return account, nil
}

// httpGetAccountTransferProto retrieves account transfer usage.
func (c *Client) httpGetAccountTransferProto(ctx context.Context) (*linodev1.AccountTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_transfer_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetAccountTransfer", err)
	}

	defer drainClose(resp)

	transfer := &linodev1.AccountTransfer{}
	if err := c.handleProtoResponse(resp, transfer); err != nil {
		return nil, err
	}

	return transfer, nil
}

// httpGetAccountSettings retrieves account-wide settings.
func (c *Client) httpGetAccountSettings(ctx context.Context) (*AccountSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_settings_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetAccountSettings", err)
	}

	defer drainClose(resp)

	var settings AccountSettings
	if err := c.handleResponse(resp, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// httpGetAccountSettingsProto retrieves account settings as a proto message.
func (c *Client) httpGetAccountSettingsProto(ctx context.Context) (*linodev1.AccountSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_settings_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetAccountSettings", err)
	}

	defer drainClose(resp)

	settings := &linodev1.AccountSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// httpUpdateAccountSettingsProto updates account settings.
func (c *Client) httpUpdateAccountSettingsProto(ctx context.Context, req *UpdateAccountSettingsRequest) (*linodev1.AccountSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_settings_update", req)
	if err != nil {
		return nil, wrapRequestError("UpdateAccountSettings", err)
	}

	defer drainClose(resp)

	settings := &linodev1.AccountSettings{}
	if err := c.handleProtoResponse(resp, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// httpEnableAccountManaged enables Linode Managed for the account.
func (c *Client) httpEnableAccountManaged(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_settings_managed_enable", nil)
	if err != nil {
		return wrapRequestError("EnableAccountManaged", err)
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpListManagedCredentialsProto lists stored Managed credentials. The secret
// material is write-only and never present in the list body.
func (c *Client) httpListManagedCredentialsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedCredential, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListManagedCredentials",
		"linode_managed_credential_list", "", nil, page, pageSize,
		func() *linodev1.ManagedCredential { return &linodev1.ManagedCredential{} })
}

// httpUpdateManagedCredentialProto updates one stored Managed credential's label.
func (c *Client) httpUpdateManagedCredentialProto(ctx context.Context, credentialID int, req UpdateManagedCredentialRequest) (*linodev1.ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_credential_update", req, credentialID)
	if err != nil {
		return nil, wrapRequestError("UpdateManagedCredential", err)
	}

	defer drainClose(resp)

	credential := &linodev1.ManagedCredential{}
	if err := c.handleProtoResponse(resp, credential); err != nil {
		return nil, err
	}

	return credential, nil
}

// httpUpdateManagedCredentialUsernamePassword updates one stored Managed
// credential's username and password.
func (c *Client) httpUpdateManagedCredentialUsernamePassword(ctx context.Context, credentialID int, req *UpdateManagedCredentialUsernamePasswordRequest) (*ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_credential_username_password_update", req, credentialID)
	if err != nil {
		return nil, wrapRequestError("UpdateManagedCredentialUsernamePassword", err)
	}

	defer drainClose(resp)

	var credential ManagedCredential
	if err := c.handleResponse(resp, &credential); err != nil {
		return nil, err
	}

	return &credential, nil
}

// httpGetManagedSSHKeyProto retrieves the account's Managed SSH public key.
func (c *Client) httpGetManagedSSHKeyProto(ctx context.Context) (*linodev1.ManagedSSHKey, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_sshkey_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetManagedSSHKey", err)
	}

	defer drainClose(resp)

	sshKey := &linodev1.ManagedSSHKey{}
	if err := c.handleProtoResponse(resp, sshKey); err != nil {
		return nil, err
	}

	return sshKey, nil
}

// httpCreateManagedCredentialProto creates a stored Managed credential. The
// response never echoes the secret.
func (c *Client) httpCreateManagedCredentialProto(ctx context.Context, request *CreateManagedCredentialRequest) (*linodev1.ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_credential_create", request)
	if err != nil {
		return nil, wrapRequestError("CreateManagedCredential", err)
	}

	defer drainClose(resp)

	credential := &linodev1.ManagedCredential{}
	if err := c.handleProtoResponse(resp, credential); err != nil {
		return nil, err
	}

	return credential, nil
}

// httpGetManagedCredential retrieves one stored Managed credential.
func (c *Client) httpGetManagedCredential(ctx context.Context, credentialID int) (*ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_credential_get", nil, credentialID)
	if err != nil {
		return nil, wrapRequestError("GetManagedCredential", err)
	}

	defer drainClose(resp)

	var credential ManagedCredential
	if err := c.handleResponse(resp, &credential); err != nil {
		return nil, err
	}

	return &credential, nil
}

// httpGetManagedCredentialProto retrieves one Managed credential.
func (c *Client) httpGetManagedCredentialProto(ctx context.Context, credentialID int) (*linodev1.ManagedCredential, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_credential_get", nil, credentialID)
	if err != nil {
		return nil, wrapRequestError("GetManagedCredential", err)
	}

	defer drainClose(resp)

	credential := &linodev1.ManagedCredential{}
	if err := c.handleProtoResponse(resp, credential); err != nil {
		return nil, err
	}

	return credential, nil
}

// httpRevokeManagedCredential revokes one stored Managed credential.
func (c *Client) httpRevokeManagedCredential(ctx context.Context, credentialID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_managed_credential_revoke", nil, credentialID)
	if err != nil {
		return wrapRequestError("RevokeManagedCredential", err)
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// httpGetAccountAgreementsProto retrieves agreement acknowledgment status. The
// endpoint returns a flat object of bool flags, not a list.
func (c *Client) httpGetAccountAgreementsProto(ctx context.Context) (*linodev1.AccountAgreements, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_agreement_list", nil)
	if err != nil {
		return nil, wrapRequestError("GetAccountAgreements", err)
	}

	defer drainClose(resp)

	agreements := &linodev1.AccountAgreements{}
	if err := c.handleProtoResponse(resp, agreements); err != nil {
		return nil, err
	}

	return agreements, nil
}

// httpListAccountMaintenanceProto lists account maintenance records.
func (c *Client) httpListAccountMaintenanceProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountMaintenance, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountMaintenance",
		"linode_account_maintenance_list", "", nil, page, pageSize,
		func() *linodev1.AccountMaintenance { return &linodev1.AccountMaintenance{} })
}

// httpListMaintenancePoliciesProto lists maintenance policies.
func (c *Client) httpListMaintenancePoliciesProto(ctx context.Context, page, pageSize int) ([]*linodev1.MaintenancePolicy, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListMaintenancePolicies",
		"linode_maintenance_policy_list", "", nil, page, pageSize,
		func() *linodev1.MaintenancePolicy { return &linodev1.MaintenancePolicy{} })
}

// httpListAccountNotificationsProto lists account notifications.
func (c *Client) httpListAccountNotificationsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountNotification, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountNotifications",
		"linode_account_notification_list", "", nil, page, pageSize,
		func() *linodev1.AccountNotification { return &linodev1.AccountNotification{} })
}

// httpListProfileDevicesProto lists trusted devices. It decodes through the
// required-data path so a missing or null data member fails instead of reporting
// zero live Remember Me sessions, matching what the Python client does.
func (c *Client) httpListProfileDevicesProto(ctx context.Context, page, pageSize int) ([]*linodev1.TrustedDevice, error) {
	return listProtoElementsPaginatedRequiredDataRouted(ctx, c, "ListProfileDevices",
		"linode_profile_device_list", "", nil, page, pageSize,
		func() *linodev1.TrustedDevice { return &linodev1.TrustedDevice{} })
}

// httpListProfileTokensProto lists personal access token metadata. The proto
// models no secret field, so a token value the API returns is dropped on decode.
func (c *Client) httpListProfileTokensProto(ctx context.Context, page, pageSize int) ([]*linodev1.PersonalAccessToken, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListProfileTokens",
		"linode_profile_token_list", "", nil, page, pageSize,
		func() *linodev1.PersonalAccessToken { return &linodev1.PersonalAccessToken{} })
}

// httpListProfileSecurityQuestionsProto lists the profile security questions.
// The endpoint is not paginated and wraps its elements under
// "security_questions" rather than the usual {data} page envelope.
func (c *Client) httpListProfileSecurityQuestionsProto(ctx context.Context) ([]*linodev1.SecurityQuestion, error) {
	return listProtoElementsKeyedRouted(ctx, c, "ListProfileSecurityQuestions",
		"linode_profile_security_question_list", "", "security_questions", nil,
		func() *linodev1.SecurityQuestion { return &linodev1.SecurityQuestion{} })
}

// httpGetAccountAvailabilityProto retrieves one region's account availability.
func (c *Client) httpGetAccountAvailabilityProto(ctx context.Context, regionID string) (*linodev1.AccountAvailability, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_availability_get", nil, regionID)
	if err != nil {
		return nil, wrapRequestError("GetAccountAvailability", err)
	}

	defer drainClose(resp)

	availability := &linodev1.AccountAvailability{}
	if err := c.handleProtoResponse(resp, availability); err != nil {
		return nil, err
	}

	return availability, nil
}

// httpListAccountAvailabilityProto lists account service availability by region.
func (c *Client) httpListAccountAvailabilityProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountAvailability, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountAvailability",
		"linode_account_availability_list", "", nil, page, pageSize,
		func() *linodev1.AccountAvailability { return &linodev1.AccountAvailability{} })
}

// httpListBetasProto lists available beta programs.
func (c *Client) httpListBetasProto(ctx context.Context, page, pageSize int) ([]*linodev1.BetaProgram, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListBetas",
		"linode_beta_list", "", nil, page, pageSize,
		func() *linodev1.BetaProgram { return &linodev1.BetaProgram{} })
}

// httpGetBetaProto retrieves one available beta program.
func (c *Client) httpGetBetaProto(ctx context.Context, betaID string) (*linodev1.BetaProgram, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_beta_get", nil, betaID)
	if err != nil {
		return nil, wrapRequestError("GetBeta", err)
	}

	defer drainClose(resp)

	beta := &linodev1.BetaProgram{}
	if err := c.handleProtoResponse(resp, beta); err != nil {
		return nil, err
	}

	return beta, nil
}

// httpListAccountBetasProto lists beta programs the account is enrolled in.
func (c *Client) httpListAccountBetasProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountBetaProgram, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountBetas",
		"linode_account_beta_list", "", nil, page, pageSize,
		func() *linodev1.AccountBetaProgram { return &linodev1.AccountBetaProgram{} })
}

// httpListAccountOAuthClientsProto lists the account's OAuth clients.
func (c *Client) httpListAccountOAuthClientsProto(ctx context.Context, page, pageSize int) ([]*linodev1.OAuthClient, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountOAuthClients",
		"linode_account_oauth_client_list", "", nil, page, pageSize,
		func() *linodev1.OAuthClient { return &linodev1.OAuthClient{} })
}

// httpListLongviewClientsProto lists Longview clients.
func (c *Client) httpListLongviewClientsProto(ctx context.Context, page, pageSize int) ([]*linodev1.LongviewClient, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListLongviewClients",
		"linode_longview_client_list", "", nil, page, pageSize,
		func() *linodev1.LongviewClient { return &linodev1.LongviewClient{} })
}

// UpdateLongviewClientProto updates one Longview client.
func (c *Client) UpdateLongviewClientProto(ctx context.Context, clientID int, req *UpdateLongviewClientRequest) (*linodev1.LongviewClient, error) {
	return c.httpUpdateLongviewClientProto(ctx, clientID, req)
}

// httpUpdateLongviewClientProto updates one Longview client.
func (c *Client) httpUpdateLongviewClientProto(ctx context.Context, clientID int, req *UpdateLongviewClientRequest) (*linodev1.LongviewClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_longview_client_update", req, clientID)
	if err != nil {
		return nil, wrapRequestError("UpdateLongviewClient", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_longview_client_delete", nil, clientID)
	if err != nil {
		return wrapRequestError("DeleteLongviewClient", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpGetAccountPaymentMethod retrieves one payment method by ID.
func (c *Client) httpGetAccountPaymentMethod(ctx context.Context, paymentMethodID string) (*AccountPaymentMethod, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_payment_method_get", nil, paymentMethodID)
	if err != nil {
		return nil, wrapRequestError("GetAccountPaymentMethod", err)
	}

	defer drainClose(resp)

	var method AccountPaymentMethod
	if err := c.handleResponse(resp, &method); err != nil {
		return nil, err
	}

	return &method, nil
}

// httpGetAccountPaymentMethodProto retrieves one payment method. The
// polymorphic data object rides through the element's Struct field.
func (c *Client) httpGetAccountPaymentMethodProto(ctx context.Context, paymentMethodID string) (*linodev1.AccountPaymentMethod, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_payment_method_get", nil, paymentMethodID)
	if err != nil {
		return nil, wrapRequestError("GetAccountPaymentMethod", err)
	}

	defer drainClose(resp)

	method := &linodev1.AccountPaymentMethod{}
	if err := c.handleProtoResponse(resp, method); err != nil {
		return nil, err
	}

	return method, nil
}

// CreateAccountPaymentMethodProto adds a payment method to the account.
func (c *Client) CreateAccountPaymentMethodProto(ctx context.Context, req *CreateAccountPaymentMethodRequest) (*linodev1.AccountPaymentMethod, error) {
	return c.httpCreateAccountPaymentMethodProto(ctx, req)
}

// httpCreateAccountPaymentMethodProto adds a payment method to the account.
func (c *Client) httpCreateAccountPaymentMethodProto(ctx context.Context, req *CreateAccountPaymentMethodRequest) (*linodev1.AccountPaymentMethod, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_payment_method_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateAccountPaymentMethod", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_account_payment_method_delete", nil, paymentMethodID)
	if err != nil {
		return wrapRequestError("DeleteAccountPaymentMethod", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

func (c *Client) httpMakeAccountPaymentMethodDefault(ctx context.Context, paymentMethodID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_payment_method_make_default", nil, paymentMethodID)
	if err != nil {
		return wrapRequestError("MakeAccountPaymentMethodDefault", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpGetAccountOAuthClient retrieves one OAuth client by ID.
func (c *Client) httpGetAccountOAuthClient(ctx context.Context, clientID string) (*OAuthClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_oauth_client_get", nil, clientID)
	if err != nil {
		return nil, wrapRequestError("GetAccountOAuthClient", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_account_oauth_client_get", nil, clientID)
	if err != nil {
		return nil, wrapRequestError("GetAccountOAuthClient", err)
	}

	defer drainClose(resp)

	oauthClient := &linodev1.OAuthClient{}
	if err := c.handleProtoResponse(resp, oauthClient); err != nil {
		return nil, err
	}

	return oauthClient, nil
}

// httpUpdateOAuthClientProto updates one OAuth client. The response element
// carries metadata only, no secret.
func (c *Client) httpUpdateOAuthClientProto(ctx context.Context, clientID string, req *UpdateOAuthClientRequest) (*linodev1.OAuthClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_oauth_client_update", req, clientID)
	if err != nil {
		return nil, wrapRequestError("UpdateOAuthClient", err)
	}

	defer drainClose(resp)

	client := &linodev1.OAuthClient{}
	if err := c.handleProtoResponse(resp, client); err != nil {
		return nil, err
	}

	return client, nil
}

// httpUpdateOAuthClientThumbnail replaces one OAuth client's thumbnail PNG.
func (c *Client) httpUpdateOAuthClientThumbnail(ctx context.Context, clientID string, thumbnailPNG []byte) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestContentType(ctx, "linode_account_oauth_client_thumbnail_update",
		contentTypePNG, bytes.NewReader(thumbnailPNG), clientID)
	if err != nil {
		return wrapRequestError("UpdateOAuthClientThumbnail", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpGetOAuthClientThumbnail retrieves one OAuth client's thumbnail PNG bytes.
func (c *Client) httpGetOAuthClientThumbnail(ctx context.Context, clientID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_oauth_client_thumbnail_get", nil, clientID)
	if err != nil {
		return nil, wrapRequestError("GetOAuthClientThumbnail", err)
	}

	defer drainClose(resp)

	// Read the body first since handleResponse would consume it
	thumbnailPNG, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &NetworkError{Operation: "GetOAuthClientThumbnail", Err: err}
	}

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

	resp, err := c.makeRouteRequest(ctx, "linode_account_oauth_client_delete", nil, clientID)
	if err != nil {
		return wrapRequestError("DeleteAccountOAuthClient", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpResetOAuthClientSecretProto resets one OAuth client's secret.
func (c *Client) httpResetOAuthClientSecretProto(ctx context.Context, clientID string) (*linodev1.OAuthClientSecret, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_oauth_client_secret_reset", nil, clientID)
	if err != nil {
		return nil, wrapRequestError("ResetOAuthClientSecret", err)
	}

	defer drainClose(resp)

	secret := &linodev1.OAuthClientSecret{}
	if err := c.handleProtoResponse(resp, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

// httpListAccountEventsProto lists account events.
func (c *Client) httpListAccountEventsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountEvent, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountEvents",
		"linode_account_event_list", "", nil, page, pageSize,
		func() *linodev1.AccountEvent { return &linodev1.AccountEvent{} })
}

// httpListAccountUsersProto lists account users.
func (c *Client) httpListAccountUsersProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountUser, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountUsers",
		"linode_account_user_list", "", nil, page, pageSize,
		func() *linodev1.AccountUser { return &linodev1.AccountUser{} })
}

// httpGetAccountUser retrieves one account user by username.
func (c *Client) httpGetAccountUser(ctx context.Context, username string) (*AccountUser, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_user_get", nil, username)
	if err != nil {
		return nil, wrapRequestError("GetAccountUser", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_account_user_get", nil, username)
	if err != nil {
		return nil, wrapRequestError("GetAccountUser", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_account_user_grants_get", nil, username)
	if err != nil {
		return nil, wrapRequestError("GetAccountUserGrants", err)
	}

	defer drainClose(resp)

	var grants Grants
	if err := c.handleResponse(resp, &grants); err != nil {
		return nil, err
	}

	return &grants, nil
}

// httpGetAccountUserGrantsProto retrieves one account user's grants. The API
// omits grant sections the user has none of; protojson leaves those repeated
// fields empty, so the canonical output normalizes them to [].
func (c *Client) httpGetAccountUserGrantsProto(ctx context.Context, username string) (*linodev1.AccountUserGrants, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_user_grants_get", nil, username)
	if err != nil {
		return nil, wrapRequestError("GetAccountUserGrants", err)
	}

	defer drainClose(resp)

	grants := &linodev1.AccountUserGrants{}
	if err := c.handleProtoResponse(resp, grants); err != nil {
		return nil, err
	}

	return grants, nil
}

// httpCreateAccountUserProto creates an account user.
func (c *Client) httpCreateAccountUserProto(ctx context.Context, request *CreateAccountUserRequest) (*linodev1.AccountUser, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_user_create", request)
	if err != nil {
		return nil, wrapRequestError("CreateAccountUser", err)
	}

	defer drainClose(resp)

	user := &linodev1.AccountUser{}
	if err := c.handleProtoResponse(resp, user); err != nil {
		return nil, err
	}

	return user, nil
}

// httpUpdateAccountUserProto updates one account user by username.
func (c *Client) httpUpdateAccountUserProto(ctx context.Context, username string, request *UpdateAccountUserRequest) (*linodev1.AccountUser, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_user_update", request, username)
	if err != nil {
		return nil, wrapRequestError("UpdateAccountUser", err)
	}

	defer drainClose(resp)

	user := &linodev1.AccountUser{}
	if err := c.handleProtoResponse(resp, user); err != nil {
		return nil, err
	}

	return user, nil
}

// httpUpdateAccountUserGrantsProto updates one account user's grants.
func (c *Client) httpUpdateAccountUserGrantsProto(ctx context.Context, username string, request *UpdateAccountUserGrantsRequest) (*linodev1.AccountUserGrants, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_user_grants_update", request, username)
	if err != nil {
		return nil, wrapRequestError("UpdateAccountUserGrants", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_account_user_delete", nil, username)
	if err != nil {
		return wrapRequestError("DeleteAccountUser", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpListAccountLoginsProto lists account logins.
func (c *Client) httpListAccountLoginsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountLogin, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountLogins",
		"linode_account_login_list", "", nil, page, pageSize,
		func() *linodev1.AccountLogin { return &linodev1.AccountLogin{} })
}

// httpGetAccountLoginProto retrieves one account login.
func (c *Client) httpGetAccountLoginProto(ctx context.Context, loginID int) (*linodev1.AccountLogin, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_login_get", nil, loginID)
	if err != nil {
		return nil, wrapRequestError("GetAccountLogin", err)
	}

	defer drainClose(resp)

	login := &linodev1.AccountLogin{}
	if err := c.handleProtoResponse(resp, login); err != nil {
		return nil, err
	}

	return login, nil
}

// httpListAccountInvoicesProto lists account invoices.
func (c *Client) httpListAccountInvoicesProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountInvoice, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountInvoices",
		"linode_account_invoice_list", "", nil, page, pageSize,
		func() *linodev1.AccountInvoice { return &linodev1.AccountInvoice{} })
}

// httpListAccountPaymentsProto lists account payments.
func (c *Client) httpListAccountPaymentsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountPayment, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountPayments",
		"linode_account_payment_list", "", nil, page, pageSize,
		func() *linodev1.AccountPayment { return &linodev1.AccountPayment{} })
}

// httpGetAccountPaymentProto retrieves one account payment.
func (c *Client) httpGetAccountPaymentProto(ctx context.Context, paymentID int) (*linodev1.AccountPayment, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_payment_get", nil, paymentID)
	if err != nil {
		return nil, wrapRequestError("GetAccountPayment", err)
	}

	defer drainClose(resp)

	payment := &linodev1.AccountPayment{}
	if err := c.handleProtoResponse(resp, payment); err != nil {
		return nil, err
	}

	return payment, nil
}

// CreateAccountPaymentProto makes a payment against the account.
func (c *Client) CreateAccountPaymentProto(ctx context.Context, req *CreateAccountPaymentRequest) (*linodev1.AccountPayment, error) {
	return c.httpCreateAccountPaymentProto(ctx, req)
}

// httpCreateAccountPaymentProto makes a payment against the account.
func (c *Client) httpCreateAccountPaymentProto(ctx context.Context, req *CreateAccountPaymentRequest) (*linodev1.AccountPayment, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_payment_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateAccountPayment", err)
	}

	defer drainClose(resp)

	payment := &linodev1.AccountPayment{}
	if err := c.handleProtoResponse(resp, payment); err != nil {
		return nil, err
	}

	return payment, nil
}

// httpGetAccountInvoiceProto retrieves one account invoice.
func (c *Client) httpGetAccountInvoiceProto(ctx context.Context, invoiceID int) (*linodev1.AccountInvoice, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_invoice_get", nil, invoiceID)
	if err != nil {
		return nil, wrapRequestError("GetAccountInvoice", err)
	}

	defer drainClose(resp)

	invoice := &linodev1.AccountInvoice{}
	if err := c.handleProtoResponse(resp, invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

// httpListAccountPaymentMethodsProto lists account payment methods. The data
// sub-object is a google.protobuf.Struct, so whatever object the API returns
// per payment method type round-trips intact.
func (c *Client) httpListAccountPaymentMethodsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountPaymentMethod, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountPaymentMethods",
		"linode_account_payment_method_list", "", nil, page, pageSize,
		func() *linodev1.AccountPaymentMethod { return &linodev1.AccountPaymentMethod{} })
}

// httpListAccountInvoiceItemsProto lists one invoice's line items.
func (c *Client) httpListAccountInvoiceItemsProto(ctx context.Context, invoiceID, page, pageSize int) ([]*linodev1.AccountInvoiceItem, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountInvoiceItems",
		"linode_account_invoice_item_list", "", []any{invoiceID}, page, pageSize,
		func() *linodev1.AccountInvoiceItem { return &linodev1.AccountInvoiceItem{} })
}

// httpListAccountChildAccountsProto lists child-level accounts.
func (c *Client) httpListAccountChildAccountsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ChildAccount, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountChildAccounts",
		"linode_account_child_account_list", "", nil, page, pageSize,
		func() *linodev1.ChildAccount { return &linodev1.ChildAccount{} })
}

// httpListAccountServiceTransfersProto lists account service transfers.
func (c *Client) httpListAccountServiceTransfersProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountEntityTransfer, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAccountServiceTransfers",
		"linode_account_service_transfer_list", "", nil, page, pageSize,
		func() *linodev1.AccountEntityTransfer { return &linodev1.AccountEntityTransfer{} })
}

// httpGetAccountServiceTransfer retrieves one service transfer by token.
func (c *Client) httpGetAccountServiceTransfer(ctx context.Context, token string) (*AccountEntityTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_service_transfer_get", nil, token)
	if err != nil {
		return nil, wrapRequestError("GetAccountServiceTransfer", err)
	}

	defer drainClose(resp)

	var transfer AccountEntityTransfer
	if err := c.handleResponse(resp, &transfer); err != nil {
		return nil, err
	}

	return &transfer, nil
}

// httpGetAccountServiceTransferProto retrieves one service transfer by token.
func (c *Client) httpGetAccountServiceTransferProto(ctx context.Context, token string) (*linodev1.AccountEntityTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_service_transfer_get", nil, token)
	if err != nil {
		return nil, wrapRequestError("GetAccountServiceTransfer", err)
	}

	defer drainClose(resp)

	transfer := &linodev1.AccountEntityTransfer{}
	if err := c.handleProtoResponse(resp, transfer); err != nil {
		return nil, err
	}

	return transfer, nil
}

// httpDeleteAccountServiceTransfer cancels one service transfer by token.
func (c *Client) httpDeleteAccountServiceTransfer(ctx context.Context, token string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_service_transfer_delete", nil, token)
	if err != nil {
		return wrapRequestError("DeleteAccountServiceTransfer", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpAcceptAccountServiceTransfer accepts one service transfer by token.
func (c *Client) httpAcceptAccountServiceTransfer(ctx context.Context, token string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_service_transfer_accept", nil, token)
	if err != nil {
		return wrapRequestError("AcceptAccountServiceTransfer", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCreateAccountServiceTransferProto creates an account service transfer.
func (c *Client) httpCreateAccountServiceTransferProto(ctx context.Context, req *CreateAccountServiceTransferRequest) (*linodev1.AccountEntityTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_service_transfer_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateAccountServiceTransfer", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_account_event_get", nil, eventID)
	if err != nil {
		return nil, wrapRequestError("GetAccountEvent", err)
	}

	defer drainClose(resp)

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

	resp, err := c.makeRouteRequest(ctx, "linode_account_event_get", nil, eventID)
	if err != nil {
		return nil, wrapRequestError("GetAccountEvent", err)
	}

	defer drainClose(resp)

	event := &linodev1.AccountEvent{}
	if err := c.handleProtoResponse(resp, event); err != nil {
		return nil, err
	}

	return event, nil
}

// httpMarkAccountEventSeen marks one account event as seen.
func (c *Client) httpMarkAccountEventSeen(ctx context.Context, eventID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_event_seen", nil, eventID)
	if err != nil {
		return wrapRequestError("MarkAccountEventSeen", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpGetAccountChildAccount retrieves one child-level account by EUUID.
func (c *Client) httpGetAccountChildAccount(ctx context.Context, euuid string) (*ChildAccount, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_child_account_get", nil, euuid)
	if err != nil {
		return nil, wrapRequestError("GetAccountChildAccount", err)
	}

	defer drainClose(resp)

	var childAccount ChildAccount
	if err := c.handleResponse(resp, &childAccount); err != nil {
		return nil, err
	}

	return &childAccount, nil
}

// httpGetAccountChildAccountProto retrieves one child-level account by EUUID.
// credit_card is a message field, so a null from the API is omitted rather than
// rendered as empty strings.
func (c *Client) httpGetAccountChildAccountProto(ctx context.Context, euuid string) (*linodev1.ChildAccount, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_child_account_get", nil, euuid)
	if err != nil {
		return nil, wrapRequestError("GetAccountChildAccount", err)
	}

	defer drainClose(resp)

	childAccount := &linodev1.ChildAccount{}
	if err := c.handleProtoResponse(resp, childAccount); err != nil {
		return nil, err
	}

	return childAccount, nil
}

// httpCreateAccountChildAccountTokenProto creates a proxy user token for one
// child-level account. The token it carries is returned to the user by design.
func (c *Client) httpCreateAccountChildAccountTokenProto(ctx context.Context, euuid string) (*linodev1.ProxyUserToken, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_child_account_token_create", nil, euuid)
	if err != nil {
		return nil, wrapRequestError("CreateAccountChildAccountToken", err)
	}

	defer drainClose(resp)

	token := &linodev1.ProxyUserToken{}
	if err := c.handleProtoResponse(resp, token); err != nil {
		return nil, err
	}

	return token, nil
}

// httpCreateOAuthClientProto creates an OAuth client. The created element
// carries the one-time secret, which no later read returns.
func (c *Client) httpCreateOAuthClientProto(ctx context.Context, req *CreateOAuthClientRequest) (*linodev1.CreatedOAuthClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_oauth_client_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateOAuthClient", err)
	}

	defer drainClose(resp)

	client := &linodev1.CreatedOAuthClient{}
	if err := c.handleProtoResponse(resp, client); err != nil {
		return nil, err
	}

	return client, nil
}

// httpGetAccountBetaProto retrieves one enrolled beta program.
func (c *Client) httpGetAccountBetaProto(ctx context.Context, betaID string) (*linodev1.AccountBetaProgram, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_beta_get", nil, betaID)
	if err != nil {
		return nil, wrapRequestError("GetAccountBeta", err)
	}

	defer drainClose(resp)

	beta := &linodev1.AccountBetaProgram{}
	if err := c.handleProtoResponse(resp, beta); err != nil {
		return nil, err
	}

	return beta, nil
}

// httpEnrollAccountBeta enrolls the account in a beta program.
func (c *Client) httpEnrollAccountBeta(ctx context.Context, req *EnrollAccountBetaRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_beta_enroll", req)
	if err != nil {
		return wrapRequestError("EnrollAccountBeta", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpAddAccountPromoCredit applies a promo credit to the account.
func (c *Client) httpAddAccountPromoCredit(ctx context.Context, req *AddAccountPromoCreditRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_promo_credit_add", req)
	if err != nil {
		return wrapRequestError("AddAccountPromoCredit", err)
	}

	defer drainClose(resp)

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

// httpAcknowledgeAccountAgreements acknowledges account agreements.
func (c *Client) httpAcknowledgeAccountAgreements(ctx context.Context, req *AcknowledgeAccountAgreementsRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_agreement_acknowledge", req)
	if err != nil {
		return wrapRequestError("AcknowledgeAccountAgreements", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// httpCancelAccountProto cancels the account.
func (c *Client) httpCancelAccountProto(ctx context.Context, req *CancelAccountRequest) (*linodev1.AccountCancelResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_account_cancel", req)
	if err != nil {
		return nil, wrapRequestError("CancelAccount", err)
	}

	defer drainClose(resp)

	cancelResponse := &linodev1.AccountCancelResponse{}
	if err := c.handleProtoResponse(resp, cancelResponse); err != nil {
		return nil, err
	}

	return cancelResponse, nil
}

// httpUpdateProfile updates the authenticated user's profile via PUT /profile.
func (c *Client) httpUpdateProfile(ctx context.Context, req *UpdateProfileRequest) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointProfile, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateProfile", Err: err}
	}

	defer drainClose(resp)

	var profile Profile
	if err := c.handleResponse(resp, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}
