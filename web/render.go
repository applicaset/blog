package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/buildset/buildset/pkg/asset"
)

//go:embed templates/*.gohtml templates/icons/*.svg
var templateFiles embed.FS

//go:embed static/style.min.css
var stylesheetContent []byte

var stylesheet = asset.New("/static/style.min.css", "text/css; charset=utf-8", stylesheetContent)

// Listing every page keeps a typo in a handler a startup problem rather than a runtime one.
var pageNames = []string{
	"index.gohtml",
	"post.gohtml",
	"account.gohtml",
	"error.gohtml",
	"admin_dashboard.gohtml",
	"admin_posts.gohtml",
	"admin_post_form.gohtml",
	"admin_users.gohtml",
	"admin_user.gohtml",
	"confirm.gohtml",
}

// confirmContent asks before an action that cannot be undone.
type confirmContent struct {
	Message   string
	ActionURL string
	Submit    string
	CancelURL string
}

// layoutData is what every template receives. Handlers fill Content with the page's own data.
type layoutData struct {
	SiteTitle     string
	Title         string
	StylesheetURL string
	CurrentUser   *User
	LoginURL      string
	LogoutURL     string
	PasswordURL   string
	CanSeeAdmin   bool
	AdminTabs     []adminTab
	ErrorMessage  string
	Notice        string
	Content       any
}

// templateFunctions are the few helpers the pages need that html/template does not provide.
func templateFunctions(basePath string) template.FuncMap {
	return template.FuncMap{
		"has":             slices.Contains[[]string, string],
		"transitionLabel": func(status string) string { return transitionLabels[status] },
		"path":            func(p string) string { return basePath + p },
	}
}

func parseTemplates(basePath string) (map[string]*template.Template, error) {
	pages := make(map[string]*template.Template, len(pageNames))

	for _, name := range pageNames {
		page, err := template.New(name).
			Funcs(templateFunctions(basePath)).
			ParseFS(templateFiles, "templates/layout.gohtml", "templates/icons/*.svg", "templates/"+name)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}

		pages[name] = page
	}

	return pages, nil
}

// newLayoutData fills in what the layout needs on every page, so no handler has to remember it.
func (s *Server) newLayoutData(r *http.Request, title string) layoutData {
	user, _ := userFromContext(r.Context())

	data := layoutData{
		SiteTitle:     s.config.SiteTitle,
		Title:         title,
		StylesheetURL: s.path(stylesheet.URL),
		CurrentUser:   user,
		LoginURL:      s.deps.Auth.LoginURL(s.currentURL(r)),
		LogoutURL:     s.deps.Auth.LogoutURL(s.path("/")),
		PasswordURL:   s.deps.Auth.PasswordURL(s.currentURL(r)),
	}

	if user != nil {
		// A failure to answer hides the administration link rather than blocking the page.
		allowed, err := s.can(r.Context(), user, ActionPostCreate, anyPostResource)
		if err != nil {
			s.logger.WarnContext(r.Context(), "check admin visibility", slog.Any("error", err))
		}

		data.CanSeeAdmin = allowed

		if strings.HasPrefix(r.URL.Path, adminPathPrefix) {
			data.AdminTabs = s.adminTabs(r, user, allowed)
		}
	}

	return data
}

// The template runs into a buffer first, so a failure becomes an error page rather than a truncated
// one already sent with a success status.
func (s *Server) render(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	page string,
	data layoutData,
) {
	tmpl, ok := s.templates[page]
	if !ok {
		s.logger.ErrorContext(r.Context(), "unknown template", slog.String("page", page))
		http.Error(w, "internal server error", http.StatusInternalServerError)

		return
	}

	var buf bytes.Buffer

	if err := tmpl.ExecuteTemplate(&buf, "layout.gohtml", data); err != nil {
		s.logger.ErrorContext(
			r.Context(),
			"execute template",
			slog.String("page", page),
			slog.Any("error", err),
		)
		http.Error(w, "internal server error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	data := s.newLayoutData(r, http.StatusText(status))
	data.Content = message

	s.render(w, r, status, "error.gohtml", data)
}

// renderInternalError puts the cause in the log and keeps it out of the response.
func (s *Server) renderInternalError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
	message string,
) {
	s.logger.ErrorContext(r.Context(), message, slog.Any("error", err))
	s.renderError(w, r, http.StatusInternalServerError, "Something went wrong. Please try again.")
}
