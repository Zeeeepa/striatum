package artifactcontracts

import (
	"strings"
	"testing"
)

// validJoinManifest is a minimal join_manifest.v1 with one staged-live in-edge
// and one quarantined terminal gap, exercising both branches of the in-edge
// provenance contract (RFC 0133 Slice 1 / RFC 0135 P0).
const validJoinManifest = `---
schema_version: "striatum.join_manifest.v1"
artifact_kind: "join_manifest"
barrier_id: "barrier_abc"
entity_kind: "job"
run_id: "run_123"
base_oid: "1111111111111111111111111111111111111111"
tree_sha: "2222222222222222222222222222222222222222"
commit_sha: "3333333333333333333333333333333333333333"
created_at: "2026-06-17T00:00:00Z"
in_edges:
  - entity_id: "wfjob_a"
    seal: 2
    status: "staged_live"
    staging_ref: "refs/striatum/staged/run_123/wfjob_a/2"
    commit_sha: "4444444444444444444444444444444444444444"
  - entity_id: "wfjob_b"
    seal: 1
    status: "quarantined"
    damage_code: "agent_exited_unsealed"
---

# Join manifest
`

func TestJoinManifestKindRegistered(t *testing.T) {
	if !IsAllowedKind("join_manifest") {
		t.Fatal("join_manifest is not an allowed kind")
	}
	if !HasFrontMatterSchema("join_manifest") {
		t.Fatal("join_manifest has no front matter schema")
	}
}

func TestJoinManifestValidAccepted(t *testing.T) {
	if err := ValidateFrontMatter("join_manifest", "JOIN_MANIFEST.md", []byte(validJoinManifest)); err != nil {
		t.Fatalf("valid join_manifest refused: %v", err)
	}
}

func TestJoinManifestRejectsBadFrontMatter(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(string) string
		wantSub string
	}{
		{
			name: "wrong schema_version",
			mutate: func(s string) string {
				return strings.Replace(s, "striatum.join_manifest.v1", "striatum.join_manifest.v2", 1)
			},
			wantSub: "schema_version",
		},
		{
			name:    "missing required commit_sha",
			mutate:  func(s string) string { return removeLine(s, "commit_sha: \"3333") },
			wantSub: "commit_sha",
		},
		{
			name:    "invalid entity_kind names the enum",
			mutate:  func(s string) string { return strings.Replace(s, `entity_kind: "job"`, `entity_kind: "widget"`, 1) },
			wantSub: "job, review_seat, review_obligation, run",
		},
		{
			name:    "empty in_edges",
			mutate:  func(s string) string { return replaceInEdgesWithEmpty(s) },
			wantSub: "at least one in-edge",
		},
		{
			name:    "in-edge missing seal",
			mutate:  func(s string) string { return removeLine(s, "seal: 2") },
			wantSub: "integer 'seal'",
		},
		{
			name:    "staged_live in-edge missing staging_ref",
			mutate:  func(s string) string { return removeLine(s, "staging_ref:") },
			wantSub: "staging_ref",
		},
		{
			name:    "terminal gap missing damage_code",
			mutate:  func(s string) string { return removeLine(s, "damage_code:") },
			wantSub: "damage_code",
		},
		{
			name:    "in-edge bad status",
			mutate:  func(s string) string { return strings.Replace(s, `status: "staged_live"`, `status: "maybe"`, 1) },
			wantSub: "staged_live, quarantined, dead_seat",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := tc.mutate(validJoinManifest)
			err := ValidateFrontMatter("join_manifest", "JOIN_MANIFEST.md", []byte(payload))
			if err == nil {
				t.Fatalf("expected rejection for %s, got nil; payload:\n%s", tc.name, payload)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("rejection for %s must mention %q; got: %v", tc.name, tc.wantSub, err)
			}
		})
	}
}

func removeLine(s, substr string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, substr) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// replaceInEdgesWithEmpty collapses the multi-line in_edges sequence to
// `in_edges: []` so the manifest declares zero in-edges.
func replaceInEdgesWithEmpty(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	skipping := false
	for _, line := range lines {
		if strings.HasPrefix(line, "in_edges:") {
			out = append(out, "in_edges: []")
			skipping = true
			continue
		}
		if skipping {
			// In-edge rows are indented list items (" " or "  - ..."). Stop at the
			// front-matter terminator (---) or any non-indented line — never skip the
			// closing fence.
			if strings.TrimSpace(line) == "---" {
				skipping = false
			} else if strings.HasPrefix(line, " ") {
				continue
			} else {
				skipping = false
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
