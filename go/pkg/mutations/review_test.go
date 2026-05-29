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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestOverrideVerdictRecoversCompletedReviewWithoutPriorVerdict(t *testing.T) {
	tx := newReviewOverrideFakeTx()
	tx.reviewState = "completed"
	runner := &reviewOverrideFakeRunner{tx: tx}

	result, err := HandleOverrideVerdict(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_override_recover",
		Method:        "review.override",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_override",
			"job_id":        "job_review",
			"verdict":       "accept",
			"rationale":     "published artifact was reviewed manually",
		},
	})
	if err != nil {
		t.Fatalf("HandleOverrideVerdict: %v", err)
	}
	if result["status"] != "recovered" {
		t.Fatalf("status = %#v, want recovered", result["status"])
	}
	if result["previous_verdict"] != nil {
		t.Fatalf("previous_verdict = %#v, want nil", result["previous_verdict"])
	}
	if tx.latestVerdict != "accept" {
		t.Fatalf("latest verdict = %q, want accept", tx.latestVerdict)
	}
	if tx.downstreamState != "queued" {
		t.Fatalf("downstream state = %q, want queued", tx.downstreamState)
	}
	if !tx.sawEvent("verdict.overridden") {
		t.Fatalf("verdict.overridden event not appended: %#v", tx.events)
	}
	if !tx.sawEvent("queue.message_enqueued") {
		t.Fatalf("downstream enqueue event not appended: %#v", tx.events)
	}
	if !tx.committed || tx.rolledBack {
		t.Fatalf("transaction commit/rollback = %v/%v", tx.committed, tx.rolledBack)
	}
}

func TestOverrideVerdictCompletesWaitingHumanAndEnqueuesDownstream(t *testing.T) {
	tx := newReviewOverrideFakeTx()
	tx.reviewState = "waiting_human"
	tx.previousVerdict = "needs_revision"
	runner := &reviewOverrideFakeRunner{tx: tx}

	result, err := HandleOverrideVerdict(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_override_waiting",
		Method:        "review.override",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_override",
			"job_id":        "job_review",
			"verdict":       "accept_with_findings",
			"rationale":     "operator accepted the revision path",
		},
	})
	if err != nil {
		t.Fatalf("HandleOverrideVerdict: %v", err)
	}
	if result["status"] != "overridden" {
		t.Fatalf("status = %#v, want overridden", result["status"])
	}
	if result["previous_verdict"] != "needs_revision" {
		t.Fatalf("previous_verdict = %#v", result["previous_verdict"])
	}
	if result["resolved_blockers"] != 1 {
		t.Fatalf("resolved_blockers = %#v, want 1", result["resolved_blockers"])
	}
	if tx.reviewState != "completed" {
		t.Fatalf("review state = %q, want completed", tx.reviewState)
	}
	if tx.downstreamState != "queued" {
		t.Fatalf("downstream state = %q, want queued", tx.downstreamState)
	}
	if !tx.sawEvent("job.completed") {
		t.Fatalf("job.completed event not appended for waiting_human override: %#v", tx.events)
	}
	if !tx.sawEvent("queue.message_enqueued") {
		t.Fatalf("downstream enqueue event not appended: %#v", tx.events)
	}
}

func TestOverrideVerdictAlreadyAcceptingDoesNotMutate(t *testing.T) {
	tx := newReviewOverrideFakeTx()
	tx.reviewState = "completed"
	tx.previousVerdict = "accept"
	runner := &reviewOverrideFakeRunner{tx: tx}

	result, err := HandleOverrideVerdict(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_override_already",
		Method:        "review.override",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_override",
			"job_id":        "job_review",
			"verdict":       "accept",
			"rationale":     "no change needed",
		},
	})
	if err != nil {
		t.Fatalf("HandleOverrideVerdict: %v", err)
	}
	if result["status"] != "already_accepting" {
		t.Fatalf("status = %#v, want already_accepting", result["status"])
	}
	if tx.insertedVerdicts != 0 {
		t.Fatalf("inserted verdicts = %d, want 0", tx.insertedVerdicts)
	}
	if tx.downstreamState != "blocked" {
		t.Fatalf("downstream state = %q, want blocked", tx.downstreamState)
	}
	if tx.sawEvent("queue.message_enqueued") {
		t.Fatalf("unexpected downstream enqueue event: %#v", tx.events)
	}
}

