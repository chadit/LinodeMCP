package linode

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

type retryConfig struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	JitterEnabled bool
}

const (
	defaultMaxRetries    = 3
	defaultMaxDelay      = 30 * time.Second
	defaultBackoffFactor = 2.0
	jitterPercent        = 0.1
)

func defaultRetryConfig() retryConfig {
	return retryConfig{
		MaxRetries:    defaultMaxRetries,
		BaseDelay:     time.Second,
		MaxDelay:      defaultMaxDelay,
		BackoffFactor: defaultBackoffFactor,
		JitterEnabled: true,
	}
}

// ListTags retrieves tags with automatic retry on transient failures.
func (c *Client) ListTags(ctx context.Context, page, pageSize int) (*PaginatedResponse[Tag], error) {
	var tags *PaginatedResponse[Tag]

	err := c.executeWithRetry(ctx, "ListTags", func() error {
		var err error

		tags, err = c.httpListTags(ctx, page, pageSize)

		return err
	})

	return tags, err
}

// ListTagsProto retrieves tags as proto messages with automatic retry on
// transient failures.
func (c *Client) ListTagsProto(ctx context.Context, page, pageSize int) ([]*linodev1.Tag, error) {
	var tags []*linodev1.Tag

	err := c.executeWithRetry(ctx, "ListTags", func() error {
		var err error

		tags, err = c.httpListTagsProto(ctx, page, pageSize)

		return err
	})

	return tags, err
}

// GetProfile retrieves the user profile with automatic retry on transient failures.
func (c *Client) GetProfile(ctx context.Context) (*Profile, error) {
	var profile *Profile

	err := c.executeWithRetry(ctx, "GetProfile", func() error {
		var err error

		profile, err = c.httpGetProfile(ctx)

		return err
	})

	return profile, err
}

// GetProfileProto retrieves the user profile as a proto message with automatic
// retry on transient failures.
func (c *Client) GetProfileProto(ctx context.Context) (*linodev1.Profile, error) {
	var profile *linodev1.Profile

	err := c.executeWithRetry(ctx, "GetProfile", func() error {
		var err error

		profile, err = c.httpGetProfileProto(ctx)

		return err
	})

	return profile, err
}

// CreateProfileToken creates a personal access token without retrying the
// credential-creating request. Retrying can create multiple tokens after a
// transient error, so this method delegates exactly once.
func (c *Client) CreateProfileToken(ctx context.Context, req CreateProfileTokenRequest) (*ProfileToken, error) {
	return c.httpCreateProfileToken(ctx, req)
}

// CreateProfileTokenProto creates a personal access token and returns it (with
// the one-time secret) as a proto message without retrying the credential-creating
// request. Retrying can create multiple tokens after a transient error, so this
// method delegates exactly once.
func (c *Client) CreateProfileTokenProto(ctx context.Context, req CreateProfileTokenRequest) (*linodev1.CreatedPersonalAccessToken, error) {
	return c.httpCreateProfileTokenProto(ctx, req)
}

// SendProfilePhoneNumberVerificationCode sends a verification code without retrying the non-idempotent POST.
func (c *Client) SendProfilePhoneNumberVerificationCode(ctx context.Context, req *ProfilePhoneNumberRequest) error {
	return c.httpSendProfilePhoneNumberVerificationCode(ctx, req)
}

// EnableProfileTFA generates a two-factor authentication secret without retrying the non-idempotent POST.
func (c *Client) EnableProfileTFA(ctx context.Context) (ProfileTFAEnableResponse, error) {
	return c.httpEnableProfileTFA(ctx)
}

// DeleteProfilePhoneNumber deletes the profile phone number without retrying the destructive DELETE.
func (c *Client) DeleteProfilePhoneNumber(ctx context.Context) error {
	return c.httpDeleteProfilePhoneNumber(ctx)
}

// VerifyProfilePhoneNumber verifies a phone number without retrying the non-idempotent POST.
func (c *Client) VerifyProfilePhoneNumber(ctx context.Context, req *ProfilePhoneNumberVerifyRequest) error {
	return c.httpVerifyProfilePhoneNumber(ctx, req)
}

// DisableProfileTFA disables two-factor authentication without retrying the security-state-changing POST.
func (c *Client) DisableProfileTFA(ctx context.Context) error {
	return c.httpDisableProfileTFA(ctx)
}

// ConfirmProfileTFAEnable confirms two-factor authentication enablement without retrying the security-state-changing POST.
func (c *Client) ConfirmProfileTFAEnable(ctx context.Context, req *ProfileTFAEnableConfirmRequest) (ProfileTFAEnableConfirmResponse, error) {
	return c.httpConfirmProfileTFAEnable(ctx, req)
}

// ListProfileSecurityQuestions lists available profile security questions with automatic retry on transient failures.
func (c *Client) ListProfileSecurityQuestions(ctx context.Context) (*ProfileSecurityQuestions, error) {
	var questions *ProfileSecurityQuestions

	err := c.executeWithRetry(ctx, "ListProfileSecurityQuestions", func() error {
		var err error

		questions, err = c.httpListProfileSecurityQuestions(ctx)

		return err
	})

	return questions, err
}

// ListProfileLogins retrieves profile login history with automatic retry on transient failures.
func (c *Client) ListProfileLogins(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountLogin], error) {
	var logins *PaginatedResponse[AccountLogin]

	err := c.executeWithRetry(ctx, "ListProfileLogins", func() error {
		var err error

		logins, err = c.httpListProfileLogins(ctx, page, pageSize)

		return err
	})

	return logins, err
}

// ListProfileLoginsProto retrieves profile login history as proto messages with
// automatic retry on transient failures.
func (c *Client) ListProfileLoginsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountLogin, error) {
	var logins []*linodev1.AccountLogin

	err := c.executeWithRetry(ctx, "ListProfileLogins", func() error {
		var retryErr error

		logins, retryErr = c.httpListProfileLoginsProto(ctx, page, pageSize)

		return retryErr
	})

	return logins, err
}

// ListProfileTokens retrieves personal access tokens with automatic retry on transient failures.
func (c *Client) ListProfileTokens(ctx context.Context, page, pageSize int) (*PaginatedResponse[ProfileToken], error) {
	var tokens *PaginatedResponse[ProfileToken]

	err := c.executeWithRetry(ctx, "ListProfileTokens", func() error {
		var err error

		tokens, err = c.httpListProfileTokens(ctx, page, pageSize)

		return err
	})

	return tokens, err
}

// DeleteProfileToken revokes a personal access token without retrying the destructive request.
func (c *Client) DeleteProfileToken(ctx context.Context, tokenID int) error {
	return c.httpDeleteProfileToken(ctx, tokenID)
}

// UpdateProfileToken updates a personal access token without automatic retry.
// Retrying can replay token changes after a transient error, so this method delegates exactly once.
func (c *Client) UpdateProfileToken(ctx context.Context, tokenID string, request UpdateProfileTokenRequest) (*ProfileToken, error) {
	return c.httpUpdateProfileToken(ctx, tokenID, request)
}

// UpdateProfileTokenProto updates a personal access token and returns its metadata
// as a proto message without automatic retry. Retrying can replay token changes
// after a transient error, so this method delegates exactly once.
func (c *Client) UpdateProfileTokenProto(ctx context.Context, tokenID string, request UpdateProfileTokenRequest) (*linodev1.PersonalAccessToken, error) {
	return c.httpUpdateProfileTokenProto(ctx, tokenID, request)
}

// EnableProfileTFAProto generates a two-factor authentication secret and returns
// it as a proto message without retrying the non-idempotent POST.
func (c *Client) EnableProfileTFAProto(ctx context.Context) (*linodev1.ProfileTfaEnableResponse, error) {
	return c.httpEnableProfileTFAProto(ctx)
}

// ListProfileDevices retrieves trusted devices with automatic retry on transient failures.
func (c *Client) ListProfileDevices(ctx context.Context, page, pageSize int) (*PaginatedResponse[ProfileDevice], error) {
	var devices *PaginatedResponse[ProfileDevice]

	err := c.executeWithRetry(ctx, "ListProfileDevices", func() error {
		var err error

		devices, err = c.httpListProfileDevices(ctx, page, pageSize)

		return err
	})

	return devices, err
}

// GetProfileLogin retrieves one profile login with automatic retry on transient failures.
func (c *Client) GetProfileLogin(ctx context.Context, loginID int) (*AccountLogin, error) {
	var login *AccountLogin

	err := c.executeWithRetry(ctx, "GetProfileLogin", func() error {
		var err error

		login, err = c.httpGetProfileLogin(ctx, loginID)

		return err
	})

	return login, err
}

// GetProfileLoginProto retrieves one profile login as a proto message with
// automatic retry on transient failures.
func (c *Client) GetProfileLoginProto(ctx context.Context, loginID int) (*linodev1.AccountLogin, error) {
	var login *linodev1.AccountLogin

	err := c.executeWithRetry(ctx, "GetProfileLogin", func() error {
		var err error

		login, err = c.httpGetProfileLoginProto(ctx, loginID)

		return err
	})

	return login, err
}

// GetProfileApp retrieves one authorized OAuth app with automatic retry on transient failures.
func (c *Client) GetProfileApp(ctx context.Context, appID int) (*ProfileApp, error) {
	var app *ProfileApp

	err := c.executeWithRetry(ctx, "GetProfileApp", func() error {
		var err error

		app, err = c.httpGetProfileApp(ctx, appID)

		return err
	})

	return app, err
}

// GetProfileAppProto retrieves one authorized OAuth app as a proto message with
// automatic retry on transient failures.
func (c *Client) GetProfileAppProto(ctx context.Context, appID int) (*linodev1.ProfileApp, error) {
	var app *linodev1.ProfileApp

	err := c.executeWithRetry(ctx, "GetProfileApp", func() error {
		var err error

		app, err = c.httpGetProfileAppProto(ctx, appID)

		return err
	})

	return app, err
}

// DeleteProfileApp revokes access for one OAuth app without retrying the destructive DELETE.
func (c *Client) DeleteProfileApp(ctx context.Context, appID int) error {
	return c.httpDeleteProfileApp(ctx, appID)
}

// GetProfileDevice retrieves one trusted device with automatic retry on transient failures.
func (c *Client) GetProfileDevice(ctx context.Context, deviceID int) (*ProfileDevice, error) {
	var device *ProfileDevice

	err := c.executeWithRetry(ctx, "GetProfileDevice", func() error {
		var err error

		device, err = c.httpGetProfileDevice(ctx, deviceID)

		return err
	})

	return device, err
}

// DeleteProfileDevice revokes one trusted device without retrying the destructive DELETE.
func (c *Client) DeleteProfileDevice(ctx context.Context, deviceID int) error {
	return c.httpDeleteProfileDevice(ctx, deviceID)
}

// GetProfileGrants retrieves the /profile/grants response with retry. Used
// by Phase 6's profile loader to enumerate OAuth scopes; PATs return an
// empty Grants struct here and the loader should inspect Profile.Scopes
// for them instead.
func (c *Client) GetProfileGrants(ctx context.Context) (*Grants, error) {
	var grants *Grants

	err := c.executeWithRetry(ctx, "GetProfileGrants", func() error {
		var err error

		grants, err = c.httpGetProfileGrants(ctx)

		return err
	})

	return grants, err
}

// GetProfileToken retrieves one personal access token with automatic retry on transient failures.
func (c *Client) GetProfileToken(ctx context.Context, tokenID int) (*ProfileToken, error) {
	var token *ProfileToken

	err := c.executeWithRetry(ctx, "GetProfileToken", func() error {
		var err error

		token, err = c.httpGetProfileToken(ctx, tokenID)

		return err
	})

	return token, err
}

// AnswerProfileSecurityQuestions answers profile security questions without retrying
// the mutating request. Retrying can replay security state changes after a transient
// error, so this method delegates exactly once.
func (c *Client) AnswerProfileSecurityQuestions(ctx context.Context, req *AnswerProfileSecurityQuestionsRequest) error {
	return c.httpAnswerProfileSecurityQuestions(ctx, req)
}

// GetProfilePreferences retrieves profile preferences with automatic retry on transient failures.
func (c *Client) GetProfilePreferences(ctx context.Context) (*ProfilePreferences, error) {
	var preferences *ProfilePreferences

	err := c.executeWithRetry(ctx, "GetProfilePreferences", func() error {
		var err error

		preferences, err = c.httpGetProfilePreferences(ctx)

		return err
	})

	return preferences, err
}

// ListProfileApps retrieves OAuth app authorizations with automatic retry on transient failures.
func (c *Client) ListProfileApps(ctx context.Context, page, pageSize int) (*PaginatedResponse[AuthorizedApp], error) {
	var apps *PaginatedResponse[AuthorizedApp]

	err := c.executeWithRetry(ctx, "ListProfileApps", func() error {
		var err error

		apps, err = c.httpListProfileApps(ctx, page, pageSize)

		return err
	})

	return apps, err
}

// ListProfileAppsProto retrieves OAuth app authorizations as proto messages with
// automatic retry on transient failures.
func (c *Client) ListProfileAppsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ProfileApp, error) {
	var apps []*linodev1.ProfileApp

	err := c.executeWithRetry(ctx, "ListProfileApps", func() error {
		var retryErr error

		apps, retryErr = c.httpListProfileAppsProto(ctx, page, pageSize)

		return retryErr
	})

	return apps, err
}

// ListInstances retrieves all instances with automatic retry on transient failures.
func (c *Client) ListInstances(ctx context.Context) ([]Instance, error) {
	var instances []Instance

	err := c.executeWithRetry(ctx, "ListInstances", func() error {
		var err error

		instances, err = c.httpListInstances(ctx)

		return err
	})

	return instances, err
}

// ListInstancesProto retrieves all instances as proto messages with automatic
// retry on transient failures.
func (c *Client) ListInstancesProto(ctx context.Context) ([]*linodev1.Instance, error) {
	var instances []*linodev1.Instance

	err := c.executeWithRetry(ctx, "ListInstances", func() error {
		var err error

		instances, err = c.httpListInstancesProto(ctx)

		return err
	})

	return instances, err
}

// GetInstance retrieves a single instance by ID with automatic retry on transient failures.
func (c *Client) GetInstance(ctx context.Context, instanceID int) (*Instance, error) {
	var instance *Instance

	err := c.executeWithRetry(ctx, "GetInstance", func() error {
		var err error

		instance, err = c.httpGetInstance(ctx, instanceID)

		return err
	})

	return instance, err
}

// GetInstanceProto retrieves a single instance by ID as a proto message with
// automatic retry on transient failures.
func (c *Client) GetInstanceProto(ctx context.Context, instanceID int) (*linodev1.Instance, error) {
	var instance *linodev1.Instance

	err := c.executeWithRetry(ctx, "GetInstance", func() error {
		var err error

		instance, err = c.httpGetInstanceProto(ctx, instanceID)

		return err
	})

	return instance, err
}

// GetInstanceStatsByYearMonth retrieves monthly instance statistics with automatic retry on transient failures.
func (c *Client) GetInstanceStatsByYearMonth(ctx context.Context, linodeID, year, month int) (*InstanceStats, error) {
	var stats *InstanceStats

	err := c.executeWithRetry(ctx, "GetInstanceStatsByYearMonth", func() error {
		var retryErr error

		stats, retryErr = c.httpGetInstanceStatsByYearMonth(ctx, linodeID, year, month)

		return retryErr
	})

	return stats, err
}

// GetInstanceTransfer retrieves monthly transfer statistics with automatic retry on transient failures.
func (c *Client) GetInstanceTransfer(ctx context.Context, linodeID int) (*InstanceTransfer, error) {
	var transfer *InstanceTransfer

	err := c.executeWithRetry(ctx, "GetInstanceTransfer", func() error {
		var err error

		transfer, err = c.httpGetInstanceTransfer(ctx, linodeID)

		return err
	})

	return transfer, err
}

// GetAccount retrieves the account information with automatic retry on transient failures.
func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	var account *Account

	err := c.executeWithRetry(ctx, "GetAccount", func() error {
		var err error

		account, err = c.httpGetAccount(ctx)

		return err
	})

	return account, err
}

// GetAccountProto retrieves the account as a proto message with automatic retry on
// transient failures.
func (c *Client) GetAccountProto(ctx context.Context) (*linodev1.Account, error) {
	var account *linodev1.Account

	err := c.executeWithRetry(ctx, "GetAccount", func() error {
		var err error

		account, err = c.httpGetAccountProto(ctx)

		return err
	})

	return account, err
}

// GetAccountTransfer retrieves account network transfer usage with automatic retry on transient failures.
func (c *Client) GetAccountTransfer(ctx context.Context) (*AccountTransfer, error) {
	var transfer *AccountTransfer

	err := c.executeWithRetry(ctx, "GetAccountTransfer", func() error {
		var err error

		transfer, err = c.httpGetAccountTransfer(ctx)

		return err
	})

	return transfer, err
}

// GetAccountTransferProto retrieves account network transfer usage as a proto
// message with automatic retry on transient failures.
func (c *Client) GetAccountTransferProto(ctx context.Context) (*linodev1.AccountTransfer, error) {
	var transfer *linodev1.AccountTransfer

	err := c.executeWithRetry(ctx, "GetAccountTransfer", func() error {
		var err error

		transfer, err = c.httpGetAccountTransferProto(ctx)

		return err
	})

	return transfer, err
}

// GetAccountSettings retrieves account-wide settings with automatic retry on transient failures.
func (c *Client) GetAccountSettings(ctx context.Context) (*AccountSettings, error) {
	var settings *AccountSettings

	err := c.executeWithRetry(ctx, "GetAccountSettings", func() error {
		var err error

		settings, err = c.httpGetAccountSettings(ctx)

		return err
	})

	return settings, err
}

// GetAccountSettingsProto retrieves account-wide settings as a proto message with
// automatic retry on transient failures.
func (c *Client) GetAccountSettingsProto(ctx context.Context) (*linodev1.AccountSettings, error) {
	var settings *linodev1.AccountSettings

	err := c.executeWithRetry(ctx, "GetAccountSettings", func() error {
		var err error

		settings, err = c.httpGetAccountSettingsProto(ctx)

		return err
	})

	return settings, err
}

// GetLongviewClient retrieves one Longview client with automatic retry on transient failures.
func (c *Client) GetLongviewClient(ctx context.Context, clientID string) (*LongviewClient, error) {
	var client *LongviewClient

	err := c.executeWithRetry(ctx, "GetLongviewClient", func() error {
		var err error

		client, err = c.httpGetLongviewClient(ctx, clientID)

		return err
	})

	return client, err
}

// GetLongviewClientProto retrieves a Longview client as a proto message with
// automatic retry on transient failures.
func (c *Client) GetLongviewClientProto(ctx context.Context, clientID string) (*linodev1.LongviewClient, error) {
	var client *linodev1.LongviewClient

	err := c.executeWithRetry(ctx, "GetLongviewClient", func() error {
		var err error

		client, err = c.httpGetLongviewClientProto(ctx, clientID)

		return err
	})

	return client, err
}

// UpdateAccountSettings updates account-wide settings without retrying the
// mutating request. Retrying can replay account state changes after a transient
// error, so this method delegates exactly once.
func (c *Client) UpdateAccountSettings(ctx context.Context, req *UpdateAccountSettingsRequest) (*AccountSettings, error) {
	return c.httpUpdateAccountSettings(ctx, req)
}

// EnableAccountManaged enables Linode Managed for the account without retrying
// the mutating request. Retrying can replay side effects after a transient
// error, so this method delegates exactly once.
func (c *Client) EnableAccountManaged(ctx context.Context) error {
	return c.httpEnableAccountManaged(ctx)
}

// ListManagedCredentials retrieves stored managed credentials with automatic retry on transient failures.
func (c *Client) ListManagedCredentials(ctx context.Context, page, pageSize int) (*PaginatedResponse[ManagedCredential], error) {
	var credentials *PaginatedResponse[ManagedCredential]

	err := c.executeWithRetry(ctx, "ListManagedCredentials", func() error {
		var err error

		credentials, err = c.httpListManagedCredentials(ctx, page, pageSize)

		return err
	})

	return credentials, err
}

// UpdateManagedCredential updates one stored Managed credential without retrying the
// mutating request. Retrying can replay side effects after a transient error,
// so this method delegates exactly once.
func (c *Client) UpdateManagedCredential(ctx context.Context, credentialID int, req UpdateManagedCredentialRequest) (*ManagedCredential, error) {
	return c.httpUpdateManagedCredential(ctx, credentialID, req)
}

// UpdateManagedCredentialProto updates one stored Managed credential's label and
// decodes the response into the proto element. Like the struct sibling it
// delegates exactly once so a transient error never replays the mutation.
func (c *Client) UpdateManagedCredentialProto(ctx context.Context, credentialID int, req UpdateManagedCredentialRequest) (*linodev1.ManagedCredential, error) {
	return c.httpUpdateManagedCredentialProto(ctx, credentialID, req)
}

// UpdateManagedCredentialUsernamePassword updates one stored Managed credential without retrying
// the mutating username/password request. Retrying can replay side effects after
// a transient error, so this method delegates exactly once.
func (c *Client) UpdateManagedCredentialUsernamePassword(ctx context.Context, credentialID int, req *UpdateManagedCredentialUsernamePasswordRequest) (*ManagedCredential, error) {
	return c.httpUpdateManagedCredentialUsernamePassword(ctx, credentialID, req)
}

// GetManagedSSHKey retrieves the account Managed SSH public key with automatic retry on transient failures.
func (c *Client) GetManagedSSHKey(ctx context.Context) (*ManagedSSHKey, error) {
	var sshKey *ManagedSSHKey

	err := c.executeWithRetry(ctx, "GetManagedSSHKey", func() error {
		var err error

		sshKey, err = c.httpGetManagedSSHKey(ctx)

		return err
	})

	return sshKey, err
}

// CreateManagedCredential creates a stored Managed credential without retrying
// the mutating request. Retrying can replay credential creation after a
// transient error, so this method delegates exactly once.
func (c *Client) CreateManagedCredential(ctx context.Context, request *CreateManagedCredentialRequest) (*ManagedCredential, error) {
	return c.httpCreateManagedCredential(ctx, request)
}

// GetManagedCredential retrieves one stored managed credential with automatic retry on transient failures.
func (c *Client) GetManagedCredential(ctx context.Context, credentialID int) (*ManagedCredential, error) {
	var credential *ManagedCredential

	err := c.executeWithRetry(ctx, "GetManagedCredential", func() error {
		var err error

		credential, err = c.httpGetManagedCredential(ctx, credentialID)

		return err
	})

	return credential, err
}

// GetManagedCredentialProto retrieves a Managed credential as a proto message with
// automatic retry on transient failures.
func (c *Client) GetManagedCredentialProto(ctx context.Context, credentialID int) (*linodev1.ManagedCredential, error) {
	var credential *linodev1.ManagedCredential

	err := c.executeWithRetry(ctx, "GetManagedCredential", func() error {
		var err error

		credential, err = c.httpGetManagedCredentialProto(ctx, credentialID)

		return err
	})

	return credential, err
}

// RevokeManagedCredential revokes one stored managed credential without retrying
// the mutating request. Retrying can replay credential revocation after a
// transient error, so this method delegates exactly once.
func (c *Client) RevokeManagedCredential(ctx context.Context, credentialID int) error {
	return c.httpRevokeManagedCredential(ctx, credentialID)
}

// GetManagedLinodeSettings retrieves Managed settings for one Linode with automatic retry on transient failures.
func (c *Client) GetManagedLinodeSettings(ctx context.Context, linodeID int) (*ManagedLinodeSettings, error) {
	var settings *ManagedLinodeSettings

	err := c.executeWithRetry(ctx, "GetManagedLinodeSettings", func() error {
		var err error

		settings, err = c.httpGetManagedLinodeSettings(ctx, linodeID)

		return err
	})

	return settings, err
}

// GetManagedLinodeSettingsProto retrieves Managed Linode settings as a proto
// message with automatic retry on transient failures.
func (c *Client) GetManagedLinodeSettingsProto(ctx context.Context, linodeID int) (*linodev1.ManagedLinodeSettings, error) {
	var settings *linodev1.ManagedLinodeSettings

	err := c.executeWithRetry(ctx, "GetManagedLinodeSettings", func() error {
		var err error

		settings, err = c.httpGetManagedLinodeSettingsProto(ctx, linodeID)

		return err
	})

	return settings, err
}

// GetManagedContact retrieves one managed contact with automatic retry on transient failures.
func (c *Client) GetManagedContact(ctx context.Context, contactID int) (*ManagedContact, error) {
	var contact *ManagedContact

	err := c.executeWithRetry(ctx, "GetManagedContact", func() error {
		var err error

		contact, err = c.httpGetManagedContact(ctx, contactID)

		return err
	})

	return contact, err
}

// GetManagedContactProto retrieves a Managed contact as a proto message with
// automatic retry on transient failures.
func (c *Client) GetManagedContactProto(ctx context.Context, contactID int) (*linodev1.ManagedContact, error) {
	var contact *linodev1.ManagedContact

	err := c.executeWithRetry(ctx, "GetManagedContact", func() error {
		var err error

		contact, err = c.httpGetManagedContactProto(ctx, contactID)

		return err
	})

	return contact, err
}

// GetAccountAgreements retrieves account agreement acknowledgment status with automatic retry on transient failures.
func (c *Client) GetAccountAgreements(ctx context.Context) (*AccountAgreements, error) {
	var agreements *AccountAgreements

	err := c.executeWithRetry(ctx, "GetAccountAgreements", func() error {
		var err error

		agreements, err = c.httpGetAccountAgreements(ctx)

		return err
	})

	return agreements, err
}

// GetAccountAgreementsProto retrieves account agreement acknowledgment status as
// a proto message with automatic retry on transient failures.
func (c *Client) GetAccountAgreementsProto(ctx context.Context) (*linodev1.AccountAgreements, error) {
	var agreements *linodev1.AccountAgreements

	err := c.executeWithRetry(ctx, "GetAccountAgreements", func() error {
		var err error

		agreements, err = c.httpGetAccountAgreementsProto(ctx)

		return err
	})

	return agreements, err
}

// ListAccountMaintenance retrieves account maintenance records with automatic retry on transient failures.
func (c *Client) ListAccountMaintenance(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountMaintenance], error) {
	var maintenance *PaginatedResponse[AccountMaintenance]

	err := c.executeWithRetry(ctx, "ListAccountMaintenance", func() error {
		var err error

		maintenance, err = c.httpListAccountMaintenance(ctx, page, pageSize)

		return err
	})

	return maintenance, err
}

// ListAccountMaintenanceProto retrieves account maintenance records as proto
// messages with automatic retry on transient failures.
func (c *Client) ListAccountMaintenanceProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountMaintenance, error) {
	var maintenance []*linodev1.AccountMaintenance

	err := c.executeWithRetry(ctx, "ListAccountMaintenance", func() error {
		var retryErr error

		maintenance, retryErr = c.httpListAccountMaintenanceProto(ctx, page, pageSize)

		return retryErr
	})

	return maintenance, err
}

// ListMaintenancePolicies retrieves available Linode maintenance policies with automatic retry on transient failures.
func (c *Client) ListMaintenancePolicies(ctx context.Context, page, pageSize int) (*PaginatedResponse[MaintenancePolicy], error) {
	var policies *PaginatedResponse[MaintenancePolicy]

	err := c.executeWithRetry(ctx, "ListMaintenancePolicies", func() error {
		var err error

		policies, err = c.httpListMaintenancePolicies(ctx, page, pageSize)

		return err
	})

	return policies, err
}

// ListMaintenancePoliciesProto retrieves available Linode maintenance policies as
// proto messages with automatic retry on transient failures.
func (c *Client) ListMaintenancePoliciesProto(ctx context.Context, page, pageSize int) ([]*linodev1.MaintenancePolicy, error) {
	var policies []*linodev1.MaintenancePolicy

	err := c.executeWithRetry(ctx, "ListMaintenancePolicies", func() error {
		var retryErr error

		policies, retryErr = c.httpListMaintenancePoliciesProto(ctx, page, pageSize)

		return retryErr
	})

	return policies, err
}

// DeleteManagedContact deletes a Managed contact without retrying the destructive request.
func (c *Client) DeleteManagedContact(ctx context.Context, contactID int) error {
	return c.executeWithoutRetry(ctx, "DeleteManagedContact", func() error {
		return c.httpDeleteManagedContact(ctx, contactID)
	})
}

// ListManagedContacts retrieves Managed contacts with automatic retry on transient failures.
func (c *Client) ListManagedContacts(ctx context.Context, page, pageSize int) (*PaginatedResponse[ManagedContact], error) {
	var contacts *PaginatedResponse[ManagedContact]

	err := c.executeWithRetry(ctx, "ListManagedContacts", func() error {
		var err error

		contacts, err = c.httpListManagedContacts(ctx, page, pageSize)

		return err
	})

	return contacts, err
}

// GetManagedStats retrieves Managed statistics with automatic retry on transient failures.
func (c *Client) GetManagedStats(ctx context.Context) (map[string]any, error) {
	var stats map[string]any

	err := c.executeWithRetry(ctx, "GetManagedStats", func() error {
		var err error

		stats, err = c.httpGetManagedStats(ctx)

		return err
	})

	return stats, err
}

// GetManagedIssue retrieves one Managed issue with automatic retry on transient failures.
func (c *Client) GetManagedIssue(ctx context.Context, issueID int) (*ManagedIssue, error) {
	var issue *ManagedIssue

	err := c.executeWithRetry(ctx, "GetManagedIssue", func() error {
		var err error

		issue, err = c.httpGetManagedIssue(ctx, issueID)

		return err
	})

	return issue, err
}

// GetManagedIssueProto retrieves one Managed issue as a proto message with
// automatic retry on transient failures.
func (c *Client) GetManagedIssueProto(ctx context.Context, issueID int) (*linodev1.ManagedIssue, error) {
	var issue *linodev1.ManagedIssue

	err := c.executeWithRetry(ctx, "GetManagedIssue", func() error {
		var err error

		issue, err = c.httpGetManagedIssueProto(ctx, issueID)

		return err
	})

	return issue, err
}

// ListManagedLinodeSettings retrieves Managed Linode settings with automatic retry on transient failures.
func (c *Client) ListManagedLinodeSettings(ctx context.Context, page, pageSize int) (*PaginatedResponse[ManagedLinodeSettings], error) {
	var settings *PaginatedResponse[ManagedLinodeSettings]

	err := c.executeWithRetry(ctx, "ListManagedLinodeSettings", func() error {
		var err error

		settings, err = c.httpListManagedLinodeSettings(ctx, page, pageSize)

		return err
	})

	return settings, err
}

// UpdateManagedLinodeSettings updates Managed Linode settings without retrying the mutating request.
func (c *Client) UpdateManagedLinodeSettings(ctx context.Context, linodeID int, req UpdateManagedLinodeSettingsRequest) (*ManagedLinodeSettings, error) {
	var settings *ManagedLinodeSettings

	err := c.executeWithoutRetry(ctx, "UpdateManagedLinodeSettings", func() error {
		var err error

		settings, err = c.httpUpdateManagedLinodeSettings(ctx, linodeID, req)

		return err
	})

	return settings, err
}

// UpdateManagedLinodeSettingsProto updates Managed Linode settings and decodes the
// response into the proto element. Like the struct sibling it does not retry, so a
// transient error never replays the mutation.
func (c *Client) UpdateManagedLinodeSettingsProto(ctx context.Context, linodeID int, req UpdateManagedLinodeSettingsRequest) (*linodev1.ManagedLinodeSettings, error) {
	var settings *linodev1.ManagedLinodeSettings

	err := c.executeWithoutRetry(ctx, "UpdateManagedLinodeSettings", func() error {
		var err error

		settings, err = c.httpUpdateManagedLinodeSettingsProto(ctx, linodeID, req)

		return err
	})

	return settings, err
}

// GetManagedService retrieves one Managed service with automatic retry on transient failures.
func (c *Client) GetManagedService(ctx context.Context, serviceID int) (*ManagedService, error) {
	var service *ManagedService

	err := c.executeWithRetry(ctx, "GetManagedService", func() error {
		var err error

		service, err = c.httpGetManagedService(ctx, serviceID)

		return err
	})

	return service, err
}

// GetManagedServiceProto retrieves a Managed service as a proto message with
// automatic retry on transient failures.
func (c *Client) GetManagedServiceProto(ctx context.Context, serviceID int) (*linodev1.ManagedService, error) {
	var service *linodev1.ManagedService

	err := c.executeWithRetry(ctx, "GetManagedService", func() error {
		var err error

		service, err = c.httpGetManagedServiceProto(ctx, serviceID)

		return err
	})

	return service, err
}

// UpdateManagedService updates one Managed service monitor without retrying the
// mutating request. Managed service updates are not guaranteed idempotent after a
// transient error, so this method delegates exactly once.
func (c *Client) UpdateManagedService(ctx context.Context, serviceID int, request *UpdateManagedServiceRequest) (*ManagedService, error) {
	var service *ManagedService

	err := c.executeWithoutRetry(ctx, "UpdateManagedService", func() error {
		var retryErr error

		service, retryErr = c.httpUpdateManagedService(ctx, serviceID, request)

		return retryErr
	})

	return service, err
}

// DeleteManagedService deletes a Managed service monitor without retrying the
// destructive request. Managed service deletion is not replay-safe after a
// transient error, so this method delegates exactly once.
func (c *Client) DeleteManagedService(ctx context.Context, serviceID int) error {
	return c.executeWithoutRetry(ctx, "DeleteManagedService", func() error {
		return c.httpDeleteManagedService(ctx, serviceID)
	})
}

// DisableManagedService disables one Managed service monitor without retrying the
// mutating request. Disabling a monitor is not replay-safe after a transient
// error, so this method delegates exactly once.
func (c *Client) DisableManagedService(ctx context.Context, serviceID int) error {
	return c.executeWithoutRetry(ctx, "DisableManagedService", func() error {
		return c.httpDisableManagedService(ctx, serviceID)
	})
}

// EnableManagedService enables one Managed service monitor without retrying the
// mutating request. Enabling a monitor is not replay-safe after a transient
// error, so this method delegates exactly once.
func (c *Client) EnableManagedService(ctx context.Context, serviceID int) error {
	return c.executeWithoutRetry(ctx, "EnableManagedService", func() error {
		return c.httpEnableManagedService(ctx, serviceID)
	})
}

// ListManagedServices retrieves Managed services with automatic retry on transient failures.
func (c *Client) ListManagedServices(ctx context.Context, page, pageSize int) (*PaginatedResponse[ManagedService], error) {
	var services *PaginatedResponse[ManagedService]

	err := c.executeWithRetry(ctx, "ListManagedServices", func() error {
		var err error

		services, err = c.httpListManagedServices(ctx, page, pageSize)

		return err
	})

	return services, err
}

// ListManagedServicesProto retrieves Managed services as proto messages with
// automatic retry on transient failures.
func (c *Client) ListManagedServicesProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedService, error) {
	var services []*linodev1.ManagedService

	err := c.executeWithRetry(ctx, "ListManagedServices", func() error {
		var err error

		services, err = c.httpListManagedServicesProto(ctx, page, pageSize)

		return err
	})

	return services, err
}

// ListManagedContactsProto retrieves Managed contacts as proto messages with
// automatic retry on transient failures.
func (c *Client) ListManagedContactsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedContact, error) {
	var contacts []*linodev1.ManagedContact

	err := c.executeWithRetry(ctx, "ListManagedContacts", func() error {
		var err error

		contacts, err = c.httpListManagedContactsProto(ctx, page, pageSize)

		return err
	})

	return contacts, err
}

// ListManagedCredentialsProto retrieves stored managed credentials as proto
// messages with automatic retry on transient failures.
func (c *Client) ListManagedCredentialsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedCredential, error) {
	var credentials []*linodev1.ManagedCredential

	err := c.executeWithRetry(ctx, "ListManagedCredentials", func() error {
		var err error

		credentials, err = c.httpListManagedCredentialsProto(ctx, page, pageSize)

		return err
	})

	return credentials, err
}

// ListManagedLinodeSettingsProto retrieves Managed Linode settings as proto
// messages with automatic retry on transient failures.
func (c *Client) ListManagedLinodeSettingsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedLinodeSettings, error) {
	var settings []*linodev1.ManagedLinodeSettings

	err := c.executeWithRetry(ctx, "ListManagedLinodeSettings", func() error {
		var err error

		settings, err = c.httpListManagedLinodeSettingsProto(ctx, page, pageSize)

		return err
	})

	return settings, err
}

// ListManagedIssuesProto retrieves Managed issues as proto messages with
// automatic retry on transient failures.
func (c *Client) ListManagedIssuesProto(ctx context.Context, page, pageSize int) ([]*linodev1.ManagedIssue, error) {
	var issues []*linodev1.ManagedIssue

	err := c.executeWithRetry(ctx, "ListManagedIssues", func() error {
		var err error

		issues, err = c.httpListManagedIssuesProto(ctx, page, pageSize)

		return err
	})

	return issues, err
}

// ListManagedIssues retrieves Managed issues with automatic retry on transient failures.
func (c *Client) ListManagedIssues(ctx context.Context, page, pageSize int) (*PaginatedResponse[ManagedIssue], error) {
	var issues *PaginatedResponse[ManagedIssue]

	err := c.executeWithRetry(ctx, "ListManagedIssues", func() error {
		var err error

		issues, err = c.httpListManagedIssues(ctx, page, pageSize)

		return err
	})

	return issues, err
}

// UpdateManagedContact updates one Managed contact without retrying the mutating request.
func (c *Client) UpdateManagedContact(ctx context.Context, contactID int, req UpdateManagedContactRequest) (*ManagedContact, error) {
	return c.httpUpdateManagedContact(ctx, contactID, req)
}

// UpdateManagedContactProto updates one Managed contact and decodes the response
// into the proto element. Like the struct sibling it delegates exactly once so a
// transient error never replays the mutation.
func (c *Client) UpdateManagedContactProto(ctx context.Context, contactID int, req UpdateManagedContactRequest) (*linodev1.ManagedContact, error) {
	return c.httpUpdateManagedContactProto(ctx, contactID, req)
}

// ListAccountNotifications retrieves account notifications with automatic retry on transient failures.
func (c *Client) ListAccountNotifications(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountNotification], error) {
	var notifications *PaginatedResponse[AccountNotification]

	err := c.executeWithRetry(ctx, "ListAccountNotifications", func() error {
		var err error

		notifications, err = c.httpListAccountNotifications(ctx, page, pageSize)

		return err
	})

	return notifications, err
}

// ListAccountNotificationsProto retrieves account notifications as proto
// messages with automatic retry on transient failures.
func (c *Client) ListAccountNotificationsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountNotification, error) {
	var notifications []*linodev1.AccountNotification

	err := c.executeWithRetry(ctx, "ListAccountNotifications", func() error {
		var retryErr error

		notifications, retryErr = c.httpListAccountNotificationsProto(ctx, page, pageSize)

		return retryErr
	})

	return notifications, err
}

