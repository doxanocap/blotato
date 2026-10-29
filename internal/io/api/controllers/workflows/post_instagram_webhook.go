package workflows

import (
	"context"
	"net/http"
	"time"

	"braces.dev/errtrace"
	"github.com/labstack/echo/v4"

	"github.com/doxanocap/blotato/internal/domain/entity/dto"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
	"github.com/doxanocap/blotato/internal/domain/errs"
)

// PostInstagramWebhook acknowledges a batch after its relevant events are saved.
//
//	@Tags			instagram
//	@Summary		Принять события Instagram
//	@Description	Только Instagram-режим. HMAC-SHA256 проверяется по исходному телу. События сохраняются последовательно; при 503 повтор пакета безопасен благодаря дедупликации сервиса. Служебные события и эхо игнорируются.
//	@Accept			json
//	@Param			X-Hub-Signature-256	header	string					true	"sha256=<HMAC исходного тела>"
//	@Param			body				body	postInstagramWebhookReq	true	"Уведомление Meta"
//	@Success		200					"Все значимые события сохранены или уже получены"
//	@Failure		400					{object}	errs.HTTPError
//	@Failure		403					{object}	errs.HTTPError
//	@Failure		503					{object}	errs.HTTPError
//	@Router			/webhook/instagram [post]
func (c *Controller) PostInstagramWebhook() echo.HandlerFunc {
	return func(ectx echo.Context) error {
		if c.accountID == "" || c.execution == nil {
			return errs.NewHTTPError(http.StatusServiceUnavailable, "not configured")
		}

		var req postInstagramWebhookReq
		if err := ectx.Bind(&req); err != nil {
			return errs.ErrBadRequest("invalid notification").WithPrivateErrs(errtrace.Wrap(err))
		}

		if req.Object != "instagram" {
			return errs.ErrBadRequest("invalid notification")
		}

		ctx := ectx.Request().Context()
		for _, entry := range req.Entry {
			if entry.ID != c.accountID {
				continue
			}

			if entry.Field == "comments" {
				if err := c.receiveComment(ctx, entry.Value); err != nil {
					return errtrace.Wrap(err)
				}
			}

			for _, change := range entry.Changes {
				if change.Field == "comments" {
					if err := c.receiveComment(ctx, change.Value); err != nil {
						return errtrace.Wrap(err)
					}
				}
			}

			for _, m := range entry.Messaging {
				if err := c.receiveMessage(ctx, m); err != nil {
					return errtrace.Wrap(err)
				}
			}
		}

		return ectx.NoContent(http.StatusOK)
	}
}

type postInstagramWebhookReq struct {
	Object string         `json:"object"`
	Entry  []webhookEntry `json:"entry"`
}

type webhookEntry struct {
	ID        string           `json:"id"`
	Field     string           `json:"field"`
	Value     webhookComment   `json:"value"`
	Changes   []webhookChange  `json:"changes"`
	Messaging []webhookMessage `json:"messaging"`
}

type webhookChange struct {
	Field string         `json:"field"`
	Value webhookComment `json:"value"`
}

type webhookComment struct {
	ID        string `json:"id"`
	CommentID string `json:"comment_id"`
	From struct {
		ID string `json:"id"`
	} `json:"from"`
	Text *string `json:"text"`
}

type webhookMessage struct {
	Timestamp *int64 `json:"timestamp"`
	Sender    struct {
		ID string `json:"id"`
	} `json:"sender"`
	Recipient struct {
		ID string `json:"id"`
	} `json:"recipient"`
	IsSelf  bool `json:"is_self"`
	Message *struct {
		ID          string  `json:"mid"`
		Text        *string `json:"text"`
		Echo        bool    `json:"is_echo"`
		Self        bool    `json:"is_self"`
		Deleted     bool    `json:"is_deleted"`
		Unsupported bool    `json:"is_unsupported"`
	} `json:"message"`
}

func (c *Controller) receiveComment(ctx context.Context, comment webhookComment) error {
	if comment.From.ID == c.accountID {
		return nil
	}

	// Unified Login uses comment_id; Instagram Login notifications use id.
	commentID := comment.CommentID
	if commentID == "" {
		commentID = comment.ID
	}

	if commentID == "" || comment.From.ID == "" || comment.Text == nil {
		return errs.ErrBadRequest("incomplete comment")
	}

	event := dto.ReceiveEventIn{
		Platform:  "instagram",
		AccountID: c.accountID,
		ContactID: comment.From.ID,
		Kind:      types.Comment,
		EventID:   commentID,
		Text:      *comment.Text,
	}

	if err := c.execution.Receive(ctx, event); err != nil {
		return errs.NewHTTPError(http.StatusServiceUnavailable, "storage unavailable").WithPrivateErrs(
			errtrace.Wrap(err),
		)
	}

	return nil
}

func (c *Controller) receiveMessage(ctx context.Context, message webhookMessage) error {
	if message.Message == nil || message.IsSelf || message.Sender.ID == c.accountID {
		return nil
	}

	payload := message.Message
	if payload.Self || payload.Echo || payload.Deleted || payload.Unsupported || payload.Text == nil {
		return nil
	}

	if message.Sender.ID == "" || message.Recipient.ID != c.accountID || payload.ID == "" {
		return errs.ErrBadRequest("incomplete message")
	}

	event := dto.ReceiveEventIn{
		Platform:  "instagram",
		AccountID: c.accountID,
		ContactID: message.Sender.ID,
		Kind:      types.Message,
		EventID:   payload.ID,
		Text:      *payload.Text,
	}
	if message.Timestamp != nil {
		event.OccurredAt = time.UnixMilli(*message.Timestamp).UTC()
	}

	if err := c.execution.Receive(ctx, event); err != nil {
		return errs.NewHTTPError(http.StatusServiceUnavailable, "storage unavailable").WithPrivateErrs(
			errtrace.Wrap(err),
		)
	}

	return nil
}
