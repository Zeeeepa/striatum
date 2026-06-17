package adapterconformance

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// The RFC 0109 P3 installed-CLI seat gate. It drives the REAL lane CLI through a
// two-turn claim → publish → claim cycle and asserts the same pre-seeded,
// attested session drives both turns (#95), which requires the lane to launch
// past its trust/feedback prompt (#76/#139) and reach work.claim without an
// MCP-discovery stall (#85).
//
// It runs only when STRIATUM_P3_INSTALLED_CLI is set — this is the
// release-blocking SCHEDULED CI tier (and a local on-demand check), kept out of
// the default unit suite because it spawns a real LLM CLI and a live PostgreSQL
// harness. Within that tier it still skips cleanly when the specific CLI is
// absent (RunInstalledCLI) or STRIATUM_PG_TEST_URL is unset (NewHarness/pgtest).

func requireInstalledCLIGate(t *testing.T) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("STRIATUM_P3_INSTALLED_CLI")) == "" {
		t.Skip("STRIATUM_P3_INSTALLED_CLI not set; the RFC 0109 P3 installed-CLI seat gate runs in the scheduled release-blocking CI tier (and locally on demand), not the default unit suite")
	}
}

// installedCLITimeout bounds the two-turn cycle. Override with
// STRIATUM_P3_RUN_TIMEOUT (a Go duration, e.g. 90s) for fast red iteration.
func installedCLITimeout(t *testing.T) time.Duration {
	t.Helper()
	if raw := strings.TrimSpace(os.Getenv("STRIATUM_P3_RUN_TIMEOUT")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
		t.Fatalf("STRIATUM_P3_RUN_TIMEOUT=%q is not a positive Go duration", raw)
	}
	return 4 * time.Minute
}

func logInstalledCLIReport(t *testing.T, r InstalledCLIReport) {
	t.Helper()
	t.Logf("installed-CLI seat report: adapter=%s command=%v\n"+
		"  seat_session=%s session_count=%d sessions=%v\n"+
		"  job1=%s (owner=%s, artifact=%t) job2=%s (owner=%s, artifact=%t)\n"+
		"  run_err=%v after_first_work_packet_triggered=%t after_first_work_packet_err=%v",
		r.Adapter, r.Command, r.SeatSession, r.SessionCount, r.SessionIDs,
		r.Job1State, r.Job1Owner, r.Artifact1, r.Job2State, r.Job2Owner, r.Artifact2,
		r.RunErr, r.AfterFirstWorkPacketTriggered, r.AfterFirstWorkPacketErr)
	if out := strings.TrimSpace(r.Output); out != "" {
		t.Logf("lane output (tail):\n%s", tailLines(out, 60))
	}
}

func tailLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// agyLoginPickerMarkers are substrings the agy CLI emits when the current
// operator environment is not authenticated enough for a non-interactive
// installed-CLI gate. Detecting one lets local/scheduled runs skip with an
// environment-specific reason instead of burning the whole RunTimeout.
var agyLoginPickerMarkers = []string{
	"sign in",
	"sign-in",
	"log in to",
	"log in with",
	"select an account",
	"choose an account",
	"authentication required",
	"oauth",
	"open this url",
	"visit the following url",
	"press enter to",
	"continue with google",
}

