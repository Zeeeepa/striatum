package mutations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/reads"
	"github.com/jackc/pgx/v5"
)

func TestRecallDigestQueryIsDeterministicMetadataOnly(t *testing.T) {
	inputs := worktreeCreateInputs{
		Job: map[string]any{
			"title":           "Implement S11",
			"workflow_job_id": "s11_implement",
			"job_type":        "build",
			"role_id":         "implementer",
			"expected_artifacts_json": []any{
				map[string]any{"logical_name": "s11_build_handoff", "kind": "handoff", "path": "workflows/remaining-campaign/artifacts/s11/build/HANDOFF.md"},
			},
		},
		Workflow: map[string]any{
			"context_docs": []any{
				map[string]any{"path": "docs/rfcs/0119-warm-tier-memory-boundary.md"},
			},
		},
	}

	got := buildRecallDigestQuery(inputs)

	for _, needle := range []string{"Implement S11", "s11_implement", "handoff", "docs/rfcs/0119-warm-tier-memory-boundary.md"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("query %q missing %q", got, needle)
		}
	}
	if strings.Contains(got, "body") || strings.Contains(got, "payload") {
		t.Fatalf("query should not include prose payload fields: %q", got)
	}
}

func TestNormalizeRecallDigestOptions(t *testing.T) {
	got := normalizeRecallDigestOptions(RecallDigestOptions{Enabled: true, Limit: 999})
	if !got.Enabled || got.Limit != reads.RecallLimitCap || got.Timeout != defaultRecallDigestTimeout {
		t.Fatalf("options = %#v", got)
	}
	got = normalizeRecallDigestOptions(RecallDigestOptions{Limit: 1, Timeout: time.Second})
	if got.Limit != 1 || got.Timeout != time.Second {
		t.Fatalf("options = %#v", got)
	}
}

type recallDigestErrorRunner struct {
	inertRunner
}

func (recallDigestErrorRunner) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("recall unavailable")
}

func TestMaybeRenderRecallShelfWritesEmptyShelfWhenRecallFails(t *testing.T) {
	root := t.TempDir()

	status := maybeRenderRecallShelf(context.Background(), recallDigestErrorRunner{}, RecallDigestOptions{
		Enabled: true,
		Limit:   1,
		Timeout: time.Second,
	}, "repo_1", worktreeCreateInputs{Job: map[string]any{"title": "S11 hot tier"}}, root)

	if status.Status != "empty" || status.Error != "recall_failed" || status.HitCount != 0 {
		t.Fatalf("status = %#v", status)
	}
	body, err := os.ReadFile(filepath.Join(root, ".striatum", "memory", "relevant.md"))
	if err != nil {
		t.Fatalf("read shelf: %v", err)
	}
	text := string(body)
	for _, needle := range []string{"degraded_reason: `recall_failed`", "redaction_tier: `metadata_only`", "No matching artifact metadata"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("shelf missing %q:\n%s", needle, text)
		}
	}
}

func TestRenderRecallShelfEscapesMetadataAsInertMarkdown(t *testing.T) {
	body := renderRecallShelf("repo_1", []reads.RecallHit{
		{ArtifactKind: "finding|decision", LogicalName: "name\nnext", RepoPath: "docs/`x`.md", RunID: "run_1", CreatedAt: "2026-06-14", Score: 0.5},
	}, reads.RecallMeta{Query: "danger `quoted`", RankingMethod: "rank", SourceSurface: "surface"})

	if strings.Contains(body, "finding|decision") {
		t.Fatalf("table cell was not escaped:\n%s", body)
	}
	if !strings.Contains(body, "finding\\|decision") || !strings.Contains(body, "name next") {
		t.Fatalf("shelf missing escaped metadata:\n%s", body)
	}
}
