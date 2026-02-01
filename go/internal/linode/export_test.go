package linode

import "time"

// ClientFields returns the internal fields of a Client for testing.
func (c *Client) ClientFields() (baseURL, token string, hasHTTPClient bool) {
	return c.baseURL, c.token, c.httpClient != nil
}

// ShouldRetry exposes the shouldRetry method for testing.
func (rc *RetryableClient) ShouldRetry(err error) bool {
	return rc.shouldRetry(err)
}

// CalculateDelay exposes the calculateDelay method for testing.
func (rc *RetryableClient) CalculateDelay(attempt int) time.Duration {
	return rc.calculateDelay(attempt)
}

// RetryConfigField returns the retry config for testing.
func (rc *RetryableClient) RetryConfigField() RetryConfig {
	return rc.retryConfig
}
