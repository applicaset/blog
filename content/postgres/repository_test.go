package postgres_test

import (
	"testing"

	"github.com/applicaset/buildset/blog/content/postgres"
	"github.com/applicaset/buildset/blog/content/repotest"
	"github.com/applicaset/buildset/pkg/pgtest"
	"github.com/stretchr/testify/require"
)

func TestRepository(t *testing.T) {
	dsn := pgtest.DSN(t)

	repotest.Run(t, func(t *testing.T) repotest.Repositories {
		t.Helper()

		db := pgtest.Open(t, dsn)
		require.NoError(t, postgres.Migrate(t.Context(), db))

		return repotest.Repositories{Post: postgres.NewPostRepository(db)}
	})
}
