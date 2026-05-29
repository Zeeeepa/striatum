package mutations

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/halbritt/striatum/go/pkg/artifactcontracts"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// cyclePlaceholder is the literal token a generated workflow places in a
// cycle-scoped expected artifact's logical_name and path. Each revision cycle
// re-runs the same job row with a bumped `attempt`, so the daemon resolves the
// token to a distinct `cycle_<attempt>` segment before the artifact is
// surfaced, published, or verified.
//
// This is the runtime half of the RFC 0093 design synthesis §4.6 cycle_<N>
// naming convention: a `needs_revision` re-loop publishes the ledger under a
// new logical name + path so it does not collide with the prior cycle's
// content-hash guard (which would otherwise deadlock the revision cycle), while
// preserving every cycle's ledger as durable provenance instead of deleting the
// prior artifact rows.
const cyclePlaceholder = "${cycle}"

// cycleSegmentForAttempt maps a job attempt (1-based) to its cycle segment.
// Attempt 1 is the first dialogue round (cycle_1); attempt N is the N-th.
func cycleSegmentForAttempt(attempt int) string {
	if attempt < 1 {
		attempt = 1
	}
	return fmt.Sprintf("cycle_%d", attempt)
}

// resolveCyclePlaceholders substitutes the cycle placeholder in a string with
// the segment derived from the job attempt. Strings without the placeholder are
// returned unchanged, so this is safe to apply to every expected artifact.
func resolveCyclePlaceholders(value string, attempt int) string {
	if !strings.Contains(value, cyclePlaceholder) {
		return value
	}
	return strings.ReplaceAll(value, cyclePlaceholder, cycleSegmentForAttempt(attempt))
}

// resolveExpectedArtifactCycles returns a copy of the expected-artifact list
// with every cycle placeholder in logical_name and path resolved against the
// supplied attempt. The daemon calls this wherever the stored
// expected_artifacts_json is consumed (work-packet build, required-artifact
// verification, and the submit-review pre-check) so the published artifact's
// attempt-scoped logical name and path agree across every code path.
func resolveExpectedArtifactCycles(expected []any, attempt int) []any {
	result := make([]any, 0, len(expected))
	for _, item := range expected {
		artifact := asMap(item)
		if len(artifact) == 0 {
			result = append(result, item)
			continue
		}
		resolved := map[string]any{}
		for key, value := range artifact {
			switch key {
			case "logical_name", "path":
				if text, ok := value.(string); ok {
					resolved[key] = resolveCyclePlaceholders(text, attempt)
					continue
				}
			}
			resolved[key] = value
		}
		result = append(result, resolved)
	}
	return result
}

// enforceCollaborationLedgerVerdict closes the primitive-path bypass (build
// finding 2). The submit-review path already refuses a verdict that disagrees
// with the collaboration_ledger front matter, but the primitive path
// (publish_artifact then review.verdict / recordVerdict) only verified the
// expected artifact exists — it never parsed the ledger. An adjudicator could
// therefore publish a `verdict: needs_revision` ledger and then record
// `accept`, clearing the substance gate it is supposed to enforce.
//
// When any required expected artifact of the verdict-capable job is a
// collaboration_ledger, the recorded verdict must equal the ledger's
// front-matter verdict. The check reads the ledger from the run's repo root and
// resolves cycle placeholders against the job attempt, matching what the
// adjudicator was told to publish.
func enforceCollaborationLedgerVerdict(ctx context.Context, runner any, repositoryID string, job map[string]any, recordedVerdict string) error {
	attempt := intValue(job["attempt"])
	expected := resolveExpectedArtifactCycles(asList(job["expected_artifacts_json"]), attempt)
	var ledgerPath string
	for _, item := range expected {
		artifact := asMap(item)
		if artifact["kind"] != "collaboration_ledger" {
			continue
		}
		if artifact["required"] != true {
			continue
		}
		if text, ok := artifact["path"].(string); ok && text != "" {
			ledgerPath = text
			break
		}
	}
	if ledgerPath == "" {
		return nil
	}
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), false)
	if err != nil {
		return err
	}
	repoRoot := fmt.Sprint(run["repo_root"])
	resolved, err := repoRelativePath(repoRoot, ledgerPath, false)
	if err != nil {
		return rpc.NewError("artifact_error", err.Error(), nil)
	}
	payload, err := os.ReadFile(resolved)
	if err != nil {
		return rpc.NewError("artifact_error", "collaboration_ledger artifact file does not exist", nil)
	}
	frontMatter, err := artifactcontracts.ParseAndValidateFrontMatter("collaboration_ledger", resolved, payload)
	if err != nil {
		return rpc.NewError("artifact_error", err.Error(), nil)
	}
	ledgerVerdict := fmt.Sprint(frontMatter["verdict"])
	if ledgerVerdict != recordedVerdict {
		return rpc.NewError("artifact_error", fmt.Sprintf("recorded verdict %q must match collaboration_ledger front matter verdict %q", recordedVerdict, ledgerVerdict), nil)
	}
	return nil
}