// ListAccountPaymentMethodsProto retrieves account payment methods as proto
// messages with automatic retry on transient failures.
func (c *Client) ListAccountPaymentMethodsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountPaymentMethod, error) {
	var methods []*linodev1.AccountPaymentMethod

	err := c.executeWithRetry(ctx, "ListAccountPaymentMethods", func() error {
		var retryErr error

		methods, retryErr = c.httpListAccountPaymentMethodsProto(ctx, page, pageSize)

		return retryErr
	})

	return methods, err
}

// ListAccountInvoiceItemsProto retrieves an invoice's line items as proto
// messages with automatic retry on transient failures.
func (c *Client) ListAccountInvoiceItemsProto(ctx context.Context, invoiceID, page, pageSize int) ([]*linodev1.AccountInvoiceItem, error) {
	var items []*linodev1.AccountInvoiceItem

	err := c.executeWithRetry(ctx, "ListAccountInvoiceItems", func() error {
		var retryErr error

		items, retryErr = c.httpListAccountInvoiceItemsProto(ctx, invoiceID, page, pageSize)

		return retryErr
	})

	return items, err
}

// ListAccountChildAccountsProto retrieves child-level accounts as proto messages
// with automatic retry on transient failures.
func (c *Client) ListAccountChildAccountsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ChildAccount, error) {
	var childAccounts []*linodev1.ChildAccount

	err := c.executeWithRetry(ctx, "ListAccountChildAccounts", func() error {
		var retryErr error

		childAccounts, retryErr = c.httpListAccountChildAccountsProto(ctx, page, pageSize)

		return retryErr
	})

	return childAccounts, err
}

// ListProfileDevicesProto retrieves trusted devices as proto messages with
// automatic retry on transient failures.
func (c *Client) ListProfileDevicesProto(ctx context.Context, page, pageSize int) ([]*linodev1.TrustedDevice, error) {
	var devices []*linodev1.TrustedDevice

	err := c.executeWithRetry(ctx, "ListProfileDevices", func() error {
		var retryErr error

		devices, retryErr = c.httpListProfileDevicesProto(ctx, page, pageSize)

		return retryErr
	})

	return devices, err
}

// ListProfileTokensProto retrieves personal access token metadata as proto
// messages with automatic retry on transient failures.
func (c *Client) ListProfileTokensProto(ctx context.Context, page, pageSize int) ([]*linodev1.PersonalAccessToken, error) {
	var tokens []*linodev1.PersonalAccessToken

	err := c.executeWithRetry(ctx, "ListProfileTokens", func() error {
		var retryErr error

		tokens, retryErr = c.httpListProfileTokensProto(ctx, page, pageSize)

		return retryErr
	})

	return tokens, err
}

// ListProfileSecurityQuestionsProto retrieves the profile security questions as
// proto messages with automatic retry on transient failures.
func (c *Client) ListProfileSecurityQuestionsProto(ctx context.Context) ([]*linodev1.SecurityQuestion, error) {
	var questions []*linodev1.SecurityQuestion

	err := c.executeWithRetry(ctx, "ListProfileSecurityQuestions", func() error {
		var retryErr error

		questions, retryErr = c.httpListProfileSecurityQuestionsProto(ctx)

		return retryErr
	})

	return questions, err
}

// GetAccountAvailability retrieves account service availability for a region with automatic retry on transient failures.
func (c *Client) GetAccountAvailability(ctx context.Context, regionID string) (*AccountAvailability, error) {
	var availability *AccountAvailability

	err := c.executeWithRetry(ctx, "GetAccountAvailability", func() error {
		var err error

		availability, err = c.httpGetAccountAvailability(ctx, regionID)

		return err
	})

	return availability, err
}

// GetAccountAvailabilityProto retrieves one region's account availability as a
// proto message with automatic retry on transient failures.
func (c *Client) GetAccountAvailabilityProto(ctx context.Context, regionID string) (*linodev1.AccountAvailability, error) {
	var availability *linodev1.AccountAvailability

	err := c.executeWithRetry(ctx, "GetAccountAvailability", func() error {
		var err error

		availability, err = c.httpGetAccountAvailabilityProto(ctx, regionID)

		return err
	})

	return availability, err
}

// ListAccountAvailability retrieves account service availability with automatic retry on transient failures.
func (c *Client) ListAccountAvailability(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountAvailability], error) {
	var availability *PaginatedResponse[AccountAvailability]

	err := c.executeWithRetry(ctx, "ListAccountAvailability", func() error {
		var err error

		availability, err = c.httpListAccountAvailability(ctx, page, pageSize)

		return err
	})

	return availability, err
}

// ListAccountAvailabilityProto retrieves account service availability as proto
// messages with automatic retry on transient failures.
func (c *Client) ListAccountAvailabilityProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountAvailability, error) {
	var availability []*linodev1.AccountAvailability

	err := c.executeWithRetry(ctx, "ListAccountAvailability", func() error {
		var retryErr error

		availability, retryErr = c.httpListAccountAvailabilityProto(ctx, page, pageSize)

		return retryErr
	})

	return availability, err
}

// ListBetas retrieves available beta programs with automatic retry on transient failures.
func (c *Client) ListBetas(ctx context.Context, page, pageSize int) (*PaginatedResponse[BetaProgram], error) {
	var betas *PaginatedResponse[BetaProgram]

	err := c.executeWithRetry(ctx, "ListBetas", func() error {
		var err error

		betas, err = c.httpListBetas(ctx, page, pageSize)

		return err
	})

	return betas, err
}

// ListBetasProto retrieves available beta programs as proto messages with
// automatic retry on transient failures.
func (c *Client) ListBetasProto(ctx context.Context, page, pageSize int) ([]*linodev1.BetaProgram, error) {
	var betas []*linodev1.BetaProgram

	err := c.executeWithRetry(ctx, "ListBetas", func() error {
		var retryErr error

		betas, retryErr = c.httpListBetasProto(ctx, page, pageSize)

		return retryErr
	})

	return betas, err
}

// GetBeta retrieves one available beta program with automatic retry on transient failures.
func (c *Client) GetBeta(ctx context.Context, betaID string) (*BetaProgram, error) {
	var beta *BetaProgram

	err := c.executeWithRetry(ctx, "GetBeta", func() error {
		var err error

		beta, err = c.httpGetBeta(ctx, betaID)

		return err
	})

	return beta, err
}

// GetBetaProto retrieves one available beta program as a proto message with
// automatic retry on transient failures.
func (c *Client) GetBetaProto(ctx context.Context, betaID string) (*linodev1.BetaProgram, error) {
	var beta *linodev1.BetaProgram

	err := c.executeWithRetry(ctx, "GetBeta", func() error {
		var err error

		beta, err = c.httpGetBetaProto(ctx, betaID)

		return err
	})

	return beta, err
}

// ListAccountBetas retrieves enrolled account beta programs with automatic retry on transient failures.
func (c *Client) ListAccountBetas(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountBetaProgram], error) {
	var betas *PaginatedResponse[AccountBetaProgram]

	err := c.executeWithRetry(ctx, "ListAccountBetas", func() error {
		var err error

		betas, err = c.httpListAccountBetas(ctx, page, pageSize)

		return err
	})

	return betas, err
}

// ListAccountBetasProto retrieves enrolled account beta programs as proto
// messages with automatic retry on transient failures.
func (c *Client) ListAccountBetasProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountBetaProgram, error) {
	var betas []*linodev1.AccountBetaProgram

	err := c.executeWithRetry(ctx, "ListAccountBetas", func() error {
		var retryErr error

		betas, retryErr = c.httpListAccountBetasProto(ctx, page, pageSize)

		return retryErr
	})

	return betas, err
}

// ListAccountOAuthClients retrieves OAuth clients with automatic retry on transient failures.
func (c *Client) ListAccountOAuthClients(ctx context.Context, page, pageSize int) (*PaginatedResponse[OAuthClient], error) {
	var clients *PaginatedResponse[OAuthClient]

	err := c.executeWithRetry(ctx, "ListAccountOAuthClients", func() error {
		var err error

		clients, err = c.httpListAccountOAuthClients(ctx, page, pageSize)

		return err
	})

	return clients, err
}

// ListAccountOAuthClientsProto retrieves OAuth clients as proto messages with
// automatic retry on transient failures.
func (c *Client) ListAccountOAuthClientsProto(ctx context.Context, page, pageSize int) ([]*linodev1.OAuthClient, error) {
	var clients []*linodev1.OAuthClient

	err := c.executeWithRetry(ctx, "ListAccountOAuthClients", func() error {
		var retryErr error

		clients, retryErr = c.httpListAccountOAuthClientsProto(ctx, page, pageSize)

		return retryErr
	})

	return clients, err
}

// GetLongviewPlan retrieves the Longview subscription plan with automatic retry on transient failures.
func (c *Client) GetLongviewPlan(ctx context.Context) (*LongviewSubscription, error) {
	var plan *LongviewSubscription

	err := c.executeWithRetry(ctx, "GetLongviewPlan", func() error {
		var err error

		plan, err = c.httpGetLongviewPlan(ctx)

		return err
	})

	return plan, err
}

// ListLongviewTypes retrieves the available Longview subscription types with automatic retry on transient failures.
func (c *Client) ListLongviewTypes(ctx context.Context) (*PaginatedResponse[LongviewType], error) {
	var types *PaginatedResponse[LongviewType]

	err := c.executeWithRetry(ctx, "ListLongviewTypes", func() error {
		var err error

		types, err = c.httpListLongviewTypes(ctx)

		return err
	})

	return types, err
}

// ListLongviewTypesProto retrieves the available Longview subscription types as
// proto messages with automatic retry on transient failures.
func (c *Client) ListLongviewTypesProto(ctx context.Context) ([]*linodev1.LongviewType, error) {
	var types []*linodev1.LongviewType

	err := c.executeWithRetry(ctx, "ListLongviewTypes", func() error {
		var err error

		types, err = c.httpListLongviewTypesProto(ctx)

		return err
	})

	return types, err
}

// ListMonitorServicesProto retrieves supported monitoring service types as proto
// messages with automatic retry on transient failures.
func (c *Client) ListMonitorServicesProto(ctx context.Context) ([]*linodev1.MonitorService, error) {
	var services []*linodev1.MonitorService

	err := c.executeWithRetry(ctx, "ListMonitorServices", func() error {
		var err error

		services, err = c.httpListMonitorServicesProto(ctx)

		return err
	})

	return services, err
}

// GetMonitorService retrieves details for one supported monitoring service type with automatic retry on transient failures.
func (c *Client) GetMonitorService(ctx context.Context, serviceType string) (MonitorService, error) {
	var service MonitorService

	err := c.executeWithRetry(ctx, "GetMonitorService", func() error {
		var err error

		service, err = c.httpGetMonitorService(ctx, serviceType)

		return err
	})

	return service, err
}

// GetMonitorServiceProto retrieves a Monitor service as a proto message with
// automatic retry on transient failures.
func (c *Client) GetMonitorServiceProto(ctx context.Context, serviceType string) (*linodev1.MonitorService, error) {
	var service *linodev1.MonitorService

	err := c.executeWithRetry(ctx, "GetMonitorService", func() error {
		var err error

		service, err = c.httpGetMonitorServiceProto(ctx, serviceType)

		return err
	})

	return service, err
}

// ListMonitorServiceMetricDefinitions retrieves metric definitions for one monitoring service type with automatic retry on transient failures.
func (c *Client) ListMonitorServiceMetricDefinitions(ctx context.Context, serviceType string) (*PaginatedResponse[MonitorMetricDefinition], error) {
	var definitions *PaginatedResponse[MonitorMetricDefinition]

	err := c.executeWithRetry(ctx, "ListMonitorServiceMetricDefinitions", func() error {
		var err error

		definitions, err = c.httpListMonitorServiceMetricDefinitions(ctx, serviceType)

		return err
	})

	return definitions, err
}

// ListMonitorServiceAlertDefinitions retrieves alert definitions for one monitoring service type with automatic retry on transient failures.
func (c *Client) ListMonitorServiceAlertDefinitions(ctx context.Context, serviceType string) (*PaginatedResponse[AlertDefinition], error) {
	var definitions *PaginatedResponse[AlertDefinition]

	err := c.executeWithRetry(ctx, "ListMonitorServiceAlertDefinitions", func() error {
		var err error

		definitions, err = c.httpListMonitorServiceAlertDefinitions(ctx, serviceType)

		return err
	})

	return definitions, err
}

// ListMonitorServiceMetricDefinitionsProto retrieves metric definitions for one
// monitoring service type as proto messages with automatic retry on transient
// failures.
func (c *Client) ListMonitorServiceMetricDefinitionsProto(ctx context.Context, serviceType string) ([]*linodev1.MonitorMetricDefinition, error) {
	var definitions []*linodev1.MonitorMetricDefinition

	err := c.executeWithRetry(ctx, "ListMonitorServiceMetricDefinitions", func() error {
		var err error

		definitions, err = c.httpListMonitorServiceMetricDefinitionsProto(ctx, serviceType)

		return err
	})

	return definitions, err
}

// ListMonitorServiceAlertDefinitionsProto retrieves alert definitions for one
// monitoring service type as proto messages with automatic retry on transient
// failures.
func (c *Client) ListMonitorServiceAlertDefinitionsProto(ctx context.Context, serviceType string) ([]*linodev1.MonitorAlertDefinition, error) {
	var definitions []*linodev1.MonitorAlertDefinition

	err := c.executeWithRetry(ctx, "ListMonitorServiceAlertDefinitions", func() error {
		var err error

		definitions, err = c.httpListMonitorServiceAlertDefinitionsProto(ctx, serviceType)

		return err
	})

	return definitions, err
}

// ListMonitorServiceDashboardsProto retrieves dashboards for one monitoring
// service type as proto messages with automatic retry on transient failures.
func (c *Client) ListMonitorServiceDashboardsProto(ctx context.Context, serviceType string) ([]*linodev1.MonitorDashboard, error) {
	var dashboards []*linodev1.MonitorDashboard

	err := c.executeWithRetry(ctx, "ListMonitorServiceDashboards", func() error {
		var retryErr error

		dashboards, retryErr = c.httpListMonitorServiceDashboardsProto(ctx, serviceType)

		return retryErr
	})

	return dashboards, err
}

// GetMonitorServiceMetrics retrieves metrics for one monitoring service type without retrying
// the POST request. The operation is read-style, but POST transport can carry entity
// query bodies, so this method delegates exactly once after transient failures.
func (c *Client) GetMonitorServiceMetrics(ctx context.Context, serviceType string) (MonitorMetrics, error) {
	var metrics MonitorMetrics

	err := c.executeWithoutRetry(ctx, "GetMonitorServiceMetrics", func() error {
		var retryErr error

		metrics, retryErr = c.httpGetMonitorServiceMetrics(ctx, serviceType)

		return retryErr
	})

	return metrics, err
}

// CreateMonitorServiceToken creates a service token without retrying
// the token-creating request. Token creation is not guaranteed idempotent
// after a transient error, so this method delegates exactly once.
func (c *Client) CreateMonitorServiceToken(ctx context.Context, serviceType string, request *CreateMonitorServiceTokenRequest) (*MonitorServiceToken, error) {
	var token *MonitorServiceToken

	err := c.executeWithoutRetry(ctx, "CreateMonitorServiceToken", func() error {
		var retryErr error

		token, retryErr = c.httpCreateMonitorServiceToken(ctx, serviceType, request)

		return retryErr
	})

	return token, err
}

// CreateMonitorServiceAlertDefinition creates an alert definition without retrying
// the mutating request. Alert definition creation is not guaranteed idempotent
// after a transient error, so this method delegates exactly once.
func (c *Client) CreateMonitorServiceAlertDefinition(ctx context.Context, serviceType string, request *CreateAlertDefinitionRequest) (*AlertDefinition, error) {
	var definition *AlertDefinition

	err := c.executeWithoutRetry(ctx, "CreateMonitorServiceAlertDefinition", func() error {
		var retryErr error

		definition, retryErr = c.httpCreateMonitorServiceAlertDefinition(ctx, serviceType, request)

		return retryErr
	})

	return definition, err
}

// CreateMonitorServiceAlertDefinitionProto creates an alert definition and
// decodes the response into the MonitorAlertDefinition proto element without
// retrying the mutating request, matching CreateMonitorServiceAlertDefinition's
// once-only semantics.
func (c *Client) CreateMonitorServiceAlertDefinitionProto(ctx context.Context, serviceType string, request *CreateAlertDefinitionRequest) (*linodev1.MonitorAlertDefinition, error) {
	var definition *linodev1.MonitorAlertDefinition

	err := c.executeWithoutRetry(ctx, "CreateMonitorServiceAlertDefinition", func() error {
		var retryErr error

		definition, retryErr = c.httpCreateMonitorServiceAlertDefinitionProto(ctx, serviceType, request)

		return retryErr
	})

	return definition, err
}

// GetMonitorServiceAlertDefinition retrieves one alert definition for one monitoring service type with automatic retry on transient failures.
func (c *Client) GetMonitorServiceAlertDefinition(ctx context.Context, serviceType string, alertID int) (AlertDefinition, error) {
	var definition AlertDefinition

	err := c.executeWithRetry(ctx, "GetMonitorServiceAlertDefinition", func() error {
		var err error

		definition, err = c.httpGetMonitorServiceAlertDefinition(ctx, serviceType, alertID)

		return err
	})

	return definition, err
}

// DeleteMonitorServiceAlertDefinition deletes one alert definition without retrying
// the destructive request. Alert definition deletion is not guaranteed idempotent
// after a transient error, so this method delegates exactly once.
func (c *Client) DeleteMonitorServiceAlertDefinition(ctx context.Context, serviceType string, alertID int) error {
	return c.executeWithoutRetry(ctx, "DeleteMonitorServiceAlertDefinition", func() error {
		return c.httpDeleteMonitorServiceAlertDefinition(ctx, serviceType, alertID)
	})
}

// UpdateMonitorServiceAlertDefinition updates an alert definition without retrying
// the mutating request. Alert definition updates can change notification state,
// so this method delegates exactly once after transient failures.
func (c *Client) UpdateMonitorServiceAlertDefinition(ctx context.Context, serviceType string, alertID int, request *UpdateAlertDefinitionRequest) (*AlertDefinition, error) {
	var definition *AlertDefinition

	err := c.executeWithoutRetry(ctx, "UpdateMonitorServiceAlertDefinition", func() error {
		var retryErr error

		definition, retryErr = c.httpUpdateMonitorServiceAlertDefinition(ctx, serviceType, alertID, request)

		return retryErr
	})

	return definition, err
}

// UpdateMonitorServiceAlertDefinitionProto updates an alert definition and
// decodes the response into the MonitorAlertDefinition proto element without
// retrying the mutating request, matching UpdateMonitorServiceAlertDefinition's
// once-only semantics.
func (c *Client) UpdateMonitorServiceAlertDefinitionProto(ctx context.Context, serviceType string, alertID int, request *UpdateAlertDefinitionRequest) (*linodev1.MonitorAlertDefinition, error) {
	var definition *linodev1.MonitorAlertDefinition

	err := c.executeWithoutRetry(ctx, "UpdateMonitorServiceAlertDefinition", func() error {
		var retryErr error

		definition, retryErr = c.httpUpdateMonitorServiceAlertDefinitionProto(ctx, serviceType, alertID, request)

		return retryErr
	})

	return definition, err
}

// ListMonitorDashboardsProto retrieves monitoring dashboards as proto messages
// with automatic retry on transient failures.
func (c *Client) ListMonitorDashboardsProto(ctx context.Context, page, pageSize int) ([]*linodev1.MonitorDashboard, error) {
	var dashboards []*linodev1.MonitorDashboard

	err := c.executeWithRetry(ctx, "ListMonitorDashboards", func() error {
		var retryErr error

		dashboards, retryErr = c.httpListMonitorDashboardsProto(ctx, page, pageSize)

		return retryErr
	})

	return dashboards, err
}

// GetMonitorDashboard retrieves one monitoring dashboard with automatic retry on transient failures.
func (c *Client) GetMonitorDashboard(ctx context.Context, dashboardID int) (MonitorDashboard, error) {
	var dashboard MonitorDashboard

	err := c.executeWithRetry(ctx, "GetMonitorDashboard", func() error {
		var err error

		dashboard, err = c.httpGetMonitorDashboard(ctx, dashboardID)

		return err
	})

	return dashboard, err
}

// ListMonitorAlertDefinitionsProto retrieves monitoring alert definitions as
// proto messages with automatic retry on transient failures.
func (c *Client) ListMonitorAlertDefinitionsProto(ctx context.Context, page, pageSize int) ([]*linodev1.MonitorAlertDefinition, error) {
	var definitions []*linodev1.MonitorAlertDefinition

	err := c.executeWithRetry(ctx, "ListMonitorAlertDefinitions", func() error {
		var err error

		definitions, err = c.httpListMonitorAlertDefinitionsProto(ctx, page, pageSize)

		return err
	})

	return definitions, err
}

// ListMonitorAlertChannelsProto retrieves monitoring alert channels as proto
// messages with automatic retry on transient failures.
func (c *Client) ListMonitorAlertChannelsProto(ctx context.Context, page, pageSize int) ([]*linodev1.MonitorAlertChannel, error) {
	var channels []*linodev1.MonitorAlertChannel

	err := c.executeWithRetry(ctx, "ListMonitorAlertChannels", func() error {
		var err error

		channels, err = c.httpListMonitorAlertChannelsProto(ctx, page, pageSize)

		return err
	})

	return channels, err
}

// ListLongviewSubscriptions retrieves available Longview subscription plans with automatic retry on transient failures.
func (c *Client) ListLongviewSubscriptions(ctx context.Context, page, pageSize int) (*PaginatedResponse[LongviewSubscription], error) {
	var subscriptions *PaginatedResponse[LongviewSubscription]

	err := c.executeWithRetry(ctx, "ListLongviewSubscriptions", func() error {
		var err error

		subscriptions, err = c.httpListLongviewSubscriptions(ctx, page, pageSize)

		return err
	})

	return subscriptions, err
}

// ListLongviewSubscriptionsProto retrieves available Longview subscription plans
// as proto messages with automatic retry on transient failures.
func (c *Client) ListLongviewSubscriptionsProto(ctx context.Context, page, pageSize int) ([]*linodev1.LongviewSubscription, error) {
	var subscriptions []*linodev1.LongviewSubscription

	err := c.executeWithRetry(ctx, "ListLongviewSubscriptions", func() error {
		var retryErr error

		subscriptions, retryErr = c.httpListLongviewSubscriptionsProto(ctx, page, pageSize)

		return retryErr
	})

	return subscriptions, err
}

// GetLongviewSubscription retrieves one Longview subscription with automatic retry on transient failures.
func (c *Client) GetLongviewSubscription(ctx context.Context, subscriptionID string) (*LongviewSubscription, error) {
	var subscription *LongviewSubscription

	err := c.executeWithRetry(ctx, "GetLongviewSubscription", func() error {
		var err error

		subscription, err = c.httpGetLongviewSubscription(ctx, subscriptionID)

		return err
	})

	return subscription, err
}

// GetLongviewSubscriptionProto retrieves a Longview subscription as a proto message
// with automatic retry on transient failures.
func (c *Client) GetLongviewSubscriptionProto(ctx context.Context, subscriptionID string) (*linodev1.LongviewSubscription, error) {
	var subscription *linodev1.LongviewSubscription

	err := c.executeWithRetry(ctx, "GetLongviewSubscription", func() error {
		var err error

		subscription, err = c.httpGetLongviewSubscriptionProto(ctx, subscriptionID)

		return err
	})

	return subscription, err
}

// ListLongviewClients retrieves Longview clients with automatic retry on transient failures.
func (c *Client) ListLongviewClients(ctx context.Context, page, pageSize int) (*PaginatedResponse[LongviewClient], error) {
	var clients *PaginatedResponse[LongviewClient]

	err := c.executeWithRetry(ctx, "ListLongviewClients", func() error {
		var err error

		clients, err = c.httpListLongviewClients(ctx, page, pageSize)

		return err
	})

	return clients, err
}

// ListLongviewClientsProto retrieves Longview clients as proto messages with
// automatic retry on transient failures.
func (c *Client) ListLongviewClientsProto(ctx context.Context, page, pageSize int) ([]*linodev1.LongviewClient, error) {
	var clients []*linodev1.LongviewClient

	err := c.executeWithRetry(ctx, "ListLongviewClients", func() error {
		var err error

		clients, err = c.httpListLongviewClientsProto(ctx, page, pageSize)

		return err
	})

	return clients, err
}

// UpdateLongviewClient updates one Longview client without retrying the mutating request.
func (c *Client) UpdateLongviewClient(ctx context.Context, clientID int, req *UpdateLongviewClientRequest) (*LongviewClient, error) {
	return c.httpUpdateLongviewClient(ctx, clientID, req)
}

// DeleteLongviewClient deletes one Longview client without retrying the
// destructive request. Retrying can replay deletion after a transient error,
// so this method delegates exactly once.
func (c *Client) DeleteLongviewClient(ctx context.Context, clientID int) error {
	return c.httpDeleteLongviewClient(ctx, clientID)
}

// UpdateLongviewPlan updates the account Longview subscription plan without
// retrying the mutating request. Retrying can replay the plan change after a
// transient error, so this method delegates exactly once.
func (c *Client) UpdateLongviewPlan(ctx context.Context, req *UpdateLongviewPlanRequest) (*LongviewSubscription, error) {
	return c.httpUpdateLongviewPlan(ctx, req)
}

// ListAccountPaymentMethods retrieves account payment methods with automatic retry on transient failures.
func (c *Client) ListAccountPaymentMethods(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountPaymentMethod], error) {
	var methods *PaginatedResponse[AccountPaymentMethod]

	err := c.executeWithRetry(ctx, "ListAccountPaymentMethods", func() error {
		var err error

		methods, err = c.httpListAccountPaymentMethods(ctx, page, pageSize)

		return err
	})

	return methods, err
}

// GetAccountPaymentMethod retrieves one account payment method with automatic retry on transient failures.
func (c *Client) GetAccountPaymentMethod(ctx context.Context, paymentMethodID string) (*AccountPaymentMethod, error) {
	var method *AccountPaymentMethod

	err := c.executeWithRetry(ctx, "GetAccountPaymentMethod", func() error {
		var err error

		method, err = c.httpGetAccountPaymentMethod(ctx, paymentMethodID)

		return err
	})

	return method, err
}

// CreateAccountPaymentMethod adds a payment method without retrying the
// mutating request. Retrying can replay payment-method creation after a
// transient error, so this method delegates exactly once.
func (c *Client) CreateAccountPaymentMethod(ctx context.Context, req *CreateAccountPaymentMethodRequest) (*AccountPaymentMethod, error) {
	return c.httpCreateAccountPaymentMethod(ctx, req)
}

// DeleteAccountPaymentMethod deletes a payment method without retrying the
// mutating request. Retrying can replay deletion after a transient error,
// so this method delegates exactly once.
func (c *Client) DeleteAccountPaymentMethod(ctx context.Context, paymentMethodID string) error {
	return c.httpDeleteAccountPaymentMethod(ctx, paymentMethodID)
}

// MakeAccountPaymentMethodDefault changes the default payment method without
// retrying the mutating request. Retrying can replay the state change after a
// transient error, so this method delegates exactly once.
func (c *Client) MakeAccountPaymentMethodDefault(ctx context.Context, paymentMethodID string) error {
	return c.httpMakeAccountPaymentMethodDefault(ctx, paymentMethodID)
}

// GetAccountOAuthClient retrieves one OAuth client with automatic retry on transient failures.
func (c *Client) GetAccountOAuthClient(ctx context.Context, clientID string) (*OAuthClient, error) {
	var client *OAuthClient

	err := c.executeWithRetry(ctx, "GetAccountOAuthClient", func() error {
		var err error

		client, err = c.httpGetAccountOAuthClient(ctx, clientID)

		return err
	})

	return client, err
}

// GetAccountOAuthClientProto retrieves one OAuth client as a proto message with
// automatic retry on transient failures.
func (c *Client) GetAccountOAuthClientProto(ctx context.Context, clientID string) (*linodev1.OAuthClient, error) {
	var oauthClient *linodev1.OAuthClient

	err := c.executeWithRetry(ctx, "GetAccountOAuthClient", func() error {
		var err error

		oauthClient, err = c.httpGetAccountOAuthClientProto(ctx, clientID)

		return err
	})

	return oauthClient, err
}

// CreateOAuthClient creates an account OAuth client without retrying the
// mutating request. Retrying can replay client creation after a transient
// error, so this method delegates exactly once.
func (c *Client) CreateOAuthClient(ctx context.Context, req *CreateOAuthClientRequest) (*CreatedOAuthClient, error) {
	return c.httpCreateOAuthClient(ctx, req)
}

// CreateOAuthClientProto creates an account OAuth client and returns the proto
// CreatedOAuthClient element (carrying the one-time secret) without retrying the
// mutating request. Retrying can replay client creation after a transient error,
// so this method delegates exactly once.
func (c *Client) CreateOAuthClientProto(ctx context.Context, req *CreateOAuthClientRequest) (*linodev1.CreatedOAuthClient, error) {
	return c.httpCreateOAuthClientProto(ctx, req)
}

// CreateLongviewClient creates a Longview client without retrying the
// mutating request. Retrying can replay client creation after a transient
// error, so this method delegates exactly once.
func (c *Client) CreateLongviewClient(ctx context.Context, req *CreateLongviewClientRequest) (*CreatedLongviewClient, error) {
	return c.httpCreateLongviewClient(ctx, req)
}

// UpdateOAuthClient updates an account OAuth client without retrying the
// mutating request. Retrying can replay updates after a transient error,
// so this method delegates exactly once.
func (c *Client) UpdateOAuthClient(ctx context.Context, clientID string, req *UpdateOAuthClientRequest) (*OAuthClient, error) {
	return c.httpUpdateOAuthClient(ctx, clientID, req)
}

// UpdateOAuthClientThumbnail updates an account OAuth client's thumbnail without
// retrying the mutating request. Retrying can replay updates after a transient
// error, so this method delegates exactly once.
func (c *Client) UpdateOAuthClientThumbnail(ctx context.Context, clientID string, thumbnailPNG []byte) error {
	return c.httpUpdateOAuthClientThumbnail(ctx, clientID, thumbnailPNG)
}

// GetOAuthClientThumbnail retrieves an OAuth client's thumbnail with automatic retry on transient failures.
func (c *Client) GetOAuthClientThumbnail(ctx context.Context, clientID string) ([]byte, error) {
	var thumbnailPNG []byte

	err := c.executeWithRetry(ctx, "GetOAuthClientThumbnail", func() error {
		var err error

		thumbnailPNG, err = c.httpGetOAuthClientThumbnail(ctx, clientID)

		return err
	})

	return thumbnailPNG, err
}

// DeleteAccountOAuthClient deletes an account OAuth client without retrying the
// destructive request. Retrying can replay client deletion after a transient
// error, so this method delegates exactly once.
func (c *Client) DeleteAccountOAuthClient(ctx context.Context, clientID string) error {
	return c.httpDeleteAccountOAuthClient(ctx, clientID)
}

// ResetOAuthClientSecret resets an account OAuth client secret without retrying
// the credential rotation. Retrying can rotate the secret more than once after
// a transient error, so this method delegates exactly once.
func (c *Client) ResetOAuthClientSecret(ctx context.Context, clientID string) (*OAuthClientSecret, error) {
	return c.httpResetOAuthClientSecret(ctx, clientID)
}

// ResetOAuthClientSecretProto resets an account OAuth client secret and returns
// the proto OAuthClientSecret element (the new one-time secret) without retrying
// the credential rotation. Retrying can rotate the secret more than once after a
// transient error, so this method delegates exactly once.
func (c *Client) ResetOAuthClientSecretProto(ctx context.Context, clientID string) (*linodev1.OAuthClientSecret, error) {
	return c.httpResetOAuthClientSecretProto(ctx, clientID)
}

// ListAccountEvents retrieves account events with automatic retry on transient failures.
func (c *Client) ListAccountEvents(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountEvent], error) {
	var events *PaginatedResponse[AccountEvent]

	err := c.executeWithRetry(ctx, "ListAccountEvents", func() error {
		var err error

		events, err = c.httpListAccountEvents(ctx, page, pageSize)

		return err
	})

	return events, err
}

// ListAccountEventsProto retrieves account events as proto messages with
// automatic retry on transient failures.
func (c *Client) ListAccountEventsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountEvent, error) {
	var events []*linodev1.AccountEvent

	err := c.executeWithRetry(ctx, "ListAccountEvents", func() error {
		var retryErr error

		events, retryErr = c.httpListAccountEventsProto(ctx, page, pageSize)

		return retryErr
	})

	return events, err
}

// ListTaggedObjects retrieves objects with the supplied tag label with automatic retry on transient failures.
func (c *Client) ListTaggedObjects(ctx context.Context, tagLabel string, page, pageSize int) (*PaginatedResponse[TaggedObject], error) {
	var taggedObjects *PaginatedResponse[TaggedObject]

	err := c.executeWithRetry(ctx, "ListTaggedObjects", func() error {
		var err error

		taggedObjects, err = c.httpListTaggedObjects(ctx, tagLabel, page, pageSize)

		return err
	})

	return taggedObjects, err
}

// ListTaggedObjectsProto retrieves tagged objects as proto messages with
// automatic retry on transient failures.
func (c *Client) ListTaggedObjectsProto(ctx context.Context, tagLabel string, page, pageSize int) ([]*linodev1.TaggedObject, error) {
	var taggedObjects []*linodev1.TaggedObject

	err := c.executeWithRetry(ctx, "ListTaggedObjects", func() error {
		var retryErr error

		taggedObjects, retryErr = c.httpListTaggedObjectsProto(ctx, tagLabel, page, pageSize)

		return retryErr
	})

	return taggedObjects, err
}

// DeleteTag deletes a tag without retrying the destructive DELETE request.
func (c *Client) DeleteTag(ctx context.Context, tagLabel string) error {
	return c.httpDeleteTag(ctx, tagLabel)
}

// GetSupportTicket retrieves one support ticket with automatic retry on transient failures.
func (c *Client) GetSupportTicket(ctx context.Context, ticketID int) (SupportTicket, error) {
	var ticket SupportTicket

	err := c.executeWithRetry(ctx, "GetSupportTicket", func() error {
		var err error

		ticket, err = c.httpGetSupportTicket(ctx, ticketID)

		return err
	})

	return ticket, err
}

// GetSupportTicketProto retrieves one support ticket as a proto message with
// automatic retry on transient failures.
func (c *Client) GetSupportTicketProto(ctx context.Context, ticketID int) (*linodev1.SupportTicket, error) {
	var ticket *linodev1.SupportTicket

	err := c.executeWithRetry(ctx, "GetSupportTicket", func() error {
		var err error

		ticket, err = c.httpGetSupportTicketProto(ctx, ticketID)

		return err
	})

	return ticket, err
}

// ListSupportTickets retrieves support tickets with automatic retry on transient failures.
func (c *Client) ListSupportTickets(ctx context.Context, page, pageSize int) (*PaginatedResponse[SupportTicket], error) {
	var tickets *PaginatedResponse[SupportTicket]

	err := c.executeWithRetry(ctx, "ListSupportTickets", func() error {
		var err error

		tickets, err = c.httpListSupportTickets(ctx, page, pageSize)

		return err
	})

	return tickets, err
}

// ListSupportTicketsProto retrieves support tickets as proto messages with
// automatic retry on transient failures.
func (c *Client) ListSupportTicketsProto(ctx context.Context, page, pageSize int) ([]*linodev1.SupportTicket, error) {
	var tickets []*linodev1.SupportTicket

	err := c.executeWithRetry(ctx, "ListSupportTickets", func() error {
		var retryErr error

		tickets, retryErr = c.httpListSupportTicketsProto(ctx, page, pageSize)

		return retryErr
	})

	return tickets, err
}

// ListSupportTicketReplies retrieves replies for one support ticket with automatic retry on transient failures.
func (c *Client) ListSupportTicketReplies(ctx context.Context, ticketID, page, pageSize int) (*PaginatedResponse[SupportTicketReply], error) {
	var replies *PaginatedResponse[SupportTicketReply]

	err := c.executeWithRetry(ctx, "ListSupportTicketReplies", func() error {
		var err error

		replies, err = c.httpListSupportTicketReplies(ctx, ticketID, page, pageSize)

		return err
	})

	return replies, err
}

// ListSupportTicketRepliesProto retrieves a support ticket's replies as proto
// messages with automatic retry on transient failures.
func (c *Client) ListSupportTicketRepliesProto(ctx context.Context, ticketID, page, pageSize int) ([]*linodev1.SupportTicketReply, error) {
	var replies []*linodev1.SupportTicketReply

	err := c.executeWithRetry(ctx, "ListSupportTicketReplies", func() error {
		var retryErr error

		replies, retryErr = c.httpListSupportTicketRepliesProto(ctx, ticketID, page, pageSize)

		return retryErr
	})

	return replies, err
}

// ListAccountUsers retrieves account users with automatic retry on transient failures.
func (c *Client) ListAccountUsers(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountUser], error) {
	var users *PaginatedResponse[AccountUser]

	err := c.executeWithRetry(ctx, "ListAccountUsers", func() error {
		var err error

		users, err = c.httpListAccountUsers(ctx, page, pageSize)

		return err
	})

	return users, err
}

// ListAccountUsersProto retrieves account users as proto messages with automatic
// retry on transient failures.
func (c *Client) ListAccountUsersProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountUser, error) {
	var users []*linodev1.AccountUser

	err := c.executeWithRetry(ctx, "ListAccountUsers", func() error {
		var retryErr error

		users, retryErr = c.httpListAccountUsersProto(ctx, page, pageSize)

		return retryErr
	})

	return users, err
}

// GetAccountUser retrieves one account user with automatic retry on transient failures.
func (c *Client) GetAccountUser(ctx context.Context, username string) (*AccountUser, error) {
	var user *AccountUser

	err := c.executeWithRetry(ctx, "GetAccountUser", func() error {
		var err error

		user, err = c.httpGetAccountUser(ctx, username)

		return err
	})

	return user, err
}

// GetAccountUserProto retrieves one account user as a proto message with
// automatic retry on transient failures.
func (c *Client) GetAccountUserProto(ctx context.Context, username string) (*linodev1.AccountUser, error) {
	var user *linodev1.AccountUser

	err := c.executeWithRetry(ctx, "GetAccountUser", func() error {
		var err error

		user, err = c.httpGetAccountUserProto(ctx, username)

		return err
	})

	return user, err
}

// GetAccountUserGrants retrieves one account user's grants with automatic retry on transient failures.
func (c *Client) GetAccountUserGrants(ctx context.Context, username string) (*Grants, error) {
	var grants *Grants

	err := c.executeWithRetry(ctx, "GetAccountUserGrants", func() error {
		var err error

		grants, err = c.httpGetAccountUserGrants(ctx, username)

		return err
	})

	return grants, err
}

