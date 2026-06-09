package mutations

import (
	"context"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// seedProcessPointer records a supervised process identity (pid + pid_start_time)
// for a session, the way supervise start would.
func seedProcessPointer(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID string, pid int, startToken string) {
	t.Helper()
	now := time.Now().UTC()
	supID := "sup_" + sessionID
	dsupID := "dsup_" + sessionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'attached',$8,'{}'::jsonb)`,
		repoID, supID, dsupID, runID, sessionID, pid, startToken, now); err != nil {
		t.Fatalf("seed pointer for %s: %v", sessionID, err)
	}
}

// seedClaimedUpstreamPacket records that a session did durable work (claimed a
// packet) on a job, satisfying the work_packets FK chain.
func seedClaimedUpstreamPacket(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, sessionID string) {
	t.Helper()
	now := time.Now().UTC()
	msgID := "msg_up_" + sessionID + "_" + jobID
	leaseID := "lease_up_" + sessionID + "_" + jobID
	pktID := "wp_up_" + sessionID + "_" + jobID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,$5,$5)`,
		repoID, msgID, runID, jobID, now); err != nil {
		t.Fatalf("seed upstream message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'released',$6,$6)`,
		repoID, leaseID, runID, jobID, sessionID, now); err != nil {
		t.Fatalf("seed upstream lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.work_packets (
		  repository_id, packet_id, run_id, job_id, message_id, lease_id,
		  session_id, packet_json, packet_sha256, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'{}'::jsonb,'sha',$8)`,
		repoID, pktID, runID, jobID, msgID, leaseID, sessionID, now); err != nil {
		t.Fatalf("seed upstream work packet: %v", err)
	}
}

// freshReviewFixture seeds a run with a completed upstream synthesis job and a
// queued fresh-context review job that depends on it.
type freshReviewFixture struct {
	repoID        string
	runID         string
	upstreamJobID string
	reviewJobID   string
	upstreamSess  string
	reviewRole    string
}

func seedFreshReviewFixture(t *testing.T, ctx context.Context, runner db.Runner, repoID string) freshReviewFixture {
	t.Helper()
	runID := "run_" + repoID
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"synthesizer": map[string]any{}, "reviewer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{"display_model": "Claude"}},
	})
	upstreamJobID := "job_synth"
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, created_at, completed_at
		) VALUES ($1,$2,$3,'synthesis',1,'completed','synthesizer','Synthesis','synthesis','idem_synth',NOW(),NOW())`,
		repoID, upstreamJobID, runID); err != nil {
		t.Fatalf("seed synth job: %v", err)
	}
	reviewJobID := "job_review"
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, fresh_session_required, idempotency_key, created_at
		) VALUES ($1,$2,$3,'review',1,'queued','reviewer','Review','review',true,'idem_review',NOW())`,
		repoID, reviewJobID, runID); err != nil {
		t.Fatalf("seed review job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
		VALUES ($1,$2,$3)`, repoID, reviewJobID, upstreamJobID); err != nil {
		t.Fatalf("seed dependency: %v", err)
	}
	upstreamSess := "sess_synth"
	intgSeedSession(t, ctx, runner, repoID, runID, upstreamSess, "synthesizer", "claude", nil, "closed")
	return freshReviewFixture{repoID, runID, upstreamJobID, reviewJobID, upstreamSess, "reviewer"}
}

func (fx freshReviewFixture) refusal(t *testing.T, ctx context.Context, runner db.Runner, claimantSession string) map[string]any {
	t.Helper()
	var refusal map[string]any
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		job, err := rowByID(ctx, tx, fx.repoID, "jobs", "job_id", fx.reviewJobID, false)
		if err != nil {
			return nil, err
		}
		r, e := freshReviewProcessLineageRefusal(ctx, tx, fx.repoID, fx.runID, claimantSession, job)
		refusal = r
		return nil, e
	}); err != nil {
		t.Fatalf("refusal eval: %v", err)
	}
	return refusal
}

// TestFreshReviewLineageRefusesSameProcess is the #222 headline: the same
// supervised process that did upstream work cannot claim the fresh review job.
func TestFreshReviewLineageRefusesSameProcess(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	fx := seedFreshReviewFixture(t, ctx, runner, "repo_fr_same_proc")

	// The upstream synthesizer process did durable work on the synthesis.
	seedProcessPointer(t, ctx, runner, fx.repoID, fx.runID, fx.upstreamSess, 4242, "proc-A")
	seedClaimedUpstreamPacket(t, ctx, runner, fx.repoID, fx.runID, fx.upstreamJobID, fx.upstreamSess)

	// A fresh session id, but on the SAME process identity (pid + start token).
	reviewer := "sess_reviewer_same"
	intgSeedSession(t, ctx, runner, fx.repoID, fx.runID, reviewer, fx.reviewRole, "claude", nil, "active")
	seedProcessPointer(t, ctx, runner, fx.repoID, fx.runID, reviewer, 4242, "proc-A")

	refusal := fx.refusal(t, ctx, runner, reviewer)
	if refusal == nil {
		t.Fatalf("expected refusal for same-process reviewer")
	}
	if refusal["status"] != "no_work" || refusal["ineligible_reason"] != "fresh_review_process_lineage" {
		t.Fatalf("refusal = %#v", refusal)
	}
	if refusal["contaminating_session_id"] != fx.upstreamSess || refusal["contaminating_job_id"] != fx.upstreamJobID {
		t.Fatalf("refusal lineage detail = %#v", refusal)
	}
}

