package entity

import "github.com/doxanocap/blotato/internal/domain/entity/types"

// SendResult holds a send outcome and its safe diagnostic reason.
type SendResult struct {
	outcome    types.SendOutcome
	externalID string
	errorCode  string
}

func NewSendResult() SendResult {
	return SendResult{
		outcome:   types.Unknown,
		errorCode: "unconfirmed_response",
	}
}

func (r *SendResult) Accept(externalID string) {
	if externalID == "" {
		r.MarkUnknown("unconfirmed_response")

		return
	}

	r.outcome = types.Accepted
	r.externalID = externalID
	r.errorCode = ""
}

func (r *SendResult) Reject(code string) {
	r.outcome = types.Rejected
	r.externalID = ""
	r.errorCode = code
}

func (r *SendResult) MarkUnknown(code string) {
	r.outcome = types.Unknown
	r.externalID = ""
	r.errorCode = code
}

func (r SendResult) Outcome() types.SendOutcome {
	if r.outcome == "" {
		return types.Unknown
	}

	return r.outcome
}

func (r SendResult) ExternalID() string {
	return r.externalID
}

// ErrorCode never exposes an arbitrary error message or payload.
func (r SendResult) ErrorCode() string {
	if r.Outcome() == types.Accepted {
		return ""
	}

	code := r.errorCode
	if code == "" {
		return string(r.Outcome())
	}

	if len(code) > 64 {
		return "unrecognized_adapter_error"
	}

	for _, char := range code {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return "unrecognized_adapter_error"
		}
	}

	return code
}
