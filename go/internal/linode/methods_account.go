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