// UpdateAccountUserGrants updates account user grants without retrying the mutating request.
// Retrying can replay grant changes after a transient error, so this method
// delegates exactly once.
func (c *Client) UpdateAccountUserGrants(ctx context.Context, username string, request *UpdateAccountUserGrantsRequest) (*Grants, error) {
	return c.httpUpdateAccountUserGrants(ctx, username, request)
}

// UpdateAccountUserGrantsProto updates account user grants and returns the proto
// AccountUserGrants element without retrying the mutating request. Retrying can
// replay grant changes after a transient error, so this method delegates exactly
// once.
func (c *Client) UpdateAccountUserGrantsProto(ctx context.Context, username string, request *UpdateAccountUserGrantsRequest) (*linodev1.AccountUserGrants, error) {
	return c.httpUpdateAccountUserGrantsProto(ctx, username, request)
}

// UpdateAccountUser updates an account user without retrying the mutating request.
// Retrying can replay user updates after a transient error, so this method
// delegates exactly once.
func (c *Client) UpdateAccountUser(ctx context.Context, username string, request *UpdateAccountUserRequest) (*AccountUser, error) {
	return c.httpUpdateAccountUser(ctx, username, request)
}

// UpdateAccountUserProto updates an account user and returns the proto
// AccountUser element without retrying the mutating request. Retrying can replay
// user updates after a transient error, so this method delegates exactly once.
func (c *Client) UpdateAccountUserProto(ctx context.Context, username string, request *UpdateAccountUserRequest) (*linodev1.AccountUser, error) {
	return c.httpUpdateAccountUserProto(ctx, username, request)
}

// DeleteAccountUser deletes an account user without retrying the destructive request.
// Retrying can replay account user deletion after a transient error, so this method
// delegates exactly once.
func (c *Client) DeleteAccountUser(ctx context.Context, username string) error {
	return c.httpDeleteAccountUser(ctx, username)
}

// CreateAccountUser creates a user without retrying the mutating request.
// Retrying can create duplicate account users after a transient error, so this
// method delegates exactly once.
func (c *Client) CreateAccountUser(ctx context.Context, request *CreateAccountUserRequest) (*AccountUser, error) {
	return c.httpCreateAccountUser(ctx, request)
}

// CreateAccountUserProto creates a user and returns the proto AccountUser element
// without retrying the mutating request. Retrying can create duplicate account
// users after a transient error, so this method delegates exactly once.
func (c *Client) CreateAccountUserProto(ctx context.Context, request *CreateAccountUserRequest) (*linodev1.AccountUser, error) {
	return c.httpCreateAccountUserProto(ctx, request)
}

// CreateSupportTicket opens a support ticket without retrying the mutating request.
// Retrying can create duplicate support tickets after a transient error, so this
// method delegates exactly once.
func (c *Client) CreateSupportTicket(ctx context.Context, request *CreateSupportTicketRequest) (*SupportTicket, error) {
	return c.httpCreateSupportTicket(ctx, request)
}

// CreateSupportTicketProto opens a support ticket and returns the created ticket
// as a proto message without retrying the mutating request. Retrying can create
// duplicate tickets after a transient error, so this method delegates exactly once.
func (c *Client) CreateSupportTicketProto(ctx context.Context, request *CreateSupportTicketRequest) (*linodev1.SupportTicket, error) {
	return c.httpCreateSupportTicketProto(ctx, request)
}

// CreateSupportTicketAttachment creates a support ticket attachment without retrying the mutating request.
// Retrying can upload duplicate attachments after a transient error, so this
// method delegates exactly once.
func (c *Client) CreateSupportTicketAttachment(ctx context.Context, ticketID int, request *CreateSupportTicketAttachmentRequest) (*SupportTicketAttachment, error) {
	return c.httpCreateSupportTicketAttachment(ctx, ticketID, request)
}

// CreateSupportTicketReply creates a support ticket reply without retrying the mutating request.
// Retrying can post duplicate replies after a transient error, so this method
// delegates exactly once.
func (c *Client) CreateSupportTicketReply(ctx context.Context, ticketID int, request *CreateSupportTicketReplyRequest) (*SupportTicketReply, error) {
	return c.httpCreateSupportTicketReply(ctx, ticketID, request)
}

// CreateSupportTicketReplyProto creates a support ticket reply and returns it as
// a proto message without retrying the mutating request. Retrying can post
// duplicate replies after a transient error, so this method delegates exactly once.
func (c *Client) CreateSupportTicketReplyProto(ctx context.Context, ticketID int, request *CreateSupportTicketReplyRequest) (*linodev1.SupportTicketReply, error) {
	return c.httpCreateSupportTicketReplyProto(ctx, ticketID, request)
}

// CloseSupportTicket closes a support ticket without retrying the mutating request.
// Retrying can replay ticket state changes after a transient error, so this
// method delegates exactly once.
func (c *Client) CloseSupportTicket(ctx context.Context, ticketID int) error {
	return c.httpCloseSupportTicket(ctx, ticketID)
}

// CreateManagedContact creates a managed contact without retrying the mutating
// request. Managed contact creation is not guaranteed idempotent after a transient
// error, so this method delegates exactly once.
func (c *Client) CreateManagedContact(ctx context.Context, request *CreateManagedContactRequest) (*ManagedContact, error) {
	var contact *ManagedContact

	err := c.executeWithoutRetry(ctx, "CreateManagedContact", func() error {
		var retryErr error

		contact, retryErr = c.httpCreateManagedContact(ctx, request)

		return retryErr
	})

	return contact, err
}

// CreateManagedService creates a Managed service monitor without retrying the
// mutating request. Managed service creation is not guaranteed idempotent after a
// transient error, so this method delegates exactly once.
func (c *Client) CreateManagedService(ctx context.Context, request *CreateManagedServiceRequest) (*ManagedService, error) {
	var service *ManagedService

	err := c.executeWithoutRetry(ctx, "CreateManagedService", func() error {
		var retryErr error

		service, retryErr = c.httpCreateManagedService(ctx, request)

		return retryErr
	})

	return service, err
}

// ListAccountLogins retrieves account user logins with automatic retry on transient failures.
func (c *Client) ListAccountLogins(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountLogin], error) {
	var logins *PaginatedResponse[AccountLogin]

	err := c.executeWithRetry(ctx, "ListAccountLogins", func() error {
		var err error

		logins, err = c.httpListAccountLogins(ctx, page, pageSize)

		return err
	})

	return logins, err
}

// ListAccountLoginsProto retrieves account logins as proto messages with
// automatic retry on transient failures.
func (c *Client) ListAccountLoginsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountLogin, error) {
	var logins []*linodev1.AccountLogin

	err := c.executeWithRetry(ctx, "ListAccountLogins", func() error {
		var retryErr error

		logins, retryErr = c.httpListAccountLoginsProto(ctx, page, pageSize)

		return retryErr
	})

	return logins, err
}

// GetAccountLogin retrieves one account login with automatic retry on transient failures.
func (c *Client) GetAccountLogin(ctx context.Context, loginID int) (*AccountLogin, error) {
	var login *AccountLogin

	err := c.executeWithRetry(ctx, "GetAccountLogin", func() error {
		var err error

		login, err = c.httpGetAccountLogin(ctx, loginID)

		return err
	})

	return login, err
}

// GetAccountLoginProto retrieves one account login as a proto message with
// automatic retry on transient failures.
func (c *Client) GetAccountLoginProto(ctx context.Context, loginID int) (*linodev1.AccountLogin, error) {
	var login *linodev1.AccountLogin

	err := c.executeWithRetry(ctx, "GetAccountLogin", func() error {
		var err error

		login, err = c.httpGetAccountLoginProto(ctx, loginID)

		return err
	})

	return login, err
}

// ListAccountInvoices retrieves account invoices with automatic retry on transient failures.
func (c *Client) ListAccountInvoices(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountInvoice], error) {
	var invoices *PaginatedResponse[AccountInvoice]

	err := c.executeWithRetry(ctx, "ListAccountInvoices", func() error {
		var err error

		invoices, err = c.httpListAccountInvoices(ctx, page, pageSize)

		return err
	})

	return invoices, err
}

// ListAccountInvoicesProto retrieves account invoices as proto messages with
// automatic retry on transient failures.
func (c *Client) ListAccountInvoicesProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountInvoice, error) {
	var invoices []*linodev1.AccountInvoice

	err := c.executeWithRetry(ctx, "ListAccountInvoices", func() error {
		var retryErr error

		invoices, retryErr = c.httpListAccountInvoicesProto(ctx, page, pageSize)

		return retryErr
	})

	return invoices, err
}

// ListAccountPayments retrieves account payments with automatic retry on transient failures.
func (c *Client) ListAccountPayments(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountPayment], error) {
	var payments *PaginatedResponse[AccountPayment]

	err := c.executeWithRetry(ctx, "ListAccountPayments", func() error {
		var err error

		payments, err = c.httpListAccountPayments(ctx, page, pageSize)

		return err
	})

	return payments, err
}

// ListAccountPaymentsProto retrieves account payments as proto messages with
// automatic retry on transient failures.
func (c *Client) ListAccountPaymentsProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountPayment, error) {
	var payments []*linodev1.AccountPayment

	err := c.executeWithRetry(ctx, "ListAccountPayments", func() error {
		var retryErr error

		payments, retryErr = c.httpListAccountPaymentsProto(ctx, page, pageSize)

		return retryErr
	})

	return payments, err
}

// GetAccountPayment retrieves one account payment with automatic retry on transient failures.
func (c *Client) GetAccountPayment(ctx context.Context, paymentID int) (*AccountPayment, error) {
	var payment *AccountPayment

	err := c.executeWithRetry(ctx, "GetAccountPayment", func() error {
		var err error

		payment, err = c.httpGetAccountPayment(ctx, paymentID)

		return err
	})

	return payment, err
}

// GetAccountPaymentProto retrieves one account payment as a proto message with
// automatic retry on transient failures.
func (c *Client) GetAccountPaymentProto(ctx context.Context, paymentID int) (*linodev1.AccountPayment, error) {
	var payment *linodev1.AccountPayment

	err := c.executeWithRetry(ctx, "GetAccountPayment", func() error {
		var err error

		payment, err = c.httpGetAccountPaymentProto(ctx, paymentID)

		return err
	})

	return payment, err
}

// CreateAccountPayment makes an account payment without retrying the mutating
// request. Retrying can replay a payment after a transient error, so this
// method delegates exactly once.
func (c *Client) CreateAccountPayment(ctx context.Context, req *CreateAccountPaymentRequest) (*AccountPayment, error) {
	return c.httpCreateAccountPayment(ctx, req)
}

// AddAccountPromoCredit applies a promo credit without retrying the mutating
// request. Retrying can replay promo-credit application after a transient
// error, so this method delegates exactly once.
func (c *Client) AddAccountPromoCredit(ctx context.Context, req *AddAccountPromoCreditRequest) error {
	return c.httpAddAccountPromoCredit(ctx, req)
}

// GetAccountInvoice retrieves one account invoice with automatic retry on transient failures.
func (c *Client) GetAccountInvoice(ctx context.Context, invoiceID int) (*AccountInvoice, error) {
	var invoice *AccountInvoice

	err := c.executeWithRetry(ctx, "GetAccountInvoice", func() error {
		var err error

		invoice, err = c.httpGetAccountInvoice(ctx, invoiceID)

		return err
	})

	return invoice, err
}

// GetAccountInvoiceProto retrieves one account invoice as a proto message with
// automatic retry on transient failures.
func (c *Client) GetAccountInvoiceProto(ctx context.Context, invoiceID int) (*linodev1.AccountInvoice, error) {
	var invoice *linodev1.AccountInvoice

	err := c.executeWithRetry(ctx, "GetAccountInvoice", func() error {
		var err error

		invoice, err = c.httpGetAccountInvoiceProto(ctx, invoiceID)

		return err
	})

	return invoice, err
}

// ListAccountInvoiceItems retrieves items for one account invoice with automatic retry on transient failures.
func (c *Client) ListAccountInvoiceItems(ctx context.Context, invoiceID, page, pageSize int) (*PaginatedResponse[AccountInvoiceItem], error) {
	var items *PaginatedResponse[AccountInvoiceItem]

	err := c.executeWithRetry(ctx, "ListAccountInvoiceItems", func() error {
		var err error

		items, err = c.httpListAccountInvoiceItems(ctx, invoiceID, page, pageSize)

		return err
	})

	return items, err
}

// ListAccountChildAccounts retrieves child-level accounts with automatic retry on transient failures.
func (c *Client) ListAccountChildAccounts(ctx context.Context, page, pageSize int) (*PaginatedResponse[ChildAccount], error) {
	var childAccounts *PaginatedResponse[ChildAccount]

	err := c.executeWithRetry(ctx, "ListAccountChildAccounts", func() error {
		var err error

		childAccounts, err = c.httpListAccountChildAccounts(ctx, page, pageSize)

		return err
	})

	return childAccounts, err
}

// ListAccountServiceTransfers retrieves account service transfers with automatic retry on transient failures.
func (c *Client) ListAccountServiceTransfers(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountEntityTransfer], error) {
	var transfers *PaginatedResponse[AccountEntityTransfer]

	err := c.executeWithRetry(ctx, "ListAccountServiceTransfers", func() error {
		var err error

		transfers, err = c.httpListAccountServiceTransfers(ctx, page, pageSize)

		return err
	})

	return transfers, err
}

// ListAccountServiceTransfersProto retrieves account service transfers as proto
// messages with automatic retry on transient failures.
func (c *Client) ListAccountServiceTransfersProto(ctx context.Context, page, pageSize int) ([]*linodev1.AccountEntityTransfer, error) {
	var transfers []*linodev1.AccountEntityTransfer

	err := c.executeWithRetry(ctx, "ListAccountServiceTransfers", func() error {
		var retryErr error

		transfers, retryErr = c.httpListAccountServiceTransfersProto(ctx, page, pageSize)

		return retryErr
	})

	return transfers, err
}

// GetAccountServiceTransfer retrieves one account service transfer with automatic retry on transient failures.
func (c *Client) GetAccountServiceTransfer(ctx context.Context, token string) (*AccountEntityTransfer, error) {
	var transfer *AccountEntityTransfer

	err := c.executeWithRetry(ctx, "GetAccountServiceTransfer", func() error {
		var err error

		transfer, err = c.httpGetAccountServiceTransfer(ctx, token)

		return err
	})

	return transfer, err
}

// GetAccountServiceTransferProto retrieves one account service transfer as a
// proto message with automatic retry on transient failures.
func (c *Client) GetAccountServiceTransferProto(ctx context.Context, token string) (*linodev1.AccountEntityTransfer, error) {
	var transfer *linodev1.AccountEntityTransfer

	err := c.executeWithRetry(ctx, "GetAccountServiceTransfer", func() error {
		var err error

		transfer, err = c.httpGetAccountServiceTransferProto(ctx, token)

		return err
	})

	return transfer, err
}

// GetAccountEvent retrieves one account event with automatic retry on transient failures.
func (c *Client) GetAccountEvent(ctx context.Context, eventID int) (*AccountEvent, error) {
	var event *AccountEvent

	err := c.executeWithRetry(ctx, "GetAccountEvent", func() error {
		var err error

		event, err = c.httpGetAccountEvent(ctx, eventID)

		return err
	})

	return event, err
}

// GetAccountEventProto retrieves one account event as a proto message with
// automatic retry on transient failures.
func (c *Client) GetAccountEventProto(ctx context.Context, eventID int) (*linodev1.AccountEvent, error) {
	var event *linodev1.AccountEvent

	err := c.executeWithRetry(ctx, "GetAccountEvent", func() error {
		var err error

		event, err = c.httpGetAccountEventProto(ctx, eventID)

		return err
	})

	return event, err
}

// MarkAccountEventSeen marks one account event as seen without retrying the
// mutating request. Retrying can replay the state change after a transient
// error, so this method delegates exactly once.
func (c *Client) MarkAccountEventSeen(ctx context.Context, eventID int) error {
	return c.httpMarkAccountEventSeen(ctx, eventID)
}

// CreateAccountServiceTransfer creates an account service transfer without retrying
// the mutating request. Retrying can replay transfer creation after a transient
// error, so this method delegates exactly once.
func (c *Client) CreateAccountServiceTransfer(ctx context.Context, req *CreateAccountServiceTransferRequest) (*AccountEntityTransfer, error) {
	return c.httpCreateAccountServiceTransfer(ctx, req)
}

// DeleteAccountServiceTransfer cancels an account service transfer without retrying
// the mutating request. Retrying can replay transfer cancellation after a
// transient error, so this method delegates exactly once.
func (c *Client) DeleteAccountServiceTransfer(ctx context.Context, token string) error {
	return c.httpDeleteAccountServiceTransfer(ctx, token)
}

// AcceptAccountServiceTransfer accepts an account service transfer without retrying
// the mutating request. Retrying can replay transfer acceptance after a transient
// error, so this method delegates exactly once.
func (c *Client) AcceptAccountServiceTransfer(ctx context.Context, token string) error {
	return c.httpAcceptAccountServiceTransfer(ctx, token)
}

// GetAccountChildAccount retrieves one child-level account with automatic retry on transient failures.
func (c *Client) GetAccountChildAccount(ctx context.Context, euuid string) (*ChildAccount, error) {
	var childAccount *ChildAccount

	err := c.executeWithRetry(ctx, "GetAccountChildAccount", func() error {
		var err error

		childAccount, err = c.httpGetAccountChildAccount(ctx, euuid)

		return err
	})

	return childAccount, err
}

// CreateAccountChildAccountToken creates a proxy user token without retrying the
// mutating request. Retrying can create multiple short-lived tokens after a
// transient error, so this method delegates exactly once.
func (c *Client) CreateAccountChildAccountToken(ctx context.Context, euuid string) (*ProxyUserToken, error) {
	return c.httpCreateAccountChildAccountToken(ctx, euuid)
}

// GetAccountBeta retrieves one enrolled account beta program with automatic retry on transient failures.
func (c *Client) GetAccountBeta(ctx context.Context, betaID string) (*AccountBetaProgram, error) {
	var beta *AccountBetaProgram

	err := c.executeWithRetry(ctx, "GetAccountBeta", func() error {
		var err error

		beta, err = c.httpGetAccountBeta(ctx, betaID)

		return err
	})

	return beta, err
}

// GetAccountBetaProto retrieves one enrolled account beta program as a proto
// message with automatic retry on transient failures.
func (c *Client) GetAccountBetaProto(ctx context.Context, betaID string) (*linodev1.AccountBetaProgram, error) {
	var beta *linodev1.AccountBetaProgram

	err := c.executeWithRetry(ctx, "GetAccountBeta", func() error {
		var err error

		beta, err = c.httpGetAccountBetaProto(ctx, betaID)

		return err
	})

	return beta, err
}

// EnrollAccountBeta enrolls the account in a beta program without retrying the
// mutating request. Retrying can replay enrollment after a transient error, so
// this method delegates exactly once.
func (c *Client) EnrollAccountBeta(ctx context.Context, req *EnrollAccountBetaRequest) error {
	return c.httpEnrollAccountBeta(ctx, req)
}

// AcknowledgeAccountAgreements acknowledges account agreements without retrying
// the mutating request. Retrying can replay agreement acknowledgement after a
// transient error, so this method delegates exactly once.
func (c *Client) AcknowledgeAccountAgreements(ctx context.Context, req *AcknowledgeAccountAgreementsRequest) error {
	return c.httpAcknowledgeAccountAgreements(ctx, req)
}

// CancelAccount cancels the account without retrying the destructive request.
// Retrying can replay account cancellation after a transient error, so this
// method delegates exactly once.
func (c *Client) CancelAccount(ctx context.Context, req *CancelAccountRequest) (*CancelAccountResponse, error) {
	return c.httpCancelAccount(ctx, req)
}

// UpdateAccount updates account billing/contact fields without retrying the
// mutating request. Retrying can replay account state changes after a transient
// error, so this method delegates exactly once.
func (c *Client) UpdateAccount(ctx context.Context, req *UpdateAccountRequest) (*Account, error) {
	return c.httpUpdateAccount(ctx, req)
}

// UpdateAccountProto updates the account as a proto message without automatic retry.
func (c *Client) UpdateAccountProto(ctx context.Context, req *UpdateAccountRequest) (*linodev1.Account, error) {
	return c.httpUpdateAccountProto(ctx, req)
}

// ListNodeBalancerVPCs retrieves VPC configurations for a NodeBalancer with automatic retry on transient failures.
func (c *Client) ListNodeBalancerVPCs(ctx context.Context, nodeBalancerID, page, pageSize int) (*PaginatedResponse[NodeBalancerVPCConfig], error) {
	var vpcs *PaginatedResponse[NodeBalancerVPCConfig]

	err := c.executeWithRetry(ctx, "ListNodeBalancerVPCs", func() error {
		var err error

		vpcs, err = c.httpListNodeBalancerVPCs(ctx, nodeBalancerID, page, pageSize)

		return err
	})

	return vpcs, err
}

// ListNodeBalancerVPCsProto retrieves a NodeBalancer's VPC configurations as
// proto messages with automatic retry on transient failures.
func (c *Client) ListNodeBalancerVPCsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.NodeBalancerVPCConfig, error) {
	var vpcConfigs []*linodev1.NodeBalancerVPCConfig

	err := c.executeWithRetry(ctx, "ListNodeBalancerVPCs", func() error {
		var retryErr error

		vpcConfigs, retryErr = c.httpListNodeBalancerVPCsProto(ctx, nodeBalancerID, page, pageSize)

		return retryErr
	})

	return vpcConfigs, err
}

// ListRegions retrieves all regions with automatic retry on transient failures.
func (c *Client) ListRegions(ctx context.Context) ([]Region, error) {
	var regions []Region

	err := c.executeWithRetry(ctx, "ListRegions", func() error {
		var err error

		regions, err = c.httpListRegions(ctx)

		return err
	})

	return regions, err
}

// ListRegionsProto retrieves all regions as proto messages with automatic retry
// on transient failures.
func (c *Client) ListRegionsProto(ctx context.Context) ([]*linodev1.Region, error) {
	var regions []*linodev1.Region

	err := c.executeWithRetry(ctx, "ListRegions", func() error {
		var retryErr error

		regions, retryErr = c.httpListRegionsProto(ctx)

		return retryErr
	})

	return regions, err
}

// GetRegion retrieves a single region with automatic retry on transient failures.
func (c *Client) GetRegion(ctx context.Context, regionID string) (*Region, error) {
	var region *Region

	err := c.executeWithRetry(ctx, "GetRegion", func() error {
		var err error

		region, err = c.httpGetRegion(ctx, regionID)

		return err
	})

	return region, err
}

// GetRegionProto retrieves one region as a proto message with automatic retry on
// transient failures.
func (c *Client) GetRegionProto(ctx context.Context, regionID string) (*linodev1.Region, error) {
	var region *linodev1.Region

	err := c.executeWithRetry(ctx, "GetRegion", func() error {
		var err error

		region, err = c.httpGetRegionProto(ctx, regionID)

		return err
	})

	return region, err
}

// ListRegionsAvailability retrieves compute type availability across regions with automatic retry on transient failures.
func (c *Client) ListRegionsAvailability(ctx context.Context) ([]RegionAvailability, error) {
	var availability []RegionAvailability

	err := c.executeWithRetry(ctx, "ListRegionsAvailability", func() error {
		var err error

		availability, err = c.httpListRegionsAvailability(ctx)

		return err
	})

	return availability, err
}

// ListRegionsAvailabilityProto retrieves compute type availability across regions
// as proto RegionAvailability messages with automatic retry on transient
// failures.
func (c *Client) ListRegionsAvailabilityProto(ctx context.Context) ([]*linodev1.RegionAvailability, error) {
	var availability []*linodev1.RegionAvailability

	err := c.executeWithRetry(ctx, "ListRegionsAvailability", func() error {
		var err error

		availability, err = c.httpListRegionsAvailabilityProto(ctx)

		return err
	})

	return availability, err
}

// GetRegionAvailability retrieves compute type availability for one region with automatic retry on transient failures.
func (c *Client) GetRegionAvailability(ctx context.Context, regionID string) ([]RegionAvailability, error) {
	var availability []RegionAvailability

	err := c.executeWithRetry(ctx, "GetRegionAvailability", func() error {
		var err error

		availability, err = c.httpGetRegionAvailability(ctx, regionID)

		return err
	})

	return availability, err
}

// ListKernels retrieves all Linode kernels with automatic retry on transient failures.
func (c *Client) ListKernels(ctx context.Context, page, pageSize int) ([]Kernel, error) {
	var kernels []Kernel

	err := c.executeWithRetry(ctx, "ListKernels", func() error {
		var err error

		kernels, err = c.httpListKernels(ctx, page, pageSize)

		return err
	})

	return kernels, err
}

// ListKernelsProto retrieves all kernels as proto messages with automatic retry
// on transient failures.
func (c *Client) ListKernelsProto(ctx context.Context, page, pageSize int) ([]*linodev1.Kernel, error) {
	var kernels []*linodev1.Kernel

	err := c.executeWithRetry(ctx, "ListKernels", func() error {
		var retryErr error

		kernels, retryErr = c.httpListKernelsProto(ctx, page, pageSize)

		return retryErr
	})

	return kernels, err
}

// GetKernel retrieves a single Linode kernel with automatic retry on transient failures.
func (c *Client) GetKernel(ctx context.Context, kernelID string) (*Kernel, error) {
	var kernel *Kernel

	err := c.executeWithRetry(ctx, "GetKernel", func() error {
		var err error

		kernel, err = c.httpGetKernel(ctx, kernelID)

		return err
	})

	return kernel, err
}

// GetKernelProto retrieves one kernel as a proto message with automatic retry on
// transient failures.
func (c *Client) GetKernelProto(ctx context.Context, kernelID string) (*linodev1.Kernel, error) {
	var kernel *linodev1.Kernel

	err := c.executeWithRetry(ctx, "GetKernel", func() error {
		var err error

		kernel, err = c.httpGetKernelProto(ctx, kernelID)

		return err
	})

	return kernel, err
}

// ListTypes retrieves all Linode types with automatic retry on transient failures.
func (c *Client) ListTypes(ctx context.Context) ([]InstanceType, error) {
	var types []InstanceType

	err := c.executeWithRetry(ctx, "ListTypes", func() error {
		var err error

		types, err = c.httpListTypes(ctx)

		return err
	})

	return types, err
}

// ListTypesProto retrieves all Linode instance types as proto messages with
// automatic retry on transient failures.
func (c *Client) ListTypesProto(ctx context.Context) ([]*linodev1.InstanceType, error) {
	var types []*linodev1.InstanceType

	err := c.executeWithRetry(ctx, "ListTypes", func() error {
		var retryErr error

		types, retryErr = c.httpListTypesProto(ctx)

		return retryErr
	})

	return types, err
}

// GetType retrieves one Linode type with automatic retry on transient failures.
func (c *Client) GetType(ctx context.Context, typeID string) (*InstanceType, error) {
	var instanceType *InstanceType

	err := c.executeWithRetry(ctx, "GetType", func() error {
		var err error

		instanceType, err = c.httpGetType(ctx, typeID)

		return err
	})

	return instanceType, err
}

// GetTypeProto retrieves one instance type as a proto message with automatic
// retry on transient failures.
func (c *Client) GetTypeProto(ctx context.Context, typeID string) (*linodev1.InstanceType, error) {
	var instanceType *linodev1.InstanceType

	err := c.executeWithRetry(ctx, "GetType", func() error {
		var err error

		instanceType, err = c.httpGetTypeProto(ctx, typeID)

		return err
	})

	return instanceType, err
}

// ReplicateImage replicates an image without retrying the mutating request.
// Retrying can replay image replication after a transient error, so this method
// delegates exactly once.
func (c *Client) ReplicateImage(ctx context.Context, imageID string, req *ReplicateImageRequest) (*Image, error) {
	return c.httpReplicateImage(ctx, imageID, req)
}

// ReplicateImageProto replicates an image as a proto message without automatic
// retry. Replaying this state-changing operation could repeat side effects.
func (c *Client) ReplicateImageProto(ctx context.Context, imageID string, req *ReplicateImageRequest) (*linodev1.Image, error) {
	return c.httpReplicateImageProto(ctx, imageID, req)
}

// CreateImageShareGroupProto creates an image share group as a proto message
// without automatic retry.
func (c *Client) CreateImageShareGroupProto(ctx context.Context, req *CreateImageShareGroupRequest) (*linodev1.ImageShareGroup, error) {
	return c.httpCreateImageShareGroupProto(ctx, req)
}

// UpdateImageShareGroupProto updates an image share group as a proto message
// without automatic retry.
func (c *Client) UpdateImageShareGroupProto(ctx context.Context, shareGroupID int, req *UpdateImageShareGroupRequest) (*linodev1.ImageShareGroup, error) {
	return c.httpUpdateImageShareGroupProto(ctx, shareGroupID, req)
}

// AddImageShareGroupImagesProto adds images to a share group and returns the
// image as a proto message without automatic retry.
func (c *Client) AddImageShareGroupImagesProto(ctx context.Context, shareGroupID int, req *AddImageShareGroupImagesRequest) (*linodev1.Image, error) {
	return c.httpAddImageShareGroupImagesProto(ctx, shareGroupID, req)
}

// AddImageShareGroupMembersProto adds members to a share group and returns the
// parent share group as a proto message without automatic retry.
func (c *Client) AddImageShareGroupMembersProto(ctx context.Context, shareGroupID int, req *AddImageShareGroupMembersRequest) (*linodev1.ImageShareGroup, error) {
	return c.httpAddImageShareGroupMembersProto(ctx, shareGroupID, req)
}

// UpdateImageShareGroupImageProto updates a shared image as a proto message
// without automatic retry.
func (c *Client) UpdateImageShareGroupImageProto(ctx context.Context, shareGroupID int, imageID string, req *UpdateImageShareGroupImageRequest) (*linodev1.Image, error) {
	return c.httpUpdateImageShareGroupImageProto(ctx, shareGroupID, imageID, req)
}

// CreateImageShareGroupTokenProto creates a membership token as a proto message
// without automatic retry.
func (c *Client) CreateImageShareGroupTokenProto(ctx context.Context, req *CreateImageShareGroupTokenRequest) (*linodev1.ImageShareGroupToken, error) {
	return c.httpCreateImageShareGroupTokenProto(ctx, req)
}

// UpdateImageShareGroupTokenProto updates a membership token label as a proto
// message without automatic retry.
func (c *Client) UpdateImageShareGroupTokenProto(ctx context.Context, tokenUUID string, req *UpdateImageShareGroupTokenRequest) (*linodev1.ImageShareGroupToken, error) {
	return c.httpUpdateImageShareGroupTokenProto(ctx, tokenUUID, req)
}

// UpdateImageShareGroupMemberProto updates a member token label as a proto
// message without automatic retry.
func (c *Client) UpdateImageShareGroupMemberProto(ctx context.Context, shareGroupID int, tokenUUID string, req *UpdateImageShareGroupMemberRequest) (*linodev1.ImageShareGroupMember, error) {
	return c.httpUpdateImageShareGroupMemberProto(ctx, shareGroupID, tokenUUID, req)
}

// UploadImageProto creates an image upload target and returns the one-time
// upload URL plus the created image as a proto message without automatic retry.
func (c *Client) UploadImageProto(ctx context.Context, req *UploadImageRequest) (*linodev1.Image, string, error) {
	return c.httpUploadImageProto(ctx, req)
}

// ListDatabaseEngines retrieves Managed Database engines with automatic retry on transient failures.
func (c *Client) ListDatabaseEngines(ctx context.Context, page, pageSize int) ([]DatabaseEngine, error) {
	var engines []DatabaseEngine

	err := c.executeWithRetry(ctx, "ListDatabaseEngines", func() error {
		var err error

		engines, err = c.httpListDatabaseEngines(ctx, page, pageSize)

		return err
	})

	return engines, err
}

// ListDatabaseEnginesProto retrieves Managed Database engines as proto messages
// with automatic retry on transient failures.
func (c *Client) ListDatabaseEnginesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseEngine, error) {
	var engines []*linodev1.DatabaseEngine

	err := c.executeWithRetry(ctx, "ListDatabaseEngines", func() error {
		var retryErr error

		engines, retryErr = c.httpListDatabaseEnginesProto(ctx, page, pageSize)

		return retryErr
	})

	return engines, err
}

// ListAllDatabaseInstancesProto retrieves cross-engine Managed Database
// instances as proto messages with automatic retry on transient failures.
func (c *Client) ListAllDatabaseInstancesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseInstance, error) {
	var instances []*linodev1.DatabaseInstance

	err := c.executeWithRetry(ctx, "ListAllDatabaseInstances", func() error {
		var retryErr error

		instances, retryErr = c.httpListAllDatabaseInstancesProto(ctx, page, pageSize)

		return retryErr
	})

	return instances, err
}

// ListDatabaseInstancesProto retrieves MySQL Managed Database instances as proto
// messages with automatic retry on transient failures.
func (c *Client) ListDatabaseInstancesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseInstance, error) {
	var instances []*linodev1.DatabaseInstance

	err := c.executeWithRetry(ctx, "ListDatabaseInstances", func() error {
		var retryErr error

		instances, retryErr = c.httpListDatabaseInstancesProto(ctx, page, pageSize)

		return retryErr
	})

	return instances, err
}

// ListDatabasePostgreSQLInstancesProto retrieves PostgreSQL Managed Database
// instances as proto messages with automatic retry on transient failures.
func (c *Client) ListDatabasePostgreSQLInstancesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseInstance, error) {
	var instances []*linodev1.DatabaseInstance

	err := c.executeWithRetry(ctx, "ListDatabasePostgreSQLInstances", func() error {
		var retryErr error

		instances, retryErr = c.httpListDatabasePostgreSQLInstancesProto(ctx, page, pageSize)

		return retryErr
	})

	return instances, err
}

// ListDatabaseTypes retrieves Managed Database node types with automatic retry on transient failures.
func (c *Client) ListDatabaseTypes(ctx context.Context, page, pageSize int) ([]DatabaseType, error) {
	var types []DatabaseType

	err := c.executeWithRetry(ctx, "ListDatabaseTypes", func() error {
		var err error

		types, err = c.httpListDatabaseTypes(ctx, page, pageSize)

		return err
	})

	return types, err
}

// ListDatabaseTypesProto retrieves Managed Database node types as proto messages
// with automatic retry on transient failures.
func (c *Client) ListDatabaseTypesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseType, error) {
	var types []*linodev1.DatabaseType

	err := c.executeWithRetry(ctx, "ListDatabaseTypes", func() error {
		var retryErr error

		types, retryErr = c.httpListDatabaseTypesProto(ctx, page, pageSize)

		return retryErr
	})

	return types, err
}

// GetDatabaseType retrieves a Managed Database node type with automatic retry on transient failures.
func (c *Client) GetDatabaseType(ctx context.Context, typeID string, page, pageSize int) (*DatabaseType, error) {
	var databaseType *DatabaseType

	err := c.executeWithRetry(ctx, "GetDatabaseType", func() error {
		var err error

		databaseType, err = c.httpGetDatabaseType(ctx, typeID, page, pageSize)

		return err
	})

	return databaseType, err
}

// GetDatabaseTypeProto retrieves one Managed Database type as a proto message with
// automatic retry on transient failures.
func (c *Client) GetDatabaseTypeProto(ctx context.Context, typeID string, page, pageSize int) (*linodev1.DatabaseType, error) {
	var databaseType *linodev1.DatabaseType

	err := c.executeWithRetry(ctx, "GetDatabaseType", func() error {
		var err error

		databaseType, err = c.httpGetDatabaseTypeProto(ctx, typeID, page, pageSize)

		return err
	})

	return databaseType, err
}

// GetDatabaseMySQLConfig retrieves MySQL Managed Database advanced parameters with automatic retry on transient failures.
func (c *Client) GetDatabaseMySQLConfig(ctx context.Context) (map[string]any, error) {
	var config map[string]any

	err := c.executeWithRetry(ctx, "GetDatabaseMySQLConfig", func() error {
		var err error

		config, err = c.httpGetDatabaseMySQLConfig(ctx)

		return err
	})

	return config, err
}

// GetDatabasePostgreSQLConfig retrieves PostgreSQL Managed Database advanced parameters with automatic retry on transient failures.
func (c *Client) GetDatabasePostgreSQLConfig(ctx context.Context) (map[string]any, error) {
	var config map[string]any

	err := c.executeWithRetry(ctx, "GetDatabasePostgreSQLConfig", func() error {
		var err error

		config, err = c.httpGetDatabasePostgreSQLConfig(ctx)

		return err
	})

	return config, err
}

// ListAllDatabaseInstances retrieves Managed Database instances across every
// engine with automatic retry on transient failures.
func (c *Client) ListAllDatabaseInstances(ctx context.Context, page, pageSize int) ([]DatabaseInstance, error) {
	var instances []DatabaseInstance

	err := c.executeWithRetry(ctx, "ListAllDatabaseInstances", func() error {
		var err error

		instances, err = c.httpListAllDatabaseInstances(ctx, page, pageSize)

		return err
	})

	return instances, err
}

// ListDatabaseInstances retrieves Managed Database instances with automatic retry on transient failures.
func (c *Client) ListDatabaseInstances(ctx context.Context, page, pageSize int) ([]DatabaseInstance, error) {
	var instances []DatabaseInstance

	err := c.executeWithRetry(ctx, "ListDatabaseInstances", func() error {
		var err error

		instances, err = c.httpListDatabaseInstances(ctx, page, pageSize)

		return err
	})

	return instances, err
}

// ListDatabasePostgreSQLInstances retrieves PostgreSQL Managed Database instances with automatic retry on transient failures.
func (c *Client) ListDatabasePostgreSQLInstances(ctx context.Context, page, pageSize int) ([]DatabaseInstance, error) {
	var instances []DatabaseInstance

	err := c.executeWithRetry(ctx, "ListDatabasePostgreSQLInstances", func() error {
		var err error

		instances, err = c.httpListDatabasePostgreSQLInstances(ctx, page, pageSize)

		return err
	})

	return instances, err
}

// GetDatabaseInstance retrieves one MySQL Managed Database instance with automatic retry on transient failures.
func (c *Client) GetDatabaseInstance(ctx context.Context, instanceID int) (*DatabaseInstance, error) {
	var instance *DatabaseInstance

	err := c.executeWithRetry(ctx, "GetDatabaseInstance", func() error {
		var err error

		instance, err = c.httpGetDatabaseInstance(ctx, instanceID)

		return err
	})

	return instance, err
}

// GetDatabasePostgreSQLInstance retrieves one PostgreSQL Managed Database instance with automatic retry on transient failures.
func (c *Client) GetDatabasePostgreSQLInstance(ctx context.Context, instanceID int) (*DatabaseInstance, error) {
	var instance *DatabaseInstance

	err := c.executeWithRetry(ctx, "GetDatabasePostgreSQLInstance", func() error {
		var err error

		instance, err = c.httpGetDatabasePostgreSQLInstance(ctx, instanceID)

		return err
	})

	return instance, err
}