func TestSubmitReviewRejectsCollaborationLedgerVerdictMismatch(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, sessionID, jobID, leaseID := seedCollaborationLedgerSubmitFixture(t, ctx, runner, "needs_revision")

	_, err := HandleSubmitReview(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id":   sessionID,
		"job_id":       jobID,
		"lease_id":     leaseID,
		"path":         "artifacts/gate/LEDGER.md",
		"kind":         "collaboration_ledger",
		"logical_name": "collaboration_ledger",
		"verdict":      "accept",
	}))
	if err == nil || !strings.Contains(err.Error(), "must match collaboration_ledger front matter verdict") {
		t.Fatalf("error = %v", err)
	}
}

func TestSubmitReviewAcceptsMatchingCollaborationLedgerVerdict(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, sessionID, jobID, leaseID := seedCollaborationLedgerSubmitFixture(t, ctx, runner, "accept")

	result, err := HandleSubmitReview(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id":   sessionID,
		"job_id":       jobID,
		"lease_id":     leaseID,
		"path":         "artifacts/gate/LEDGER.md",
		"kind":         "collaboration_ledger",
		"logical_name": "collaboration_ledger",
		"verdict":      "accept",
	}))
	if err != nil {
		t.Fatalf("submit review: %v", err)
	}
	if result["job_state"] != "completed" {
		t.Fatalf("job_state = %#v; result=%#v", result["job_state"], result)
	}
}

// TestRecordVerdictRejectsCollaborationLedgerVerdictMismatch covers build
// finding 2: the primitive path (publish_artifact then review.verdict /
// recordVerdict) must enforce the same collaboration_ledger verdict
// consistency the submit path does. The adjudicator here published a
// `verdict: needs_revision` ledger and tries to record `accept` to clear the
// gate; the daemon must refuse.
func TestRecordVerdictRejectsCollaborationLedgerVerdictMismatch(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, sessionID, jobID, leaseID := seedCollaborationLedgerSubmitFixture(t, ctx, runner, "needs_revision")
	seedPublishedCollaborationLedgerArtifact(t, ctx, runner, repoID, sessionID, jobID)

	_, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"verdict":    "accept",
	}))
	if err == nil || !strings.Contains(err.Error(), "must match collaboration_ledger front matter verdict") {
		t.Fatalf("error = %v", err)
	}
}

// TestRecordVerdictAcceptsMatchingCollaborationLedgerVerdict is the green
// counterpart: a matching verdict on the primitive path is accepted and
// completes the gate job.
func TestRecordVerdictAcceptsMatchingCollaborationLedgerVerdict(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, sessionID, jobID, leaseID := seedCollaborationLedgerSubmitFixture(t, ctx, runner, "accept")
	seedPublishedCollaborationLedgerArtifact(t, ctx, runner, repoID, sessionID, jobID)

	result, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"verdict":    "accept",
	}))
	if err != nil {
		t.Fatalf("record verdict: %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %#v; result=%#v", result["status"], result)
	}
}

