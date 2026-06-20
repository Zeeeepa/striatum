package metrics

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// goldenPath is the committed byte-for-byte exposition the redaction test pins.
const goldenPath = "testdata/metrics_golden.txt"

// sentinelBuiltAt / sentinelNow fix the snapshot age so the rendered body is
// deterministic (age == 12.000000) and the golden stays byte-stable.
var (
	sentinelBuiltAt = time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	sentinelNow     = sentinelBuiltAt.Add(12 * time.Second)
)

// sentinelLiterals are distinctive values planted in the snapshot input. None
// may appear anywhere in the rendered /metrics body.
var sentinelLiterals = []string{
	"/home/halbritt/git/secret-target-repo",
	"feature/leak-sentinel-branch",
	"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	"--dangerous-prompt=LEAK_SENTINEL_ARGV",
	"author: leaker-model-007",
	"/var/lib/striatum/exotic-leak-path",
	"LEAK_SENTINEL",
	"leaker-model",
	"repo_LEAK_SENTINEL_raw_repository_id",
}

// forbiddenShapes catch a leaked *value* under an already-allowed label name —
// the case the golden hash alone cannot see (it only catches changed label
// names). Each shape mirrors a class the exporter must never emit: a 40-hex git
// sha, a filesystem path, a branch/path slash form, an argv `--flag=value`
// fragment, and an `author:` byline.
var forbiddenShapes = []*regexp.Regexp{
	regexp.MustCompile(`[0-9a-f]{40}`),                   // git object sha
	regexp.MustCompile(`(?:/[A-Za-z0-9._-]+){2,}`),       // filesystem path
	regexp.MustCompile(`[A-Za-z0-9_]+/[A-Za-z0-9._/-]+`), // branch / path slash shape
	regexp.MustCompile(`--[A-Za-z][A-Za-z0-9-]*=`),       // argv flag=value
	regexp.MustCompile(`\bauthor:`),                      // role byline
}