// GetDatabaseInstanceSSL retrieves the SSL CA certificate for a MySQL Managed Database instance with automatic retry on transient failures.
func (c *Client) GetDatabaseInstanceSSL(ctx context.Context, instanceID int) (*DatabaseSSL, error) {
	var ssl *DatabaseSSL

	err := c.executeWithRetry(ctx, "GetDatabaseInstanceSSL", func() error {
		var err error

		ssl, err = c.httpGetDatabaseInstanceSSL(ctx, instanceID)

		return err
	})

	return ssl, err
}

// GetDatabaseInstanceSSLProto retrieves a MySQL database SSL certificate as a proto
// message with automatic retry on transient failures.
func (c *Client) GetDatabaseInstanceSSLProto(ctx context.Context, instanceID int) (*linodev1.DatabaseSSL, error) {
	var ssl *linodev1.DatabaseSSL

	err := c.executeWithRetry(ctx, "GetDatabaseInstanceSSL", func() error {
		var err error

		ssl, err = c.httpGetDatabaseInstanceSSLProto(ctx, instanceID)

		return err
	})

	return ssl, err
}

// GetDatabasePostgreSQLInstanceSSL retrieves the SSL CA certificate for a PostgreSQL Managed Database instance with automatic retry on transient failures.
func (c *Client) GetDatabasePostgreSQLInstanceSSL(ctx context.Context, instanceID int) (*DatabaseSSL, error) {
	var ssl *DatabaseSSL

	err := c.executeWithRetry(ctx, "GetDatabasePostgreSQLInstanceSSL", func() error {
		var err error

		ssl, err = c.httpGetDatabasePostgreSQLInstanceSSL(ctx, instanceID)

		return err
	})

	return ssl, err
}

// GetDatabasePostgreSQLInstanceSSLProto retrieves a PostgreSQL database SSL
// certificate as a proto message with automatic retry on transient failures.
func (c *Client) GetDatabasePostgreSQLInstanceSSLProto(ctx context.Context, instanceID int) (*linodev1.DatabaseSSL, error) {
	var ssl *linodev1.DatabaseSSL

	err := c.executeWithRetry(ctx, "GetDatabasePostgreSQLInstanceSSL", func() error {
		var err error

		ssl, err = c.httpGetDatabasePostgreSQLInstanceSSLProto(ctx, instanceID)

		return err
	})

	return ssl, err
}

// GetDatabaseInstanceCredentials retrieves MySQL Managed Database credentials with automatic retry on transient failures.
func (c *Client) GetDatabaseInstanceCredentials(ctx context.Context, instanceID int) (*DatabaseCredentials, error) {
	var credentials *DatabaseCredentials

	err := c.executeWithRetry(ctx, "GetDatabaseInstanceCredentials", func() error {
		var err error

		credentials, err = c.httpGetDatabaseInstanceCredentials(ctx, instanceID)

		return err
	})

	return credentials, err
}

// GetDatabasePostgreSQLInstanceCredentials retrieves PostgreSQL Managed Database credentials with automatic retry on transient failures.
func (c *Client) GetDatabasePostgreSQLInstanceCredentials(ctx context.Context, instanceID int) (*DatabaseCredentials, error) {
	var credentials *DatabaseCredentials

	err := c.executeWithRetry(ctx, "GetDatabasePostgreSQLInstanceCredentials", func() error {
		var err error

		credentials, err = c.httpGetDatabasePostgreSQLInstanceCredentials(ctx, instanceID)

		return err
	})

	return credentials, err
}

// GetDatabaseEngine retrieves a Managed Database engine with automatic retry on transient failures.
func (c *Client) GetDatabaseEngine(ctx context.Context, engineID string) (*DatabaseEngine, error) {
	var engine *DatabaseEngine

	err := c.executeWithRetry(ctx, "GetDatabaseEngine", func() error {
		var err error

		engine, err = c.httpGetDatabaseEngine(ctx, engineID)

		return err
	})

	return engine, err
}

// GetDatabaseEngineProto retrieves one Managed Database engine as a proto message
// with automatic retry on transient failures.
func (c *Client) GetDatabaseEngineProto(ctx context.Context, engineID string) (*linodev1.DatabaseEngine, error) {
	var engine *linodev1.DatabaseEngine

	err := c.executeWithRetry(ctx, "GetDatabaseEngine", func() error {
		var err error

		engine, err = c.httpGetDatabaseEngineProto(ctx, engineID)

		return err
	})

	return engine, err
}

// ResetDatabaseInstanceCredentials resets MySQL Managed Database credentials without retrying the POST.
func (c *Client) ResetDatabaseInstanceCredentials(ctx context.Context, instanceID int) (*DatabaseCredentials, error) {
	return c.httpResetDatabaseInstanceCredentials(ctx, instanceID)
}

// ResetDatabasePostgreSQLInstanceCredentials resets PostgreSQL Managed Database credentials without retrying the POST.
func (c *Client) ResetDatabasePostgreSQLInstanceCredentials(ctx context.Context, instanceID int) error {
	return c.httpResetDatabasePostgreSQLInstanceCredentials(ctx, instanceID)
}

// CreateDatabaseInstance creates or restores a MySQL Managed Database instance without retrying the POST.
func (c *Client) CreateDatabaseInstance(ctx context.Context, req *CreateDatabaseInstanceRequest) (*DatabaseInstance, error) {
	return c.httpCreateDatabaseInstance(ctx, req)
}

// CreateDatabasePostgreSQLInstance creates or restores a PostgreSQL Managed Database instance without retrying the POST.
func (c *Client) CreateDatabasePostgreSQLInstance(ctx context.Context, req *CreateDatabaseInstanceRequest) (*DatabaseInstance, error) {
	return c.httpCreateDatabasePostgreSQLInstance(ctx, req)
}

// UpdateDatabaseInstance updates one MySQL Managed Database instance without retrying the PUT.
func (c *Client) UpdateDatabaseInstance(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*DatabaseInstance, error) {
	return c.httpUpdateDatabaseInstance(ctx, instanceID, req)
}

// UpdateDatabasePostgreSQLInstance updates one PostgreSQL Managed Database instance without retrying the PUT.
func (c *Client) UpdateDatabasePostgreSQLInstance(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*DatabaseInstance, error) {
	return c.httpUpdateDatabasePostgreSQLInstance(ctx, instanceID, req)
}

// CreateDatabaseInstanceProto creates a MySQL Managed Database instance and
// returns the proto element without retrying the POST.
func (c *Client) CreateDatabaseInstanceProto(ctx context.Context, req *CreateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	return c.httpCreateDatabaseInstanceProto(ctx, req)
}

// CreateDatabasePostgreSQLInstanceProto creates a PostgreSQL Managed Database
// instance and returns the proto element without retrying the POST.
func (c *Client) CreateDatabasePostgreSQLInstanceProto(ctx context.Context, req *CreateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	return c.httpCreateDatabasePostgreSQLInstanceProto(ctx, req)
}

// UpdateDatabaseInstanceProto updates one MySQL Managed Database instance and
// returns the proto element without retrying the PUT.
func (c *Client) UpdateDatabaseInstanceProto(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	return c.httpUpdateDatabaseInstanceProto(ctx, instanceID, req)
}

// UpdateDatabasePostgreSQLInstanceProto updates one PostgreSQL Managed Database
// instance and returns the proto element without retrying the PUT.
func (c *Client) UpdateDatabasePostgreSQLInstanceProto(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	return c.httpUpdateDatabasePostgreSQLInstanceProto(ctx, instanceID, req)
}

// DeleteDatabaseInstance deletes one MySQL Managed Database instance without retrying the DELETE.
func (c *Client) DeleteDatabaseInstance(ctx context.Context, instanceID int) error {
	return c.httpDeleteDatabaseInstance(ctx, instanceID)
}

// DeleteDatabasePostgreSQLInstance deletes one PostgreSQL Managed Database instance without retrying the DELETE.
func (c *Client) DeleteDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	return c.httpDeleteDatabasePostgreSQLInstance(ctx, instanceID)
}

// PatchDatabaseInstance applies security patches and updates to one MySQL Managed Database instance without retrying the POST.
func (c *Client) PatchDatabaseInstance(ctx context.Context, instanceID int) error {
	return c.httpPatchDatabaseInstance(ctx, instanceID)
}

// PatchDatabasePostgreSQLInstance applies security patches and updates to one PostgreSQL Managed Database instance without retrying the POST.
func (c *Client) PatchDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	return c.httpPatchDatabasePostgreSQLInstance(ctx, instanceID)
}

// SuspendDatabaseInstance suspends one active MySQL Managed Database instance without retrying the POST.
func (c *Client) SuspendDatabaseInstance(ctx context.Context, instanceID int) error {
	return c.httpSuspendDatabaseInstance(ctx, instanceID)
}

// SuspendDatabasePostgreSQLInstance suspends one active PostgreSQL Managed Database instance without retrying the POST.
func (c *Client) SuspendDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	return c.httpSuspendDatabasePostgreSQLInstance(ctx, instanceID)
}

// ResumeDatabaseInstance resumes one suspended MySQL Managed Database instance without retrying the POST.
func (c *Client) ResumeDatabaseInstance(ctx context.Context, instanceID int) error {
	return c.httpResumeDatabaseInstance(ctx, instanceID)
}

// ResumeDatabasePostgreSQLInstance resumes one suspended PostgreSQL Managed Database instance without retrying the POST.
func (c *Client) ResumeDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	return c.httpResumeDatabasePostgreSQLInstance(ctx, instanceID)
}

// ListVolumes retrieves all volumes with automatic retry on transient failures.
func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	var volumes []Volume

	err := c.executeWithRetry(ctx, "ListVolumes", func() error {
		var err error

		volumes, err = c.httpListVolumes(ctx)

		return err
	})

	return volumes, err
}

// ListVolumesProto retrieves all volumes as proto messages with automatic
// retry on transient failures.
func (c *Client) ListVolumesProto(ctx context.Context) ([]*linodev1.Volume, error) {
	var volumes []*linodev1.Volume

	err := c.executeWithRetry(ctx, "ListVolumes", func() error {
		var err error

		volumes, err = c.httpListVolumesProto(ctx)

		return err
	})

	return volumes, err
}

// ListImages retrieves all images with automatic retry on transient failures.
func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	var images []Image

	err := c.executeWithRetry(ctx, "ListImages", func() error {
		var err error

		images, err = c.httpListImages(ctx)

		return err
	})

	return images, err
}

// ListImagesProto retrieves images as proto messages with automatic retry on
// transient failures.
func (c *Client) ListImagesProto(ctx context.Context) ([]*linodev1.Image, error) {
	var images []*linodev1.Image

	err := c.executeWithRetry(ctx, "ListImages", func() error {
		var err error

		images, err = c.httpListImagesProto(ctx)

		return err
	})

	return images, err
}

// GetImage retrieves one image with automatic retry on transient failures.
func (c *Client) GetImage(ctx context.Context, imageID string) (*Image, error) {
	var image *Image

	err := c.executeWithRetry(ctx, "GetImage", func() error {
		var err error

		image, err = c.httpGetImage(ctx, imageID)

		return err
	})

	return image, err
}

// GetImageProto retrieves an image as a proto message with automatic retry on
// transient failures.
func (c *Client) GetImageProto(ctx context.Context, imageID string) (*linodev1.Image, error) {
	var image *linodev1.Image

	err := c.executeWithRetry(ctx, "GetImage", func() error {
		var err error

		image, err = c.httpGetImageProto(ctx, imageID)

		return err
	})

	return image, err
}

// DeleteImage deletes a private image without automatic retry.
// Replaying this destructive operation could repeat side effects after a transient failure.
func (c *Client) DeleteImage(ctx context.Context, imageID string) error {
	return c.httpDeleteImage(ctx, imageID)
}

// UpdateImage updates editable fields for a Linode image without automatic retry.
// Replaying this mutating operation could repeat side effects after a transient failure.
func (c *Client) UpdateImage(ctx context.Context, imageID string, req *UpdateImageRequest) (*Image, error) {
	return c.httpUpdateImage(ctx, imageID, req)
}

// UpdateImageProto updates an image as a proto message without automatic retry.
func (c *Client) UpdateImageProto(ctx context.Context, imageID string, req *UpdateImageRequest) (*linodev1.Image, error) {
	return c.httpUpdateImageProto(ctx, imageID, req)
}

// UpdatePlacementGroup updates a placement group without retrying the mutating request.
// Replaying this operation could repeat side effects after a transient failure.
func (c *Client) UpdatePlacementGroup(ctx context.Context, groupID int, request *UpdatePlacementGroupRequest) (*PlacementGroup, error) {
	return c.httpUpdatePlacementGroup(ctx, groupID, request)
}

// UpdatePlacementGroupProto updates a placement group as a proto message without
// automatic retry.
func (c *Client) UpdatePlacementGroupProto(ctx context.Context, groupID int, request *UpdatePlacementGroupRequest) (*linodev1.PlacementGroup, error) {
	return c.httpUpdatePlacementGroupProto(ctx, groupID, request)
}

// AssignPlacementGroupLinodes assigns Linodes to a placement group without automatic retry.
// Replaying this state-changing operation could repeat side effects after a transient failure.
func (c *Client) AssignPlacementGroupLinodes(ctx context.Context, groupID int, req *AssignPlacementGroupLinodesRequest) (*PlacementGroup, error) {
	return c.httpAssignPlacementGroupLinodes(ctx, groupID, req)
}

// AssignPlacementGroupLinodesProto assigns Linodes to a placement group as a proto
// message without automatic retry.
func (c *Client) AssignPlacementGroupLinodesProto(ctx context.Context, groupID int, req *AssignPlacementGroupLinodesRequest) (*linodev1.PlacementGroup, error) {
	return c.httpAssignPlacementGroupLinodesProto(ctx, groupID, req)
}

// ListPlacementGroupsProto retrieves placement groups as proto messages with
// automatic retry on transient failures.
func (c *Client) ListPlacementGroupsProto(ctx context.Context, page, pageSize int) ([]*linodev1.PlacementGroup, error) {
	var placementGroups []*linodev1.PlacementGroup

	err := c.executeWithRetry(ctx, "ListPlacementGroups", func() error {
		var retryErr error

		placementGroups, retryErr = c.httpListPlacementGroupsProto(ctx, page, pageSize)

		return retryErr
	})

	return placementGroups, err
}

// ListPlacementGroups retrieves placement groups with automatic retry on transient failures.
func (c *Client) ListPlacementGroups(ctx context.Context, page, pageSize int) (*PaginatedResponse[PlacementGroup], error) {
	var placementGroups *PaginatedResponse[PlacementGroup]

	err := c.executeWithRetry(ctx, "ListPlacementGroups", func() error {
		var err error

		placementGroups, err = c.httpListPlacementGroups(ctx, page, pageSize)

		return err
	})

	return placementGroups, err
}

// CreatePlacementGroup creates a placement group without automatic retry.
// Replaying this create operation could repeat side effects after a transient failure.
func (c *Client) CreatePlacementGroup(ctx context.Context, req *CreatePlacementGroupRequest) (*PlacementGroup, error) {
	return c.httpCreatePlacementGroup(ctx, req)
}

// CreatePlacementGroupProto creates a placement group as a proto message without
// automatic retry (a replay could repeat side effects).
func (c *Client) CreatePlacementGroupProto(ctx context.Context, req *CreatePlacementGroupRequest) (*linodev1.PlacementGroup, error) {
	return c.httpCreatePlacementGroupProto(ctx, req)
}

// UnassignPlacementGroup removes Linodes from a placement group without automatic retry.
// Replaying this unassign operation could repeat side effects after a transient failure.
func (c *Client) UnassignPlacementGroup(ctx context.Context, groupID int, req *PlacementGroupUnassignRequest) (*PlacementGroup, error) {
	return c.httpUnassignPlacementGroup(ctx, groupID, req)
}

// UnassignPlacementGroupProto removes Linodes from a placement group as a proto
// message without automatic retry.
func (c *Client) UnassignPlacementGroupProto(ctx context.Context, groupID int, req *PlacementGroupUnassignRequest) (*linodev1.PlacementGroup, error) {
	return c.httpUnassignPlacementGroupProto(ctx, groupID, req)
}

// ListImageShareGroups retrieves owned image share groups with automatic retry on transient failures.
func (c *Client) ListImageShareGroups(ctx context.Context, page, pageSize int) (*PaginatedResponse[ImageShareGroup], error) {
	var shareGroups *PaginatedResponse[ImageShareGroup]

	err := c.executeWithRetry(ctx, "ListImageShareGroups", func() error {
		var err error

		shareGroups, err = c.httpListImageShareGroups(ctx, page, pageSize)

		return err
	})

	return shareGroups, err
}

// ListImageShareGroupsProto retrieves owned image share groups as proto messages
// with automatic retry on transient failures.
func (c *Client) ListImageShareGroupsProto(ctx context.Context, page, pageSize int) ([]*linodev1.ImageShareGroup, error) {
	var shareGroups []*linodev1.ImageShareGroup

	err := c.executeWithRetry(ctx, "ListImageShareGroups", func() error {
		var retryErr error

		shareGroups, retryErr = c.httpListImageShareGroupsProto(ctx, page, pageSize)

		return retryErr
	})

	return shareGroups, err
}

// GetImageShareGroup retrieves a single image share group with automatic retry on transient failures.
func (c *Client) GetImageShareGroup(ctx context.Context, shareGroupID int) (*ImageShareGroup, error) {
	var shareGroup *ImageShareGroup

	err := c.executeWithRetry(ctx, "GetImageShareGroup", func() error {
		var err error

		shareGroup, err = c.httpGetImageShareGroup(ctx, shareGroupID)

		return err
	})

	return shareGroup, err
}

// GetImageShareGroupProto retrieves one image share group as a proto message with
// automatic retry on transient failures.
func (c *Client) GetImageShareGroupProto(ctx context.Context, shareGroupID int) (*linodev1.ImageShareGroup, error) {
	var shareGroup *linodev1.ImageShareGroup

	err := c.executeWithRetry(ctx, "GetImageShareGroup", func() error {
		var err error

		shareGroup, err = c.httpGetImageShareGroupProto(ctx, shareGroupID)

		return err
	})

	return shareGroup, err
}

// ListImageShareGroupsByImage retrieves share groups that contain an image with automatic retry on transient failures.
func (c *Client) ListImageShareGroupsByImage(ctx context.Context, imageID string, page, pageSize int) (*PaginatedResponse[ImageShareGroup], error) {
	var shareGroups *PaginatedResponse[ImageShareGroup]

	err := c.executeWithRetry(ctx, "ListImageShareGroupsByImage", func() error {
		var err error

		shareGroups, err = c.httpListImageShareGroupsByImage(ctx, imageID, page, pageSize)

		return err
	})

	return shareGroups, err
}

// ListImageShareGroupsByImageProto retrieves the share groups that contain an
// image as proto messages with automatic retry on transient failures.
func (c *Client) ListImageShareGroupsByImageProto(ctx context.Context, imageID string, page, pageSize int) ([]*linodev1.ImageShareGroup, error) {
	var shareGroups []*linodev1.ImageShareGroup

	err := c.executeWithRetry(ctx, "ListImageShareGroupsByImage", func() error {
		var retryErr error

		shareGroups, retryErr = c.httpListImageShareGroupsByImageProto(ctx, imageID, page, pageSize)

		return retryErr
	})

	return shareGroups, err
}

// ListImagesByShareGroup retrieves images shared in an owned image share group with automatic retry on transient failures.
func (c *Client) ListImagesByShareGroup(ctx context.Context, shareGroupID, page, pageSize int) (*PaginatedResponse[Image], error) {
	var images *PaginatedResponse[Image]

	err := c.executeWithRetry(ctx, "ListImagesByShareGroup", func() error {
		var err error

		images, err = c.httpListImagesByShareGroup(ctx, shareGroupID, page, pageSize)

		return err
	})

	return images, err
}

// ListImagesByShareGroupProto retrieves the images shared in an owned image share
// group as proto messages with automatic retry on transient failures.
func (c *Client) ListImagesByShareGroupProto(ctx context.Context, shareGroupID, page, pageSize int) ([]*linodev1.Image, error) {
	var images []*linodev1.Image

	err := c.executeWithRetry(ctx, "ListImagesByShareGroup", func() error {
		var retryErr error

		images, retryErr = c.httpListImagesByShareGroupProto(ctx, shareGroupID, page, pageSize)

		return retryErr
	})

	return images, err
}

// ListMembersByImageShareGroup retrieves members linked to an owned image share group with automatic retry on transient failures.
func (c *Client) ListMembersByImageShareGroup(ctx context.Context, shareGroupID, page, pageSize int) (*PaginatedResponse[ImageShareGroupMember], error) {
	var members *PaginatedResponse[ImageShareGroupMember]

	err := c.executeWithRetry(ctx, "ListMembersByImageShareGroup", func() error {
		var err error

		members, err = c.httpListMembersByImageShareGroup(ctx, shareGroupID, page, pageSize)

		return err
	})

	return members, err
}

// ListMembersByImageShareGroupProto retrieves members linked to an owned image
// share group as proto messages with automatic retry on transient failures.
func (c *Client) ListMembersByImageShareGroupProto(ctx context.Context, shareGroupID, page, pageSize int) ([]*linodev1.ImageShareGroupMember, error) {
	var members []*linodev1.ImageShareGroupMember

	err := c.executeWithRetry(ctx, "ListMembersByImageShareGroup", func() error {
		var retryErr error

		members, retryErr = c.httpListMembersByImageShareGroupProto(ctx, shareGroupID, page, pageSize)

		return retryErr
	})

	return members, err
}

// GetImageShareGroupMemberToken retrieves a member token linked to an owned image share group with automatic retry on transient failures.
func (c *Client) GetImageShareGroupMemberToken(ctx context.Context, shareGroupID int, tokenUUID string) (*ImageShareGroupMember, error) {
	var member *ImageShareGroupMember

	err := c.executeWithRetry(ctx, "GetImageShareGroupMemberToken", func() error {
		var err error

		member, err = c.httpGetImageShareGroupMemberToken(ctx, shareGroupID, tokenUUID)

		return err
	})

	return member, err
}

// GetImageShareGroupMemberTokenProto retrieves one image share group member token
// as a proto message with automatic retry on transient failures.
func (c *Client) GetImageShareGroupMemberTokenProto(ctx context.Context, shareGroupID int, tokenUUID string) (*linodev1.ImageShareGroupMember, error) {
	var member *linodev1.ImageShareGroupMember

	err := c.executeWithRetry(ctx, "GetImageShareGroupMemberToken", func() error {
		var err error

		member, err = c.httpGetImageShareGroupMemberTokenProto(ctx, shareGroupID, tokenUUID)

		return err
	})

	return member, err
}

// UpdateImageShareGroupMember updates a membership token label without automatic retry.
// Replaying this mutating operation could repeat side effects after a transient failure.
func (c *Client) UpdateImageShareGroupMember(ctx context.Context, shareGroupID int, tokenUUID string, req *UpdateImageShareGroupMemberRequest) (*ImageShareGroupMember, error) {
	return c.httpUpdateImageShareGroupMember(ctx, shareGroupID, tokenUUID, req)
}

// CreateImageShareGroup creates an image share group without automatic retry.
// Replaying this non-idempotent create operation could create duplicate share groups.
func (c *Client) CreateImageShareGroup(ctx context.Context, req *CreateImageShareGroupRequest) (*ImageShareGroup, error) {
	return c.httpCreateImageShareGroup(ctx, req)
}

// UploadImage creates an image upload target without automatic retry.
// Replaying this non-idempotent create operation could create duplicate uploads.
func (c *Client) UploadImage(ctx context.Context, req *UploadImageRequest) (*UploadImageResponse, error) {
	return c.httpUploadImage(ctx, req)
}

// AddImageShareGroupImages adds images to a share group without automatic retry.
// Replaying this non-idempotent operation could add images more than once or
// duplicate side effects if the server processed the first request.
func (c *Client) AddImageShareGroupImages(ctx context.Context, shareGroupID int, req *AddImageShareGroupImagesRequest) (*Image, error) {
	return c.httpAddImageShareGroupImages(ctx, shareGroupID, req)
}

// AddImageShareGroupMembers adds members to a share group without automatic retry.
// Replaying this non-idempotent operation could duplicate member-side effects if the server processed the first request.
func (c *Client) AddImageShareGroupMembers(ctx context.Context, shareGroupID int, req *AddImageShareGroupMembersRequest) (*ImageShareGroup, error) {
	return c.httpAddImageShareGroupMembers(ctx, shareGroupID, req)
}

// DeleteImageShareGroupImage revokes access to one shared image without automatic retry.
// Replaying this destructive operation could repeat side effects after a transient failure.
func (c *Client) DeleteImageShareGroupImage(ctx context.Context, shareGroupID, imageID int) error {
	return c.httpDeleteImageShareGroupImage(ctx, shareGroupID, imageID)
}

// UpdateImageShareGroup updates an image share group without automatic retry.
// Replaying this mutating operation could repeat side effects after a transient failure.
func (c *Client) UpdateImageShareGroup(ctx context.Context, shareGroupID int, req *UpdateImageShareGroupRequest) (*ImageShareGroup, error) {
	return c.httpUpdateImageShareGroup(ctx, shareGroupID, req)
}

// UpdateImageShareGroupImage updates a shared image without automatic retry.
// Replaying this mutating operation could repeat side effects after a transient failure.
func (c *Client) UpdateImageShareGroupImage(ctx context.Context, shareGroupID int, imageID string, req *UpdateImageShareGroupImageRequest) (*Image, error) {
	return c.httpUpdateImageShareGroupImage(ctx, shareGroupID, imageID, req)
}

// DeleteImageShareGroup deletes an owned image share group without automatic retry.
// Replaying this destructive operation could repeat side effects after a transient failure.
func (c *Client) DeleteImageShareGroup(ctx context.Context, shareGroupID int) error {
	return c.httpDeleteImageShareGroup(ctx, shareGroupID)
}

// ListImageShareGroupTokens retrieves image share group tokens with automatic retry on transient failures.
func (c *Client) ListImageShareGroupTokens(ctx context.Context, page, pageSize int) (*PaginatedResponse[ImageShareGroupToken], error) {
	var tokens *PaginatedResponse[ImageShareGroupToken]

	err := c.executeWithRetry(ctx, "ListImageShareGroupTokens", func() error {
		var err error

		tokens, err = c.httpListImageShareGroupTokens(ctx, page, pageSize)

		return err
	})

	return tokens, err
}

// ListImageShareGroupTokensProto retrieves image share group tokens for the user
// as proto messages with automatic retry on transient failures.
func (c *Client) ListImageShareGroupTokensProto(ctx context.Context, page, pageSize int) ([]*linodev1.ImageShareGroupToken, error) {
	var tokens []*linodev1.ImageShareGroupToken

	err := c.executeWithRetry(ctx, "ListImageShareGroupTokens", func() error {
		var retryErr error

		tokens, retryErr = c.httpListImageShareGroupTokensProto(ctx, page, pageSize)

		return retryErr
	})

	return tokens, err
}

// CreateImageShareGroupToken creates a single-use image share group membership token without automatic retry.
// Replaying this non-idempotent create operation could create duplicate token material.
func (c *Client) CreateImageShareGroupToken(ctx context.Context, req *CreateImageShareGroupTokenRequest) (*ImageShareGroupToken, error) {
	return c.httpCreateImageShareGroupToken(ctx, req)
}

// GetImageShareGroupToken retrieves an image share group token with automatic retry on transient failures.
func (c *Client) GetImageShareGroupToken(ctx context.Context, tokenUUID string) (*ImageShareGroupToken, error) {
	var token *ImageShareGroupToken

	err := c.executeWithRetry(ctx, "GetImageShareGroupToken", func() error {
		var err error

		token, err = c.httpGetImageShareGroupToken(ctx, tokenUUID)

		return err
	})

	return token, err
}

// GetImageShareGroupTokenProto retrieves one image share group token as a proto
// message with automatic retry on transient failures.
func (c *Client) GetImageShareGroupTokenProto(ctx context.Context, tokenUUID string) (*linodev1.ImageShareGroupToken, error) {
	var token *linodev1.ImageShareGroupToken

	err := c.executeWithRetry(ctx, "GetImageShareGroupToken", func() error {
		var err error

		token, err = c.httpGetImageShareGroupTokenProto(ctx, tokenUUID)

		return err
	})

	return token, err
}

// ListImagesByShareGroupToken retrieves images available through an image share group token with automatic retry on transient failures.
func (c *Client) ListImagesByShareGroupToken(ctx context.Context, tokenUUID string, page, pageSize int) (*PaginatedResponse[Image], error) {
	var images *PaginatedResponse[Image]

	err := c.executeWithRetry(ctx, "ListImagesByShareGroupToken", func() error {
		var err error

		images, err = c.httpListImagesByShareGroupToken(ctx, tokenUUID, page, pageSize)

		return err
	})

	return images, err
}

// ListImagesByShareGroupTokenProto retrieves the images available through an
// image share group token as proto messages with automatic retry on transient
// failures.
func (c *Client) ListImagesByShareGroupTokenProto(ctx context.Context, tokenUUID string, page, pageSize int) ([]*linodev1.Image, error) {
	var images []*linodev1.Image

	err := c.executeWithRetry(ctx, "ListImagesByShareGroupToken", func() error {
		var retryErr error

		images, retryErr = c.httpListImagesByShareGroupTokenProto(ctx, tokenUUID, page, pageSize)

		return retryErr
	})

	return images, err
}

// UpdateImageShareGroupToken updates a token label without automatic retry.
// Replaying this mutating token operation could repeat side effects after a transient failure.
func (c *Client) UpdateImageShareGroupToken(ctx context.Context, tokenUUID string, req *UpdateImageShareGroupTokenRequest) (*ImageShareGroupToken, error) {
	return c.httpUpdateImageShareGroupToken(ctx, tokenUUID, req)
}

// DeleteImageShareGroupToken removes one image share group membership token without automatic retry.
// Replaying this destructive DELETE could remove or race token state after a transient response.
func (c *Client) DeleteImageShareGroupToken(ctx context.Context, tokenUUID string) error {
	return c.httpDeleteImageShareGroupToken(ctx, tokenUUID)
}

// DeleteImageShareGroupMemberToken revokes one accepted membership token without automatic retry.
// Replaying this destructive DELETE could repeat revocation side effects after a transient response.
func (c *Client) DeleteImageShareGroupMemberToken(ctx context.Context, shareGroupID int, tokenUUID string) error {
	return c.httpDeleteImageShareGroupMemberToken(ctx, shareGroupID, tokenUUID)
}

// GetImageShareGroupByToken retrieves a token's share group with automatic retry on transient failures.
func (c *Client) GetImageShareGroupByToken(ctx context.Context, tokenUUID string) (*ImageShareGroup, error) {
	var shareGroup *ImageShareGroup

	err := c.executeWithRetry(ctx, "GetImageShareGroupByToken", func() error {
		var err error

		shareGroup, err = c.httpGetImageShareGroupByToken(ctx, tokenUUID)

		return err
	})

	return shareGroup, err
}

// GetImageShareGroupByTokenProto resolves a token to its parent share group as a
// proto message with automatic retry on transient failures.
func (c *Client) GetImageShareGroupByTokenProto(ctx context.Context, tokenUUID string) (*linodev1.ImageShareGroup, error) {
	var shareGroup *linodev1.ImageShareGroup

	err := c.executeWithRetry(ctx, "GetImageShareGroupByToken", func() error {
		var err error

		shareGroup, err = c.httpGetImageShareGroupByTokenProto(ctx, tokenUUID)

		return err
	})

	return shareGroup, err
}

// CreateImage creates a private image from a Linode disk without automatic retry.
// Replaying this non-idempotent create operation could create duplicate images.
func (c *Client) CreateImage(ctx context.Context, req *CreateImageRequest) (*Image, error) {
	return c.httpCreateImage(ctx, req)
}

// CreateImageProto creates an image as a proto message without automatic retry.
func (c *Client) CreateImageProto(ctx context.Context, req *CreateImageRequest) (*linodev1.Image, error) {
	return c.httpCreateImageProto(ctx, req)
}

// ListSSHKeys retrieves all SSH keys with automatic retry on transient failures.
func (c *Client) ListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	var keys []SSHKey

	err := c.executeWithRetry(ctx, "ListSSHKeys", func() error {
		var err error

		keys, err = c.httpListSSHKeys(ctx)

		return err
	})

	return keys, err
}

// ListSSHKeysProto retrieves all SSH keys as proto messages with automatic retry
// on transient failures.
func (c *Client) ListSSHKeysProto(ctx context.Context) ([]*linodev1.SSHKey, error) {
	var keys []*linodev1.SSHKey

	err := c.executeWithRetry(ctx, "ListSSHKeys", func() error {
		var retryErr error

		keys, retryErr = c.httpListSSHKeysProto(ctx)

		return retryErr
	})

	return keys, err
}

// ListDomains retrieves all domains with automatic retry on transient failures.
func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	var domains []Domain

	err := c.executeWithRetry(ctx, "ListDomains", func() error {
		var err error

		domains, err = c.httpListDomains(ctx)

		return err
	})

	return domains, err
}

// ListDomainsProto retrieves all domains as proto messages with automatic retry
// on transient failures.
func (c *Client) ListDomainsProto(ctx context.Context) ([]*linodev1.Domain, error) {
	var domains []*linodev1.Domain

	err := c.executeWithRetry(ctx, "ListDomains", func() error {
		var err error

		domains, err = c.httpListDomainsProto(ctx)

		return err
	})

	return domains, err
}

// GetDomain retrieves a single domain by ID with automatic retry on transient failures.
func (c *Client) GetDomain(ctx context.Context, domainID int) (*Domain, error) {
	var domain *Domain

	err := c.executeWithRetry(ctx, "GetDomain", func() error {
		var err error

		domain, err = c.httpGetDomain(ctx, domainID)

		return err
	})

	return domain, err
}

// GetDomainProto retrieves a domain as a proto message with automatic retry on
// transient failures.
func (c *Client) GetDomainProto(ctx context.Context, domainID int) (*linodev1.Domain, error) {
	var domain *linodev1.Domain

	err := c.executeWithRetry(ctx, "GetDomain", func() error {
		var err error

		domain, err = c.httpGetDomainProto(ctx, domainID)

		return err
	})

	return domain, err
}

// ListDomainRecords retrieves all records for a domain with automatic retry on transient failures.
func (c *Client) ListDomainRecords(ctx context.Context, domainID int) ([]DomainRecord, error) {
	var records []DomainRecord

	err := c.executeWithRetry(ctx, "ListDomainRecords", func() error {
		var err error

		records, err = c.httpListDomainRecords(ctx, domainID)

		return err
	})

	return records, err
}

// ListDomainRecordsProto retrieves a domain's DNS records as proto messages with
// automatic retry on transient failures.
func (c *Client) ListDomainRecordsProto(ctx context.Context, domainID int) ([]*linodev1.DomainRecord, error) {
	var records []*linodev1.DomainRecord

	err := c.executeWithRetry(ctx, "ListDomainRecords", func() error {
		var retryErr error

		records, retryErr = c.httpListDomainRecordsProto(ctx, domainID)

		return retryErr
	})

	return records, err
}

// GetDomainZoneFile retrieves the rendered zone file for a domain with automatic retry on transient failures.
func (c *Client) GetDomainZoneFile(ctx context.Context, domainID int) (*DomainZoneFile, error) {
	var zoneFile *DomainZoneFile

	err := c.executeWithRetry(ctx, "GetDomainZoneFile", func() error {
		var err error

		zoneFile, err = c.httpGetDomainZoneFile(ctx, domainID)

		return err
	})

	return zoneFile, err
}

// GetDomainZoneFileProto retrieves a domain's rendered zone file as a proto
// message with automatic retry on transient failures.
func (c *Client) GetDomainZoneFileProto(ctx context.Context, domainID int) (*linodev1.DomainZoneFile, error) {
	var zoneFile *linodev1.DomainZoneFile

	err := c.executeWithRetry(ctx, "GetDomainZoneFile", func() error {
		var err error

		zoneFile, err = c.httpGetDomainZoneFileProto(ctx, domainID)

		return err
	})

	return zoneFile, err
}

// ListFirewalls retrieves all firewalls with automatic retry on transient failures.
func (c *Client) ListFirewalls(ctx context.Context) ([]Firewall, error) {
	var firewalls []Firewall

	err := c.executeWithRetry(ctx, "ListFirewalls", func() error {
		var err error

		firewalls, err = c.httpListFirewalls(ctx)

		return err
	})

	return firewalls, err
}

// ListFirewallsProto retrieves all firewalls as proto messages with automatic
// retry on transient failures.
func (c *Client) ListFirewallsProto(ctx context.Context) ([]*linodev1.Firewall, error) {
	var firewalls []*linodev1.Firewall

	err := c.executeWithRetry(ctx, "ListFirewalls", func() error {
		var err error

		firewalls, err = c.httpListFirewallsProto(ctx)

		return err
	})

	return firewalls, err
}

// ListVLANs retrieves all VLANs with automatic retry on transient failures.
func (c *Client) ListVLANs(ctx context.Context, page, pageSize int) (*PaginatedResponse[VLAN], error) {
	var vlans *PaginatedResponse[VLAN]

	err := c.executeWithRetry(ctx, "ListVLANs", func() error {
		var err error

		vlans, err = c.httpListVLANs(ctx, page, pageSize)

		return err
	})

	return vlans, err
}

// ListVLANsProto retrieves VLANs as proto messages with automatic retry on
// transient failures.
func (c *Client) ListVLANsProto(ctx context.Context, page, pageSize int) ([]*linodev1.VLAN, error) {
	var vlans []*linodev1.VLAN

	err := c.executeWithRetry(ctx, "ListVLANs", func() error {
		var err error

		vlans, err = c.httpListVLANsProto(ctx, page, pageSize)

		return err
	})

	return vlans, err
}

// DeleteVLAN deletes one VLAN without retrying the destructive request.
// Retrying can replay VLAN deletion after a transient error, so this method
// delegates exactly once.
func (c *Client) DeleteVLAN(ctx context.Context, regionID, label string) error {
	return c.executeWithoutRetry(ctx, "DeleteVLAN", func() error {
		return c.httpDeleteVLAN(ctx, regionID, label)
	})
}

// ListFirewallRules retrieves firewall rules with automatic retry on transient failures.
func (c *Client) ListFirewallRules(ctx context.Context, firewallID int) (*FirewallRules, error) {
	var rules *FirewallRules

	err := c.executeWithRetry(ctx, "ListFirewallRules", func() error {
		var err error

		rules, err = c.httpListFirewallRules(ctx, firewallID)

		return err
	})

	return rules, err
}