// seedPublishedCollaborationLedgerArtifact inserts the artifacts row that the
// adjudicator's publish_artifact call would have created, so the primitive
// verdict path's verifyRequiredArtifacts finds the expected ledger and
// enforceCollaborationLedgerVerdict can read it from disk.
func seedPublishedCollaborationLedgerArtifact(t *testing.T, ctx context.Context, runner db.Runner, repoID, sessionID, jobID string) {
	t.Helper()
	runID := "run_" + repoID
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id, logical_name,
		  artifact_kind, repo_path, content_sha256, size_bytes, publish_mode, created_at
		) VALUES ($1,$2,$3,$4,$5,'collaboration_ledger','collaboration_ledger',
		  'artifacts/gate/LEDGER.md','sha_'||$2,1,'create',$6)`,
		repoID, "art_ledger_"+repoID, runID, jobID, sessionID, now); err != nil {
		t.Fatalf("insert collaboration ledger artifact: %v", err)
	}
}

func seedCollaborationLedgerSubmitFixture(t *testing.T, ctx context.Context, runner db.Runner, verdict string) (repoID, sessionID, jobID, leaseID string) {
	t.Helper()
	repoID = "repo_collab_" + strings.ReplaceAll(verdict, "_", "")
	runID := "run_" + repoID
	sessionID = "sess_adjudicator_" + repoID
	jobID = "job_adjudicate_" + repoID
	leaseID = "lease_" + repoID
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "artifacts", "gate"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "artifacts", "gate", "LEDGER.md"), []byte(collaborationLedgerSubmitPayload(verdict)), 0o644); err != nil {
		t.Fatal(err)
	}
	intgSeedRepo(t, ctx, runner, repoID)
	workflow := map[string]any{
		"workflow_id": "collaboration_gate",
		"lanes": map[string]any{
			"adjudicator": map[string]any{"display_model": "Codex GPT-5.5"},
		},
		"jobs": []any{
			map[string]any{"id": "adjudicate", "type": "phase_synthesis", "role_id": "adjudicator"},
		},
	}
	intgSeedRun(t, ctx, runner, repoID, runID, workflow)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs SET repo_root = $1
		 WHERE repository_id = $2 AND run_id = $3`, repoRoot, repoID, runID); err != nil {
		t.Fatalf("update run repo root: %v", err)
	}
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "adjudicator", "adjudicator", []string{"review"}, "active")
	now := time.Now().UTC()
	laneSelectorArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "adjudicator"})
	if err != nil {
		t.Fatal(err)
	}
	writeScopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode":            "review_only_artifact",
		"repo_write":      false,
		"allowed_paths":   []string{"artifacts/gate/"},
		"forbidden_paths": []string{".striatum/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedArg, err := db.JSONBArg(runner, []map[string]any{{
		"logical_name": "collaboration_ledger",
		"kind":         "collaboration_ledger",
		"path":         "artifacts/gate/LEDGER.md",
		"required":     true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	// run.prepare maps a `phase_synthesis` WORKFLOW job to the stored
	// job_type='review' (see run.go: the jobs_job_type_check constraint does not
	// permit 'phase_synthesis'). Seed the live stored shape so the test exercises
	// the real row and the collaboration-ledger verdict guard fires on the same
	// path production does. The workflow_json snapshot still carries
	// type=phase_synthesis for the `adjudicate` job def, matching production.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, lane_selector_json, write_scope_json,
		  expected_artifacts_json, idempotency_key, created_at, started_at
		) VALUES ($1,$2,$3,'adjudicate',1,'running','adjudicator','Adjudicate','review',
		  $4::jsonb,$5::jsonb,$6::jsonb,'idem_'||$2,$7,$7)`,
		repoID, jobID, runID, laneSelectorArg, writeScopeArg, expectedArg, now); err != nil {
		t.Fatalf("insert adjudication job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7)`,
		repoID, leaseID, runID, jobID, sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	return repoID, sessionID, jobID, leaseID
}

func collaborationLedgerSubmitPayload(verdict string) string {
	entries := `  - kind: claim
    by: sess_holder
    refs: ["dialogue:1"]
    text: "The proposal is ready."
  - kind: challenge
    by: sess_falsifier
    refs: ["dialogue:2"]
    text: "The proposal lacks migration evidence."`
	rationale := "A challenge landed but has not been rebutted."
	if verdict == "accept" {
		entries += `
  - kind: rebuttal
    by: sess_holder
    refs: ["dialogue:3"]
    text: "The migration evidence is in the linked fixture."`
		rationale = "A material challenge landed and was rebutted on the record."
	}
	return `---
schema_version: "striatum.collaboration_ledger.v1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "substance gate"
participants: ["sess_holder", "sess_falsifier"]
entries:
` + entries + `
verdict: "` + verdict + `"
rationale: "` + rationale + `"
---

# Ledger
`
}

