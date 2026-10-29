package errs

import (
	"net/http"
	"strings"
)

// HTTPError exposes a public message while keeping causes out of JSON.
type HTTPError struct {
	StatusCode  int    `json:"-"`
	RequestID   string `json:"request_id,omitempty"`
	Message     string `json:"message"`
	privateErrs []error
}

func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}

func (e *HTTPError) Error() string { return e.Message }

func (e *HTTPError) Unwrap() []error { return e.privateErrs }

func (e *HTTPError) WithPrivateErrs(causes ...error) *HTTPError {
	e.privateErrs = append(e.privateErrs, causes...)
	return e
}

func (e *HTTPError) WithRequestID(id string) *HTTPError {
	copy := *e
	copy.RequestID = id
	return &copy
}

func ErrBadRequest(details ...string) *HTTPError {
	message := "invalid input"
	if len(details) != 0 {
		message = strings.Join(details, ", ")
	}
	return NewHTTPError(http.StatusBadRequest, message)
}
