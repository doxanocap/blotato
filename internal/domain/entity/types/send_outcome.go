package types

// SendOutcome describes platform acceptance, never delivery or reading.
type SendOutcome string

const (
	Accepted  SendOutcome = "accepted"
	Retryable SendOutcome = "retryable"
	Rejected  SendOutcome = "rejected"
	Unknown   SendOutcome = "unknown"
)

func (outcome SendOutcome) IsValid() bool {
	return outcome == Accepted || outcome == Retryable || outcome == Rejected || outcome == Unknown
}
