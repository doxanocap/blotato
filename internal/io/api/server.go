package api

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echomiddle "github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"

	_ "github.com/doxanocap/blotato/docs"
	"github.com/doxanocap/blotato/internal/io/api/middleware"
	"github.com/doxanocap/blotato/internal/pkg/config"
)

type Server struct {
	srv  *echo.Echo
	cfg  config.App
	meta config.Meta
}

func NewServer(cfg config.App, meta config.Meta) *Server {
	s := &Server{
		srv:  echo.New(),
		cfg:  cfg,
		meta: meta,
	}
	s.srv.HideBanner = true
	s.srv.HidePort = true
	s.srv.HTTPErrorHandler = middleware.HandleError
	s.srv.Use(
		echomiddle.RequestID(),
		middleware.LogMW(),
		middleware.ErrorMW(),
		echomiddle.Recover(),
	)

	s.srv.Server = &http.Server{
		Addr:              cfg.Listen,
		Handler:           s.srv,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      55 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Swagger metadata is configured by the application before serving.
	s.srv.GET("/swagger/", func(ectx echo.Context) error {
		return ectx.Redirect(http.StatusTemporaryRedirect, "index.html")
	})
	s.srv.GET("/swagger/*", echoSwagger.WrapHandler)

	return s
}

func (s *Server) Serve() error {
	return s.srv.Server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Server.Shutdown(ctx)
}
