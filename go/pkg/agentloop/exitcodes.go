package agentloop

import "errors"

// ExitUnrecoverableAcrossRotation is the reserved agentloop process exit code for
// the RFC 0143 Slice A floor (D261, Option 4). The supervised lane wrapper exits
// with this code when, ON ITS OWN MCP CLIENT PATH, it observes the daemon rejecting
// it as stale_daemon_identity across a boot-epoch rotation (#512), OR when its
// credential-resolution chain would fall through to the owner-only admin runtime
// client-token for a NON-OWNER lane. It is a DIRECT-PATH CORROBORATOR of the typed
// floor, not its primary producer: the primary producer is the daemon's own
// observation of the rejection (daemon.stale_epoch_rotation; see pkg/mcp/http.go).
// The daemon reads the code from durable process state (agent_exited.exit_code on
// the direct path) and routes the typed session_unrecoverable_across_rotation
// recovery class.
//
// Forge-resistance scope (RFC 0143 Slice A Correction 2): the reserved code is
// forge-resistant on the DIRECT (plain-PTY) path only — it is the wrapper's own
// Cmd.Wait status, which a provider child cannot drive (normalizeAgentExitError,
// loop.go, refuses to propagate a child's numeric exit). It is NOT claimed
// forge-resistant on the tmux path (a same-uid sibling can respawn the pane with
// `exit 97`), so the recovery classifier never trusts a bare tmux #{pane_dead_status}
// == 97 without a corroborating daemon observation. Under the shared striatum-lane
// uid no lane-side signal is forge-resistant; that residual is RFC-0168-bounded
// (#585) and is sound only because the typed floor is observability-only: it grants
// no new authority over agent_exited_unsealed.
//
// Slice A owns ONLY this floor code. The reseal-request code (98), resealInFlightJob,
// the connect-out channel, the kernel-token capture, the CapabilityReseal authority,
// and owner bundle 0021 are Slice B (RFC 0168 / #585) — NOT defined here.
const ExitUnrecoverableAcrossRotation = 97

// ErrUnrecoverableAcrossRotation is the typed sentinel the lane maps — and ONLY it —
// to a clean exit with ExitUnrecoverableAcrossRotation. It is returned by the
// credential resolver (NEVER a token, NEVER a read of the admin file) when a
// non-owner lane's chain reaches the owner-only admin runtime client-token, AND by
// the rotation watcher / codex wedge when the lane's own MCP path is provably lost
// across a daemon boot-epoch rotation. An owner process never receives the resolver
// sentinel (adminTokenReachedByNonOwner returns false for the owner).
var ErrUnrecoverableAcrossRotation = errors.New("agentloop: session unrecoverable across daemon boot-epoch rotation")
