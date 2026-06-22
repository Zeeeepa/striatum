package workflowauthoring

import (
	"errors"
	"strings"
	"testing"
)

// jobWithAllowedPaths builds a minimal workflow map with a single job whose
// write_scope.allowed_paths is the given list. It is intentionally NOT a fully
// valid workflow — RefuseCrossRepoWriteScope runs on the unvalidated shape.
func jobWithAllowedPaths(jobID string, allowed ...string) map[string]any {
	pathsAny := make([]any, 0, len(allowed))
	for _, p := range allowed {
		pathsAny = append(pathsAny, p)
	}
	return map[string]any{
		"jobs": []any{
			map[string]any{
				"id": jobID,
				"write_scope": map[string]any{
					"mode":          "repo_write",
					"repo_write":    true,
					"allowed_paths": pathsAny,
				},
			},
		},
	}
}

func TestRefuseCrossRepoWriteScopeAcceptsInRepoPaths(t *testing.T) {
	for _, allowed := range [][]string{
		{"go/", "docs/rfcs/"},
		{"."},
		{"docs/campaigns/rfc-0128/"},
		{"a/../b"}, // nets back inside the repo; structural Validate handles it, not this guard
	} {
		wf := jobWithAllowedPaths("draft", allowed...)
		if err := RefuseCrossRepoWriteScope(wf); err != nil {
			t.Fatalf("allowed=%v: unexpected refusal: %v", allowed, err)
		}
	}
}

func TestRefuseCrossRepoWriteScopeRefusesEscapingPaths(t *testing.T) {
	cases := map[string]string{
		"absolute":         "/home/op/git/other-repo/src",
		"parent_escape":    "../striatum-warmtier/go",
		"bare_parent":      "..",
		"nested_escape":    "go/../../sibling-repo",
		"absolute_no_trim": "/etc",
	}
	for name, offending := range cases {
		t.Run(name, func(t *testing.T) {
			wf := jobWithAllowedPaths("draft", "go/", offending)
			err := RefuseCrossRepoWriteScope(wf)
			if err == nil {
				t.Fatalf("offending=%q: expected refusal, got nil", offending)
			}
			var crErr *CrossRepoWriteScopeError
			if !errors.As(err, &crErr) {
				t.Fatalf("offending=%q: expected *CrossRepoWriteScopeError, got %T", offending, err)
			}
			if crErr.OffendingPath != offending {
				t.Fatalf("offending path = %q, want %q", crErr.OffendingPath, offending)
			}
			if crErr.JobID != "draft" {
				t.Fatalf("job id = %q, want draft", crErr.JobID)
			}
			if !strings.Contains(crErr.Error(), offending) {
				t.Fatalf("message does not name offending path: %s", crErr.Error())
			}
			if !strings.Contains(crErr.Error(), "single-repo") && !strings.Contains(crErr.Error(), "one registered repository") {
				t.Fatalf("message does not state the single-repo invariant: %s", crErr.Error())
			}
		})
	}
}

func TestRefuseCrossRepoWriteScopeIsDeterministicAcrossJobs(t *testing.T) {
	// Two jobs each carry an escaping path; the surfaced job must be the
	// lexicographically-first id ("apply" < "draft") regardless of slice order.
	wf := map[string]any{
		"jobs": []any{
			map[string]any{
				"id":          "draft",
				"write_scope": map[string]any{"allowed_paths": []any{"../zzz-repo"}},
			},
			map[string]any{
				"id":          "apply",
				"write_scope": map[string]any{"allowed_paths": []any{"/abs/aaa-repo"}},
			},
		},
	}
	err := RefuseCrossRepoWriteScope(wf)
	var crErr *CrossRepoWriteScopeError
	if !errors.As(err, &crErr) || crErr.JobID != "apply" {
		t.Fatalf("expected deterministic refusal on job 'apply', got %v", err)
	}
}

func TestRefuseCrossRepoWriteScopeIsPanicSafeOnMalformedJobs(t *testing.T) {
	// Malformed (non-object) job entries and a missing write_scope must be
	// skipped, not panic — this guard runs on the UNVALIDATED workflow.
	wf := map[string]any{
		"jobs": []any{
			"not-an-object",
			map[string]any{"id": "no-scope"},
			map[string]any{"id": "ok", "write_scope": map[string]any{"allowed_paths": []any{"go/"}}},
		},
	}
	if err := RefuseCrossRepoWriteScope(wf); err != nil {
		t.Fatalf("unexpected refusal on malformed-but-in-repo workflow: %v", err)
	}
}

