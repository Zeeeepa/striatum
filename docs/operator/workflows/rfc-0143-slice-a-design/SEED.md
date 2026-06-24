# Design-Run Seed — RFC 0143 **Slice A**: the legible `session_unrecoverable_across_rotation` typed-exit floor (decoupled, ships now)

> **This is a fresh design run for Slice A only.** RFC 0143 was split by **D261**
> (2026-06-24) after a seven-cycle `falsification_gate` design run (v1→v7, banked
> under `docs/operator/artifacts/rfc-0143-design-v{6,7}/`) and a `/adhd` analysis.
> The gate proved the **authenticated reseal channel** (options 2/3 — the
> `CapabilityReseal` authority, the W1 connect-out control channel, the kernel-token
> capture) is **unsolvable while every lane shares the `striatum-lane` uid**
> (`BC1-W1-ORACLE`: the production tmux control surface runs as the shared uid with a
> deterministic session name and no private socket, so a same-uid sibling can
> `respawn-pane`-replace the pane the daemon launched and the daemon — whose only
> handle is a post-launch tmux query — authenticates the replacement; a `0600` reseal
> file is the same same-uid replay surface). That whole channel is **Slice B**, now
> **blocked on [RFC 0168](../../../rfcs/0168-per-lane-security-principal.md)** (per-lane
> OS uid) and tracked by **#585**. **Do NOT design any of Slice B here.**
>
> This run designs **Slice A**: the maintainer-ratified **Option 4** floor — make a
> `striatum-lane` lane that cannot reseal after a daemon boot-epoch rotation **fail
> LEGIBLY** with a typed `session_unrecoverable_across_rotation` signal instead of a
> silent unsealed exit or a misleading "permission denied". Per D261 this is **pure,
> daemon-side observability**: it **mints no credential, widens no token, and does not
> touch the credential trust model**. The deliverable of this run is a **falsifiable
> implementation spec** (`PROPOSAL.md`) the `rfc-0143-slice-a-build` `code_change`
> run executes contract-first (TDD).
>
> **Required reading before any artifact:** this whole SEED; the committed RFC
> `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md` **in full**,
> especially `## Current behavior` and `## Decision (D261)`; the D261 row in
> `docs/decisions/decision-log.md`; and the `BC1-W1-CAPTURE-FLOOR` finding in the v7
> ledger `docs/operator/artifacts/rfc-0143-design-v7/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
> (wired in as a required `context_doc` — it gives the exact fix shape for the
> capture-boundary→typed-floor wiring, adapted here to the **decoupled** world).

## Framing — what this run must produce

A **falsifiable implementation spec** for Slice A that the build run executes TDD.
Slice A surfaces the typed `session_unrecoverable_across_rotation` floor in **two
related spots**, both computed from **daemon-side durable / process state only**,
with **no dependency on an authenticated inbound frame** from the lane (that
independence is the load-bearing premise D261 source-verified and ratified — see
`## The decoupling premise (ratified, do NOT relitigate)`).

The spec must be concrete and falsifiable — file:line anchors, named Go tests, and a
mechanically-derived classification, not a restatement of the RFC.

## The design shape this run must harden (the ratified Option-4 floor)

The maintainer ratified **Option 4** (D261). The mechanism — confirmed by the v7
design record, which named `ExitUnrecoverableAcrossRotation = 97` as "the Option-4
floor … observed via the `#{pane_dead_status}` capture, never from
`result.Cmd.Wait()`" — is a **reserved agentloop exit code observed by the daemon
from durable process/tmux state**. That is precisely why Slice A is decoupled: a
process exit status is *not* an authenticated inbound frame. The holder must specify
(and the falsifiers re-attack) the following shape, or a strictly-better decoupled
shape that meets every constraint below:

1. **A reserved agentloop exit code** for the floor (new
   `go/pkg/agentloop/exitcodes.go`, e.g. `ExitUnrecoverableAcrossRotation = 97`).
   **Slice A owns ONLY this floor code.** The reseal-request code (the v7 `98`), the
   `resealInFlightJob` mutation, the connect-out channel, the kernel-token capture,
   the `CapabilityReseal` authority, and owner bundle 0021 are **Slice B — out of
   scope, do not introduce them.**