type reviewOverrideFakeRunner struct {
	tx *reviewOverrideFakeTx
}

func (r *reviewOverrideFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec outside tx")
}

func (r *reviewOverrideFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return reviewOverrideFakeRow{err: errors.New("unexpected runner query row outside tx")}
}

func (r *reviewOverrideFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected runner query scalar outside tx")
}

func (r *reviewOverrideFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return r.tx, nil
}

type reviewOverrideFakeTx struct {
	reviewState      string
	downstreamState  string
	previousVerdict  string
	latestVerdict    string
	insertedVerdicts int
	events           []string
	nextEvent        int64
	committed        bool
	rolledBack       bool
}

func newReviewOverrideFakeTx() *reviewOverrideFakeTx {
	return &reviewOverrideFakeTx{
		reviewState:     "completed",
		downstreamState: "blocked",
	}
}

func (tx *reviewOverrideFakeTx) Exec(_ context.Context, sql string, args ...any) error {
	switch {
	case strings.Contains(sql, "INSERT INTO striatumd.verdicts"):
		tx.insertedVerdicts++
		tx.latestVerdict = args[5].(string)
	case strings.Contains(sql, "UPDATE striatumd.jobs") && strings.Contains(sql, "SET state = 'completed'"):
		tx.reviewState = "completed"
	case strings.Contains(sql, "UPDATE striatumd.jobs") && strings.Contains(sql, "SET state = 'queued'"):
		tx.downstreamState = "queued"
	case strings.Contains(sql, "INSERT INTO striatumd.events"):
		tx.events = append(tx.events, args[3].(string))
	}
	return nil
}

func (tx *reviewOverrideFakeTx) QueryRow(_ context.Context, sql string, _ ...any) db.Row {
	switch {
	case strings.Contains(sql, "repo_event_chain_heads"):
		return reviewOverrideFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "nextval"):
		tx.nextEvent++
		return reviewOverrideFakeRow{values: []any{tx.nextEvent}}
	default:
		return reviewOverrideFakeRow{err: errors.New("unexpected query row: " + sql)}
	}
}

func (tx *reviewOverrideFakeTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (tx *reviewOverrideFakeTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.jobs"):
		return tx.queryJobs(sql, args...), nil
	case strings.Contains(sql, "FROM striatumd.sessions"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"session_id": "sess_override",
			"run_id":     "run_1",
			"state":      "active",
			"role_id":    "reviewer",
			"lane_id":    "codex",
		}}), nil
	case strings.Contains(sql, "SELECT 1 FROM striatumd.verdicts"):
		return runPrepareRowsFromMaps(nil), nil
	case strings.Contains(sql, "SELECT * FROM striatumd.verdicts"):
		if tx.previousVerdict == "" {
			return runPrepareRowsFromMaps(nil), nil
		}
		return runPrepareRowsFromMaps([]map[string]any{{
			"verdict_id":           "verdict_previous",
			"verdict":              tx.previousVerdict,
			"findings_artifact_id": nil,
		}}), nil
	case strings.Contains(sql, "SELECT verdict FROM striatumd.verdicts"):
		if tx.latestVerdict == "" {
			return runPrepareRowsFromMaps(nil), nil
		}
		return runPrepareRowsFromMaps([]map[string]any{{"verdict": tx.latestVerdict}}), nil
	case strings.Contains(sql, "FROM striatumd.runs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"run_id":               "run_1",
			"state":                "running",
			"workflow_snapshot_id": "snap_1",
		}}), nil
	case strings.Contains(sql, "FROM striatumd.workflow_snapshots"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"workflow_snapshot_id": "snap_1",
			"workflow_json": map[string]any{
				"jobs": []any{
					map[string]any{"id": "review", "type": "review"},
				},
			},
		}}), nil
	case strings.Contains(sql, "FROM striatumd.blockers"):
		return runPrepareRowsFromMaps([]map[string]any{{"blocker_id": "blk_1"}}), nil
	case strings.Contains(sql, "FROM striatumd.job_dependencies"):
		return tx.queryDependencies(sql, args...), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

