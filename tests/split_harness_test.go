package tests

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	authapp "github.com/applicaset/auth/app"
	authui "github.com/applicaset/auth/ui"
	authzapp "github.com/applicaset/authz/app"
	contentapp "github.com/applicaset/blog/content/app"
	webapp "github.com/applicaset/blog/web/app"
	"github.com/applicaset/blog/web/remote"
	"github.com/applicaset/pkg/config"
	"github.com/applicaset/pkg/mail"
	"github.com/applicaset/pkg/serve"
	"github.com/applicaset/pkg/storage"
	"github.com/stretchr/testify/require"
)

// blogPath is where the gateway mounts the blog, as the Caddyfile does.
const blogPath = "/blog"

// newSplitHarness boots the four services behind a proxy with the gateway's routing, so the browser
// sees one origin, as in compose. They share one SQLite file, unlike a deployment. That is enough:
// a split breaks at the boundaries between processes, not in storage.
func newSplitHarness(t *testing.T) *harness {
	t.Helper()

	browser := newBrowser(t)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	database := storage.Config{
		Driver: storage.DriverSQLite,
		Path:   filepath.Join(t.TempDir(), "test.db"),
	}

	authzURL := startAuthz(t, logger, database)
	contentURL := startContent(t, logger, database)
	authURL := startAuth(t, logger, database, authzURL)
	webURL := startWeb(t, logger, authURL, authzURL, contentURL)

	gateway := httptest.NewServer(newGateway(t, authURL, webURL))
	t.Cleanup(gateway.Close)

	return &harness{url: gateway.URL, site: blogPath, browser: browser}
}

// newGateway reproduces the Caddyfile: the identity paths to one service, the blog under its prefix
// with the prefix stripped, and the root redirected to the blog.
func newGateway(t *testing.T, authURL, webURL string) http.Handler {
	t.Helper()

	identity := newProxy(t, authURL)
	site := http.StripPrefix(blogPath, newProxy(t, webURL))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case slices.Contains(authui.Paths, r.URL.Path) ||
			slices.ContainsFunc(authui.Prefixes, func(prefix string) bool {
				return strings.HasPrefix(r.URL.Path, prefix)
			}):
			identity.ServeHTTP(w, r)
		case r.URL.Path == "/" || r.URL.Path == blogPath:
			http.Redirect(w, r, blogPath+"/", http.StatusFound)
		case strings.HasPrefix(r.URL.Path, blogPath+"/"):
			site.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

func newProxy(t *testing.T, target string) *httputil.ReverseProxy {
	t.Helper()

	parsed, err := url.Parse(target)
	require.NoError(t, err)

	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(parsed)
			// The original Host is preserved, because both services set cookies for it.
			r.Out.Host = r.In.Host
		},
	}
}

func startAuthz(t *testing.T, logger *slog.Logger, database storage.Config) string {
	t.Helper()

	service, err := authzapp.New(context.Background(), &authzapp.Config{
		Port: "8080", ShutdownTimeout: time.Second,
		Database: database,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = service.Close() })

	return startServer(
		t,
		serve.Options{Name: "authz", Logger: logger, Routes: service.Routes(), Ready: service.Ping},
	)
}

func startContent(t *testing.T, logger *slog.Logger, database storage.Config) string {
	t.Helper()

	service, err := contentapp.New(context.Background(), &contentapp.Config{
		Port: "8080", ShutdownTimeout: time.Second,
		Database: database,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = service.Close() })

	return startServer(
		t,
		serve.Options{
			Name:   "content",
			Logger: logger,
			Routes: service.Routes(),
			Ready:  service.Ping,
		},
	)
}

func startAuth(t *testing.T, logger *slog.Logger, database storage.Config, authzURL string) string {
	t.Helper()

	service, err := authapp.New(context.Background(), &authapp.Config{
		Port: "8080", ShutdownTimeout: time.Second,
		// The test server speaks plain HTTP, so a Secure cookie would never come back.
		Cookie:      config.Cookie{Name: config.SessionCookieName, Secure: false},
		Database:    database,
		Auth:        testAuthConfig(),
		Mailer:      &mail.Recorder{},
		AdminRole:   "admin",
		AuthzURL:    authzURL,
		HTTPTimeout: 5 * time.Second,
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = service.Close() })

	return startServer(t, serve.Options{
		Name:        "auth",
		Logger:      logger,
		Routes:      service.Routes(),
		Ready:       service.Ping,
		CrossOrigin: true,
	})
}

func startWeb(t *testing.T, logger *slog.Logger, authURL, authzURL, contentURL string) string {
	t.Helper()

	site, err := webapp.New(&webapp.Config{
		Port: "8080", ShutdownTimeout: time.Second,
		Cookie:      config.Cookie{Name: config.SessionCookieName, Secure: false},
		SiteTitle:   "Test Blog",
		BasePath:    blogPath,
		AuthURL:     authURL,
		AuthzURL:    authzURL,
		ContentURL:  contentURL,
		HTTPTimeout: 5 * time.Second,
		URLs:        remote.DefaultURLs(),
	}, logger)
	require.NoError(t, err)

	return startServer(t, serve.Options{
		Name: "web", Logger: logger, Routes: site.Routes(), CrossOrigin: true,
	})
}

// startServer serves one binary's routes through the same assembly its own main would use, so the
// health probes and the middleware are the real ones.
func startServer(t *testing.T, options serve.Options) string {
	t.Helper()

	server := httptest.NewServer(serve.Handler(options))
	t.Cleanup(server.Close)

	return server.URL
}