// ListFirewallRulesProto retrieves a firewall's ruleset as a proto message with
// automatic retry on transient failures.
func (c *Client) ListFirewallRulesProto(ctx context.Context, firewallID int) (*linodev1.FirewallRules, error) {
	var rules *linodev1.FirewallRules

	err := c.executeWithRetry(ctx, "ListFirewallRules", func() error {
		var err error

		rules, err = c.httpListFirewallRulesProto(ctx, firewallID)

		return err
	})

	return rules, err
}

// UpdateFirewallRules replaces firewall rules without retrying the mutating PUT request.
func (c *Client) UpdateFirewallRules(ctx context.Context, firewallID int, req *FirewallRules) (*FirewallRules, error) {
	var rules *FirewallRules

	err := c.executeWithoutRetry(ctx, "UpdateFirewallRules", func() error {
		var err error

		rules, err = c.httpUpdateFirewallRules(ctx, firewallID, req)

		return err
	})

	return rules, err
}

// ListFirewallRuleVersionsProto retrieves a Cloud Firewall's rule-version history
// as proto messages with automatic retry on transient failures.
func (c *Client) ListFirewallRuleVersionsProto(ctx context.Context, firewallID int) ([]*linodev1.FirewallRuleVersion, error) {
	var versions []*linodev1.FirewallRuleVersion

	err := c.executeWithRetry(ctx, "ListFirewallRuleVersions", func() error {
		var retryErr error

		versions, retryErr = c.httpListFirewallRuleVersionsProto(ctx, firewallID)

		return retryErr
	})

	return versions, err
}

// GetFirewallRuleVersion retrieves one firewall rule version with automatic retry on transient failures.
func (c *Client) GetFirewallRuleVersion(ctx context.Context, firewallID, version int) (*Firewall, error) {
	var firewall *Firewall

	err := c.executeWithRetry(ctx, "GetFirewallRuleVersion", func() error {
		var err error

		firewall, err = c.httpGetFirewallRuleVersion(ctx, firewallID, version)

		return err
	})

	return firewall, err
}

// GetFirewallRuleVersionProto retrieves one rule-version snapshot and decodes the
// response into the FirewallRuleVersion proto element with automatic retry on
// transient failures.
func (c *Client) GetFirewallRuleVersionProto(ctx context.Context, firewallID, version int) (*linodev1.FirewallRuleVersion, error) {
	var ruleVersion *linodev1.FirewallRuleVersion

	err := c.executeWithRetry(ctx, "GetFirewallRuleVersion", func() error {
		var err error

		ruleVersion, err = c.httpGetFirewallRuleVersionProto(ctx, firewallID, version)

		return err
	})

	return ruleVersion, err
}

// ListFirewallDevices retrieves devices assigned to a Cloud Firewall with automatic retry on transient failures.
func (c *Client) ListFirewallDevices(ctx context.Context, firewallID, page, pageSize int) (*PaginatedResponse[FirewallDevice], error) {
	var devices *PaginatedResponse[FirewallDevice]

	err := c.executeWithRetry(ctx, "ListFirewallDevices", func() error {
		var err error

		devices, err = c.httpListFirewallDevices(ctx, firewallID, page, pageSize)

		return err
	})

	return devices, err
}

// ListFirewallDevicesProto retrieves a Cloud Firewall's assigned devices as proto
// messages with automatic retry on transient failures.
func (c *Client) ListFirewallDevicesProto(ctx context.Context, firewallID, page, pageSize int) ([]*linodev1.FirewallDevice, error) {
	var devices []*linodev1.FirewallDevice

	err := c.executeWithRetry(ctx, "ListFirewallDevices", func() error {
		var retryErr error

		devices, retryErr = c.httpListFirewallDevicesProto(ctx, firewallID, page, pageSize)

		return retryErr
	})

	return devices, err
}

// CreateFirewallDevice assigns a device to a Cloud Firewall without retrying the mutating request.
func (c *Client) CreateFirewallDevice(ctx context.Context, firewallID int, req *CreateFirewallDeviceRequest) (*FirewallDevice, error) {
	var device *FirewallDevice

	err := c.executeWithoutRetry(ctx, "CreateFirewallDevice", func() error {
		var err error

		device, err = c.httpCreateFirewallDevice(ctx, firewallID, req)

		return err
	})

	return device, err
}

// GetFirewallDevice retrieves one device assigned to a Cloud Firewall with automatic retry on transient failures.
func (c *Client) GetFirewallDevice(ctx context.Context, firewallID, deviceID int) (*FirewallDevice, error) {
	var device *FirewallDevice

	err := c.executeWithRetry(ctx, "GetFirewallDevice", func() error {
		var err error

		device, err = c.httpGetFirewallDevice(ctx, firewallID, deviceID)

		return err
	})

	return device, err
}

// GetFirewallDeviceProto retrieves one firewall device as a proto message with
// automatic retry on transient failures.
func (c *Client) GetFirewallDeviceProto(ctx context.Context, firewallID, deviceID int) (*linodev1.FirewallDevice, error) {
	var device *linodev1.FirewallDevice

	err := c.executeWithRetry(ctx, "GetFirewallDevice", func() error {
		var err error

		device, err = c.httpGetFirewallDeviceProto(ctx, firewallID, deviceID)

		return err
	})

	return device, err
}

// DeleteFirewallDevice removes one device assignment from a Cloud Firewall without retrying the mutating request.
func (c *Client) DeleteFirewallDevice(ctx context.Context, firewallID, deviceID int) error {
	return c.executeWithoutRetry(ctx, "DeleteFirewallDevice", func() error {
		return c.httpDeleteFirewallDevice(ctx, firewallID, deviceID)
	})
}

// ListFirewallSettings retrieves default firewall assignments with automatic retry on transient failures.
func (c *Client) ListFirewallSettings(ctx context.Context, page, pageSize int) (*FirewallSettings, error) {
	var settings *FirewallSettings

	err := c.executeWithRetry(ctx, "ListFirewallSettings", func() error {
		var err error

		settings, err = c.httpListFirewallSettings(ctx, page, pageSize)

		return err
	})

	return settings, err
}

// ListFirewallTemplates retrieves reusable Cloud Firewall templates with automatic retry on transient failures.
func (c *Client) ListFirewallTemplates(ctx context.Context, page, pageSize int) (*PaginatedResponse[FirewallTemplate], error) {
	var templates *PaginatedResponse[FirewallTemplate]

	err := c.executeWithRetry(ctx, "ListFirewallTemplates", func() error {
		var err error

		templates, err = c.httpListFirewallTemplates(ctx, page, pageSize)

		return err
	})

	return templates, err
}

// GetFirewallTemplate retrieves a reusable Cloud Firewall template by slug with automatic retry on transient failures.
func (c *Client) GetFirewallTemplate(ctx context.Context, slug string, page, pageSize int) (*PaginatedResponse[FirewallTemplate], error) {
	var template *PaginatedResponse[FirewallTemplate]

	err := c.executeWithRetry(ctx, "GetFirewallTemplate", func() error {
		var err error

		template, err = c.httpGetFirewallTemplate(ctx, slug, page, pageSize)

		return err
	})

	return template, err
}

// GetFirewallTemplateProto retrieves a reusable Cloud Firewall template by slug
// and decodes the response into the FirewallTemplate proto element with automatic
// retry on transient failures.
func (c *Client) GetFirewallTemplateProto(ctx context.Context, slug string, page, pageSize int) (*linodev1.FirewallTemplate, error) {
	var template *linodev1.FirewallTemplate

	err := c.executeWithRetry(ctx, "GetFirewallTemplate", func() error {
		var err error

		template, err = c.httpGetFirewallTemplateProto(ctx, slug, page, pageSize)

		return err
	})

	return template, err
}

// UpdateFirewallSettings updates default firewall assignments without retrying the mutating request.
func (c *Client) UpdateFirewallSettings(ctx context.Context, req *UpdateFirewallSettingsRequest) (*FirewallSettings, error) {
	var settings *FirewallSettings

	err := c.executeWithoutRetry(ctx, "UpdateFirewallSettings", func() error {
		var err error

		settings, err = c.httpUpdateFirewallSettings(ctx, req)

		return err
	})

	return settings, err
}

// ListNetworkingIPs retrieves all account IP addresses with automatic retry on transient failures.
func (c *Client) ListNetworkingIPs(ctx context.Context, skipIPv6RDNS bool) (*PaginatedResponse[IPAddress], error) {
	var ips *PaginatedResponse[IPAddress]

	err := c.executeWithRetry(ctx, "ListNetworkingIPs", func() error {
		var retryErr error

		ips, retryErr = c.httpListNetworkingIPs(ctx, skipIPv6RDNS)

		return retryErr
	})

	return ips, err
}

// ListNetworkingIPsProto retrieves all account IP addresses as proto messages
// with automatic retry on transient failures.
func (c *Client) ListNetworkingIPsProto(ctx context.Context, skipIPv6RDNS bool) ([]*linodev1.IPAddress, error) {
	var ips []*linodev1.IPAddress

	err := c.executeWithRetry(ctx, "ListNetworkingIPs", func() error {
		var retryErr error

		ips, retryErr = c.httpListNetworkingIPsProto(ctx, skipIPv6RDNS)

		return retryErr
	})

	return ips, err
}

// GetNetworkingIP retrieves an account-level IP address with automatic retry on transient failures.
func (c *Client) GetNetworkingIP(ctx context.Context, address string) (*IPAddress, error) {
	var networkingIPAddr *IPAddress

	err := c.executeWithRetry(ctx, "GetNetworkingIP", func() error {
		var retryErr error

		networkingIPAddr, retryErr = c.httpGetNetworkingIP(ctx, address)

		return retryErr
	})

	return networkingIPAddr, err
}

// GetNetworkingIPProto retrieves a networking IP as a proto message with automatic
// retry on transient failures.
func (c *Client) GetNetworkingIPProto(ctx context.Context, address string) (*linodev1.IPAddress, error) {
	var networkingIPAddr *linodev1.IPAddress

	err := c.executeWithRetry(ctx, "GetNetworkingIP", func() error {
		var retryErr error

		networkingIPAddr, retryErr = c.httpGetNetworkingIPProto(ctx, address)

		return retryErr
	})

	return networkingIPAddr, err
}

// UpdateNetworkingIP updates reverse DNS for an account-level IP address without retrying the mutating PUT.
func (c *Client) UpdateNetworkingIP(ctx context.Context, address string, req UpdateNetworkingIPRequest) (*IPAddress, error) {
	var ipAddr *IPAddress

	err := c.executeWithoutRetry(ctx, "UpdateNetworkingIP", func() error {
		var err error

		ipAddr, err = c.httpUpdateNetworkingIP(ctx, address, req)

		return err
	})

	return ipAddr, err
}

// UpdateNetworkingIPProto updates a networking IP as a proto message without
// automatic retry, matching the non-proto sibling.
func (c *Client) UpdateNetworkingIPProto(ctx context.Context, address string, req UpdateNetworkingIPRequest) (*linodev1.IPAddress, error) {
	var ipAddr *linodev1.IPAddress

	err := c.executeWithoutRetry(ctx, "UpdateNetworkingIP", func() error {
		var err error

		ipAddr, err = c.httpUpdateNetworkingIPProto(ctx, address, req)

		return err
	})

	return ipAddr, err
}

// AllocateNetworkingIP allocates an account-level IP address without retrying the non-idempotent POST.
func (c *Client) AllocateNetworkingIP(ctx context.Context, req AllocateNetworkingIPRequest) (*IPAddress, error) {
	var ipAddr *IPAddress

	err := c.executeWithoutRetry(ctx, "AllocateNetworkingIP", func() error {
		var err error

		ipAddr, err = c.httpAllocateNetworkingIP(ctx, req)

		return err
	})

	return ipAddr, err
}

// AllocateNetworkingIPProto allocates an account-level IP address and returns
// the proto IPAddress element. The POST is non-idempotent, so it is not retried.
func (c *Client) AllocateNetworkingIPProto(ctx context.Context, req AllocateNetworkingIPRequest) (*linodev1.IPAddress, error) {
	var ipAddr *linodev1.IPAddress

	err := c.executeWithoutRetry(ctx, "AllocateNetworkingIP", func() error {
		var err error

		ipAddr, err = c.httpAllocateNetworkingIPProto(ctx, req)

		return err
	})

	return ipAddr, err
}

// AssignNetworkingIPs assigns IP addresses without retrying the non-idempotent POST.
func (c *Client) AssignNetworkingIPs(ctx context.Context, req AssignNetworkingIPsRequest) (map[string]any, error) {
	var response map[string]any

	err := c.executeWithoutRetry(ctx, "AssignNetworkingIPs", func() error {
		var err error

		response, err = c.httpAssignNetworkingIPs(ctx, req)

		return err
	})

	return response, err
}

// AssignNetworkingIPv4s assigns IPv4 addresses without retrying the non-idempotent POST.
func (c *Client) AssignNetworkingIPv4s(ctx context.Context, req AssignNetworkingIPsRequest) (map[string]any, error) {
	var response map[string]any

	err := c.executeWithoutRetry(ctx, "AssignNetworkingIPv4s", func() error {
		var err error

		response, err = c.httpAssignNetworkingIPv4s(ctx, req)

		return err
	})

	return response, err
}

// ShareNetworkingIPv4s shares IP addresses without retrying the non-idempotent POST.
func (c *Client) ShareNetworkingIPv4s(ctx context.Context, req ShareNetworkingIPsRequest) (map[string]any, error) {
	var response map[string]any

	err := c.executeWithoutRetry(ctx, "ShareNetworkingIPv4s", func() error {
		var err error

		response, err = c.httpShareNetworkingIPv4s(ctx, req)

		return err
	})

	return response, err
}

// ShareNetworkingIPs shares IP addresses via the generic /networking/ips/share
// endpoint without retrying the non-idempotent POST.
func (c *Client) ShareNetworkingIPs(ctx context.Context, req ShareNetworkingIPsRequest) (map[string]any, error) {
	var response map[string]any

	err := c.executeWithoutRetry(ctx, "ShareNetworkingIPs", func() error {
		var err error

		response, err = c.httpShareNetworkingIPs(ctx, req)

		return err
	})

	return response, err
}

// ListNetworkTransferPrices retrieves network transfer prices with automatic retry on transient failures.
func (c *Client) ListNetworkTransferPrices(ctx context.Context) (*PaginatedResponse[NetworkTransferPrice], error) {
	var prices *PaginatedResponse[NetworkTransferPrice]

	err := c.executeWithRetry(ctx, "ListNetworkTransferPrices", func() error {
		var err error

		prices, err = c.httpListNetworkTransferPrices(ctx)

		return err
	})

	return prices, err
}

// ListIPv6Pools retrieves IPv6 pools with automatic retry on transient failures.
func (c *Client) ListIPv6Pools(ctx context.Context, page, pageSize int) (*PaginatedResponse[IPv6Pool], error) {
	var pools *PaginatedResponse[IPv6Pool]

	err := c.executeWithRetry(ctx, "ListIPv6Pools", func() error {
		var err error

		pools, err = c.httpListIPv6Pools(ctx, page, pageSize)

		return err
	})

	return pools, err
}

// ListIPv6Ranges retrieves IPv6 ranges with automatic retry on transient failures.
func (c *Client) ListIPv6Ranges(ctx context.Context, page, pageSize int) (*PaginatedResponse[IPv6Range], error) {
	var ranges *PaginatedResponse[IPv6Range]

	err := c.executeWithRetry(ctx, "ListIPv6Ranges", func() error {
		var err error

		ranges, err = c.httpListIPv6Ranges(ctx, page, pageSize)

		return err
	})

	return ranges, err
}

// CreateIPv6Range creates an IPv6 range without retrying the non-idempotent POST.
func (c *Client) CreateIPv6Range(ctx context.Context, req CreateIPv6RangeRequest) (*IPv6Range, error) {
	var ipv6Range *IPv6Range

	err := c.executeWithoutRetry(ctx, "CreateIPv6Range", func() error {
		var err error

		ipv6Range, err = c.httpCreateIPv6Range(ctx, req)

		return err
	})

	return ipv6Range, err
}

// GetIPv6Range retrieves one IPv6 range with automatic retry on transient failures.
func (c *Client) GetIPv6Range(ctx context.Context, ipv6Range string) (*IPv6Range, error) {
	var result *IPv6Range

	err := c.executeWithRetry(ctx, "GetIPv6Range", func() error {
		var err error

		result, err = c.httpGetIPv6Range(ctx, ipv6Range)

		return err
	})

	return result, err
}

// DeleteIPv6Range deletes one IPv6 range without retrying the destructive DELETE.
func (c *Client) DeleteIPv6Range(ctx context.Context, ipv6Range string) error {
	return c.executeWithoutRetry(ctx, "DeleteIPv6Range", func() error {
		return c.httpDeleteIPv6Range(ctx, ipv6Range)
	})
}

// ListNodeBalancerTypes retrieves available node balancer types with automatic retry on transient failures.
func (c *Client) ListNodeBalancerTypes(ctx context.Context) (*PaginatedResponse[NodeBalancerType], error) {
	var types *PaginatedResponse[NodeBalancerType]

	err := c.executeWithRetry(ctx, "ListNodeBalancerTypes", func() error {
		var err error

		types, err = c.httpListNodeBalancerTypes(ctx)

		return err
	})

	return types, err
}

// ListNodeBalancerTypesProto retrieves available NodeBalancer types as proto
// messages with automatic retry on transient failures.
func (c *Client) ListNodeBalancerTypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	var types []*linodev1.LinodeType

	err := c.executeWithRetry(ctx, "ListNodeBalancerTypes", func() error {
		var retryErr error

		types, retryErr = c.httpListNodeBalancerTypesProto(ctx)

		return retryErr
	})

	return types, err
}

// ListNodeBalancers retrieves all node balancers with automatic retry on transient failures.
func (c *Client) ListNodeBalancers(ctx context.Context) ([]NodeBalancer, error) {
	var nodeBalancers []NodeBalancer

	err := c.executeWithRetry(ctx, "ListNodeBalancers", func() error {
		var err error

		nodeBalancers, err = c.httpListNodeBalancers(ctx)

		return err
	})

	return nodeBalancers, err
}

// ListNodeBalancersProto retrieves all NodeBalancers as proto messages with
// automatic retry on transient failures.
func (c *Client) ListNodeBalancersProto(ctx context.Context) ([]*linodev1.NodeBalancer, error) {
	var nodeBalancers []*linodev1.NodeBalancer

	err := c.executeWithRetry(ctx, "ListNodeBalancers", func() error {
		var err error

		nodeBalancers, err = c.httpListNodeBalancersProto(ctx)

		return err
	})

	return nodeBalancers, err
}

// GetNodeBalancer retrieves a single node balancer by ID with automatic retry on transient failures.
func (c *Client) GetNodeBalancer(ctx context.Context, nodeBalancerID int) (*NodeBalancer, error) {
	var nodeBalancer *NodeBalancer

	err := c.executeWithRetry(ctx, "GetNodeBalancer", func() error {
		var err error

		nodeBalancer, err = c.httpGetNodeBalancer(ctx, nodeBalancerID)

		return err
	})

	return nodeBalancer, err
}

// GetNodeBalancerProto retrieves a NodeBalancer as a proto message with automatic
// retry on transient failures.
func (c *Client) GetNodeBalancerProto(ctx context.Context, nodeBalancerID int) (*linodev1.NodeBalancer, error) {
	var nodeBalancer *linodev1.NodeBalancer

	err := c.executeWithRetry(ctx, "GetNodeBalancer", func() error {
		var err error

		nodeBalancer, err = c.httpGetNodeBalancerProto(ctx, nodeBalancerID)

		return err
	})

	return nodeBalancer, err
}

// GetNodeBalancerVPCConfig retrieves a NodeBalancer VPC configuration by ID with automatic retry on transient failures.
func (c *Client) GetNodeBalancerVPCConfig(ctx context.Context, nodeBalancerID, vpcConfigID int) (*NodeBalancerVPCConfig, error) {
	var config *NodeBalancerVPCConfig

	err := c.executeWithRetry(ctx, "GetNodeBalancerVPCConfig", func() error {
		var err error

		config, err = c.httpGetNodeBalancerVPCConfig(ctx, nodeBalancerID, vpcConfigID)

		return err
	})

	return config, err
}

// GetNodeBalancerVPCConfigProto retrieves one NodeBalancer VPC config as a proto
// message with automatic retry on transient failures.
func (c *Client) GetNodeBalancerVPCConfigProto(ctx context.Context, nodeBalancerID, vpcConfigID int) (*linodev1.NodeBalancerVPCConfig, error) {
	var config *linodev1.NodeBalancerVPCConfig

	err := c.executeWithRetry(ctx, "GetNodeBalancerVPCConfig", func() error {
		var err error

		config, err = c.httpGetNodeBalancerVPCConfigProto(ctx, nodeBalancerID, vpcConfigID)

		return err
	})

	return config, err
}

// ListNodeBalancerConfigs retrieves configs for a node balancer by ID with automatic retry on transient failures.
func (c *Client) ListNodeBalancerConfigs(ctx context.Context, nodeBalancerID, page, pageSize int) ([]NodeBalancerConfig, error) {
	var configs []NodeBalancerConfig

	err := c.executeWithRetry(ctx, "ListNodeBalancerConfigs", func() error {
		var err error

		configs, err = c.httpListNodeBalancerConfigs(ctx, nodeBalancerID, page, pageSize)

		return err
	})

	return configs, err
}

// ListNodeBalancerConfigsProto retrieves a NodeBalancer's configs as proto
// messages with automatic retry on transient failures.
func (c *Client) ListNodeBalancerConfigsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.NodeBalancerConfig, error) {
	var configs []*linodev1.NodeBalancerConfig

	err := c.executeWithRetry(ctx, "ListNodeBalancerConfigs", func() error {
		var retryErr error

		configs, retryErr = c.httpListNodeBalancerConfigsProto(ctx, nodeBalancerID, page, pageSize)

		return retryErr
	})

	return configs, err
}

// ListNodeBalancerFirewalls retrieves Cloud Firewalls assigned to a NodeBalancer with automatic retry on transient failures.
func (c *Client) ListNodeBalancerFirewalls(ctx context.Context, nodeBalancerID, page, pageSize int) ([]Firewall, error) {
	var firewalls []Firewall

	err := c.executeWithRetry(ctx, "ListNodeBalancerFirewalls", func() error {
		var err error

		firewalls, err = c.httpListNodeBalancerFirewalls(ctx, nodeBalancerID, page, pageSize)

		return err
	})

	return firewalls, err
}

// ListNodeBalancerFirewallsProto retrieves Cloud Firewalls assigned to a
// NodeBalancer as proto messages with automatic retry on transient failures.
func (c *Client) ListNodeBalancerFirewallsProto(ctx context.Context, nodeBalancerID, page, pageSize int) ([]*linodev1.Firewall, error) {
	var firewalls []*linodev1.Firewall

	err := c.executeWithRetry(ctx, "ListNodeBalancerFirewalls", func() error {
		var retryErr error

		firewalls, retryErr = c.httpListNodeBalancerFirewallsProto(ctx, nodeBalancerID, page, pageSize)

		return retryErr
	})

	return firewalls, err
}

// UpdateNodeBalancerFirewalls replaces firewall assignments for a NodeBalancer without replaying the state-changing request.
func (c *Client) UpdateNodeBalancerFirewalls(ctx context.Context, nodeBalancerID, page, pageSize int, req *UpdateNodeBalancerFirewallsRequest) ([]Firewall, error) {
	var firewalls []Firewall

	err := c.executeWithoutRetry(ctx, "UpdateNodeBalancerFirewalls", func() error {
		var retryErr error

		firewalls, retryErr = c.httpUpdateNodeBalancerFirewalls(ctx, nodeBalancerID, page, pageSize, req)

		return retryErr
	})

	return firewalls, err
}

// ListNodeBalancerConfigNodes retrieves nodes for a node balancer config with automatic retry on transient failures.
func (c *Client) ListNodeBalancerConfigNodes(ctx context.Context, nodeBalancerID, configID, page, pageSize int) (*PaginatedResponse[NodeBalancerConfigNode], error) {
	var nodes *PaginatedResponse[NodeBalancerConfigNode]

	err := c.executeWithRetry(ctx, "ListNodeBalancerConfigNodes", func() error {
		var retryErr error

		nodes, retryErr = c.httpListNodeBalancerConfigNodes(ctx, nodeBalancerID, configID, page, pageSize)

		return retryErr
	})

	return nodes, err
}

// ListNodeBalancerConfigNodesProto retrieves a NodeBalancer config's backend
// nodes as proto messages with automatic retry on transient failures.
func (c *Client) ListNodeBalancerConfigNodesProto(ctx context.Context, nodeBalancerID, configID, page, pageSize int) ([]*linodev1.NodeBalancerConfigNode, error) {
	var nodes []*linodev1.NodeBalancerConfigNode

	err := c.executeWithRetry(ctx, "ListNodeBalancerConfigNodes", func() error {
		var retryErr error

		nodes, retryErr = c.httpListNodeBalancerConfigNodesProto(ctx, nodeBalancerID, configID, page, pageSize)

		return retryErr
	})

	return nodes, err
}

// GetNodeBalancerConfig retrieves one node balancer config by IDs with automatic retry on transient failures.
func (c *Client) GetNodeBalancerConfig(ctx context.Context, nodeBalancerID, configID int) (*NodeBalancerConfig, error) {
	var config *NodeBalancerConfig

	err := c.executeWithRetry(ctx, "GetNodeBalancerConfig", func() error {
		var err error

		config, err = c.httpGetNodeBalancerConfig(ctx, nodeBalancerID, configID)

		return err
	})

	return config, err
}

// GetNodeBalancerConfigProto retrieves one NodeBalancer config as a proto message
// with automatic retry on transient failures.
func (c *Client) GetNodeBalancerConfigProto(ctx context.Context, nodeBalancerID, configID int) (*linodev1.NodeBalancerConfig, error) {
	var config *linodev1.NodeBalancerConfig

	err := c.executeWithRetry(ctx, "GetNodeBalancerConfig", func() error {
		var err error

		config, err = c.httpGetNodeBalancerConfigProto(ctx, nodeBalancerID, configID)

		return err
	})

	return config, err
}

// GetNodeBalancerConfigNode retrieves one node for a node balancer config with automatic retry on transient failures.
func (c *Client) GetNodeBalancerConfigNode(ctx context.Context, nodeBalancerID, configID, nodeID int) (*NodeBalancerConfigNode, error) {
	var node *NodeBalancerConfigNode

	err := c.executeWithRetry(ctx, "GetNodeBalancerConfigNode", func() error {
		var retryErr error

		node, retryErr = c.httpGetNodeBalancerConfigNode(ctx, nodeBalancerID, configID, nodeID)

		return retryErr
	})

	return node, err
}

// GetNodeBalancerConfigNodeProto retrieves one NodeBalancer config node as a proto
// message with automatic retry on transient failures.
func (c *Client) GetNodeBalancerConfigNodeProto(ctx context.Context, nodeBalancerID, configID, nodeID int) (*linodev1.NodeBalancerConfigNode, error) {
	var node *linodev1.NodeBalancerConfigNode

	err := c.executeWithRetry(ctx, "GetNodeBalancerConfigNode", func() error {
		var retryErr error

		node, retryErr = c.httpGetNodeBalancerConfigNodeProto(ctx, nodeBalancerID, configID, nodeID)

		return retryErr
	})

	return node, err
}

// DeleteNodeBalancerConfigNode deletes one node from a node balancer config without retrying the destructive request.
// Retrying can replay node deletion after a transient error, so this method delegates exactly once.
func (c *Client) DeleteNodeBalancerConfigNode(ctx context.Context, nodeBalancerID, configID, nodeID int) error {
	return c.executeWithoutRetry(ctx, "DeleteNodeBalancerConfigNode", func() error {
		return c.httpDeleteNodeBalancerConfigNode(ctx, nodeBalancerID, configID, nodeID)
	})
}

// CreateNodeBalancerConfig creates a config for a node balancer by ID without retrying the POST create call.
func (c *Client) CreateNodeBalancerConfig(ctx context.Context, nodeBalancerID int, req *CreateNodeBalancerConfigRequest) (*NodeBalancerConfig, error) {
	var config *NodeBalancerConfig

	err := c.executeWithoutRetry(ctx, "CreateNodeBalancerConfig", func() error {
		var retryErr error

		config, retryErr = c.httpCreateNodeBalancerConfig(ctx, nodeBalancerID, req)

		return retryErr
	})

	return config, err
}

// CreateNodeBalancerNode creates a node for a node balancer config without retrying the POST create call.
func (c *Client) CreateNodeBalancerNode(ctx context.Context, nodeBalancerID, configID int, req *CreateNodeBalancerNodeRequest) (*NodeBalancerNode, error) {
	var node *NodeBalancerNode

	err := c.executeWithoutRetry(ctx, "CreateNodeBalancerNode", func() error {
		var retryErr error

		node, retryErr = c.httpCreateNodeBalancerNode(ctx, nodeBalancerID, configID, req)

		return retryErr
	})

	return node, err
}

// UpdateNodeBalancerConfig updates a node balancer config by ID without retrying the PUT update call.
func (c *Client) UpdateNodeBalancerConfig(ctx context.Context, nodeBalancerID, configID int, req *UpdateNodeBalancerConfigRequest) (*NodeBalancerConfig, error) {
	var config *NodeBalancerConfig

	err := c.executeWithoutRetry(ctx, "UpdateNodeBalancerConfig", func() error {
		var retryErr error

		config, retryErr = c.httpUpdateNodeBalancerConfig(ctx, nodeBalancerID, configID, req)

		return retryErr
	})

	return config, err
}

// RebuildNodeBalancerConfig rebuilds a node balancer config without retrying the POST rebuild call.
// Retrying can replay config rebuild after a transient error, so this method delegates exactly once.
func (c *Client) RebuildNodeBalancerConfig(ctx context.Context, nodeBalancerID, configID int) (*NodeBalancerConfig, error) {
	var config *NodeBalancerConfig

	err := c.executeWithoutRetry(ctx, "RebuildNodeBalancerConfig", func() error {
		var retryErr error

		config, retryErr = c.httpRebuildNodeBalancerConfig(ctx, nodeBalancerID, configID)

		return retryErr
	})

	return config, err
}

// DeleteNodeBalancerConfig deletes one node balancer config without retrying the destructive request.
// Retrying can replay config deletion after a transient error, so this method delegates exactly once.
func (c *Client) DeleteNodeBalancerConfig(ctx context.Context, nodeBalancerID, configID int) error {
	return c.executeWithoutRetry(ctx, "DeleteNodeBalancerConfig", func() error {
		return c.httpDeleteNodeBalancerConfig(ctx, nodeBalancerID, configID)
	})
}

// UpdateNodeBalancerNode updates a node for a node balancer config without retrying the PUT update call.
func (c *Client) UpdateNodeBalancerNode(ctx context.Context, nodeBalancerID, configID, nodeID int, req *UpdateNodeBalancerNodeRequest) (*NodeBalancerNode, error) {
	var node *NodeBalancerNode

	err := c.executeWithoutRetry(ctx, "UpdateNodeBalancerNode", func() error {
		var retryErr error

		node, retryErr = c.httpUpdateNodeBalancerNode(ctx, nodeBalancerID, configID, nodeID, req)

		return retryErr
	})

	return node, err
}

// CreateNodeBalancerConfigProto creates a node balancer config and decodes the
// response into the proto element without retrying the POST create call.
func (c *Client) CreateNodeBalancerConfigProto(ctx context.Context, nodeBalancerID int, req *CreateNodeBalancerConfigRequest) (*linodev1.NodeBalancerConfig, error) {
	var config *linodev1.NodeBalancerConfig

	err := c.executeWithoutRetry(ctx, "CreateNodeBalancerConfig", func() error {
		var retryErr error

		config, retryErr = c.httpCreateNodeBalancerConfigProto(ctx, nodeBalancerID, req)

		return retryErr
	})

	return config, err
}

// UpdateNodeBalancerConfigProto updates a node balancer config and decodes the
// response into the proto element without retrying the PUT update call.
func (c *Client) UpdateNodeBalancerConfigProto(ctx context.Context, nodeBalancerID, configID int, req *UpdateNodeBalancerConfigRequest) (*linodev1.NodeBalancerConfig, error) {
	var config *linodev1.NodeBalancerConfig

	err := c.executeWithoutRetry(ctx, "UpdateNodeBalancerConfig", func() error {
		var retryErr error

		config, retryErr = c.httpUpdateNodeBalancerConfigProto(ctx, nodeBalancerID, configID, req)

		return retryErr
	})

	return config, err
}

// RebuildNodeBalancerConfigProto rebuilds a node balancer config and decodes the
// response into the proto element without retrying the POST rebuild call.
// Retrying can replay config rebuild after a transient error, so this method delegates exactly once.
func (c *Client) RebuildNodeBalancerConfigProto(ctx context.Context, nodeBalancerID, configID int) (*linodev1.NodeBalancerConfig, error) {
	var config *linodev1.NodeBalancerConfig

	err := c.executeWithoutRetry(ctx, "RebuildNodeBalancerConfig", func() error {
		var retryErr error

		config, retryErr = c.httpRebuildNodeBalancerConfigProto(ctx, nodeBalancerID, configID)

		return retryErr
	})

	return config, err
}

// CreateNodeBalancerNodeProto creates a node balancer config node and decodes the
// response into the proto element without retrying the POST create call.
func (c *Client) CreateNodeBalancerNodeProto(ctx context.Context, nodeBalancerID, configID int, req *CreateNodeBalancerNodeRequest) (*linodev1.NodeBalancerConfigNode, error) {
	var node *linodev1.NodeBalancerConfigNode

	err := c.executeWithoutRetry(ctx, "CreateNodeBalancerNode", func() error {
		var retryErr error

		node, retryErr = c.httpCreateNodeBalancerNodeProto(ctx, nodeBalancerID, configID, req)

		return retryErr
	})

	return node, err
}

// UpdateNodeBalancerNodeProto updates a node balancer config node and decodes the
// response into the proto element without retrying the PUT update call.
func (c *Client) UpdateNodeBalancerNodeProto(ctx context.Context, nodeBalancerID, configID, nodeID int, req *UpdateNodeBalancerNodeRequest) (*linodev1.NodeBalancerConfigNode, error) {
	var node *linodev1.NodeBalancerConfigNode

	err := c.executeWithoutRetry(ctx, "UpdateNodeBalancerNode", func() error {
		var retryErr error

		node, retryErr = c.httpUpdateNodeBalancerNodeProto(ctx, nodeBalancerID, configID, nodeID, req)

		return retryErr
	})

	return node, err
}

// GetNodeBalancerStats retrieves node balancer statistics by ID with automatic retry on transient failures.
func (c *Client) GetNodeBalancerStats(ctx context.Context, nodeBalancerID int) (*NodeBalancerStats, error) {
	var stats *NodeBalancerStats

	err := c.executeWithRetry(ctx, "GetNodeBalancerStats", func() error {
		var err error

		stats, err = c.httpGetNodeBalancerStats(ctx, nodeBalancerID)

		return err
	})

	return stats, err
}

// ListStackScripts retrieves all stack scripts with automatic retry on transient failures.
func (c *Client) ListStackScripts(ctx context.Context) ([]StackScript, error) {
	var scripts []StackScript

	err := c.executeWithRetry(ctx, "ListStackScripts", func() error {
		var err error

		scripts, err = c.httpListStackScripts(ctx)

		return err
	})

	return scripts, err
}

// ListStackScriptsProto retrieves StackScripts as proto messages with automatic
// retry on transient failures.
func (c *Client) ListStackScriptsProto(ctx context.Context) ([]*linodev1.StackScript, error) {
	var scripts []*linodev1.StackScript

	err := c.executeWithRetry(ctx, "ListStackScripts", func() error {
		var retryErr error

		scripts, retryErr = c.httpListStackScriptsProto(ctx)

		return retryErr
	})

	return scripts, err
}

// GetStackScript retrieves one StackScript with automatic retry on transient failures.
func (c *Client) GetStackScript(ctx context.Context, stackScriptID int) (*StackScript, error) {
	var script *StackScript

	err := c.executeWithRetry(ctx, "GetStackScript", func() error {
		var err error

		script, err = c.httpGetStackScript(ctx, stackScriptID)

		return err
	})

	return script, err
}

// GetStackScriptProto retrieves one StackScript as a proto message with automatic
// retry on transient failures.
func (c *Client) GetStackScriptProto(ctx context.Context, stackScriptID int) (*linodev1.StackScript, error) {
	var script *linodev1.StackScript

	err := c.executeWithRetry(ctx, "GetStackScript", func() error {
		var err error

		script, err = c.httpGetStackScriptProto(ctx, stackScriptID)

		return err
	})

	return script, err
}

// CreateStackScript creates a new StackScript with automatic retry on transient failures.
func (c *Client) CreateStackScript(ctx context.Context, req *CreateStackScriptRequest) (*StackScript, error) {
	var script *StackScript

	err := c.executeWithoutRetry(ctx, "CreateStackScript", func() error {
		var err error

		script, err = c.httpCreateStackScript(ctx, req)

		return err
	})

	return script, err
}

// DeleteStackScript deletes a StackScript without retrying the DELETE call.
func (c *Client) DeleteStackScript(ctx context.Context, stackScriptID int) error {
	return c.executeWithoutRetry(ctx, "DeleteStackScript", func() error {
		return c.httpDeleteStackScript(ctx, stackScriptID)
	})
}

// UpdateStackScript updates editable fields on a StackScript without automatic retry.
// Replaying this mutating operation could repeat side effects after a transient failure.
func (c *Client) UpdateStackScript(ctx context.Context, stackScriptID int, req *UpdateStackScriptRequest) (*StackScript, error) {
	var script *StackScript

	err := c.executeWithoutRetry(ctx, "UpdateStackScript", func() error {
		var err error

		script, err = c.httpUpdateStackScript(ctx, stackScriptID, req)

		return err
	})

	return script, err
}

// GetFirewall retrieves a single firewall by ID with automatic retry on transient failures.
func (c *Client) GetFirewall(ctx context.Context, firewallID int) (*Firewall, error) {
	var firewall *Firewall

	err := c.executeWithRetry(ctx, "GetFirewall", func() error {
		var err error

		firewall, err = c.httpGetFirewall(ctx, firewallID)

		return err
	})

	return firewall, err
}

// GetFirewallProto retrieves one Cloud Firewall as a proto message with automatic
// retry on transient failures.
func (c *Client) GetFirewallProto(ctx context.Context, firewallID int) (*linodev1.Firewall, error) {
	var firewall *linodev1.Firewall

	err := c.executeWithRetry(ctx, "GetFirewall", func() error {
		var err error

		firewall, err = c.httpGetFirewallProto(ctx, firewallID)

		return err
	})

	return firewall, err
}

// GetVolume retrieves a single volume by ID with automatic retry on transient failures.
func (c *Client) GetVolume(ctx context.Context, volumeID int) (*Volume, error) {
	var volume *Volume

	err := c.executeWithRetry(ctx, "GetVolume", func() error {
		var err error

		volume, err = c.httpGetVolume(ctx, volumeID)

		return err
	})

	return volume, err
}

