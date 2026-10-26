// Package gitcrawl reads the GitHub mirror that gitcrawl
// (github.com/openclaw/gitcrawl) keeps in SQLite.
//
// The adapter reads the repositories, threads, comments, pull_request_details,
// pull_request_commits, and sync_runs tables.
package gitcrawl

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/vlsi/issuecrawl/core"
	"github.com/vlsi/issuecrawl/internal/rodb"
)

// Source is a [core.Source] over one gitcrawl database.
type Source struct {
	db   *sql.DB
	path string
}

var _ core.Source = (*Source)(nil)

// Open opens the gitcrawl database at path read-only.
func Open(path string) (*Source, error) {
	db, err := rodb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("gitcrawl: %w", err)
	}
	return &Source{db: db, path: path}, nil
}

// Close closes the database.
func (s *Source) Close() error { return s.db.Close() }

func (s *Source) Name() string        { return "gitcrawl" }
func (s *Source) System() core.System { return core.GitHub }

// mergeSHA is the SQL expression for a pull request's merge commit. gitcrawl
// keeps the pull request object in pull_request_details when it hydrated the
// details, and in threads.raw_json when it fetched the pull request itself.
const mergeSHA = `coalesce(
	json_extract(pd.raw_json, '$.merge_commit_sha'),
	case when json_valid(t.raw_json) then json_extract(t.raw_json, '$.merge_commit_sha') end, '')`

// Get returns a pull request, an issue, or a commit of a pull request.
//
// For a pull request, State is "merged" once it is merged and the GitHub
// state otherwise, Closed is the merge or close time, and Fields carries
// merge_commit_sha and draft when set. A commit resolves only when gitcrawl
// hydrated the commits of a pull request that contains it; its Fields carry
// pull_request, the key of that pull request.
func (s *Source) Get(ctx context.Context, ref core.Ref) (*core.Item, error) {
	switch ref.Kind {
	case core.KindPullRequest, core.KindIssue:
		return s.getThread(ctx, ref)
	case core.KindCommit:
		return s.getCommit(ctx, ref)
	}
	return nil, core.ErrNotFound
}

func (s *Source) getThread(ctx context.Context, ref core.Ref) (*core.Item, error) {
	repo, num, ok := strings.Cut(ref.Key, "#")
	number, err := strconv.Atoi(num)
	if !ok || err != nil {
		return nil, core.ErrNotFound
	}
	var (
		title, state, url, labels, sha         string
		draft                                  bool
		author, body, created, updated, closed sql.NullString
		merged                                 sql.NullString
	)
	err = s.db.QueryRowContext(ctx, `
		select t.title, t.state, t.html_url, t.labels_json, t.is_draft, `+mergeSHA+`,
		       t.author_login, t.body, t.created_at_gh, t.updated_at_gh, t.closed_at_gh, t.merged_at_gh
		from threads t
		join repositories r on r.id = t.repo_id
		left join pull_request_details pd on pd.thread_id = t.id
		where r.full_name = ? collate nocase and t.kind = ? and t.number = ?`,
		repo, string(ref.Kind), number).Scan(
		&title, &state, &url, &labels, &draft, &sha,
		&author, &body, &created, &updated, &closed, &merged)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item := &core.Item{
		Ref:     ref,
		Title:   strings.TrimSpace(title),
		State:   state,
		URL:     url,
		Author:  author.String,
		Created: rodb.Time(created),
		Updated: rodb.Time(updated),
		Closed:  rodb.Time(closed),
		Labels:  labelNames(labels),
		Fields:  map[string]any{},
		Body:    body.String,
	}
	if m := rodb.Time(merged); m != nil {
		item.State, item.Closed = "merged", m
	}
	if sha != "" {
		item.Fields["merge_commit_sha"] = sha
	}
	if draft {
		item.Fields["draft"] = true
	}
	return item, nil
}

