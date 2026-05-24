package linode

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"
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

// CreateOAuthClient creates an account OAuth client without retrying the
// mutating request. Retrying can replay client creation after a transient
// error, so this method delegates exactly once.
func (c *Client) CreateOAuthClient(ctx context.Context, req *CreateOAuthClientRequest) (*CreatedOAuthClient, error) {
	return c.httpCreateOAuthClient(ctx, req)
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

// UpdateAccountUser updates an account user without retrying the mutating request.
// Retrying can replay user updates after a transient error, so this method
// delegates exactly once.
func (c *Client) UpdateAccountUser(ctx context.Context, username string, request *UpdateAccountUserRequest) (*AccountUser, error) {
	return c.httpUpdateAccountUser(ctx, username, request)
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

// ListAccountEntityTransfers retrieves account entity transfers with automatic retry on transient failures.
func (c *Client) ListAccountEntityTransfers(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountEntityTransfer], error) {
	var transfers *PaginatedResponse[AccountEntityTransfer]

	err := c.executeWithRetry(ctx, "ListAccountEntityTransfers", func() error {
		var err error

		transfers, err = c.httpListAccountEntityTransfers(ctx, page, pageSize)

		return err
	})

	return transfers, err
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

// GetAccountEntityTransfer retrieves one account entity transfer with automatic retry on transient failures.
func (c *Client) GetAccountEntityTransfer(ctx context.Context, token string) (*AccountEntityTransfer, error) {
	var transfer *AccountEntityTransfer

	err := c.executeWithRetry(ctx, "GetAccountEntityTransfer", func() error {
		var err error

		transfer, err = c.httpGetAccountEntityTransfer(ctx, token)

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

// MarkAccountEventSeen marks one account event as seen without retrying the
// mutating request. Retrying can replay the state change after a transient
// error, so this method delegates exactly once.
func (c *Client) MarkAccountEventSeen(ctx context.Context, eventID int) error {
	return c.httpMarkAccountEventSeen(ctx, eventID)
}

// CreateAccountEntityTransfer creates an account entity transfer without retrying
// the mutating request. Retrying can replay transfer creation after a transient
// error, so this method delegates exactly once.
func (c *Client) CreateAccountEntityTransfer(ctx context.Context, req *CreateAccountEntityTransferRequest) (*AccountEntityTransfer, error) {
	return c.httpCreateAccountEntityTransfer(ctx, req)
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

// AcceptAccountEntityTransfer accepts an account entity transfer without retrying
// the mutating request. Retrying can replay transfer acceptance after a transient
// error, so this method delegates exactly once.
func (c *Client) AcceptAccountEntityTransfer(ctx context.Context, token string) error {
	return c.httpAcceptAccountEntityTransfer(ctx, token)
}

// DeleteAccountEntityTransfer cancels an account entity transfer without retrying
// the mutating request. Retrying can replay transfer cancellation after a
// transient error, so this method delegates exactly once.
func (c *Client) DeleteAccountEntityTransfer(ctx context.Context, token string) error {
	return c.httpDeleteAccountEntityTransfer(ctx, token)
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

// ResetDatabaseInstanceCredentials resets MySQL Managed Database credentials without retrying the POST.
func (c *Client) ResetDatabaseInstanceCredentials(ctx context.Context, instanceID int) (*DatabaseCredentials, error) {
	return c.httpResetDatabaseInstanceCredentials(ctx, instanceID)
}

// CreateDatabaseInstance creates or restores a MySQL Managed Database instance without retrying the POST.
func (c *Client) CreateDatabaseInstance(ctx context.Context, req *CreateDatabaseInstanceRequest) (*DatabaseInstance, error) {
	return c.httpCreateDatabaseInstance(ctx, req)
}

// UpdateDatabaseInstance updates one MySQL Managed Database instance without retrying the PUT.
func (c *Client) UpdateDatabaseInstance(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*DatabaseInstance, error) {
	return c.httpUpdateDatabaseInstance(ctx, instanceID, req)
}

// DeleteDatabaseInstance deletes one MySQL Managed Database instance without retrying the DELETE.
func (c *Client) DeleteDatabaseInstance(ctx context.Context, instanceID int) error {
	return c.httpDeleteDatabaseInstance(ctx, instanceID)
}

// PatchDatabaseInstance applies security patches and updates to one MySQL Managed Database instance without retrying the POST.
func (c *Client) PatchDatabaseInstance(ctx context.Context, instanceID int) error {
	return c.httpPatchDatabaseInstance(ctx, instanceID)
}

// SuspendDatabaseInstance suspends one active MySQL Managed Database instance without retrying the POST.
func (c *Client) SuspendDatabaseInstance(ctx context.Context, instanceID int) error {
	return c.httpSuspendDatabaseInstance(ctx, instanceID)
}

// ResumeDatabaseInstance resumes one suspended MySQL Managed Database instance without retrying the POST.
func (c *Client) ResumeDatabaseInstance(ctx context.Context, instanceID int) error {
	return c.httpResumeDatabaseInstance(ctx, instanceID)
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

// CreateImage creates a private image from a Linode disk without automatic retry.
// Replaying this non-idempotent create operation could create duplicate images.
func (c *Client) CreateImage(ctx context.Context, req *CreateImageRequest) (*Image, error) {
	return c.httpCreateImage(ctx, req)
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

// CreateStackScript creates a new StackScript with automatic retry on transient failures.
func (c *Client) CreateStackScript(ctx context.Context, req *CreateStackScriptRequest) (*StackScript, error) {
	var script *StackScript

	err := c.executeWithRetry(ctx, "CreateStackScript", func() error {
		var err error

		script, err = c.httpCreateStackScript(ctx, req)

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

// DeleteDomainRecord deletes a domain record with automatic retry on transient failures.
func (c *Client) DeleteDomainRecord(ctx context.Context, domainID, recordID int) error {
	return c.executeWithRetry(ctx, "DeleteDomainRecord", func() error {
		return c.httpDeleteDomainRecord(ctx, domainID, recordID)
	})
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

// ListObjectStorageBucketContents lists objects in a bucket with automatic retry.
func (c *Client) ListObjectStorageBucketContents(ctx context.Context, region, label string, params map[string]string) ([]ObjectStorageObject, bool, string, error) {
	var objects []ObjectStorageObject

	var isTruncated bool

	var nextMarker string

	err := c.executeWithRetry(ctx, "ListObjectStorageBucketContents", func() error {
		var err error

		objects, isTruncated, nextMarker, err = c.httpListObjectStorageBucketContents(ctx, region, label, params)

		return err
	})

	return objects, isTruncated, nextMarker, err
}

// ListObjectStorageClusters retrieves Object Storage clusters with automatic retry.
func (c *Client) ListObjectStorageClusters(ctx context.Context) ([]ObjectStorageCluster, error) {
	var clusters []ObjectStorageCluster

	err := c.executeWithRetry(ctx, "ListObjectStorageClusters", func() error {
		var err error

		clusters, err = c.httpListObjectStorageClusters(ctx)

		return err
	})

	return clusters, err
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

// CreateLKENodePool creates a new node pool with automatic retry on transient failures.
func (c *Client) CreateLKENodePool(ctx context.Context, clusterID int, req *CreateLKENodePoolRequest) (*LKENodePool, error) {
	var pool *LKENodePool

	err := c.executeWithRetry(ctx, "CreateLKENodePool", func() error {
		var err error

		pool, err = c.httpCreateLKENodePool(ctx, clusterID, req)

		return err
	})

	return pool, err
}

// UpdateLKENodePool updates a node pool with automatic retry on transient failures.
func (c *Client) UpdateLKENodePool(ctx context.Context, clusterID, poolID int, req UpdateLKENodePoolRequest) (*LKENodePool, error) {
	var pool *LKENodePool

	err := c.executeWithRetry(ctx, "UpdateLKENodePool", func() error {
		var err error

		pool, err = c.httpUpdateLKENodePool(ctx, clusterID, poolID, req)

		return err
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

// DeleteLKEServiceToken deletes the service token for an LKE cluster with automatic retry on transient failures.
func (c *Client) DeleteLKEServiceToken(ctx context.Context, clusterID int) error {
	return c.executeWithRetry(ctx, "DeleteLKEServiceToken", func() error {
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

// DeleteLKEControlPlaneACL deletes the control plane ACL with automatic retry on transient failures.
func (c *Client) DeleteLKEControlPlaneACL(ctx context.Context, clusterID int) error {
	return c.executeWithRetry(ctx, "DeleteLKEControlPlaneACL", func() error {
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

// ListLKETierVersions retrieves all LKE tier versions with automatic retry on transient failures.
func (c *Client) ListLKETierVersions(ctx context.Context) ([]LKETierVersion, error) {
	var versions []LKETierVersion

	err := c.executeWithRetry(ctx, "ListLKETierVersions", func() error {
		var err error

		versions, err = c.httpListLKETierVersions(ctx)

		return err
	})

	return versions, err
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

// DeleteVPC deletes a VPC with automatic retry on transient failures.
func (c *Client) DeleteVPC(ctx context.Context, vpcID int) error {
	return c.executeWithRetry(ctx, "DeleteVPC", func() error {
		return c.httpDeleteVPC(ctx, vpcID)
	})
}

// ListVPCIPs retrieves all VPC IP addresses with automatic retry on transient failures.
func (c *Client) ListVPCIPs(ctx context.Context) ([]VPCIP, error) {
	var ips []VPCIP

	err := c.executeWithRetry(ctx, "ListVPCIPs", func() error {
		var retryErr error

		ips, retryErr = c.httpListVPCIPs(ctx)

		return retryErr
	})

	return ips, err
}

// ListVPCIPAddresses retrieves IP addresses for a specific VPC with automatic retry on transient failures.
func (c *Client) ListVPCIPAddresses(ctx context.Context, vpcID int) ([]VPCIP, error) {
	var ips []VPCIP

	err := c.executeWithRetry(ctx, "ListVPCIPAddresses", func() error {
		var retryErr error

		ips, retryErr = c.httpListVPCIPAddresses(ctx, vpcID)

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

// CreateInstanceBackup creates a manual snapshot with automatic retry on transient failures.
func (c *Client) CreateInstanceBackup(ctx context.Context, linodeID int) (*InstanceBackup, error) {
	var backup *InstanceBackup

	err := c.executeWithRetry(ctx, "CreateInstanceBackup", func() error {
		var retryErr error

		backup, retryErr = c.httpCreateInstanceBackup(ctx, linodeID)

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

// MigrateInstance migrates an instance with automatic retry on transient failures.
func (c *Client) MigrateInstance(ctx context.Context, linodeID int, region string) error {
	return c.executeWithRetry(ctx, "MigrateInstance", func() error {
		return c.httpMigrateInstance(ctx, linodeID, region)
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

// ResetInstancePassword resets the root password with automatic retry on transient failures.
func (c *Client) ResetInstancePassword(ctx context.Context, linodeID int, rootPass string) error {
	return c.executeWithRetry(ctx, "ResetInstancePassword", func() error {
		return c.httpResetInstancePassword(ctx, linodeID, rootPass)
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
