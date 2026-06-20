package metrics

import (
	"bytes"
	"strings"
	"testing"
)

// totalNecrosis / totalApoptosis sum a snapshot's failure-mode counters across
// all label tuples (test helpers — same package, so the unexported maps are
// visible).
func totalNecrosis(s *Snapshot) int {
	n := 0
	for _, v := range s.necrosis {
		n += v
	}
	return n
}

func totalApoptosis(s *Snapshot) int {
	n := 0
	for _, v := range s.apoptosis {
		n += v
	}
	return n
}

func renderToString(t *testing.T, s *Snapshot) string {
	t.Helper()
	var buf bytes.Buffer
	if err := s.WriteText(&buf, sentinelNow); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	return buf.String()
}

// TestLivenessFoldRoutesReversiblePairOutsideNecrosis is the unit half of the
// RFC 0137 F-A6 contract: given the EXACT durable event types refreshRunLiveness
// emits (session.liveness_deadline_missed / session.liveness_recovered), the fold
// routes both to striatum_liveness_deadline_events_total and NEVER to
// striatum_necrosis_total. The behavioral half — that the real liveness refresh
// path actually produces those events from an active -> deadline_missed ->
// recovered transition — lives in package mutations as
// TestLivenessMissCanRecoverWithoutNecrosis (it needs the unexported
// refreshRunLiveness and a live DB, so it cannot import-cycle through here).
func TestLivenessFoldRoutesReversiblePairOutsideNecrosis(t *testing.T) {
	events := []LifecycleEvent{
		{EventType: "session.liveness_deadline_missed"},
		{EventType: "session.liveness_recovered"},
	}
	snap := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, Events: events})

	// Both halves of the reversible pair moved the liveness-events counter.
	if got := snap.livenessEvents[string(LivenessDeadlineMissed)]; got != 1 {
		t.Errorf("liveness deadline_missed = %d, want 1", got)
	}
	if got := snap.livenessEvents[string(LivenessDeadlineRecovered)]; got != 1 {
		t.Errorf("liveness recovered = %d, want 1", got)
	}

	// The load-bearing assertion: necrosis did NOT increment on a recoverable
	// stall. No apoptosis either — the liveness pair is outside the conservation
	// law, so the lifecycle balance (apoptosis+necrosis) stayed at zero.
	if n := totalNecrosis(snap); n != 0 {
		t.Fatalf("striatum_necrosis_total = %d after a recoverable liveness miss, want 0 (F-A6 violated)", n)
	}
	if a := totalApoptosis(snap); a != 0 {
		t.Fatalf("striatum_apoptosis_total = %d after a recoverable liveness miss, want 0", a)
	}

	// Prove it on the operator-visible wire surface too: every necrosis series is
	// zero, and no liveness reason ever appears as a necrosis label.
	body := renderToString(t, snap)
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, metricNecrosis+"{") {
			if !strings.HasSuffix(line, " 0") {
				t.Errorf("necrosis series is nonzero on a recoverable miss: %q", line)
			}
			if strings.Contains(line, "deadline_missed") || strings.Contains(line, "liveness") || strings.Contains(line, "recovered") {
				t.Errorf("a liveness reason leaked into a necrosis label: %q", line)
			}
		}
	}
	// The liveness family is present and nonzero on the wire.
	if !strings.Contains(body, metricLivenessEvents+`{reason="deadline_missed"} 1`) {
		t.Errorf("liveness deadline_missed=1 not rendered:\n%s", body)
	}
	if !strings.Contains(body, metricLivenessEvents+`{reason="recovered"} 1`) {
		t.Errorf("liveness recovered=1 not rendered:\n%s", body)
	}
}

