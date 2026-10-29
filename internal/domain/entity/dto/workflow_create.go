package dto

// PostWorkflowIn contains the rule fields accepted by the service.
type PostWorkflowIn struct {
	Keyword       string
	PublicReply   string
	EmailQuestion string
	Link          string
}
