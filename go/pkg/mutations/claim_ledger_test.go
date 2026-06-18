package mutations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// setupClaimLedgerRun inserts a repository, workflow snapshot, run, a build job,
// a session, and an active lease scoped to write under docs/run/, so a lane can
// publish claim_ledger artifacts. It returns the repo root. It does NOT publish
// any ledger; each test writes and publishes its own ledgers in seal order.
func setupClaimLedgerRun(t *testing.T, ctx context.Context, runner db.Runner) string {
	t.Helper()
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs", "run"), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()

	workflow := map[string]any{
		"workflow_id": "wf_claim",
		"lanes":       map[string]any{"author": map[string]any{"display_model": "Claude"}},
		"jobs": []any{
			map[string]any{"id": "build", "type": "build", "role_id": "builder", "lane_id": "author"},
		},
	}
	workflowArg, err := db.JSONBArg(runner, workflow)
	if err != nil {
		t.Fatal(err)
	}
	writeScopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode":          "repo_write",
		"repo_write":    true,
		"allowed_paths": []string{"docs/run/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	laneSelectorArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "author"})
	if err != nil {
		t.Fatal(err)
	}

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_claim','ident_claim',$1,$2,'repo',$3,14,'active')`,
		repoRoot, filepath.Join(repoRoot, ".striatum"), now,
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ('repo_claim','snap_claim','wf_claim','sha',$1::jsonb,$2)`, workflowArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ('repo_claim','run_claim','snap_claim',$1,'running',$2)`, repoRoot, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  state, registered_at
		) VALUES ('repo_claim','sess_build','run_claim','builder','author','builder-1',1,'active',$1)`, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  lane_selector_json, state, write_scope_json, idempotency_key, created_at
		) VALUES ('repo_claim','job_build','run_claim','build','Build','build','builder',
		  $1::jsonb,'running',$2::jsonb,'idem_build',$3)`, laneSelectorArg, writeScopeArg, now); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ('repo_claim','lease_build','run_claim','job','job_build','sess_build','active',$1,$2)`,
		now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	return repoRoot
}

// publishClaimLedger writes the ledger body to repoRoot/relPath, registers a
// prior-row sentinel via the publish itself, and publishes it as a claim_ledger
// artifact through the real HandlePublishArtifact path so the lattice writer
// gate runs. The logical name varies per call so attempt-scoped collision rules
// do not interfere.
func publishClaimLedger(t *testing.T, ctx context.Context, runner db.Runner, repoRoot, logicalName, relPath, body string) (map[string]any, error) {
	t.Helper()
	full := filepath.Join(repoRoot, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return HandlePublishArtifact(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_claim_publish_" + logicalName,
		Method:        "artifact.publish",
		Params: map[string]any{
			"repository_id": "repo_claim",
			"session_id":    "sess_build",
			"job_id":        "job_build",
			"lease_id":      "lease_build",
			"kind":          "claim_ledger",
			"logical_name":  logicalName,
			"path":          relPath,
		},
	})
}

func claimLedgerDoc(seal string, claimsBlock string) string {
	return `---
schema_version: "striatum.claim_ledger.v1"
artifact_kind: "claim_ledger"
run_id: "run_claim"
ledger_seal: ` + seal + `
claims:
` + claimsBlock + `---

# Claims
`
}

// TestClaimLedgerWriterAcceptsFirstLedger verifies the first ledger of a run
// publishes (no prior seal to compare against).
func TestClaimLedgerWriterAcceptsFirstLedger(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupClaimLedgerRun(t, ctx, runner)

	body := claimLedgerDoc("0", `  - id: C1
    status: ASSERTED
    text: "The publisher refuses invalid front matter with exit code 6."
`)
	result, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_0", "docs/run/CLAIMS_0.md", body)
	if err != nil {
		t.Fatalf("first claim_ledger should publish: %v", err)
	}
	if result["status"] != "published" {
		t.Fatalf("publish result = %#v, want published", result)
	}
}

// TestClaimLedgerWriterEnforcesMonotonicSeal verifies a republish at a seal that
// does not exceed the prior ledger's seal is refused (append-only).
func TestClaimLedgerWriterEnforcesMonotonicSeal(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupClaimLedgerRun(t, ctx, runner)

	first := claimLedgerDoc("3", `  - id: C1
    status: ASSERTED
    text: "first."