func TestPathReachesOutsideRepoRoot(t *testing.T) {
	outside := []string{"/", "/etc", "..", "../x", "go/../../x", "/abs/path"}
	for _, p := range outside {
		if !pathReachesOutsideRepoRoot(p) {
			t.Errorf("%q: expected outside-repo, got inside", p)
		}
	}
	inside := []string{"", "go/", "docs/rfcs", ".", "a/../b", "deep/nested/path"}
	for _, p := range inside {
		if pathReachesOutsideRepoRoot(p) {
			t.Errorf("%q: expected inside-repo, got outside", p)
		}
	}
}

func TestForeignRepoReachWarningsFlagsForeignSlugAndPaths(t *testing.T) {
	wf := map[string]any{
		"jobs": []any{
			map[string]any{
				"id":          "draft",
				"task_prompt": map[string]any{"inline": "Harden acme/widget-service and the repo at github.com/octo/secret-svc."},
				"objective":   "Also touch /var/lib/other-repo and ../sibling-repo/src.",
			},
		},
	}
	warnings := ForeignRepoReachWarnings(wf, "striatum")
	tokens := map[string]bool{}
	for _, w := range warnings {
		if w["rule"] != "foreign_repo_reach" || w["severity"] != "warning" {
			t.Fatalf("unexpected warning shape: %#v", w)
		}
		if w["job_id"] != "draft" {
			t.Fatalf("job_id = %#v", w["job_id"])
		}
		tokens[w["token"].(string)] = true
	}
	for _, want := range []string{"acme/widget-service", "octo/secret-svc", "/var/lib/other-repo", "../sibling-repo/src"} {
		if !tokens[want] {
			t.Errorf("missing foreign-reach warning for %q; got tokens %v", want, tokens)
		}
	}
}

func TestForeignRepoReachWarningsIgnoresInRepoReferences(t *testing.T) {
	wf := map[string]any{
		"jobs": []any{
			map[string]any{
				"id":          "draft",
				"task_prompt": map[string]any{"inline": "Edit go/pkg/workflowauthoring and docs/rfcs/0128-cross-repo-run-boundary.md; see roles/author.md."},
				"objective":   "Produce the starter artifact for this workflow.",
				"title":       "Draft starter artifact",
			},
		},
	}
	if warnings := ForeignRepoReachWarnings(wf, "striatum"); len(warnings) != 0 {
		t.Fatalf("expected no false-positive warnings on in-repo references, got %#v", warnings)
	}
}

func TestForeignRepoReachWarningsDoesNotFlagRegisteredRepo(t *testing.T) {
	wf := map[string]any{
		"jobs": []any{
			map[string]any{
				"id":          "draft",
				"task_prompt": map[string]any{"inline": "Work on github.com/halbritt/striatum only."},
			},
		},
	}
	if warnings := ForeignRepoReachWarnings(wf, "striatum"); len(warnings) != 0 {
		t.Fatalf("expected no warning for the registered repo, got %#v", warnings)
	}
}

// TestSecondaryReposManifestIsNotHonored is the RFC 0128 test obligation #4
// guard: the deferred first-class multi-repo manifest has NO code surface. A
// workflow that declares a secondary_repos manifest gains zero cross-repo write
// capability — the manifest is ignored, and a lane write-scope that reaches
// outside the registered repo is still refused exactly as without it.
func TestSecondaryReposManifestIsNotHonored(t *testing.T) {
	manifest := []any{
		map[string]any{
			"repo":        "octo/satellite",
			"write_scope": map[string]any{"allowed_paths": []any{"src/"}},
		},
	}

	// (a) A manifest does NOT create an escape hatch: a job whose own
	// write-scope reaches outside the repo is still refused, manifest or not.
	escaping := jobWithAllowedPaths("draft", "../octo-satellite/src")
	escaping[DeferredSecondaryReposManifestKey] = manifest
	if err := RefuseCrossRepoWriteScope(escaping); err == nil {
		t.Fatalf("secondary_repos manifest must not suppress the cross-repo refusal")
	}

	// (b) A manifest is otherwise ignored: an in-repo workflow validates the
	// same with or without the manifest key (it grants no extra capability).
	withManifest := jobWithAllowedPaths("draft", "go/")
	withManifest[DeferredSecondaryReposManifestKey] = manifest
	if err := RefuseCrossRepoWriteScope(withManifest); err != nil {
		t.Fatalf("manifest with an in-repo job must validate clean, got %v", err)
	}
}
