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

func TestInstalledCLISeatAgyTwoTurn(t *testing.T) {
	requireInstalledCLIGate(t)
	h := NewHarness(t, "repo_conf_agy_seat")
	report := RunInstalledCLI(t, h, "agy", InstalledCLIOptions{RunTimeout: installedCLITimeout(t)})
	logInstalledCLIReport(t, report)
	if !report.Passed() {
		t.Fatalf("agy installed-CLI seat gate (RFC 0109 P3 / #95) FAILED:\n  %s", strings.Join(report.Failures, "\n  "))
	}
}

// TestInstalledCLISeatAgyRestartWhileLeased is the #151 direct proof: the real
// agy CLI receives a work packet, the harness confirms the pre-seeded agy
// session owns an active job lease, then the daemon-facing HTTP + unix-socket
// surfaces are dropped, rebuilt, and rebound at the same addresses before agy
// publishes/completes the packet and continues to turn 2.
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
