// Package testfixture builds small gitcrawl and gadak databases for tests.
//
// The schemas are subsets of the real ones, copied from live databases, so an
// adapter query that names a column the real tool does not have fails here
// too. The seed files hold the CALCITE-7377 scenario: a comment on the Jira
// issue cites the full URL of commit fd7220017887, and pull request
// apache/calcite#4746, whose title carries the key, was merged as that commit.
package testfixture

import (
	"database/sql"
	_ "embed"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

var (
	//go:embed gitcrawl_schema.sql
	gitcrawlSchema string
	//go:embed gitcrawl_calcite_7377.sql
	gitcrawlSeed string
	//go:embed gadak_schema.sql
	gadakSchema string
	//go:embed gadak_calcite_7377.sql
	gadakSeed string
)

// GitcrawlDB creates a gitcrawl database holding the CALCITE-7377 scenario,
// then runs extra, and returns its path. The file is removed with t.TempDir.
func GitcrawlDB(t testing.TB, extra ...string) string {
	return create(t, "gitcrawl.db", append([]string{gitcrawlSchema, gitcrawlSeed}, extra...))
}

// GadakDB creates a gadak database holding the CALCITE-7377 scenario, then
// runs extra, and returns its path. The file is removed with t.TempDir.
func GadakDB(t testing.TB, extra ...string) string {
	return create(t, "gadak.db", append([]string{gadakSchema, gadakSeed}, extra...))
}

func create(t testing.TB, name string, scripts []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	defer db.Close()
	for _, script := range scripts {
		if _, err := db.Exec(script); err != nil {
			t.Fatalf("populate %s: %v", name, err)
		}
	}
	return path
}
