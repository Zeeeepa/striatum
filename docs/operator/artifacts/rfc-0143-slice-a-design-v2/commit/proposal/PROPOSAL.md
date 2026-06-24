# RFC 0143 Slice A — final implementation spec (operator-cleared after v1+v2 falsification)

author: operator-claude-opus-4-8-001

Status: **cleared for build** by the operator, folding the v1 + v2 falsification
dialogue to its honest landing. The two design gates (v1 `rfc-0143-slice-a-design`,
v2 `rfc-0143-slice-a-design-v2`) drove the design to a precise, shippable shape; this
spec is the authoritative build contract. The detailed mechanics live in the **v2
`HOLDER.md`** (`docs/operator/artifacts/rfc-0143-slice-a-design-v2/dialogue/holder/HOLDER.md`,
§§1–8) — read it as the primary source. This document states the **two authoritative
corrections** the v2 falsifiers established, which OVERRIDE the v2 HOLDER where they
conflict.

## The honest design (what the gates converged on)

Slice A makes a `striatum-lane` lane that dies unsealed across a daemon boot-epoch
rotation (#512) **fail LEGIBLY** — the daemon records a typed
`session_unrecoverable_across_rotation` recovery class instead of a generic
`agent_exited_unsealed` / silent unsealed exit, so the operator (and the RFC 0137
exporter) immediately see *why* the lane stopped and that its deliverable is
complete-on-disk and just needs a requeue.

**Load-bearing truth (do NOT over-claim):** under the shared `striatum-lane` uid,
**no signal attributable to a session is forge-resistant** — a same-uid sibling holds
the session-bound token and can forge an exit code, a tmux pane status, *or* a
daemon-observed stale-epoch rejection (by presenting the stolen token + a stale
epoch). This is the v7 `BC1-W1-ORACLE` root and is exactly why real forge-resistance
is **RFC-0168-bounded** (Slice B, #585). Slice A is therefore **best-effort
legibility**, and that is sound because of the **observability-only** invariant: the
typed class grants **NO new authority** — it routes the *same* finalize-or-escalate
path `agent_exited_unsealed` already takes, only with a distinct reason. So a *forged*
typed class is **no more privileged than a forged `agent_exited_unsealed`** (which a
same-uid child can already cause by killing its own lane). The safety argument is
observability-only, **NOT** forge-resistance.

## Build per the v2 HOLDER, with these TWO authoritative corrections

### CARRY (from v2 HOLDER §§1–8, unchanged)
- **§1** reserved exit code `ExitUnrecoverableAcrossRotation = 97` + sentinel
  `ErrUnrecoverableAcrossRotation` in new `go/pkg/agentloop/exitcodes.go` (Slice A owns
  ONLY 97; NO reseal-98 / `resealInFlightJob` / connect-out channel / kernel-token
  capture / `CapabilityReseal` / owner bundle 0021).
- **§2** Spot-1 credential-chain **narrowing**: `adminTokenReachedByNonOwner` applied
  ahead of the read at the runtime `client-token` tier in both resolvers
  (`ResolveTokenMaterial` `token.go`, `ResolveTokenMaterialFresh` `endpoint.go`),
  refuse-before-read for a non-owner lane; `ReadTokenFile` owner-mode guard
  (`token.go:75-92`) retained; owner unaffected. (Secondary producer, not central.)
- **§3 (T1)** the **daemon-observed** producer: when `validateBootEpoch` rejects a
  request as `stale_daemon_identity` (`go/pkg/mcp/http.go:166-169,:681-700`), record a
  durable daemon-side `daemon.stale_epoch_rotation` observation attributed to the
  presenting session (pre-auth attribution per v2 HOLDER §3.2 — read-only identification
  of the bound session from the daemon's own token store; **grants no capability, the
  request is still rejected, widens nothing**). Route the codex rotated-endpoint wedge
  (`loop.go:625-646`) to the floor.
- **§4.2–4.3** the new `stallClassSessionUnrecoverableAcrossRotation`, interposed FIRST
  in `recoverStuckJobs` (`recovery_decision_tree.go`), exact-attribution; additive
  `isNecrosisStallClass` growth (the single disclosed existing-test change is
  `TestNecrosisDomainMatchesConfirmedDeadConstants`).
- **§4.5** launch-handshake dissolution: 97 is produced only after `agent_started`, so a
  genuine launch failure stays a raw `helper_error` (no raw-leak).
- **§5.1–5.2** relationship to `agent_exited_unsealed` (strict refinement, same routing)
  and `HandleRecoveryCompleteStalled` (#292 — route/escalate, do not duplicate/override).
- **§6** direct-path C2: `normalizeAgentExitError` (`loop.go:371-379`) keeps a provider
  child's 97/98 from driving the reserved code on the direct path.

### CORRECTION 1 (resolves v2 falsifier_1 — the T1 over-fire). **BINDING.**
The T1 observation MUST be **cleared / superseded when its session successfully
reconnects after the rotation** — i.e. when the session next passes `validateBootEpoch`
with the *current* epoch (or heartbeats / completes work successfully). The recovery
sweep fires the typed floor **only** when ALL hold: the session is **lane-lost**
(tmux `#{pane_dead}` / `/proc` / `kill(0)`), its required artifacts are
**complete-on-disk** (`verifyRequiredArtifacts` + `verifyRequiredArtifactReconstructable`),
**and a LIVE (un-superseded) `daemon.stale_epoch_rotation` observation** exists for it.
A session that recorded a stale-epoch observation but then **recovered** and later dies
for an **ordinary** reason MUST classify as `agent_exited_unsealed`, NOT the typed
floor. Test: `TestRecoveredSessionClearsStaleEpochObservationThenOrdinaryDeathStaysUnsealed`.

### CORRECTION 2 (resolves v2 falsifier_2 — drop the forge-resistance claim). **BINDING.**
Do **NOT** claim T1 (or any carrier) is forge-resistant. The spec, code comments, and
tests must frame the floor as **best-effort legibility, RFC-0168-bounded** (a same-uid
sibling can forge the stale-epoch observation by presenting the session's stolen token
with a stale epoch — `TestSameUidSiblingCanForgeStaleEpochObservation` documents this
honestly as a KNOWN RFC-0168-bounded limitation, not a defect). The safety rests on the
**observability-only** invariant, which MUST be enforced and tested: the typed floor's
recovery routing is **no-more-privileged than `agent_exited_unsealed`** — it triggers
**no auto-seal**, mints/uses no credential, and a lane still requires an operator
requeue (or Slice B) to seal. Tests:
`TestTypedFloorGrantsNoAutoSealAuthority` (the typed class routes the identical
finalize-or-escalate path as `agent_exited_unsealed`; no seal/publish happens on its
strength) and keep
`TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` (direct-path C2).
The tmux `#{pane_dead_status}` carrier is best-effort (records the class only when a
LIVE T1 observation corroborates, per Correction 1), explicitly NOT claimed
forge-resistant; replace the misleading
`TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation` framing with the
observability-only negative above.

## HARD CONSTRAINTS (build guardrails — a violation fails verification)
1. **No token widening** — no path adds a read of the admin runtime `client-token` for
   a lane; T1's pre-auth attribution is read-only identification, grants nothing; no
   minted credential carries `{admin, apply, recovery, surgical_recovery}`.
2. **No Slice B** — no `CapabilityReseal`, connect-out channel, kernel-token capture,
   reseal-token file, reseal-98, `resealInFlightJob`, or owner bundle 0021.
3. **Daemon-side / process state only** — no inbound authenticated frame dependency.
4. **No over-fire, no raw-error leak, no silent rotation exit** — Correction 1 enforces
   no over-fire; the floor fires on the genuine #512 path; ordinary/healthy/recovered
   lanes stay their existing class.
5. **Default-off / additive** — new file `exitcodes.go`; a refusal branch ahead of an
   existing read; a new daemon observation + stall class; one additive tmux probe field;
   one additive necrosis member. **Existing recovery + supervise + agentloop tests pass
   unchanged** except the single disclosed additive
   `TestNecrosisDomainMatchesConfirmedDeadConstants`.
6. **Product-boundary-safe** — no hosted service, durable transcript, or external
   persistence; state is the existing daemon PostgreSQL + local process/tmux observation.
   `write_scope` is `go/` (NOT `go/**` — #586). Any new runtime table/column is additive
   runtime-owned (no owner DDL); update the authority/inventory matrices if a new
   striatumd table is added.

## Acceptance criteria (the build + sealed verify must satisfy)
- (a) **Fires on the genuine #512 path:** a session-bound lane (carrying
  `STRIATUM_MCP_TOKEN`) that presents a stale boot epoch the daemon rejects records a
  LIVE `daemon.stale_epoch_rotation`; on its lane-lost + complete-on-disk death the
  recovery sweep records `session_unrecoverable_across_rotation` (not a generic class /
  silent exit).
- (b) **No over-fire (Correction 1):** a session that recovered (re-passed the epoch
  check) and later dies ordinarily classifies `agent_exited_unsealed`.
- (c) **No over-fire (baseline):** an ordinary unsealed exit with no observation stays
  `agent_exited_unsealed`; a healthy/in-progress lane is untouched.
- (d) **Observability-only (Correction 2):** the typed floor grants no auto-seal
  authority — routing identical to `agent_exited_unsealed`; a forged class causes no
  seal.
- (e) **Direct-path C2:** a provider child's 97/98 cannot drive the reserved code.
- (f) **No widening:** the resolver still refuses the non-owner-only admin token; the
  lane never reads it; T1 attribution grants nothing.
- (g) `go build ./... && go vet ./...` clean from `go/`; targeted package tests
  (`./pkg/agentloop/... ./pkg/mcp/... ./pkg/supervisor/... ./pkg/mutations/...`) green;
  existing suites pass unchanged (except the disclosed additive necrosis-domain test).

## Build slices (contract-first, smallest safe first)
1. `go/pkg/agentloop/exitcodes.go` — reserved code 97 + sentinel (+ unit).
2. Spot-1 narrowing in `token.go` / `endpoint.go` + caller mapping in `loop.go`
   (`Run`/`RunContext` → exit 97; #323 watcher / codex wedge routing) + `normalizeAgentExitError`
   C2 (tests A1/A4/C2).
3. T1 daemon observation: `go/pkg/mcp/http.go` `validateBootEpoch` records
   `daemon.stale_epoch_rotation` (durable, additive) with read-only pre-auth session
   attribution; the supersede/clear-on-recovery hook (Correction 1).
4. Recovery classification: new `stallClassSessionUnrecoverableAcrossRotation` + the
   exact predicate (lane-lost + complete-on-disk + LIVE observation) interposed FIRST in
   `recoverStuckJobs`; additive `isNecrosisStallClass`; observability-only routing
   (Corrections 1 + 2 tests).
5. tmux probe field (additive `PaneDeadStatus` on `ProbeTmuxLiveness`,
   `tmux_liveness.go`) as a best-effort corroborated carrier.
6. Docs/inventory: if a new striatumd table is added, update the read+write inventory +
   authority matrix; otherwise none.
