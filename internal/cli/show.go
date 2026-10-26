package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vlsi/issuecrawl/core"
)

func newShowCmd(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "show JIRA-KEY",
		Short: "Show a Jira issue with its linked pull requests, commits, and issues",
		Long: `show prints a Jira issue together with everything linked to it in either
direction: pull requests and commits its comments cite, pull requests whose
title, body, comments, or commit messages mention its key, the merge commits
of those pull requests, and Jira issue links.`,
		Example: `  issuecrawl show CALCITE-7377
  issuecrawl show CALCITE-7377 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := core.LoadConfig(opts.configPath)
			if err != nil {
				return err
			}
			ref := core.JiraIssue(args[0])
			p, err := cfg.ProjectForKey(ref.Key)
			if err != nil {
				return err
			}
			sources, warnings, closeAll := openSources(cfg)
			defer closeAll()
			res, err := core.Show(cmd.Context(), sources, p, ref)
			if err != nil {
				for _, w := range warnings {
					fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
				}
				return err
			}
			res.Warnings = append(warnings, res.Warnings...)
			if opts.json {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			renderShow(cmd.OutOrStdout(), res)
			return nil
		},
	}
}

func renderShow(w io.Writer, res *core.ShowResult) {
	it := res.Item
	fmt.Fprintf(w, "%s  %s\n", it.Ref.Key, it.Title)
	var facts []string
	state := it.State
	if r, ok := it.Fields["resolution"].(string); ok {
		state += " (" + r + ")"
	}
	facts = append(facts, state)
	for _, f := range []string{"issue_type", "priority"} {
		if v, ok := it.Fields[f].(string); ok {
			facts = append(facts, v)
		}
	}
	if v, ok := it.Fields["fix_versions"].([]string); ok {
		facts = append(facts, "fix "+strings.Join(v, ", "))
	}
	fmt.Fprintf(w, "  %s\n", strings.Join(facts, " · "))
	var people []string
	if it.Author != "" {
		people = append(people, "reporter "+it.Author)
	}
	if a, ok := it.Fields["assignee"].(string); ok {
		people = append(people, "assignee "+a)
	}
	if it.Created != nil {
		people = append(people, "created "+day(it.Created))
	}
	if it.Closed != nil {
		people = append(people, "resolved "+day(it.Closed))
	}
	fmt.Fprintf(w, "  %s\n", strings.Join(people, " · "))
	if it.URL != "" {
		fmt.Fprintf(w, "  %s\n", it.URL)
	}

	sections := []struct {
		title string
		kind  core.Kind
		sys   core.System
	}{
		{"Jira issues", core.KindIssue, core.Jira},
		{"Pull requests", core.KindPullRequest, core.GitHub},
		{"GitHub issues", core.KindIssue, core.GitHub},
		{"Commits", core.KindCommit, core.GitHub},
	}
	for _, sec := range sections {
		header := false
		for _, l := range res.Links {
			if l.To.Kind != sec.kind || l.To.System != sec.sys {
				continue
			}
			if !header {
				fmt.Fprintf(w, "\n%s\n", sec.title)
				header = true
			}
			title, state := "(not mirrored)", ""
			if l.Item != nil {
				title, state = l.Item.Title, l.Item.State
				if l.Item.Closed != nil {
					state += " " + day(l.Item.Closed)
				}
			}
			line := []string{shortKey(l.To)}
			if state != "" {
				line = append(line, state)
			}
			line = append(line, "["+strings.Join(l.Rels, ", ")+"]", title)
			fmt.Fprintf(w, "  %s\n", strings.Join(line, "  "))
			fmt.Fprintf(w, "    %s\n", l.URL)
			for _, e := range l.Evidence {
				fmt.Fprintf(w, "    - %s\n", describe(e))
			}
		}
	}
	if len(res.Warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings")
		for _, warning := range res.Warnings {
			fmt.Fprintf(w, "  %s\n", warning)
		}
	}
}

func describe(e core.Evidence) string {
	s := e.Where
	if e.Where == "issue-link" {
		return "issue link: " + e.Excerpt
	}
	s += " of " + shortKey(e.In)
	if e.Author != "" {
		s += " by " + e.Author
	}
	if e.At != nil {
		s += " on " + day(e.At)
	}
	if e.Excerpt != "" {
		s += ": " + truncate(e.Excerpt, 120)
	}
	return s
}

// shortKey abbreviates a commit SHA to 10 characters.
func shortKey(r core.Ref) string {
	if repo, sha, ok := strings.Cut(r.Key, "@"); ok && r.Kind == core.KindCommit && len(sha) > 10 {
		return repo + "@" + sha[:10]
	}
	return r.Key
}

func day(t *time.Time) string { return t.Format(time.DateOnly) }

// truncate shortens s to at most n runes, marking the cut with an ellipsis.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
