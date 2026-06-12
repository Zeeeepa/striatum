package rundrive

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestRunDriveReconcileIsIdempotent(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDrive()
	fake.jobs = []map[string]any{
		job("job_author", "author_draft", "author", "codex", "queued", 1),
		job("job_review", "review", "reviewer", "agy", "queued", 1),
	}
	driver := testDriver(fake)

	actions1, _, _, err := driver.ReconcileOnce(ctx)
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	actions2, _, _, err := driver.ReconcileOnce(ctx)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if got := fake.count("session.register"); got != 2 {
		t.Fatalf("session.register calls = %d, want 2; actions1=%#v actions2=%#v", got, actions1, actions2)
	}
	if got := fake.count("supervise.start"); got != 2 {
		t.Fatalf("supervise.start calls = %d, want 2", got)
	}
	if len(actions2) != 0 {
		t.Fatalf("second reconcile should be quiet/idempotent, got %#v", actions2)
	}
}

func TestRunDriveAdoptsExistingSessions(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDrive()
	fake.jobs = []map[string]any{job("job_author", "author_draft", "author", "codex", "queued", 1)}
	fake.sessions = []map[string]any{session("sess_existing", "author", "codex", "active")}
	driver := testDriver(fake)

	actions, _, _, err := driver.ReconcileOnce(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := fake.count("session.register"); got != 0 {
		t.Fatalf("session.register calls = %d, want 0", got)
	}
	if got := fake.count("supervise.start"); got != 0 {
		t.Fatalf("supervise.start calls = %d, want 0", got)
	}
	if len(actions) != 1 || actions[0].Action != "adopt" || actions[0].SessionID != "sess_existing" {
		t.Fatalf("actions = %#v, want adopt existing session", actions)
	}
}

func TestRunDriveClosesCompletedLanesBeforeFreshReviewer(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDrive()
	fake.jobs = []map[string]any{
		job("job_author", "author_draft", "author", "codex", "completed", 1),
		job("job_review", "review", "reviewer", "agy", "queued", 1),
	}
	fake.sessions = []map[string]any{session("sess_author", "author", "codex", "active")}
	driver := testDriver(fake)
	driver.launched[slotKey{WorkflowJobID: "author_draft", Attempt: 1}] = "sess_author"

	actions, _, _, err := driver.ReconcileOnce(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	stopIndex := fake.first("supervise.stop")
	registerIndex := fake.first("session.register")
	if stopIndex < 0 || registerIndex < 0 || stopIndex > registerIndex {
		t.Fatalf("call order = %#v, want supervise.stop before session.register; actions=%#v", fake.calls, actions)
	}
}

func TestRunDriveRefreshesSessionsAfterStoppingCompletedLane(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDrive()
	fake.jobs = []map[string]any{
		job("job_author_1", "author_draft", "author", "codex", "completed", 1),
		job("job_author_2", "author_revision", "author", "codex", "queued", 1),
	}
	fake.sessions = []map[string]any{session("sess_author_1", "author", "codex", "active")}
	driver := testDriver(fake)
	driver.launched[slotKey{WorkflowJobID: "author_draft", Attempt: 1}] = "sess_author_1"

	actions, _, _, err := driver.ReconcileOnce(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := fake.count("session.register"); got != 1 {
		t.Fatalf("session.register calls = %d, want 1; calls=%#v actions=%#v", got, fake.calls, actions)
	}
	for _, action := range actions {
		if action.Action == "adopt" && action.SessionID == "sess_author_1" {
			t.Fatalf("stopped session was adopted by queued job: actions=%#v", actions)
		}
	}
}

func TestRunDriveStopsSupersededLaunchedAttempt(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDrive()
	fake.jobs = []map[string]any{job("job_review", "review", "reviewer", "agy", "queued", 2)}
	fake.sessions = []map[string]any{session("sess_review_1", "reviewer", "agy", "active")}
	driver := testDriver(fake)
	driver.launched[slotKey{WorkflowJobID: "review", Attempt: 1}] = "sess_review_1"

	actions, _, _, err := driver.ReconcileOnce(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	stopIndex := fake.first("supervise.stop")
	registerIndex := fake.first("session.register")
	if stopIndex < 0 || registerIndex < 0 || stopIndex > registerIndex {
		t.Fatalf("call order = %#v, want superseded attempt stopped before register; actions=%#v", fake.calls, actions)
	}
	if !fake.sessionState("sess_review_1", "closed") {
		t.Fatalf("superseded review session was not closed; sessions=%#v actions=%#v", fake.sessions, actions)
	}
}

func TestRunDriveFreshRetryStopsOnlyCompletedRoleLane(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDrive()
	fake.freshRefusals = 1
	fake.jobs = []map[string]any{
		job("job_author_done", "author_draft", "author", "codex", "completed", 1),
		job("job_review", "review", "reviewer", "agy", "queued", 1),
	}
	fake.sessions = []map[string]any{
		session("sess_author_done", "author", "codex", "active"),
		session("sess_author_other_lane", "author", "agy", "active"),
	}
	driver := testDriver(fake)

	actions, _, _, err := driver.ReconcileOnce(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := fake.count("session.register"); got != 2 {
		t.Fatalf("session.register calls = %d, want failed attempt plus retry; calls=%#v actions=%#v", got, fake.calls, actions)
	}
	if !fake.sessionState("sess_author_done", "closed") {
		t.Fatalf("completed author lane was not stopped; sessions=%#v actions=%#v", fake.sessions, actions)
	}
	if !fake.sessionState("sess_author_other_lane", "active") {
		t.Fatalf("other same-role lane was stopped; sessions=%#v actions=%#v", fake.sessions, actions)
	}
}

func TestRunDriveNeverCallsRescueVerbs(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDrive()
	fake.jobs = []map[string]any{job("job_author", "author_draft", "author", "codex", "queued", 1)}
	driver := testDriver(fake)

	if _, _, _, err := driver.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	for _, call := range fake.calls {
		if !AllowedMethods[call.method] {
			t.Fatalf("method %s is outside run-drive allowlist", call.method)
		}
		switch call.method {
		case "recovery.requeue_stale", "review.override", "run.retry_job":
			t.Fatalf("run drive must not call rescue verb %s", call.method)
		}
		if force, _ := call.params["force"].(bool); force {
			t.Fatalf("run drive must not use --force-style params: %#v", call)
		}
	}
}

func TestRunDriveStopsFastOnProviderAuthRefusal(t *testing.T) {
	ctx := context.Background()
	fake := newFakeDrive()
	fake.jobs = []map[string]any{job("job_author", "author_draft", "author", "codex", "queued", 1)}
	fake.startErr = rpc.NewError("lane_provider_auth_failed", "raw provider output must not be replayed", nil)
	driver := New(fake, Options{
		RepositoryID:     "repo_1",
		RunID:            "run_1",
		ProviderAuthGate: "required",
		Now:              func() time.Time { return time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC) },
	})

	actions, _, _, err := driver.ReconcileOnce(ctx)
	var providerErr ProviderAuthRefusalError
	if !errors.As(err, &providerErr) || providerErr.Code != "lane_provider_auth_failed" {
		t.Fatalf("err = %#v, want ProviderAuthRefusalError lane_provider_auth_failed", err)
	}
	if got := fake.count("session.register"); got != 1 {
		t.Fatalf("session.register calls = %d, want 1; calls=%#v", got, fake.calls)
	}
	if got := fake.count("supervise.start"); got != 1 {
		t.Fatalf("supervise.start calls = %d, want 1; calls=%#v", got, fake.calls)
	}
	if got := fake.count("session.close"); got != 1 {
		t.Fatalf("session.close calls = %d, want 1; calls=%#v", got, fake.calls)
	}
	if len(actions) != 1 || actions[0].Action != "supervise.start" || actions[0].Result != "blocked" {
		t.Fatalf("actions = %#v, want one blocked supervise.start action", actions)
	}
	if strings.Contains(actions[0].Message, "raw provider output") {
		t.Fatalf("run-drive action replayed raw provider message: %#v", actions[0])
	}
	startCall := fake.calls[fake.first("supervise.start")]
	if startCall.params["provider_auth_gate"] != "required" {
		t.Fatalf("supervise.start params = %#v", startCall.params)
	}
	closeCall := fake.calls[fake.first("session.close")]
	if !strings.Contains(fmt.Sprint(closeCall.params["reason"]), "lane_provider_auth_failed") {
		t.Fatalf("session.close reason = %#v", closeCall.params["reason"])
	}
}

func TestRunDriveRunUsesDefaultSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := newFakeDrive()

	err := Run(ctx, fake, Options{
		RepositoryID: "repo_1",
		RunID:        "run_1",
		Interval:     time.Hour,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
}

type fakeDrive struct {
	runState      string
	jobs          []map[string]any
	sessions      []map[string]any
	calls         []fakeCall
	nextID        int
	freshRefusals int
	startErr      error
}

type fakeCall struct {
	method string
	params map[string]any
}

func newFakeDrive() *fakeDrive {
	return &fakeDrive{runState: "running", nextID: 1}
}

func (f *fakeDrive) Invoke(_ context.Context, method string, params map[string]any) (map[string]any, error) {
	f.calls = append(f.calls, fakeCall{method: method, params: cloneParams(params)})
	switch method {
	case "run.detail":
		return map[string]any{
			"run":      map[string]any{"run_id": params["run_id"], "state": f.runState},
			"jobs":     cloneRows(f.jobs),
			"workflow": map[string]any{"jobs": []any{}},
		}, nil
	case "list.sessions":
		return map[string]any{"items": cloneRows(f.sessions)}, nil
	case "session.register":
		if f.freshRefusals > 0 {
			f.freshRefusals--
			return nil, errors.New("reviewer_context_policy: fresh registration refused; pass --force-non-fresh to override")
		}
		role := fmt.Sprint(params["role"])
		lane := fmt.Sprint(params["lane"])
		sessionID := fmt.Sprintf("sess_%d", f.nextID)
		f.nextID++
		f.sessions = append(f.sessions, session(sessionID, role, lane, "active"))
		return map[string]any{"session_id": sessionID}, nil
	case "supervise.start":
		if f.startErr != nil {
			return nil, f.startErr
		}
		return map[string]any{"state": "attached", "session_id": params["session_id"]}, nil
	case "supervise.stop":
		sessionID := fmt.Sprint(params["session_id"])
		for _, sess := range f.sessions {
			if fmt.Sprint(sess["session_id"]) == sessionID {
				sess["state"] = "closed"
			}
		}
		return map[string]any{"state": "stopped", "session_id": sessionID}, nil
	case "session.close":
		sessionID := fmt.Sprint(params["session_id"])
		for _, sess := range f.sessions {
			if fmt.Sprint(sess["session_id"]) == sessionID {
				sess["state"] = "closed"
			}
		}
		return map[string]any{"state": "closed", "session_id": params["session_id"]}, nil
	default:
		return nil, errors.New("unexpected method: " + method)
	}
}

func (f *fakeDrive) count(method string) int {
	total := 0
	for _, call := range f.calls {
		if call.method == method {
			total++
		}
	}
	return total
}

func (f *fakeDrive) first(method string) int {
	for i, call := range f.calls {
		if call.method == method {
			return i
		}
	}
	return -1
}

func (f *fakeDrive) sessionState(sessionID string, state string) bool {
	for _, sess := range f.sessions {
		if fmt.Sprint(sess["session_id"]) == sessionID {
			return fmt.Sprint(sess["state"]) == state
		}
	}
	return false
}

func testDriver(fake *fakeDrive) *Driver {
	return New(fake, Options{
		RepositoryID: "repo_1",
		RunID:        "run_1",
		Once:         true,
		Now: func() time.Time {
			return time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
		},
	})
}

func job(jobID, workflowJobID, role, lane, state string, attempt int) map[string]any {
	return map[string]any{
		"job_id":             jobID,
		"workflow_job_id":    workflowJobID,
		"role_id":            role,
		"state":              state,
		"attempt":            attempt,
		"lane_selector_json": map[string]any{"lane_id": lane},
	}
}

func session(sessionID, role, lane, state string) map[string]any {
	return map[string]any{
		"session_id": sessionID,
		"role_id":    role,
		"lane_id":    lane,
		"state":      state,
	}
}

func cloneRows(rows []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, cloneParams(row))
	}
	return result
}

func cloneParams(row map[string]any) map[string]any {
	result := make(map[string]any, len(row))
	for key, value := range row {
		result[key] = value
	}
	return result
}