// GetVolumeProto retrieves a volume as a proto message with automatic retry on
// transient failures.
func (c *Client) GetVolumeProto(ctx context.Context, volumeID int) (*linodev1.Volume, error) {
	var volume *linodev1.Volume

	err := c.executeWithRetry(ctx, "GetVolume", func() error {
		var err error

		volume, err = c.httpGetVolumeProto(ctx, volumeID)

		return err
	})

	return volume, err
}

// GetSSHKey retrieves a single SSH key by ID with automatic retry on transient failures.
func (c *Client) GetSSHKey(ctx context.Context, sshKeyID int) (*SSHKey, error) {
	var sshKey *SSHKey

	err := c.executeWithRetry(ctx, "GetSSHKey", func() error {
		var err error

		sshKey, err = c.httpGetSSHKey(ctx, sshKeyID)

		return err
	})

	return sshKey, err
}

// GetSSHKeyProto retrieves one SSH key as a proto message with automatic retry
// on transient failures.
func (c *Client) GetSSHKeyProto(ctx context.Context, sshKeyID int) (*linodev1.SSHKey, error) {
	var sshKey *linodev1.SSHKey

	err := c.executeWithRetry(ctx, "GetSSHKey", func() error {
		var err error

		sshKey, err = c.httpGetSSHKeyProto(ctx, sshKeyID)

		return err
	})

	return sshKey, err
}

// CreateSSHKey creates a new SSH key with automatic retry on transient failures.
func (c *Client) CreateSSHKey(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error) {
	var sshKey *SSHKey

	err := c.executeWithRetry(ctx, "CreateSSHKey", func() error {
		var err error

		sshKey, err = c.httpCreateSSHKey(ctx, req)

		return err
	})

	return sshKey, err
}

// UpdateSSHKey updates an SSH key with automatic retry on transient failures.
func (c *Client) UpdateSSHKey(ctx context.Context, sshKeyID int, req UpdateSSHKeyRequest) (*SSHKey, error) {
	var sshKey *SSHKey

	err := c.executeWithRetry(ctx, "UpdateSSHKey", func() error {
		var err error

		sshKey, err = c.httpUpdateSSHKey(ctx, sshKeyID, req)

		return err
	})

	return sshKey, err
}

// CreateSSHKeyProto creates an SSH key and returns it as a proto message with
// automatic retry on transient failures.
func (c *Client) CreateSSHKeyProto(ctx context.Context, req CreateSSHKeyRequest) (*linodev1.SSHKey, error) {
	var sshKey *linodev1.SSHKey

	err := c.executeWithRetry(ctx, "CreateSSHKey", func() error {
		var err error

		sshKey, err = c.httpCreateSSHKeyProto(ctx, req)

		return err
	})

	return sshKey, err
}

// UpdateSSHKeyProto updates an SSH key and returns it as a proto message with
// automatic retry on transient failures.
func (c *Client) UpdateSSHKeyProto(ctx context.Context, sshKeyID int, req UpdateSSHKeyRequest) (*linodev1.SSHKey, error) {
	var sshKey *linodev1.SSHKey

	err := c.executeWithRetry(ctx, "UpdateSSHKey", func() error {
		var err error

		sshKey, err = c.httpUpdateSSHKeyProto(ctx, sshKeyID, req)

		return err
	})

	return sshKey, err
}

// DeleteSSHKey deletes an SSH key with automatic retry on transient failures.
func (c *Client) DeleteSSHKey(ctx context.Context, sshKeyID int) error {
	return c.executeWithRetry(ctx, "DeleteSSHKey", func() error {
		return c.httpDeleteSSHKey(ctx, sshKeyID)
	})
}

// BootInstance boots a Linode instance with automatic retry on transient failures.
func (c *Client) BootInstance(ctx context.Context, instanceID int, configID *int) error {
	return c.executeWithRetry(ctx, "BootInstance", func() error {
		return c.httpBootInstance(ctx, instanceID, configID)
	})
}

// RebootInstance reboots a Linode instance with automatic retry on transient failures.
func (c *Client) RebootInstance(ctx context.Context, instanceID int, configID *int) error {
	return c.executeWithRetry(ctx, "RebootInstance", func() error {
		return c.httpRebootInstance(ctx, instanceID, configID)
	})
}

// ShutdownInstance shuts down a Linode instance with automatic retry on transient failures.
func (c *Client) ShutdownInstance(ctx context.Context, instanceID int) error {
	return c.executeWithRetry(ctx, "ShutdownInstance", func() error {
		return c.httpShutdownInstance(ctx, instanceID)
	})
}

// CreateInstance creates a new Linode instance with automatic retry on transient failures.
func (c *Client) CreateInstance(ctx context.Context, req *CreateInstanceRequest) (*Instance, error) {
	var instance *Instance

	err := c.executeWithRetry(ctx, "CreateInstance", func() error {
		var err error

		instance, err = c.httpCreateInstance(ctx, req)

		return err
	})

	return instance, err
}

// CreateInstanceProto creates an instance as a proto message with automatic
// retry on transient failures.
func (c *Client) CreateInstanceProto(ctx context.Context, req *CreateInstanceRequest) (*linodev1.Instance, error) {
	var instance *linodev1.Instance

	err := c.executeWithRetry(ctx, "CreateInstance", func() error {
		var err error

		instance, err = c.httpCreateInstanceProto(ctx, req)

		return err
	})

	return instance, err
}

// DeleteInstance deletes a Linode instance with automatic retry on transient failures.
func (c *Client) DeleteInstance(ctx context.Context, instanceID int) error {
	return c.executeWithRetry(ctx, "DeleteInstance", func() error {
		return c.httpDeleteInstance(ctx, instanceID)
	})
}

// ResizeInstance resizes a Linode instance with automatic retry on transient failures.
func (c *Client) ResizeInstance(ctx context.Context, instanceID int, req ResizeInstanceRequest) error {
	return c.executeWithRetry(ctx, "ResizeInstance", func() error {
		return c.httpResizeInstance(ctx, instanceID, req)
	})
}

// UpdateInstance updates a Linode instance with automatic retry on transient failures.
func (c *Client) UpdateInstance(ctx context.Context, instanceID int, req *UpdateInstanceRequest) (*Instance, error) {
	var instance *Instance

	err := c.executeWithRetry(ctx, "UpdateInstance", func() error {
		var err error

		instance, err = c.httpUpdateInstance(ctx, instanceID, req)

		return err
	})

	return instance, err
}

// UpdateInstanceProto updates an instance as a proto message with automatic
// retry on transient failures.
func (c *Client) UpdateInstanceProto(ctx context.Context, instanceID int, req *UpdateInstanceRequest) (*linodev1.Instance, error) {
	var instance *linodev1.Instance

	err := c.executeWithRetry(ctx, "UpdateInstance", func() error {
		var err error

		instance, err = c.httpUpdateInstanceProto(ctx, instanceID, req)

		return err
	})

	return instance, err
}

// CreateFirewall creates a new firewall with automatic retry on transient failures.
func (c *Client) CreateFirewall(ctx context.Context, req CreateFirewallRequest) (*Firewall, error) {
	var firewall *Firewall

	err := c.executeWithRetry(ctx, "CreateFirewall", func() error {
		var err error

		firewall, err = c.httpCreateFirewall(ctx, req)

		return err
	})

	return firewall, err
}

// UpdateFirewall updates a firewall with automatic retry on transient failures.
func (c *Client) UpdateFirewall(ctx context.Context, firewallID int, req UpdateFirewallRequest) (*Firewall, error) {
	var firewall *Firewall

	err := c.executeWithRetry(ctx, "UpdateFirewall", func() error {
		var err error

		firewall, err = c.httpUpdateFirewall(ctx, firewallID, req)

		return err
	})

	return firewall, err
}

// DeleteFirewall deletes a firewall with automatic retry on transient failures.
func (c *Client) DeleteFirewall(ctx context.Context, firewallID int) error {
	return c.executeWithRetry(ctx, "DeleteFirewall", func() error {
		return c.httpDeleteFirewall(ctx, firewallID)
	})
}

// CreateFirewallProto creates a firewall and decodes the response into the
// Firewall proto element with automatic retry on transient failures.
func (c *Client) CreateFirewallProto(ctx context.Context, req CreateFirewallRequest) (*linodev1.Firewall, error) {
	var firewall *linodev1.Firewall

	err := c.executeWithRetry(ctx, "CreateFirewall", func() error {
		var err error

		firewall, err = c.httpCreateFirewallProto(ctx, req)

		return err
	})

	return firewall, err
}

// UpdateFirewallProto updates a firewall and decodes the response into the
// Firewall proto element with automatic retry on transient failures.
func (c *Client) UpdateFirewallProto(ctx context.Context, firewallID int, req UpdateFirewallRequest) (*linodev1.Firewall, error) {
	var firewall *linodev1.Firewall

	err := c.executeWithRetry(ctx, "UpdateFirewall", func() error {
		var err error

		firewall, err = c.httpUpdateFirewallProto(ctx, firewallID, req)

		return err
	})

	return firewall, err
}

// UpdateFirewallRulesProto replaces a firewall's rules and decodes the response
// into the FirewallRules proto element without retrying the PUT update call.
func (c *Client) UpdateFirewallRulesProto(ctx context.Context, firewallID int, req *FirewallRules) (*linodev1.FirewallRules, error) {
	var rules *linodev1.FirewallRules

	err := c.executeWithoutRetry(ctx, "UpdateFirewallRules", func() error {
		var err error

		rules, err = c.httpUpdateFirewallRulesProto(ctx, firewallID, req)

		return err
	})

	return rules, err
}

// ImportDomain imports a domain zone without retrying this non-idempotent create operation.
func (c *Client) ImportDomain(ctx context.Context, req *ImportDomainRequest) (*Domain, error) {
	return c.httpImportDomain(ctx, req)
}

// ImportDomainProto imports a domain as a proto message without automatic retry.
func (c *Client) ImportDomainProto(ctx context.Context, req *ImportDomainRequest) (*linodev1.Domain, error) {
	return c.httpImportDomainProto(ctx, req)
}

// CloneDomain clones a domain without retrying this non-idempotent create operation.
func (c *Client) CloneDomain(ctx context.Context, domainID int, req *CloneDomainRequest) (*Domain, error) {
	return c.httpCloneDomain(ctx, domainID, req)
}

// CloneDomainProto clones a domain as a proto message without automatic retry.
func (c *Client) CloneDomainProto(ctx context.Context, domainID int, req *CloneDomainRequest) (*linodev1.Domain, error) {
	return c.httpCloneDomainProto(ctx, domainID, req)
}

// CreateTag creates a tag without retrying the mutating request.
// Retrying can create duplicate tag creations after a transient error, so this
// method delegates exactly once.
func (c *Client) CreateTag(ctx context.Context, req *CreateTagRequest) (*Tag, error) {
	return c.httpCreateTag(ctx, req)
}

// CreateTagProto creates a tag as a proto message without automatic retry.
func (c *Client) CreateTagProto(ctx context.Context, req *CreateTagRequest) (*linodev1.Tag, error) {
	return c.httpCreateTagProto(ctx, req)
}

// CreateDomain creates a new domain with automatic retry on transient failures.
func (c *Client) CreateDomain(ctx context.Context, req *CreateDomainRequest) (*Domain, error) {
	var domain *Domain

	err := c.executeWithRetry(ctx, "CreateDomain", func() error {
		var err error

		domain, err = c.httpCreateDomain(ctx, req)

		return err
	})

	return domain, err
}

// CreateDomainProto creates a domain as a proto message with automatic retry on
// transient failures.
func (c *Client) CreateDomainProto(ctx context.Context, req *CreateDomainRequest) (*linodev1.Domain, error) {
	var domain *linodev1.Domain

	err := c.executeWithRetry(ctx, "CreateDomain", func() error {
		var err error

		domain, err = c.httpCreateDomainProto(ctx, req)

		return err
	})

	return domain, err
}

// UpdateDomain updates a domain with automatic retry on transient failures.
func (c *Client) UpdateDomain(ctx context.Context, domainID int, req *UpdateDomainRequest) (*Domain, error) {
	var domain *Domain

	err := c.executeWithRetry(ctx, "UpdateDomain", func() error {
		var err error

		domain, err = c.httpUpdateDomain(ctx, domainID, req)

		return err
	})

	return domain, err
}

// UpdateDomainProto updates a domain as a proto message with automatic retry on
// transient failures.
func (c *Client) UpdateDomainProto(ctx context.Context, domainID int, req *UpdateDomainRequest) (*linodev1.Domain, error) {
	var domain *linodev1.Domain

	err := c.executeWithRetry(ctx, "UpdateDomain", func() error {
		var err error

		domain, err = c.httpUpdateDomainProto(ctx, domainID, req)

		return err
	})

	return domain, err
}

// DeleteDomain deletes a domain with automatic retry on transient failures.
func (c *Client) DeleteDomain(ctx context.Context, domainID int) error {
	return c.executeWithRetry(ctx, "DeleteDomain", func() error {
		return c.httpDeleteDomain(ctx, domainID)
	})
}

// GetDomainRecord gets a domain record with automatic retry on transient failures.
func (c *Client) GetDomainRecord(ctx context.Context, domainID, recordID int) (*DomainRecord, error) {
	var record *DomainRecord

	err := c.executeWithRetry(ctx, "GetDomainRecord", func() error {
		var err error

		record, err = c.httpGetDomainRecord(ctx, domainID, recordID)

		return err
	})

	return record, err
}

// GetDomainRecordProto retrieves a domain record as a proto message with automatic
// retry on transient failures.
func (c *Client) GetDomainRecordProto(ctx context.Context, domainID, recordID int) (*linodev1.DomainRecord, error) {
	var record *linodev1.DomainRecord

	err := c.executeWithRetry(ctx, "GetDomainRecord", func() error {
		var err error

		record, err = c.httpGetDomainRecordProto(ctx, domainID, recordID)

		return err
	})

	return record, err
}

// CreateDomainRecord creates a domain record with automatic retry on transient failures.
func (c *Client) CreateDomainRecord(ctx context.Context, domainID int, req *CreateDomainRecordRequest) (*DomainRecord, error) {
	var record *DomainRecord

	err := c.executeWithRetry(ctx, "CreateDomainRecord", func() error {
		var err error

		record, err = c.httpCreateDomainRecord(ctx, domainID, req)

		return err
	})

	return record, err
}

// CreateDomainRecordProto creates a domain record as a proto message with automatic
// retry on transient failures.
func (c *Client) CreateDomainRecordProto(ctx context.Context, domainID int, req *CreateDomainRecordRequest) (*linodev1.DomainRecord, error) {
	var record *linodev1.DomainRecord

	err := c.executeWithRetry(ctx, "CreateDomainRecord", func() error {
		var err error

		record, err = c.httpCreateDomainRecordProto(ctx, domainID, req)

		return err
	})

	return record, err
}

// UpdateDomainRecord updates a domain record with automatic retry on transient failures.
func (c *Client) UpdateDomainRecord(ctx context.Context, domainID, recordID int, req *UpdateDomainRecordRequest) (*DomainRecord, error) {
	var record *DomainRecord

	err := c.executeWithRetry(ctx, "UpdateDomainRecord", func() error {
		var err error

		record, err = c.httpUpdateDomainRecord(ctx, domainID, recordID, req)

		return err
	})

	return record, err
}

// UpdateDomainRecordProto updates a domain record as a proto message with automatic
// retry on transient failures.
func (c *Client) UpdateDomainRecordProto(ctx context.Context, domainID, recordID int, req *UpdateDomainRecordRequest) (*linodev1.DomainRecord, error) {
	var record *linodev1.DomainRecord

	err := c.executeWithRetry(ctx, "UpdateDomainRecord", func() error {
		var err error

		record, err = c.httpUpdateDomainRecordProto(ctx, domainID, recordID, req)

		return err
	})

	return record, err
}

// DeleteDomainRecord deletes a domain record with automatic retry on transient failures.
func (c *Client) DeleteDomainRecord(ctx context.Context, domainID, recordID int) error {
	return c.executeWithRetry(ctx, "DeleteDomainRecord", func() error {
		return c.httpDeleteDomainRecord(ctx, domainID, recordID)
	})
}

// ListVolumeTypes retrieves all volume types with automatic retry on transient failures.
func (c *Client) ListVolumeTypes(ctx context.Context) ([]VolumeType, error) {
	var types []VolumeType

	err := c.executeWithRetry(ctx, "ListVolumeTypes", func() error {
		var err error

		types, err = c.httpListVolumeTypes(ctx)

		return err
	})

	return types, err
}

// ListVolumeTypesProto retrieves all block storage volume types as proto
// messages with automatic retry on transient failures.
func (c *Client) ListVolumeTypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	var types []*linodev1.LinodeType

	err := c.executeWithRetry(ctx, "ListVolumeTypes", func() error {
		var retryErr error

		types, retryErr = c.httpListVolumeTypesProto(ctx)

		return retryErr
	})

	return types, err
}

// CreateVolume creates a new volume with automatic retry on transient failures.
func (c *Client) CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*Volume, error) {
	var volume *Volume

	err := c.executeWithRetry(ctx, "CreateVolume", func() error {
		var err error

		volume, err = c.httpCreateVolume(ctx, req)

		return err
	})

	return volume, err
}

// CreateVolumeProto creates a volume as a proto message with automatic retry on
// transient failures.
func (c *Client) CreateVolumeProto(ctx context.Context, req *CreateVolumeRequest) (*linodev1.Volume, error) {
	var volume *linodev1.Volume

	err := c.executeWithRetry(ctx, "CreateVolume", func() error {
		var err error

		volume, err = c.httpCreateVolumeProto(ctx, req)

		return err
	})

	return volume, err
}

// CloneVolume clones an existing volume without automatic retry to avoid replaying resource creation.
func (c *Client) CloneVolume(ctx context.Context, volumeID int, req CloneVolumeRequest) (*Volume, error) {
	// CloneVolume intentionally bypasses executeWithRetry because this POST creates a
	// new volume; replaying a transient failure can create duplicate volumes.
	return c.httpCloneVolume(ctx, volumeID, req)
}

// CloneVolumeProto clones a volume as a proto message. Like CloneVolume it
// bypasses retry because the POST creates a new volume.
func (c *Client) CloneVolumeProto(ctx context.Context, volumeID int, req CloneVolumeRequest) (*linodev1.Volume, error) {
	return c.httpCloneVolumeProto(ctx, volumeID, req)
}

// AttachVolume attaches a volume to a Linode with automatic retry on transient failures.
func (c *Client) AttachVolume(ctx context.Context, volumeID int, req AttachVolumeRequest) (*Volume, error) {
	var volume *Volume

	err := c.executeWithRetry(ctx, "AttachVolume", func() error {
		var err error

		volume, err = c.httpAttachVolume(ctx, volumeID, req)

		return err
	})

	return volume, err
}

// AttachVolumeProto attaches a volume as a proto message with automatic retry on
// transient failures.
func (c *Client) AttachVolumeProto(ctx context.Context, volumeID int, req AttachVolumeRequest) (*linodev1.Volume, error) {
	var volume *linodev1.Volume

	err := c.executeWithRetry(ctx, "AttachVolume", func() error {
		var err error

		volume, err = c.httpAttachVolumeProto(ctx, volumeID, req)

		return err
	})

	return volume, err
}

// DetachVolume detaches a volume from a Linode with automatic retry on transient failures.
func (c *Client) DetachVolume(ctx context.Context, volumeID int) error {
	return c.executeWithRetry(ctx, "DetachVolume", func() error {
		return c.httpDetachVolume(ctx, volumeID)
	})
}

// ResizeVolume resizes a volume with automatic retry on transient failures.
func (c *Client) ResizeVolume(ctx context.Context, volumeID, size int) (*Volume, error) {
	var volume *Volume

	err := c.executeWithRetry(ctx, "ResizeVolume", func() error {
		var err error

		volume, err = c.httpResizeVolume(ctx, volumeID, size)

		return err
	})

	return volume, err
}

// ResizeVolumeProto resizes a volume as a proto message with automatic retry on
// transient failures.
func (c *Client) ResizeVolumeProto(ctx context.Context, volumeID, size int) (*linodev1.Volume, error) {
	var volume *linodev1.Volume

	err := c.executeWithRetry(ctx, "ResizeVolume", func() error {
		var err error

		volume, err = c.httpResizeVolumeProto(ctx, volumeID, size)

		return err
	})

	return volume, err
}

// DeleteVolume deletes a volume with automatic retry on transient failures.
func (c *Client) DeleteVolume(ctx context.Context, volumeID int) error {
	return c.executeWithRetry(ctx, "DeleteVolume", func() error {
		return c.httpDeleteVolume(ctx, volumeID)
	})
}

// UpdateVolume updates a volume's label or tags with automatic retry on transient failures.
func (c *Client) UpdateVolume(ctx context.Context, volumeID int, req *UpdateVolumeRequest) (*Volume, error) {
	var volume *Volume

	err := c.executeWithRetry(ctx, "UpdateVolume", func() error {
		var err error

		volume, err = c.httpUpdateVolume(ctx, volumeID, req)

		return err
	})

	return volume, err
}

// UpdateVolumeProto updates a volume as a proto message with automatic retry on
// transient failures.
func (c *Client) UpdateVolumeProto(ctx context.Context, volumeID int, req *UpdateVolumeRequest) (*linodev1.Volume, error) {
	var volume *linodev1.Volume

	err := c.executeWithRetry(ctx, "UpdateVolume", func() error {
		var err error

		volume, err = c.httpUpdateVolumeProto(ctx, volumeID, req)

		return err
	})

	return volume, err
}

// CreateNodeBalancer creates a new NodeBalancer with automatic retry on transient failures.
func (c *Client) CreateNodeBalancer(ctx context.Context, req CreateNodeBalancerRequest) (*NodeBalancer, error) {
	var nodeBalancer *NodeBalancer

	err := c.executeWithRetry(ctx, "CreateNodeBalancer", func() error {
		var err error

		nodeBalancer, err = c.httpCreateNodeBalancer(ctx, req)

		return err
	})

	return nodeBalancer, err
}

// CreateNodeBalancerProto creates a NodeBalancer as a proto message with automatic
// retry on transient failures.
func (c *Client) CreateNodeBalancerProto(ctx context.Context, req CreateNodeBalancerRequest) (*linodev1.NodeBalancer, error) {
	var nodeBalancer *linodev1.NodeBalancer

	err := c.executeWithRetry(ctx, "CreateNodeBalancer", func() error {
		var err error

		nodeBalancer, err = c.httpCreateNodeBalancerProto(ctx, req)

		return err
	})

	return nodeBalancer, err
}

// UpdateNodeBalancer updates a NodeBalancer with automatic retry on transient failures.
func (c *Client) UpdateNodeBalancer(ctx context.Context, nodeBalancerID int, req UpdateNodeBalancerRequest) (*NodeBalancer, error) {
	var nodeBalancer *NodeBalancer

	err := c.executeWithRetry(ctx, "UpdateNodeBalancer", func() error {
		var err error

		nodeBalancer, err = c.httpUpdateNodeBalancer(ctx, nodeBalancerID, req)

		return err
	})

	return nodeBalancer, err
}

// UpdateNodeBalancerProto updates a NodeBalancer as a proto message with automatic
// retry on transient failures.
func (c *Client) UpdateNodeBalancerProto(ctx context.Context, nodeBalancerID int, req UpdateNodeBalancerRequest) (*linodev1.NodeBalancer, error) {
	var nodeBalancer *linodev1.NodeBalancer

	err := c.executeWithRetry(ctx, "UpdateNodeBalancer", func() error {
		var err error

		nodeBalancer, err = c.httpUpdateNodeBalancerProto(ctx, nodeBalancerID, req)

		return err
	})

	return nodeBalancer, err
}

// DeleteNodeBalancer deletes a NodeBalancer with automatic retry on transient failures.
func (c *Client) DeleteNodeBalancer(ctx context.Context, nodeBalancerID int) error {
	return c.executeWithRetry(ctx, "DeleteNodeBalancer", func() error {
		return c.httpDeleteNodeBalancer(ctx, nodeBalancerID)
	})
}

// ListObjectStorageBuckets retrieves all Object Storage buckets with automatic retry.
func (c *Client) ListObjectStorageBuckets(ctx context.Context) ([]ObjectStorageBucket, error) {
	var buckets []ObjectStorageBucket

	err := c.executeWithRetry(ctx, "ListObjectStorageBuckets", func() error {
		var err error

		buckets, err = c.httpListObjectStorageBuckets(ctx)

		return err
	})

	return buckets, err
}

// ListObjectStorageBucketsProto retrieves all Object Storage buckets as proto
// messages with automatic retry on transient failures.
func (c *Client) ListObjectStorageBucketsProto(ctx context.Context) ([]*linodev1.ObjectStorageBucket, error) {
	var buckets []*linodev1.ObjectStorageBucket

	err := c.executeWithRetry(ctx, "ListObjectStorageBuckets", func() error {
		var retryErr error

		buckets, retryErr = c.httpListObjectStorageBucketsProto(ctx)

		return retryErr
	})

	return buckets, err
}

// ListObjectStorageBucketsByRegion retrieves Object Storage buckets in a region with automatic retry.
func (c *Client) ListObjectStorageBucketsByRegion(ctx context.Context, region string) ([]ObjectStorageBucket, error) {
	var buckets []ObjectStorageBucket

	err := c.executeWithRetry(ctx, "ListObjectStorageBucketsByRegion", func() error {
		var err error

		buckets, err = c.httpListObjectStorageBucketsByRegion(ctx, region)

		return err
	})

	return buckets, err
}

// GetObjectStorageBucket retrieves a specific bucket with automatic retry.
func (c *Client) GetObjectStorageBucket(ctx context.Context, region, label string) (*ObjectStorageBucket, error) {
	var bucket *ObjectStorageBucket

	err := c.executeWithRetry(ctx, "GetObjectStorageBucket", func() error {
		var err error

		bucket, err = c.httpGetObjectStorageBucket(ctx, region, label)

		return err
	})

	return bucket, err
}

// GetObjectStorageBucketProto retrieves an Object Storage bucket as a proto message
// with automatic retry on transient failures.
func (c *Client) GetObjectStorageBucketProto(ctx context.Context, region, label string) (*linodev1.ObjectStorageBucket, error) {
	var bucket *linodev1.ObjectStorageBucket

	err := c.executeWithRetry(ctx, "GetObjectStorageBucket", func() error {
		var err error

		bucket, err = c.httpGetObjectStorageBucketProto(ctx, region, label)

		return err
	})

	return bucket, err
}

// ListObjectStorageBucketContentsProto lists objects in a bucket as proto
// messages with automatic retry, returning the elements plus the S3 pagination
// metadata.
func (c *Client) ListObjectStorageBucketContentsProto(ctx context.Context, region, label string, params map[string]string) (*ObjectStorageBucketContentsPage, error) {
	var page *ObjectStorageBucketContentsPage

	err := c.executeWithRetry(ctx, "ListObjectStorageBucketContents", func() error {
		var retryErr error

		page, retryErr = c.httpListObjectStorageBucketContentsProto(ctx, region, label, params)

		return retryErr
	})

	return page, err
}

// ListObjectStorageEndpoints retrieves Object Storage endpoints with automatic retry.
func (c *Client) ListObjectStorageEndpoints(ctx context.Context) ([]ObjectStorageEndpoint, error) {
	var endpoints []ObjectStorageEndpoint

	err := c.executeWithRetry(ctx, "ListObjectStorageEndpoints", func() error {
		var err error

		endpoints, err = c.httpListObjectStorageEndpoints(ctx)

		return err
	})

	return endpoints, err
}

// ListFirewallTemplatesProto retrieves reusable Cloud Firewall templates as proto
// FirewallTemplate messages with automatic retry on transient failures.
func (c *Client) ListFirewallTemplatesProto(ctx context.Context, page, pageSize int) ([]*linodev1.FirewallTemplate, error) {
	var templates []*linodev1.FirewallTemplate

	err := c.executeWithRetry(ctx, "ListFirewallTemplates", func() error {
		var retryErr error

		templates, retryErr = c.httpListFirewallTemplatesProto(ctx, page, pageSize)

		return retryErr
	})

	return templates, err
}

// ListIPv6PoolsProto retrieves IPv6 pools as proto IPv6Pool messages with
// automatic retry on transient failures.
func (c *Client) ListIPv6PoolsProto(ctx context.Context, page, pageSize int) ([]*linodev1.IPv6Pool, error) {
	var pools []*linodev1.IPv6Pool

	err := c.executeWithRetry(ctx, "ListIPv6Pools", func() error {
		var retryErr error

		pools, retryErr = c.httpListIPv6PoolsProto(ctx, page, pageSize)

		return retryErr
	})

	return pools, err
}

// ListIPv6RangesProto retrieves IPv6 ranges as proto IPv6Range messages with
// automatic retry on transient failures.
func (c *Client) ListIPv6RangesProto(ctx context.Context, page, pageSize int) ([]*linodev1.IPv6Range, error) {
	var ranges []*linodev1.IPv6Range

	err := c.executeWithRetry(ctx, "ListIPv6Ranges", func() error {
		var retryErr error

		ranges, retryErr = c.httpListIPv6RangesProto(ctx, page, pageSize)

		return retryErr
	})

	return ranges, err
}

// ListNetworkTransferPricesProto retrieves network transfer prices as proto
// LinodeType messages with automatic retry on transient failures.
func (c *Client) ListNetworkTransferPricesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	var prices []*linodev1.LinodeType

	err := c.executeWithRetry(ctx, "ListNetworkTransferPrices", func() error {
		var retryErr error

		prices, retryErr = c.httpListNetworkTransferPricesProto(ctx)

		return retryErr
	})

	return prices, err
}

// ListObjectStorageEndpointsProto retrieves Object Storage endpoints as proto
// ObjectStorageEndpoint messages with automatic retry on transient failures.
func (c *Client) ListObjectStorageEndpointsProto(ctx context.Context) ([]*linodev1.ObjectStorageEndpoint, error) {
	var endpoints []*linodev1.ObjectStorageEndpoint

	err := c.executeWithRetry(ctx, "ListObjectStorageEndpoints", func() error {
		var err error

		endpoints, err = c.httpListObjectStorageEndpointsProto(ctx)

		return err
	})

	return endpoints, err
}

// ListObjectStorageTypes retrieves Object Storage types with automatic retry.
func (c *Client) ListObjectStorageTypes(ctx context.Context) ([]ObjectStorageType, error) {
	var types []ObjectStorageType

	err := c.executeWithRetry(ctx, "ListObjectStorageTypes", func() error {
		var err error

		types, err = c.httpListObjectStorageTypes(ctx)

		return err
	})

	return types, err
}

// ListObjectStorageTypesProto retrieves Object Storage types as proto LinodeType
// messages with automatic retry on transient failures.
func (c *Client) ListObjectStorageTypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	var types []*linodev1.LinodeType

	err := c.executeWithRetry(ctx, "ListObjectStorageTypes", func() error {
		var err error

		types, err = c.httpListObjectStorageTypesProto(ctx)

		return err
	})

	return types, err
}

// ListObjectStorageQuotas retrieves Object Storage quotas with automatic retry.
func (c *Client) ListObjectStorageQuotas(ctx context.Context) ([]ObjectStorageQuota, error) {
	var quotas []ObjectStorageQuota

	err := c.executeWithRetry(ctx, "ListObjectStorageQuotas", func() error {
		var err error

		quotas, err = c.httpListObjectStorageQuotas(ctx)

		return err
	})

	return quotas, err
}

// ListObjectStorageQuotasProto retrieves Object Storage quotas as proto
// ObjectStorageQuota messages with automatic retry on transient failures.
func (c *Client) ListObjectStorageQuotasProto(ctx context.Context) ([]*linodev1.ObjectStorageQuota, error) {
	var quotas []*linodev1.ObjectStorageQuota

	err := c.executeWithRetry(ctx, "ListObjectStorageQuotas", func() error {
		var err error

		quotas, err = c.httpListObjectStorageQuotasProto(ctx)

		return err
	})

	return quotas, err
}

// ListObjectStorageKeys retrieves all Object Storage access keys with automatic retry.
func (c *Client) ListObjectStorageKeys(ctx context.Context) ([]ObjectStorageKey, error) {
	var keys []ObjectStorageKey

	err := c.executeWithRetry(ctx, "ListObjectStorageKeys", func() error {
		var err error

		keys, err = c.httpListObjectStorageKeys(ctx)

		return err
	})

	return keys, err
}

// ListObjectStorageKeysProto retrieves Object Storage keys as proto messages with
// automatic retry on transient failures.
func (c *Client) ListObjectStorageKeysProto(ctx context.Context) ([]*linodev1.ObjectStorageKey, error) {
	var keys []*linodev1.ObjectStorageKey

	err := c.executeWithRetry(ctx, "ListObjectStorageKeys", func() error {
		var retryErr error

		keys, retryErr = c.httpListObjectStorageKeysProto(ctx)

		return retryErr
	})

	return keys, err
}

// GetObjectStorageKey retrieves a specific access key with automatic retry.
func (c *Client) GetObjectStorageKey(ctx context.Context, keyID int) (*ObjectStorageKey, error) {
	var key *ObjectStorageKey

	err := c.executeWithRetry(ctx, "GetObjectStorageKey", func() error {
		var err error

		key, err = c.httpGetObjectStorageKey(ctx, keyID)

		return err
	})

	return key, err
}

// GetObjectStorageKeyProto retrieves an Object Storage key as a proto message with
// automatic retry on transient failures.
func (c *Client) GetObjectStorageKeyProto(ctx context.Context, keyID int) (*linodev1.ObjectStorageKey, error) {
	var key *linodev1.ObjectStorageKey

	err := c.executeWithRetry(ctx, "GetObjectStorageKey", func() error {
		var err error

		key, err = c.httpGetObjectStorageKeyProto(ctx, keyID)

		return err
	})

	return key, err
}

// GetObjectStorageQuotaUsage retrieves Object Storage quota usage with automatic retry.
func (c *Client) GetObjectStorageQuotaUsage(ctx context.Context, quotaID string) (*ObjectStorageQuotaUsage, error) {
	var usage *ObjectStorageQuotaUsage

	err := c.executeWithRetry(ctx, "GetObjectStorageQuotaUsage", func() error {
		var err error

		usage, err = c.httpGetObjectStorageQuotaUsage(ctx, quotaID)

		return err
	})

	return usage, err
}

// GetObjectStorageTransfer retrieves Object Storage transfer usage with automatic retry.
func (c *Client) GetObjectStorageTransfer(ctx context.Context) (*ObjectStorageTransfer, error) {
	var transfer *ObjectStorageTransfer

	err := c.executeWithRetry(ctx, "GetObjectStorageTransfer", func() error {
		var err error

		transfer, err = c.httpGetObjectStorageTransfer(ctx)

		return err
	})

	return transfer, err
}

// GetObjectStorageQuota retrieves a single Object Storage quota with automatic retry.
func (c *Client) GetObjectStorageQuota(ctx context.Context, objQuotaID string) (*ObjectStorageQuota, error) {
	var quota *ObjectStorageQuota

	err := c.executeWithRetry(ctx, "GetObjectStorageQuota", func() error {
		var err error

		quota, err = c.httpGetObjectStorageQuota(ctx, objQuotaID)

		return err
	})

	return quota, err
}

// CancelObjectStorage cancels Object Storage service without retrying the state-changing request.
func (c *Client) CancelObjectStorage(ctx context.Context) error {
	return c.httpCancelObjectStorage(ctx)
}

// GetObjectStorageBucketAccess retrieves bucket ACL/CORS settings with automatic retry.
func (c *Client) GetObjectStorageBucketAccess(ctx context.Context, region, label string) (*ObjectStorageBucketAccess, error) {
	var access *ObjectStorageBucketAccess

	err := c.executeWithRetry(ctx, "GetObjectStorageBucketAccess", func() error {
		var err error

		access, err = c.httpGetObjectStorageBucketAccess(ctx, region, label)

		return err
	})

	return access, err
}

// GetObjectStorageBucketAccessProto retrieves a bucket's access config as a proto
// message with automatic retry on transient failures.
func (c *Client) GetObjectStorageBucketAccessProto(ctx context.Context, region, label string) (*linodev1.ObjectStorageBucketAccess, error) {
	var access *linodev1.ObjectStorageBucketAccess

	err := c.executeWithRetry(ctx, "GetObjectStorageBucketAccess", func() error {
		var err error

		access, err = c.httpGetObjectStorageBucketAccessProto(ctx, region, label)

		return err
	})

	return access, err
}

// CreateObjectStorageBucket creates a new Object Storage bucket with automatic retry.
func (c *Client) CreateObjectStorageBucket(ctx context.Context, req CreateObjectStorageBucketRequest) (*ObjectStorageBucket, error) {
	var bucket *ObjectStorageBucket

	err := c.executeWithRetry(ctx, "CreateObjectStorageBucket", func() error {
		var err error

		bucket, err = c.httpCreateObjectStorageBucket(ctx, req)

		return err
	})

	return bucket, err
}

// CreateObjectStorageBucketProto creates an Object Storage bucket as a proto message
// with automatic retry on transient failures.
func (c *Client) CreateObjectStorageBucketProto(ctx context.Context, req CreateObjectStorageBucketRequest) (*linodev1.ObjectStorageBucket, error) {
	var bucket *linodev1.ObjectStorageBucket

	err := c.executeWithRetry(ctx, "CreateObjectStorageBucket", func() error {
		var err error

		bucket, err = c.httpCreateObjectStorageBucketProto(ctx, req)

		return err
	})

	return bucket, err
}

// DeleteObjectStorageBucket deletes an Object Storage bucket with automatic retry.
func (c *Client) DeleteObjectStorageBucket(ctx context.Context, region, label string) error {
	return c.executeWithRetry(ctx, "DeleteObjectStorageBucket", func() error {
		return c.httpDeleteObjectStorageBucket(ctx, region, label)
	})
}

// UpdateObjectStorageBucketAccess updates bucket access settings with automatic retry.
func (c *Client) UpdateObjectStorageBucketAccess(ctx context.Context, region, label string, req UpdateObjectStorageBucketAccessRequest) error {
	return c.executeWithRetry(ctx, "UpdateObjectStorageBucketAccess", func() error {
		return c.httpUpdateObjectStorageBucketAccess(ctx, region, label, req)
	})
}

// AllowObjectStorageBucketAccess applies bucket access settings without retrying the state-changing request.
func (c *Client) AllowObjectStorageBucketAccess(ctx context.Context, region, label string, req AllowObjectStorageBucketAccessRequest) error {
	return c.httpAllowObjectStorageBucketAccess(ctx, region, label, req)
}

// CreateObjectStorageKey creates a new Object Storage access key with automatic retry.
func (c *Client) CreateObjectStorageKey(ctx context.Context, req CreateObjectStorageKeyRequest) (*ObjectStorageKey, error) {
	var key *ObjectStorageKey

	err := c.executeWithRetry(ctx, "CreateObjectStorageKey", func() error {
		var err error

		key, err = c.httpCreateObjectStorageKey(ctx, req)

		return err
	})

	return key, err
}

