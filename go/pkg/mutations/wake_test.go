package mutations

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestWakeCollectorPublishesOnlyAfterCommit(t *testing.T) {
	broker := resetWakeBusForTest(t)
	ch, cancel := broker.Subscribe(wakeFilter{RepositoryID: "repo_1", RunID: "run_1"})
	defer cancel()
	runner := &wakeTestRunner{tx: &wakeTestTx{}}

	_, err := withTx(context.Background(), runner, func(tx db.TxRunner) (map[string]any, error) {
		recordWake(tx, WakeEvent{
			RepositoryID: "repo_1",
			RunID:        "run_1",
			Kind:         "work_available",
			MessageID:    "msg_1",
		})
		select {
		case event := <-ch:
			t.Fatalf("wake published before commit: %#v", event)
		default:
		}
		return map[string]any{"ok": true}, nil
	})
	if err != nil {
		t.Fatalf("withTx: %v", err)
	}
	if !runner.tx.committed {
		t.Fatal("transaction was not committed")
	}
	event := mustReceiveWake(t, ch)
	if event.Kind != "work_available" || event.MessageID != "msg_1" || event.Sequence == 0 {
		t.Fatalf("wake event = %#v", event)
	}
}

func TestWakeCollectorDropsRollbackEvents(t *testing.T) {
	broker := resetWakeBusForTest(t)
	ch, cancel := broker.Subscribe(wakeFilter{RepositoryID: "repo_1", RunID: "run_1"})
	defer cancel()
	runner := &wakeTestRunner{tx: &wakeTestTx{}}
	wantErr := errors.New("abort")

	_, err := withTx(context.Background(), runner, func(tx db.TxRunner) (map[string]any, error) {
		recordWake(tx, WakeEvent{
			RepositoryID: "repo_1",
			RunID:        "run_1",
			Kind:         "work_available",
			MessageID:    "msg_rollback",
		})
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("withTx err = %v, want %v", err, wantErr)
	}
	if !runner.tx.rolledBack {
		t.Fatal("transaction was not rolled back")
	}
	select {
	case event := <-ch:
		t.Fatalf("rollback wake published: %#v", event)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestWakeWaitReturnsNotification(t *testing.T) {
	broker := resetWakeBusForTest(t)
	ctx := context.Background()
	resultCh := make(chan map[string]any, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := HandleWakeWait(ctx, inertRunner{}, rpc.Envelope{
			SchemaVersion: rpc.SupportedEnvelopeVersion,
			Method:        "wake.wait",
			Params: map[string]any{
				"repository_id": "repo_1",
				"run_id":        "run_1",
				"timeout_ms":    500,
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	time.Sleep(10 * time.Millisecond)
	broker.Publish(WakeEvent{RepositoryID: "repo_1", RunID: "run_1", Kind: "agent_message_available", MessageID: "msg_1"})

	select {
	case err := <-errCh:
		t.Fatalf("wake.wait error: %v", err)
	case result := <-resultCh:
		if result["status"] != "notified" {
			t.Fatalf("wake.wait result = %#v", result)
		}
		event := asMap(result["event"])
		if event["kind"] != "agent_message_available" || event["message_id"] != "msg_1" {
			t.Fatalf("wake event payload = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("wake.wait did not return notification")
	}
}

// TestWakeBusIsolatesAcrossRunsAndRepositories locks the wakeFilter negative
// case (RFC 0120 review Q13): a waiter scoped to one (repository, run) must
// never be woken by activity in a different run or a different repository, and a
// repository-wide waiter (empty run_id — the shape `run drive` uses) must see
// every run in its own repository but nothing from another repository. Every
// other wake_test subscriber filters on the matching repo/run, so only the
// positive match was covered; a regression in wakeSubscription.matches would
// cross-wake the wrong driver to a no-op reconcile (benign) or mask a missing
// same-run wake behind a neighbor's event (a stall).
func TestWakeBusIsolatesAcrossRunsAndRepositories(t *testing.T) {
	broker := resetWakeBusForTest(t)

	repoARunA, cancelAA := broker.Subscribe(wakeFilter{RepositoryID: "repo_A", RunID: "run_A"})
	defer cancelAA()
	repoAWide, cancelAWide := broker.Subscribe(wakeFilter{RepositoryID: "repo_A"})
	defer cancelAWide()
	repoBRunB, cancelBB := broker.Subscribe(wakeFilter{RepositoryID: "repo_B", RunID: "run_B"})
	defer cancelBB()

	// A wake for repo_A/run_B reaches the repo_A-wide waiter only: not the
	// run_A-scoped waiter (different run, same repo), not the repo_B waiter.
	broker.Publish(WakeEvent{RepositoryID: "repo_A", RunID: "run_B", Kind: "work_available", MessageID: "msg_AB"})
	assertNoWake(t, repoARunA, "repo_A/run_A waiter received a repo_A/run_B wake (cross-run leak)")
	assertNoWake(t, repoBRunB, "repo_B/run_B waiter received a repo_A/run_B wake (cross-repository leak)")
	if got := mustReceiveWake(t, repoAWide); got.RunID != "run_B" || got.MessageID != "msg_AB" {
		t.Fatalf("repo_A-wide waiter event = %#v, want repo_A/run_B/msg_AB", got)
	}

	// A wake for repo_B/run_B reaches only the repo_B waiter — not the repo_A
	// run-scoped waiter and not the repo_A-wide waiter (repository isolation
	// must hold even for the empty-run_id, repo-wide subscription).
	broker.Publish(WakeEvent{RepositoryID: "repo_B", RunID: "run_B", Kind: "work_available", MessageID: "msg_BB"})
	assertNoWake(t, repoARunA, "repo_A/run_A waiter received a repo_B wake")
	assertNoWake(t, repoAWide, "repo_A-wide waiter received a repo_B wake (cross-repository leak)")
	if got := mustReceiveWake(t, repoBRunB); got.RepositoryID != "repo_B" || got.MessageID != "msg_BB" {
		t.Fatalf("repo_B/run_B waiter event = %#v, want repo_B/run_B/msg_BB", got)
	}

	// Positive control: the matching same-run wake still reaches its run-scoped
	// waiter (and the repo-wide one), proving the negatives above are isolation,
	// not a dead bus.
	broker.Publish(WakeEvent{RepositoryID: "repo_A", RunID: "run_A", Kind: "work_available", MessageID: "msg_AA"})
	if got := mustReceiveWake(t, repoARunA); got.RunID != "run_A" || got.MessageID != "msg_AA" {
		t.Fatalf("repo_A/run_A waiter event = %#v, want run_A/msg_AA", got)
	}
	if got := mustReceiveWake(t, repoAWide); got.RunID != "run_A" || got.MessageID != "msg_AA" {
		t.Fatalf("repo_A-wide waiter event = %#v, want run_A/msg_AA", got)
	}
}

func TestWakeWaitReturnsTimeout(t *testing.T) {
	ctx := context.Background()
	timeout, err := HandleWakeWait(ctx, inertRunner{}, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "wake.wait",
		Params: map[string]any{
			"repository_id": "repo_1",
			"run_id":        "run_1",
			"timeout_ms":    0,
		},
	})
	if err != nil {
		t.Fatalf("wake.wait timeout: %v", err)
	}
	if timeout["status"] != "timeout" {
		t.Fatalf("timeout result = %#v", timeout)
	}
}

func TestWakeEventEmittedForRunStartEnqueue(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	broker := resetWakeBusForTest(t)
	repoID := "repo_wake_run_start"
	runID := "run_wake_run_start"

	seedWakeAuthorRun(t, ctx, runner, repoID, runID)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs SET state = 'ready'
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID); err != nil {
		t.Fatalf("mark run ready: %v", err)
	}
	insertWakeAuthorJob(t, ctx, runner, repoID, runID, "job_author", "author_draft", "blocked")
	workCh, cancelWork := broker.Subscribe(wakeFilter{RepositoryID: repoID, RunID: runID})
	defer cancelWork()
	if _, err := HandleRunStart(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID})); err != nil {
		t.Fatalf("run.start: %v", err)
	}
	assertWakeKind(t, mustReceiveWake(t, workCh), "work_available")
}

func TestWakeEventEmittedForSameAttemptRequeue(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	broker := resetWakeBusForTest(t)
	repoID := "repo_wake_requeue"
	runID := "run_wake_requeue"

	seedWakeAuthorRun(t, ctx, runner, repoID, runID)
	requeueJobID := "job_requeue"
	insertWakeAuthorJob(t, ctx, runner, repoID, runID, requeueJobID, "requeue", "running")
	requeueCh, cancelRequeue := broker.Subscribe(wakeFilter{RepositoryID: repoID, RunID: runID})
	defer cancelRequeue()
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		job, err := rowByID(ctx, tx, repoID, "jobs", "job_id", requeueJobID, true)
		if err != nil {
			return nil, err
		}
		_, err = requeueJobSameAttempt(ctx, tx, repoID, job, requeueSameAttemptOptions{})
		return map[string]any{"ok": true}, err
	}); err != nil {
		t.Fatalf("same-attempt requeue: %v", err)
	}
	assertWakeKind(t, mustReceiveWake(t, requeueCh), "work_available")
}

func TestWakeEventEmittedForInterrogationQuestion(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	broker := resetWakeBusForTest(t)
	repoID := "repo_wake_peer"
	runID, interrogator, target := intgFixture(t, ctx, runner, repoID)
	interrogationID := mustOpen(t, ctx, runner, repoID, interrogator, target)

	agentCh, cancelAgent := broker.Subscribe(wakeFilter{RepositoryID: repoID, RunID: runID})
	defer cancelAgent()
	if _, err := HandleInterrogationAsk(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id":       interrogator,
		"interrogation_id": interrogationID,
		"body":             "what changed?",
	})); err != nil {
		t.Fatalf("interrogation.ask: %v", err)
	}
	assertWakeKind(t, mustReceiveWake(t, agentCh), "agent_message_available")
}

func TestWakeEventEmittedForConversationOpen(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	broker := resetWakeBusForTest(t)
	repoID := "repo_wake_conv_open"
	runID := "run_wake_conv_open"
	a, b := seedWakeConversation(t, ctx, runner, repoID, runID)

	conversationCh, cancelConversation := broker.Subscribe(wakeFilter{RepositoryID: repoID, RunID: runID})
	defer cancelConversation()
	if _, err := HandleConversationOpen(ctx, runner, intgEnv(repoID, map[string]any{
		"participant_session_ids": []any{a, b},
		"topic":                   "wake",
		"max_rounds":              2,
	})); err != nil {
		t.Fatalf("conversation.open: %v", err)
	}
	assertWakeKind(t, mustReceiveWake(t, conversationCh), "conversation_turn_available")
}

func TestWakeEventEmittedForConversationSay(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	broker := resetWakeBusForTest(t)
	repoID := "repo_wake_conv_say"
	runID := "run_wake_conv_say"
	a, b := seedWakeConversation(t, ctx, runner, repoID, runID)

	open, err := HandleConversationOpen(ctx, runner, intgEnv(repoID, map[string]any{
		"participant_session_ids": []any{a, b},
		"topic":                   "wake",
		"max_rounds":              2,
	}))
	if err != nil {
		t.Fatalf("conversation.open: %v", err)
	}
	conversationCh, cancelConversation := broker.Subscribe(wakeFilter{RepositoryID: repoID, RunID: runID})
	defer cancelConversation()

	if _, err := HandleConversationSay(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id":      a,
		"conversation_id": fmt.Sprint(open["conversation_id"]),
		"body":            "first turn",
	})); err != nil {
		t.Fatalf("conversation.say: %v", err)
	}
	assertWakeKind(t, mustReceiveWake(t, conversationCh), "conversation_turn_available")
}

func seedWakeAuthorRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string) {
	t.Helper()
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"author": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
		"jobs":        []any{map[string]any{"id": "author_draft", "type": "draft", "role_id": "author"}},
	})
}

func insertWakeAuthorJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, workflowJobID, state string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  lane_selector_json, title, job_type, idempotency_key, expected_artifacts_json,
		  created_at
		) VALUES ($1,$2,$3,$4,1,$5,'author',
		  '{"lane_id":"codex"}'::jsonb,'Author','draft','idem_'||$2,'[]'::jsonb,NOW())`,
		repoID, jobID, runID, workflowJobID, state); err != nil {
		t.Fatalf("insert wake author job: %v", err)
	}
}

func seedWakeConversation(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string) (string, string) {
	t.Helper()
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"jobs": []any{}})
	a := "sess_conv_a_" + repoID
	b := "sess_conv_b_" + repoID
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, a, "participant", "claude", nil, "active", 1)
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, b, "participant", "codex", nil, "active", 2)
	return a, b
}

func resetWakeBusForTest(t *testing.T) *wakeBroker {
	t.Helper()
	old := wakeBus
	broker := newWakeBroker()
	wakeBus = broker
	t.Cleanup(func() {
		wakeBus = old
	})
	return broker
}

func assertNoWake(t *testing.T, ch <-chan WakeEvent, msg string) {
	t.Helper()
	// Publish is synchronous into buffered(1) channels, so a matching event is
	// already queued by the time Publish returns; a short window is enough to
	// catch a leak without being flaky.
	select {
	case event := <-ch:
		t.Fatalf("%s: %#v", msg, event)
	case <-time.After(20 * time.Millisecond):
	}
}

func mustReceiveWake(t *testing.T, ch <-chan WakeEvent) WakeEvent {
	t.Helper()
	select {
	case event := <-ch:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for wake event")
	}
	return WakeEvent{}
}

func assertWakeKind(t *testing.T, event WakeEvent, kind string) {
	t.Helper()
	if event.Kind != kind {
		t.Fatalf("wake kind = %q, want %q; event=%#v", event.Kind, kind, event)
	}
	if event.RepositoryID == "" || event.RunID == "" {
		t.Fatalf("wake event missing scope: %#v", event)
	}
}

type wakeTestRunner struct {
	tx *wakeTestTx
}

func (r *wakeTestRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected exec")
}

func (r *wakeTestRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{}
}

func (r *wakeTestRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (r *wakeTestRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return r.tx, nil
}

type wakeTestTx struct {
	committed  bool
	rolledBack bool
}

func (tx *wakeTestTx) Exec(context.Context, string, ...any) error {
	return nil
}

func (tx *wakeTestTx) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{}
}

func (tx *wakeTestTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (tx *wakeTestTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *wakeTestTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}
