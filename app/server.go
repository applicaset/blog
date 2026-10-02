package app

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/applicaset/blog/web"
)

// Routes mounts every service handler. pkg/serve adds health probes and middleware, so this binary
// and the split ones answer them identically.
func Routes(cfg *Config, svc *services, logger *slog.Logger) (http.Handler, error) {
	mux := http.NewServeMux()

	authHandler := svc.authPages
	authHandler.Register(mux)

	site, err := web.New(web.Dependencies{
		Auth:    directAuth{service: svc.auth, pages: authHandler},
		Authz:   directAuthz{service: svc.authz},
		Content: directContent{service: svc.content},
	}, web.Config{
		SessionCookieName: cfg.SessionCookieName,
		SecureCookies:     cfg.SecureCookies,
		SiteTitle:         cfg.SiteTitle,
		BasePath:          cfg.BasePath,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("build web server: %w", err)
	}

	// The site takes every path the routes above did not claim.
	mux.Handle("/", site.Handler())

	return mux, nil
}
