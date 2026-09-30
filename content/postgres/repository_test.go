package postgres_test

import (
	"testing"

	"github.com/applicaset/buildset/blog/content"
	"github.com/applicaset/buildset/blog/content/postgres"
	"github.com/applicaset/buildset/blog/content/repotest"
	"github.com/applicaset/buildset/pkg/pgtest"
	"github.com/stretchr/testify/require"
)

func TestRepository(t *testing.T) {
	dsn := pgtest.DSN(t)

	repotest.Run(t, func(t *testing.T) content.Repository {
		t.Helper()

		repository, err := postgres.NewRepository(t.Context(), pgtest.Open(t, dsn))
		require.NoError(t, err)

		return repository
	})
}