2. **Spot 1 — credential-chain narrowing (RFC 0143 Option 4 / #512).** When a
   **non-owner** lane's credential-resolution chain would fall through to the
   owner-only admin runtime `client-token`, the resolver **refuses** that step and
   returns a **typed sentinel**; the agentloop maps the sentinel to a **clean exit
   with the reserved floor code** instead of a generic permission error or a silent
   unsealed exit. This removes the misleading "permission denied" dead-end **without
   letting the lane read the admin token** (it still cannot reseal — that needs Slice
   B).
3. **Spot 2 — daemon observation + recovery routing (closes `BC1-W1-CAPTURE-FLOOR`).**
   The daemon observes the reserved floor code from **durable state** (`#{pane_dead_status}`
   on the tmux path; `processExitCode` on the direct path) and **records/routes the
   typed `session_unrecoverable_across_rotation` class** in supervision + the recovery
   sweep. The **launch/attach failure path** that today surfaces a raw
   `helper_error` / "PTY helper failed before attach" must, when the floor applies,
   record the **typed class**, not a raw error. A truly silent death (no reserved
   code, no typed event) falls back to the **existing `agent_exited_unsealed` class —
   the typed floor must NOT over-fire on it.**
4. **C2 forge-resistance (carry the v7 commitment).** The wrapper must never let a
   **provider child's** exit status (97/98 emitted by the underlying agent process)
   drive the reserved floor code — keep / extend
   `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`.

## The decoupling premise (ratified, do NOT relitigate the conclusion; DO verify the wiring)

D261 **overrode** the SEED-OQ1 clause "Slice A must route over the structurally-bound
channel" as a *documentation* coupling, after **source-verifying** that Slice A's
predicates are computed entirely from daemon-side durable state with **no inbound
authenticated frame**:

- **Deliverable-complete-on-disk:** `striatumd.artifacts` rows via
  `verifyRequiredArtifacts` + git-blob reconstructability via
  `verifyRequiredArtifactReconstructable`
  (`go/pkg/mutations/recovery_complete_stalled.go`).
- **Lane-lost:** tmux `#{pane_dead}` / `#{pane_dead_status}` +
  `/proc`+`kill(0)` liveness (`go/pkg/supervisor/tmux_liveness.go`).
- **The floor signal itself:** a **process exit status** (the reserved code), observed
  via `#{pane_dead_status}` / `processExitCode`. A process exit status is durable
  observed state, **not** an authenticated frame.

The conclusion (Slice A is decoupled; the W1 connect-out channel is a pure Slice-B
feature, NOT a prerequisite) is **ratified — do not reopen it.** What the falsifiers
**must** verify is that the holder's concrete wiring **actually honors** it: that no
step the spec specifies secretly needs an inbound authenticated frame, the W1
channel, a kernel-token capture, or any Slice-B artifact.

## Source anchors (re-verify against current `main` and correct any drift)

- **Spot 1 — credential chain.** `go/pkg/agentloop/token.go` —
  `ResolveTokenMaterial` step 3 falls through to the runtime `client-token`
  (`token.go:31-42`); `ReadTokenFile` rejects a non-owner-only file (`token.go:75-92`).
  `go/pkg/agentloop/endpoint.go:110-135` — `ResolveTokenMaterialFresh` re-reads the
  runtime `client-token` directly (the #323 endpoint-rotation recovery path).
  Callers: `go/pkg/agentloop/loop.go:37`, `:78`, and the #323 rotation recovery at
  `loop.go:602`. The admin token's full grant set is minted in
  `go/pkg/admin/bootstrap.go` (`{admin,read,write,claim,review,apply,recovery,surgical_recovery}`,
  `0600`, owner-only by construction).
- **Spot 2 — launch/attach + helper events + recovery sweep.**
  `go/pkg/supervisor/helper.go:157-173` — `RunHelper` emits `helper_error` phase
  `launch` on a launch error. `go/pkg/mutations/supervision_launch.go:562-591` —
  `waitForHelperAgentStart` treats a helper error before `agent_started` as a raw
  "PTY helper failed before attach". `go/pkg/mutations/supervision.go:217-234` — the
  helper-event schema admits only `{agent_exited, agent_started, artifact_observed,
  attach_client_exited, helper_error, packet_accepted, process_terminated, progress}`
  — **no** typed capture-boundary event. `go/pkg/mutations/recovery_decision_tree.go`
  — `recoverStuckJobs` classifies via `sessionliveness.Classify` into
  `stallClassAgentPIDDead = "agent_pid_dead"` / `stallClassAgentExitedUnsealed =
  "agent_exited_unsealed"` (`:171-197`); `isNecrosisStallClass` (`:187-197`) tags the
  necrosis set the RFC 0137 Phase-B exporter counts — the new typed class must be
  classified coherently here. `go/pkg/mutations/recovery_complete_stalled.go` —
  `HandleRecoveryCompleteStalled` (#292) already finalizes a dead-lane job from
  durable artifacts; the spec must state how the typed floor **relates to** this verb
  (route to it / escalate, not duplicate it).
- **Boot epoch (context only — NOT a per-lease durable record).**
  `go/cmd/striatumd/main.go` — `daemonBootEpoch()` is an **in-memory per-process**
  random value written to an owner-only `mcp-boot-epoch` file (`:713-763`); validated
  on MCP HTTP via `X-Striatum-Boot-Epoch` (`go/pkg/mcp/http.go:681-699`). **There is
  no durable per-lease/per-job record of the epoch a lease was minted under.** So the
  Slice-A predicate must NOT assume it can detect "a rotation happened" from durable
  DB state; the **causal** information lives at Spot 1 (the credential refusal), and
  Spot 2 routes the **reserved code**, falling back to `agent_exited_unsealed` when
  there is no reserved code.

## HARD CONSTRAINTS (Slice A only — a violation is `reject`/`needs_revision`)

1. **No token widening.** No path widens who can read the admin runtime
   `client-token`; no lane ever reads it. No minted credential carries any of
   `{admin, apply, recovery, surgical_recovery}`. (Spot 1 *narrows* the chain — it
   refuses a step, never adds a read path.) This is the single hottest blast-radius
   dimension; a widening is `reject`.
2. **No new credential / no Slice B.** Do **not** design the `CapabilityReseal`
   authority, the connect-out control channel, any reseal-token file, the
   kernel-token capture (W1/W2/W3), reserved code `98`, or owner bundle 0021. Slice A
   does not let the lane reseal; it makes the failure legible. The lane still requires
   an operator requeue (or Slice B, later) to actually seal.
3. **Daemon-side durable / process state only.** Every predicate is computed from
   `striatumd.artifacts` rows, git-blob reconstructability, tmux/`#{pane_dead_status}`/
   `/proc`/`kill(0)` liveness, and the reserved process exit code. **No dependency on
   an authenticated inbound frame** from the lane.
4. **No over-fire; no raw-error leak.** A capture-boundary / launch-attach miss that
   the floor covers must produce the **typed class**, never a raw `helper_error` /
   "PTY helper failed before attach" / generic permission error as the terminal
   explanation. Conversely an ordinary unsealed exit or a healthy/in-progress lane
   (no reserved floor code) must **NOT** be misclassified as the typed floor — it
   stays `agent_exited_unsealed` / its existing class.
5. **Default-off / additive where it touches live paths.** Existing recovery +
   supervise + agentloop tests must pass **unchanged**. A new helper-event type /
   recovery class / exit code is additive; do not change the meaning of an existing
   one.
6. **No durable transcript / external persistence / product-boundary breach** (AGENTS.md).

## Falsifiable assertions the spec MUST state (each paired with its named test)

- **A1 (Spot 1, narrowing):** a **non-owner** lane whose resolution chain reaches the
  admin runtime `client-token` gets a typed sentinel → reserved floor exit, **not** a
  generic permission error and **not** a silent unsealed exit; an **owner** process
  (the operator) is unaffected and still resolves the admin token normally. Test:
  `TestResolveRefusesRuntimeClientTokenForLane` (the v7 carried-set name) + an
  owner-unaffected companion.
- **A2 (Spot 2, capture-boundary → typed class):** a launch/attach miss that the floor
  covers records the typed `session_unrecoverable_across_rotation` class, **not** a
  raw `helper_error` / launch-handshake failure as the terminal explanation. Test: a
  new supervision/recovery test (name it).
- **A3 (no over-fire):** an ordinary `agent_exited_unsealed` (lane died with a
  complete-on-disk deliverable but **no** reserved floor code) stays
  `agent_exited_unsealed`; the typed floor does not fire. Test: a negative.
- **A4 (no widening):** no path reads the admin token from a lane; no minted
  credential carries an elevated verb. Test: assert the resolver still refuses the
  non-owner-only file and the lane never obtains the admin token.
- **A5 (C2 forge-resistance):** a provider child's 97/98 cannot drive the reserved
  floor code. Test: `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`.
- **A6 (no regression):** existing recovery (`recoverStuckJobs`, the necrosis
  classification, `HandleRecoveryCompleteStalled`) and supervise/agentloop tests pass
  unchanged; the new helper-event type / recovery class / exit code is additive.

## Lenses for the two falsifiers

- **falsifier_1 — DECOUPLING / DAEMON-SIDE lens.** Does the holder's concrete wiring
  truly compute every predicate and the floor signal from daemon-side durable /
  process state with **no inbound authenticated frame**? Find any step that secretly
  needs the W1 channel, a kernel-token capture, a reseal-token file, or any Slice-B
  artifact. Probe the boot-epoch gap: the spec must NOT silently assume a durable
  per-lease epoch record (there is none — `main.go:713-763`). Does Spot 2 correctly
  attribute the floor **only** from the reserved exit code (not from ambiguous
  "complete-on-disk + lane-lost" alone, which would over-fire on ordinary unsealed
  exits)?
- **falsifier_2 — SECURITY / LEGIBILITY / REGRESSION lens.** Does any path widen who
  can read the admin token, or mint a credential carrying an elevated verb (→
  `reject`)? Does any covered miss still leak a raw `helper_error` / generic
  permission error (→ legibility failure)? Does the typed floor over-fire on an
  ordinary unsealed exit or a healthy lane? Are existing recovery/supervise/agentloop
  tests regressed, or is an existing event-type/class/exit-code's meaning changed
  (must be additive)? Can a provider child forge the reserved code (C2)?

## Clearing condition

The adjudicator clears the gate (`accept` / `accept_with_findings`) **only if** all of:

1. The two spots are specified concretely with file:line anchors and the reserved
   floor exit code, and **both** are computed from daemon-side durable / process state
   with **no inbound authenticated frame** (the decoupling premise honored in the
   actual wiring).
2. **No HARD CONSTRAINT is violated** — no token widening, no Slice-B artifact, no
   over-fire, no raw-error leak, additive-only, daemon-side-only.
3. Every falsifiable assertion A1–A6 is stated and paired with a named test, including
   the no-over-fire negative (A3) and the forge-resistance test (A5).
4. The relationship to the existing `agent_exited_unsealed` class and the
   `HandleRecoveryCompleteStalled` (#292) verb is stated (the typed floor routes /
   escalates legibly; it does not duplicate or override them).
5. No new material challenge stands unrebutted.

**Verdict guidance.** `reject` only if a path widens admin-token exposure or mints a
credential carrying any of `{admin, apply, recovery, surgical_recovery}`, or if the
spec smuggles in Slice B. Otherwise `needs_revision` if any spot depends on an inbound
authenticated frame / a Slice-B artifact, if the floor over-fires or leaks a raw
error, if a falsifiable assertion or its test is missing, or if an existing class is
regressed. This run allows **one** revision cycle; the falsifiers re-attack the
revised spec.

---
<sub>Operator scaffold for the RFC 0143 **Slice A** falsification-gate design run —
the decoupled Option-4 `session_unrecoverable_across_rotation` typed-exit floor
(reserved agentloop exit code observed from durable process/tmux state; credential-chain
narrowing at `agentloop.ResolveTokenMaterial{,Fresh}`; capture-boundary→typed-class
recovery routing closing `BC1-W1-CAPTURE-FLOOR`), with ZERO trust-model change (no
admin-token widening, no minted credential, no Slice-B channel/reseal/kernel-token/owner-bundle).
Slice B (the `CapabilityReseal` authority + connect-out channel) is blocked on RFC 0168
(#585) and is OUT OF SCOPE. Lanes: author=claude (holder/adjudicator/committer),
reviewer=claude (falsifiers/final).</sub>
