package errs

import "errors"

var (
	ErrInvalidEvent    = errors.New("invalid event")
	ErrInvalidWorkflow = errors.New("invalid workflow")

	ErrRequestRejected    = errors.New("request rejected before sending")
	ErrUnexpectedResponse = errors.New("unexpected external response")
)
