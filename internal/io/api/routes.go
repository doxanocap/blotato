package api

import (
	echomiddle "github.com/labstack/echo/v4/middleware"

	"github.com/doxanocap/blotato/internal/io/api/controllers/workflows"
	"github.com/doxanocap/blotato/internal/io/api/middleware"
)

func (s *Server) SetupRoutes(controller workflows.Controller) *Server {
	v1 := s.srv.Group("/api/v1")
	w := v1.Group("/workflows")
	if s.cfg.Mode == "instagram" {
		w.Use(middleware.Auth("bearer", s.cfg.AdminToken))
	}

	w.POST(
		"",
		controller.PostWorkflow(),
		echomiddle.BodyLimit("8K"),
	)
	w.GET("", controller.GetWorkflows())
	w.GET("/runs", controller.GetRuns())

	if s.cfg.Mode == "fake" {
		s.srv.POST(
			"/events",
			controller.PostEvent(),
			echomiddle.BodyLimit("8K"),
		)
		s.srv.POST(
			"/fake/outcome",
			controller.PostFakeOutcome(),
			echomiddle.BodyLimit("1K"),
		)
	} else {
		s.srv.GET(
			"/webhook/instagram",
			controller.GetInstagramWebhook(s.meta.VerifyToken),
		)
		s.srv.POST(
			"/webhook/instagram",
			controller.PostInstagramWebhook(),
			middleware.Auth("signature", s.meta.AppSecret),
		)
	}

	return s
}
