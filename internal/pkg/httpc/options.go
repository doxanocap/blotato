package httpc

import (
	"net/http"
	"time"
)

type Option func(*Client)

// WithHTTPClient copies client settings without mutating the supplied client.
// Custom transports and observer callbacks must be safe for concurrent use.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient == nil {
			c.httpClient = nil
			return
		}
		copy := *httpClient
		if copy.Timeout == 0 && c.httpClient != nil {
			copy.Timeout = c.httpClient.Timeout
		}
		c.httpClient = &copy
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if c.httpClient != nil {
			c.httpClient.Timeout = timeout
		}
	}
}

func WithHeader(key, value string) Option {
	return func(c *Client) { c.headers[key] = value }
}

func WithTransport(transport http.RoundTripper) Option {
	return func(c *Client) {
		if c.httpClient != nil {
			c.httpClient.Transport = transport
		}
	}
}

// WithResponseBodyLimit bounds buffering by SetResponseModel. When no model
// matches, the response is streamed to the caller without being read here.
func WithResponseBodyLimit(limit int64) Option {
	return func(c *Client) { c.responseBodyLimit = limit }
}

func WithRequestObservers(start RequestStartObserver, end RequestEndObserver) Option {
	return func(c *Client) {
		c.onStartObserver = start
		c.onEndObserver = end
	}
}
