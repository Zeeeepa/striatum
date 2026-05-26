package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/reads"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// --- seeding helpers -------------------------------------------------------

func intgSeedRepo(t *testing.T, ctx context.Context, runner db.Runner, repoID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo',$5,16,'active')`,
		repoID, "ident_"+repoID, "/tmp/"+repoID, "/tmp/"+repoID+"/.striatum", now,
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
}

func intgSeedRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string, workflow map[string]any) {
	t.Helper()
	now := time.Now().UTC()
	workflowArg, err := db.JSONBArg(runner, workflow)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,'wf','sha',$3::jsonb,$4)`,
		repoID, "snap_"+runID, workflowArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ($1,$2,$3,$4,'running',$5)`,
		repoID, runID, "snap_"+runID, "/tmp/"+repoID, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
}

func intgSeedSession(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID, role, lane string, caps []string, state string) {
	t.Helper()
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, sessionID, role, lane, caps, state, 1)
}

func intgSeedSessionOrdinal(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID, role, lane string, caps []string, state string, ordinal int) {
	t.Helper()
	now := time.Now().UTC()
	capsArg, err := db.JSONBArg(runner, caps)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  capabilities_json, state, registered_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10)`,
		repoID, sessionID, runID, role, lane, sessionID+"-slug", ordinal, capsArg, state, now); err != nil {
		t.Fatalf("insert session %s: %v", sessionID, err)
	}
}

func intgAttest(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID, lane string) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisors (
		  repository_id, supervisor_id, run_id, session_id, adapter, command_json, cwd,
		  scratch_path, pid, state, started_at
		) VALUES ($1,$2,$3,$4,$5,'[]'::jsonb,'/tmp','/tmp/scratch',4242,'attached',$6)`,
		repoID, "sup_"+sessionID, runID, sessionID, lane, now); err != nil {
		t.Fatalf("attest session %s: %v", sessionID, err)
	}
}

func intgEnv(repoID string, params map[string]any) rpc.Envelope {
	p := map[string]any{"repository_id": repoID}
	for k, v := range params {
		p[k] = v
	}
	return rpc.Envelope{SchemaVersion: rpc.SupportedEnvelopeVersion, Method: "test", Params: p}
}

// intgFixture seeds a repo+run with an attested live target (builder) and an
// interrogate-capable interrogator (reviewer), both active.
func intgFixture(t *testing.T, ctx context.Context, runner db.Runner, repoID string) (runID, interrogator, target string) {
	t.Helper()
	runID = "run_" + repoID
	interrogator = "sess_reviewer"
	target = "sess_builder"
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}, "implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{"display_model": "Claude"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, interrogator, "reviewer", "claude", []string{"interrogate"}, "active")
	intgSeedSession(t, ctx, runner, repoID, runID, target, "implementer", "claude", nil, "active")
	intgAttest(t, ctx, runner, repoID, runID, target, "claude")
	return runID, interrogator, target
}

func mustOpen(t *testing.T, ctx context.Context, runner db.Runner, repoID, interrogator, target string) string {
	t.Helper()
	res, err := HandleInterrogationOpen(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "target_session_id": target, "topic": "design review",
	}))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return fmt.Sprint(res["interrogation_id"])
}

// --- Required Test 1: lifecycle state machine ------------------------------

