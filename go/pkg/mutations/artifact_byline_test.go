package mutations

import (
	"strings"
	"testing"
)

// TestCanonicalBylineFormPreservesLabelUnderscores is the #143 gate. The
// file-side byline canonicalization must NOT strip underscores that are
// legitimate inside an author label — operator self-declared labels and role
// ids (reviewer_ops_tests, findings_ledger) routinely contain them. Markdown
// emphasis (*bold*/_italic_) WRAPS a byline, so it is neutralized by trimming
// the wrappers, never by deleting interior underscores.
func TestCanonicalBylineFormPreservesLabelUnderscores(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "operator self-declared label with underscores (#143 repro)",
			in:   "author: operator [self-declared: 0058-ops_tests-fin]",
			want: "author: operator [self-declared: 0058-ops_tests-fin]",
		},
		{
			name: "role byline with underscores",
			in:   "author: reviewer_ops_tests-claude-007",
			want: "author: reviewer_ops_tests-claude-007",
		},
		{
			name: "bold-wrapped byline neutralized, label intact",
			in:   "**author: findings_ledger-codex-002**",
			want: "author: findings_ledger-codex-002",
		},
		{
			name: "italic-wrapped underscore label: emphasis trimmed, label kept",
			in:   "_author: operator [self-declared: ops_tests]_",
			want: "author: operator [self-declared: ops_tests]",
		},
		{
			name: "heading prefix + casing canonicalized",
			in:   "# Author: Reviewer-Claude-007",
			want: "author: reviewer-claude-007",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalBylineForm(tc.in); got != tc.want {
				t.Fatalf("canonicalBylineForm(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCanonicalBylineFormMatchesLowercasedPlainLine pins the cross-side
// invariant the #143 mismatch broke: validateMarkdownAuthorLine compares the
// file's canonicalBylineForm against the packet's expectedAuthorLine, which is
// just strings.ToLower of the raw byline. For a plain (un-emphasized) byline the
// two canonicalizations must agree — otherwise a byte-correct byline is rejected
// as "is X but the work packet requires Y" where X and Y differ only by a
// stripped underscore.
func TestCanonicalBylineFormMatchesLowercasedPlainLine(t *testing.T) {
	for _, plain := range []string{
		"author: operator [self-declared: 0058-ops_tests-fin]",
		"author: reviewer_ops_tests-claude-007",
		"author: findings_ledger-codex-002",
	} {
		if got, want := canonicalBylineForm(plain), strings.ToLower(plain); got != want {
			t.Fatalf("canonicalBylineForm(%q) = %q but the packet side (expectedAuthorLine) derives %q — the two sides disagree, which is the #143 publish rejection", plain, got, want)
		}
	}
}
