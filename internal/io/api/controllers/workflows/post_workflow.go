package workflows

import (
	"net/http"

	"braces.dev/errtrace"
	"github.com/labstack/echo/v4"

	"github.com/doxanocap/blotato/internal/domain/entity/dto"
	"github.com/doxanocap/blotato/internal/domain/errs"
	"github.com/doxanocap/blotato/internal/io/api/controllers/models"
)

// PostWorkflow creates a saved comment-to-DM rule.
//
//	@Tags			workflows
//	@Summary		Создать правило обработки комментариев
//	@Description	В Instagram-режиме требуется Bearer BLOTATO_ADMIN_TOKEN; в fake авторизация не требуется.
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			body	body		postWorkflowReq	true	"Параметры правила"
//	@Success		201		{object}	models.Workflow
//	@Failure		400		{object}	errs.HTTPError
//	@Failure		401		{object}	errs.HTTPError
//	@Failure		413		{object}	errs.HTTPError
//	@Failure		500		{object}	errs.HTTPError
//	@Router			/api/v1/workflows [post]
func (c *Controller) PostWorkflow() echo.HandlerFunc {
	return func(ectx echo.Context) error {
		var req postWorkflowReq
		if err := ectx.Bind(&req); err != nil {
			return errs.ErrBadRequest("invalid input").WithPrivateErrs(errtrace.Wrap(err))
		}

		in := dto.PostWorkflowIn{
			Keyword:       req.Keyword,
			PublicReply:   req.PublicReply,
			EmailQuestion: req.EmailQuestion,
			Link:          req.Link,
		}

		workflow, err := c.service.CreateWorkflow(ectx.Request().Context(), in)
		if err != nil {
			return errtrace.Wrap(err)
		}

		return ectx.JSON(
			http.StatusCreated,
			new(models.Workflow).FromEntity(workflow),
		)
	}
}

type postWorkflowReq struct {
	Keyword       string `json:"keyword" example:"pricing"`
	PublicReply   string `json:"public_reply" example:"Sent you a DM!"`
	EmailQuestion string `json:"email_question" example:"What is your email address?"`
	Link          string `json:"link" example:"https://example.com/pricing"`
}