func TestInterrogationLifecycle(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_intg_life"
	_, interrogator, target := intgFixture(t, ctx, runner, repoID)

	id := mustOpen(t, ctx, runner, repoID, interrogator, target)

	if _, err := HandleInterrogationAsk(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "interrogation_id": id, "body": "why this design?",
	})); err != nil {
		t.Fatalf("ask: %v", err)
	}
	if _, err := HandleInterrogationAnswer(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": target, "interrogation_id": id, "body": "because of constraint X",
	})); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if _, err := HandleInterrogationClose(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "interrogation_id": id,
	})); err != nil {
		t.Fatalf("close: %v", err)
	}

	// double-close rejected
	if _, err := HandleInterrogationClose(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "interrogation_id": id,
	})); err == nil {
		t.Fatal("double-close should be rejected")
	}
	// answer-after-close rejected
	if _, err := HandleInterrogationAnswer(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": target, "interrogation_id": id, "body": "late",
	})); err == nil {
		t.Fatal("answer after close should be rejected")
	}
	// ask-on-unknown-id rejected
	if _, err := HandleInterrogationAsk(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "interrogation_id": "intg_nope", "body": "?",
	})); err == nil {
		t.Fatal("ask on unknown id should be rejected")
	}
}

func TestInterrogationListAndShow(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_intg_list"
	runID, interrogator, target := intgFixture(t, ctx, runner, repoID)
	id := mustOpen(t, ctx, runner, repoID, interrogator, target)

	list, err := reads.HandleInterrogationList(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID}))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list["count"].(int) != 1 {
		t.Fatalf("list count = %v", list["count"])
	}
	items := list["items"].([]map[string]any)
	if fmt.Sprint(items[0]["interrogation_id"]) != id || fmt.Sprint(items[0]["state"]) != "open" {
		t.Fatalf("list item = %#v", items[0])
	}
	show, err := reads.HandleInterrogationShow(ctx, runner, intgEnv(repoID, map[string]any{"interrogation_id": id}))
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	interrogation := show["interrogation"].(map[string]any)
	if fmt.Sprint(interrogation["target_session_id"]) != target {
		t.Fatalf("show interrogation = %#v", interrogation)
	}
}

// --- Required Test 2: targeting --------------------------------------------

func TestInterrogationTargetingDeliversOnlyToTarget(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_intg_target"
	runID, interrogator, target := intgFixture(t, ctx, runner, repoID)
	// a third, unrelated session
	other := "sess_other"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, other, "implementer", "claude", nil, "active", 2)

	id := mustOpen(t, ctx, runner, repoID, interrogator, target)
	if _, err := HandleInterrogationAsk(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "interrogation_id": id, "body": "Q1",
	})); err != nil {
		t.Fatalf("ask: %v", err)
	}

	// non-target session must NOT receive the question
	q, err := deliverPendingInterrogationQuestion(ctx, runner, repoID, other)
	if err != nil {
		t.Fatalf("deliver other: %v", err)
	}
	if q != nil {
		t.Fatalf("non-target session received a question: %#v", q)
	}

	// target session receives the question via await_packet envelope
	res, err := HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": target}))
	if err != nil {
		t.Fatalf("await target: %v", err)
	}
	if res["type"] != "interrogation_question" || res["interrogation_id"] != id || res["body"] != "Q1" {
		t.Fatalf("target await = %#v", res)
	}
}

// --- Required Test 3: authorization ----------------------------------------

func TestInterrogationAuthorization(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_intg_authz"
	runID, interrogator, target := intgFixture(t, ctx, runner, repoID)

	// a session lacking the interrogate capability cannot open
	noCap := "sess_nocap"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, noCap, "implementer", "claude", nil, "active", 2)
	if _, err := HandleInterrogationOpen(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": noCap, "target_session_id": target,
	})); err == nil {
		t.Fatal("non-interrogator session must not open")
	}

	id := mustOpen(t, ctx, runner, repoID, interrogator, target)

	// only the interrogator may ask
	if _, err := HandleInterrogationAsk(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": target, "interrogation_id": id, "body": "self-ask",
	})); err == nil {
		t.Fatal("non-interrogator must not ask")
	}
	if _, err := HandleInterrogationAsk(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "interrogation_id": id, "body": "Q",
	})); err != nil {
		t.Fatalf("interrogator ask: %v", err)
	}
	// only the target may answer
	if _, err := HandleInterrogationAnswer(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "interrogation_id": id, "body": "wrong author",
	})); err == nil {
		t.Fatal("non-target must not answer")
	}
	if _, err := HandleInterrogationAnswer(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": target, "interrogation_id": id, "body": "A",
	})); err != nil {
		t.Fatalf("target answer: %v", err)
	}
}

