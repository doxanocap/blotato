package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
)

// LogMW records routing diagnostics without headers, query values or bodies.
func LogMW() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ectx echo.Context) error {
			start := time.Now()
			err := next(ectx)

			attrs := []any{
				"method", ectx.Request().Method,
				"path", ectx.Path(),
				"status", ectx.Response().Status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", ectx.Response().Header().Get(echo.HeaderXRequestID),
			}

			if err != nil {
				slog.ErrorContext(ectx.Request().Context(), "http request", attrs...)
			} else {
				slog.InfoContext(ectx.Request().Context(), "http request", attrs...)
			}

			return err
		}
	}
}
