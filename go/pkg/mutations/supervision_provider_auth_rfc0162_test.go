package mutations

import (
	"context"
	"errors"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/laneproviderauth"
)

// passedCodexCheck stubs a real, passing codex provider-auth check.
func passedCodexCheck(_ context.Context, params laneproviderauth.Params) laneproviderauth.Result {
	return laneproviderauth.Result{
		Checked:   true,
		Provider:  laneproviderauth.ProviderCodex,
		RunID:     params.RunID,
		LaneID:    params.LaneID,
		RunAsUser: params.RunAsUser,
		Status:    laneproviderauth.StatusPassed,
	}
}

// TestAuthSuccessEventOnlyOnPassedCodex is FA-5: the heartbeat is emitted strictly
// downstream of a real result.Passed() for a provider that actually runs Check
// (codex), and NEVER on the unsupported early return or on a failed check.
func TestAuthSuccessEventOnlyOnPassedCodex(t *testing.T) {
	origCheck := supervisionProviderAuthCheck
	origEmit := emitLaneAuthSuccessEvent
	defer func() {
		supervisionProviderAuthCheck = origCheck
		emitLaneAuthSuccessEvent = origEmit
	}()

	var emittedLaneUser string
	emitCount := 0
	emitLaneAuthSuccessEvent = func(_ context.Context, _ db.Runner, config supervisionStartConfig, result laneproviderauth.Result) error {
		emitCount++
		emittedLaneUser = config.RunAsUser
		if result.Provider != laneproviderauth.ProviderCodex {
			t.Fatalf("heartbeat emitted for non-codex provider %q", result.Provider)
		}
		return nil
	}

	codexConfig := supervisionStartConfig{
		RepositoryID:    "repo_1",
		RunID:           "run_1",
		LaneID:          "lane_1",
		SessionID:       "sess_1",
		AgentLoopMode:   agentLoopModeSelfDriving,
		OriginalCommand: []string{"codex"},
		RunAsUser:       "codexuser",
	}

	// Passed codex check → exactly one heartbeat naming the lane user.
	supervisionProviderAuthCheck = passedCodexCheck
	if err := runSuperviseProviderAuthGate(context.Background(), nil, codexConfig, laneproviderauth.GateRequired); err != nil {
		t.Fatalf("passed codex gate returned error: %v", err)
	}
	if emitCount != 1 || emittedLaneUser != "codexuser" {
		t.Fatalf("passed codex: emitCount=%d laneUser=%q; want 1, codexuser", emitCount, emittedLaneUser)
	}

	// Failed codex check → no heartbeat (and the gate refuses).
	emitCount = 0
	supervisionProviderAuthCheck = func(_ context.Context, params laneproviderauth.Params) laneproviderauth.Result {
		return laneproviderauth.Result{
			Checked: true, Provider: laneproviderauth.ProviderCodex,
			Status: laneproviderauth.StatusFailed, FailureClass: laneproviderauth.FailureAuthFailed,
		}
	}
	if err := runSuperviseProviderAuthGate(context.Background(), nil, codexConfig, laneproviderauth.GateRequired); err == nil {
		t.Fatalf("failed codex gate returned nil; want refusal")
	}
	if emitCount != 0 {
		t.Fatalf("failed codex emitted %d heartbeats; want 0", emitCount)
	}

	// Unsupported provider (non-codex) → the supported==false early return, never a
	// heartbeat, even though it never runs Check.
	emitCount = 0
	supervisionProviderAuthCheck = func(_ context.Context, _ laneproviderauth.Params) laneproviderauth.Result {
		t.Fatalf("Check must not run for an unsupported provider")
		return laneproviderauth.Result{}
	}
	claudeConfig := codexConfig
	claudeConfig.OriginalCommand = []string{"claude"}
	if err := runSuperviseProviderAuthGate(context.Background(), nil, claudeConfig, laneproviderauth.GateAuto); err != nil {
		t.Fatalf("unsupported provider GateAuto returned error: %v", err)
	}
	if emitCount != 0 {
		t.Fatalf("unsupported provider emitted %d heartbeats; want 0", emitCount)
	}
}

// TestAuthSuccessEventWriteFailureDoesNotChangeGateVerdict is FA-7: a failed
// heartbeat write is swallowed — the gate still returns nil on a passed check, so
// telemetry can never flip a gate decision.
func TestAuthSuccessEventWriteFailureDoesNotChangeGateVerdict(t *testing.T) {
	origCheck := supervisionProviderAuthCheck
	origEmit := emitLaneAuthSuccessEvent
	defer func() {
		supervisionProviderAuthCheck = origCheck
		emitLaneAuthSuccessEvent = origEmit
	}()

	supervisionProviderAuthCheck = passedCodexCheck
	emitLaneAuthSuccessEvent = func(_ context.Context, _ db.Runner, _ supervisionStartConfig, _ laneproviderauth.Result) error {
		return errors.New("simulated event-write failure")
	}

	config := supervisionStartConfig{
		RepositoryID:    "repo_1",
		RunID:           "run_1",
		LaneID:          "lane_1",
		SessionID:       "sess_1",
		AgentLoopMode:   agentLoopModeSelfDriving,
		OriginalCommand: []string{"codex"},
		RunAsUser:       "codexuser",
	}
	if err := runSuperviseProviderAuthGate(context.Background(), nil, config, laneproviderauth.GateRequired); err != nil {
		t.Fatalf("a failed heartbeat write changed the gate verdict: %v", err)
	}
}