// TestNecrosisIsCountedFromTaggedSiteEvents proves the F-A6 zero above is NOT
// vacuous: a confirmed-dead recovery close (tagged necrosis at the site) and a
// recovery_exhausted escalation DO move striatum_necrosis_total — so the
// liveness-miss test's zero genuinely means "excluded", not "never counts".
func TestNecrosisIsCountedFromTaggedSiteEvents(t *testing.T) {
	events := []LifecycleEvent{
		// Confirmed-dead session close, tagged at closeStalledOwningSession.
		{EventType: "session.closed", LifecycleTag: LifecycleTagNecrosis, StallClass: string(NecrosisAgentPIDDead)},
		{EventType: "session.closed", LifecycleTag: LifecycleTagNecrosis, StallClass: string(NecrosisAgentExitedUnsealed)},
		// Recovery-exhausted escalation, classified from its blocker_kind.
		{EventType: "run.escalated", BlockerKind: string(NecrosisRecoveryExhausted)},
		{EventType: "recovery.job_quarantined", BlockerKind: string(NecrosisRecoveryExhausted)},
	}
	snap := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, Events: events})
	if got := totalNecrosis(snap); got != 4 {
		t.Fatalf("necrosis_total = %d, want 4 (necrosis fold is broken; F-A6 zero would be vacuous)", got)
	}
	if snap.necrosis[originReason{OriginReconcileSweep, string(NecrosisRecoveryExhausted)}] != 2 {
		t.Errorf("recovery_exhausted necrosis = %d, want 2", snap.necrosis[originReason{OriginReconcileSweep, string(NecrosisRecoveryExhausted)}])
	}
}

// TestRecoveryTransferCloseIsNotApoptosis proves the tag distinguishes a clean
// session close (apoptosis) from a recovery transfer-close of an honestly-stalled
// but NOT dead lane (skipped). Without the site tag, the transfer-close would be
// miscounted as a clean apoptosis.
func TestRecoveryTransferCloseIsNotApoptosis(t *testing.T) {
	clean := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, Events: []LifecycleEvent{
		{EventType: "session.closed"}, // no tag, no stall class -> clean apoptosis
	}})
	if clean.apoptosis[originReason{OriginLane, string(ApoptosisSessionClosedClean)}] != 1 {
		t.Errorf("clean session.closed should be one session_closed_clean apoptosis, got %v", clean.apoptosis)
	}

	transfer := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, Events: []LifecycleEvent{
		{EventType: "session.closed", LifecycleTag: LifecycleTagRecoveryTransfer, StallClass: "agent_protocol_idle_stall"},
	}})
	if got := totalApoptosis(transfer); got != 0 {
		t.Errorf("recovery transfer-close counted as apoptosis (%d); the site tag should exclude it", got)
	}
	if got := totalNecrosis(transfer); got != 0 {
		t.Errorf("recovery transfer-close (not dead) counted as necrosis (%d)", got)
	}
}

// TestApoptosisClassifiedFromHealthyEventTypes pins the apoptosis side of the
// split: run.completed and job.completed are unambiguous healthy terminations.
func TestApoptosisClassifiedFromHealthyEventTypes(t *testing.T) {
	snap := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, Events: []LifecycleEvent{
		{EventType: "run.completed"},
		{EventType: "job.completed"},
		{EventType: "job.completed"},
	}})
	if snap.apoptosis[originReason{OriginDaemonCore, string(ApoptosisRunCompleted)}] != 1 {
		t.Errorf("run.completed -> apoptosis run_completed failed: %v", snap.apoptosis)
	}
	if snap.apoptosis[originReason{OriginLane, string(ApoptosisJobSucceeded)}] != 2 {
		t.Errorf("job.completed -> apoptosis job_succeeded failed: %v", snap.apoptosis)
	}
	if totalNecrosis(snap) != 0 {
		t.Errorf("healthy terminations produced necrosis: %v", snap.necrosis)
	}
}

