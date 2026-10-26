// Package rodb opens the SQLite databases of other tools for reading only.
package rodb

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Open opens the SQLite database at path read-only. SQLite refuses every
// write on the returned handle, so a query cannot modify a mirror that its
// own tool owns. Open fails when path does not exist instead of creating it.
func Open(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	// A relative path would land in the authority part of the file: URI.
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: abs, RawQuery: "mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

// Time parses an RFC 3339 timestamp, as both gitcrawl and gadak store them,
// and returns nil for an empty or unparsable one.
func Time(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s.String)
	if err != nil {
		return nil
	}
	return &t
}
