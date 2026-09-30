package app

import (
	"context"
	"log/slog"

	"github.com/applicaset/buildset/auth"
	"github.com/applicaset/buildset/auth/kit"
	"github.com/applicaset/buildset/pkg/config"
	"github.com/applicaset/buildset/pkg/storage"
	"github.com/nasermirzaei89/env"
)

// SessionCookieName is fixed rather than configurable, so the service that sets the cookie and the
// ones that read it cannot disagree.
const SessionCookieName = config.SessionCookieName

// Re-exported so a caller building a Config need not import pkg/storage.
const (
	DriverSQLite   = storage.DriverSQLite
	DriverPostgres = storage.DriverPostgres
)

type DatabaseConfig = storage.Config

type Config struct {
	config.Server

	LogLevel          slog.Level
	LogJSON           bool
	SessionCookieName string
	SecureCookies     bool
	SiteTitle         string
	BasePath          string
	Database          DatabaseConfig
	Auth              AuthConfig
	// Mailer replaces the SMTP sender Auth.Mail describes. Tests set it to read what was sent.
	Mailer auth.Mailer
}

// AuthConfig is shared with the identity binary, which reads the same variables.
type AuthConfig = kit.Config

func LoadConfig(ctx context.Context) (*Config, error) {
	log, err := config.LoadLog()
	if err != nil {
		return nil, err
	}

	cookie := config.LoadCookie()

	basePath, err := config.LoadBasePath()
	if err != nil {
		return nil, err
	}

	authConfig, err := kit.LoadConfig()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server:            config.LoadServer(),
		LogLevel:          log.Level,
		LogJSON:           log.JSON,
		SessionCookieName: cookie.Name,
		SecureCookies:     cookie.Secure,
		SiteTitle:         env.GetString("SITE_TITLE", "Blog"),
		BasePath:          basePath,
		Database:          storage.Load(),
		Auth:              authConfig,
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

	if err := c.Database.Validate(); err != nil {
		return err
	}

	return c.Auth.Validate()
}
