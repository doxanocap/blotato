package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"braces.dev/errtrace"

	"github.com/doxanocap/blotato/docs"
	"github.com/doxanocap/blotato/internal/app/worker"
	"github.com/doxanocap/blotato/internal/clients"
	"github.com/doxanocap/blotato/internal/clients/jev"
	"github.com/doxanocap/blotato/internal/domain/service/automation"
	"github.com/doxanocap/blotato/internal/domain/service/resolver"
	"github.com/doxanocap/blotato/internal/domain/service/workflow"
	workflowcontracts "github.com/doxanocap/blotato/internal/domain/service/workflow/contracts"
	"github.com/doxanocap/blotato/internal/io/api"
	"github.com/doxanocap/blotato/internal/io/api/controllers/workflows"
	controllercontracts "github.com/doxanocap/blotato/internal/io/api/controllers/workflows/contracts"
	"github.com/doxanocap/blotato/internal/pkg/config"
	sqlitepkg "github.com/doxanocap/blotato/internal/pkg/sqlite"
	"github.com/doxanocap/blotato/internal/repo"
)

type appConfig struct {
	App       config.App
	SQLite    config.SQLite
	Instagram config.Instagram
	Meta      config.Meta
	Jev       config.Jev
}

func (c appConfig) Validate() error {
	if err := c.App.Validate(); err != nil {
		return errtrace.Wrap(err)
	}

	if c.SQLite.Path == "" {
		return errtrace.New("database path is required")
	}
	if strings.TrimSpace(c.Jev.APIKey) == "" {
		return errtrace.New("OPENROUTER_API_KEY is required")
	}

	if c.App.Mode == "instagram" {
		if err := c.Instagram.Validate(); err != nil {
			return errtrace.Wrap(err)
		}

		if c.Meta.AppSecret == "" || c.Meta.VerifyToken == "" {
			return errtrace.New("Meta app secret and verify token are required")
		}
	}

	return nil
}

type App struct {
	pool      *sql.DB
	server    *api.Server
	scheduler *worker.Worker
}

func newApp(ctx context.Context, cfg appConfig) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errtrace.Wrap(err)
	}

	clientSet, err := clients.New(config.Clients{
		Mode:      cfg.App.Mode,
		Instagram: cfg.Instagram,
	})
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	pool, err := sqlitepkg.Open(ctx, cfg.SQLite.Path)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	storage := repo.New(pool)

	var instagramClient workflowcontracts.InstagramClient
	if sender := clientSet.Fake(); sender != nil {
		instagramClient = sender
	} else {
		instagramClient = clientSet.Instagram()
	}

	resolverService := resolver.NewService(storage.SQLite())
	automationService := automation.NewService(storage.SQLite())
	workflowService := workflow.NewService(
		storage.SQLite(),
		resolverService,
		instagramClient,
		cfg.Instagram.AccountID,
	)
	if cfg.Jev.Enabled && cfg.Jev.APIKey != "" {
		workflowService.SetEmailClassifier(jev.New(cfg.Jev.APIKey, ""))
	}
	if err := workflowService.RecoverSending(ctx); err != nil {
		return nil, errtrace.Wrap(errors.Join(err, pool.Close()))
	}

	var fakeSender controllercontracts.FakeSender
	if sender := clientSet.Fake(); sender != nil {
		fakeSender = sender
	}

	server := api.NewServer(cfg.App, cfg.Meta).SetupRoutes(
		workflows.New(
			automationService,
			workflowService,
			fakeSender,
			cfg.Instagram.AccountID,
		),
	)

	return &App{
		pool:   pool,
		server: server,
		scheduler: worker.NewWorker(
			200*time.Millisecond,
			[]string{worker.WorkflowJobName, worker.EmailDecisionJobName},
			worker.NewWorkflowJob(workflowService),
			worker.NewEmailDecisionJob(workflowService),
		),
	}, nil
}

func Run(ctx context.Context) (err error) {
	cfg, err := config.InitConfig[appConfig]()
	if err != nil {
		return errtrace.Wrap(err)
	}

	docs.SwaggerInfo.BasePath = cfg.App.SwaggerBasePath

	app, err := newApp(ctx, *cfg)
	if err != nil {
		return errtrace.Wrap(err)
	}
	defer func() {
		err = errors.Join(err, errtrace.Wrap(app.pool.Close()))
	}()

	return app.Serve(ctx)
}

func (a *App) Serve(ctx context.Context) error {
	workerCtx, stopWorker := context.WithCancel(ctx)
	defer stopWorker()
	a.scheduler.Serve(workerCtx)

	serverErr := make(chan error, 1)
	go func() {
		err := a.server.Serve()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		serverErr <- err
	}()

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-serverErr:
	}

	stopWorker()

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	shutdownErr := a.server.Shutdown(shutdownCtx)
	a.scheduler.WaitShutdown()

	return errtrace.Wrap(errors.Join(serveErr, shutdownErr))
}
