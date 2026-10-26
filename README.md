# issuecrawl

issuecrawl answers questions across local mirrors of issue trackers with one command. Give it a Jira issue key, and it
prints the issue together with the GitHub pull requests and commits linked to it, in either direction, as text or JSON.
It is built for maintainers and for coding agents that need the whole story of a ticket in one call.

issuecrawl reads two SQLite mirrors that other tools maintain, and never writes to them or contacts the trackers:

| Tracker | Mirror tool | Database |
| --- | --- | --- |
| Jira | [gadak](https://github.com/midagedev/gadak) | `~/.gadak/profiles/<workspace>/gadak.db` |
| GitHub | [gitcrawl](https://github.com/openclaw/gitcrawl) | `gitcrawl/gitcrawl.db` in the user config directory (`~/Library/Application Support` on macOS) |

## Install

issuecrawl needs Go 1.27 or later.

```bash
go install github.com/vlsi/issuecrawl/cmd/issuecrawl@latest
```

From a clone, run `go install ./cmd/issuecrawl` instead.

## Prepare the mirrors

Sync the Jira project with gadak and the GitHub repository with gitcrawl. A plain `gitcrawl sync` mirrors only open pull
requests, so merged pull requests are missing until you sync closed ones too:

```bash
gitcrawl sync apache/calcite --state all --with pr-metadata
```

`--with pr-metadata` stores each pull request's merge commit, which issuecrawl needs to match a pull request with the
commit a Jira comment cites. issuecrawl prints a warning, with this command, when a project's repository is missing from
gitcrawl, was never synced with `--state all`, or has merged pull requests without their details.

With that sync, issuecrawl finds the pull requests that mention a key in their title or description. Two more gitcrawl
options make it search more text, at the cost of more GitHub requests per pull request:

| gitcrawl option | What issuecrawl also searches |
| --- | --- |
| `--include-comments` | Pull request comments and reviews |
| `--with pr-details` (instead of `pr-metadata`) | Commit messages of each pull request; it also lets `show` print the subject of a merge commit |

## Show an issue

```bash
issuecrawl show CALCITE-7377
```

```text
CALCITE-7377  Validator should reject a DESCRIPTOR in a table function when it is not an identifier
  Closed (Fixed) · Bug · Minor · fix 1.42.0
  reporter Mihai Budiu · assignee Mihai Budiu · created 2026-01-14 · resolved 2026-05-31
  https://issues.apache.org/jira/browse/CALCITE-7377

Jira issues
  CALCITE-7890  Open  [linked, mentioned-by]  SqlDescriptorOperator.checkOperandTypes rejects simple identifiers and is never called
    https://issues.apache.org/jira/browse/CALCITE-7890
    - issue link: Reference (inward)
    - body of CALCITE-7890: …which CALCITE-7377 added…

Pull requests
  apache/calcite#4746  merged 2026-05-30  [mentioned-by]  [CALCITE-7377] Validator should reject a DESCRIPTOR in a table function when it is not an identifier
    https://github.com/apache/calcite/pull/4746
    - title of apache/calcite#4746 by mihaibudiu on 2026-01-15: [CALCITE-7377] Validator should reject…

Commits
  apache/calcite@fd72200178  [mentions, merged-as]  (not mirrored)
    https://github.com/apache/calcite/commit/fd7220017887c96a46ff604a67954242a67b1513
    - comment of CALCITE-7377 by Mihai Budiu on 2026-05-30: Fixed in [https://github.com/apache/calcite/commit/fd72200…]
    - merge-commit of apache/calcite#4746 on 2026-05-30
```

A link appears when any of these holds:

| Relation | Meaning |
| --- | --- |
| `linked` | A Jira issue link connects the two issues. |
| `mentions` | The issue's description or a comment holds the URL of a GitHub pull request or commit, including the ASF gitbox and git-wip-us commit URLs. |
| `mentioned-by` | A Jira issue, or a GitHub pull request or issue, holds the key in its title, description, a comment, or a commit message. The key matches as a whole token, so `CALCITE-73770` does not mention `CALCITE-7377`. |
| `merged-as` | The commit is the merge commit of a linked pull request. |

Links to the same target merge into one, with every piece of evidence listed. A target that no mirror holds, such as a
commit, still appears, marked `(not mirrored)`, with its URL.

`show` exits with status 1 when the Jira mirror does not hold the issue.

### JSON output

`--json` prints the same result as one object, for agents and scripts:

```json
{
  "schema": "issuecrawl.show.v1",
  "project": "calcite",
  "item": { "ref": { "system": "jira", "kind": "issue", "key": "CALCITE-7377" }, "title": "…", "state": "Closed", "url": "…", "fields": { "resolution": "Fixed", "fix_versions": ["1.42.0"] }, "body": "…" },
  "links": [
    {
      "to": { "system": "github", "kind": "pull_request", "key": "apache/calcite#4746" },
      "rels": ["mentioned-by"],
      "mirrored": true,
      "url": "https://github.com/apache/calcite/pull/4746",
      "item": { "title": "…", "state": "merged", "fields": { "merge_commit_sha": "fd7220017887c96a46ff604a67954242a67b1513" } },
      "evidence": [ { "source": "gitcrawl", "in": { "system": "github", "kind": "pull_request", "key": "apache/calcite#4746" }, "where": "title", "excerpt": "…" } ]
    }
  ]
}
```

- `schema` names the shape; a change that breaks it gets a new name.
- `item.body` is the issue description. Linked items carry no body.
- `ref.key` is `PROJECT-123` for a Jira issue, `owner/repo#123` for a pull request or GitHub issue, and `owner/repo@sha`
  for a commit.
- `evidence[].where` is one of `title`, `body`, `comment`, `commit`, `issue-link`, `merge-commit`.
- `warnings` lists gaps in the mirrors that can make the answer incomplete. It is omitted when empty.

## Configuration

issuecrawl reads `~/.config/issuecrawl/config.toml`, or `$XDG_CONFIG_HOME/issuecrawl/config.toml` when that variable
is set, on macOS as on Linux. `--config` names another file. Every setting is optional.

```toml
[sources.gitcrawl]
db = "~/Library/Application Support/gitcrawl/gitcrawl.db"

[sources.gadak]
workspace = "asf"            # or: db = "~/.gadak/profiles/asf/gadak.db"

[projects.calcite]
github = "apache/calcite"
jira = "CALCITE"
jira_url = "https://issues.apache.org/jira"
key_pattern = 'CALCITE-\d+'  # the default is <jira>-<digits>, case-insensitive
```

| Setting | Default | Meaning |
| --- | --- | --- |
| `sources.gitcrawl.db` | gitcrawl's default database path | The gitcrawl database. |
| `sources.gadak.db` | unset | The gadak database. Takes precedence over `workspace`. |
| `sources.gadak.workspace` | the only workspace in `~/.gadak/profiles` | The gadak workspace whose `gadak.db` to read. With several workspaces and neither setting, issuecrawl reports an error that lists them. |
| `projects.<name>.jira` | required | The Jira project key. |
| `projects.<name>.github` | unset | The repository, as `owner/name`, that holds the project's pull requests. When unset, every repository in gitcrawl is searched and a warning says so. |
| `projects.<name>.jira_url` | unset | The Jira base URL, used for issue links when the mirror stores none. |
| `projects.<name>.key_pattern` | `<jira>-\d+`, case-insensitive | A regular expression that the project's issue keys match in full. issuecrawl picks the project whose pattern matches the key it is given, checking projects in name order. |

A key that matches no project still works: issuecrawl treats its prefix as the Jira project and searches every GitHub
repository.

## Development

```bash
go test ./...
```

The tests build small gitcrawl and gadak databases from schemas copied out of the real tools (`internal/testfixture`).
To run the end-to-end check against the mirrors on your machine, set `ISSUECRAWL_LIVE`:

```bash
ISSUECRAWL_LIVE=1 go test ./internal/cli -run Live
```

The code is split into `core` (the domain model, the `Source` interface, and the cross-source queries), one read-only
adapter per mirror under `source/`, and a thin [cobra](https://github.com/spf13/cobra) command line under
`internal/cli`.
