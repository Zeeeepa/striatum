package mutations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestEnforceSessionBindingContract pins the per-session capability-token guard
// (RFC 0096 V2 / #135), the single shared predicate every session-scoped work
// handler consults. It is hermetic (no DB): the guard is a pure function of the
// AuthContext threaded onto ctx by the RPC dispatch layer.
//
//   - a BOUND token acting as its OWN session is allowed, not an override;
//   - a BOUND token acting as ANOTHER session is refused capability_denied —
//     this is the closed impersonation vector;
//   - an UNBOUND (operator/coordinator) token may act as any session, flagged
//     OperatorOverride so the handler records honest provenance;
//   - NO AuthContext on ctx (a direct call / a transport that does not
//     authorize) is treated as UNBOUND, never as a bound bypass.
func TestEnforceSessionBindingContract(t *testing.T) {
	ctx := context.Background()
	const repo = "repo_bind"

	t.Run("bound token own session allowed", func(t *testing.T) {
		binding, err := enforceSessionBinding(boundCtx(ctx, repo, "sess_A"), "sess_A")
		if err != nil {
			t.Fatalf("own-session bound: unexpected error %v", err)
		}
		if binding.OperatorOverride {
			t.Fatalf("own-session bound must not be an operator override")
		}
	})

	t.Run("bound token cross session denied", func(t *testing.T) {
		_, err := enforceSessionBinding(boundCtx(ctx, repo, "sess_A"), "sess_B")
		if !isRPCCode(err, "capability_denied") {
			t.Fatalf("cross-session bound: want capability_denied, got %v", err)
		}
	})

	t.Run("unbound operator token is override", func(t *testing.T) {
		binding, err := enforceSessionBinding(operatorCtx(ctx, repo), "sess_anything")
		if err != nil {
			t.Fatalf("operator override: unexpected error %v", err)
		}
		if !binding.OperatorOverride {
			t.Fatalf("unbound operator token must be flagged OperatorOverride")
		}
	})

	t.Run("no auth context treated as override not bypass", func(t *testing.T) {
		binding, err := enforceSessionBinding(ctx, "sess_anything")
		if err != nil {
			t.Fatalf("no-auth: unexpected error %v", err)
		}
		if !binding.OperatorOverride {
			t.Fatalf("no AuthContext must be treated as unbound override, never a bound bypass")
		}
	})
}

