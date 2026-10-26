package core

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	githubURL = regexp.MustCompile(`(?i)https?://github\.com/([\w.-]+/[\w.-]+)/(pull|commit)/([0-9a-f]+)`)
	// ASF mirrors every GitHub repository apache/<name> as <name>.git on its
	// own git hosts, and older Jira comments cite those URLs.
	gitboxURL = regexp.MustCompile(`(?i)https?://(?:gitbox|git-wip-us)\.apache\.org/repos/asf\?p=([\w.-]+?)\.git;a=commit;h=([0-9a-f]{7,40})`)
	gitwipURL = regexp.MustCompile(`(?i)https?://git-wip-us\.apache\.org/repos/asf/([\w.-]+?)(?:\.git)?/commit/([0-9a-f]{7,40})`)
)

// TextRef is a reference to another item found in free text.
type TextRef struct {
	Ref Ref
	// Match is the text that produced Ref, such as the URL.
	Match string
}

// ExtractRefs returns the GitHub pull request and commit URLs in text, in the
// order they appear, without duplicates. Commit URLs on the ASF git hosts
// (gitbox.apache.org, git-wip-us.apache.org) map to the apache/<name>
// repository on GitHub.
func ExtractRefs(text string) []TextRef {
	type hit struct {
		pos int
		ref TextRef
	}
	var hits []hit
	for _, m := range githubURL.FindAllStringSubmatchIndex(text, -1) {
		repo, kind, id := text[m[2]:m[3]], strings.ToLower(text[m[4]:m[5]]), text[m[6]:m[7]]
		var ref Ref
		switch kind {
		case "pull":
			n, err := strconv.Atoi(id)
			if err != nil {
				continue
			}
			ref = PullRequest(repo, n)
		case "commit":
			if len(id) < 7 {
				continue
			}
			ref = Commit(repo, id)
		}
		hits = append(hits, hit{m[0], TextRef{ref, text[m[0]:m[1]]}})
	}
	for _, re := range []*regexp.Regexp{gitboxURL, gitwipURL} {
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			ref := Commit("apache/"+text[m[2]:m[3]], text[m[4]:m[5]])
			hits = append(hits, hit{m[0], TextRef{ref, text[m[0]:m[1]]}})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return a.pos - b.pos })
	var out []TextRef
	seen := map[Ref]bool{}
	for _, h := range hits {
		if !seen[h.ref.Ref] {
			seen[h.ref.Ref] = true
			out = append(out, h.ref)
		}
	}
	return out
}

// KeyMatcher returns a case-insensitive pattern that matches the Jira key as a
// whole token: CALCITE-7377 matches in "[CALCITE-7377] Fix" and does not match
// in "CALCITE-73770".
func KeyMatcher(key string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])` + regexp.QuoteMeta(key) + `(?:$|[^0-9])`)
}

// Excerpt returns about width characters of text around the first match of
// re, with whitespace collapsed, or "" when re does not match.
func Excerpt(text string, re *regexp.Regexp, width int) string {
	loc := re.FindStringIndex(text)
	if loc == nil {
		return ""
	}
	start := max(0, loc[0]-width/2)
	end := min(len(text), loc[1]+width/2)
	// Step back to rune boundaries so the excerpt stays valid UTF-8.
	for start > 0 && !isRuneStart(text[start]) {
		start--
	}
	for end < len(text) && !isRuneStart(text[end]) {
		end++
	}
	s := strings.Join(strings.Fields(text[start:end]), " ")
	if start > 0 {
		s = "…" + s
	}
	if end < len(text) {
		s += "…"
	}
	return s
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
