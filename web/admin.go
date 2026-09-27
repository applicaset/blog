package web

import (
	"log/slog"
	"net/http"
	"strings"
)

const (
	adminPathPrefix  = "/admin/"
	dashboardPath    = "/admin/dashboard"
	recentPostsShown = 5
)

type adminTab struct {
	Label string
	// Path is the section's route here, and URL the link to it as the browser sees it.
	Path   string
	URL    string
	Active bool
}

type dashboardContent struct {
	CanManagePosts bool
	CanManageUsers bool
	// ShowingEveryone tells an author that the numbers are theirs rather than the whole site's.
	ShowingEveryone bool
	Published       int
	Drafts          int
	Archived        int
	RecentPosts     []Post
	Users           int
}

func (s *Server) adminIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.path(dashboardPath), http.StatusFound)
}

// adminDashboard is reachable by anyone who can do something on one of the other admin pages, so
// an author sees it without being an administrator.
func (s *Server) adminDashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	canManagePosts, err := s.can(r.Context(), user, ActionPostCreate, anyPostResource)
	if err != nil {
		s.renderInternalError(w, r, err, "check post permission")

		return
	}

	canManageUsers, err := s.can(r.Context(), user, ActionUserRead, anyUserResource)
	if err != nil {
		s.renderInternalError(w, r, err, "check user permission")

		return
	}

	if !canManagePosts && !canManageUsers {
		s.logDenied(r, ActionPostCreate, anyPostResource)
		s.renderError(w, r, http.StatusForbidden, "You do not have access to that.")

		return
	}

	content := dashboardContent{CanManagePosts: canManagePosts, CanManageUsers: canManageUsers}

	if canManagePosts && !s.fillPostStats(w, r, user, &content) {
		return
	}

	if canManageUsers {
		users, err := s.deps.Auth.ListUsers(r.Context(), 0)
		if err != nil {
			s.renderInternalError(w, r, err, "list users")

			return
		}

		content.Users = len(users)
	}

	data := s.newLayoutData(r, "Dashboard")
	data.Content = content

	s.render(w, r, http.StatusOK, "admin_dashboard.gohtml", data)
}

func (s *Server) fillPostStats(
	w http.ResponseWriter,
	r *http.Request,
	user *User,
	content *dashboardContent,
) bool {
	// The same scope as the posts page: everyone's posts for an editor, their own for an author.
	seesEveryone, err := s.can(r.Context(), user, ActionPostUpdate, anyPostResource)
	if err != nil {
		s.renderInternalError(w, r, err, "check post permission")

		return false
	}

	authorRef := user.Ref
	if seesEveryone {
		authorRef = ""
	}

	posts, err := s.deps.Content.ListPosts(r.Context(), "", authorRef, 0)
	if err != nil {
		s.renderInternalError(w, r, err, "list posts")

		return false
	}

	for _, post := range posts {
		switch post.Status {
		case StatusPublished:
			content.Published++
		case StatusDraft:
			content.Drafts++
		case StatusArchived:
			content.Archived++
		}
	}

	content.ShowingEveryone = seesEveryone
	content.RecentPosts = posts[:min(len(posts), recentPostsShown)]

	return true
}

// adminTabs lists the admin sections the user may open. As with the Admin link, a failure to
// answer hides the section rather than blocking the page.
func (s *Server) adminTabs(r *http.Request, user *User, canManagePosts bool) []adminTab {
	canManageUsers, err := s.can(r.Context(), user, ActionUserRead, anyUserResource)
	if err != nil {
		s.logger.WarnContext(r.Context(), "check users tab visibility", slog.Any("error", err))
	}

	if !canManagePosts && !canManageUsers {
		return nil
	}

	tabs := []adminTab{{Label: "Dashboard", Path: dashboardPath}}

	if canManagePosts {
		tabs = append(tabs, adminTab{Label: "Posts", Path: "/admin/posts"})
	}

	if canManageUsers {
		tabs = append(tabs, adminTab{Label: "Users", Path: "/admin/users"})
	}

	for i := range tabs {
		tabs[i].URL = s.path(tabs[i].Path)
		tabs[i].Active = r.URL.Path == tabs[i].Path ||
			strings.HasPrefix(r.URL.Path, tabs[i].Path+"/")
	}

	return tabs
}
