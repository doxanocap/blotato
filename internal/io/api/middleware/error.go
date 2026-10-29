package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/doxanocap/blotato/internal/domain/errs"
)

// ErrorMW writes public errors once and lets outer middleware observe failure.
func ErrorMW() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ectx echo.Context) error {
			err := next(ectx)
			if err != nil {
				HandleError(err, ectx)
			}

			return err
		}
	}
}

// HandleError also covers routing failures that bypass route middleware.
func HandleError(err error, ectx echo.Context) {
	if ectx.Response().Committed {
		return
	}

	var (
		status    = http.StatusInternalServerError
		message   = "internal server error"
		httpErr   *echo.HTTPError
		domainErr *errs.HTTPError
	)

	switch {
	case errors.As(err, &domainErr):
		status = domainErr.StatusCode
		message = domainErr.Message
	case errors.As(err, &httpErr):
		status = httpErr.Code
		message = http.StatusText(status)
		if text, ok := httpErr.Message.(string); ok {
			message = text
		}
	}

	response := errs.HTTPError{
		Message:   message,
		RequestID: ectx.Response().Header().Get(echo.HeaderXRequestID),
	}
	if writeErr := ectx.JSON(status, response); writeErr != nil {
		ectx.Logger().Error("write error response failed")
	}
}
