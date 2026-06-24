You are the **Committer** for the **RFC 0143 Slice A** design run (v2 REVISION). The
adjudicator's collaboration ledger has cleared the gate. Publish the final,
falsification-hardened **implementation spec** as your `PROPOSAL.md` artifact — the
design run's primary deliverable, the spec the `rfc-0143-slice-a-build` `code_change`
run will build contract-first (TDD).

Start from the revised `HOLDER.md` and fold in every challenge the adjudicator recorded
as material-and-incorporated. The committed spec MUST carry, as first-class, build-ready
design:

- **FIX-1 (rotation-path producer that FIRES on the real #512 path):** the
  forge-resistant daemon-side observation — when the daemon rejects a stale-epoch
  request (`validateBootEpoch`, `http.go:166-169/:681-699`) it records the presenting
  session as unrecoverable-across-rotation, and the recovery sweep records the typed
  class for a session observed presenting a stale epoch + complete-on-disk deliverable
  + lane-lost — with the **pre-auth session attribution** specified concretely and not
  over-firing (or the honestly-scoped lane-side `stale_daemon_identity` → 97 fallback);
  the codex wedge (`loop.go:625-646`) routed.
- **FIX-2 (forge-resistant carrier):** the trusted carrier is the daemon-observed
  rejection / the direct-path `agent_exited.exit_code`; the tmux `#{pane_dead_status}`
  carrier is corroborated or honestly RFC-0168-scoped (not claimed forge-resistant).
- **The v1-credited skeleton carried forward unregressed:** §1 reserved code + sentinel
  (`go/pkg/agentloop/exitcodes.go`, `ExitUnrecoverableAcrossRotation`; Slice A owns
  ONLY 97); §2 Spot-1 narrowing; §3.2–3.4 exact-code-only classification; §3.5
  launch-handshake dissolution; §3.6 #292 relationship; §4 direct-path C2; the
  no-widening invariant.
- **The OBSERVABILITY-ONLY clarification:** the typed floor grants no new auto-seal
  authority; routing no-more-privileged than `agent_exited_unsealed`.

The spec MUST:

- **Name the exact surfaces** to touch (`go/pkg/agentloop/token.go`, `endpoint.go`,
  `loop.go`, `exitcodes.go`; `go/pkg/mcp/http.go` `validateBootEpoch`;
  `go/pkg/supervisor/helper.go`, `tmux_liveness.go`; `go/pkg/mutations/supervision.go`,
  `supervision_launch.go`, `recovery_decision_tree.go`, `recovery_complete_stalled.go`)
  with the precise edit per surface, and the daemon-side durable record (additive).
- **Specify the build slices in contract-first order** (smallest safe first), each with
  its named Go tests and exact file touches, additive-only.
- **State the explicit Acceptance Criteria** an impl + verify run must meet, including
  the game-day shapes: **(a)** a session-bound lane (carrying `STRIATUM_MCP_TOKEN`) that
  presents a stale boot epoch surfaces the typed `session_unrecoverable_across_rotation`
  floor on the real rotation path (FIX-1), **not** a silent unsealed exit / ordinary
  class; **(b)** a same-uid `tmux respawn-pane … exit 97` does NOT forge the typed class
  (FIX-2 negative); **(c)** an ordinary unsealed exit / healthy lane stays its existing
  class (no over-fire); **(d)** a provider child cannot drive the reserved code on the
  direct path (C2). Map each `Test*` name to the criterion it satisfies.
- **Restate the HARD CONSTRAINTS** as build guardrails: no token widening, no Slice-B
  artifact, daemon-side/own-observation only, no over-fire / no raw-error leak / no
  silent rotation exit, additive-only (existing recovery/supervise/agentloop tests pass
  unchanged), product-boundary-safe.
- **Recommend `write_scope`** `go/` (NOT `go/**` — #586, the prefix matcher is not a
  glob) and the build context_docs (this `PROPOSAL.md`, the v2 design ledger, the RFC
  0143 Decision, the v7 `BC1-W1-ORACLE` finding).

Publish the spec only after confirming the ledger verdict cleared the gate.