// TestFreshReviewLineageAllowsFreshProcess: a genuinely fresh process (different
// pid_start_time) may claim the review.
func TestFreshReviewLineageAllowsFreshProcess(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	fx := seedFreshReviewFixture(t, ctx, runner, "repo_fr_fresh_proc")

	seedProcessPointer(t, ctx, runner, fx.repoID, fx.runID, fx.upstreamSess, 4242, "proc-A")
	seedClaimedUpstreamPacket(t, ctx, runner, fx.repoID, fx.runID, fx.upstreamJobID, fx.upstreamSess)

	reviewer := "sess_reviewer_fresh"
	intgSeedSession(t, ctx, runner, fx.repoID, fx.runID, reviewer, fx.reviewRole, "claude", nil, "active")
	seedProcessPointer(t, ctx, runner, fx.repoID, fx.runID, reviewer, 4242, "proc-B") // different start token

	if refusal := fx.refusal(t, ctx, runner, reviewer); refusal != nil {
		t.Fatalf("fresh process should be allowed, got refusal %#v", refusal)
	}
}

// TestFreshReviewLineageRefusesUnverifiedIdentity: a supervised fresh-review lane
// with no verifiable process identity cannot claim.
func TestFreshReviewLineageRefusesUnverifiedIdentity(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	fx := seedFreshReviewFixture(t, ctx, runner, "repo_fr_unverified")

	reviewer := "sess_reviewer_unverified"
	intgSeedSession(t, ctx, runner, fx.repoID, fx.runID, reviewer, fx.reviewRole, "claude", nil, "active")
	seedProcessPointer(t, ctx, runner, fx.repoID, fx.runID, reviewer, 4242, "") // empty start token

	refusal := fx.refusal(t, ctx, runner, reviewer)
	if refusal == nil || refusal["ineligible_reason"] != "fresh_review_process_lineage" {
		t.Fatalf("expected unverified-identity refusal, got %#v", refusal)
	}
	if refusal["identity_status"] != "missing_or_unverified" {
		t.Fatalf("identity_status = %#v", refusal["identity_status"])
	}
}

// TestFreshReviewLineageSkipsNonReviewJob: the gate only applies to fresh-context
// verdict-capable jobs; a non-review job is untouched.
func TestFreshReviewLineageSkipsNonReviewJob(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	fx := seedFreshReviewFixture(t, ctx, runner, "repo_fr_nonreview")
	// Flip the review job to a build job type — the lineage gate must not apply.
	if err := runner.Exec(ctx, `UPDATE striatumd.jobs SET job_type='build' WHERE repository_id=$1 AND job_id=$2`,
		fx.repoID, fx.reviewJobID); err != nil {
		t.Fatalf("flip job_type: %v", err)
	}
	seedProcessPointer(t, ctx, runner, fx.repoID, fx.runID, fx.upstreamSess, 4242, "proc-A")
	seedClaimedUpstreamPacket(t, ctx, runner, fx.repoID, fx.runID, fx.upstreamJobID, fx.upstreamSess)
	reviewer := "sess_builder"
	intgSeedSession(t, ctx, runner, fx.repoID, fx.runID, reviewer, fx.reviewRole, "claude", nil, "active")
	seedProcessPointer(t, ctx, runner, fx.repoID, fx.runID, reviewer, 4242, "proc-A")

	if refusal := fx.refusal(t, ctx, runner, reviewer); refusal != nil {
		t.Fatalf("non-review job should not be lineage-gated, got %#v", refusal)
	}
}

// TestFreshReviewLineageAuditDeduped: repeated refusals (a polling lane) record
// the ineligible-claim audit event only once.
func TestFreshReviewLineageAuditDeduped(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	fx := seedFreshReviewFixture(t, ctx, runner, "repo_fr_dedup")
	seedProcessPointer(t, ctx, runner, fx.repoID, fx.runID, fx.upstreamSess, 4242, "proc-A")
	seedClaimedUpstreamPacket(t, ctx, runner, fx.repoID, fx.runID, fx.upstreamJobID, fx.upstreamSess)
	reviewer := "sess_reviewer_dedup"
	intgSeedSession(t, ctx, runner, fx.repoID, fx.runID, reviewer, fx.reviewRole, "claude", nil, "active")
	seedProcessPointer(t, ctx, runner, fx.repoID, fx.runID, reviewer, 4242, "proc-A")

	for i := 0; i < 3; i++ {
		if refusal := fx.refusal(t, ctx, runner, reviewer); refusal == nil {
			t.Fatalf("poll %d expected refusal", i)
		}
	}
	count, err := runner.QueryScalar(ctx, `
		SELECT COUNT(*) FROM striatumd.events
		 WHERE repository_id=$1 AND event_type=$2 AND actor_session_id=$3 AND job_id=$4`,
		fx.repoID, freshReviewLineageEvent, reviewer, fx.reviewJobID)
	if err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != "1" {
		t.Fatalf("audit event count = %s, want 1 (deduped over polling)", count)
	}
}
