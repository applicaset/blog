package tests

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFirstRunToPublishedPost covers the core journey: an empty instance becomes a blog with a post
// a stranger can read.
func TestFirstRunToPublishedPost(t *testing.T) {
	forEachTopology(t, firstRunToPublishedPost)
}

func firstRunToPublishedPost(t *testing.T, h *harness) {
	t.Helper()

	admin := h.newPage(t)

	// An instance with no accounts sends its first visitor to set itself up.
	h.openSite(t, admin, "/")
	require.Contains(t, admin.URL(), "/setup")

	fill(t, admin, "Username", "ada")
	fill(t, admin, "Email", "ada@example.com")
	fill(t, admin, "Display name", "Ada Lovelace")
	fill(t, admin, "Password", "correct horse battery")
	click(t, admin, "Create administrator")

	// This link is here only because the first account is an administrator.
	require.NoError(
		t,
		admin.GetByRole("link", playwright.PageGetByRoleOptions{Name: "Admin"}).WaitFor(),
	)

	h.openSite(t, admin, "/admin/posts/new")
	fill(t, admin, "Title", "On Computing")
	fill(t, admin, "Body", "First paragraph.\n\nSecond paragraph.")
	click(t, admin, "Create draft")

	// A draft is invisible to anyone else. The home page shows no trace of it.
	stranger := h.newPage(t)
	response := h.openSite(t, stranger, "/")
	require.Equal(t, 200, response.Status())

	content, err := stranger.Content()
	require.NoError(t, err)
	assert.NotContains(t, content, "On Computing", "a draft must not appear on the home page")

	click(t, admin, "Publish")

	// Now the stranger can read it, without signing in.
	h.openSite(t, stranger, "/")
	require.NoError(
		t,
		stranger.GetByRole("link", playwright.PageGetByRoleOptions{Name: "On Computing"}).Click(),
	)

	body, err := stranger.Locator("article").TextContent()
	require.NoError(t, err)
	assert.Contains(t, body, "First paragraph.")
	assert.Contains(t, body, "Second paragraph.")
}

// TestSignedInReaderHasNoAdministration checks that having an account does not grant
// administration access.
func TestSignedInReaderHasNoAdministration(t *testing.T) {
	forEachTopology(t, signedInReaderHasNoAdministration)
}

func signedInReaderHasNoAdministration(t *testing.T, h *harness) {
	t.Helper()

	admin := h.newPage(t)
	h.open(t, admin, "/setup")
	fill(t, admin, "Username", "ada")
	fill(t, admin, "Email", "ada@example.com")
	fill(t, admin, "Display name", "Ada Lovelace")
	fill(t, admin, "Password", "correct horse battery")
	click(t, admin, "Create administrator")

	reader := h.newPage(t)
	h.open(t, reader, "/register")
	fill(t, reader, "Username", "grace")
	fill(t, reader, "Email", "grace@example.com")
	fill(t, reader, "Display name", "Grace Hopper")
	fill(t, reader, "Password", "another good secret")
	click(t, reader, "Create account")

	response := h.openSite(t, reader, "/admin")
	assert.Equal(t, 403, response.Status(), "a signed-in reader has no administration")

	// The public site still works for them.
	response = h.openSite(t, reader, "/")
	assert.Equal(t, 200, response.Status())

	count, err := reader.GetByRole("link", playwright.PageGetByRoleOptions{Name: "Admin"}).Count()
	require.NoError(t, err)
	assert.Zero(t, count, "a link to a page they cannot open must not be shown")
}

// TestReaderDiscussesAPublishedPost covers a thread from a signed-in reader's side, and what a
// stranger then sees of it.
func TestReaderDiscussesAPublishedPost(t *testing.T) {
	forEachTopology(t, readerDiscussesAPublishedPost)
}

