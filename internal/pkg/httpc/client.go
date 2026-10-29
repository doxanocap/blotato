// Package httpc provides the request-building API used by service clients.
// Its layout and fluent methods follow employee-education's httpc package.
// It has no WB metrics dependency, implicit body logging or application retry.
package httpc

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultTimeout = 15 * time.Second
const defaultResponseBodyLimit int64 = 1 << 20

var (
	ErrInvalidBuild     = errors.New("invalid request build")
	ErrInvalidURL       = errors.New("invalid URL")
	ErrInvalidMethod    = errors.New("invalid HTTP method")
	ErrResponseTooLarge = errors.New("response body exceeds limit")
)

// Client is configured once. Independent Request builders may use it concurrently.
type Client struct {
	httpClient        *http.Client
	url               *url.URL
	headers           map[string]string
	responseBodyLimit int64
	onStartObserver   RequestStartObserver
	onEndObserver     RequestEndObserver
}

// New validates configuration. Redirects are disabled unless an injected
// HTTP client explicitly supplies its own CheckRedirect policy. TLS uses the
// standard verified transport. Authentication and bodies are never logged.
func New(host string, opts ...Option) (*Client, error) {
	base, err := url.Parse(host)
	if err != nil || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" ||
		(base.Scheme != "http" && base.Scheme != "https") {
		return nil, ErrInvalidURL
	}

	client := &Client{
		httpClient:        &http.Client{Timeout: defaultTimeout},
		url:               base,
		headers:           make(map[string]string),
		responseBodyLimit: defaultResponseBodyLimit,
	}
	for _, option := range opts {
		if option != nil {
			option(client)
		}
	}
	if client.httpClient == nil || client.httpClient.Timeout <= 0 || client.responseBodyLimit <= 0 || client.responseBodyLimit == 1<<63-1 {
		return nil, fmt.Errorf("%w: HTTP client, positive timeout and body limit are required", ErrInvalidBuild)
	}
	if client.httpClient.CheckRedirect == nil {
		client.httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return client, nil
}

// MustNew is the fail-fast configuration constructor used by the reference
// services. It panics on invalid configuration; request failures return errors.
func MustNew(host string, opts ...Option) *Client {
	client, err := New(host, opts...)
	if err != nil {
		panic(err)
	}
	return client
}

func (c *Client) Get(path string, args ...any) *Request {
	return c.NewRequest().SetMethod(http.MethodGet).SetPath(path, args...)
}

func (c *Client) Post(path string, args ...any) *Request {
	return c.NewRequest().SetMethod(http.MethodPost).SetPath(path, args...)
}

func (c *Client) Put(path string, args ...any) *Request {
	return c.NewRequest().SetMethod(http.MethodPut).SetPath(path, args...)
}

func (c *Client) Patch(path string, args ...any) *Request {
	return c.NewRequest().SetMethod(http.MethodPatch).SetPath(path, args...)
}

func (c *Client) Delete(path string, args ...any) *Request {
	return c.NewRequest().SetMethod(http.MethodDelete).SetPath(path, args...)
}

// NewRequest creates independent request state, including default headers.
func (c *Client) NewRequest() *Request {
	request := &Request{
		client:         c,
		headers:        make(http.Header),
		queryParams:    make(map[string]string),
		responseModels: make(map[int]any),
	}
	if c != nil {
		for key, value := range c.headers {
			request.headers.Set(key, value)
		}
	}
	return request
}
