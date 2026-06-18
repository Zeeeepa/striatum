package mutations

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
	"github.com/jackc/pgx/v5"
)

// confidenceGateTx is a minimal db.TxRunner fake that captures Exec SQL so the
// RFC 0131 131-C confidence-gate decision logic can be unit-tested without a real
// PostgreSQL. applyConfidenceGate only Exec's the gate-state upsert (it reads its
// inputs from the already-loaded budget struct), so capturing Exec is sufficient.
type confidenceGateTx struct {
	execs []string
}

func (tx *confidenceGateTx) Exec(_ context.Context, sql string, _ ...any) error {
	tx.execs = append(tx.execs, sql)
	return nil
}
func (tx *confidenceGateTx) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: pgx.ErrNoRows}
}
func (tx *confidenceGateTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}
func (tx *confidenceGateTx) Commit(context.Context) error   { return nil }
func (tx *confidenceGateTx) Rollback(context.Context) error { return nil }

func (tx *confidenceGateTx) wroteGateState() bool {
	for _, sql := range tx.execs {
		if strings.Contains(sql, "consecutive_silent_sweeps") && strings.Contains(sql, "misfire_evidence_score") {
			return true
		}
	}
	return false
}

// TestSilentSweepCapDefault asserts the RFC 0131 Layer 4 escape-valve cap default
// (maxRequeues*2)+3, the +3 floor when requeues are disabled, and an operator
// override.
func TestSilentSweepCapDefault(t *testing.T) {
	cases := []struct {
		name   string
		policy recoveryPolicy
		want   int
	}{
		{"default max_requeues=2", recoveryPolicy{maxRequeues: 2}, 7},     // (2*2)+3
		{"max_requeues=0 floors at 3", recoveryPolicy{maxRequeues: 0}, 3}, // floor
		{"operator override wins", recoveryPolicy{maxRequeues: 2, maxSilentSweeps: 4}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.policy.silentSweepCap(); got != tc.want {
				t.Fatalf("silentSweepCap() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestTopologyAdaptiveSilentSweepCap asserts the RFC 0131 #349 topology-adaptive
// escape-valve cap: a critical-path job whose downstream is blocked escalates at
// the tight single-cap FLOOR (no regression vs the pre-#349 single cap), while a
// leaf job whose silence wedges nothing downstream gets a loosened (doubled) cap —
// yet the cap stays finite in both cases, so the never-un-escalatable invariant
// (Goal 3) holds for every topology. An operator max_silent_sweeps override is the
// floor for the critical-path case and is still loosened for a leaf.
func TestTopologyAdaptiveSilentSweepCap(t *testing.T) {
	cases := []struct {
		name              string
		policy            recoveryPolicy
		downstreamBlocked bool
		want              int
	}{
		// Default cap floor = (2*2)+3 = 7.
		{"critical-path job escalates at the single-cap floor", recoveryPolicy{maxRequeues: 2}, true, 7},
		{"leaf job gets the loosened (doubled) cap", recoveryPolicy{maxRequeues: 2}, false, 14},
		// Requeues disabled: floor = 3.
		{"critical-path at the +3 floor", recoveryPolicy{maxRequeues: 0}, true, 3},
		{"leaf doubles the +3 floor", recoveryPolicy{maxRequeues: 0}, false, 6},
		// Operator override floor.
		{"critical-path honors the operator override floor", recoveryPolicy{maxRequeues: 2, maxSilentSweeps: 4}, true, 4},
		{"leaf doubles the operator override", recoveryPolicy{maxRequeues: 2, maxSilentSweeps: 4}, false, 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.policy.topologyAdaptiveSilentSweepCap(tc.downstreamBlocked)
			if got != tc.want {
				t.Fatalf("topologyAdaptiveSilentSweepCap(%v) = %d, want %d", tc.downstreamBlocked, got, tc.want)
			}
			// The single cap is always the FLOOR — no topology ever escalates sooner
			// than the pre-#349 single cap.
			if got < tc.policy.silentSweepCap() {
				t.Fatalf("adaptive cap %d undercut the single-cap floor %d", got, tc.policy.silentSweepCap())
			}
		})
	}
}

// TestApplyConfidenceGate exercises the RFC 0131 Layer 3/4 gate decision matrix
// directly: a single deadline_elapsed_only sweep never escalates a pipe lane;
// forgery-resistant progress resets and defers; the second consecutive silent
// sweep escalates; the Layer-4 cap forces escalation regardless of confidence; a
// deployment behind on migration 0035 degrades to ungated escalation.
func TestApplyConfidenceGate(t *testing.T) {
	ctx := context.Background()
	policy := recoveryPolicy{maxRequeues: 2} // cap = 7
	deadlineBasis := string(sessionliveness.ProbeBasisDeadlineElapsedOnly)

	t.Run("first silent sweep debounces (single sweep never escalates a pipe lane)", func(t *testing.T) {
		tx := &confidenceGateTx{}
		budget := jobRecoveryBudget{confidenceColumns: true} // no prior basis, 0 sweeps
		got, err := applyConfidenceGate(ctx, tx, "repo", "run", "job", budget, policy, false, 2, policy.silentSweepCap())
		if err != nil {
			t.Fatalf("applyConfidenceGate: %v", err)
		}
		if got.escalate {
			t.Fatalf("first silent sweep must NOT escalate; got %#v", got)
		}
		if got.silentSweeps != 1 || got.misfireScore != 1 {
			t.Fatalf("counters = silent:%d misfire:%d, want 1/1", got.silentSweeps, got.misfireScore)
		}
		if !tx.wroteGateState() {
			t.Fatalf("gate must persist its state on a debounce")
		}
	})

	t.Run("forgery-resistant progress resets and defers", func(t *testing.T) {
		tx := &confidenceGateTx{}
		// A lane that had accrued silent sweeps but just sealed real progress.
		budget := jobRecoveryBudget{
			confidenceColumns:       true,
			consecutiveSilentSweeps: 5,
			misfireEvidenceScore:    5,
			lastProbeBasis:          deadlineBasis,
		}
		got, err := applyConfidenceGate(ctx, tx, "repo", "run", "job", budget, policy, true /* progress advanced */, 2, policy.silentSweepCap())
		if err != nil {
			t.Fatalf("applyConfidenceGate: %v", err)
		}
		if got.escalate {
			t.Fatalf("progress-advanced sweep must NOT escalate; got %#v", got)
		}
		if !got.reset || got.silentSweeps != 0 || got.misfireScore != 0 {
			t.Fatalf("progress must reset counters to 0; got %#v", got)
		}
	})

	t.Run("second consecutive silent sweep escalates (two-sweep bar)", func(t *testing.T) {
		tx := &confidenceGateTx{}
		// The prior sweep was already deadline_elapsed_only (1 silent sweep recorded).
		budget := jobRecoveryBudget{
			confidenceColumns:       true,
			consecutiveSilentSweeps: 1,
			misfireEvidenceScore:    1,
			lastProbeBasis:          deadlineBasis,
		}
		got, err := applyConfidenceGate(ctx, tx, "repo", "run", "job", budget, policy, false, 2, policy.silentSweepCap())
		if err != nil {
			t.Fatalf("applyConfidenceGate: %v", err)
		}
		if !got.escalate {
			t.Fatalf("second consecutive silent sweep must escalate; got %#v", got)
		}
		if got.capFired {
			t.Fatalf("the two-sweep bar must NOT be reported as a cap firing; got %#v", got)
		}
		if got.silentSweeps != 2 {
			t.Fatalf("silentSweeps = %d, want 2", got.silentSweeps)
		}
	})

	t.Run("escape-valve cap forces escalation regardless of confidence", func(t *testing.T) {
		tx := &confidenceGateTx{}
		// One short of the cap (7), prior basis deadline_elapsed_only: the increment
		// hits the cap. A high misfire score must NOT defer it (silence is the oracle).
		budget := jobRecoveryBudget{
			confidenceColumns:       true,
			consecutiveSilentSweeps: 6,
			misfireEvidenceScore:    999,
			lastProbeBasis:          deadlineBasis,
		}
		got, err := applyConfidenceGate(ctx, tx, "repo", "run", "job", budget, policy, false, 2, policy.silentSweepCap())
		if err != nil {
			t.Fatalf("applyConfidenceGate: %v", err)
		}
		if !got.escalate || !got.capFired {
			t.Fatalf("at the cap, escalation must fire via the escape valve; got %#v", got)
		}
		if got.silentSweeps != 7 || got.silentCap != 7 {
			t.Fatalf("cap fields = silent:%d cap:%d, want 7/7", got.silentSweeps, got.silentCap)
		}
	})

	t.Run("deployment behind on migration 0035 degrades to ungated escalation", func(t *testing.T) {
		tx := &confidenceGateTx{}
		budget := jobRecoveryBudget{confidenceColumns: false} // 0035 not applied
		got, err := applyConfidenceGate(ctx, tx, "repo", "run", "job", budget, policy, false, 2, policy.silentSweepCap())
		if err != nil {
			t.Fatalf("applyConfidenceGate: %v", err)
		}
		if !got.escalate {
			t.Fatalf("a deployment behind on 0035 must escalate immediately (degrade-safe); got %#v", got)
		}
		if tx.wroteGateState() {
			t.Fatalf("the degrade-safe path must NOT attempt to persist gate state it cannot store")
		}
	})

	t.Run("never un-escalatable: a forging chatter-only lane still hits the cap", func(t *testing.T) {
		// Simulate the #324 spinner class: it never makes forgery-resistant progress,
		// so progressAdvanced is always false. Drive consecutive sweeps and assert it
		// escalates within the cap by construction.
		budget := jobRecoveryBudget{confidenceColumns: true}
		silentCap := policy.silentSweepCap()
		escalatedAt := 0
		for sweep := 1; sweep <= silentCap+2; sweep++ {
			tx := &confidenceGateTx{}
			got, err := applyConfidenceGate(ctx, tx, "repo", "run", "job", budget, policy, false, 2, policy.silentSweepCap())
			if err != nil {
				t.Fatalf("sweep %d: %v", sweep, err)
			}
			// Feed the post-update state back as the next sweep's prior state.
			budget.consecutiveSilentSweeps = got.silentSweeps
			budget.misfireEvidenceScore = got.misfireScore
			budget.lastProbeBasis = string(sessionliveness.ProbeBasisDeadlineElapsedOnly)
			if got.escalate && escalatedAt == 0 {
				escalatedAt = sweep
				break
			}
		}
		if escalatedAt == 0 {
			t.Fatalf("a forging chatter-only pipe lane must be escalatable in bounded sweeps (cap %d)", silentCap)
		}
		// The two-sweep bar fires before the cap, so the chatter-only lane escalates
		// at sweep 2 (it has no oracle and no progress, the conservative escalation).
		if escalatedAt > silentCap {
			t.Fatalf("escalated at sweep %d, must be <= cap %d (never un-escalatable)", escalatedAt, silentCap)
		}
	})
}

// TestWindowStartForGate asserts the "since the last sweep" boundary: the prior
// last_recovery_at when present, else a bounded look-back so a fresh row still
// credits very-recent sealed progress.
func TestWindowStartForGate(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	prior := now.Add(-2 * time.Minute)

	if got := windowStartForGate(jobRecoveryBudget{lastRecoveryAt: prior}, now); !got.Equal(prior) {
		t.Fatalf("with a prior sweep, window start = %v, want %v", got, prior)
	}
	// Fresh row (zero lastRecoveryAt): a bounded look-back before now.
	got := windowStartForGate(jobRecoveryBudget{}, now)
	if !got.Before(now) {
		t.Fatalf("fresh-row window start %v must be before now %v", got, now)
	}
	lookback := now.Sub(got)
	if lookback <= 0 || lookback > 1*time.Hour {
		t.Fatalf("fresh-row look-back = %v, want a small bounded positive window", lookback)
	}
}

// TestCohortHasFresherLiveness asserts cross-lane corroboration: a sibling with
// fresh activity corroborates (defer), the self job is excluded, and a uniformly
// silent cohort does not corroborate (escalation must still be possible).
func TestCohortHasFresherLiveness(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-1 * time.Minute)

	rows := []map[string]any{
		{"job_id": "self", sessionliveness.LastMCPRequestAt: now.Add(-5 * time.Minute)},         // self: stale + excluded
		{"job_id": "sibling_live", sessionliveness.LastMCPRequestAt: now.Add(-5 * time.Second)}, // fresh sibling
	}
	if !cohortHasFresherLiveness(rows, "self", windowStart, now) {
		t.Fatalf("a live sibling must corroborate")
	}

	// Exclude self only; no live sibling => no corroboration.
	silent := []map[string]any{
		{"job_id": "self", sessionliveness.LastMCPRequestAt: now.Add(-5 * time.Second)}, // fresh BUT it is self
		{"job_id": "sibling_silent", sessionliveness.LastMCPRequestAt: now.Add(-10 * time.Minute)},
	}
	if cohortHasFresherLiveness(silent, "self", windowStart, now) {
		t.Fatalf("self must be excluded and a silent sibling must NOT corroborate")
	}
}
