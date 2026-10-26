package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vlsi/issuecrawl/core"
	"github.com/vlsi/issuecrawl/internal/cli"
	"github.com/vlsi/issuecrawl/internal/testfixture"
)

// run executes issuecrawl with a config that points at the given mirrors.
func run(t *testing.T, gitcrawlDB, gadakDB string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	config := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(config, fmt.Appendf(nil, `
[sources.gitcrawl]
db = %q
[sources.gadak]
db = %q
[projects.calcite]
github = "apache/calcite"
jira = "CALCITE"
`, gitcrawlDB, gadakDB), 0o600))

	var out, errOut bytes.Buffer
	cmd := cli.NewRootCmd(&out, &errOut)
	cmd.SetArgs(append([]string{"--config", config}, args...))
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

// runOnFixtures executes issuecrawl against the CALCITE-7377 fixture mirrors.
func runOnFixtures(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	return run(t, testfixture.GitcrawlDB(t), testfixture.GadakDB(t), args...)
}

func TestShowJSONFollowsTheSchema(t *testing.T) {
	stdout, _, err := runOnFixtures(t, "show", "CALCITE-7377", "--json")
	require.NoError(t, err)

	var res core.ShowResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), "stdout: %s", stdout)
	assert.Equal(t, "issuecrawl.show.v1", res.Schema)
	assert.Equal(t, "calcite", res.Project)
	assert.Equal(t, "CALCITE-7377", res.Item.Ref.Key)
	var keys []string
	for _, l := range res.Links {
		keys = append(keys, l.To.Key)
	}
	assert.Equal(t, []string{"CALCITE-7890", "apache/calcite#4746", "apache/calcite@fd7220017887c96a46ff604a67954242a67b1513"}, keys)
}

func TestShowTextListsPullRequestsAndCommits(t *testing.T) {
	stdout, _, err := runOnFixtures(t, "show", "CALCITE-7377")
	require.NoError(t, err)

	assert.Contains(t, stdout, "\nPull requests\n  apache/calcite#4746  merged 2026-05-30  [mentioned-by]  [CALCITE-7377] Validator")
	assert.Contains(t, stdout, "\nCommits\n  apache/calcite@fd72200178  [mentions, merged-as]  (not mirrored)\n")
}

func TestShowFailsForAnIssueTheMirrorDoesNotHold(t *testing.T) {
	_, stderr, err := runOnFixtures(t, "show", "CALCITE-1")

	require.Error(t, err)
	assert.Contains(t, stderr, "CALCITE-1: not found in mirror (searched gadak)")
}

// A mirror that cannot be opened becomes a warning, and the command still
// answers from the others.
func TestShowAnswersWithoutTheGitHubMirror(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.db")
	stdout, _, err := run(t, missing, testfixture.GadakDB(t), "show", "CALCITE-7377", "--json")
	require.NoError(t, err)

	var res core.ShowResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), "stdout: %s", stdout)
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0], "gitcrawl: stat "+missing)
	var keys []string
	for _, l := range res.Links {
		keys = append(keys, l.To.Key)
	}
	assert.Equal(t, []string{"CALCITE-7890", "apache/calcite@fd7220017887c96a46ff604a67954242a67b1513"}, keys)
}
