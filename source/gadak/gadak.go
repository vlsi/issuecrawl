// Package gadak reads the Jira mirror that gadak (github.com/midagedev/gadak)
// keeps in SQLite.
//
// The adapter reads only the issues_full view, which gadak documents as its
// stable data model, and the items, comments, and links tables, so that a
// different Jira mirror can replace gadak by providing the same few columns.
package gadak

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/vlsi/issuecrawl/core"
	"github.com/vlsi/issuecrawl/internal/rodb"
)

// Source is a [core.Source] over one gadak database.
type Source struct {
	db   *sql.DB
	path string
}

var _ core.Source = (*Source)(nil)

// Open opens the gadak database at path read-only.
func Open(path string) (*Source, error) {
	db, err := rodb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("gadak: %w", err)
	}
	return &Source{db: db, path: path}, nil
}

// Close closes the database.
func (s *Source) Close() error { return s.db.Close() }

func (s *Source) Name() string        { return "gadak" }
func (s *Source) System() core.System { return core.Jira }

// Get returns a Jira issue. Fields carries issue_type, priority, resolution,
// status_category, assignee, components, and fix_versions, each only when
// set; State is the workflow status, such as "Closed", and Closed is the
// resolution time.
func (s *Source) Get(ctx context.Context, ref core.Ref) (*core.Item, error) {
	if ref.System != core.Jira || ref.Kind != core.KindIssue {
		return nil, core.ErrNotFound
	}
	var (
		key, summary, url                                  string
		status, statusCategory, resolution, issueType      sql.NullString
		priority, assignee, reporter, labels, components   sql.NullString
		fixVersions, created, updated, resolved, descrText sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
		select i.key, coalesce(i.summary, ''), coalesce(it.url, ''),
		       i.status, i.status_category, i.resolution, i.issue_type, i.priority,
		       i.assignee, i.reporter, i.labels, i.components, i.fix_versions,
		       i.created_at, i.updated_at, i.resolved_at, i.description_text
		from issues_full i left join items it on it.id = i.item_id
		where i.key = ?`, strings.ToUpper(ref.Key)).Scan(
		&key, &summary, &url,
		&status, &statusCategory, &resolution, &issueType, &priority,
		&assignee, &reporter, &labels, &components, &fixVersions,
		&created, &updated, &resolved, &descrText)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	fields := map[string]any{}
	for name, v := range map[string]sql.NullString{
		"issue_type": issueType, "priority": priority, "resolution": resolution,
		"status_category": statusCategory, "assignee": assignee,
	} {
		if v.String != "" {
			fields[name] = v.String
		}
	}
	for name, v := range map[string]sql.NullString{"components": components, "fix_versions": fixVersions} {
		if list := jsonList(v); len(list) > 0 {
			fields[name] = list
		}
	}
	return &core.Item{
		Ref:     core.JiraIssue(key),
		Title:   summary,
		State:   status.String,
		URL:     url,
		Author:  reporter.String,
		Created: rodb.Time(created),
		Updated: rodb.Time(updated),
		Closed:  rodb.Time(resolved),
		Labels:  jsonList(labels),
		Fields:  fields,
		Body:    descrText.String,
	}, nil
}

// Outgoing returns the issue's Jira issue links and the GitHub pull request
// and commit URLs in its description and comments.
func (s *Source) Outgoing(ctx context.Context, _ core.Project, ref core.Ref) ([]core.Link, error) {
	if ref.System != core.Jira || ref.Kind != core.KindIssue {
		return nil, nil
	}
	var links []core.Link
	rows, err := s.db.QueryContext(ctx, `
		select l.type, l.direction, l.target_key
		from links l join issues_full i on i.item_id = l.item_id
		where i.key = ?
		order by l.type, l.direction, l.target_key`, ref.Key)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var typ, dir, target string
		if err := rows.Scan(&typ, &dir, &target); err != nil {
			rows.Close()
			return nil, err
		}
		links = append(links, core.Link{To: core.JiraIssue(target), Evidence: []core.Evidence{{
			Source: s.Name(), In: ref, Where: "issue-link", Excerpt: typ + " (" + dir + ")",
		}}})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.QueryContext(ctx, `
		select 'body', '', null, i.description_text
		from issues_full i where i.key = ?
		union all
		select 'comment', coalesce(c.author, ''), c.created_at, c.body_text
		from comments c join issues_full i on i.item_id = c.item_id
		where i.key = ?
		order by 3`, ref.Key, ref.Key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var where, author string
		var at, text sql.NullString
		if err := rows.Scan(&where, &author, &at, &text); err != nil {
			return nil, err
		}
		for _, tr := range core.ExtractRefs(text.String) {
			links = append(links, core.Link{To: tr.Ref, Evidence: []core.Evidence{{
				Source: s.Name(), In: ref, Where: where, Author: author, At: rodb.Time(at),
				Excerpt: core.Excerpt(text.String, regexp.MustCompile(regexp.QuoteMeta(tr.Match)), 200),
			}}})
		}
	}
	return links, rows.Err()
}

// Mentions returns the Jira issues whose description or comments mention the
// issue key ref names. Any Jira item other than an issue mentions nothing.
func (s *Source) Mentions(ctx context.Context, _ core.Project, ref core.Ref) ([]core.Link, error) {
	if ref.System != core.Jira || ref.Kind != core.KindIssue {
		return nil, nil
	}
	like := "%" + ref.Key + "%"
	rows, err := s.db.QueryContext(ctx, `
		select i.key, 'body', '', null, i.description_text
		from issues_full i
		where i.description_text like ? and i.key <> ?
		union all
		select i.key, 'comment', coalesce(c.author, ''), c.created_at, c.body_text
		from comments c join issues_full i on i.item_id = c.item_id
		where c.body_text like ? and i.key <> ?
		order by 1, 4`, like, ref.Key, like, ref.Key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	re := core.KeyMatcher(ref.Key)
	var links []core.Link
	for rows.Next() {
		var key, where, author string
		var at, text sql.NullString
		if err := rows.Scan(&key, &where, &author, &at, &text); err != nil {
			return nil, err
		}
		if !re.MatchString(text.String) {
			continue
		}
		in := core.JiraIssue(key)
		links = append(links, core.Link{To: in, Evidence: []core.Evidence{{
			Source: s.Name(), In: in, Where: where, Author: author, At: rodb.Time(at),
			Excerpt: core.Excerpt(text.String, re, 200),
		}}})
	}
	return links, rows.Err()
}

// Warnings reports a Jira project that the mirror holds no issues of.
func (s *Source) Warnings(ctx context.Context, p core.Project) []string {
	var n int
	err := s.db.QueryRowContext(ctx, `select count(*) from issues_full where project_key = ?`, p.Jira).Scan(&n)
	if err != nil {
		return []string{fmt.Sprintf("gadak: count issues of %s in %s: %v", p.Jira, s.path, err)}
	}
	if n == 0 {
		return []string{fmt.Sprintf("gadak: %s holds no issues of Jira project %s; sync it with gadak", s.path, p.Jira)}
	}
	return nil
}

// jsonList decodes a JSON array of strings, as gadak stores labels and
// versions, and returns nil for anything else.
func jsonList(v sql.NullString) []string {
	var out []string
	if v.String == "" || json.Unmarshal([]byte(v.String), &out) != nil {
		return nil
	}
	return out
}
