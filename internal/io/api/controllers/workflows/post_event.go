package workflows

import (
	"net/http"
	"time"

	"braces.dev/errtrace"
	"github.com/labstack/echo/v4"

	"github.com/doxanocap/blotato/internal/domain/entity/dto"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
	"github.com/doxanocap/blotato/internal/domain/errs"
)

// PostEvent accepts a local event without contacting Instagram.
//
//	@Tags			fake
//	@Summary		Принять тестовое событие
//	@Description	Только fake-режим. Ответ 202 возвращается после сохранения; повтор известного события безопасен. Для отправки ссылки нужен occurred_at сообщения.
//	@Accept			json
//	@Param			body	body	postEventReq	true	"Входящее событие"
//	@Success		202		"Событие сохранено или уже получено"
//	@Failure		400		{object}	errs.HTTPError
//	@Failure		413		{object}	errs.HTTPError
//	@Failure		500		{object}	errs.HTTPError
//	@Router			/events [post]
func (c *Controller) PostEvent() echo.HandlerFunc {
	return func(ectx echo.Context) error {
		var req postEventReq
		if err := ectx.Bind(&req); err != nil {
			return errs.ErrBadRequest("invalid input").WithPrivateErrs(errtrace.Wrap(err))
		}

		in := dto.ReceiveEventIn{
			Platform:   req.Platform,
			AccountID:  req.AccountID,
			ContactID:  req.ContactID,
			Kind:       req.Kind,
			EventID:    req.EventID,
			Text:       req.Text,
			OccurredAt: req.OccurredAt,
		}

		if err := c.execution.Receive(ectx.Request().Context(), in); err != nil {
			return errtrace.Wrap(err)
		}

		return ectx.NoContent(http.StatusAccepted)
	}
}

type postEventReq struct {
	Platform   string          `json:"platform" example:"fake"`
	AccountID  string          `json:"account_id" example:"brand"`
	ContactID  string          `json:"contact_id" example:"person"`
	Kind       types.EventType `json:"kind" enums:"comment,message"`
	EventID    string          `json:"event_id" example:"c1"`
	Text       string          `json:"text" example:"pricing"`
	OccurredAt time.Time       `json:"occurred_at"`
}
