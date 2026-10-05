package web_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/applicaset/blog/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	publishedID  = "0199bf3c-7a1e-7c2b-9f10-0000000000bb"
	publishedRef = "urn:content:post:0199bf3c-7a1e-7c2b-9f10-0000000000bb"
	postPath     = "/posts/" + publishedID
	signedIn     = "urn:auth:group:authenticated"
)

// newDiscussionHarness has a published post that every signed-in user may comment on, through
// the group a session carries rather than a grant of their own.
func newDiscussionHarness(t *testing.T) *harness {
	t.Helper()

	h := newHarness(t)
	h.content.posts[publishedID] = &web.Post{
		Ref:       publishedRef,
		ID:        publishedID,
		AuthorRef: adaRef,
		Title:     "A published post",
		Body:      "Public.",
		Status:    web.StatusPublished,
	}

	for _, user := range h.auth.sessions {
		user.Groups = []string{signedIn}
	}

	h.authz.allow(signedIn, web.ActionCommentCreate, publishedRef)

	return h
}

func (h *harness) comment(t *testing.T, token, parent, body string) string {
	t.Helper()

	response := h.request(t, http.MethodPost, postPath+"/comments", token, url.Values{
		"parent": {parent},
		"body":   {body},
	})
	require.Equal(t, http.StatusSeeOther, response.Code)

	comment := h.discuss.comments[len(h.discuss.comments)-1]
	assert.Equal(t, postPath+"#comment-"+comment.ID, response.Header().Get("Location"))

	return comment.ID
}

func TestAnonymousVisitorReadsCommentsButGetsNoForm(t *testing.T) {
	h := newDiscussionHarness(t)
	h.comment(t, adaToken, "", "Welcome, everyone.")

	response := h.request(t, http.MethodGet, postPath, "", nil)

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "Welcome, everyone.")
	assert.Contains(t, body, "Ada")
	assert.Contains(t, body, `<a href="/login?next=`+postPath+`">Sign in to comment</a>`)
	assert.NotContains(t, body, `action="`+postPath+`/comments"`)
}

func TestAnonymousCommentIsRefused(t *testing.T) {
	h := newDiscussionHarness(t)

	response := h.request(t, http.MethodPost, postPath+"/comments", "", url.Values{
		"body": {"Hello."},
	})

	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Empty(t, h.discuss.comments)
}

func TestSignedInReaderCommentsAndReplies(t *testing.T) {
	h := newDiscussionHarness(t)

	parent := h.comment(t, graceName, "", "First.")
	reply := h.comment(t, adaToken, parent, "A reply.")

	require.Len(t, h.discuss.comments, 2)
	assert.Equal(t, graceRef, h.discuss.comments[0].AuthorRef)
	assert.Equal(t, parent, h.discuss.comments[1].ParentID)

	body := h.request(t, http.MethodGet, postPath, graceName, nil).Body.String()
	assert.Contains(
		t,
		body,
		`id="comment-`+reply+`" class="as-card blog-comment flex flex-col gap-2" style="--depth: 1"`,
	)
	assert.Contains(t, body, `aria-label="Reply to Ada"`)
	assert.Contains(t, body, `action="`+postPath+`/comments"`)
}

func TestReaderWithoutPermissionCannotComment(t *testing.T) {
	h := newDiscussionHarness(t)
	h.auth.sessions[graceName].Groups = nil

	page := h.request(t, http.MethodGet, postPath, graceName, nil).Body.String()
	assert.NotContains(t, page, `action="`+postPath+`/comments"`)
	assert.NotContains(t, page, "Sign in to comment")

	response := h.request(t, http.MethodPost, postPath+"/comments", graceName, url.Values{
		"body": {"Hello."},
	})

	assert.Equal(
		t,
		http.StatusForbidden,
		response.Code,
		"the post is public, so the refusal is honest",
	)
	assert.Empty(t, h.discuss.comments)
}

