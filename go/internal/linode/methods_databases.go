package linode

import (
	"context"
	"net/http"
	"net/url"
)

const endpointDatabaseEngines = "/databases/engines"

// ListDatabaseEngines retrieves available Managed Database engines.
func (c *Client) httpListDatabaseEngines(ctx context.Context, page, pageSize int) ([]DatabaseEngine, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointDatabaseEngines, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDatabaseEngines", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[DatabaseEngine]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetDatabaseEngine retrieves a Managed Database engine by ID.
func (c *Client) httpGetDatabaseEngine(ctx context.Context, engineID string) (*DatabaseEngine, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseEngines + "/" + url.PathEscape(engineID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabaseEngine", Err: err}
	}

	defer drainClose(resp)

	var engine DatabaseEngine
	if err := c.handleResponse(resp, &engine); err != nil {
		return nil, err
	}

	return &engine, nil
}