func readerDiscussesAPublishedPost(t *testing.T, h *harness) {
	t.Helper()

	admin := h.newPage(t)
	h.open(t, admin, "/setup")
	fill(t, admin, "Username", "ada")
	fill(t, admin, "Email", "ada@example.com")
	fill(t, admin, "Display name", "Ada Lovelace")
	fill(t, admin, "Password", "correct horse battery")
	click(t, admin, "Create administrator")
	require.NoError(
		t,
		admin.GetByRole("link", playwright.PageGetByRoleOptions{Name: "Admin"}).WaitFor(),
	)

	h.openSite(t, admin, "/admin/posts/new")
	fill(t, admin, "Title", "On Computing")
	fill(t, admin, "Body", "First paragraph.")
	click(t, admin, "Create draft")
	click(t, admin, "Publish")
	require.NoError(t, admin.GetByText("published", playwright.PageGetByTextOptions{
		Exact: new(true),
	}).WaitFor())

	reader := h.newPage(t)
	h.open(t, reader, "/register")
	fill(t, reader, "Username", "grace")
	fill(t, reader, "Email", "grace@example.com")
	fill(t, reader, "Display name", "Grace Hopper")
	fill(t, reader, "Password", "another good secret")
	click(t, reader, "Create account")
	require.NoError(t, reader.GetByLabel("Account menu").WaitFor())

	h.openSite(t, reader, "/")
	require.NoError(
		t,
		reader.GetByRole("link", playwright.PageGetByRoleOptions{Name: "On Computing"}).Click(),
	)

	require.NoError(t, reader.GetByLabel("Comment", playwright.PageGetByLabelOptions{
		Exact: new(true),
	}).Fill("Great post."))
	clickExact(t, reader, "Comment")

	comments := reader.Locator("#comments article")
	first := comments.Nth(0)
	require.NoError(t, commentText(first, "Great post.").WaitFor())

	require.NoError(t, first.Locator("summary", playwright.LocatorLocatorOptions{
		HasText: "Reply",
	}).Click())
	require.NoError(t, first.GetByLabel("Reply to Grace Hopper").Fill("And a reply."))
	require.NoError(t, first.GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Reply",
	}).Click())

	reply := comments.Nth(1)
	require.NoError(t, commentText(reply, "And a reply.").WaitFor())

	first = reader.Locator("#comments article").Nth(0)
	require.NoError(t, first.Locator("summary", playwright.LocatorLocatorOptions{
		HasText: "Edit",
	}).Click())
	require.NoError(t, first.GetByLabel("Edit comment").Fill("Great post, really."))
	require.NoError(t, first.GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Save",
	}).Click())
	require.NoError(t, commentText(first, "Great post, really.").WaitFor())
	require.NoError(t, reader.Locator("#comments article").Nth(0).GetByText("edited").WaitFor())

	require.NoError(t, reader.Locator("#comments article").Nth(1).
		GetByRole("link", playwright.LocatorGetByRoleOptions{Name: "Delete"}).Click())
	click(t, reader, "Delete comment")
	require.NoError(t, reader.GetByText("This comment was deleted.").WaitFor())

	// A stranger reads the thread but has nothing to write in.
	stranger := h.newPage(t)
	h.openSite(t, stranger, "/")
	require.NoError(
		t,
		stranger.GetByRole("link", playwright.PageGetByRoleOptions{Name: "On Computing"}).Click(),
	)
	require.NoError(t, commentText(stranger.Locator("#comments"), "Great post, really.").WaitFor())
	require.NoError(t, stranger.GetByText("This comment was deleted.").WaitFor())
	require.NoError(t, stranger.GetByRole("link", playwright.PageGetByRoleOptions{
		Name: "Sign in to comment",
	}).WaitFor())

	forms, err := stranger.GetByRole("textbox").Count()
	require.NoError(t, err)
	assert.Zero(t, forms, "a stranger must not be offered a form they cannot submit")
}

func clickExact(t *testing.T, page playwright.Page, name string) {
	t.Helper()

	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name:  name,
		Exact: new(true),
	}).Click())
}

// commentText finds a comment's body, not the edit form that repeats it.
func commentText(scope playwright.Locator, text string) playwright.Locator {
	return scope.Locator("p", playwright.LocatorLocatorOptions{HasText: text})
}
