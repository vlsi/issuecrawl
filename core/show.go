package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ShowSchema identifies the JSON shape of [ShowResult].
const ShowSchema = "issuecrawl.show.v1"

// ShowResult is an item together with everything linked to it.
type ShowResult struct {
	Schema   string   `json:"schema"`
	Project  string   `json:"project"`
	Item     *Item    `json:"item"`
	Links    []Link   `json:"links"`
	Warnings []string `json:"warnings,omitempty"`
}

// Show returns the item ref names and its links across sources.
//
// The links are the union of what ref's own text points at, what mentions ref
// in any source, and, for every linked pull request, its merge commit. Links
// to the same target merge into one, with the evidence of both. A target that
// no source holds stays in the result with Mirrored false.
//
// Show fails when no source of ref's system holds ref, or when any source
// returns an error other than [ErrNotFound].
func Show(ctx context.Context, sources []Source, p Project, ref Ref) (*ShowResult, error) {
	item, err := get(ctx, sources, ref)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("%s: %w (searched %s)", ref.Key, ErrNotFound, names(sources, ref.System))
	}
	if item.URL == "" {
		item.URL = defaultURL(p, ref)
	}
	res := &ShowResult{Schema: ShowSchema, Project: p.Name, Item: item, Links: []Link{}}
	for _, s := range sources {
		if s.System() == ref.System {
			links, err := s.Outgoing(ctx, p, ref)
			if err != nil {
				return nil, fmt.Errorf("%s: links of %s: %w", s.Name(), ref.Key, err)
			}
			res.Links = mergeLinks(res.Links, links)
		}
		links, err := s.Mentions(ctx, p, ref)
		if err != nil {
			return nil, fmt.Errorf("%s: mentions of %s: %w", s.Name(), ref.Key, err)
		}
		res.Links = mergeLinks(res.Links, links)
	}
	// Expand pull requests by one hop, to their merge commits. A merge commit
	// is the commit a Jira comment cites, so this joins the two directions.
	for _, l := range slices.Clone(res.Links) {
		if l.To.Kind != KindPullRequest {
			continue
		}
		for _, s := range sources {
			if s.System() != l.To.System {
				continue
			}
			links, err := s.Outgoing(ctx, p, l.To)
			if err != nil {
				return nil, fmt.Errorf("%s: links of %s: %w", s.Name(), l.To.Key, err)
			}
			res.Links = mergeLinks(res.Links, slices.DeleteFunc(links, func(x Link) bool { return x.To.Kind != KindCommit }))
		}
	}
	res.Links = slices.DeleteFunc(res.Links, func(l Link) bool { return l.To.Same(ref) })
	for i := range res.Links {
		l := &res.Links[i]
		l.Rels = rels(ref, l.Evidence)
		target, err := get(ctx, sources, l.To)
		if err != nil {
			return nil, err
		}
		if target != nil {
			target.Body = ""
			l.Item, l.Mirrored, l.URL = target, true, target.URL
		}
		if l.URL == "" {
			l.URL = defaultURL(p, l.To)
		}
	}
	slices.SortStableFunc(res.Links, func(a, b Link) int { return compareRefs(a.To, b.To) })
	if p.GitHub == "" {
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"project %s names no GitHub repository, so every mirrored repository was searched; set projects.%s.github in the config",
			p.Name, p.Name))
	}
	for _, s := range sources {
		res.Warnings = append(res.Warnings, s.Warnings(ctx, p)...)
	}
	return res, nil
}

// get returns the item from the first source of ref's system that holds it,
// or nil when none does.
func get(ctx context.Context, sources []Source, ref Ref) (*Item, error) {
	for _, s := range sources {
		if s.System() != ref.System {
			continue
		}
		item, err := s.Get(ctx, ref)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s: get %s: %w", s.Name(), ref.Key, err)
		}
		return item, nil
	}
	return nil, nil
}

// mergeLinks appends each of add to links, or merges its evidence into the
// link with the same target. Of two commit refs with the same SHA prefix, the
// longer one stays.
func mergeLinks(links, add []Link) []Link {
	for _, a := range add {
		i := slices.IndexFunc(links, func(l Link) bool { return l.To.Same(a.To) })
		if i < 0 {
			links = append(links, Link{To: a.To, Evidence: slices.Clone(a.Evidence)})
			continue
		}
		if len(a.To.Key) > len(links[i].To.Key) {
			links[i].To = a.To
		}
		links[i].Evidence = append(links[i].Evidence, a.Evidence...)
	}
	return links
}

func rels(shown Ref, evidence []Evidence) []string {
	var out []string
	for _, e := range evidence {
		var r string
		switch {
		case e.Where == "issue-link":
			r = RelLinked
		case e.Where == "merge-commit":
			r = RelMergedAs
		case e.In.Same(shown):
			r = RelMentions
		default:
			r = RelMentionedBy
		}
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	slices.Sort(out)
	return out
}

func defaultURL(p Project, ref Ref) string {
	switch {
	case ref.System == Jira && p.JiraURL != "":
		return strings.TrimSuffix(p.JiraURL, "/") + "/browse/" + ref.Key
	case ref.Kind == KindPullRequest:
		repo, num, _ := strings.Cut(ref.Key, "#")
		return "https://github.com/" + repo + "/pull/" + num
	case ref.Kind == KindCommit:
		repo, sha, _ := strings.Cut(ref.Key, "@")
		return "https://github.com/" + repo + "/commit/" + sha
	}
	return ""
}

// compareRefs orders Jira issues before GitHub items, issues before pull
// requests before commits, and issues and pull requests of one project or
// repository by number.
func compareRefs(a, b Ref) int {
	systemRank := map[System]int{Jira: 0, GitHub: 1}
	kindRank := map[Kind]int{KindIssue: 0, KindPullRequest: 1, KindCommit: 2}
	if c := cmp.Or(cmp.Compare(systemRank[a.System], systemRank[b.System]), cmp.Compare(kindRank[a.Kind], kindRank[b.Kind])); c != 0 {
		return c
	}
	sep := "#"
	if a.System == Jira {
		sep = "-"
	}
	if a.Kind != KindCommit {
		pa, na, _ := strings.Cut(a.Key, sep)
		pb, nb, _ := strings.Cut(b.Key, sep)
		ia, errA := strconv.Atoi(na)
		ib, errB := strconv.Atoi(nb)
		if pa == pb && errA == nil && errB == nil {
			return cmp.Compare(ia, ib)
		}
	}
	return cmp.Compare(a.Key, b.Key)
}

func names(sources []Source, system System) string {
	var out []string
	for _, s := range sources {
		if s.System() == system {
			out = append(out, s.Name())
		}
	}
	if len(out) == 0 {
		return "no " + string(system) + " source is configured"
	}
	return strings.Join(out, ", ")
}
