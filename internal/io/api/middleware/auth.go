package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// Auth checks the scheme assigned to the route, never one chosen by the request.
func Auth(scheme, secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ectx echo.Context) error {
			var err error
			switch scheme {
			case "bearer":
				err = checkBearer(ectx, secret)
			case "signature":
				err = checkSignature(ectx, secret)
			default:
				return echo.NewHTTPError(http.StatusInternalServerError, "invalid authentication scheme")
			}
			if err != nil {
				return err
			}

			return next(ectx)
		}
	}
}

func checkBearer(ectx echo.Context, secret string) error {
	provided, ok := strings.CutPrefix(ectx.Request().Header.Get("Authorization"), "Bearer ")
	wantHash := sha256.Sum256([]byte(secret))
	gotHash := sha256.Sum256([]byte(provided))
	if secret == "" || !ok || subtle.ConstantTimeCompare(wantHash[:], gotHash[:]) != 1 {
		ectx.Response().Header().Set("WWW-Authenticate", "Bearer")

		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	return nil
}

func checkSignature(ectx echo.Context, secret string) error {
	if secret == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "webhook not configured")
	}

	request := ectx.Request()
	body, err := io.ReadAll(http.MaxBytesReader(ectx.Response(), request.Body, 1<<20))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	signature, ok := strings.CutPrefix(request.Header.Get("X-Hub-Signature-256"), "sha256=")
	provided, err := hex.DecodeString(signature)
	mac := hmac.New(sha256.New, []byte(secret))
	if _, writeErr := mac.Write(body); writeErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	if !ok || err != nil || !hmac.Equal(provided, mac.Sum(nil)) {
		return echo.NewHTTPError(http.StatusForbidden, "invalid signature")
	}

	request.Body = io.NopCloser(bytes.NewReader(body))

	return nil
}
