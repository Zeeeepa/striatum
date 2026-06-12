package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Truth-mechanization guard (deep architecture review 2026-06-11, P1):
// docs/operator/BRIEF.md is the cold-start orientation surface AGENTS.md
// routes operators through, and it sat 22 releases stale while the
// `operator bootstrap` freshness probe stayed advisory. Reuse the
// production probe here so staleness fails CI instead: the front matter
// must validate against the operator_brief contract, declare status
// current, and the body must mention the current VERSION — bumping
// VERSION now requires refreshing the brief in the same change. The
// latest-tag needle is passed empty because CI checkouts do not fetch
// tags; VERSION is the canonical needle.
func TestOperatorBriefStaysCurrent(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	version, err := os.ReadFile(filepath.Join(repoRoot, "VERSION"))
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	needle := strings.TrimSpace(string(version))
	summary := summarizeOperatorBrief(repoRoot, "", needle, "", 8)
	if summary.Status != "current" {
		t.Fatalf("docs/operator/BRIEF.md is %q (evidence: %s; error: %s); refresh the brief so its front matter validates as operator_brief with status current and its body mentions v%s",
			summary.Status, strings.Join(summary.Evidence, "; "), summary.Error, needle)
	}
}