func (tx *reviewOverrideFakeTx) queryJobs(sql string, args ...any) pgx.Rows {
	if strings.Contains(sql, "state = 'failed'") {
		return runPrepareRowsFromMaps(nil)
	}
	if strings.Contains(sql, "state NOT IN") {
		if tx.downstreamState == "completed" || tx.downstreamState == "skipped" || tx.downstreamState == "canceled" {
			return runPrepareRowsFromMaps(nil)
		}
		return runPrepareRowsFromMaps([]map[string]any{{"job_id": "job_apply"}})
	}
	if strings.Contains(sql, "state = 'completed'") {
		return runPrepareRowsFromMaps([]map[string]any{{"job_id": "job_review"}})
	}
	jobID := args[1].(string)
	if jobID == "job_review" {
		return runPrepareRowsFromMaps([]map[string]any{{
			"job_id":             "job_review",
			"run_id":             "run_1",
			"workflow_job_id":    "review",
			"job_type":           "review",
			"state":              tx.reviewState,
			"current_message_id": "msg_review",
		}})
	}
	return runPrepareRowsFromMaps([]map[string]any{{
		"job_id":             "job_apply",
		"run_id":             "run_1",
		"workflow_job_id":    "apply",
		"job_type":           "synthesis",
		"state":              tx.downstreamState,
		"role_id":            "author",
		"lane_selector_json": map[string]any{},
		"max_attempts":       1,
	}})
}

func (tx *reviewOverrideFakeTx) queryDependencies(sql string, args ...any) pgx.Rows {
	if strings.Contains(sql, "depends_on_job_id") {
		if args[1] == "job_review" {
			return runPrepareRowsFromMaps([]map[string]any{{"job_id": "job_apply"}})
		}
		return runPrepareRowsFromMaps(nil)
	}
	if args[1] == "job_apply" {
		return runPrepareRowsFromMaps([]map[string]any{{
			"job_id":            "job_apply",
			"depends_on_job_id": "job_review",
			"gate_json": map[string]any{
				"requires_verdict": []any{"accept", "accept_with_findings"},
			},
		}})
	}
	return runPrepareRowsFromMaps(nil)
}

func (tx *reviewOverrideFakeTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *reviewOverrideFakeTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *reviewOverrideFakeTx) sawEvent(eventType string) bool {
	for _, event := range tx.events {
		if event == eventType {
			return true
		}
	}
	return false
}

type reviewOverrideFakeRow struct {
	values []any
	err    error
}

func (r reviewOverrideFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *int64:
			*target = value.(int64)
		case **string:
			if value == nil {
				*target = nil
			} else {
				text := value.(string)
				*target = &text
			}
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

func TestSubmitReviewDuplicateArtifactHandling(t *testing.T) {
	tmpDir := t.TempDir()
	artPath := filepath.Join(tmpDir, "finding.md")
	mustWrite(t, artPath, `---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept
---
author: operator

Everything looks perfect.`)

	runner := &submitReviewFakeRunner{
		repoRoot: tmpDir,
	}

	result, err := HandleSubmitReview(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_submit_duplicate",
		Method:        "review.submit",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_review",
			"job_id":        "job_review",
			"lease_id":      "lease_1",
			"path":          "finding.md",
			"verdict":       "accept",
			"logical_name":  "review_art",
			"kind":          "finding",
			"rationale":     "looks great",
		},
	})
	if err != nil {
		t.Fatalf("HandleSubmitReview failed: %v", err)
	}

	if result == nil {
		t.Fatalf("expected result, got nil")
	}

	artifact := result["artifact"].(map[string]any)
	if artifact["artifact_id"] != "art_existing" {
		t.Fatalf("expected artifact_id to be 'art_existing', got %v", artifact["artifact_id"])
	}
	if artifact["status"] != "already_published" {
		t.Fatalf("expected status to be 'already_published', got %v", artifact["status"])
	}

	if runner.tx1 == nil || !runner.tx1.rolledBack {
		t.Fatalf("expected first transaction to be rolled back")
	}
	if runner.tx2 == nil || !runner.tx2.committed {
		t.Fatalf("expected second transaction to be committed")
	}
}

