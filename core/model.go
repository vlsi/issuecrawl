// Package core holds the issuecrawl domain model, the [Source] interface that
// every mirror adapter implements, project configuration, and the queries that
// combine several sources, such as [Show].
//
// core never opens a database itself: callers construct the adapters (see the
// source/gadak and source/gitcrawl packages) and pass them in, so the query
// logic is testable against any [Source].
package core

import (
	"strconv"
	"strings"
	"time"
)

// System names a tracker that a [Ref] belongs to.
type System string

// The systems issuecrawl links between. A commit belongs to [GitHub], because
// its key names the GitHub repository it was pushed to.
const (
	Jira   System = "jira"
	GitHub System = "github"
)

// Kind is the type of a tracked item within its [System].
type Kind string

const (
	KindIssue       Kind = "issue"
	KindPullRequest Kind = "pull_request"
	KindCommit      Kind = "commit"
)

// Ref identifies an item independently of the mirror that holds it.
//
// Key formats:
//
//	jira issue           CALCITE-7377
//	github pull_request  apache/calcite#4746
//	github commit        apache/calcite@fd7220017887c96a46ff604a67954242a67b1513
//
// A commit key may carry an abbreviated SHA; [Ref.Same] matches it against the
// full one.
type Ref struct {
	System System `json:"system"`
	Kind   Kind   `json:"kind"`
	Key    string `json:"key"`
}

// JiraIssue returns the Ref of a Jira issue. The key is upper-cased.
func JiraIssue(key string) Ref {
	return Ref{System: Jira, Kind: KindIssue, Key: strings.ToUpper(key)}
}

// PullRequest returns the Ref of a GitHub pull request in repo ("owner/name").
func PullRequest(repo string, number int) Ref {
	return Ref{System: GitHub, Kind: KindPullRequest, Key: strings.ToLower(repo) + "#" + strconv.Itoa(number)}
}

// Commit returns the Ref of a commit in the GitHub repository repo. The SHA is
// lower-cased and may be abbreviated.
func Commit(repo, sha string) Ref {
	return Ref{System: GitHub, Kind: KindCommit, Key: strings.ToLower(repo) + "@" + strings.ToLower(sha)}
}

// String formats r as "system:kind:key".
func (r Ref) String() string {
	return string(r.System) + ":" + string(r.Kind) + ":" + r.Key
}

// Same reports whether r and o name the same item. Two commit refs in one
// repository are the same when one SHA is a prefix of the other.
func (r Ref) Same(o Ref) bool {
	if r == o {
		return true
	}
	if r.System != o.System || r.Kind != KindCommit || o.Kind != KindCommit {
		return false
	}
	repoA, shaA, _ := strings.Cut(r.Key, "@")
	repoB, shaB, _ := strings.Cut(o.Key, "@")
	return repoA == repoB && (strings.HasPrefix(shaA, shaB) || strings.HasPrefix(shaB, shaA))
}

// Repo returns the "owner/name" part of a GitHub pull request or commit key,
// or "" for any other ref.
func (r Ref) Repo() string {
	if r.System != GitHub {
		return ""
	}
	if repo, _, ok := strings.Cut(r.Key, "#"); ok && r.Kind == KindPullRequest {
		return repo
	}
	if repo, _, ok := strings.Cut(r.Key, "@"); ok && r.Kind == KindCommit {
		return repo
	}
	return ""
}

// Item is one issue, pull request, or commit as a mirror stores it.
type Item struct {
	Ref     Ref        `json:"ref"`
	Title   string     `json:"title"`
	State   string     `json:"state,omitempty"`
	URL     string     `json:"url,omitempty"`
	Author  string     `json:"author,omitempty"`
	Created *time.Time `json:"created,omitempty"`
	Updated *time.Time `json:"updated,omitempty"`
	// Closed is when a Jira issue was resolved, or a pull request closed or merged.
	Closed *time.Time `json:"closed,omitempty"`
	Labels []string   `json:"labels,omitempty"`
	// Fields holds the source-specific attributes that have no common meaning
	// across trackers, such as a Jira resolution or a pull request's merge
	// commit. Each adapter documents the names it sets.
	Fields map[string]any `json:"fields,omitempty"`
	// Body is the description text. [Show] keeps it only for the item it was
	// asked about.
	Body string `json:"body,omitempty"`
}

// Relations between the item [Show] was asked about and a linked item.
const (
	// RelLinked is an explicit tracker link, such as a Jira issue link.
	RelLinked = "linked"
	// RelMentions means the shown item's text mentions the linked item.
	RelMentions = "mentions"
	// RelMentionedBy means the linked item's text mentions the shown item.
	RelMentionedBy = "mentioned-by"
	// RelMergedAs means the linked commit is the merge commit of a pull
	// request that is itself linked.
	RelMergedAs = "merged-as"
)

// Evidence is one observation that supports a [Link].
type Evidence struct {
	// Source is the name of the [Source] that reported the observation.
	Source string `json:"source"`
	// In is the item whose text or metadata carries the observation.
	In Ref `json:"in"`
	// Where is the part of In that matched: title, body, comment, commit,
	// issue-link, or merge-commit.
	Where   string     `json:"where"`
	Author  string     `json:"author,omitempty"`
	At      *time.Time `json:"at,omitempty"`
	Excerpt string     `json:"excerpt,omitempty"`
}

// Link connects the shown item to another one.
type Link struct {
	To Ref `json:"to"`
	// Rels is the set of Rel* constants that the evidence establishes, sorted.
	Rels []string `json:"rels"`
	// Mirrored reports whether a local mirror holds the target. When it does
	// not, Item is nil and URL still points at the target.
	Mirrored bool       `json:"mirrored"`
	URL      string     `json:"url,omitempty"`
	Item     *Item      `json:"item,omitempty"`
	Evidence []Evidence `json:"evidence"`
}
