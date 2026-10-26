package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vlsi/issuecrawl/core"
	"github.com/vlsi/issuecrawl/internal/testfixture"
	"github.com/vlsi/issuecrawl/source/gadak"
	"github.com/vlsi/issuecrawl/source/gitcrawl"
)

var calcite = core.Project{Name: "calcite", GitHub: "apache/calcite", Jira: "CALCITE", JiraURL: "https://issues.apache.org/jira"}

func openSources(t *testing.T, gadakPath, gitcrawlPath string) []core.Source {
	t.Helper()
	jira, err := gadak.Open(gadakPath)
	require.NoError(t, err)
	t.Cleanup(func() { jira.Close() })
	github, err := gitcrawl.Open(gitcrawlPath)
	require.NoError(t, err)
	t.Cleanup(func() { github.Close() })
	return []core.Source{jira, github}
}

// linkSummary is the part of a link that the linking rules decide.
type linkSummary struct {
	Key      string
	Rels     []string
	Mirrored bool
}

func summarize(links []core.Link) []linkSummary {
	var out []linkSummary
	for _, l := range links {
		out = append(out, linkSummary{l.To.Key, l.Rels, l.Mirrored})
	}
	return out
}

// The Jira comment cites the commit by URL, and the pull request names the
// issue in its title and was merged as that commit; Show joins the two into
// one commit link carrying the evidence of both.
func TestShowJoinsTheCommitACommentCitesWithThePullRequestMergedAsIt(t *testing.T) {
	sources := openSources(t, testfixture.GadakDB(t), testfixture.GitcrawlDB(t))

	res, err := core.Show(context.Background(), sources, calcite, core.JiraIssue("CALCITE-7377"))
	require.NoError(t, err)

	assert.Equal(t, "Validator should reject a DESCRIPTOR in a table function when it is not an identifier", res.Item.Title)
	assert.Equal(t, []linkSummary{
		{"CALCITE-7890", []string{core.RelLinked, core.RelMentionedBy}, true},
		{"apache/calcite#4746", []string{core.RelMentionedBy}, true},
		{"apache/calcite@fd7220017887c96a46ff604a67954242a67b1513", []string{core.RelMentions, core.RelMergedAs}, false},
	}, summarize(res.Links))
	assert.Empty(t, res.Warnings)
}

func TestShowKeepsTheBodyOfTheShownItemOnly(t *testing.T) {
	sources := openSources(t, testfixture.GadakDB(t), testfixture.GitcrawlDB(t))

	res, err := core.Show(context.Background(), sources, calcite, core.JiraIssue("CALCITE-7377"))
	require.NoError(t, err)

	assert.Contains(t, res.Item.Body, "DESCRIPTOR(1 + 2)")
	for _, l := range res.Links {
		if l.Item != nil {
			assert.Empty(t, l.Item.Body, "body of linked %s", l.To.Key)
		}
	}
}

func TestShowReportsAnIssueTheMirrorDoesNotHold(t *testing.T) {
	sources := openSources(t, testfixture.GadakDB(t), testfixture.GitcrawlDB(t))

	_, err := core.Show(context.Background(), sources, calcite, core.JiraIssue("CALCITE-1"))
	assert.ErrorIs(t, err, core.ErrNotFound)
}

// gitcrawl syncs only open pull requests unless asked for all, and a merged
// pull request is then missing without any other sign.
func TestShowWarnsWhenGitcrawlNeverSyncedClosedPullRequests(t *testing.T) {
	sources := openSources(t, testfixture.GadakDB(t),
		testfixture.GitcrawlDB(t, `update sync_runs set scope = 'open'`))

	res, err := core.Show(context.Background(), sources, calcite, core.JiraIssue("CALCITE-7377"))
	require.NoError(t, err)

	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0], "no sync of closed ones")
	assert.Contains(t, res.Warnings[0], "run: gitcrawl sync apache/calcite --state all --with pr-metadata")
}

// A Jira comment usually cites a commit by an abbreviated SHA; it is the same
// commit as the full merge SHA gitcrawl stores, and the link keeps the full one.
func TestShowJoinsAnAbbreviatedCommitWithTheFullMergeCommit(t *testing.T) {
	sources := openSources(t,
		testfixture.GadakDB(t, `update comments set body_text = 'Fixed in https://github.com/apache/calcite/commit/fd72200178' where id = '2'`),
		testfixture.GitcrawlDB(t))

	res, err := core.Show(context.Background(), sources, calcite, core.JiraIssue("CALCITE-7377"))
	require.NoError(t, err)

	assert.Equal(t, []linkSummary{
		{"CALCITE-7890", []string{core.RelLinked, core.RelMentionedBy}, true},
		{"apache/calcite#4746", []string{core.RelMentionedBy}, true},
		{"apache/calcite@fd7220017887c96a46ff604a67954242a67b1513", []string{core.RelMentions, core.RelMergedAs}, false},
	}, summarize(res.Links))
}

func TestShowMergesAPullRequestTheIssueCitesWithOneThatMentionsIt(t *testing.T) {
	sources := openSources(t,
		testfixture.GadakDB(t, `insert into comments(id, item_id, author, body_text, created_at) values
			('3', 'jira:13639034', 'Mihai Budiu', 'PR: https://github.com/apache/calcite/pull/4746', '2026-01-15T02:00:00.000Z')`),
		testfixture.GitcrawlDB(t))

	res, err := core.Show(context.Background(), sources, calcite, core.JiraIssue("CALCITE-7377"))
	require.NoError(t, err)

	require.Len(t, res.Links, 3)
	assert.Equal(t, linkSummary{"apache/calcite#4746", []string{core.RelMentionedBy, core.RelMentions}, true}, summarize(res.Links)[1])
}

func TestShowWarnsWhenTheProjectNamesNoRepository(t *testing.T) {
	sources := openSources(t, testfixture.GadakDB(t), testfixture.GitcrawlDB(t))
	adHoc := core.Project{Name: "calcite", Jira: "CALCITE"}

	res, err := core.Show(context.Background(), sources, adHoc, core.JiraIssue("CALCITE-7377"))
	require.NoError(t, err)

	require.Len(t, res.Links, 3)
	assert.Equal(t, []string{"project calcite names no GitHub repository, so every mirrored repository was searched; set projects.calcite.github in the config"}, res.Warnings)
	assert.Equal(t, "apache/calcite#4746", summarize(res.Links)[1].Key, "pull request found without a configured repository")
}
