package linode

import (
	"context"
	"net/http"
	"net/url" // path parameter escaping
	"strconv"
)

const (
	endpointProfile                = "/profile"
	endpointProfileGrants          = "/profile/grants"
	endpointAccount                = "/account"
	endpointAccountCancel          = "/account/cancel"
	endpointAccountAgreements      = "/account/agreements"
	endpointAccountMaintenance     = "/account/maintenance"
	endpointAccountNotifications   = "/account/notifications"
	endpointAccountAvailability    = "/account/availability"
	endpointAccountBetas           = "/account/betas"
	endpointAccountOAuthClients    = "/account/oauth-clients"
	endpointAccountEvents          = "/account/events"
	endpointAccountLogins          = "/account/logins"
	endpointAccountInvoices        = "/account/invoices"
	endpointAccountChildAccounts   = "/account/child-accounts"
	endpointAccountEntityTransfers = "/account/entity-transfers"
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
