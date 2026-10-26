package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractRefs(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []Ref
	}{
		{
			name: "pull request URL",
			text: "See https://github.com/apache/calcite/pull/4746 for the change.",
			want: []Ref{PullRequest("apache/calcite", 4746)},
		},
		{
			name: "commit URL inside Jira link markup",
			text: "Fixed in [https://github.com/apache/calcite/commit/fd7220017887c96a46ff604a67954242a67b1513]",
			want: []Ref{Commit("apache/calcite", "fd7220017887c96a46ff604a67954242a67b1513")},
		},
		{
			name: "commit URL with a label in Jira link markup",
			text: "Fixed in [fd72200|https://github.com/Apache/Calcite/commit/FD72200178]",
			want: []Ref{Commit("apache/calcite", "fd72200178")},
		},
		{
			name: "gitbox commit URL",
			text: "Fixed in https://gitbox.apache.org/repos/asf?p=calcite.git;a=commit;h=2b2a3e7b2",
			want: []Ref{Commit("apache/calcite", "2b2a3e7b2")},
		},
		{
			name: "git-wip-us commit URL in the gitbox query form",
			text: "Fixed in http://git-wip-us.apache.org/repos/asf?p=calcite.git;a=commit;h=2b2a3e7b2",
			want: []Ref{Commit("apache/calcite", "2b2a3e7b2")},
		},
		{
			name: "git-wip-us commit path",
			text: "Fixed in https://git-wip-us.apache.org/repos/asf/calcite/commit/0c7fb3a1d",
			want: []Ref{Commit("apache/calcite", "0c7fb3a1d")},
		},
		{
			name: "commit SHA of 7 characters is accepted",
			text: "https://github.com/apache/calcite/commit/fd72200",
			want: []Ref{Commit("apache/calcite", "fd72200")},
		},
		{
			name: "commit SHA of 6 characters is ignored",
			text: "https://github.com/apache/calcite/commit/fd7220",
			want: nil,
		},
		{
			name: "gitbox SHA of 6 characters is ignored",
			text: "https://gitbox.apache.org/repos/asf?p=calcite.git;a=commit;h=2b2a3e",
			want: nil,
		},
		{
			name: "repeated URL is reported once, in text order",
			text: "https://github.com/apache/calcite/pull/2 then https://github.com/apache/calcite/pull/1 and https://github.com/apache/calcite/pull/2 again",
			want: []Ref{PullRequest("apache/calcite", 2), PullRequest("apache/calcite", 1)},
		},
		{
			name: "issue URL is not a pull request",
			text: "https://github.com/apache/calcite/issues/12",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []Ref
			for _, r := range ExtractRefs(tt.text) {
				got = append(got, r.Ref)
			}
			assert.Equal(t, tt.want, got, "ExtractRefs(%q)", tt.text)
		})
	}
}

func TestKeyMatcher(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"key in brackets", "[CALCITE-7377] Validator should reject", true},
		{"key at end of text", "Fixes CALCITE-7377", true},
		{"lower-case key", "fixes calcite-7377.", true},
		{"key followed by another digit", "[CALCITE-73770] Unrelated", false},
		{"key preceded by a letter", "XCALCITE-7377", false},
	}
	re := KeyMatcher("CALCITE-7377")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, re.MatchString(tt.text), "KeyMatcher(CALCITE-7377).MatchString(%q)", tt.text)
		})
	}
}

func TestRefSame(t *testing.T) {
	full := Commit("apache/calcite", "fd7220017887c96a46ff604a67954242a67b1513")
	tests := []struct {
		name  string
		other Ref
		want  bool
	}{
		{"abbreviated SHA of the same commit", Commit("apache/calcite", "fd72200178"), true},
		{"same SHA prefix in another repository", Commit("apache/calcite-avatica", "fd72200178"), false},
		{"different SHA", Commit("apache/calcite", "fd72200179"), false},
		{"pull request whose key differs", PullRequest("apache/calcite", 4746), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, full.Same(tt.other), "%s.Same(%s)", full, tt.other)
		})
	}
}

func TestExcerpt(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{"text shorter than the width is returned whole", "Fixes  CALCITE-7377\nnow", 80, "Fixes CALCITE-7377 now"},
		{"cut on both sides is marked", "aaaa CALCITE-7377 bbbb", 4, "…aa CALCITE-7377 bb…"},
		{"cut inside a two-byte rune keeps the whole rune", "ééé CALCITE-7377 ééé", 3, "…é CALCITE-7377 é…"},
		{"no match yields nothing", "CALCITE-73770", 80, ""},
	}
	re := KeyMatcher("CALCITE-7377")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Excerpt(tt.text, re, tt.width), "Excerpt(%q, %d)", tt.text, tt.width)
		})
	}
}
