package mutations

import (
	"context"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/laneproviderauth"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

var supervisionProviderAuthCheck = laneproviderauth.Check

// emitLaneAuthSuccessEvent is the RFC 0162 Layer 3 heartbeat write, a package var
// so tests can observe it / simulate a write failure. It is BEST-EFFORT: the gate
// discards its return, so a failed event write can never change the gate verdict
// (FA-7).
var emitLaneAuthSuccessEvent = defaultEmitLaneAuthSuccessEvent

func providerAuthGateMode(envelope rpc.Envelope) (laneproviderauth.GateMode, error) {
	raw, ok := envelope.Params["provider_auth_gate"]
	if !ok || raw == nil {
		return laneproviderauth.GateAuto, nil
	}
	text, ok := raw.(string)
	if !ok {
		return "", rpc.NewError("schema_invalid", "provider_auth_gate must be one of auto, required, off", nil)
	}
	mode, err := laneproviderauth.ParseGateMode(text)
	if err != nil {
		return "", rpc.NewError("schema_invalid", err.Error(), nil)
	}
	return mode, nil
}

func runSuperviseProviderAuthGate(ctx context.Context, runner db.Runner, config supervisionStartConfig, mode laneproviderauth.GateMode) error {
	if mode == laneproviderauth.GateOff {
		return nil
	}
	provider := config.adapterName()
	if provider == "" {
		provider = "unknown"
	}
	supported := config.AgentLoopMode == agentLoopModeSelfDriving && provider == laneproviderauth.ProviderCodex
	if !supported {
		if mode == laneproviderauth.GateRequired {
			return providerAuthRPCError(unsupportedProviderAuthResult(config, provider))
		}
		return nil
	}
	if mode == laneproviderauth.GateAuto && strings.TrimSpace(config.RunAsUser) == "" {
		return nil
	}
	result := supervisionProviderAuthCheck(ctx, laneproviderauth.Params{
		Provider:  provider,
		Binary:    providerAuthBinary(config),
		RunID:     config.RunID,
		LaneID:    config.LaneID,
		RunAsUser: config.RunAsUser,
		Env:       providerAuthPreflightEnv(config),
	})
	if result.Passed() {
		// RFC 0162 Layer 3 (FA-5): emit the codex-scoped auth-success heartbeat
		// strictly downstream of a real result.Passed() — never on the
		// unsupported early return above. BEST-EFFORT (FA-7): the write is on the
		// success path only and its error is swallowed, so a failed heartbeat
		// write can never flip the gate verdict or alter a timeout.
		_ = emitLaneAuthSuccessEvent(ctx, runner, config, result)
		return nil
	}
	return providerAuthRPCError(result)
}

// defaultEmitLaneAuthSuccessEvent writes the lane.auth_success domain event into
// the durable events ledger (folded by the metrics collector into
// striatum_lane_auth_last_success_timestamp_seconds{lane}). The payload carries
// only the closed-enum lane_user (roster slug / OS user), provider, and kind — no
// repo path, sha, branch, prompt, or byline. It runs in its own short transaction
// so the event-chain head stays contiguous; any error is returned to the caller,
// which discards it (best-effort, FA-7).
func defaultEmitLaneAuthSuccessEvent(ctx context.Context, runner db.Runner, config supervisionStartConfig, result laneproviderauth.Result) error {
	if runner == nil {
		return nil
	}
	laneUser := strings.TrimSpace(config.RunAsUser)
	payload := map[string]any{
		"lane_user": laneUser,
		"provider":  result.Provider,
		"kind":      laneproviderauth.KindOAuth,
	}
	_, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if _, aerr := appendEvent(ctx, tx, config.RepositoryID, config.RunID, "lane.auth_success", config.SessionID, nil, nil, nil, nil, payload); aerr != nil {
			return nil, aerr
		}
		return nil, nil
	})
	return err
}

func providerAuthBinary(config supervisionStartConfig) string {
	if len(config.OriginalCommand) == 0 || strings.TrimSpace(config.OriginalCommand[0]) == "" {
		return laneproviderauth.ProviderCodex
	}
	return strings.TrimSpace(config.OriginalCommand[0])
}

func unsupportedProviderAuthResult(config supervisionStartConfig, provider string) laneproviderauth.Result {
	return laneproviderauth.Result{
		Checked:           false,
		Provider:          provider,
		RunID:             config.RunID,
		LaneID:            config.LaneID,
		RunAsUser:         strings.TrimSpace(config.RunAsUser),
		Status:            laneproviderauth.StatusFailed,
		FailureClass:      laneproviderauth.FailureUnsupported,
		RawOutputReturned: false,
		Network:           "provider_cli_may_use_network",
		Costing:           "provider_tokens_may_be_spent",
		Remediation:       "use --provider-auth-gate off or configure a provider with a supported auth preflight",
	}
}

func providerAuthRPCError(result laneproviderauth.Result) *rpc.Error {
	message := fmt.Sprintf("lane provider auth preflight refused launch: %s", result.Remediation)
	details := map[string]any{"lane_provider_auth": result.ToMap()}
	var err *rpc.Error
	switch result.FailureClass {
	case laneproviderauth.FailureAuthFailed:
		err = rpc.NewError("lane_provider_auth_failed", message, details)
	case laneproviderauth.FailureBinaryMissing:
		err = rpc.NewError("lane_provider_binary_missing", message, details)
	case laneproviderauth.FailureUnavailable:
		err = rpc.NewError("lane_provider_unavailable", message, details)
	case laneproviderauth.FailureTimeout:
		err = rpc.NewError("lane_provider_preflight_timeout", message, details)
	case laneproviderauth.FailureLaunchFailed:
		err = rpc.NewError("lane_provider_preflight_launch_failed", message, details)
	case laneproviderauth.FailureUnsupported:
		err = rpc.NewError("lane_provider_preflight_unsupported", message, details)
	case laneproviderauth.FailureUnexpectedResult:
		err = rpc.NewError("lane_provider_preflight_unexpected_result", message, details)
	default:
		err = rpc.NewError("lane_provider_preflight_unexpected_result", message, details)
	}
	err.Suggestion = result.Remediation
	return err
}
