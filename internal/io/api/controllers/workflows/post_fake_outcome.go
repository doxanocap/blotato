package workflows

import (
	"net/http"

	"braces.dev/errtrace"
	"github.com/labstack/echo/v4"

	"github.com/doxanocap/blotato/internal/domain/entity/types"
	"github.com/doxanocap/blotato/internal/domain/errs"
)

// PostFakeOutcome configures the local sender without making a platform call.
//
//	@Tags			fake
//	@Summary		Задать исход тестовой отправки
//	@Description	Только fake-режим. Не отправляет сообщения в Instagram.
//	@Accept			json
//	@Param			body	body	postFakeOutcomeReq	true	"Шаг и исход"
//	@Success		204		"Исход установлен"
//	@Failure		400		{object}	errs.HTTPError
//	@Failure		413		{object}	errs.HTTPError
//	@Router			/fake/outcome [post]
func (c *Controller) PostFakeOutcome() echo.HandlerFunc {
	return func(ectx echo.Context) error {
		var req postFakeOutcomeReq
		if err := ectx.Bind(&req); err != nil {
			return errs.ErrBadRequest("invalid input").WithPrivateErrs(errtrace.Wrap(err))
		}

		if !req.Step.IsValid() || !req.Outcome.IsValid() {
			return errs.ErrBadRequest("invalid outcome")
		}

		c.sender.Set(req.Step, req.Outcome)

		return ectx.NoContent(http.StatusNoContent)
	}
}

type postFakeOutcomeReq struct {
	Step    types.ActionStep  `json:"step" enums:"reply_comment,ask_email,send_link"`
	Outcome types.SendOutcome `json:"outcome" enums:"accepted,retryable,rejected,unknown"`
}
