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
	fill(t, admin, "Display name", "Ada Lovelace")
	fill(t, admin, "Password", "correct horse battery")
	click(t, admin, "Create administrator")

	reader := h.newPage(t)
	h.open(t, reader, "/register")
	fill(t, reader, "Username", "grace")
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
