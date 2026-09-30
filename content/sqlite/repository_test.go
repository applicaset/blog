package sqlite_test

import (
	"database/sql"
	"testing"

	"github.com/applicaset/buildset/blog/content/repotest"
	"github.com/applicaset/buildset/blog/content/sqlite"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestRepository(t *testing.T) {
	repotest.Run(t, func(t *testing.T) repotest.Repositories {
		t.Helper()

		db, err := sql.Open("sqlite", "file:"+t.TempDir()+"/test.db")
		require.NoError(t, err)

		db.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = db.Close() })

		require.NoError(t, sqlite.Migrate(t.Context(), db))

		return repotest.Repositories{Post: sqlite.NewPostRepository(db)}
	})
}