func (s *Source) getCommit(ctx context.Context, ref core.Ref) (*core.Item, error) {
	repo, sha, _ := strings.Cut(ref.Key, "@")
	var (
		fullSHA, message, url, prRepo string
		number                        int
		login, name, committed        sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
		select c.sha, coalesce(c.message, ''), coalesce(c.html_url, ''), r.full_name, t.number,
		       c.author_login, c.author_name, c.committed_at
		from pull_request_commits c
		join threads t on t.id = c.thread_id
		join repositories r on r.id = t.repo_id
		where r.full_name = ? collate nocase and c.sha like ? and c.deleted_at is null
		order by t.number
		limit 1`, repo, sha+"%").Scan(
		&fullSHA, &message, &url, &prRepo, &number, &login, &name, &committed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	subject, _, _ := strings.Cut(message, "\n")
	return &core.Item{
		Ref:     core.Commit(repo, fullSHA),
		Title:   subject,
		URL:     url,
		Author:  cmp.Or(login.String, name.String),
		Created: rodb.Time(committed),
		Fields:  map[string]any{"pull_request": core.PullRequest(prRepo, number).Key},
		Body:    message,
	}, nil
}

// Outgoing returns the merge commit of a pull request, when the mirror knows
// it. Other refs point at nothing.
func (s *Source) Outgoing(ctx context.Context, _ core.Project, ref core.Ref) ([]core.Link, error) {
	if ref.Kind != core.KindPullRequest {
		return nil, nil
	}
	item, err := s.getThread(ctx, ref)
	if errors.Is(err, core.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sha, _ := item.Fields["merge_commit_sha"].(string)
	if sha == "" || item.State != "merged" {
		return nil, nil
	}
	return []core.Link{{To: core.Commit(ref.Repo(), sha), Evidence: []core.Evidence{{
		Source: s.Name(), In: ref, Where: "merge-commit", At: item.Closed,
	}}}}, nil
}

// Mentions returns the pull requests and issues of p.GitHub whose title,
// body, comments, or commit messages mention the key ref names. When
// p.GitHub is empty, every mirrored repository is searched.
func (s *Source) Mentions(ctx context.Context, p core.Project, ref core.Ref) ([]core.Link, error) {
	if ref.System != core.Jira {
		return nil, nil
	}
	like := "%" + ref.Key + "%"
	// An empty repo matches every repository.
	rows, err := s.db.QueryContext(ctx, `
		with t as (
			select t.id, t.kind, t.number, r.full_name repo, t.title, t.body, t.author_login, t.created_at_gh
			from threads t join repositories r on r.id = t.repo_id
			where ? = '' or r.full_name = ? collate nocase
		)
		select repo, kind, number, 'title', coalesce(author_login, ''), created_at_gh, title
		from t where title like ?
		union all
		select repo, kind, number, 'body', coalesce(author_login, ''), created_at_gh, body
		from t where body like ?
		union all
		select t.repo, t.kind, t.number, 'comment', coalesce(c.author_login, ''), c.created_at_gh, c.body
		from comments c join t on t.id = c.thread_id
		where c.body like ? and c.deleted_at is null
		union all
		select t.repo, t.kind, t.number, 'commit', coalesce(c.author_login, c.author_name, ''), c.committed_at, c.message
		from pull_request_commits c join t on t.id = c.thread_id
		where c.message like ? and c.deleted_at is null
		order by 1, 3, 6`, p.GitHub, p.GitHub, like, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	re := core.KeyMatcher(ref.Key)
	var links []core.Link
	for rows.Next() {
		var repo, kind, where, author string
		var number int
		var at, text sql.NullString
		if err := rows.Scan(&repo, &kind, &number, &where, &author, &at, &text); err != nil {
			return nil, err
		}
		if !re.MatchString(text.String) {
			continue
		}
		in := core.PullRequest(repo, number)
		in.Kind = core.Kind(kind)
		links = append(links, core.Link{To: in, Evidence: []core.Evidence{{
			Source: s.Name(), In: in, Where: where, Author: author, At: rodb.Time(at),
			Excerpt: core.Excerpt(text.String, re, 200),
		}}})
	}
	return links, rows.Err()
}

// syncCommand is the gitcrawl sync that stores what this adapter reads for
// a repository: closed pull requests, and the merge commit of each.
const syncCommand = "gitcrawl sync %s --state all --with pr-metadata"

// Warnings reports a repository of p that gitcrawl has not mirrored, or has
// mirrored without what linking needs: without a sync of closed pull
// requests, merged ones are missing, and without pull request details, their
// merge commits are unknown.
//
// A sync that failed part-way, such as on the GitHub rate limit, stores what
// it fetched and records no run, so a mirror that holds more closed threads
// than open ones counts as synced without a recorded run.
func (s *Source) Warnings(ctx context.Context, p core.Project) []string {
	if p.GitHub == "" {
		return nil
	}
	remedy := "run: " + fmt.Sprintf(syncCommand, p.GitHub)
	var threads, closed, fullSyncs, merged, details int
	err := s.db.QueryRowContext(ctx, `
		select
			(select count(*) from threads t where t.repo_id = r.id),
			(select count(*) from threads t where t.repo_id = r.id and t.state = 'closed'),
			(select count(*) from sync_runs sr
			 where sr.repo_id = r.id and sr.status = 'success' and sr.scope in ('all', 'closed')),
			(select count(*) from threads t where t.repo_id = r.id and t.merged_at_gh is not null),
			(select count(*) from pull_request_details pd where pd.repo_id = r.id)
		from repositories r where r.full_name = ? collate nocase`, p.GitHub).Scan(&threads, &closed, &fullSyncs, &merged, &details)
	if errors.Is(err, sql.ErrNoRows) {
		return []string{fmt.Sprintf("gitcrawl: %s does not mirror %s; %s", s.path, p.GitHub, remedy)}
	}
	if err != nil {
		return []string{fmt.Sprintf("gitcrawl: check %s in %s: %v", p.GitHub, s.path, err)}
	}
	if fullSyncs == 0 && closed < threads-closed {
		return []string{fmt.Sprintf("gitcrawl: %s holds %d threads of %s, %d of them closed, and no sync of closed ones, so merged pull requests may be missing; %s",
			s.path, threads, p.GitHub, closed, remedy)}
	}
	if merged > 0 && details == 0 {
		return []string{fmt.Sprintf("gitcrawl: %s holds %d merged pull requests of %s but no pull request details, so their merge commits are unknown; %s",
			s.path, merged, p.GitHub, remedy)}
	}
	return nil
}

// labelNames extracts the names from gitcrawl's labels_json, an array of
// GitHub label objects.
func labelNames(labelsJSON string) []string {
	var labels []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(labelsJSON), &labels) != nil {
		return nil
	}
	var out []string
	for _, l := range labels {
		out = append(out, l.Name)
	}
	return out
}
