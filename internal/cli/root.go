// Package cli implements the issuecrawl command line on top of package core.
// It parses flags, opens the configured sources, and renders results; every
// query lives in core.
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/vlsi/issuecrawl/core"
	"github.com/vlsi/issuecrawl/source/gadak"
	"github.com/vlsi/issuecrawl/source/gitcrawl"
)

type options struct {
	configPath string
	json       bool
}

// Execute runs the command line with the process arguments and returns the
// exit code.
func Execute() int {
	if err := NewRootCmd(os.Stdout, os.Stderr).Execute(); err != nil {
		return 1
	}
	return 0
}

// NewRootCmd returns the issuecrawl command, writing results to stdout and
// errors to stderr.
func NewRootCmd(stdout, stderr io.Writer) *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:   "issuecrawl",
		Short: "Query local mirrors of Jira and GitHub as one tracker",
		Long: `issuecrawl answers questions across local SQLite mirrors of issue trackers:
the Jira mirror of gadak and the GitHub mirror of gitcrawl. It never writes to
the mirrors and never contacts the trackers; sync them with their own tools.`,
		SilenceUsage: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&opts.configPath, "config", "", "config file (default: $XDG_CONFIG_HOME/issuecrawl/config.toml, or ~/.config/issuecrawl/config.toml)")
	root.PersistentFlags().BoolVar(&opts.json, "json", false, "write JSON instead of text")
	root.AddCommand(newShowCmd(opts))
	return root
}

// openSources opens every mirror the config locates. A mirror that fails to
// open is left out and reported in the returned warnings, so that a command
// still answers from the others.
func openSources(cfg *core.Config) (sources []core.Source, warnings []string, closeAll func()) {
	var closers []io.Closer
	if path, err := cfg.GadakDB(); err != nil {
		warnings = append(warnings, fmt.Sprintf("gadak: %v", err))
	} else if s, err := gadak.Open(path); err != nil {
		warnings = append(warnings, err.Error())
	} else {
		sources, closers = append(sources, s), append(closers, s)
	}
	if path, err := cfg.GitcrawlDB(); err != nil {
		warnings = append(warnings, fmt.Sprintf("gitcrawl: %v", err))
	} else if s, err := gitcrawl.Open(path); err != nil {
		warnings = append(warnings, err.Error())
	} else {
		sources, closers = append(sources, s), append(closers, s)
	}
	return sources, warnings, func() {
		for _, c := range closers {
			c.Close()
		}
	}
}
