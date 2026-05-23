package linode

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url" // path parameter escaping
	"strconv"
)

const (
	endpointProfile                      = "/profile"
	endpointProfileGrants                = "/profile/grants"
	endpointAccount                      = "/account"
	endpointAccountTransfer              = "/account/transfer"
	endpointAccountSettings              = "/account/settings"
	endpointAccountSettingsManagedEnable = "/account/settings/managed-enable"
	endpointAccountCancel                = "/account/cancel"
	endpointAccountAgreements            = "/account/agreements"
	endpointAccountMaintenance           = "/account/maintenance"
	endpointAccountNotifications         = "/account/notifications"
	endpointAccountAvailability          = "/account/availability"
	endpointAccountBetas                 = "/account/betas"
	endpointAccountOAuthClients          = "/account/oauth-clients"
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

// httpListAccountEntityTransfers retrieves account entity transfers.
func (c *Client) httpListAccountEntityTransfers(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountEntityTransfer], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointAccountEntityTransfers, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountEntityTransfers", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var transfers PaginatedResponse[AccountEntityTransfer]
	if err := c.handleResponse(resp, &transfers); err != nil {
		return nil, err
	}

	return &transfers, nil
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

// httpGetAccountEntityTransfer retrieves one account entity transfer by token.
func (c *Client) httpGetAccountEntityTransfer(ctx context.Context, token string) (*AccountEntityTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountEntityTransfers + "/" + url.PathEscape(token)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountEntityTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var transfer AccountEntityTransfer
	if err := c.handleResponse(resp, &transfer); err != nil {
		return nil, err
	}

	return &transfer, nil
}

// httpCreateAccountEntityTransfer creates an account entity transfer.
func (c *Client) httpCreateAccountEntityTransfer(ctx context.Context, req *CreateAccountEntityTransferRequest) (*AccountEntityTransfer, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountEntityTransfers, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateAccountEntityTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var transfer AccountEntityTransfer
	if err := c.handleResponse(resp, &transfer); err != nil {
		return nil, err
	}

	return &transfer, nil
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

// httpAcceptAccountEntityTransfer accepts one account entity transfer by token.
func (c *Client) httpAcceptAccountEntityTransfer(ctx context.Context, token string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountEntityTransfers + "/" + url.PathEscape(token) + "/accept"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "AcceptAccountEntityTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpDeleteAccountEntityTransfer cancels one account entity transfer by token.
func (c *Client) httpDeleteAccountEntityTransfer(ctx context.Context, token string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountEntityTransfers + "/" + url.PathEscape(token)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteAccountEntityTransfer", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
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
