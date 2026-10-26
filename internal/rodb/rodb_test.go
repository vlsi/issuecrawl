package rodb_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vlsi/issuecrawl/internal/rodb"
	"github.com/vlsi/issuecrawl/internal/testfixture"
)

// The mirrors belong to gitcrawl and gadak, and issuecrawl must never change
// them, even through a query that a future adapter gets wrong.
func TestOpenRefusesWrites(t *testing.T) {
	db, err := rodb.Open(testfixture.GitcrawlDB(t))
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`delete from threads`)
	require.Error(t, err)

	var n int
	require.NoError(t, db.QueryRow(`select count(*) from threads`).Scan(&n))
	assert.Equal(t, 1, n, "threads left after the refused delete")
}

func TestOpenDoesNotCreateAMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.db")

	_, err := rodb.Open(path)
	require.Error(t, err)
	assert.NoFileExists(t, path)
}

func TestOpenAcceptsARelativePath(t *testing.T) {
	path := testfixture.GitcrawlDB(t)
	t.Chdir(filepath.Dir(path))

	db, err := rodb.Open(filepath.Base(path))
	require.NoError(t, err)
	defer db.Close()
	var n int
	require.NoError(t, db.QueryRow(`select count(*) from threads`).Scan(&n))
	assert.Equal(t, 1, n, "threads in the fixture")
}
