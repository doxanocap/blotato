package workflows

import (
	"crypto/subtle"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/doxanocap/blotato/internal/domain/errs"
)

// GetInstagramWebhook returns the verification challenge as plain text.
//
//	@Tags			instagram
//	@Summary		Подтвердить подключение webhook
//	@Description	Только Instagram-режим. Проверка challenge не подтверждает доставку реальных событий.
//	@Produce		plain
//	@Param			hub.mode			query		string	true	"subscribe"
//	@Param			hub.verify_token	query		string	true	"Verify token, настроенный на сервере"
//	@Param			hub.challenge		query		string	true	"Challenge Meta"
//	@Success		200					{string}	string	"Исходный challenge"
//	@Failure		403					{object}	errs.HTTPError
//	@Failure		503					{object}	errs.HTTPError
//	@Router			/webhook/instagram [get]
func (c *Controller) GetInstagramWebhook(verifyToken string) echo.HandlerFunc {
	return func(ectx echo.Context) error {
		if verifyToken == "" {
			return errs.NewHTTPError(http.StatusServiceUnavailable, "webhook not configured")
		}

		query := ectx.QueryParams()
		provided := query.Get("hub.verify_token")
		if query.Get("hub.mode") != "subscribe" ||
			query.Get("hub.challenge") == "" ||
			subtle.ConstantTimeCompare([]byte(provided), []byte(verifyToken)) != 1 {
			return errs.NewHTTPError(http.StatusForbidden, "verification failed")
		}

		return ectx.String(http.StatusOK, query.Get("hub.challenge"))
	}
}
