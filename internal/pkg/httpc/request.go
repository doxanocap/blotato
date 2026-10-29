package httpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RequestStartObserver func(*http.Request)
type RequestEndObserver func(*http.Request, *http.Response, error, time.Duration)

// Request is a single request builder, not safe for concurrent reuse.
type Request struct {
	client *Client
	method string
	path   string
	args   []any
	headers        http.Header
	queryParams    map[string]string
	responseModels map[int]any
	body           io.Reader
}

func (r *Request) SetMethod(method string) *Request {
	r.method = method
	return r
}

func (r *Request) SetPath(path string, args ...any) *Request {
	r.path = path
	r.args = args
	return r
}

func (r *Request) SetHeader(key, value string) *Request {
	r.headers.Set(key, value)
	return r
}

func (r *Request) SetRequestID(id string) *Request { return r.SetHeader("X-Request-ID", id) }

func (r *Request) SetContentType(value string) *Request { return r.SetHeader("Content-Type", value) }

func (r *Request) SetContentTypeJSON() *Request { return r.SetContentType("application/json") }

func (r *Request) SetAuthorization(value string) *Request { return r.SetHeader("Authorization", value) }

func (r *Request) SetBearerToken(value string) *Request { return r.SetAuthorization("Bearer " + value) }

func (r *Request) SetRequestBody(body io.Reader) *Request {
	r.body = body
	return r
}

func (r *Request) SetRequestBodyBuffer(body *bytes.Buffer) *Request {
	if body == nil {
		r.body = nil
	} else {
		r.body = bytes.NewReader(body.Bytes())
	}
	return r
}

func (r *Request) SetQueryParam(key, value string) *Request {
	r.queryParams[key] = value
	return r
}

func (r *Request) SetOffset(value int) *Request { return r.SetQueryParam("offset", strconv.Itoa(value)) }

func (r *Request) SetLimit(value int) *Request { return r.SetQueryParam("limit", strconv.Itoa(value)) }

func (r *Request) SetResponseModel(model any, statusCode int) *Request {
	r.responseModels[statusCode] = model
	return r
}

// Execute makes one Do call. It never retries or interprets non-2xx as errors.
// The caller owns every returned non-nil response.Body, including on decode
// errors. Without a matching model, the body remains unread. Models use bounded
// buffering and leave a readable replacement body after successful decoding.
func (r *Request) Execute(ctx context.Context) (response *http.Response, err error) {
	if r == nil || r.client == nil || r.client.httpClient == nil || ctx == nil {
		return nil, ErrInvalidBuild
	}
	switch r.method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
		http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace:
	default:
		return nil, ErrInvalidMethod
	}
	endpoint, err := r.buildURL()
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, r.method, endpoint.String(), r.body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	request.Header = r.headers.Clone()
	if request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if request.Header.Get("X-Request-ID") == "" {
		request.Header.Set("X-Request-ID", uuid.NewString())
	}

	start := time.Now()
	if r.client.onStartObserver != nil {
		r.client.onStartObserver(request)
	}
	defer func() {
		if r.client.onEndObserver != nil {
			r.client.onEndObserver(request, response, err, time.Since(start))
		}
	}()
	response, err = r.client.httpClient.Do(request)
	if err != nil {
		return response, fmt.Errorf("execute request: %w", err)
	}
	if model, ok := r.responseModels[response.StatusCode]; ok {
		err = r.decodeResponse(response, model)
	}
	return response, err
}

func (r *Request) decodeResponse(response *http.Response, model any) error {
	body, readErr := io.ReadAll(io.LimitReader(response.Body, r.client.responseBodyLimit+1))
	closeErr := response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(body))
	if err := errors.Join(readErr, closeErr); err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if int64(len(body)) > r.client.responseBodyLimit {
		return ErrResponseTooLarge
	}
	if err := json.Unmarshal(body, model); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	return nil
}

func (r *Request) buildURL() (*url.URL, error) {
	path := r.path
	if len(r.args) > 0 {
		path = fmt.Sprintf(path, flattenArgs(r.args)...)
	}
	relative, err := url.Parse(path)
	if err != nil || relative.IsAbs() || relative.Host != "" || relative.Fragment != "" {
		return nil, ErrInvalidURL
	}

	base := *r.client.url
	joined := strings.TrimRight(base.EscapedPath(), "/") + "/" + strings.TrimLeft(relative.EscapedPath(), "/")
	base.Path, err = url.PathUnescape(joined)
	if err != nil {
		return nil, ErrInvalidURL
	}
	base.RawPath = joined
	query := relative.Query()
	for key, value := range r.queryParams {
		query.Set(key, value)
	}
	base.RawQuery = query.Encode()
	return &base, nil
}

func flattenArgs(args []any) []any {
	var flat []any
	for _, arg := range args {
		switch values := arg.(type) {
		case []any:
			flat = append(flat, flattenArgs(values)...)
		case []int:
			for _, value := range values {
				flat = append(flat, value)
			}
		case []string:
			for _, value := range values {
				flat = append(flat, value)
			}
		default:
			flat = append(flat, arg)
		}
	}
	return flat
}
