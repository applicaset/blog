package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
)

// maxVisualDepth caps the reply indent. Deeper replies say whom they answer instead, so the text
// keeps a readable width on a phone.
const maxVisualDepth = 4

// unknownAuthor names a commenter whose account is gone or could not be read.
const unknownAuthor = "Someone"

type commentView struct {
	Comment

	Author      string
	Mine        bool
	VisualDepth int
	// ReplyingTo is set for a reply shown past the deepest indent.
	ReplyingTo string
}

// buildThread lays out comments, oldest first, with each reply under its parent. names maps author
// refs to what to call them, and viewerRef is who is looking.
func buildThread(comments []Comment, names map[string]string, viewerRef string) []commentView {
	present := make(map[string]bool, len(comments))
	for _, comment := range comments {
		present[comment.ID] = true
	}

	// A reply whose parent is not loaded is shown at the top, so it can never disappear.
	var roots []Comment

	replies := make(map[string][]Comment)

	for _, comment := range comments {
		if comment.ParentID != "" && present[comment.ParentID] && comment.ParentID != comment.ID {
			replies[comment.ParentID] = append(replies[comment.ParentID], comment)

			continue
		}

		roots = append(roots, comment)
	}

	rows := make([]commentView, 0, len(comments))
	for _, root := range roots {
		rows = appendThread(rows, root, replies, names, viewerRef, 0, "")
	}

	return rows
}

func appendThread(
	rows []commentView,
	comment Comment,
	replies map[string][]Comment,
	names map[string]string,
	viewerRef string,
	depth int,
	parentName string,
) []commentView {
	view := commentView{
		Comment:     comment,
		Author:      names[comment.AuthorRef],
		Mine:        viewerRef != "" && comment.AuthorRef == viewerRef,
		VisualDepth: min(depth, maxVisualDepth),
	}

	if depth > maxVisualDepth {
		view.ReplyingTo = parentName
	}

	rows = append(rows, view)

	for _, reply := range replies[comment.ID] {
		rows = appendThread(rows, reply, replies, names, viewerRef, depth+1, view.Author)
	}

	return rows
}

// commentAuthors looks each author up once per page.
func (s *Server) commentAuthors(ctx context.Context, comments []Comment) map[string]string {
	names := make(map[string]string)

	for _, comment := range comments {
		if _, seen := names[comment.AuthorRef]; seen {
			continue
		}

		names[comment.AuthorRef] = s.authorName(ctx, comment.AuthorRef)
	}

	return names
}

// A missing name must not take the post down with it.
func (s *Server) authorName(ctx context.Context, userRef string) string {
	user, err := s.deps.Auth.GetUserByRef(ctx, userRef)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			s.logger.WarnContext(ctx, "get comment author",
				slog.String("user_ref", userRef),
				slog.Any("error", err),
			)
		}

		return unknownAuthor
	}

	if user.Name != "" {
		return user.Name
	}

	return user.Username
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok || !s.parseForm(w, r) {
		return
	}

	post, ok := s.publishedPost(w, r)
	if !ok {
		return
	}

	allowed, err := s.can(r.Context(), user, ActionCommentCreate, post.Ref)
	if err != nil {
		s.renderInternalError(w, r, err, "check comment permission")

		return
	}

	if !allowed {
		// The post is public, so a 403 gives nothing away.
		s.renderError(w, r, http.StatusForbidden, "You cannot comment on this post.")

		return
	}

	comment, err := s.deps.Discuss.AddComment(
		r.Context(),
		user.Ref,
		post.Ref,
		r.PostFormValue("parent"),
		r.PostFormValue("body"),
	)
	if err != nil {
		s.renderCommentFailure(w, r, post, err, "add comment")

		return
	}

	http.Redirect(w, r, s.commentURL(post.ID, comment.ID), http.StatusSeeOther)
}

func (s *Server) editComment(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok || !s.parseForm(w, r) {
		return
	}

	post, ok := s.publishedPost(w, r)
	if !ok {
		return
	}

	revision, err := strconv.Atoi(r.PostFormValue("revision"))
	if err != nil {
		s.renderError(w, r, http.StatusBadRequest, "That form could not be read.")

		return
	}

	commentID := r.PathValue("comment")

	if err := s.deps.Discuss.EditComment(
		r.Context(),
		user.Ref,
		post.Ref,
		commentID,
		revision,
		r.PostFormValue("body"),
	); err != nil {
		s.renderCommentFailure(w, r, post, err, "edit comment")

		return
	}

	http.Redirect(w, r, s.commentURL(post.ID, commentID), http.StatusSeeOther)
}

func (s *Server) confirmDeleteComment(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}

	post, ok := s.publishedPost(w, r)
	if !ok {
		return
	}

	commentID := r.PathValue("comment")

	data := s.newLayoutData(r, "Delete comment")
	data.Content = confirmContent{
		Message: "Delete this comment? Replies stay, and it shows as deleted. This cannot be " +
			"undone.",
		ActionURL: s.path(
			"/posts/" + post.ID + "/comments/" + url.PathEscape(commentID) + "/delete",
		),
		Submit:    "Delete comment",
		CancelURL: s.commentURL(post.ID, commentID),
	}

	s.render(w, r, http.StatusOK, "confirm.gohtml", data)
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok || !s.parseForm(w, r) {
		return
	}

	post, ok := s.publishedPost(w, r)
	if !ok {
		return
	}

	commentID := r.PathValue("comment")

	if err := s.deps.Discuss.DeleteComment(
		r.Context(),
		user.Ref,
		post.Ref,
		commentID,
	); err != nil {
		s.renderCommentFailure(w, r, post, err, "delete comment")

		return
	}

	// The comment keeps its place as a tombstone, so the anchor still lands on it.
	http.Redirect(w, r, s.commentURL(post.ID, commentID), http.StatusSeeOther)
}

// publishedPost is the post a comment request names. Only a published post has a discussion, and
// any other answers as missing, as showPost does for a visitor without access.
func (s *Server) publishedPost(w http.ResponseWriter, r *http.Request) (*Post, bool) {
	id := r.PathValue("id")

	if _, err := postResource(id); err != nil {
		s.renderError(w, r, http.StatusNotFound, "There is no such post.")

		return nil, false
	}

	post, err := s.deps.Content.GetPost(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.renderError(w, r, http.StatusNotFound, "There is no such post.")

			return nil, false
		}

		s.renderInternalError(w, r, err, "get post")

		return nil, false
	}

	if post.Status != StatusPublished {
		s.renderError(w, r, http.StatusNotFound, "There is no such post.")

		return nil, false
	}

	return post, true
}

// renderCommentFailure shows a refused comment on the post, so the visitor can fix it in place.
// Anything else gets the error page with the service's sentence.
func (s *Server) renderCommentFailure(
	w http.ResponseWriter,
	r *http.Request,
	post *Post,
	err error,
	operation string,
) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		s.renderPost(w, r, http.StatusBadRequest, post, err.Error())
	case errors.Is(err, ErrNotFound):
		s.renderError(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrForbidden):
		s.renderError(w, r, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrConflict):
		s.renderError(w, r, http.StatusConflict, err.Error())
	default:
		s.renderInternalError(w, r, err, operation)
	}
}

func (s *Server) commentURL(postID, commentID string) string {
	return s.path("/posts/"+postID) + "#comment-" + url.PathEscape(commentID)
}