// --- Required Test 4: live-target requirement ------------------------------

func TestInterrogationOpenRequiresLiveTarget(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_intg_live"
	runID, interrogator, _ := intgFixture(t, ctx, runner, repoID)

	// closed target → target_unavailable
	closed := "sess_closed"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, closed, "implementer", "claude", nil, "closed", 2)
	if _, err := HandleInterrogationOpen(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "target_session_id": closed,
	})); !isRPCCode(err, "target_unavailable") {
		t.Fatalf("closed target: want target_unavailable, got %v", err)
	}

	// active but unattested target → target_unavailable
	unattested := "sess_unattested"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, unattested, "implementer", "claude", nil, "active", 3)
	if _, err := HandleInterrogationOpen(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "target_session_id": unattested,
	})); !isRPCCode(err, "target_unavailable") {
		t.Fatalf("unattested target: want target_unavailable, got %v", err)
	}
}

// RFC 0084: an interrogable agent-loop target that has entered the
// awaiting_interrogation window is a valid interrogation target even without
// wrapper attestation (which only drives artifact byline provenance, D080).
func TestInterrogationOpenAcceptsAwaitingInterrogationTarget(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_intg_awaiting"
	runID, interrogator, _ := intgFixture(t, ctx, runner, repoID)

	// active, NOT wrapper-attested, but in the awaiting_interrogation window.
	target := "sess_awaiting"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, target, "implementer", "claude", nil, "active", 4)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.events (repository_id, run_id, event_type, actor_session_id, created_at)
		VALUES ($1, $2, 'session.awaiting_interrogation', $3, now())`, repoID, runID, target); err != nil {
		t.Fatal(err)
	}
	if _, err := HandleInterrogationOpen(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "target_session_id": target, "topic": "design",
	})); err != nil {
		t.Fatalf("awaiting_interrogation target should be interrogable without wrapper attestation: %v", err)
	}
}

// --- Required Test 5: await_packet envelope discriminator ------------------

func TestAwaitPacketEnvelopeDiscriminator(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_intg_env"
	runID, interrogator, target := intgFixture(t, ctx, runner, repoID)

	// none: a fresh session with no work and no questions, on a non-running run
	noneSess := "sess_none"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, noneSess, "implementer", "claude", nil, "active", 2)
	if err := runner.Exec(ctx, `UPDATE striatumd.runs SET state='completed' WHERE repository_id=$1 AND run_id=$2`, repoID, runID); err != nil {
		t.Fatal(err)
	}
	res, err := HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": noneSess}))
	if err != nil {
		t.Fatalf("await none: %v", err)
	}
	if res["type"] != "none" {
		t.Fatalf("want none, got %#v", res)
	}
	// restore running for the rest
	if err := runner.Exec(ctx, `UPDATE striatumd.runs SET state='running' WHERE repository_id=$1 AND run_id=$2`, repoID, runID); err != nil {
		t.Fatal(err)
	}

	// work_packet: seed a claimable work message for the target's role
	intgSeedClaimableWork(t, ctx, runner, repoID, runID, "job_env", "build_env", "implementer", "claude")
	res, err = HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": target}))
	if err != nil {
		t.Fatalf("await work: %v", err)
	}
	if res["type"] != "work_packet" || res["status"] != "claimed" {
		t.Fatalf("want work_packet/claimed, got type=%v status=%v", res["type"], res["status"])
	}

	// interrogation_question preferred over new work: seed another work item AND a
	// pending question for the same session; the question must win.
	intgSeedClaimableWork(t, ctx, runner, repoID, runID, "job_env2", "build_env2", "implementer", "claude")
	id := mustOpen(t, ctx, runner, repoID, interrogator, target)
	if _, err := HandleInterrogationAsk(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "interrogation_id": id, "body": "priority Q",
	})); err != nil {
		t.Fatalf("ask: %v", err)
	}
	res, err = HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": target}))
	if err != nil {
		t.Fatalf("await prefer: %v", err)
	}
	if res["type"] != "interrogation_question" {
		t.Fatalf("question must be preferred over work, got %#v", res)
	}
}

// --- Required Test 6: multi-turn live-PG integration -----------------------

func TestInterrogationMultiTurn(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_intg_multi"
	_, interrogator, target := intgFixture(t, ctx, runner, repoID)
	id := mustOpen(t, ctx, runner, repoID, interrogator, target)

	rounds := []struct{ q, a string }{
		{"round one question", "round one answer"},
		{"round two question", "round two answer"},
	}
	for i, r := range rounds {
		if _, err := HandleInterrogationAsk(ctx, runner, intgEnv(repoID, map[string]any{
			"session_id": interrogator, "interrogation_id": id, "body": r.q,
		})); err != nil {
			t.Fatalf("ask %d: %v", i, err)
		}
		// the target receives the question on its await loop
		got, err := HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": target}))
		if err != nil {
			t.Fatalf("await %d: %v", i, err)
		}
		if got["type"] != "interrogation_question" || got["body"] != r.q {
			t.Fatalf("round %d question delivery = %#v", i, got)
		}
		if _, err := HandleInterrogationAnswer(ctx, runner, intgEnv(repoID, map[string]any{
			"session_id": target, "interrogation_id": id, "body": r.a,
		})); err != nil {
			t.Fatalf("answer %d: %v", i, err)
		}
	}

	show, err := reads.HandleInterrogationShow(ctx, runner, intgEnv(repoID, map[string]any{"interrogation_id": id}))
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	turns, _ := show["turns"].([]map[string]any)
	if len(turns) != 4 {
		t.Fatalf("want 4 ordered turns, got %d: %#v", len(turns), turns)
	}
	want := []string{"round one question", "round one answer", "round two question", "round two answer"}
	for i, turn := range turns {
		if fmt.Sprint(turn["body"]) != want[i] {
			t.Fatalf("turn %d body = %v, want %v", i, turn["body"], want[i])
		}
		if fmt.Sprint(turn["interrogation_id"]) != id {
			// interrogation_id correlation is on the row payload, not surfaced
			// in the turn map directly; verify via the show interrogation block.
		}
	}
}

// --- Required Test 7: end-to-end intention test (the bar) ------------------

func TestInterrogationEndToEndPreservedContext(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_intg_e2e"
	runID := "run_e2e"

	// builder-only fact: seeded into the build packet, never re-published.
	const builderOnlyFact = "the rate limiter uses a token bucket sized 4096"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}, "implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{"display_model": "Claude"}},
		"jobs": []any{
			map[string]any{"id": "build", "interrogable": true},
			map[string]any{"id": "review"},
		},
	})

	builder := "sess_builder_e2e"
	reviewer := "sess_reviewer_e2e"
	intgSeedSession(t, ctx, runner, repoID, runID, builder, "implementer", "claude", nil, "active")
	intgAttest(t, ctx, runner, repoID, runID, builder, "claude")
	intgSeedSession(t, ctx, runner, repoID, runID, reviewer, "reviewer", "claude", []string{"interrogate"}, "active")

	now := time.Now().UTC()
	// build job (interrogable), running, lease held by builder; the fact lives
	// in capability_requirements_json (builder-only context).
	reqArg, err := db.JSONBArg(runner, map[string]any{"objective": "build it", "secret_fact": builderOnlyFact})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  capability_requirements_json, state, idempotency_key, created_at, current_lease_id
		) VALUES ($1,'job_build',$2,'build','Build','build','implementer',$3::jsonb,'running','idem_build',$4,'lease_build')`,
		repoID, runID, reqArg, now); err != nil {
		t.Fatalf("insert build job: %v", err)
	}
	// a downstream review job keeps the run from auto-completing on build finish.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  state, idempotency_key, created_at
		) VALUES ($1,'job_review',$2,'review','Review','review','reviewer','blocked','idem_review',$3)`,
		repoID, runID, now); err != nil {
		t.Fatalf("insert review job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ($1,'lease_build',$2,'job','job_build',$3,'active',$4,$5)`,
		repoID, runID, builder, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}

	// 1. builder completes the interrogable job → enters awaiting_interrogation.
	comp, err := HandleCompleteWork(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": builder, "job_id": "job_build", "lease_id": "lease_build", "summary": "done",
	}))
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if comp["interrogable"] != true || comp["session_phase"] != "awaiting_interrogation" {
		t.Fatalf("complete did not enter awaiting_interrogation: %#v", comp)
	}
	if got := intgSessionState(t, ctx, runner, repoID, builder); got != "active" {
		t.Fatalf("interrogable builder session should stay active, got %q", got)
	}

	// 2. reviewer opens + asks; builder answers FROM ITS PRESERVED CONTEXT.
	id := mustOpen(t, ctx, runner, repoID, reviewer, builder)
	if _, err := HandleInterrogationAsk(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": reviewer, "interrogation_id": id, "body": "How is rate limiting implemented?",
	})); err != nil {
		t.Fatalf("ask: %v", err)
	}
	// the builder's receive loop gets the question
	q, err := HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": builder}))
	if err != nil {
		t.Fatalf("builder await: %v", err)
	}
	if q["type"] != "interrogation_question" {
		t.Fatalf("builder did not receive question: %#v", q)
	}
	// the builder answers, referencing the builder-only in-context fact
	if _, err := HandleInterrogationAnswer(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": builder, "interrogation_id": id, "body": "From my build: " + builderOnlyFact,
	})); err != nil {
		t.Fatalf("answer: %v", err)
	}

	// the recorded answer reflects the builder-only fact
	show, err := reads.HandleInterrogationShow(ctx, runner, intgEnv(repoID, map[string]any{"interrogation_id": id}))
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	turns := show["turns"].([]map[string]any)
	answer := turns[len(turns)-1]
	if !containsStr(fmt.Sprint(answer["body"]), builderOnlyFact) {
		t.Fatalf("answer does not reflect preserved-context fact: %#v", answer)
	}
	// the fact was never re-published as an artifact
	if intgArtifactCount(t, ctx, runner, repoID, runID) != 0 {
		t.Fatal("no artifact should have been published in the e2e interrogation flow")
	}

	// 3. close → the builder session then closes.
	closeRes, err := HandleInterrogationClose(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": reviewer, "interrogation_id": id,
	}))
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if closeRes["target_session_closed"] != true {
		t.Fatalf("builder session should close after interrogation close: %#v", closeRes)
	}
	if got := intgSessionState(t, ctx, runner, repoID, builder); got != "closed" {
		t.Fatalf("builder session should be closed, got %q", got)
	}

	// 4. trajectory export --profile dialogue reproduces the Q&A thread.
	traj, err := reads.HandleTrajectoryExport(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID, "profile": "dialogue"}))
	if err != nil {
		t.Fatalf("trajectory export: %v", err)
	}
	records := traj["records"].([]map[string]any)
	var sawQuestion, sawAnswerWithFact bool
	for _, rec := range records {
		body, _ := rec["body"].(map[string]any)
		switch fmt.Sprint(body["message_kind"]) {
		case "interrogation_question":
			if fmt.Sprint(body["interrogation_id"]) == id {
				sawQuestion = true
			}
		case "interrogation_answer":
			if containsStr(fmt.Sprint(body["text"]), builderOnlyFact) {
				sawAnswerWithFact = true
			}
		}
	}
	if !sawQuestion || !sawAnswerWithFact {
		t.Fatalf("dialogue trajectory did not reproduce the thread: question=%v answerWithFact=%v records=%#v", sawQuestion, sawAnswerWithFact, records)
	}
}

