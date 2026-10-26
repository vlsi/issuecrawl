package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vlsi/issuecrawl/core"
	"github.com/vlsi/issuecrawl/internal/cli"
)

// TestLiveCalcite7377 runs against the mirrors on this machine, through the
// default config, and is skipped unless ISSUECRAWL_LIVE is set. It needs
// CALCITE-7377 in gadak, and apache/calcite synced in gitcrawl with
// --state all --with pr-metadata.
func TestLiveCalcite7377(t *testing.T) {
	if os.Getenv("ISSUECRAWL_LIVE") == "" {
		t.Skip("set ISSUECRAWL_LIVE=1 to run against the local mirrors")
	}
	var out, errOut bytes.Buffer
	cmd := cli.NewRootCmd(&out, &errOut)
	cmd.SetArgs([]string{"show", "CALCITE-7377", "--json"})
	require.NoError(t, cmd.Execute(), "stderr: %s", errOut.String())

	var res core.ShowResult
	require.NoError(t, json.Unmarshal(out.Bytes(), &res))
	rels := map[string][]string{}
	for _, l := range res.Links {
		rels[l.To.Key] = l.Rels
	}
	assert.Equal(t, []string{core.RelMentionedBy}, rels["apache/calcite#4746"], "rels of apache/calcite#4746")
	commit := "apache/calcite@fd7220017887c96a46ff604a67954242a67b1513"
	assert.True(t, slices.Contains(rels[commit], core.RelMergedAs), "rels of %s: %v", commit, rels[commit])
	assert.Empty(t, res.Warnings)
}