// CreateObjectStorageKeyProto creates an Object Storage key as a proto message with
// automatic retry on transient failures.
func (c *Client) CreateObjectStorageKeyProto(ctx context.Context, req CreateObjectStorageKeyRequest) (*linodev1.ObjectStorageKey, error) {
	var key *linodev1.ObjectStorageKey

	err := c.executeWithRetry(ctx, "CreateObjectStorageKey", func() error {
		var err error

		key, err = c.httpCreateObjectStorageKeyProto(ctx, req)

		return err
	})

	return key, err
}

// UpdateObjectStorageKey updates an Object Storage access key with automatic retry.
func (c *Client) UpdateObjectStorageKey(ctx context.Context, keyID int, req UpdateObjectStorageKeyRequest) error {
	return c.executeWithRetry(ctx, "UpdateObjectStorageKey", func() error {
		return c.httpUpdateObjectStorageKey(ctx, keyID, req)
	})
}

// DeleteObjectStorageKey revokes an Object Storage access key with automatic retry.
func (c *Client) DeleteObjectStorageKey(ctx context.Context, keyID int) error {
	return c.executeWithRetry(ctx, "DeleteObjectStorageKey", func() error {
		return c.httpDeleteObjectStorageKey(ctx, keyID)
	})
}

// CreatePresignedURL generates a presigned URL with automatic retry.
func (c *Client) CreatePresignedURL(ctx context.Context, region, label string, req PresignedURLRequest) (*PresignedURLResponse, error) {
	var result *PresignedURLResponse

	err := c.executeWithRetry(ctx, "CreatePresignedURL", func() error {
		var retryErr error

		result, retryErr = c.httpCreatePresignedURL(ctx, region, label, req)

		return retryErr
	})

	return result, err
}

// GetObjectACL retrieves an object's ACL with automatic retry.
func (c *Client) GetObjectACL(ctx context.Context, region, label, name string) (*ObjectACL, error) {
	var result *ObjectACL

	err := c.executeWithRetry(ctx, "GetObjectACL", func() error {
		var retryErr error

		result, retryErr = c.httpGetObjectACL(ctx, region, label, name)

		return retryErr
	})

	return result, err
}

// GetObjectACLProto retrieves an object's ACL as a proto message with automatic
// retry on transient failures.
func (c *Client) GetObjectACLProto(ctx context.Context, region, label, name string) (*linodev1.ObjectACL, error) {
	var result *linodev1.ObjectACL

	err := c.executeWithRetry(ctx, "GetObjectACL", func() error {
		var retryErr error

		result, retryErr = c.httpGetObjectACLProto(ctx, region, label, name)

		return retryErr
	})

	return result, err
}

// UpdateObjectACL updates an object's ACL with automatic retry.
func (c *Client) UpdateObjectACL(ctx context.Context, region, label string, req ObjectACLUpdateRequest) (*ObjectACL, error) {
	var result *ObjectACL

	err := c.executeWithRetry(ctx, "UpdateObjectACL", func() error {
		var retryErr error

		result, retryErr = c.httpUpdateObjectACL(ctx, region, label, req)

		return retryErr
	})

	return result, err
}

// GetBucketSSL retrieves a bucket's SSL status with automatic retry.
func (c *Client) GetBucketSSL(ctx context.Context, region, label string) (*BucketSSL, error) {
	var result *BucketSSL

	err := c.executeWithRetry(ctx, "GetBucketSSL", func() error {
		var retryErr error

		result, retryErr = c.httpGetBucketSSL(ctx, region, label)

		return retryErr
	})

	return result, err
}

// GetBucketSSLProto retrieves a bucket's TLS status as a proto message with
// automatic retry on transient failures.
func (c *Client) GetBucketSSLProto(ctx context.Context, region, label string) (*linodev1.BucketSSL, error) {
	var result *linodev1.BucketSSL

	err := c.executeWithRetry(ctx, "GetBucketSSL", func() error {
		var retryErr error

		result, retryErr = c.httpGetBucketSSLProto(ctx, region, label)

		return retryErr
	})

	return result, err
}

// DeleteBucketSSL removes a bucket's SSL certificate with automatic retry.
func (c *Client) DeleteBucketSSL(ctx context.Context, region, label string) error {
	return c.executeWithRetry(ctx, "DeleteBucketSSL", func() error {
		return c.httpDeleteBucketSSL(ctx, region, label)
	})
}

// UploadBucketSSL uploads an SSL/TLS certificate to a bucket with automatic retry.
func (c *Client) UploadBucketSSL(ctx context.Context, region, label string, req UploadBucketSSLRequest) (*BucketSSL, error) {
	var result *BucketSSL

	err := c.executeWithRetry(ctx, "UploadBucketSSL", func() error {
		var retryErr error

		result, retryErr = c.httpUploadBucketSSL(ctx, region, label, req)

		return retryErr
	})

	return result, err
}

// LKE (Kubernetes Engine) operations

// ListLKEClusters retrieves all LKE clusters with automatic retry on transient failures.
func (c *Client) ListLKEClusters(ctx context.Context) ([]LKECluster, error) {
	var clusters []LKECluster

	err := c.executeWithRetry(ctx, "ListLKEClusters", func() error {
		var err error

		clusters, err = c.httpListLKEClusters(ctx)

		return err
	})

	return clusters, err
}

// ListLKEClustersProto retrieves all LKE clusters as proto messages with
// automatic retry on transient failures.
func (c *Client) ListLKEClustersProto(ctx context.Context) ([]*linodev1.LKECluster, error) {
	var clusters []*linodev1.LKECluster

	err := c.executeWithRetry(ctx, "ListLKEClusters", func() error {
		var err error

		clusters, err = c.httpListLKEClustersProto(ctx)

		return err
	})

	return clusters, err
}

// GetLKECluster retrieves a single LKE cluster by ID with automatic retry on transient failures.
func (c *Client) GetLKECluster(ctx context.Context, clusterID int) (*LKECluster, error) {
	var cluster *LKECluster

	err := c.executeWithRetry(ctx, "GetLKECluster", func() error {
		var err error

		cluster, err = c.httpGetLKECluster(ctx, clusterID)

		return err
	})

	return cluster, err
}

// GetLKEClusterProto retrieves an LKE cluster as a proto message with automatic
// retry on transient failures.
func (c *Client) GetLKEClusterProto(ctx context.Context, clusterID int) (*linodev1.LKECluster, error) {
	var cluster *linodev1.LKECluster

	err := c.executeWithRetry(ctx, "GetLKECluster", func() error {
		var err error

		cluster, err = c.httpGetLKEClusterProto(ctx, clusterID)

		return err
	})

	return cluster, err
}

// CreateLKECluster creates a new LKE cluster with automatic retry on transient failures.
func (c *Client) CreateLKECluster(ctx context.Context, req *CreateLKEClusterRequest) (*LKECluster, error) {
	var cluster *LKECluster

	err := c.executeWithRetry(ctx, "CreateLKECluster", func() error {
		var err error

		cluster, err = c.httpCreateLKECluster(ctx, req)

		return err
	})

	return cluster, err
}

// CreateLKEClusterProto creates an LKE cluster as a proto message with automatic
// retry on transient failures.
func (c *Client) CreateLKEClusterProto(ctx context.Context, req *CreateLKEClusterRequest) (*linodev1.LKECluster, error) {
	var cluster *linodev1.LKECluster

	err := c.executeWithRetry(ctx, "CreateLKECluster", func() error {
		var err error

		cluster, err = c.httpCreateLKEClusterProto(ctx, req)

		return err
	})

	return cluster, err
}

// UpdateLKECluster updates an LKE cluster with automatic retry on transient failures.
func (c *Client) UpdateLKECluster(ctx context.Context, clusterID int, req UpdateLKEClusterRequest) (*LKECluster, error) {
	var cluster *LKECluster

	err := c.executeWithRetry(ctx, "UpdateLKECluster", func() error {
		var err error

		cluster, err = c.httpUpdateLKECluster(ctx, clusterID, req)

		return err
	})

	return cluster, err
}

// UpdateLKEClusterProto updates an LKE cluster as a proto message with automatic
// retry on transient failures.
func (c *Client) UpdateLKEClusterProto(ctx context.Context, clusterID int, req UpdateLKEClusterRequest) (*linodev1.LKECluster, error) {
	var cluster *linodev1.LKECluster

	err := c.executeWithRetry(ctx, "UpdateLKECluster", func() error {
		var err error

		cluster, err = c.httpUpdateLKEClusterProto(ctx, clusterID, req)

		return err
	})

	return cluster, err
}

// DeleteLKECluster deletes an LKE cluster with automatic retry on transient failures.
func (c *Client) DeleteLKECluster(ctx context.Context, clusterID int) error {
	return c.executeWithRetry(ctx, "DeleteLKECluster", func() error {
		return c.httpDeleteLKECluster(ctx, clusterID)
	})
}

// RecycleLKECluster recycles all nodes in an LKE cluster with automatic retry on transient failures.
func (c *Client) RecycleLKECluster(ctx context.Context, clusterID int) error {
	return c.executeWithRetry(ctx, "RecycleLKECluster", func() error {
		return c.httpRecycleLKECluster(ctx, clusterID)
	})
}

// RegenerateLKECluster regenerates the service token for an LKE cluster with automatic retry on transient failures.
func (c *Client) RegenerateLKECluster(ctx context.Context, clusterID int) error {
	return c.executeWithRetry(ctx, "RegenerateLKECluster", func() error {
		return c.httpRegenerateLKECluster(ctx, clusterID)
	})
}

// ListLKENodePools retrieves all node pools for an LKE cluster with automatic retry on transient failures.
func (c *Client) ListLKENodePools(ctx context.Context, clusterID int) ([]LKENodePool, error) {
	var pools []LKENodePool

	err := c.executeWithRetry(ctx, "ListLKENodePools", func() error {
		var err error

		pools, err = c.httpListLKENodePools(ctx, clusterID)

		return err
	})

	return pools, err
}

// ListLKENodePoolsProto retrieves an LKE cluster's node pools as proto messages
// with automatic retry on transient failures.
func (c *Client) ListLKENodePoolsProto(ctx context.Context, clusterID int) ([]*linodev1.LKENodePool, error) {
	var pools []*linodev1.LKENodePool

	err := c.executeWithRetry(ctx, "ListLKENodePools", func() error {
		var retryErr error

		pools, retryErr = c.httpListLKENodePoolsProto(ctx, clusterID)

		return retryErr
	})

	return pools, err
}

// ListLKEAPIEndpointsProto retrieves an LKE cluster's API endpoints as proto
// messages with automatic retry on transient failures.
func (c *Client) ListLKEAPIEndpointsProto(ctx context.Context, clusterID int) ([]*linodev1.LKEAPIEndpoint, error) {
	var endpoints []*linodev1.LKEAPIEndpoint

	err := c.executeWithRetry(ctx, "ListLKEAPIEndpoints", func() error {
		var retryErr error

		endpoints, retryErr = c.httpListLKEAPIEndpointsProto(ctx, clusterID)

		return retryErr
	})

	return endpoints, err
}

// GetLKENodePool retrieves a single node pool by ID with automatic retry on transient failures.
func (c *Client) GetLKENodePool(ctx context.Context, clusterID, poolID int) (*LKENodePool, error) {
	var pool *LKENodePool

	err := c.executeWithRetry(ctx, "GetLKENodePool", func() error {
		var err error

		pool, err = c.httpGetLKENodePool(ctx, clusterID, poolID)

		return err
	})

	return pool, err
}

// GetLKENodePoolProto retrieves one LKE node pool as a proto message with
// automatic retry on transient failures.
func (c *Client) GetLKENodePoolProto(ctx context.Context, clusterID, poolID int) (*linodev1.LKENodePool, error) {
	var pool *linodev1.LKENodePool

	err := c.executeWithRetry(ctx, "GetLKENodePool", func() error {
		var err error

		pool, err = c.httpGetLKENodePoolProto(ctx, clusterID, poolID)

		return err
	})

	return pool, err
}

// CreateLKENodePoolProto creates a node pool and decodes the response into the
// proto element with automatic retry on transient failures.
func (c *Client) CreateLKENodePoolProto(ctx context.Context, clusterID int, req *CreateLKENodePoolRequest) (*linodev1.LKENodePool, error) {
	var pool *linodev1.LKENodePool

	err := c.executeWithRetry(ctx, "CreateLKENodePool", func() error {
		var retryErr error

		pool, retryErr = c.httpCreateLKENodePoolProto(ctx, clusterID, req)

		return retryErr
	})

	return pool, err
}

// UpdateLKENodePoolProto updates a node pool and decodes the response into the
// proto element with automatic retry on transient failures.
func (c *Client) UpdateLKENodePoolProto(ctx context.Context, clusterID, poolID int, req UpdateLKENodePoolRequest) (*linodev1.LKENodePool, error) {
	var pool *linodev1.LKENodePool

	err := c.executeWithRetry(ctx, "UpdateLKENodePool", func() error {
		var retryErr error

		pool, retryErr = c.httpUpdateLKENodePoolProto(ctx, clusterID, poolID, req)

		return retryErr
	})

	return pool, err
}

// DeleteLKENodePool deletes a node pool with automatic retry on transient failures.
func (c *Client) DeleteLKENodePool(ctx context.Context, clusterID, poolID int) error {
	return c.executeWithRetry(ctx, "DeleteLKENodePool", func() error {
		return c.httpDeleteLKENodePool(ctx, clusterID, poolID)
	})
}

// RecycleLKENodePool recycles all nodes in a node pool with automatic retry on transient failures.
func (c *Client) RecycleLKENodePool(ctx context.Context, clusterID, poolID int) error {
	return c.executeWithRetry(ctx, "RecycleLKENodePool", func() error {
		return c.httpRecycleLKENodePool(ctx, clusterID, poolID)
	})
}

// GetLKENode retrieves a single node by ID with automatic retry on transient failures.
func (c *Client) GetLKENode(ctx context.Context, clusterID int, nodeID string) (*LKENode, error) {
	var node *LKENode

	err := c.executeWithRetry(ctx, "GetLKENode", func() error {
		var err error

		node, err = c.httpGetLKENode(ctx, clusterID, nodeID)

		return err
	})

	return node, err
}

// GetLKENodeProto retrieves one LKE cluster node as a proto message with automatic
// retry on transient failures.
func (c *Client) GetLKENodeProto(ctx context.Context, clusterID int, nodeID string) (*linodev1.LKENode, error) {
	var node *linodev1.LKENode

	err := c.executeWithRetry(ctx, "GetLKENode", func() error {
		var err error

		node, err = c.httpGetLKENodeProto(ctx, clusterID, nodeID)

		return err
	})

	return node, err
}

// DeleteLKENode deletes a node with automatic retry on transient failures.
func (c *Client) DeleteLKENode(ctx context.Context, clusterID int, nodeID string) error {
	return c.executeWithRetry(ctx, "DeleteLKENode", func() error {
		return c.httpDeleteLKENode(ctx, clusterID, nodeID)
	})
}

// RecycleLKENode recycles a specific node with automatic retry on transient failures.
func (c *Client) RecycleLKENode(ctx context.Context, clusterID int, nodeID string) error {
	return c.executeWithRetry(ctx, "RecycleLKENode", func() error {
		return c.httpRecycleLKENode(ctx, clusterID, nodeID)
	})
}

// GetLKEKubeconfig retrieves the kubeconfig for an LKE cluster with automatic retry on transient failures.
func (c *Client) GetLKEKubeconfig(ctx context.Context, clusterID int) (*LKEKubeconfig, error) {
	var kubeconfig *LKEKubeconfig

	err := c.executeWithRetry(ctx, "GetLKEKubeconfig", func() error {
		var err error

		kubeconfig, err = c.httpGetLKEKubeconfig(ctx, clusterID)

		return err
	})

	return kubeconfig, err
}

// GetLKEKubeconfigProto retrieves an LKE cluster kubeconfig as a proto message
// with automatic retry on transient failures.
func (c *Client) GetLKEKubeconfigProto(ctx context.Context, clusterID int) (*linodev1.LKEKubeconfig, error) {
	var kubeconfig *linodev1.LKEKubeconfig

	err := c.executeWithRetry(ctx, "GetLKEKubeconfig", func() error {
		var err error

		kubeconfig, err = c.httpGetLKEKubeconfigProto(ctx, clusterID)

		return err
	})

	return kubeconfig, err
}

// DeleteLKEKubeconfig deletes the kubeconfig for an LKE cluster with automatic retry on transient failures.
func (c *Client) DeleteLKEKubeconfig(ctx context.Context, clusterID int) error {
	return c.executeWithRetry(ctx, "DeleteLKEKubeconfig", func() error {
		return c.httpDeleteLKEKubeconfig(ctx, clusterID)
	})
}

// GetLKEDashboard retrieves the dashboard URL for an LKE cluster with automatic retry on transient failures.
func (c *Client) GetLKEDashboard(ctx context.Context, clusterID int) (*LKEDashboard, error) {
	var dashboard *LKEDashboard

	err := c.executeWithRetry(ctx, "GetLKEDashboard", func() error {
		var err error

		dashboard, err = c.httpGetLKEDashboard(ctx, clusterID)

		return err
	})

	return dashboard, err
}

// GetLKEDashboardProto retrieves the LKE dashboard URL as a proto message with
// automatic retry on transient failures.
func (c *Client) GetLKEDashboardProto(ctx context.Context, clusterID int) (*linodev1.LKEDashboard, error) {
	var dashboard *linodev1.LKEDashboard

	err := c.executeWithRetry(ctx, "GetLKEDashboard", func() error {
		var err error

		dashboard, err = c.httpGetLKEDashboardProto(ctx, clusterID)

		return err
	})

	return dashboard, err
}

// ListLKEAPIEndpoints retrieves API endpoints for an LKE cluster with automatic retry on transient failures.
func (c *Client) ListLKEAPIEndpoints(ctx context.Context, clusterID int) ([]LKEAPIEndpoint, error) {
	var endpoints []LKEAPIEndpoint

	err := c.executeWithRetry(ctx, "ListLKEAPIEndpoints", func() error {
		var err error

		endpoints, err = c.httpListLKEAPIEndpoints(ctx, clusterID)

		return err
	})

	return endpoints, err
}

// DeleteLKEServiceToken deletes the service token for an LKE cluster without retrying the DELETE call.
func (c *Client) DeleteLKEServiceToken(ctx context.Context, clusterID int) error {
	return c.executeWithoutRetry(ctx, "DeleteLKEServiceToken", func() error {
		return c.httpDeleteLKEServiceToken(ctx, clusterID)
	})
}

// GetLKEControlPlaneACL retrieves the control plane ACL with automatic retry on transient failures.
func (c *Client) GetLKEControlPlaneACL(ctx context.Context, clusterID int) (*LKEControlPlaneACL, error) {
	var acl *LKEControlPlaneACL

	err := c.executeWithRetry(ctx, "GetLKEControlPlaneACL", func() error {
		var err error

		acl, err = c.httpGetLKEControlPlaneACL(ctx, clusterID)

		return err
	})

	return acl, err
}

// UpdateLKEControlPlaneACL updates the control plane ACL with automatic retry on transient failures.
func (c *Client) UpdateLKEControlPlaneACL(ctx context.Context, clusterID int, req UpdateLKEControlPlaneACLRequest) (*LKEControlPlaneACL, error) {
	var acl *LKEControlPlaneACL

	err := c.executeWithRetry(ctx, "UpdateLKEControlPlaneACL", func() error {
		var err error

		acl, err = c.httpUpdateLKEControlPlaneACL(ctx, clusterID, req)

		return err
	})

	return acl, err
}

// DeleteLKEControlPlaneACL deletes the control plane ACL without retrying the destructive request.
func (c *Client) DeleteLKEControlPlaneACL(ctx context.Context, clusterID int) error {
	return c.executeWithoutRetry(ctx, "DeleteLKEControlPlaneACL", func() error {
		return c.httpDeleteLKEControlPlaneACL(ctx, clusterID)
	})
}

// ListLKEVersions retrieves all LKE versions with automatic retry on transient failures.
func (c *Client) ListLKEVersions(ctx context.Context) ([]LKEVersion, error) {
	var versions []LKEVersion

	err := c.executeWithRetry(ctx, "ListLKEVersions", func() error {
		var err error

		versions, err = c.httpListLKEVersions(ctx)

		return err
	})

	return versions, err
}

// ListLKEVersionsProto retrieves all available Kubernetes versions as proto
// messages with automatic retry on transient failures.
func (c *Client) ListLKEVersionsProto(ctx context.Context) ([]*linodev1.LKEVersion, error) {
	var versions []*linodev1.LKEVersion

	err := c.executeWithRetry(ctx, "ListLKEVersions", func() error {
		var retryErr error

		versions, retryErr = c.httpListLKEVersionsProto(ctx)

		return retryErr
	})

	return versions, err
}

// GetLKEVersion retrieves a specific LKE version with automatic retry on transient failures.
func (c *Client) GetLKEVersion(ctx context.Context, versionID string) (*LKEVersion, error) {
	var version *LKEVersion

	err := c.executeWithRetry(ctx, "GetLKEVersion", func() error {
		var err error

		version, err = c.httpGetLKEVersion(ctx, versionID)

		return err
	})

	return version, err
}

// GetLKEVersionProto retrieves one LKE Kubernetes version as a proto message with
// automatic retry on transient failures.
func (c *Client) GetLKEVersionProto(ctx context.Context, versionID string) (*linodev1.LKEVersion, error) {
	var version *linodev1.LKEVersion

	err := c.executeWithRetry(ctx, "GetLKEVersion", func() error {
		var err error

		version, err = c.httpGetLKEVersionProto(ctx, versionID)

		return err
	})

	return version, err
}

// ListLKETypes retrieves all LKE types with automatic retry on transient failures.
func (c *Client) ListLKETypes(ctx context.Context) ([]LKEType, error) {
	var types []LKEType

	err := c.executeWithRetry(ctx, "ListLKETypes", func() error {
		var err error

		types, err = c.httpListLKETypes(ctx)

		return err
	})

	return types, err
}

// ListLKETypesProto retrieves all LKE node types as proto messages with
// automatic retry on transient failures.
func (c *Client) ListLKETypesProto(ctx context.Context) ([]*linodev1.LinodeType, error) {
	var types []*linodev1.LinodeType

	err := c.executeWithRetry(ctx, "ListLKETypes", func() error {
		var retryErr error

		types, retryErr = c.httpListLKETypesProto(ctx)

		return retryErr
	})

	return types, err
}

// ListLKETierVersions retrieves LKE tier versions with automatic retry on transient failures.
func (c *Client) ListLKETierVersions(ctx context.Context, tier string) ([]LKETierVersion, error) {
	var versions []LKETierVersion

	err := c.executeWithRetry(ctx, "ListLKETierVersions", func() error {
		var err error

		versions, err = c.httpListLKETierVersions(ctx, tier)

		return err
	})

	return versions, err
}

// ListLKETierVersionsProto retrieves LKE tier versions for a tier as proto
// messages with automatic retry on transient failures.
func (c *Client) ListLKETierVersionsProto(ctx context.Context, tier string) ([]*linodev1.LKETierVersion, error) {
	var versions []*linodev1.LKETierVersion

	err := c.executeWithRetry(ctx, "ListLKETierVersions", func() error {
		var err error

		versions, err = c.httpListLKETierVersionsProto(ctx, tier)

		return err
	})

	return versions, err
}

// GetLKETierVersion retrieves a specific LKE tier version with automatic retry on transient failures.
func (c *Client) GetLKETierVersion(ctx context.Context, tierID, versionID string) (*LKETierVersion, error) {
	var version *LKETierVersion

	err := c.executeWithRetry(ctx, "GetLKETierVersion", func() error {
		var err error

		version, err = c.httpGetLKETierVersion(ctx, tierID, versionID)

		return err
	})

	return version, err
}

// GetLKETierVersionProto retrieves one LKE tier Kubernetes version as a proto
// message with automatic retry on transient failures.
func (c *Client) GetLKETierVersionProto(ctx context.Context, tierID, versionID string) (*linodev1.LKETierVersion, error) {
	var version *linodev1.LKETierVersion

	err := c.executeWithRetry(ctx, "GetLKETierVersion", func() error {
		var err error

		version, err = c.httpGetLKETierVersionProto(ctx, tierID, versionID)

		return err
	})

	return version, err
}

// VPC operations

// ListVPCs retrieves all VPCs with automatic retry on transient failures.
func (c *Client) ListVPCs(ctx context.Context) ([]VPC, error) {
	var vpcs []VPC

	err := c.executeWithRetry(ctx, "ListVPCs", func() error {
		var retryErr error

		vpcs, retryErr = c.httpListVPCs(ctx)

		return retryErr
	})

	return vpcs, err
}

// ListVPCsProto retrieves all VPCs as proto messages with automatic retry on
// transient failures.
func (c *Client) ListVPCsProto(ctx context.Context) ([]*linodev1.Vpc, error) {
	var vpcs []*linodev1.Vpc

	err := c.executeWithRetry(ctx, "ListVPCs", func() error {
		var retryErr error

		vpcs, retryErr = c.httpListVPCsProto(ctx)

		return retryErr
	})

	return vpcs, err
}

// GetVPC retrieves a single VPC by ID with automatic retry on transient failures.
func (c *Client) GetVPC(ctx context.Context, vpcID int) (*VPC, error) {
	var vpc *VPC

	err := c.executeWithRetry(ctx, "GetVPC", func() error {
		var retryErr error

		vpc, retryErr = c.httpGetVPC(ctx, vpcID)

		return retryErr
	})

	return vpc, err
}

// GetVPCProto retrieves a VPC as a proto message with automatic retry on
// transient failures.
func (c *Client) GetVPCProto(ctx context.Context, vpcID int) (*linodev1.Vpc, error) {
	var vpc *linodev1.Vpc

	err := c.executeWithRetry(ctx, "GetVPC", func() error {
		var retryErr error

		vpc, retryErr = c.httpGetVPCProto(ctx, vpcID)

		return retryErr
	})

	return vpc, err
}

// GetPlacementGroup retrieves a single placement group by ID with automatic retry on transient failures.
func (c *Client) GetPlacementGroup(ctx context.Context, groupID int) (*PlacementGroup, error) {
	var group *PlacementGroup

	err := c.executeWithRetry(ctx, "GetPlacementGroup", func() error {
		var retryErr error

		group, retryErr = c.httpGetPlacementGroup(ctx, groupID)

		return retryErr
	})

	return group, err
}

// GetPlacementGroupProto retrieves one placement group as a proto message with
// automatic retry on transient failures.
func (c *Client) GetPlacementGroupProto(ctx context.Context, groupID int) (*linodev1.PlacementGroup, error) {
	var group *linodev1.PlacementGroup

	err := c.executeWithRetry(ctx, "GetPlacementGroup", func() error {
		var retryErr error

		group, retryErr = c.httpGetPlacementGroupProto(ctx, groupID)

		return retryErr
	})

	return group, err
}

// DeletePlacementGroup deletes a placement group by ID without automatic retry.
// Replaying this destructive operation could repeat side effects after a transient failure.
func (c *Client) DeletePlacementGroup(ctx context.Context, groupID int) error {
	return c.httpDeletePlacementGroup(ctx, groupID)
}

// CreateVPC creates a new VPC with automatic retry on transient failures.
func (c *Client) CreateVPC(ctx context.Context, req CreateVPCRequest) (*VPC, error) {
	var vpc *VPC

	err := c.executeWithRetry(ctx, "CreateVPC", func() error {
		var retryErr error

		vpc, retryErr = c.httpCreateVPC(ctx, req)

		return retryErr
	})

	return vpc, err
}

// CreateVPCProto creates a VPC as a proto message with automatic retry on
// transient failures.
func (c *Client) CreateVPCProto(ctx context.Context, req CreateVPCRequest) (*linodev1.Vpc, error) {
	var vpc *linodev1.Vpc

	err := c.executeWithRetry(ctx, "CreateVPC", func() error {
		var retryErr error

		vpc, retryErr = c.httpCreateVPCProto(ctx, req)

		return retryErr
	})

	return vpc, err
}

// UpdateVPC updates a VPC with automatic retry on transient failures.
func (c *Client) UpdateVPC(ctx context.Context, vpcID int, req UpdateVPCRequest) (*VPC, error) {
	var vpc *VPC

	err := c.executeWithRetry(ctx, "UpdateVPC", func() error {
		var retryErr error

		vpc, retryErr = c.httpUpdateVPC(ctx, vpcID, req)

		return retryErr
	})

	return vpc, err
}

// UpdateVPCProto updates a VPC as a proto message with automatic retry on
// transient failures.
func (c *Client) UpdateVPCProto(ctx context.Context, vpcID int, req UpdateVPCRequest) (*linodev1.Vpc, error) {
	var vpc *linodev1.Vpc

	err := c.executeWithRetry(ctx, "UpdateVPC", func() error {
		var retryErr error

		vpc, retryErr = c.httpUpdateVPCProto(ctx, vpcID, req)

		return retryErr
	})

	return vpc, err
}

// DeleteVPC deletes a VPC with automatic retry on transient failures.
func (c *Client) DeleteVPC(ctx context.Context, vpcID int) error {
	return c.executeWithRetry(ctx, "DeleteVPC", func() error {
		return c.httpDeleteVPC(ctx, vpcID)
	})
}

// ListVPCIPsProto retrieves all VPC IP addresses as proto messages with
// automatic retry on transient failures.
func (c *Client) ListVPCIPsProto(ctx context.Context) ([]*linodev1.VPCIP, error) {
	var ips []*linodev1.VPCIP

	err := c.executeWithRetry(ctx, "ListVPCIPs", func() error {
		var retryErr error

		ips, retryErr = c.httpListVPCIPsProto(ctx)

		return retryErr
	})

	return ips, err
}

// ListVPCIPAddressesProto retrieves a VPC's IP addresses as proto messages with
// automatic retry on transient failures.
func (c *Client) ListVPCIPAddressesProto(ctx context.Context, vpcID int) ([]*linodev1.VPCIP, error) {
	var ips []*linodev1.VPCIP

	err := c.executeWithRetry(ctx, "ListVPCIPAddresses", func() error {
		var retryErr error

		ips, retryErr = c.httpListVPCIPAddressesProto(ctx, vpcID)

		return retryErr
	})

	return ips, err
}

// ListVPCSubnets retrieves all subnets for a VPC with automatic retry on transient failures.
func (c *Client) ListVPCSubnets(ctx context.Context, vpcID int) ([]VPCSubnet, error) {
	var subnets []VPCSubnet

	err := c.executeWithRetry(ctx, "ListVPCSubnets", func() error {
		var retryErr error

		subnets, retryErr = c.httpListVPCSubnets(ctx, vpcID)

		return retryErr
	})

	return subnets, err
}

// ListVPCSubnetsProto retrieves a VPC's subnets as proto messages with automatic
// retry on transient failures.
func (c *Client) ListVPCSubnetsProto(ctx context.Context, vpcID int) ([]*linodev1.VpcSubnet, error) {
	var subnets []*linodev1.VpcSubnet

	err := c.executeWithRetry(ctx, "ListVPCSubnets", func() error {
		var retryErr error

		subnets, retryErr = c.httpListVPCSubnetsProto(ctx, vpcID)

		return retryErr
	})

	return subnets, err
}

// GetVPCSubnet retrieves a single subnet by ID with automatic retry on transient failures.
func (c *Client) GetVPCSubnet(ctx context.Context, vpcID, subnetID int) (*VPCSubnet, error) {
	var subnet *VPCSubnet

	err := c.executeWithRetry(ctx, "GetVPCSubnet", func() error {
		var retryErr error

		subnet, retryErr = c.httpGetVPCSubnet(ctx, vpcID, subnetID)

		return retryErr
	})

	return subnet, err
}

// GetVPCSubnetProto retrieves a subnet as a proto message with automatic retry on
// transient failures.
func (c *Client) GetVPCSubnetProto(ctx context.Context, vpcID, subnetID int) (*linodev1.VpcSubnet, error) {
	var subnet *linodev1.VpcSubnet

	err := c.executeWithRetry(ctx, "GetVPCSubnet", func() error {
		var retryErr error

		subnet, retryErr = c.httpGetVPCSubnetProto(ctx, vpcID, subnetID)

		return retryErr
	})

	return subnet, err
}

// CreateVPCSubnet creates a new subnet in a VPC with automatic retry on transient failures.
func (c *Client) CreateVPCSubnet(ctx context.Context, vpcID int, req CreateSubnetRequest) (*VPCSubnet, error) {
	var subnet *VPCSubnet

	err := c.executeWithRetry(ctx, "CreateVPCSubnet", func() error {
		var retryErr error

		subnet, retryErr = c.httpCreateVPCSubnet(ctx, vpcID, req)

		return retryErr
	})

	return subnet, err
}

// CreateVPCSubnetProto creates a subnet as a proto message with automatic retry on
// transient failures.
func (c *Client) CreateVPCSubnetProto(ctx context.Context, vpcID int, req CreateSubnetRequest) (*linodev1.VpcSubnet, error) {
	var subnet *linodev1.VpcSubnet

	err := c.executeWithRetry(ctx, "CreateVPCSubnet", func() error {
		var retryErr error

		subnet, retryErr = c.httpCreateVPCSubnetProto(ctx, vpcID, req)

		return retryErr
	})

	return subnet, err
}

// UpdateVPCSubnet updates a subnet in a VPC with automatic retry on transient failures.
func (c *Client) UpdateVPCSubnet(ctx context.Context, vpcID, subnetID int, req UpdateSubnetRequest) (*VPCSubnet, error) {
	var subnet *VPCSubnet

	err := c.executeWithRetry(ctx, "UpdateVPCSubnet", func() error {
		var retryErr error

		subnet, retryErr = c.httpUpdateVPCSubnet(ctx, vpcID, subnetID, req)

		return retryErr
	})

	return subnet, err
}

// UpdateVPCSubnetProto updates a subnet as a proto message with automatic retry on
// transient failures.
func (c *Client) UpdateVPCSubnetProto(ctx context.Context, vpcID, subnetID int, req UpdateSubnetRequest) (*linodev1.VpcSubnet, error) {
	var subnet *linodev1.VpcSubnet

	err := c.executeWithRetry(ctx, "UpdateVPCSubnet", func() error {
		var retryErr error

		subnet, retryErr = c.httpUpdateVPCSubnetProto(ctx, vpcID, subnetID, req)

		return retryErr
	})

	return subnet, err
}

// DeleteVPCSubnet deletes a subnet from a VPC with automatic retry on transient failures.
func (c *Client) DeleteVPCSubnet(ctx context.Context, vpcID, subnetID int) error {
	return c.executeWithRetry(ctx, "DeleteVPCSubnet", func() error {
		return c.httpDeleteVPCSubnet(ctx, vpcID, subnetID)
	})
}

// Instance deep operations

// ListInstanceBackups retrieves all backups for an instance with automatic retry on transient failures.
func (c *Client) ListInstanceBackups(ctx context.Context, linodeID int) (*InstanceBackupsResponse, error) {
	var backups *InstanceBackupsResponse

	err := c.executeWithRetry(ctx, "ListInstanceBackups", func() error {
		var retryErr error

		backups, retryErr = c.httpListInstanceBackups(ctx, linodeID)

		return retryErr
	})

	return backups, err
}

// ListInstanceBackupsProto retrieves all backups for an instance as a proto
// message with automatic retry on transient failures.
func (c *Client) ListInstanceBackupsProto(ctx context.Context, linodeID int) (*linodev1.InstanceBackupsResponse, error) {
	var backups *linodev1.InstanceBackupsResponse

	err := c.executeWithRetry(ctx, "ListInstanceBackups", func() error {
		var retryErr error

		backups, retryErr = c.httpListInstanceBackupsProto(ctx, linodeID)

		return retryErr
	})

	return backups, err
}

// GetInstanceStats retrieves daily statistics for an instance with automatic retry on transient failures.
func (c *Client) GetInstanceStats(ctx context.Context, linodeID int) (*InstanceStats, error) {
	var stats *InstanceStats

	err := c.executeWithRetry(ctx, "GetInstanceStats", func() error {
		var retryErr error

		stats, retryErr = c.httpGetInstanceStats(ctx, linodeID)

		return retryErr
	})

	return stats, err
}

// GetInstanceTransferByYearMonth retrieves monthly network transfer statistics with automatic retry on transient failures.
func (c *Client) GetInstanceTransferByYearMonth(ctx context.Context, linodeID, year, month int) (*Transfer, error) {
	var transfer *Transfer

	err := c.executeWithRetry(ctx, "GetInstanceTransferByYearMonth", func() error {
		var retryErr error

		transfer, retryErr = c.httpGetInstanceTransferByYearMonth(ctx, linodeID, year, month)

		return retryErr
	})

	return transfer, err
}

// GetInstanceBackup retrieves a specific backup with automatic retry on transient failures.
func (c *Client) GetInstanceBackup(ctx context.Context, linodeID, backupID int) (*InstanceBackup, error) {
	var backup *InstanceBackup

	err := c.executeWithRetry(ctx, "GetInstanceBackup", func() error {
		var retryErr error

		backup, retryErr = c.httpGetInstanceBackup(ctx, linodeID, backupID)

		return retryErr
	})

	return backup, err
}

// GetInstanceBackupProto retrieves one instance backup as a proto message with
// automatic retry on transient failures.
func (c *Client) GetInstanceBackupProto(ctx context.Context, linodeID, backupID int) (*linodev1.InstanceBackup, error) {
	var backup *linodev1.InstanceBackup

	err := c.executeWithRetry(ctx, "GetInstanceBackup", func() error {
		var retryErr error

		backup, retryErr = c.httpGetInstanceBackupProto(ctx, linodeID, backupID)

		return retryErr
	})

	return backup, err
}

// CreateInstanceBackup creates a manual snapshot with automatic retry on transient failures.
func (c *Client) CreateInstanceBackup(ctx context.Context, linodeID int, label string) (*InstanceBackup, error) {
	var backup *InstanceBackup

	err := c.executeWithRetry(ctx, "CreateInstanceBackup", func() error {
		var retryErr error

		backup, retryErr = c.httpCreateInstanceBackup(ctx, linodeID, label)

		return retryErr
	})

	return backup, err
}

// RestoreInstanceBackup restores a backup to an instance with automatic retry on transient failures.
func (c *Client) RestoreInstanceBackup(ctx context.Context, linodeID, backupID int, req RestoreBackupRequest) error {
	return c.executeWithRetry(ctx, "RestoreInstanceBackup", func() error {
		return c.httpRestoreInstanceBackup(ctx, linodeID, backupID, req)
	})
}

// EnableInstanceBackups enables the backup service with automatic retry on transient failures.
func (c *Client) EnableInstanceBackups(ctx context.Context, linodeID int) error {
	return c.executeWithRetry(ctx, "EnableInstanceBackups", func() error {
		return c.httpEnableInstanceBackups(ctx, linodeID)
	})
}