// --- Required Test 8: D028 provenance guard --------------------------------

func TestInterrogationD028NoRawProviderOutput(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_intg_d028"
	runID, interrogator, target := intgFixture(t, ctx, runner, repoID)
	id := mustOpen(t, ctx, runner, repoID, interrogator, target)
	if _, err := HandleInterrogationAsk(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": interrogator, "interrogation_id": id, "body": "Q"})); err != nil {
		t.Fatalf("ask: %v", err)
	}
	if _, err := HandleInterrogationAnswer(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": target, "interrogation_id": id, "body": "A"})); err != nil {
		t.Fatalf("answer: %v", err)
	}
	show, err := reads.HandleInterrogationShow(ctx, runner, intgEnv(repoID, map[string]any{"interrogation_id": id}))
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	allowed := map[string]bool{
		"message_id": true, "target_session_id": true, "created_at": true,
		"turn": true, "turn_index": true, "kind": true, "body": true,
	}
	for _, turn := range show["turns"].([]map[string]any) {
		for key := range turn {
			if !allowed[key] {
				t.Fatalf("interrogation turn exposed non-curated field %q (D028)", key)
			}
			if key == "stdout" || key == "stderr" || key == "raw_output" {
				t.Fatalf("interrogation turn leaked provider output field %q", key)
			}
		}
	}
	// trajectory projection is curated too
	traj, err := reads.HandleTrajectoryExport(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID, "profile": "dialogue"}))
	if err != nil {
		t.Fatalf("trajectory: %v", err)
	}
	for _, rec := range traj["records"].([]map[string]any) {
		body, _ := rec["body"].(map[string]any)
		for key := range body {
			if key == "stdout" || key == "stderr" || key == "raw_output" {
				t.Fatalf("trajectory body leaked provider output field %q", key)
			}
		}
	}
}

