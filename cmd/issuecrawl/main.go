// Issuecrawl answers questions across local SQLite mirrors of Jira and GitHub.
// Run "issuecrawl help" for the commands.
package main

import (
	"os"

	"github.com/vlsi/issuecrawl/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
