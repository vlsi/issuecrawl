package gadak_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vlsi/issuecrawl/core"
	"github.com/vlsi/issuecrawl/internal/testfixture"
	"github.com/vlsi/issuecrawl/source/gadak"
)

var calcite = core.Project{Name: "calcite", GitHub: "apache/calcite", Jira: "CALCITE"}

func open(t *testing.T, extra ...string) *gadak.Source {
	t.Helper()
	s, err := gadak.Open(testfixture.GadakDB(t, extra...))
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestGetReadsTheIssueFields(t *testing.T) {
	item, err := open(t).Get(context.Background(), core.JiraIssue("calcite-7377"))
	require.NoError(t, err)

	assert.Equal(t, core.JiraIssue("CALCITE-7377"), item.Ref)
	assert.Equal(t, "Closed", item.State)
	assert.Equal(t, "https://issues.apache.org/jira/browse/CALCITE-7377", item.URL)
	assert.Equal(t, []string{"pull-request-available"}, item.Labels)
	assert.Equal(t, map[string]any{
		"issue_type": "Bug", "priority": "Minor", "resolution": "Fixed", "status_category": "done",
		"assignee": "Mihai Budiu", "components": []string{"core"}, "fix_versions": []string{"1.42.0"},
	}, item.Fields)
}

func TestOutgoingReadsIssueLinksAndCommentURLs(t *testing.T) {
	links, err := open(t).Outgoing(context.Background(), calcite, core.JiraIssue("CALCITE-7377"))
	require.NoError(t, err)

	type outgoing struct{ to, where string }
	var got []outgoing
	for _, l := range links {
		got = append(got, outgoing{l.To.Key, l.Evidence[0].Where})
	}
	assert.Equal(t, []outgoing{
		{"CALCITE-7890", "issue-link"},
		{"apache/calcite@fd7220017887c96a46ff604a67954242a67b1513", "comment"},
	}, got)
}

// An issue whose description holds a longer key with the same leading digits
// does not mention the shorter one.
func TestMentionsMatchesTheKeyAsAWholeToken(t *testing.T) {
	s := open(t, `
		insert into items(id, source_id, kind, key, title, body_text) values
		  ('jira:2', 'jira', 'issue', 'CALCITE-7999', 'Unrelated', 'Similar to CALCITE-73770.');
		insert into issues_raw(item_id, key, project_key) values ('jira:2', 'CALCITE-7999', 'CALCITE')`)

	links, err := s.Mentions(context.Background(), calcite, core.JiraIssue("CALCITE-7377"))
	require.NoError(t, err)

	var got []string
	for _, l := range links {
		got = append(got, l.To.Key)
	}
	assert.Equal(t, []string{"CALCITE-7890"}, got)
}

func TestOutgoingReadsURLsInTheDescription(t *testing.T) {
	s := open(t, `update items set body_text = 'Proposed fix: https://github.com/apache/calcite/pull/4746' where key = 'CALCITE-7377'`)

	links, err := s.Outgoing(context.Background(), calcite, core.JiraIssue("CALCITE-7377"))
	require.NoError(t, err)

	var got []string
	for _, l := range links {
		if l.Evidence[0].Where == "body" {
			got = append(got, l.To.Key)
		}
	}
	assert.Equal(t, []string{"apache/calcite#4746"}, got)
}

func TestWarningsNameAJiraProjectTheMirrorHoldsNoIssuesOf(t *testing.T) {
	s := open(t)

	warnings := s.Warnings(context.Background(), core.Project{Name: "hive", Jira: "HIVE"})
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "holds no issues of Jira project HIVE")
	assert.Empty(t, s.Warnings(context.Background(), calcite), "warnings for CALCITE")
}