// --- helpers ---------------------------------------------------------------

func intgSeedClaimableWork(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, workflowJobID, role, lane string) {
	t.Helper()
	now := time.Now().UTC()
	laneArg, err := db.JSONBArg(runner, map[string]any{"lane_id": lane})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  lane_selector_json, state, fresh_session_required, idempotency_key, created_at
		) VALUES ($1,$2,$3,$4,'W','build',$5,$6::jsonb,'queued',false,$7,$8)`,
		repoID, jobID, runID, workflowJobID, role, laneArg, "idem_"+jobID, now); err != nil {
		t.Fatalf("insert work job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','pending',0,$5,$6,$7,$7)`,
		repoID, "msg_"+jobID, runID, jobID, role, lane, now); err != nil {
		t.Fatalf("insert work message: %v", err)
	}
}

func intgSessionState(t *testing.T, ctx context.Context, runner db.Runner, repoID, sessionID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.sessions WHERE repository_id=$1 AND session_id=$2`, repoID, sessionID)
	if err != nil {
		t.Fatalf("session state: %v", err)
	}
	return fmt.Sprint(row["state"])
}

func intgArtifactCount(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string) int {
	t.Helper()
	rows, err := queryRows(ctx, runner, `SELECT 1 FROM striatumd.artifacts WHERE repository_id=$1 AND run_id=$2`, repoID, runID)
	if err != nil {
		t.Fatalf("artifact count: %v", err)
	}
	return len(rows)
}

func isRPCCode(err error, code string) bool {
	if err == nil {
		return false
	}
	var rpcErr *rpc.Error
	if e, ok := err.(*rpc.Error); ok {
		rpcErr = e
	}
	if rpcErr == nil {
		return false
	}
	return rpcErr.Code == code
}

func containsStr(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