// TestApoptosisReasonsAreAllProducedFromRealEventTypes pins the RFC 0137 Phase B
// completeness requirement that NO apoptosis reason is decorative: every value in
// the closed enum must be produced by folding the real durable event type it is
// wired to (prior-review F2 rejected lease_handoff and supervisor_drained as
// reasons the fold could never increment). It folds one event of each real type
// and then asserts the closed enum is fully covered — a future reason added
// without an emit site fails here.
func TestApoptosisReasonsAreAllProducedFromRealEventTypes(t *testing.T) {
	cases := []struct {
		name   string
		event  LifecycleEvent
		origin Origin
		reason ApoptosisReason
	}{
		{"run.completed", LifecycleEvent{EventType: "run.completed"}, OriginDaemonCore, ApoptosisRunCompleted},
		{"job.completed", LifecycleEvent{EventType: "job.completed"}, OriginLane, ApoptosisJobSucceeded},
		{"session.closed (clean)", LifecycleEvent{EventType: "session.closed"}, OriginLane, ApoptosisSessionClosedClean},
		{"lease.released (handoff)", LifecycleEvent{EventType: "lease.released", LeaseTransfer: true}, OriginLane, ApoptosisLeaseHandoff},
		{"supervisor.stopped", LifecycleEvent{EventType: "supervisor.stopped"}, OriginSupervisor, ApoptosisSupervisorDrained},
	}
	covered := map[ApoptosisReason]bool{}
	for _, tc := range cases {
		snap := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, Events: []LifecycleEvent{tc.event}})
		if got := snap.apoptosis[originReason{tc.origin, string(tc.reason)}]; got != 1 {
			t.Errorf("%s: apoptosis{%s,%s} = %d, want 1 (reason is hollow — its real event type does not produce it)",
				tc.name, tc.origin, tc.reason, got)
		}
		covered[tc.reason] = true
	}
	for _, r := range ApoptosisReasons() {
		if !covered[r] {
			t.Errorf("apoptosis reason %q is in the closed enum but no real event type produces it (hollow)", r)
		}
	}
}

// TestLeaseReleaseHandoffExcludesCompletionReleases proves lease_handoff counts
// only genuine handoffs (transfer / supersession), not the far more common
// completion or blocked releases — so the reason stays a healthy-handoff signal
// rather than a generic lease.released tally.
func TestLeaseReleaseHandoffExcludesCompletionReleases(t *testing.T) {
	handoff := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, Events: []LifecycleEvent{
		{EventType: "lease.released", LeaseReason: "superseded"},
		{EventType: "lease.released", LeaseTransfer: true, LeaseReason: "operator_transfer"},
	}})
	if got := handoff.apoptosis[originReason{OriginLane, string(ApoptosisLeaseHandoff)}]; got != 2 {
		t.Errorf("lease_handoff = %d, want 2 (supersession + transfer)", got)
	}
	notHandoff := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, Events: []LifecycleEvent{
		{EventType: "lease.released", LeaseReason: "completed"},
		{EventType: "lease.released", LeaseReason: "blocked"},
	}})
	if got := totalApoptosis(notHandoff); got != 0 {
		t.Errorf("completion/blocked lease.released counted as handoff apoptosis (%d); only transfers/supersessions are handoffs", got)
	}
}

// TestJobStateAdvanceSetExcludesNonJobEvents is the unit regression for
// prior-review F1: the wedge-age evidence set must EXCLUDE daemon.recovery_sweep
// (which fires every sweep tick on a wedged run) and every other non-job event,
// and include the real job-state transitions. It pins the exact bug — a
// recovery-sweep tick counting as "progress" — at the source-of-truth set the SQL
// filter and the in-process predicate share.
func TestJobStateAdvanceSetExcludesNonJobEvents(t *testing.T) {
	for _, nonJob := range []string{
		"daemon.recovery_sweep", "lease.heartbeat", "run.completed",
		"session.liveness_recovered", "supervisor.stopped", "lease.released",
	} {
		if isJobStateAdvanceEvent(nonJob) {
			t.Errorf("%q must NOT count as a job-state advance (it can fire while jobs stay stuck)", nonJob)
		}
	}
	for _, jobEv := range []string{"job.created", "job.queued", "job.completed", "job.failed", "job.retried"} {
		if !isJobStateAdvanceEvent(jobEv) {
			t.Errorf("%q must count as a job-state advance", jobEv)
		}
	}
}

