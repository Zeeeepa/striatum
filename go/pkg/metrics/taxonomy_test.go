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

// TestLivenessMissCanRecoverWithoutNecrosis is the RFC 0137 F-A6 contract: it
// drives a session through active -> liveness_deadline_missed ->
// liveness_recovered (the EXACT durable event types refreshRunLiveness emits at
// recovery.go:1229 / :1244) and asserts the reversible miss moved
// striatum_liveness_deadline_events_total but NEVER striatum_necrosis_total. A
// recoverable stall is a pre-death observation, not death — so it must stay out
// of the apoptosis/necrosis conservation law entirely.
func TestLivenessMissCanRecoverWithoutNecrosis(t *testing.T) {
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
