package contracts

import (
	"context"
	"time"

	"github.com/doxanocap/blotato/internal/clients/instagram"
	"github.com/doxanocap/blotato/internal/domain/entity"
	"github.com/doxanocap/blotato/internal/domain/entity/dto"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
)

// Repo provides persistence consumed by the workflow service.
type Repo interface {
	Tx(ctx context.Context, fn func(context.Context) error) error

	// inbox
	CreateInboxEvent(ctx context.Context, in dto.ReceiveEventIn, receivedAt time.Time) (types.ID, bool, error)
	DeferInboxEvent(ctx context.Context, eventID, runID types.ID) error
	ProcessInboxEvent(ctx context.Context, eventID types.ID) error
	GetDeferredInboxMessages(ctx context.Context, runID types.ID) ([]entity.InboxMessage, error)
	QueueEmailDecision(ctx context.Context, eventID, runID types.ID) error
	NextEmailDecision(ctx context.Context) (entity.InboxMessage, types.ID, bool, error)
	FinishEmailDecision(ctx context.Context, eventID types.ID, decision, model, version string, confidence float64) (bool, error)
	ApplyEmailDecision(ctx context.Context, eventID, runID types.ID, email string, now time.Time) error

	// runs
	CreateRun(ctx context.Context, run entity.Run) (types.ID, bool, error)
	GetActiveRun(ctx context.Context, accountID, contactID string) (entity.Run, bool, error)
	UpdateRunEmail(ctx context.Context, runID types.ID, email string, occurredAt time.Time) error
	UpdateRunState(ctx context.Context, runID types.ID, state types.RunState) error
	FailRun(ctx context.Context, runID types.ID, state types.RunState, code string) error
	SetRunWaitingEmail(ctx context.Context, runID types.ID, now time.Time) error
	GetRunDeadline(ctx context.Context, runID types.ID) (time.Time, error)
	ExpireRuns(ctx context.Context, now time.Time) error
	GetRuns(ctx context.Context) ([]dto.RunStatus, error)

	// actions
	CreateAction(ctx context.Context, runID types.ID, step types.ActionStep) error
	IsQuestionActionSending(ctx context.Context, runID types.ID) (bool, error)
	RecoverSendingActions(ctx context.Context) error
	ClaimAction(ctx context.Context) (dto.SendActionIn, bool, error)
	GetSendingActionRun(ctx context.Context, actionID string) (types.ID, error)
	FinishAction(
		ctx context.Context,
		actionID string,
		result entity.SendResult,
	) error
}

type Resolver interface {
	GetWorkflowForComment(
		ctx context.Context,
		text string,
	) (entity.Workflow, bool, error)
}

type InstagramClient interface {
	ReplyComment(
		ctx context.Context,
		commentID string,
		text string,
	) (string, error)

	PrivateReply(
		ctx context.Context,
		accountID string,
		commentID string,
		contactID string,
		text string,
	) (instagram.Message, error)

	SendMessage(
		ctx context.Context,
		accountID string,
		contactID string,
		text string,
	) (instagram.Message, error)
}