// TestLeaseTransitionTargetDistinguishesStaleLease is the unit regression for
// prior-review F1: the lease_transitions fold must render a repo-write stale-lease
// expiry (the RFC 0137 primary stale-lease storm signal) as a DISTINCT
// to="stale_lease" series rather than collapsing it into ordinary to="expired".
// It pins the (to, reason) derivation at the source-of-truth helper the collector
// SQL feeds, then proves on the rendered wire surface that the two expiries stay
// distinct. The behavioral half — that the real recovery expiry path stamps
// job_state="stale_lease" on the durable event the fold reads — lives in package
// mutations as TestStaleLeaseExpiryRendersDistinctTransition (it needs a live DB).
func TestLeaseTransitionTargetDistinguishesStaleLease(t *testing.T) {
	cases := []struct {
		name       string
		eventType  string
		reason     string
		jobState   string
		wantTo     string
		wantReason string
	}{
		{"released keeps its payload reason", "lease.released", "operator_transfer", "", "released", "operator_transfer"},
		{"repo-write expiry parks in stale_lease", "lease.expired", "", "stale_lease", "stale_lease", "expired"},
		{"ordinary expiry re-queues the job", "lease.expired", "", "queued", "expired", "expired"},
		{"expiry with no job_state is plain expiry", "lease.expired", "", "", "expired", "expired"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			to, reason := leaseTransitionTarget(tc.eventType, tc.reason, tc.jobState)
			if to != tc.wantTo {
				t.Errorf("to = %q, want %q", to, tc.wantTo)
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}

	// End-to-end through Build + render: a stale-lease expiry and an ordinary
	// expiry (both with the lease.expired raw reason "expired") must produce two
	// DISTINCT series — to="stale_lease" and to="expired" — both with the bucketed
	// reason "expiry", never collapsed onto one line and never falling into the
	// "other" reason bucket.
	staleTo, staleReason := leaseTransitionTarget("lease.expired", "", "stale_lease")
	plainTo, plainReason := leaseTransitionTarget("lease.expired", "", "queued")
	snap := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, LeaseTransitionCounts: []LeaseTransitionCount{
		{Transition: LeaseTransition{From: "active", To: staleTo, Reason: staleReason}, Count: 1},
		{Transition: LeaseTransition{From: "active", To: plainTo, Reason: plainReason}, Count: 1},
	}})
	body := renderToString(t, snap)
	if !strings.Contains(body, `striatum_lease_transitions_total{from="active",to="stale_lease",reason="expiry"} 1`) {
		t.Errorf("stale-lease expiry did not render a distinct to=\"stale_lease\" series:\n%s", body)
	}
	if !strings.Contains(body, `striatum_lease_transitions_total{from="active",to="expired",reason="expiry"} 1`) {
		t.Errorf("ordinary expiry did not render to=\"expired\":\n%s", body)
	}
	if strings.Contains(body, `to="expired",reason="other"`) {
		t.Errorf("expiry fell into the generic \"other\" reason bucket:\n%s", body)
	}
}

// TestLeaseReasonBucketingIsBounded confirms unknown lease reasons collapse to
// the reserved "other" bucket so the reason label cannot grow cardinality.
func TestLeaseReasonBucketingIsBounded(t *testing.T) {
	if got := bucketLeaseReason("some_unforeseen_future_reason"); got != leaseReasonOther {
		t.Errorf("unknown reason bucketed to %q, want %q", got, leaseReasonOther)
	}
	if got := bucketLeaseReason("recovery_transfer"); got != "recovery_transfer" {
		t.Errorf("recovery_transfer bucketed to %q", got)
	}
	if got := bucketLeaseState("some_unknown_state"); got != leaseStateOther {
		t.Errorf("unknown lease state bucketed to %q, want %q", got, leaseStateOther)
	}
}