type submitReviewFakeRunner struct {
	repoRoot string
	tx1      *submitReviewFakeTx
	tx2      *submitReviewFakeTx
	txCount  int
}

func (r *submitReviewFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec outside tx")
}

func (r *submitReviewFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return reviewOverrideFakeRow{err: errors.New("unexpected runner query row outside tx")}
}

func (r *submitReviewFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected runner query scalar outside tx")
}

func (r *submitReviewFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	r.txCount++
	if r.txCount == 1 {
		r.tx1 = &submitReviewFakeTx{repoRoot: r.repoRoot, failPublish: true}
		return r.tx1, nil
	}
	r.tx2 = &submitReviewFakeTx{repoRoot: r.repoRoot, failPublish: false}
	return r.tx2, nil
}

type submitReviewFakeTx struct {
	repoRoot    string
	failPublish bool
	committed   bool
	rolledBack  bool
}

func (tx *submitReviewFakeTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *submitReviewFakeTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *submitReviewFakeTx) Exec(_ context.Context, sql string, args ...any) error {
	if strings.Contains(sql, "INSERT INTO striatumd.artifacts") {
		if tx.failPublish {
			return &pgconn.PgError{Code: "23505", Message: "duplicate key violates unique constraint"}
		}
	}
	return nil
}

func (tx *submitReviewFakeTx) QueryRow(_ context.Context, sql string, _ ...any) db.Row {
	switch {
	case strings.Contains(sql, "repo_event_chain_heads"):
		return reviewOverrideFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "nextval"):
		return reviewOverrideFakeRow{values: []any{int64(1)}}
	default:
		return reviewOverrideFakeRow{err: pgx.ErrNoRows}
	}
}

func (tx *submitReviewFakeTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (tx *submitReviewFakeTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.jobs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"job_id":             "job_review",
			"run_id":             "run_1",
			"job_type":           "review",
			"state":              "running",
			"current_message_id": "msg_review",
			"role_id":            "author",
			"workflow_job_id":    "review",
			"lane_selector_json": map[string]any{},
			"write_scope_json": map[string]any{
				"allowed_paths": []any{"."},
			},
		}}), nil
	case strings.Contains(sql, "FROM striatumd.leases"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"lease_id":         "lease_1",
			"owner_session_id": "sess_review",
			"resource_id":      "job_review",
			"state":            "active",
		}}), nil
	case strings.Contains(sql, "FROM striatumd.sessions"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"session_id": "sess_review",
			"run_id":     "run_1",
			"state":      "active",
			"ordinal":    1,
		}}), nil
	case strings.Contains(sql, "FROM striatumd.runs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"run_id":               "run_1",
			"repo_root":            tx.repoRoot,
			"workflow_snapshot_id": "snap_1",
			"state":                "running",
		}}), nil
	case strings.Contains(sql, "FROM striatumd.workflow_snapshots"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"workflow_snapshot_id": "snap_1",
			"workflow_json": map[string]any{
				"jobs": []any{
					map[string]any{"id": "review", "type": "review"},
				},
			},
		}}), nil
	case strings.Contains(sql, "SELECT artifact_id FROM striatumd.artifacts"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"artifact_id": "art_existing",
		}}), nil
	case strings.Contains(sql, "SELECT * FROM striatumd.artifacts"):
		if strings.Contains(sql, "logical_name") {
			return runPrepareRowsFromMaps(nil), nil
		}
		return runPrepareRowsFromMaps([]map[string]any{{
			"artifact_id":    "art_existing",
			"run_id":         "run_1",
			"job_id":         "job_review",
			"logical_name":   "review_art",
			"artifact_kind":  "finding",
			"repo_path":      "finding.md",
			"content_sha256": "some-sha",
		}}), nil
	case strings.Contains(sql, "FROM striatumd.job_dependencies"):
		return runPrepareRowsFromMaps(nil), nil
	default:
		return runPrepareRowsFromMaps(nil), nil
	}
}
