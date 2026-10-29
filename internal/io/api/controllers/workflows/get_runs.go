package workflows

import (
	"net/http"

	"braces.dev/errtrace"
	"github.com/labstack/echo/v4"

	"github.com/doxanocap/blotato/internal/io/api/controllers/models"
)

// GetRuns returns the run state together with its associated outgoing actions.
//
//	@Tags			workflows
//	@Summary		Получить состояния выполнений
//	@Description	Каждая строка содержит выполнение и одно его действие. В Instagram-режиме требуется Bearer BLOTATO_ADMIN_TOKEN; в fake авторизация не требуется.
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{array}		models.Run
//	@Failure		401	{object}	errs.HTTPError
//	@Failure		500	{object}	errs.HTTPError
//	@Router			/api/v1/workflows/runs [get]
func (c *Controller) GetRuns() echo.HandlerFunc {
	return func(ectx echo.Context) error {
		runs, err := c.execution.GetRuns(ectx.Request().Context())
		if err != nil {
			return errtrace.Wrap(err)
		}

		response := make([]models.Run, len(runs))
		for i, run := range runs {
			response[i] = new(models.Run).FromDTO(run)
		}

		return ectx.JSON(http.StatusOK, response)
	}
}
