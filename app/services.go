package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/applicaset/buildset/auth"
	authbackend "github.com/applicaset/buildset/auth/backend"
	"github.com/applicaset/buildset/auth/kit"
	authui "github.com/applicaset/buildset/auth/ui"
	"github.com/applicaset/buildset/authz"
	authzbackend "github.com/applicaset/buildset/authz/backend"
	"github.com/applicaset/buildset/blog/content"
	contentpostgres "github.com/applicaset/buildset/blog/content/postgres"
	contentsqlite "github.com/applicaset/buildset/blog/content/sqlite"
	"github.com/applicaset/buildset/blog/web"
	"github.com/applicaset/buildset/pkg/storage"
)

// administratorRole is the application's vocabulary; authz only stores the string.
const administratorRole = "admin"

// services holds every service this binary runs. Each is built from its own backend and never sees
// the others' packages.
type services struct {
	auth      *auth.Service
	authPages *authui.Handler
	authz     *authz.Service
	content   *content.Service
}

func newServices(
	ctx context.Context,
	cfg *Config,
	stores *Stores,
	logger *slog.Logger,
) (*services, error) {
	authRepos, err := authbackend.New(ctx, cfg.Database.Driver, &storage.Handle{SQL: stores.Auth})
	if err != nil {
		return nil, fmt.Errorf("build auth repositories: %w", err)
	}

	authzRepos, err := authzbackend.New(
		ctx,
		cfg.Database.Driver,
		&storage.Handle{SQL: stores.Authz},
	)
	if err != nil {
		return nil, fmt.Errorf("build authz repositories: %w", err)
	}

	authzService := authz.NewService(authzRepos.Role, authzRepos.SubjectRole, authzRepos.Grant)

	// The hook lets auth create the first administrator without knowing that authz exists.
	firstUserHook := auth.FirstUserHookFunc(func(ctx context.Context, userRef string) error {
		if err := authzService.AssignRole(ctx, userRef, administratorRole); err != nil {
			return fmt.Errorf("assign %s role to %s: %w", administratorRole, userRef, err)
		}

		logger.InfoContext(
			ctx,
			"first user is now an administrator",
			slog.String("user_ref", userRef),
		)

		return nil
	})

	registrationPolicy := authui.SwitchPolicy{
		Open: cfg.Auth.RegistrationOpen,
		CanAddUser: func(ctx context.Context, actorRef string) (bool, error) {
			return authzService.Can(ctx, actorRef, web.ActionUserCreate, web.AnyUserResource)
		},
	}

	authService, authPages, err := kit.Build(kit.Options{
		Config:        cfg.Auth,
		UserRepo:      authRepos.User,
		SessionRepo:   authRepos.Session,
		TokenRepo:     authRepos.Token,
		IdentityRepo:  authRepos.Identity,
		FirstUserHook: firstUserHook,
		Registration:  registrationPolicy,
		Cookie: authui.Config{
			SessionCookieName: cfg.SessionCookieName,
			SecureCookies:     cfg.SecureCookies,
		},
		Mailer: cfg.Mailer,
		Logger: logger,
	})
	if err != nil {
		return nil, err
	}

	postRepo, err := newPostRepository(ctx, cfg.Database.Driver, stores.Content)
	if err != nil {
		return nil, fmt.Errorf("build content repository: %w", err)
	}

	return &services{
		auth:      authService,
		authPages: authPages,
		authz:     authzService,
		content:   content.NewService(postRepo),
	}, nil
}

// The factory below is the only place that names a storage backend.

func newPostRepository(
	ctx context.Context,
	driver string,
	db *sql.DB,
) (content.PostRepository, error) {
	if driver == DriverPostgres {
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
