package gitcrawl_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vlsi/issuecrawl/core"
	"github.com/vlsi/issuecrawl/internal/testfixture"
	"github.com/vlsi/issuecrawl/source/gitcrawl"
)

var calcite = core.Project{Name: "calcite", GitHub: "apache/calcite", Jira: "CALCITE"}

func open(t *testing.T, extra ...string) *gitcrawl.Source {
	t.Helper()
	s, err := gitcrawl.Open(testfixture.GitcrawlDB(t, extra...))
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestGetPullRequestReportsMergeState(t *testing.T) {
	item, err := open(t).Get(context.Background(), core.PullRequest("apache/calcite", 4746))
	require.NoError(t, err)

	assert.Equal(t, "merged", item.State)
	assert.Equal(t, "2026-05-30T07:58:00Z", item.Closed.Format("2006-01-02T15:04:05Z07:00"))
	assert.Equal(t, map[string]any{"merge_commit_sha": "fd7220017887c96a46ff604a67954242a67b1513"}, item.Fields)
	assert.Equal(t, []string{"LGTM-will-merge-soon"}, item.Labels)
}

// A comment and a commit message mention the key too, and a pull request
// whose title holds a longer key that starts with the same digits does not.
func TestMentionsMatchesTheKeyAsAWholeToken(t *testing.T) {
	s := open(t, `
		insert into threads(id, repo_id, github_id, number, kind, state, title, html_url,
		                    labels_json, assignees_json, raw_json, content_hash, updated_at) values
		  (4800, 2, '3170000002', 4800, 'pull_request', 'open', '[CALCITE-73770] Unrelated change',
		   'https://github.com/apache/calcite/pull/4800', '[]', '[]', '{}', 'h4800', '2026-10-26T00:00:00Z'),
		  (4801, 2, '3170000003', 4801, 'pull_request', 'open', 'Follow-up',
		   'https://github.com/apache/calcite/pull/4801', '[]', '[]', '{}', 'h4801', '2026-10-26T00:00:00Z'),
		  (4802, 2, '3170000004', 4802, 'issue', 'open', 'Crash in CALCITE-7377 validator',
		   'https://github.com/apache/calcite/issues/4802', '[]', '[]', '{}', 'h4802', '2026-10-26T00:00:00Z');
		insert into comments(thread_id, github_id, comment_type, author_login, body, raw_json, created_at_gh) values
		  (4801, 'c1', 'issue_comment', 'julianhyde', 'This continues CALCITE-7377.', '{}', '2026-10-01T00:00:00Z');
		insert into pull_request_commits(thread_id, sha, message, author_login, raw_json, fetched_at) values
		  (4800, 'aaaaaaa1', '[CALCITE-7377] cherry-picked', 'someone', '{}', '2026-10-26T00:00:00Z')`)

	links, err := s.Mentions(context.Background(), calcite, core.JiraIssue("CALCITE-7377"))
	require.NoError(t, err)

	type mention struct {
		in    string
		kind  core.Kind
		where string
	}
	var got []mention
	for _, l := range links {
		got = append(got, mention{l.Evidence[0].In.Key, l.Evidence[0].In.Kind, l.Evidence[0].Where})
	}
	assert.ElementsMatch(t, []mention{
		{"apache/calcite#4746", core.KindPullRequest, "title"},
		{"apache/calcite#4746", core.KindPullRequest, "body"},
		{"apache/calcite#4800", core.KindPullRequest, "commit"},
		{"apache/calcite#4801", core.KindPullRequest, "comment"},
		{"apache/calcite#4802", core.KindIssue, "title"},
	}, got)
}

func TestOutgoingOfAnOpenPullRequestHasNoMergeCommit(t *testing.T) {
	s := open(t, `update threads set state = 'open', closed_at_gh = null, merged_at_gh = null where number = 4746`)

	links, err := s.Outgoing(context.Background(), calcite, core.PullRequest("apache/calcite", 4746))
	require.NoError(t, err)
	assert.Empty(t, links)
}

func TestWarningsNameAnUnmirroredRepository(t *testing.T) {
	warnings := open(t).Warnings(context.Background(), core.Project{Name: "avatica", GitHub: "apache/calcite-avatica", Jira: "CALCITE"})

	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "does not mirror apache/calcite-avatica")
}

// Without hydrated details, gitcrawl keeps the pull request object, and so
// the merge commit, in the thread's raw_json.
func TestGetPullRequestReadsTheMergeCommitFromTheThreadWithoutDetails(t *testing.T) {
	s := open(t, `
		delete from pull_request_details;
		update threads set raw_json = '{"merge_commit_sha":"fd7220017887c96a46ff604a67954242a67b1513"}' where number = 4746`)

	item, err := s.Get(context.Background(), core.PullRequest("apache/calcite", 4746))
	require.NoError(t, err)
	assert.Equal(t, "fd7220017887c96a46ff604a67954242a67b1513", item.Fields["merge_commit_sha"])
}

func TestGetCommitResolvesAnAbbreviatedSHAFromPullRequestCommits(t *testing.T) {
	s := open(t, `
		insert into pull_request_commits(thread_id, sha, message, author_login, committed_at, html_url, raw_json, fetched_at) values
		  (4746, 'fd7220017887c96a46ff604a67954242a67b1513',
		   '[CALCITE-7377] Validator should reject a DESCRIPTOR in a table function when it is not an identifier

Signed-off-by: Mihai Budiu', 'mihaibudiu', '2026-05-30T07:58:00Z',
		   'https://github.com/apache/calcite/commit/fd7220017887c96a46ff604a67954242a67b1513', '{}', '2026-10-26T00:00:00Z')`)

	item, err := s.Get(context.Background(), core.Commit("apache/calcite", "fd72200178"))
	require.NoError(t, err)

	assert.Equal(t, core.Commit("apache/calcite", "fd7220017887c96a46ff604a67954242a67b1513"), item.Ref)
	assert.Equal(t, "[CALCITE-7377] Validator should reject a DESCRIPTOR in a table function when it is not an identifier", item.Title)
	assert.Equal(t, "mihaibudiu", item.Author)
	assert.Equal(t, map[string]any{"pull_request": "apache/calcite#4746"}, item.Fields)
}

func TestGetCommitNotInAnyPullRequestIsNotFound(t *testing.T) {
	_, err := open(t).Get(context.Background(), core.Commit("apache/calcite", "fd72200178"))
	assert.ErrorIs(t, err, core.ErrNotFound)
}

func TestWarningsNameMergedPullRequestsWithoutDetails(t *testing.T) {
	warnings := open(t, `delete from pull_request_details`).Warnings(context.Background(), calcite)

	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "holds 1 merged pull requests of apache/calcite but no pull request details")
	assert.Contains(t, warnings[0], "run: gitcrawl sync apache/calcite --state all --with pr-metadata")
}

func TestWarningsAreEmptyForAFullyMirroredRepository(t *testing.T) {
	assert.Empty(t, open(t).Warnings(context.Background(), calcite))
}