func outputLooksLikeAgyLoginPicker(output string) bool {
	lower := strings.ToLower(output)
	for _, marker := range agyLoginPickerMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func TestInstalledCLISeatAgyTwoTurn(t *testing.T) {
	requireInstalledCLIGate(t)
	h := NewHarness(t, "repo_conf_agy_seat")
	report := RunInstalledCLI(t, h, "agy", InstalledCLIOptions{RunTimeout: installedCLITimeout(t)})
	logInstalledCLIReport(t, report)
	// An unauthenticated local agy install can still show an interactive login
	// picker. That is an environment setup gap for this on-demand gate, not a
	// supervised-seat regression in the authenticated environments where the gate
	// is expected to run green.
	if !report.Passed() && outputLooksLikeAgyLoginPicker(report.Output) {
		t.Skipf("agy CLI displayed an interactive login picker in this environment; authenticate agy before running the RFC 0109 P3 installed-CLI gate locally")
	}
	if !report.Passed() {
		t.Fatalf("agy installed-CLI seat gate (RFC 0109 P3 / #95) FAILED:\n  %s", strings.Join(report.Failures, "\n  "))
	}
}

// TestInstalledCLISeatAgyRestartWhileLeased is the #151 stress gate: the real agy
// CLI receives a work packet, the harness confirms the pre-seeded agy session
// owns an active job lease, then the daemon-facing HTTP + unix-socket surfaces
// are dropped, rebuilt, and rebound at the same addresses before agy is expected
// to publish/complete the packet and continue to turn 2.
func TestInstalledCLISeatAgyRestartWhileLeased(t *testing.T) {
	requireInstalledCLIGate(t)
	h := NewHarness(t, "repo_conf_agy_restart")
	report := RunInstalledCLI(t, h, "agy", InstalledCLIOptions{
		RunTimeout: installedCLITimeout(t),
		AfterFirstWorkPacket: func(ctx context.Context, event InstalledCLIFirstWorkPacket) error {
			lease, err := event.Harness.Observer().ActiveLease(ctx, event.RepositoryID, event.SessionID, event.JobID)
			if err != nil {
				return err
			}
			if lease == nil {
				return fmt.Errorf("no active lease for session %s job %s before restart", event.SessionID, event.JobID)
			}
			if fmt.Sprint(lease["owner_session_id"]) != event.SessionID {
				return fmt.Errorf("active lease owner = %v, want %s", lease["owner_session_id"], event.SessionID)
			}
			time.Sleep(100 * time.Millisecond)
			return event.Harness.RestartDaemonSurfaces()
		},
	})
	logInstalledCLIReport(t, report)
	// Same unauthenticated-environment skip as TestInstalledCLISeatAgyTwoTurn.
	if !report.Passed() && outputLooksLikeAgyLoginPicker(report.Output) {
		t.Skipf("agy CLI displayed an interactive login picker in this environment; authenticate agy before running the #151 restart-while-leased gate locally")
	}
	if !report.Passed() {
		t.Fatalf("agy restart-while-leased installed-CLI gate (#151) FAILED:\n  %s", strings.Join(report.Failures, "\n  "))
	}
}

// TestInstalledCLISeatCodexTwoTurn backs codex's supported seat tier. It covers
// the hermetic path that previously stayed queued against the in-process MCP
// harness: codex must launch past terminal/trust prompts, claim/publish/complete
// turn 1, then claim/publish/complete turn 2 under the same attested session.
func TestInstalledCLISeatCodexTwoTurn(t *testing.T) {
	requireInstalledCLIGate(t)
	h := NewHarness(t, "repo_conf_codex_seat")
	report := RunInstalledCLI(t, h, "codex", InstalledCLIOptions{RunTimeout: installedCLITimeout(t)})
	logInstalledCLIReport(t, report)
	if !report.Passed() {
		t.Fatalf("codex installed-CLI seat gate (RFC 0109 P3 / #95) FAILED:\n  %s", strings.Join(report.Failures, "\n  "))
	}
}

// TestInstalledCLISeatClaudeTwoTurn is the #358 conformance-honesty fixture for the
// claude seat — the flagship adapter that previously had NO installed-CLI coverage
// at all. It drives the real claude CLI through the same two-turn
// claim → publish → claim cycle under one attested session.
//
// claude's seat is still classified `experimental`, not `supported`: it is NOT in
// CIExecutedInstalledCLISeats and the scheduled `make installed-cli-check` does not
// run it yet (an authenticated-claude-on-CI gate that has not landed). The fixture
// exists so the day claude is wired into the CI gate, graduating its seat is a
// one-line registry change with an already-written, runnable conformance check —
// closing the "flagship adapter has no real-binary coverage" gap on the test side
// while the seat tier keeps telling the honest truth (experimental until CI-run).
//
// Like the other fixtures it is gated behind STRIATUM_P3_INSTALLED_CLI and skips
// cleanly when claude is absent, so it is a no-op in the default unit suite.
func TestInstalledCLISeatClaudeTwoTurn(t *testing.T) {
	requireInstalledCLIGate(t)
	h := NewHarness(t, "repo_conf_claude_seat")
	report := RunInstalledCLI(t, h, "claude", InstalledCLIOptions{RunTimeout: installedCLITimeout(t)})
	logInstalledCLIReport(t, report)
	if !report.Passed() {
		t.Fatalf("claude installed-CLI seat gate (RFC 0109 P3 / #95 / #358) FAILED:\n  %s", strings.Join(report.Failures, "\n  "))
	}
}
