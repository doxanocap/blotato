package workflows

import (
	"net/http"

	"braces.dev/errtrace"
	"github.com/labstack/echo/v4"

	"github.com/doxanocap/blotato/internal/io/api/controllers/models"
)

// GetWorkflows lists saved rules in the order used to match comments.
//
//	@Tags			workflows
//	@Summary		Получить правила
//	@Description	В Instagram-режиме требуется Bearer BLOTATO_ADMIN_TOKEN; в fake авторизация не требуется.
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{array}		models.Workflow
//	@Failure		401	{object}	errs.HTTPError
//	@Failure		500	{object}	errs.HTTPError
//	@Router			/api/v1/workflows [get]
func (c *Controller) GetWorkflows() echo.HandlerFunc {
	return func(ectx echo.Context) error {
		workflows, err := c.service.GetWorkflows(ectx.Request().Context())
		if err != nil {
			return errtrace.Wrap(err)
		}

		response := make([]models.Workflow, len(workflows))
		for i, workflow := range workflows {
			response[i] = new(models.Workflow).FromEntity(workflow)
		}

		return ectx.JSON(http.StatusOK, response)
	}
}