func TestEmptyCommentShowsThePostWithTheReason(t *testing.T) {
	h := newDiscussionHarness(t)

	response := h.request(t, http.MethodPost, postPath+"/comments", graceName, url.Values{
		"body": {"   "},
	})

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "Write a comment first.")
	assert.Contains(t, response.Body.String(), "Public.")
}

func TestAuthorEditsAndDeletesTheirComment(t *testing.T) {
	h := newDiscussionHarness(t)
	id := h.comment(t, graceName, "", "Frist.")

	response := h.request(t, http.MethodPost, postPath+"/comments/"+id, graceName, url.Values{
		"revision": {"0"},
		"body":     {"First."},
	})
	require.Equal(t, http.StatusSeeOther, response.Code)
	assert.Equal(t, postPath+"#comment-"+id, response.Header().Get("Location"))
	assert.Equal(t, "First.", h.discuss.comments[0].Body)

	stale := h.request(t, http.MethodPost, postPath+"/comments/"+id, graceName, url.Values{
		"revision": {"0"},
		"body":     {"Over a stale copy."},
	})
	assert.Equal(t, http.StatusConflict, stale.Code)

	page := h.request(t, http.MethodGet, postPath, graceName, nil).Body.String()
	assert.Contains(t, page, "<span>edited</span>")

	confirm := h.request(t, http.MethodGet, postPath+"/comments/"+id+"/delete", graceName, nil)
	require.Equal(t, http.StatusOK, confirm.Code)
	assert.Contains(t, confirm.Body.String(), `action="`+postPath+`/comments/`+id+`/delete"`)
	assert.Nil(t, h.discuss.comments[0].DeletedAt, "asking first must not act")

	response = h.request(
		t,
		http.MethodPost,
		postPath+"/comments/"+id+"/delete",
		graceName,
		url.Values{},
	)
	require.Equal(t, http.StatusSeeOther, response.Code)

	page = h.request(t, http.MethodGet, postPath, graceName, nil).Body.String()
	assert.Contains(t, page, "This comment was deleted.")
	assert.NotContains(t, page, "First.")
}

func TestChangingSomeoneElsesCommentIsRefused(t *testing.T) {
	h := newDiscussionHarness(t)
	id := h.comment(t, adaToken, "", "Mine.")

	page := h.request(t, http.MethodGet, postPath, graceName, nil).Body.String()
	assert.NotContains(t, page, `action="`+postPath+`/comments/`+id+`"`)

	edit := h.request(t, http.MethodPost, postPath+"/comments/"+id, graceName, url.Values{
		"revision": {"0"},
		"body":     {"Not yours."},
	})
	assert.Equal(t, http.StatusForbidden, edit.Code)

	remove := h.request(
		t,
		http.MethodPost,
		postPath+"/comments/"+id+"/delete",
		graceName,
		url.Values{},
	)
	assert.Equal(t, http.StatusForbidden, remove.Code)

	assert.Equal(t, "Mine.", h.discuss.comments[0].Body)
	assert.Nil(t, h.discuss.comments[0].DeletedAt)
}

func TestDraftHasNoDiscussion(t *testing.T) {
	h := newDiscussionHarness(t)
	h.authz.allow(adaRef, web.ActionPostRead, draftRef)
	h.authz.allow(signedIn, web.ActionCommentCreate, draftRef)

	page := h.request(t, http.MethodGet, "/posts/"+draftID, adaToken, nil)
	require.Equal(t, http.StatusOK, page.Code)
	assert.NotContains(t, page.Body.String(), "Comments")

	response := h.request(t, http.MethodPost, "/posts/"+draftID+"/comments", adaToken, url.Values{
		"body": {"Too early."},
	})
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Empty(t, h.discuss.comments)
}

func TestReplyDeeperThanTheIndentNamesItsParent(t *testing.T) {
	h := newDiscussionHarness(t)

	parent := ""
	for range 6 {
		parent = h.comment(t, adaToken, parent, "Deeper.")
	}

	page := h.request(t, http.MethodGet, postPath, "", nil).Body.String()
	assert.Contains(t, page, "replying to Ada")
	assert.NotContains(t, page, `style="--depth: 5"`)
}
