package core

import (
	"context"
	"errors"
)

// ErrNotFound is returned by [Source.Get] when the mirror does not hold the
// item.
var ErrNotFound = errors.New("not found in mirror")

// Source is a read-only adapter over one local mirror of a tracker.
//
// Every method answers from the mirror alone and performs no network I/O. A
// Source returns links whose Rels, Mirrored, URL, and Item are left for [Show]
// to fill in; it reports only the target and the evidence.
type Source interface {
	// Name identifies the source in evidence and warnings, such as "gadak".
	Name() string
	// System is the tracker whose items Get resolves.
	System() System
	// Get returns the item ref names, or an error wrapping [ErrNotFound].
	Get(ctx context.Context, ref Ref) (*Item, error)
	// Outgoing returns the items that ref's own text or metadata point at:
	// URLs in its description and comments, tracker links, a merge commit.
	// The evidence of each link has In set to ref.
	Outgoing(ctx context.Context, p Project, ref Ref) ([]Link, error)
	// Mentions returns the items in this source, other than ref itself, whose
	// text mentions ref. An adapter may narrow the search to p. The evidence of each link has In set to the
	// mentioning item.
	Mentions(ctx context.Context, p Project, ref Ref) ([]Link, error)
	// Warnings describes gaps in the mirror that make answers about p
	// incomplete, such as a repository that was never synced.
	Warnings(ctx context.Context, p Project) []string
}
