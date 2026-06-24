You are the **Committer** for the **RFC 0143 Slice A** design run. The adjudicator's
collaboration ledger has cleared the gate. Publish the final,
falsification-hardened **implementation spec** as your `PROPOSAL.md` artifact — the
design run's primary deliverable, the spec the `rfc-0143-slice-a-build` `code_change`
run will build contract-first (TDD).

Start from the Holder's `HOLDER.md` and fold in every challenge the adjudicator
recorded as material-and-incorporated. The committed spec MUST carry, as first-class,
build-ready design:

- **The reserved agentloop floor exit code** (`go/pkg/agentloop/exitcodes.go`,
  `ExitUnrecoverableAcrossRotation`) — and an explicit statement that Slice A owns ONLY
  this floor code (reseal code 98, `resealInFlightJob`, the connect-out channel, the
  kernel-token capture, `CapabilityReseal`, and owner bundle 0021 are Slice B, OUT OF
  SCOPE, blocked on RFC 0168 / #585).
- **Spot 1 — credential-chain narrowing**: the exact function, the non-owner-lane
  detection, the typed sentinel, and the agentloop mapping to a clean reserved-code
  exit — with the no-admin-token-widening invariant explicit (the owner process is
  unaffected; the lane never reads the admin token).
- **Spot 2 — daemon observation + typed-class recovery routing**: the
  `#{pane_dead_status}`/`processExitCode` observation, the new typed helper-event /
  recorder branch / recovery-class wiring, the launch/attach-failure path recording the
  typed class instead of a raw `helper_error`, the silent-death fallback to
  `agent_exited_unsealed` (no over-fire), the `recoverStuckJobs` /
  `isNecrosisStallClass` classification, and the relationship to
  `HandleRecoveryCompleteStalled` (#292).

The spec MUST:

- **Name the exact surfaces** to touch (`go/pkg/agentloop/token.go`,
  `endpoint.go`, `loop.go`, `exitcodes.go`; `go/pkg/supervisor/helper.go`;
  `go/pkg/mutations/supervision_launch.go`, `supervision.go`,
  `recovery_decision_tree.go`, `recovery_complete_stalled.go`) with the precise edit
  per surface.
- **Specify the build slices in contract-first order** (smallest safe first), each with
  its named Go tests and exact file touches, additive-only.
- **State the explicit Acceptance Criteria** an impl + verify run must meet, including
  the two mandatory game-day shapes: **(a)** a non-owner lane whose resolution chain
  reaches the admin `client-token` surfaces the typed
  `session_unrecoverable_across_rotation` floor (reserved exit code observed by the
  daemon), **not** a generic permission error and **not** a silent unsealed exit, while
  an owner process still resolves the admin token; **(b)** a capture-boundary /
  launch-attach miss that the floor covers records the typed class, **not** a raw
  `helper_error` / "PTY helper failed before attach"; and the negatives — **(c)** an
  ordinary unsealed exit with no reserved code stays `agent_exited_unsealed` (no
  over-fire), and **(d)** a provider child's 97/98 cannot forge the reserved code (C2).
- **Restate the HARD CONSTRAINTS** as build guardrails: no token widening, no Slice-B
  artifact, daemon-side/process state only, no over-fire, no raw-error leak,
  additive-only (existing recovery/supervise/agentloop tests pass unchanged),
  product-boundary-safe.
- **Recommend `write_scope`** `go/` (NOT `go/**` — see #586, the prefix matcher is
  not a glob) and the build context_docs (this `PROPOSAL.md`, the RFC 0143 Decision,
  the v7 `BC1-W1-CAPTURE-FLOOR` finding).

Publish the spec only after confirming the ledger verdict cleared the gate.
