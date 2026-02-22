package linode

import "time"

// ClientFields returns the internal fields of a Client for testing.
func (c *Client) ClientFields() (string, string, bool) {
	return c.baseURL, c.token, c.httpClient != nil
}

// ExportedShouldRetry exposes the shouldRetry method for testing.
func (rc *RetryableClient) ExportedShouldRetry(err error) bool {
	return rc.shouldRetry(err)
}

// ExportedCalculateDelay exposes the calculateDelay method for testing.
func (rc *RetryableClient) ExportedCalculateDelay(attempt int) time.Duration {
	return rc.calculateDelay(attempt)
}

// RetryConfigField returns the retry config for testing.
func (rc *RetryableClient) RetryConfigField() RetryConfig {
	return rc.retryConfig
}