// sentinelRuns plants every sensitive provenance class onto run observations
// whose only wire-eligible field is State. Aggregate counts: running=2,
// blocked=1, completed=4, needs_operator=1, exotic->other=1; stranded=3.
func sentinelRuns() []RunObservation {
	return []RunObservation{
		{State: "running"},
		{State: "running", RepoPath: "/home/halbritt/git/secret-target-repo", Branch: "feature/leak-sentinel-branch"},
		{State: "blocked", HeadSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", PromptFragment: "--dangerous-prompt=LEAK_SENTINEL_ARGV"},
		{State: "completed"},
		{State: "completed"},
		{State: "completed"},
		{State: "completed", AuthorByline: "author: leaker-model-007"},
		{State: "needs_operator"},
		{State: "exotic_unmapped_state", RepoPath: "/var/lib/striatum/exotic-leak-path"},
	}
}

// sentinelEvents exercises every Phase B failure-mode family in the golden: the
// apoptosis/necrosis split, the F-A6 liveness pair, and the recovery-transfer
// close the tag must exclude from both counters. The event fields are all
// closed-enum classification tags — there is nothing sensitive to leak — but
// they prove the families render deterministically with no forbidden content.
func sentinelEvents() []LifecycleEvent {
	return []LifecycleEvent{
		{EventType: "run.completed"},
		{EventType: "job.completed"},
		{EventType: "job.completed"},
		{EventType: "session.closed"}, // clean -> apoptosis session_closed_clean
		{EventType: "session.closed", LifecycleTag: LifecycleTagNecrosis, StallClass: string(NecrosisAgentPIDDead)},
		{EventType: "session.closed", LifecycleTag: LifecycleTagRecoveryTransfer, StallClass: "agent_protocol_idle_stall"}, // skipped
		{EventType: "run.escalated", BlockerKind: string(NecrosisRecoveryExhausted)},
		{EventType: "session.liveness_deadline_missed"},
		{EventType: "session.liveness_recovered"},
		// Handoff release -> apoptosis lease_handoff (origin lane); a plain
		// completion release is NOT a handoff and renders no series.
		{EventType: "lease.released", LeaseTransfer: true, LeaseReason: "operator_transfer"},
		{EventType: "lease.released", LeaseReason: "completed"},
		// Supervisor drain/stop -> apoptosis supervisor_drained (origin supervisor).
		{EventType: "supervisor.stopped"},
		// OQ2 lifecycle-balance: a session.closed that DECLARES a death (necrosis
		// tag) but carries a stall_class outside the closed necrosis domain — a
		// confirmed-dead transition the fold can account for in neither counter, so
		// it increments striatum_lifecycle_balance (the "second doctor").
		{EventType: "session.closed", LifecycleTag: LifecycleTagNecrosis, StallClass: "unknown_bogus_stall_class"},
	}
}

// sentinelRepoMetrics plants the per-repo Phase D families. The raw repository_id
// is a sentinel that must NEVER reach the wire (only the salted bucket may); one
// repo consents (so its repo_runs series render) and one does not (so it emits
// only metrics_repo_consent{...}=0 and no provenance series), proving the consent
// gate at fold time and that the surrogate, not the id, is what is exposed.
func sentinelRepoMetrics() []RepoMetric {
	return []RepoMetric{
		{
			RepoID:    "repo_LEAK_SENTINEL_raw_repository_id",
			Bucket:    "7",
			Consented: true,
			RunStates: map[string]int{"running": 2, "completed": 5, "exotic_unmapped_state": 1},
		},
		{
			RepoID:    "repo_unconsented_sentinel",
			Bucket:    "19",
			Consented: false,
			RunStates: map[string]int{"running": 3},
		},
	}
}

func sentinelLeaseTransitions() []LeaseTransition {
	return []LeaseTransition{
		{From: "active", To: "released", Reason: "completed"},
		{From: "active", To: "released", Reason: "recovery_transfer"},
		{From: "active", To: "expired", Reason: "expired"},
		// A repo-write stale-lease expiry — the RFC stale-lease storm signal —
		// renders as a DISTINCT to="stale_lease" series (prior-review F1). It is a
		// closed-enum value with no slash/sha/argv/byline shape, so the golden +
		// forbidden-content regex prove the now-live render path stays redacted.
		{From: "active", To: "stale_lease", Reason: "expired"},
	}
}

func sentinelWedgeAges() []WedgeObservation {
	return []WedgeObservation{
		{Origin: OriginDaemonCore, AgeSeconds: 42},
		{Origin: OriginDaemonCore, AgeSeconds: 1200},
	}
}

func sentinelMargins() []MarginObservation {
	return []MarginObservation{
		{Origin: OriginLane, MarginSeconds: 120},
		{Origin: OriginLane, MarginSeconds: -45},
	}
}

// sentinelDoctorRecords plants every sensitive provenance class onto doctor
// problem records, whose ONLY wire-eligible field is the static `check` code. The
// id-bearing fields (run_id, gate_id, byline) reuse the sentinel literals, so the
// golden + forbidden-content regex prove the Phase C doctor_problems fold copies
// only the static class onto the wire (F-A8) and leaks no dynamic id.
func sentinelDoctorRecords() []map[string]any {
	return []map[string]any{
		{"check": "recovery_sweep_cursor_wedged", "run_id": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
		{"check": "recovery_sweep_cursor_wedged", "run_id": "/home/halbritt/git/secret-target-repo"},
		{"check": "dissent_ledger_incomplete", "gate_id": "feature/leak-sentinel-branch"},
		{"check": "artifact_anchor_hash_mismatch", "byline": "author: leaker-model-007"},
	}
}

func renderSentinelSnapshot(t *testing.T) []byte {
	t.Helper()
	snap := Build(SnapshotInput{
		BuiltAt:              sentinelBuiltAt,
		Runs:                 sentinelRuns(),
		StrandedSupervisors:  3,
		Events:               sentinelEvents(),
		LeaseTransitions:     sentinelLeaseTransitions(),
		WedgeAges:            sentinelWedgeAges(),
		LivenessMargins:      sentinelMargins(),
		DoctorProblemRecords: sentinelDoctorRecords(),
		RepoMetrics:          sentinelRepoMetrics(),
	})
	var buf bytes.Buffer
	if err := snap.WriteText(&buf, sentinelNow); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	return buf.Bytes()
}

// TestMetricsRedactionGoldenAndForbiddenContent is the exfiltration contract and
// the load-bearing backstop: it pins the exposition byte-for-byte against a
// committed golden AND independently asserts the body carries no sensitive
// value, even though the snapshot input was seeded with every sensitive class.
// Both must pass. It runs under `go test ./...` / `make check` — not as a
// manual-only check.
func TestMetricsRedactionGoldenAndForbiddenContent(t *testing.T) {
	body := renderSentinelSnapshot(t)

	// Regenerate the golden with STRIATUM_UPDATE_GOLDEN=1 when the intended
	// label *names* change; the diff is then a deliberate, reviewed edit.
	if os.Getenv("STRIATUM_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, body, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated golden %s", goldenPath)
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenPath, err)
	}
	if !bytes.Equal(body, want) {
		t.Fatalf("rendered /metrics body does not match golden %s.\n--- got ---\n%s\n--- want ---\n%s",
			goldenPath, body, want)
	}

	text := string(body)
	for _, lit := range sentinelLiterals {
		if strings.Contains(text, lit) {
			t.Errorf("forbidden sentinel %q leaked into /metrics body", lit)
		}
	}
	for _, re := range forbiddenShapes {
		if loc := re.FindString(text); loc != "" {
			t.Errorf("forbidden-content shape %q matched %q in /metrics body", re.String(), loc)
		}
	}
}