`)
	if _, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_3", "docs/run/CLAIMS_3.md", first); err != nil {
		t.Fatalf("first ledger should publish: %v", err)
	}

	// A second ledger at the SAME seal must be refused.
	same := claimLedgerDoc("3", `  - id: C1
    status: ASSERTED
    text: "stale reseal."
`)
	_, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_3b", "docs/run/CLAIMS_3B.md", same)
	if err == nil {
		t.Fatal("a republish at the same seal must be refused (append-only)")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("error must name the append-only/monotonic rule: %v", err)
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "artifact_error" {
		t.Fatalf("error code = %#v, want artifact_error (exit code 6)", err)
	}
}

// TestClaimLedgerWriterRefusesSelfPromotion verifies a claim demoted-but-never-
// promoted: raising a claim from ASSERTED to VERIFIED across a reseal is refused
// even if the new ledger carries a (bound) receipt — promotion is the sealed
// mint, off the gate path.
func TestClaimLedgerWriterRefusesSelfPromotion(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupClaimLedgerRun(t, ctx, runner)

	first := claimLedgerDoc("0", `  - id: C1
    status: ASSERTED
    text: "structured read says the gate exists."
`)
	if _, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_0", "docs/run/CLAIMS_0.md", first); err != nil {
		t.Fatalf("first ledger should publish: %v", err)
	}

	// Seal advances (monotonic ok), but C1 is raised ASSERTED → VERIFIED with a
	// bound receipt. The provenance lint would pass it in isolation; the writer's
	// no-self-promotion rule must still refuse the raise across seals.
	promoted := claimLedgerDoc("1", `  - id: C1
    status: VERIFIED
    text: "now claimed verified."
    bound_input_digest: "treesha_abc"
    receipt_ref: "docs/run/receipts/C1.md"
    evidence_digest: "treesha_abc"
`)
	_, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_1", "docs/run/CLAIMS_1.md", promoted)
	if err == nil {
		t.Fatal("raising a claim's status across seals must be refused (never self-promotable)")
	}
	for _, want := range []string{"never self-promotable", "C1", "ASSERTED", "VERIFIED"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must report the self-promotion of C1; want %q, got: %v", want, err)
		}
	}
}

// TestClaimLedgerWriterAllowsDemotionAndAdvance verifies the lattice's allowed
// transitions: a higher seal that LOWERS a claim (VERIFIED → ASSERTED) and adds
// a fresh claim publishes cleanly.
func TestClaimLedgerWriterAllowsDemotionAndAdvance(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupClaimLedgerRun(t, ctx, runner)

	first := claimLedgerDoc("0", `  - id: C1
    status: VERIFIED
    text: "go test passes."
    bound_input_digest: "treesha_abc"
    receipt_ref: "docs/run/receipts/C1.md"
    evidence_digest: "treesha_abc"
`)
	if _, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_0", "docs/run/CLAIMS_0.md", first); err != nil {
		t.Fatalf("first ledger should publish: %v", err)
	}

	// Inputs changed: C1 is honestly demoted to ASSERTED, and a new claim C2 is
	// added at DESIGNED. Seal advances. All allowed.
	second := claimLedgerDoc("1", `  - id: C1
    status: ASSERTED
    text: "inputs changed; re-asserting pending a fresh witness."
  - id: C2
    status: DESIGNED
    text: "We intend to add a second witness."
`)
	result, err := publishClaimLedger(t, ctx, runner, repoRoot, "claims_1", "docs/run/CLAIMS_1.md", second)
	if err != nil {
		t.Fatalf("a demotion-and-advance ledger should publish: %v", err)
	}
	if result["status"] != "published" {
		t.Fatalf("publish result = %#v, want published", result)
	}
}