// CancelInstanceBackups cancels the backup service with automatic retry on transient failures.
func (c *Client) CancelInstanceBackups(ctx context.Context, linodeID int) error {
	return c.executeWithRetry(ctx, "CancelInstanceBackups", func() error {
		return c.httpCancelInstanceBackups(ctx, linodeID)
	})
}

// ApplyInstanceFirewalls reapplies assigned firewalls without retrying the mutating POST call.
func (c *Client) ApplyInstanceFirewalls(ctx context.Context, linodeID int) error {
	return c.executeWithoutRetry(ctx, "ApplyInstanceFirewalls", func() error {
		return c.httpApplyInstanceFirewalls(ctx, linodeID)
	})
}

// CreateInstanceConfig creates a configuration profile without retrying the POST create call.
func (c *Client) CreateInstanceConfig(ctx context.Context, linodeID int, req *CreateConfigRequest) (*InstanceConfig, error) {
	var config *InstanceConfig

	err := c.executeWithoutRetry(ctx, "CreateInstanceConfig", func() error {
		var retryErr error

		config, retryErr = c.httpCreateInstanceConfig(ctx, linodeID, req)

		return retryErr
	})

	return config, err
}

// UpdateInstanceConfig updates a configuration profile without retrying the PUT update call.
func (c *Client) UpdateInstanceConfig(ctx context.Context, linodeID, configID int, req *UpdateConfigRequest) (*InstanceConfig, error) {
	var config *InstanceConfig

	err := c.executeWithoutRetry(ctx, "UpdateInstanceConfig", func() error {
		var retryErr error

		config, retryErr = c.httpUpdateInstanceConfig(ctx, linodeID, configID, req)

		return retryErr
	})

	return config, err
}

// AddInstanceConfigInterface appends an interface without retrying the POST append call.
func (c *Client) AddInstanceConfigInterface(ctx context.Context, linodeID, configID int, req *ConfigInterface) (*ConfigInterface, error) {
	var configInterface *ConfigInterface

	err := c.executeWithoutRetry(ctx, "AddInstanceConfigInterface", func() error {
		var retryErr error

		configInterface, retryErr = c.httpAddInstanceConfigInterface(ctx, linodeID, configID, req)

		return retryErr
	})

	return configInterface, err
}

// UpdateInstanceConfigInterface updates an interface without retrying the PUT update call.
func (c *Client) UpdateInstanceConfigInterface(ctx context.Context, linodeID, configID, interfaceID int, req *UpdateConfigInterfaceRequest) (*ConfigInterfaceResponse, error) {
	var configInterface *ConfigInterfaceResponse

	err := c.executeWithoutRetry(ctx, "UpdateInstanceConfigInterface", func() error {
		var retryErr error

		configInterface, retryErr = c.httpUpdateInstanceConfigInterface(ctx, linodeID, configID, interfaceID, req)

		return retryErr
	})

	return configInterface, err
}

// ReorderInstanceConfigInterfaces reorders configuration interfaces without retrying the POST reorder call.
func (c *Client) ReorderInstanceConfigInterfaces(ctx context.Context, linodeID, configID int, req *ReorderConfigInterfacesRequest) error {
	return c.executeWithoutRetry(ctx, "ReorderInstanceConfigInterfaces", func() error {
		return c.httpReorderInstanceConfigInterfaces(ctx, linodeID, configID, req)
	})
}

// GetInstanceConfigInterface retrieves an interface with automatic retry on transient failures.
func (c *Client) GetInstanceConfigInterface(ctx context.Context, linodeID, configID, interfaceID int) (*ConfigInterfaceResponse, error) {
	var configInterface *ConfigInterfaceResponse

	err := c.executeWithRetry(ctx, "GetInstanceConfigInterface", func() error {
		var retryErr error

		configInterface, retryErr = c.httpGetInstanceConfigInterface(ctx, linodeID, configID, interfaceID)

		return retryErr
	})

	return configInterface, err
}

// GetInstanceConfigInterfaceProto retrieves one config interface as a proto
// message with automatic retry on transient failures.
func (c *Client) GetInstanceConfigInterfaceProto(ctx context.Context, linodeID, configID, interfaceID int) (*linodev1.ConfigInterfaceResponse, error) {
	var configInterface *linodev1.ConfigInterfaceResponse

	err := c.executeWithRetry(ctx, "GetInstanceConfigInterface", func() error {
		var retryErr error

		configInterface, retryErr = c.httpGetInstanceConfigInterfaceProto(ctx, linodeID, configID, interfaceID)

		return retryErr
	})

	return configInterface, err
}

// DeleteInstanceConfigInterface removes an interface without retrying the DELETE call.
func (c *Client) DeleteInstanceConfigInterface(ctx context.Context, linodeID, configID, interfaceID int) error {
	return c.executeWithoutRetry(ctx, "DeleteInstanceConfigInterface", func() error {
		return c.httpDeleteInstanceConfigInterface(ctx, linodeID, configID, interfaceID)
	})
}

// ListInstanceDisks retrieves all disks for an instance with automatic retry on transient failures.
func (c *Client) ListInstanceDisks(ctx context.Context, linodeID int) ([]InstanceDisk, error) {
	var disks []InstanceDisk

	err := c.executeWithRetry(ctx, "ListInstanceDisks", func() error {
		var retryErr error

		disks, retryErr = c.httpListInstanceDisks(ctx, linodeID)

		return retryErr
	})

	return disks, err
}

// ListInstanceDisksProto retrieves an instance's disks as proto messages with
// automatic retry on transient failures.
func (c *Client) ListInstanceDisksProto(ctx context.Context, linodeID int) ([]*linodev1.InstanceDisk, error) {
	var disks []*linodev1.InstanceDisk

	err := c.executeWithRetry(ctx, "ListInstanceDisks", func() error {
		var retryErr error

		disks, retryErr = c.httpListInstanceDisksProto(ctx, linodeID)

		return retryErr
	})

	return disks, err
}

// ListInstanceConfigs retrieves all configuration profiles for an instance with automatic retry on transient failures.
func (c *Client) ListInstanceConfigs(ctx context.Context, linodeID, page, pageSize int) ([]InstanceConfig, error) {
	var configs []InstanceConfig

	err := c.executeWithRetry(ctx, "ListInstanceConfigs", func() error {
		var retryErr error

		configs, retryErr = c.httpListInstanceConfigs(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return configs, err
}

// ListInstanceConfigsProto retrieves an instance's configuration profiles as
// proto messages with automatic retry on transient failures.
func (c *Client) ListInstanceConfigsProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.InstanceConfig, error) {
	var configs []*linodev1.InstanceConfig

	err := c.executeWithRetry(ctx, "ListInstanceConfigs", func() error {
		var retryErr error

		configs, retryErr = c.httpListInstanceConfigsProto(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return configs, err
}

// ListInstanceVolumes retrieves all volumes attached to an instance with automatic retry on transient failures.
func (c *Client) ListInstanceVolumes(ctx context.Context, linodeID, page, pageSize int) ([]Volume, error) {
	var volumes []Volume

	err := c.executeWithRetry(ctx, "ListInstanceVolumes", func() error {
		var retryErr error

		volumes, retryErr = c.httpListInstanceVolumes(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return volumes, err
}

// ListInstanceVolumesProto retrieves an instance's attached volumes as proto
// messages with automatic retry on transient failures.
func (c *Client) ListInstanceVolumesProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.Volume, error) {
	var volumes []*linodev1.Volume

	err := c.executeWithRetry(ctx, "ListInstanceVolumes", func() error {
		var retryErr error

		volumes, retryErr = c.httpListInstanceVolumesProto(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return volumes, err
}

// ListInstanceNodeBalancers retrieves NodeBalancers assigned to an instance with automatic retry on transient failures.
func (c *Client) ListInstanceNodeBalancers(ctx context.Context, linodeID int) ([]NodeBalancer, error) {
	var nodeBalancers []NodeBalancer

	err := c.executeWithRetry(ctx, "ListInstanceNodeBalancers", func() error {
		var retryErr error

		nodeBalancers, retryErr = c.httpListInstanceNodeBalancers(ctx, linodeID)

		return retryErr
	})

	return nodeBalancers, err
}

// ListInstanceNodeBalancersProto retrieves the NodeBalancers assigned to an
// instance as proto messages with automatic retry on transient failures.
func (c *Client) ListInstanceNodeBalancersProto(ctx context.Context, linodeID int) ([]*linodev1.NodeBalancer, error) {
	var nodeBalancers []*linodev1.NodeBalancer

	err := c.executeWithRetry(ctx, "ListInstanceNodeBalancers", func() error {
		var retryErr error

		nodeBalancers, retryErr = c.httpListInstanceNodeBalancersProto(ctx, linodeID)

		return retryErr
	})

	return nodeBalancers, err
}

// ListInstanceInterfaces retrieves Linode interfaces with automatic retry on transient failures.
func (c *Client) ListInstanceInterfaces(ctx context.Context, linodeID int) ([]InstanceInterface, error) {
	var interfaces []InstanceInterface

	err := c.executeWithRetry(ctx, "ListInstanceInterfaces", func() error {
		var retryErr error

		interfaces, retryErr = c.httpListInstanceInterfaces(ctx, linodeID)

		return retryErr
	})

	return interfaces, err
}

// ListInstanceInterfacesProto retrieves an instance's current-generation
// interfaces as proto messages with automatic retry on transient failures.
func (c *Client) ListInstanceInterfacesProto(ctx context.Context, linodeID int) ([]*linodev1.InstanceInterface, error) {
	var interfaces []*linodev1.InstanceInterface

	err := c.executeWithRetry(ctx, "ListInstanceInterfaces", func() error {
		var retryErr error

		interfaces, retryErr = c.httpListInstanceInterfacesProto(ctx, linodeID)

		return retryErr
	})

	return interfaces, err
}

// UpgradeLinodeInterfaces upgrades legacy config interfaces without retrying the POST upgrade call.
func (c *Client) UpgradeLinodeInterfaces(ctx context.Context, linodeID int, req *UpgradeLinodeInterfacesRequest) (*UpgradeLinodeInterfacesResponse, error) {
	var result *UpgradeLinodeInterfacesResponse

	err := c.executeWithoutRetry(ctx, "UpgradeLinodeInterfaces", func() error {
		var retryErr error

		result, retryErr = c.httpUpgradeLinodeInterfaces(ctx, linodeID, req)

		return retryErr
	})

	return result, err
}

// GetInstanceInterface retrieves a Linode interface with automatic retry on transient failures.
func (c *Client) GetInstanceInterface(ctx context.Context, linodeID, interfaceID int) (*InstanceInterface, error) {
	var instanceInterface *InstanceInterface

	err := c.executeWithRetry(ctx, "GetInstanceInterface", func() error {
		var retryErr error

		instanceInterface, retryErr = c.httpGetInstanceInterface(ctx, linodeID, interfaceID)

		return retryErr
	})

	return instanceInterface, err
}

// GetInstanceInterfaceProto retrieves a Linode interface as a proto message with
// automatic retry on transient failures.
func (c *Client) GetInstanceInterfaceProto(ctx context.Context, linodeID, interfaceID int) (*linodev1.InstanceInterface, error) {
	var instanceInterface *linodev1.InstanceInterface

	err := c.executeWithRetry(ctx, "GetInstanceInterface", func() error {
		var retryErr error

		instanceInterface, retryErr = c.httpGetInstanceInterfaceProto(ctx, linodeID, interfaceID)

		return retryErr
	})

	return instanceInterface, err
}

// ListInstanceInterfaceFirewalls retrieves Cloud Firewalls assigned to a Linode interface with automatic retry on transient failures.
func (c *Client) ListInstanceInterfaceFirewalls(ctx context.Context, linodeID, interfaceID int) ([]Firewall, error) {
	var firewalls []Firewall

	err := c.executeWithRetry(ctx, "ListInstanceInterfaceFirewalls", func() error {
		var retryErr error

		firewalls, retryErr = c.httpListInstanceInterfaceFirewalls(ctx, linodeID, interfaceID)

		return retryErr
	})

	return firewalls, err
}

// ListInstanceInterfaceFirewallsProto retrieves Cloud Firewalls assigned to a
// Linode interface as proto messages with automatic retry on transient failures.
func (c *Client) ListInstanceInterfaceFirewallsProto(ctx context.Context, linodeID, interfaceID int) ([]*linodev1.Firewall, error) {
	var firewalls []*linodev1.Firewall

	err := c.executeWithRetry(ctx, "ListInstanceInterfaceFirewalls", func() error {
		var retryErr error

		firewalls, retryErr = c.httpListInstanceInterfaceFirewallsProto(ctx, linodeID, interfaceID)

		return retryErr
	})

	return firewalls, err
}

// ListInstanceInterfaceHistory retrieves Linode interface history with automatic retry on transient failures.
func (c *Client) ListInstanceInterfaceHistory(ctx context.Context, linodeID, page, pageSize int) (*PaginatedResponse[InstanceInterfaceHistory], error) {
	var history *PaginatedResponse[InstanceInterfaceHistory]

	err := c.executeWithRetry(ctx, "ListInstanceInterfaceHistory", func() error {
		var retryErr error

		history, retryErr = c.httpListInstanceInterfaceHistory(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return history, err
}

// ListInstanceInterfaceHistoryProto retrieves an instance's historical interface
// versions as proto messages with automatic retry on transient failures.
func (c *Client) ListInstanceInterfaceHistoryProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.InstanceInterfaceHistory, error) {
	var history []*linodev1.InstanceInterfaceHistory

	err := c.executeWithRetry(ctx, "ListInstanceInterfaceHistory", func() error {
		var retryErr error

		history, retryErr = c.httpListInstanceInterfaceHistoryProto(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return history, err
}

// ListInstanceConfigInterfacesProto retrieves a configuration profile's legacy
// network interfaces as proto messages with automatic retry on transient
// failures.
func (c *Client) ListInstanceConfigInterfacesProto(ctx context.Context, linodeID, configID int) ([]*linodev1.ConfigInterfaceResponse, error) {
	var interfaces []*linodev1.ConfigInterfaceResponse

	err := c.executeWithRetry(ctx, "ListInstanceConfigInterfaces", func() error {
		var retryErr error

		interfaces, retryErr = c.httpListInstanceConfigInterfacesProto(ctx, linodeID, configID)

		return retryErr
	})

	return interfaces, err
}

// GetInstanceInterfaceSettings retrieves Linode interface settings with automatic retry on transient failures.
func (c *Client) GetInstanceInterfaceSettings(ctx context.Context, linodeID int) (*InstanceInterfaceSettings, error) {
	var settings *InstanceInterfaceSettings

	err := c.executeWithRetry(ctx, "GetInstanceInterfaceSettings", func() error {
		var retryErr error

		settings, retryErr = c.httpGetInstanceInterfaceSettings(ctx, linodeID)

		return retryErr
	})

	return settings, err
}

// GetInstanceInterfaceSettingsProto retrieves a Linode's interface settings as a
// proto message with automatic retry on transient failures.
func (c *Client) GetInstanceInterfaceSettingsProto(ctx context.Context, linodeID int) (*linodev1.InstanceInterfaceSettings, error) {
	var settings *linodev1.InstanceInterfaceSettings

	err := c.executeWithRetry(ctx, "GetInstanceInterfaceSettings", func() error {
		var retryErr error

		settings, retryErr = c.httpGetInstanceInterfaceSettingsProto(ctx, linodeID)

		return retryErr
	})

	return settings, err
}

// UpdateInstanceInterfaceSettings updates Linode interface settings without retrying the PUT mutation.
func (c *Client) UpdateInstanceInterfaceSettings(ctx context.Context, linodeID int, req *UpdateInstanceInterfaceSettingsRequest) (*InstanceInterfaceSettings, error) {
	var settings *InstanceInterfaceSettings

	err := c.executeWithoutRetry(ctx, "UpdateInstanceInterfaceSettings", func() error {
		var retryErr error

		settings, retryErr = c.httpUpdateInstanceInterfaceSettings(ctx, linodeID, req)

		return retryErr
	})

	return settings, err
}

// AddInstanceInterface creates an interface without retrying the POST create call.
func (c *Client) AddInstanceInterface(ctx context.Context, linodeID int, req *AddInstanceInterfaceRequest) (*InstanceInterface, error) {
	var instanceInterface *InstanceInterface

	err := c.executeWithoutRetry(ctx, "AddInstanceInterface", func() error {
		var retryErr error

		instanceInterface, retryErr = c.httpAddInstanceInterface(ctx, linodeID, req)

		return retryErr
	})

	return instanceInterface, err
}

// UpdateInstanceInterface updates an interface without retrying the PUT mutation.
func (c *Client) UpdateInstanceInterface(ctx context.Context, linodeID, interfaceID int, req *UpdateInstanceInterfaceRequest) (*InstanceInterface, error) {
	var instanceInterface *InstanceInterface

	err := c.executeWithoutRetry(ctx, "UpdateInstanceInterface", func() error {
		var retryErr error

		instanceInterface, retryErr = c.httpUpdateInstanceInterface(ctx, linodeID, interfaceID, req)

		return retryErr
	})

	return instanceInterface, err
}

// DeleteInstanceInterface removes an interface without retrying the DELETE call.
func (c *Client) DeleteInstanceInterface(ctx context.Context, linodeID, interfaceID int) error {
	return c.executeWithoutRetry(ctx, "DeleteInstanceInterface", func() error {
		return c.httpDeleteInstanceInterface(ctx, linodeID, interfaceID)
	})
}

// ListInstanceConfigInterfaces retrieves configuration profile interfaces with automatic retry on transient failures.
func (c *Client) ListInstanceConfigInterfaces(ctx context.Context, linodeID, configID int) ([]ConfigInterfaceResponse, error) {
	var interfaces []ConfigInterfaceResponse

	err := c.executeWithRetry(ctx, "ListInstanceConfigInterfaces", func() error {
		var retryErr error

		interfaces, retryErr = c.httpListInstanceConfigInterfaces(ctx, linodeID, configID)

		return retryErr
	})

	return interfaces, err
}

// UpdateInstanceFirewalls replaces firewall assignments for an instance without replaying the state-changing request.
func (c *Client) UpdateInstanceFirewalls(ctx context.Context, linodeID, page, pageSize int, req *UpdateInstanceFirewallsRequest) ([]Firewall, error) {
	var firewalls []Firewall

	err := c.executeWithoutRetry(ctx, "UpdateInstanceFirewalls", func() error {
		var retryErr error

		firewalls, retryErr = c.httpUpdateInstanceFirewalls(ctx, linodeID, page, pageSize, req)

		return retryErr
	})

	return firewalls, err
}

// GetInstanceConfig retrieves a specific configuration profile with automatic retry on transient failures.
func (c *Client) GetInstanceConfig(ctx context.Context, linodeID, configID int) (*InstanceConfig, error) {
	var config *InstanceConfig

	err := c.executeWithRetry(ctx, "GetInstanceConfig", func() error {
		var retryErr error

		config, retryErr = c.httpGetInstanceConfig(ctx, linodeID, configID)

		return retryErr
	})

	return config, err
}

// DeleteInstanceConfig deletes a configuration profile without retrying the DELETE call.
func (c *Client) DeleteInstanceConfig(ctx context.Context, linodeID, configID int) error {
	return c.executeWithoutRetry(ctx, "DeleteInstanceConfig", func() error {
		return c.httpDeleteInstanceConfig(ctx, linodeID, configID)
	})
}

// ListInstanceFirewalls retrieves all Cloud Firewalls assigned to an instance with automatic retry on transient failures.
func (c *Client) ListInstanceFirewalls(ctx context.Context, linodeID, page, pageSize int) ([]Firewall, error) {
	var firewalls []Firewall

	err := c.executeWithRetry(ctx, "ListInstanceFirewalls", func() error {
		var retryErr error

		firewalls, retryErr = c.httpListInstanceFirewalls(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return firewalls, err
}

// ListInstanceFirewallsProto retrieves an instance's assigned Cloud Firewalls as
// proto messages with automatic retry on transient failures.
func (c *Client) ListInstanceFirewallsProto(ctx context.Context, linodeID, page, pageSize int) ([]*linodev1.Firewall, error) {
	var firewalls []*linodev1.Firewall

	err := c.executeWithRetry(ctx, "ListInstanceFirewalls", func() error {
		var retryErr error

		firewalls, retryErr = c.httpListInstanceFirewallsProto(ctx, linodeID, page, pageSize)

		return retryErr
	})

	return firewalls, err
}

// GetInstanceDisk retrieves a specific disk with automatic retry on transient failures.
func (c *Client) GetInstanceDisk(ctx context.Context, linodeID, diskID int) (*InstanceDisk, error) {
	var disk *InstanceDisk

	err := c.executeWithRetry(ctx, "GetInstanceDisk", func() error {
		var retryErr error

		disk, retryErr = c.httpGetInstanceDisk(ctx, linodeID, diskID)

		return retryErr
	})

	return disk, err
}

// GetInstanceDiskProto retrieves one instance disk as a proto message with
// automatic retry on transient failures.
func (c *Client) GetInstanceDiskProto(ctx context.Context, linodeID, diskID int) (*linodev1.InstanceDisk, error) {
	var disk *linodev1.InstanceDisk

	err := c.executeWithRetry(ctx, "GetInstanceDisk", func() error {
		var retryErr error

		disk, retryErr = c.httpGetInstanceDiskProto(ctx, linodeID, diskID)

		return retryErr
	})

	return disk, err
}

// CreateInstanceDisk creates a new disk with automatic retry on transient failures.
func (c *Client) CreateInstanceDisk(ctx context.Context, linodeID int, req *CreateDiskRequest) (*InstanceDisk, error) {
	var disk *InstanceDisk

	err := c.executeWithRetry(ctx, "CreateInstanceDisk", func() error {
		var retryErr error

		disk, retryErr = c.httpCreateInstanceDisk(ctx, linodeID, req)

		return retryErr
	})

	return disk, err
}

// UpdateInstanceDisk updates a disk with automatic retry on transient failures.
func (c *Client) UpdateInstanceDisk(ctx context.Context, linodeID, diskID int, req UpdateDiskRequest) (*InstanceDisk, error) {
	var disk *InstanceDisk

	err := c.executeWithRetry(ctx, "UpdateInstanceDisk", func() error {
		var retryErr error

		disk, retryErr = c.httpUpdateInstanceDisk(ctx, linodeID, diskID, req)

		return retryErr
	})

	return disk, err
}

// DeleteInstanceDisk deletes a disk with automatic retry on transient failures.
func (c *Client) DeleteInstanceDisk(ctx context.Context, linodeID, diskID int) error {
	return c.executeWithRetry(ctx, "DeleteInstanceDisk", func() error {
		return c.httpDeleteInstanceDisk(ctx, linodeID, diskID)
	})
}

// CloneInstanceDisk clones a disk with automatic retry on transient failures.
func (c *Client) CloneInstanceDisk(ctx context.Context, linodeID, diskID int) (*InstanceDisk, error) {
	var disk *InstanceDisk

	err := c.executeWithRetry(ctx, "CloneInstanceDisk", func() error {
		var retryErr error

		disk, retryErr = c.httpCloneInstanceDisk(ctx, linodeID, diskID)

		return retryErr
	})

	return disk, err
}

// ResizeInstanceDisk resizes a disk with automatic retry on transient failures.
func (c *Client) ResizeInstanceDisk(ctx context.Context, linodeID, diskID int, req ResizeDiskRequest) error {
	return c.executeWithRetry(ctx, "ResizeInstanceDisk", func() error {
		return c.httpResizeInstanceDisk(ctx, linodeID, diskID, req)
	})
}

// ListInstanceIPs retrieves all IP addresses for an instance with automatic retry on transient failures.
func (c *Client) ListInstanceIPs(ctx context.Context, linodeID int) (*InstanceIPAddresses, error) {
	var ips *InstanceIPAddresses

	err := c.executeWithRetry(ctx, "ListInstanceIPs", func() error {
		var retryErr error

		ips, retryErr = c.httpListInstanceIPs(ctx, linodeID)

		return retryErr
	})

	return ips, err
}

// ListInstanceIPsProto retrieves the full IPv4/IPv6 address configuration for an
// instance as a proto message with automatic retry on transient failures.
func (c *Client) ListInstanceIPsProto(ctx context.Context, linodeID int) (*linodev1.InstanceIPsResponse, error) {
	var ips *linodev1.InstanceIPsResponse

	err := c.executeWithRetry(ctx, "ListInstanceIPs", func() error {
		var retryErr error

		ips, retryErr = c.httpListInstanceIPsProto(ctx, linodeID)

		return retryErr
	})

	return ips, err
}

// GetInstanceIP retrieves a specific IP address with automatic retry on transient failures.
func (c *Client) GetInstanceIP(ctx context.Context, linodeID int, address string) (*IPAddress, error) {
	var ipAddr *IPAddress

	err := c.executeWithRetry(ctx, "GetInstanceIP", func() error {
		var retryErr error

		ipAddr, retryErr = c.httpGetInstanceIP(ctx, linodeID, address)

		return retryErr
	})

	return ipAddr, err
}

// GetInstanceIPProto retrieves one instance IP address as a proto message with
// automatic retry on transient failures.
func (c *Client) GetInstanceIPProto(ctx context.Context, linodeID int, address string) (*linodev1.IPAddress, error) {
	var ipAddr *linodev1.IPAddress

	err := c.executeWithRetry(ctx, "GetInstanceIP", func() error {
		var retryErr error

		ipAddr, retryErr = c.httpGetInstanceIPProto(ctx, linodeID, address)

		return retryErr
	})

	return ipAddr, err
}

// AllocateInstanceIP allocates a new IP address with automatic retry on transient failures.
func (c *Client) AllocateInstanceIP(ctx context.Context, linodeID int, req AllocateIPRequest) (*IPAddress, error) {
	var ipAddr *IPAddress

	err := c.executeWithRetry(ctx, "AllocateInstanceIP", func() error {
		var retryErr error

		ipAddr, retryErr = c.httpAllocateInstanceIP(ctx, linodeID, req)

		return retryErr
	})

	return ipAddr, err
}

// UpdateInstanceIP updates an IP address RDNS with automatic retry on transient failures.
func (c *Client) UpdateInstanceIP(ctx context.Context, linodeID int, address string, req UpdateIPRDNSRequest) (*IPAddress, error) {
	var ipAddr *IPAddress

	err := c.executeWithRetry(ctx, "UpdateInstanceIP", func() error {
		var retryErr error

		ipAddr, retryErr = c.httpUpdateInstanceIP(ctx, linodeID, address, req)

		return retryErr
	})

	return ipAddr, err
}

// DeleteInstanceIP removes an IP address with automatic retry on transient failures.
func (c *Client) DeleteInstanceIP(ctx context.Context, linodeID int, address string) error {
	return c.executeWithRetry(ctx, "DeleteInstanceIP", func() error {
		return c.httpDeleteInstanceIP(ctx, linodeID, address)
	})
}

// CloneInstance clones an instance with automatic retry on transient failures.
func (c *Client) CloneInstance(ctx context.Context, linodeID int, req *CloneInstanceRequest) (*Instance, error) {
	var instance *Instance

	err := c.executeWithRetry(ctx, "CloneInstance", func() error {
		var retryErr error

		instance, retryErr = c.httpCloneInstance(ctx, linodeID, req)

		return retryErr
	})

	return instance, err
}

// CloneInstanceProto clones an instance as a proto message with automatic retry
// on transient failures.
func (c *Client) CloneInstanceProto(ctx context.Context, linodeID int, req *CloneInstanceRequest) (*linodev1.Instance, error) {
	var instance *linodev1.Instance

	err := c.executeWithRetry(ctx, "CloneInstance", func() error {
		var retryErr error

		instance, retryErr = c.httpCloneInstanceProto(ctx, linodeID, req)

		return retryErr
	})

	return instance, err
}

// CreateInstanceConfigProto creates a configuration profile and returns the proto
// element, with automatic retry on transient failures.
func (c *Client) CreateInstanceConfigProto(ctx context.Context, linodeID int, req *CreateConfigRequest) (*linodev1.InstanceConfig, error) {
	var config *linodev1.InstanceConfig

	err := c.executeWithRetry(ctx, "CreateInstanceConfig", func() error {
		var retryErr error

		config, retryErr = c.httpCreateInstanceConfigProto(ctx, linodeID, req)

		return retryErr
	})

	return config, err
}

// UpdateInstanceConfigProto updates a configuration profile and returns the proto
// element, with automatic retry on transient failures.
func (c *Client) UpdateInstanceConfigProto(ctx context.Context, linodeID, configID int, req *UpdateConfigRequest) (*linodev1.InstanceConfig, error) {
	var config *linodev1.InstanceConfig

	err := c.executeWithRetry(ctx, "UpdateInstanceConfig", func() error {
		var retryErr error

		config, retryErr = c.httpUpdateInstanceConfigProto(ctx, linodeID, configID, req)

		return retryErr
	})

	return config, err
}

// AddInstanceConfigInterfaceProto appends a network interface to a configuration
// profile and returns the proto element, with automatic retry on transient
// failures.
func (c *Client) AddInstanceConfigInterfaceProto(ctx context.Context, linodeID, configID int, req *ConfigInterface) (*linodev1.ConfigInterfaceResponse, error) {
	var configInterface *linodev1.ConfigInterfaceResponse

	err := c.executeWithRetry(ctx, "AddInstanceConfigInterface", func() error {
		var retryErr error

		configInterface, retryErr = c.httpAddInstanceConfigInterfaceProto(ctx, linodeID, configID, req)

		return retryErr
	})

	return configInterface, err
}

// UpdateInstanceConfigInterfaceProto updates a configuration profile interface and
// returns the proto element, with automatic retry on transient failures.
func (c *Client) UpdateInstanceConfigInterfaceProto(ctx context.Context, linodeID, configID, interfaceID int, req *UpdateConfigInterfaceRequest) (*linodev1.ConfigInterfaceResponse, error) {
	var configInterface *linodev1.ConfigInterfaceResponse

	err := c.executeWithRetry(ctx, "UpdateInstanceConfigInterface", func() error {
		var retryErr error

		configInterface, retryErr = c.httpUpdateInstanceConfigInterfaceProto(ctx, linodeID, configID, interfaceID, req)

		return retryErr
	})

	return configInterface, err
}

// CreateInstanceDiskProto creates a disk and returns the proto element, with
// automatic retry on transient failures.
func (c *Client) CreateInstanceDiskProto(ctx context.Context, linodeID int, req *CreateDiskRequest) (*linodev1.InstanceDisk, error) {
	var disk *linodev1.InstanceDisk

	err := c.executeWithRetry(ctx, "CreateInstanceDisk", func() error {
		var retryErr error

		disk, retryErr = c.httpCreateInstanceDiskProto(ctx, linodeID, req)

		return retryErr
	})

	return disk, err
}

// UpdateInstanceDiskProto updates a disk and returns the proto element, with
// automatic retry on transient failures.
func (c *Client) UpdateInstanceDiskProto(ctx context.Context, linodeID, diskID int, req UpdateDiskRequest) (*linodev1.InstanceDisk, error) {
	var disk *linodev1.InstanceDisk

	err := c.executeWithRetry(ctx, "UpdateInstanceDisk", func() error {
		var retryErr error

		disk, retryErr = c.httpUpdateInstanceDiskProto(ctx, linodeID, diskID, req)

		return retryErr
	})

	return disk, err
}

// CloneInstanceDiskProto clones a disk and returns the proto element, with
// automatic retry on transient failures.
func (c *Client) CloneInstanceDiskProto(ctx context.Context, linodeID, diskID int) (*linodev1.InstanceDisk, error) {
	var disk *linodev1.InstanceDisk

	err := c.executeWithRetry(ctx, "CloneInstanceDisk", func() error {
		var retryErr error

		disk, retryErr = c.httpCloneInstanceDiskProto(ctx, linodeID, diskID)

		return retryErr
	})

	return disk, err
}

// CreateInstanceBackupProto takes a manual snapshot and returns the proto
// element, with automatic retry on transient failures.
func (c *Client) CreateInstanceBackupProto(ctx context.Context, linodeID int, label string) (*linodev1.InstanceBackup, error) {
	var backup *linodev1.InstanceBackup

	err := c.executeWithRetry(ctx, "CreateInstanceBackup", func() error {
		var retryErr error

		backup, retryErr = c.httpCreateInstanceBackupProto(ctx, linodeID, label)

		return retryErr
	})

	return backup, err
}

// RebuildInstanceProto rebuilds an instance and returns the proto element, with
// automatic retry on transient failures.
func (c *Client) RebuildInstanceProto(ctx context.Context, linodeID int, req *RebuildInstanceRequest) (*linodev1.Instance, error) {
	var instance *linodev1.Instance

	err := c.executeWithRetry(ctx, "RebuildInstance", func() error {
		var retryErr error

		instance, retryErr = c.httpRebuildInstanceProto(ctx, linodeID, req)

		return retryErr
	})

	return instance, err
}

// MigrateInstance migrates an instance with automatic retry on transient failures.
func (c *Client) MigrateInstance(ctx context.Context, linodeID int, region string) error {
	return c.executeWithRetry(ctx, "MigrateInstance", func() error {
		return c.httpMigrateInstance(ctx, linodeID, region)
	})
}

// MutateInstance upgrades an instance without retrying the mutating request.
// Retrying can replay the upgrade after a transient error, so this method
// delegates exactly once.
func (c *Client) MutateInstance(ctx context.Context, linodeID int, req *MutateInstanceRequest) error {
	return c.executeWithoutRetry(ctx, "MutateInstance", func() error {
		return c.httpMutateInstance(ctx, linodeID, req)
	})
}

// RebuildInstance rebuilds an instance with automatic retry on transient failures.
func (c *Client) RebuildInstance(ctx context.Context, linodeID int, req *RebuildInstanceRequest) (*Instance, error) {
	var instance *Instance

	err := c.executeWithRetry(ctx, "RebuildInstance", func() error {
		var retryErr error

		instance, retryErr = c.httpRebuildInstance(ctx, linodeID, req)

		return retryErr
	})

	return instance, err
}

// RescueInstance boots an instance into rescue mode with automatic retry on transient failures.
func (c *Client) RescueInstance(ctx context.Context, linodeID int, req RescueInstanceRequest) error {
	return c.executeWithRetry(ctx, "RescueInstance", func() error {
		return c.httpRescueInstance(ctx, linodeID, req)
	})
}

// UpdateProfilePreferences updates profile preferences with automatic retry on transient failures.
func (c *Client) UpdateProfilePreferences(ctx context.Context, req ProfilePreferences) (ProfilePreferences, error) {
	var preferences ProfilePreferences

	err := c.executeWithRetry(ctx, "UpdateProfilePreferences", func() error {
		var err error

		preferences, err = c.httpUpdateProfilePreferences(ctx, req)

		return err
	})

	return preferences, err
}

// ResetInstancePassword resets the root password with automatic retry on transient failures.
func (c *Client) ResetInstancePassword(ctx context.Context, linodeID int, rootPass string) error {
	return c.executeWithRetry(ctx, "ResetInstancePassword", func() error {
		return c.httpResetInstancePassword(ctx, linodeID, rootPass)
	})
}

// ResetInstanceDiskPassword resets a disk root password without retrying the credential mutation.
func (c *Client) ResetInstanceDiskPassword(ctx context.Context, linodeID, diskID int, password string) error {
	return c.executeWithoutRetry(ctx, "ResetInstanceDiskPassword", func() error {
		return c.httpResetInstanceDiskPassword(ctx, linodeID, diskID, password)
	})
}

// UpdateProfile updates the user profile with automatic retry on transient failures.
func (c *Client) UpdateProfile(ctx context.Context, req *UpdateProfileRequest) (*Profile, error) {
	var profile *Profile

	err := c.executeWithRetry(ctx, "UpdateProfile", func() error {
		var err error

		profile, err = c.httpUpdateProfile(ctx, req)

		return err
	})

	return profile, err
}

func (c *Client) executeWithoutRetry(ctx context.Context, operation string, run func() error) error {
	if err := c.circuit.Allow(); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}

	err := run()
	if err == nil {
		c.circuit.RecordSuccess()

		return nil
	}

	if c.shouldRecordCircuitFailure(err) {
		c.circuit.RecordFailure()
	}

	return err
}

func (*Client) shouldRecordCircuitFailure(err error) bool {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.IsRateLimitError() || apiErr.IsServerError()
	}

	return isNetworkError(err) || isTimeoutError(err)
}

func (c *Client) executeWithRetry(ctx context.Context, operation string, retryFunc func() error) error {
	if err := c.circuit.Allow(); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	var lastErr error

	var attempt int

	for attempt <= c.retryCfg.MaxRetries {
		if attempt > 0 {
			delay := c.delayForAttempt(attempt, lastErr)
			select {
			case <-ctx.Done():
				// Caller canceled; not an upstream-health signal.
				return fmt.Errorf("context canceled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		err := retryFunc()
		if err == nil {
			c.circuit.RecordSuccess()

			return nil
		}

		lastErr = err
		attempt++

		if !c.shouldRetry(err) {
			// Non-retryable (e.g. auth), not the breaker's concern.
			return err
		}
	}

	// Retries exhausted on a retryable failure. This is exactly the signal
	// the breaker exists to track.
	c.circuit.RecordFailure()

	return fmt.Errorf("%s: %w", operation, lastErr)
}

// delayForAttempt picks how long to wait before the next attempt. When the
// upstream returned a Retry-After hint (typically 429), we honor that exactly
// so we stop hammering the API. For everything else we fall back to the
// exponential-with-jitter backoff. The hint is clamped to MaxRetryDelay so a
// hostile or buggy server can't ask us to wait an hour.
func (c *Client) delayForAttempt(attempt int, lastErr error) time.Duration {
	if apiErr, ok := errors.AsType[*APIError](lastErr); ok && apiErr.RetryAfter > 0 {
		hint := apiErr.RetryAfter
		if hint > c.retryCfg.MaxDelay {
			return c.retryCfg.MaxDelay
		}

		return hint
	}

	return c.calculateDelay(attempt)
}

func (c *Client) calculateDelay(attempt int) time.Duration {
	delay := float64(c.retryCfg.BaseDelay) * math.Pow(c.retryCfg.BackoffFactor, float64(attempt-1))

	if c.retryCfg.JitterEnabled {
		jitterMax := big.NewInt(int64(delay * jitterPercent))
		if jitterMax.Int64() > 0 {
			jitterBig, err := rand.Int(rand.Reader, jitterMax)
			if err != nil {
				return c.retryCfg.BaseDelay
			}

			jitter := float64(jitterBig.Int64())
			delay += jitter
		}
	}

	maxDelay := float64(c.retryCfg.MaxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}

	return time.Duration(delay)
}

func (*Client) shouldRetry(err error) bool {
	// Short-circuit on non-retryable API errors before falling through to
	// the general retryability check, which would otherwise return false for
	// these anyway but only after additional type assertions.
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		if apiErr.IsAuthenticationError() || apiErr.IsForbiddenError() {
			return false
		}
	}

	return isRetryable(err)
}
