package runreconcile

import (
	"reflect"
	"testing"
)

func job(workflowJobID, role, lane, state string, attempt int) map[string]any {
	m := map[string]any{
		"workflow_job_id": workflowJobID,
		"role_id":         role,
		"state":           state,
		"attempt":         attempt,
	}
	if lane != "" {
		m["lane_selector_json"] = map[string]any{"lane_id": lane}
	}
	return m
}

func session(sessionID, role, lane, state string) map[string]any {
	return map[string]any{
		"session_id": sessionID,
		"role_id":    role,
		"lane_id":    lane,
		"state":      state,
	}
}

// kinds projects a decision slice down to (slot, kind, session) tuples so the
// parity assertions read clearly.
type decisionShape struct {
	WorkflowJobID string
	Attempt       int
	Kind          LaunchKind
	Role          string
	Lane          string
	SessionID     string
}

func shapes(decisions []LaunchDecision) []decisionShape {
	out := make([]decisionShape, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, decisionShape{
			WorkflowJobID: d.Slot.WorkflowJobID,
			Attempt:       d.Slot.Attempt,
			Kind:          d.Kind,
			Role:          d.Role,
			Lane:          d.Lane,
			SessionID:     d.SessionID,
		})
	}
	return out
}

// TestReconcilePredicateParityRegistersFreshForQueued: queued jobs with no
// adoptable sessions each become a LaunchRegisterFresh decision — the spawn the
// driver and the daemon scheduler must make identically (C3).
func TestReconcilePredicateParityRegistersFreshForQueued(t *testing.T) {
	jobs := []map[string]any{
		job("author_draft", "author", "codex", "queued", 1),
		job("review", "reviewer", "agy", "queued", 1),
	}
	got := shapes(PlanLaunch(jobs, nil, nil, nil))
	want := []decisionShape{
		{WorkflowJobID: "author_draft", Attempt: 1, Kind: LaunchRegisterFresh, Role: "author", Lane: "codex"},
		{WorkflowJobID: "review", Attempt: 1, Kind: LaunchRegisterFresh, Role: "reviewer", Lane: "agy"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

// TestReconcilePredicateParityAdoptsExistingSession: an unused active session for
// the role+lane slot is adopted rather than re-registered.
func TestReconcilePredicateParityAdoptsExistingSession(t *testing.T) {
	jobs := []map[string]any{job("author_draft", "author", "codex", "queued", 1)}
	sessions := []map[string]any{session("sess_existing", "author", "codex", "active")}
	got := shapes(PlanLaunch(jobs, nil, sessions, nil))
	want := []decisionShape{
		{WorkflowJobID: "author_draft", Attempt: 1, Kind: LaunchAdoptExisting, Role: "author", Lane: "codex", SessionID: "sess_existing"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

// TestReconcilePredicateParitySkipsAlreadyLaunchedSlot: a slot already launched
// (present in the launched map) yields no decision — the idempotent re-derive
// both homes rely on for restart safety (C4).
func TestReconcilePredicateParitySkipsAlreadyLaunchedSlot(t *testing.T) {
	jobs := []map[string]any{job("author_draft", "author", "codex", "queued", 1)}
	launched := map[SlotKey]string{{WorkflowJobID: "author_draft", Attempt: 1}: "sess_1"}
	if got := PlanLaunch(jobs, nil, nil, launched); len(got) != 0 {
		t.Fatalf("decisions = %#v, want none for an already-launched slot", got)
	}
}

// TestReconcilePredicateParityAmbiguousLane: a queued job whose lane cannot be
// resolved is flagged ambiguous, never auto-spawned.
func TestReconcilePredicateParityAmbiguousLane(t *testing.T) {
	jobs := []map[string]any{job("mystery", "author", "", "queued", 1)}
	got := shapes(PlanLaunch(jobs, map[string]any{"jobs": []any{}}, nil, nil))
	want := []decisionShape{
		{WorkflowJobID: "mystery", Attempt: 1, Kind: LaunchAmbiguousLane, Role: "author"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

// TestReconcilePredicateParitySharedRoleLaneSplitsAdoptAndRegister: two queued
// jobs sharing a role+lane with a single active session — the first adopts it,
// the second registers fresh. This pins the in-pass used-session bookkeeping that
// keeps two slots from adopting the same session.
func TestReconcilePredicateParitySharedRoleLaneSplitsAdoptAndRegister(t *testing.T) {
	jobs := []map[string]any{
		job("a1", "author", "codex", "queued", 1),
		job("a2", "author", "codex", "queued", 1),
	}
	sessions := []map[string]any{session("sess_one", "author", "codex", "active")}
	got := shapes(PlanLaunch(jobs, nil, sessions, nil))
	want := []decisionShape{
		{WorkflowJobID: "a1", Attempt: 1, Kind: LaunchAdoptExisting, Role: "author", Lane: "codex", SessionID: "sess_one"},
		{WorkflowJobID: "a2", Attempt: 1, Kind: LaunchRegisterFresh, Role: "author", Lane: "codex"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

// TestReconcilePredicateParityLaunchedSessionExcludedFromAdoption: a session that
// already serves a launched slot is never adopted by another queued job.
func TestReconcilePredicateParityLaunchedSessionExcludedFromAdoption(t *testing.T) {
	jobs := []map[string]any{job("a2", "author", "codex", "queued", 1)}
	sessions := []map[string]any{session("sess_used", "author", "codex", "active")}
	launched := map[SlotKey]string{{WorkflowJobID: "a1", Attempt: 1}: "sess_used"}
	got := shapes(PlanLaunch(jobs, nil, sessions, launched))
	want := []decisionShape{
		{WorkflowJobID: "a2", Attempt: 1, Kind: LaunchRegisterFresh, Role: "author", Lane: "codex"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

// TestReconcilePredicateParityResolvesLaneFromWorkflowSnapshot: when the job row
// carries no lane, the lane is resolved from the workflow snapshot's jobs list.
func TestReconcilePredicateParityResolvesLaneFromWorkflowSnapshot(t *testing.T) {
	jobs := []map[string]any{job("build", "author", "", "queued", 1)}
	workflow := map[string]any{"jobs": []any{
		map[string]any{"id": "build", "lane_id": "claude"},
	}}
	got := shapes(PlanLaunch(jobs, workflow, nil, nil))
	want := []decisionShape{
		{WorkflowJobID: "build", Attempt: 1, Kind: LaunchRegisterFresh, Role: "author", Lane: "claude"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

// TestReconcilePredicateParityOnlyQueuedJobsDecide: non-queued jobs are inert.
func TestReconcilePredicateParityOnlyQueuedJobsDecide(t *testing.T) {
	jobs := []map[string]any{
		job("done", "author", "codex", "completed", 1),
		job("running", "author", "codex", "claimed", 1),
		job("next", "reviewer", "agy", "queued", 1),
	}
	got := shapes(PlanLaunch(jobs, nil, nil, nil))
	want := []decisionShape{
		{WorkflowJobID: "next", Attempt: 1, Kind: LaunchRegisterFresh, Role: "reviewer", Lane: "agy"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

func TestSortedJobsOrdersByWorkflowJobIDThenAttempt(t *testing.T) {
	jobs := []map[string]any{
		job("b", "r", "l", "queued", 1),
		job("a", "r", "l", "queued", 2),
		job("a", "r", "l", "queued", 1),
	}
	sorted := SortedJobs(jobs)
	var order []string
	for _, j := range sorted {
		order = append(order, StringValue(j["workflow_job_id"])+":"+StringValue(j["attempt"]))
	}
	want := []string{"a:1", "a:2", "b:1"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %#v, want %#v", order, want)
	}
}

func TestIsTerminalRunState(t *testing.T) {
	for _, state := range []string{"completed", "failed", "canceled", "needs_operator", "waiting_human"} {
		if !IsTerminalRunState(state) {
			t.Fatalf("IsTerminalRunState(%q) = false, want true", state)
		}
	}
	for _, state := range []string{"running", "ready", "blocked", ""} {
		if IsTerminalRunState(state) {
			t.Fatalf("IsTerminalRunState(%q) = true, want false", state)
		}
	}
}

func TestShouldCloseLaunchedBeforeTerminal(t *testing.T) {
	for _, state := range []string{"needs_operator", "waiting_human"} {
		if !ShouldCloseLaunchedBeforeTerminal(state) {
			t.Fatalf("ShouldCloseLaunchedBeforeTerminal(%q) = false, want true", state)
		}
	}
	for _, state := range []string{"completed", "failed", "canceled", "running"} {
		if ShouldCloseLaunchedBeforeTerminal(state) {
			t.Fatalf("ShouldCloseLaunchedBeforeTerminal(%q) = true, want false", state)
		}
	}
}

func TestIsLaunchedJobDone(t *testing.T) {
	for _, state := range []string{"completed", "failed", "canceled", "skipped", "waiting_human"} {
		if !IsLaunchedJobDone(state) {
			t.Fatalf("IsLaunchedJobDone(%q) = false, want true", state)
		}
	}
	for _, state := range []string{"queued", "claimed", "running"} {
		if IsLaunchedJobDone(state) {
			t.Fatalf("IsLaunchedJobDone(%q) = true, want false", state)
		}
	}
}

func TestIsPausedRun(t *testing.T) {
	if IsPausedRun(map[string]any{}) {
		t.Fatal("absent paused_at should not be paused")
	}
	if IsPausedRun(map[string]any{"paused_at": nil}) {
		t.Fatal("nil paused_at should not be paused")
	}
	if IsPausedRun(map[string]any{"paused_at": ""}) {
		t.Fatal("empty paused_at should not be paused")
	}
	if !IsPausedRun(map[string]any{"paused_at": "2026-06-14T00:00:00Z"}) {
		t.Fatal("set paused_at should be paused")
	}
	if !IsPausedRun(map[string]any{"paused_at": true}) {
		t.Fatal("non-string non-nil paused_at should be paused")
	}
}

func TestActiveSessionsBySlotAndIDs(t *testing.T) {
	sessions := []map[string]any{
		session("s1", "author", "codex", "active"),
		session("s2", "author", "codex", "active"),
		session("s3", "reviewer", "agy", "closed"),
		session("s4", "", "codex", "active"), // incomplete slot — skipped
	}
	bySlot := ActiveSessionsBySlot(sessions)
	if got := bySlot[RoleLane{Role: "author", Lane: "codex"}]; !reflect.DeepEqual(got, []string{"s1", "s2"}) {
		t.Fatalf("author/codex sessions = %#v, want [s1 s2]", got)
	}
	if _, ok := bySlot[RoleLane{Role: "reviewer", Lane: "agy"}]; ok {
		t.Fatal("closed session should not appear in active slot map")
	}
	ids := ActiveSessionIDs(sessions)
	if !ids["s1"] || !ids["s2"] || ids["s3"] || !ids["s4"] {
		t.Fatalf("active ids = %#v", ids)
	}
}

func TestNextUnusedSession(t *testing.T) {
	used := map[string]bool{"s1": true}
	if got := NextUnusedSession([]string{"s1", "s2"}, used); got != "s2" {
		t.Fatalf("NextUnusedSession = %q, want s2", got)
	}
	if got := NextUnusedSession([]string{"s1"}, used); got != "" {
		t.Fatalf("NextUnusedSession = %q, want empty", got)
	}
}

func TestResolveLanePrefersJobRow(t *testing.T) {
	j := map[string]any{"workflow_job_id": "x", "lane_id": "direct"}
	if lane, ok := ResolveLane(j, nil); !ok || lane != "direct" {
		t.Fatalf("ResolveLane = %q,%v, want direct,true", lane, ok)
	}
	j2 := map[string]any{"workflow_job_id": "x"}
	if _, ok := ResolveLane(j2, map[string]any{"jobs": []any{}}); ok {
		t.Fatal("ResolveLane should be ambiguous with no lane anywhere")
	}
}

func TestJobSlot(t *testing.T) {
	got := JobSlot(map[string]any{"workflow_job_id": "x", "attempt": 3})
	if got != (SlotKey{WorkflowJobID: "x", Attempt: 3}) {
		t.Fatalf("JobSlot = %#v", got)
	}
}

func TestIntValueCoercions(t *testing.T) {
	cases := map[any]int{int(1): 1, int32(2): 2, int64(3): 3, float64(4): 4, "5": 5, "x": 0, nil: 0}
	for in, want := range cases {
		if got := IntValue(in); got != want {
			t.Fatalf("IntValue(%v) = %d, want %d", in, got, want)
		}
	}
}
