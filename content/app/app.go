// Package app is the composition root of the content service running on its own. It is a
// package rather than a main so a test can build the routes without a listener.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/applicaset/blog/content"
	"github.com/applicaset/blog/content/httpapi"
	contentpostgres "github.com/applicaset/blog/content/postgres"
	contentsqlite "github.com/applicaset/blog/content/sqlite"
	"github.com/applicaset/pkg/config"
	"github.com/applicaset/pkg/serve"
	"github.com/applicaset/pkg/storage"
)

// schema is the Postgres schema this service owns. SQLite ignores it.
const schema = "content"

type Config struct {
	config.Server

	Log      config.Log
	Database storage.Config
}

func LoadConfig(ctx context.Context) (*Config, error) {
	log, err := config.LoadLog()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server:   config.LoadServer(),
		Log:      log,
		Database: storage.Load(),
	}

	if err := cfg.Validate(ctx); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate(ctx context.Context) error {
	if err := c.Server.Validate(ctx); err != nil {
		return err
	}

	return c.Database.Validate()
}

type Service struct {
	db     *sql.DB
	routes http.Handler
}

func New(ctx context.Context, cfg *Config, logger *slog.Logger) (*Service, error) {
	db, err := storage.Open(ctx, cfg.Database, schema)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	postRepo, err := newPostRepository(ctx, cfg.Database.Driver, db)
	if err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("build content repository: %w", err)
	}

	handler, err := httpapi.NewHandler(content.NewService(postRepo), logger)
	if err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("build content handler: %w", err)
	}

	mux := http.NewServeMux()
	handler.Register(mux)

	return &Service{db: db, routes: mux}, nil
}

func (svc *Service) Routes() http.Handler { return svc.routes }

func (svc *Service) Ping(ctx context.Context) error {
	if err := svc.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return nil
}

func (svc *Service) Close() error {
	if err := svc.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	return nil
}

func Run(ctx context.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := serve.NewLogger(cfg.Log)
	slog.SetDefault(logger)

	service, err := New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := service.Close(); err != nil {
			logger.ErrorContext(ctx, "close service", slog.Any("error", err))
		}
	}()

	return serve.Run(ctx, serve.Options{
		Name:            "content",
		Address:         cfg.Address(),
		ShutdownTimeout: cfg.ShutdownTimeout,
		Logger:          logger,
		Routes:          service.Routes(),
		Ready:           service.Ping,
		// No browser reaches this service, so there is no cross-origin form to protect. Callers are
		// sibling services, so their request identifier is kept.
		CrossOrigin:    false,
		TrustRequestID: true,
	})
}

func newPostRepository(
	ctx context.Context,
	driver string,
	db *sql.DB,
) (content.PostRepository, error) {
	if driver == storage.DriverPostgres {
		if err := contentpostgres.Migrate(ctx, db); err != nil {
			return nil, err
		}

		return contentpostgres.NewPostRepository(db), nil
	}

	if err := contentsqlite.Migrate(ctx, db); err != nil {
		return nil, err
	}

	return contentsqlite.NewPostRepository(db), nil
}
