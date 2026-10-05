package web

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
)

type indexContent struct {
	Posts []Post
}

type postContent struct {
	Post Post
	Body template.HTML
	// URL is the post's own address, the base of every comment form.
	URL string
	// Discussion is false on a post that is not published, which shows no comments.
	Discussion   bool
	Comments     []commentView
	CanComment   bool
	SignedIn     bool
	SignInURL    string
	CommentError string
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	// An instance with no users has nothing to show, so the first visitor is sent to set it up.
	open, err := s.deps.Auth.SetupOpen(r.Context())
	if err != nil {
		s.renderInternalError(w, r, err, "check setup state")

		return
	}

	if open {
		http.Redirect(w, r, s.deps.Auth.SetupURL(s.path("/")), http.StatusSeeOther)

		return
	}

	posts, err := s.deps.Content.ListPosts(r.Context(), StatusPublished, "", 0)
	if err != nil {
		s.renderInternalError(w, r, err, "list published posts")

		return
	}

	data := s.newLayoutData(r, s.config.SiteTitle)
	data.Content = indexContent{Posts: posts}

	s.render(w, r, http.StatusOK, "index.gohtml", data)
}

func (s *Server) showPost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	resource, err := postResource(id)
	if err != nil {
		s.renderError(w, r, http.StatusNotFound, "There is no such post.")

		return
	}

	post, err := s.deps.Content.GetPost(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.renderError(w, r, http.StatusNotFound, "There is no such post.")

			return
		}

		s.renderInternalError(w, r, err, "get post")

		return
	}

	if post.Status != StatusPublished {
		user, _ := userFromContext(r.Context())

		allowed, err := s.can(r.Context(), user, ActionPostRead, resource)
		if err != nil {
			s.renderInternalError(w, r, err, "check post access")

			return
		}

		if !allowed {
			// Not 403. A refusal would confirm that a draft with this identifier exists.
			s.logDenied(r, ActionPostRead, resource)
			s.renderError(w, r, http.StatusNotFound, "There is no such post.")

			return
		}
	}

	s.renderPost(w, r, http.StatusOK, post, "")
}

// renderPost shows the post with its discussion. commentError is a refused comment's sentence.
func (s *Server) renderPost(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	post *Post,
	commentError string,
) {
	body, err := s.deps.Content.RenderBody(r.Context(), post)
	if err != nil {
		s.renderInternalError(w, r, err, "render post body")

		return
	}

	page := postContent{
		Post:         *post,
		Body:         body,
		URL:          s.path("/posts/" + post.ID),
		CommentError: commentError,
	}

	if post.Status == StatusPublished {
		comments, err := s.deps.Discuss.ListComments(r.Context(), post.Ref)
		if err != nil {
			s.renderInternalError(w, r, err, "list comments")

			return
		}

		user, _ := userFromContext(r.Context())

		viewerRef := ""
		if user != nil {
			viewerRef = user.Ref
		}

		// A failure to answer hides the form rather than blocking the post.
		canComment, err := s.can(r.Context(), user, ActionCommentCreate, post.Ref)
		if err != nil {
			s.logger.WarnContext(r.Context(), "check comment permission", slog.Any("error", err))
		}

		page.Discussion = true
		page.Comments = buildThread(comments, s.commentAuthors(r.Context(), comments), viewerRef)
		page.CanComment = canComment
		page.SignedIn = user != nil
		page.SignInURL = s.deps.Auth.LoginURL(page.URL)
	}

	data := s.newLayoutData(r, post.Title)
	data.Content = page

	s.render(w, r, status, "post.gohtml", data)
}