// TestClaimNextRejectsCrossSessionBoundToken proves the guard is WIRED into
// work.claim_next (RFC 0096 V2 / #135): a token bound to session B cannot claim
// work as session A, and a token bound to A claiming as A is not refused by the
// binding guard. PG-gated.
func TestClaimNextRejectsCrossSessionBoundToken(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_claim_bound_cross"
	runID := "run_claim_bound_cross"
	sessionA := "sess_claim_A"
	role := "worker"
	lane := "claude"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{role: map[string]any{}},
		"lanes":       map[string]any{lane: map[string]any{"display_model": "Claude"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionA, role, lane, nil, "active")
	intgAttest(t, ctx, runner, repoID, runID, sessionA, lane)

	// A token bound to a DIFFERENT session (B) trying to claim AS session A.
	spoof := boundCtx(ctx, repoID, "sess_claim_B")
	_, err := HandleClaimNext(spoof, runner, intgEnv(repoID, map[string]any{"session_id": sessionA}))
	if !isRPCCode(err, "capability_denied") {
		t.Fatalf("cross-session claim: want capability_denied, got %v", err)
	}

	// A token bound to A claiming as A must NOT be refused by the binding guard
	// (it returns no_work / a packet, never capability_denied).
	own := boundCtx(ctx, repoID, sessionA)
	if _, err := HandleClaimNext(own, runner, intgEnv(repoID, map[string]any{"session_id": sessionA})); isRPCCode(err, "capability_denied") {
		t.Fatalf("own-session claim was wrongly refused by the binding guard: %v", err)
	}
}

// TestPublishArtifactRejectsCrossSessionBoundToken proves the guard is WIRED
// into artifact.publish (RFC 0096 V2 / #135): a token bound to session B cannot
// publish as session A — refused on receipt, before the lease-ownership check
// (so knowing another session's observable lease_id no longer enables a publish
// spoof). PG-gated.
func TestPublishArtifactRejectsCrossSessionBoundToken(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_publish_bound_cross"
	intgSeedRepo(t, ctx, runner, repoID)

	spoof := boundCtx(ctx, repoID, "sess_publish_B")
	_, err := HandlePublishArtifact(spoof, runner, intgEnv(repoID, map[string]any{
		"session_id":   "sess_publish_A",
		"job_id":       "job_x",
		"lease_id":     "lease_x",
		"kind":         "finding",
		"logical_name": "FINDING",
		"path":         "FINDING.md",
	}))
	if !isRPCCode(err, "capability_denied") {
		t.Fatalf("cross-session publish: want capability_denied, got %v", err)
	}
}

func TestClosedPredecessorTokenReturnsStaleSessionRemediation(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_stale_session_token"
	runID := "run_stale_session_token"
	oldSession := "sess_stale_old"
	successorSession := "sess_stale_successor"
	role := "worker"
	lane := "codex"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{role: map[string]any{}},
		"lanes":       map[string]any{lane: map[string]any{"display_model": "Codex"}},
	})
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, oldSession, role, lane, nil, "closed", 1)
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, successorSession, role, lane, nil, "active", 2)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET closed_at = NOW(), close_reason = 'superseded'
		 WHERE repository_id = $1 AND session_id = $2`, repoID, oldSession); err != nil {
		t.Fatalf("mark predecessor closed: %v", err)
	}

	callers := []struct {
		name string
		call func() (map[string]any, error)
	}{
		{
			name: "artifact.publish",
			call: func() (map[string]any, error) {
				return HandlePublishArtifact(boundCtx(ctx, repoID, oldSession), runner, intgEnv(repoID, map[string]any{
					"session_id":   successorSession,
					"job_id":       "job_x",
					"lease_id":     "lease_x",
					"kind":         "finding",
					"logical_name": "FINDING",
					"path":         "FINDING.md",
				}))
			},
		},
		{
			name: "review.submit",
			call: func() (map[string]any, error) {
				return HandleSubmitReview(boundCtx(ctx, repoID, oldSession), runner, intgEnv(repoID, map[string]any{
					"session_id": successorSession,
					"job_id":     "job_x",
					"lease_id":   "lease_x",
					"path":       "REVIEW.md",
					"verdict":    "accept",
				}))
			},
		},
		{
			name: "review.verdict",
			call: func() (map[string]any, error) {
				return HandleRecordVerdict(boundCtx(ctx, repoID, oldSession), runner, intgEnv(repoID, map[string]any{
					"session_id": successorSession,
					"job_id":     "job_x",
					"lease_id":   "lease_x",
					"verdict":    "accept",
				}))
			},
		},
		{
			name: "work.complete",
			call: func() (map[string]any, error) {
				return HandleCompleteWork(boundCtx(ctx, repoID, oldSession), runner, intgEnv(repoID, map[string]any{
					"session_id": successorSession,
					"job_id":     "job_x",
					"lease_id":   "lease_x",
					"summary":    "done",
				}))
			},
		},
	}

	for _, caller := range callers {
		t.Run(caller.name, func(t *testing.T) {
			_, err := caller.call()
			var rpcErr *rpc.Error
			if !errors.As(err, &rpcErr) || rpcErr.Code != "session_token_stale" {
				t.Fatalf("%s err = %v, want session_token_stale", caller.name, err)
			}
			if got := rpcErr.Details["bound_session_id"]; got != oldSession {
				t.Fatalf("%s bound_session_id = %v, want %s", caller.name, got, oldSession)
			}
			if got := rpcErr.Details["requested_session_id"]; got != successorSession {
				t.Fatalf("%s requested_session_id = %v, want %s", caller.name, got, successorSession)
			}
			if got := rpcErr.Details["method"]; got != caller.name {
				t.Fatalf("%s method detail = %v, want %s", caller.name, got, caller.name)
			}
			if !strings.Contains(rpcErr.Suggestion, "supervise.start") || !strings.Contains(rpcErr.Suggestion, "supervise.rebridge") {
				t.Fatalf("%s suggestion = %q, want start/rebridge remediation", caller.name, rpcErr.Suggestion)
			}
		})
	}
}
