# RFC 0131: Transport-aware liveness confidence and escalation gating

Status: accepted (D211) — Layers 1–3+4 (131-A #334, 131-B #335, 131-C #336) IMPLEMENTED; 131-D (#337, doctor/legibility) remains — OQ3 resolved D230 (closes #348): pursue the confidence model (layers 131-A..D); the PTY-shim-all-pipe-lanes alternative stays tracked, to re-evaluate after 131-A + 131-C land with measured pipe-lane misclassification data
Date: 2026-06-17
author: proposer-claude-opus-4-8-001
Context:
- GitHub [#311](https://github.com/halbritt/striatum/issues/311) — the incident
  (a flaky `agy`/Gemini reviewer lane wedged an otherwise-complete run to
  `needs_operator` via `liveness_deadline_missed` → `recovery.budget_exhausted`).
  This RFC graduates the **P1** design the issue's closing disposition captured
  but deliberately deferred.
- Decision [D209](../decisions/decision-log.md) — #311 **P0** (per-job quarantine
  + run finalize-the-majority). P0 is shipped and deployed; this RFC does **not**
  restate it. P0 and P1 compose: see "Relationship to P0" below.
- Decision [D210](../decisions/decision-log.md) — the Wave-2 closeout whose
  "Revisit Trigger" column names this graduation explicitly.
- [RFC 0118](0118-gate-run-completion-on-attested-provenance.md) — the
  run-completion provenance gate that the escape-valve must not let a quarantine
  bypass.
- [RFC 0026](0026-lane-attestation-and-operator-byline-honesty.md) — lane
  attestation derives from a supervised-session binding; the transport split this
  RFC formalizes is the same `pty_helper`-vs-pipe distinction.
- Prior art in source: `go/pkg/sessionliveness/liveness.go` (`Classify()` +
  `Policy`), `go/pkg/mutations/recovery_decision_tree.go`
  (`recoverStuckJobs`, `supervisedAgentConfirmedDead`, `markRecoveryEscalation`),
  `go/pkg/db/sql/0020_job_recovery_state.sql`,
  `go/pkg/db/sql/0019_pty_and_tool_liveness.sql`,
  `go/pkg/db/sql/0012_mcp_activity_liveness.sql`.
- GitHub [#241](https://github.com/halbritt/striatum/issues/241) — the helper
  false-liveness class this RFC is adjacent to.

## Problem

The session-liveness classifier (`sessionliveness.Classify()` + `Policy`) is
**transport-agnostic**. A lane that runs under a `pty_helper` (claude/codex,
`transport: pty_helper`, `require_tmux: true`) reports `last_pty_activity_at`,
and the recovery decision tree can call `supervisedAgentConfirmedDead()` to
*prove* the supervised process is gone. A bare **pipe / agent-loop lane**
(`agy`/Gemini, no supervision block) has **no PTY activity signal and no
confirmed-dead oracle**. The classifier nonetheless judges it on the same
deadline, so a quiet-but-working pipe lane reads as "stuck" too easily. Recovery
then restart-loops it (`forget → start → forget → start`) until
`recovery.budget_exhausted` fires, which — pre-P0 — escalated the **whole run**
to `needs_operator`, discarding every completed sibling's attested work.

P0 (D209) fixed the *blast radius*: a single exhausted leaf job is quarantined
and the run finalizes the majority. But P0 gates the **outcome** (what happens
once a job is declared exhausted). It does not change the **decision** that got
the pipe lane to exhaustion in the first place. `recovery.budget_exhausted` still
carries no signal distinguishing *"the agent genuinely hung"* from *"liveness
tracking misfired for a lane type that has no oracle"*. A pipe lane that was
fine still burns its whole restart budget on misfires, and still ends up
quarantined (work lost) rather than left to finish.

Three concrete gaps:

1. **No transport in the liveness verdict.** `Classify()` cannot say "this was a
   PTY-confirmed death" vs "this was only a deadline elapsing on a lane I cannot
   probe." Both collapse to the same stall verdict.
2. **A pipe lane has a thin liveness surface that goes unused.** A working pipe
   lane almost always touches the daemon (MCP request, lease renewal, artifact
   query) — that activity is already recorded
   (`0012_mcp_activity_liveness.sql`) but is not consulted as a pipe-transport
   liveness rung the way `last_pty_activity_at` is for PTY lanes.
3. **Escalation is single-sweep and ungated for pipe lanes.** One sweep's
   deadline-elapsed verdict can advance a pipe lane toward whole-budget
   exhaustion with no requirement for corroboration and no record of *why* it
   was treated as dead.

## Goals

1. Make **transport** and a typed **`probe_basis`** first-class outputs of the
   liveness classifier, so every recovery action records *what kind of evidence*
   it acted on.
2. Give a pipe-transport lane a **confidence-gated escalation path**: a
   `deadline_elapsed_only` stall on a lane with no confirmed-dead oracle must
   clear a higher bar (corroboration and/or sustained, compounding silence)
   before it may flip the run toward `needs_operator`.
3. **Never create an un-escalatable lane.** A genuinely-hung pipe lane never
   produces a confirmed-dead signal, so the design must include a hard,
   finite-by-construction escape valve that fires regardless of confidence.
4. Keep the escape valve **un-gameable**: it must key off evidence a dead-but-
   spinning agent cannot forge.
5. Make every gating decision **auditable provenance** (events + doctor +
   dashboard), per the product boundary — escalation must not become silent
   daemon behavior.

## Non-Goals

- Changing P0's quarantine/finalize-the-majority mechanics (D209). This RFC
  feeds P0 a better decision; it does not alter P0's outcome handling.
- A general distributed-consensus liveness service. Cross-lane corroboration
  here is opportunistic (the cohort is already in hand), not a new subsystem.
- Telemetry, hosted health checks, or any external liveness probe. All signals
  are daemon-local PostgreSQL state (RFC 0033/0043 boundary).
- Correctness of the work the lane produces. Liveness confidence is orthogonal
  to verdict/artifact correctness; this RFC must never conflate "lane is live"
  with "lane did the right thing."

## Proposed design

A four-layer design. Layers 1–2 are cheap and strictly additive; layer 3 is the
confidence gate; layer 4 is the non-negotiable safety floor. They are listed in
recommended landing order.

### Layer 1 — Transport + `probe_basis` as classifier outputs (no migration)

**IMPLEMENTED (131-A, #334).** Thread a `TransportType`
(`pty_helper | pipe`) into `sessionliveness.Classify()` and stamp
`Result.ProbeBasis ∈ { pty_confirmed_dead, deadline_elapsed_only }`.
`pty_confirmed_dead` is set only when `supervisedAgentConfirmedDead()` had a pane
/ PID to judge and judged it dead; `deadline_elapsed_only` is the pipe case (and
any PTY case where no process could be probed). This is a pure-function change
unit-testable against the existing `liveness_test.go`, with **no schema change**,
and it is a prerequisite for everything below. The recovery action map in
`recoverStuckJobs` carries `probe_basis` onto the `recovery.*` event payload.

As built:

- Transport threads in via a new `Activity.Transport TransportType`
  (`TransportPTYHelper | TransportPipe | TransportUnknown`), derived in
  `ActivityFromRow` from the supervised lane's
  `process_supervisor_pointers.metadata_json` `transport` field (always
  `pty_helper` or `pipe` from `supervise.start` config). An absent/unrecognized
  value yields `TransportUnknown`, treated as the lower-confidence pipe case
  (degrade-safe, per the Risks "default to the lower-confidence classification on
  ambiguity").
- `Classify()` is a pure function with no oracle, so it stamps EVERY stall
  `Result.ProbeBasis = deadline_elapsed_only` (and leaves it empty for a
  non-stall verdict). The `pty_confirmed_dead` UPGRADE is performed in the
  recovery decision tree by `sessionliveness.UpgradeProbeBasisConfirmedDead`,
  which promotes a `pty_helper` lane's `deadline_elapsed_only` to
  `pty_confirmed_dead` only once `supervisedAgentConfirmedDead()` fires (a pipe /
  unknown lane has no PTY oracle and stays `deadline_elapsed_only`).
- `recoverStuckJobs` carries `transport` + typed `probe_basis` onto the
  `recovery.budget_exhausted` event payload and the requeue/transfer action
  records. This slice is OUTPUTS only: no Layer 2 pipe rung, no Layer 3
  confidence gate, no Layer 4 escape-valve cap, and **no migration**.

### Layer 2 — A pipe-transport liveness rung from existing RPC activity

**IMPLEMENTED (131-B, #335).** Surface the daemon-side activity timestamp a pipe
lane already produces (`last_mcp_request_at` / lease-renewal touch, recorded by
`0012_mcp_activity_liveness.sql`) as a pipe-transport liveness input, analogous
to `last_pty_activity_at` for PTY lanes. A working pipe lane that is mid-RPC is
classified `working_local`, not stalled. This shrinks the hard problem to the
genuinely-silent-but-working case (a long model generation with no intervening
RPC), which is exactly the case the confidence gate exists for.

As built: `Classify()` adds a `pipeMidRPCFresh` rung — for a `TransportPipe`
lane whose `last_mcp_request_at` is fresh within `ProtocolFreshSeconds`, it
returns `working_local` BEFORE the await-packet / ack stall rungs (which are
anchored on stale `last_tools_list_at` / `last_packet_delivered_at` and would
otherwise read a mid-RPC pipe lane as stalled). It is scoped to `pipe` transport
(pty_helper lanes keep their exact prior classification — their `working_local`
comes from the PTY rung) and never weakens dead-lane detection: a pipe lane whose
`last_mcp_request_at` has aged past `ProtocolFreshSeconds` falls through to the
existing rungs (the protocol-idle catch-all already folds `last_mcp_request_at`
into its base, so a genuinely-silent pipe lane still trips `StallProtocolIdle`).

### Layer 3 — Confidence-gated escalation for `deadline_elapsed_only` pipe stalls

**IMPLEMENTED (131-C, #336).** Add `misfire_evidence_score int`,
`consecutive_silent_sweeps int`, and `last_probe_basis text` to
**`job_recovery_state`** (a *runtime* table — not owner-held — so this is a
runtime migration, no owner bundle). None of these columns existed; migration
`0035_job_recovery_confidence_gate.sql` adds them (substrate_version → 35) via
`ADD COLUMN IF NOT EXISTS` with the runtime-role GRANT, the same shape migration
0021 used to extend this runtime-owned table.

When `recoverStuckJobs` would escalate a job AND `probe_basis ==
deadline_elapsed_only` AND the session is neither dead/absent nor
confirmed-dead (the genuinely-ambiguous no-oracle case — a closed session or a
confirmed-dead process probe is itself a death oracle and escalates immediately):

- If a **forgery-resistant progress signal** advanced since the last sweep
  (a sealed artifact anchor / sealed verdict whose `created_at` is after the
  prior sweep, or a cross-lane cohort sibling showing fresher liveness), reset
  `misfire_evidence_score` and `consecutive_silent_sweeps` to 0 and **defer**
  (the lane is demonstrably working).
- Else compound `misfire_evidence_score`, increment
  `consecutive_silent_sweeps`, write a `recovery.escalation_debounced` event, and
  **do not escalate this sweep**.
- Escalation commits only when the prior sweep was also `deadline_elapsed_only`
  and this is the **2nd consecutive silent sweep** (a single sweep never
  escalates a pipe lane), OR the Layer-4 cap fires.

PTY lanes that upgrade to `pty_confirmed_dead` skip the gate entirely — they
already have a real oracle.

**Cross-lane corroboration (free).** `recoverStuckJobs` already scans every
unfinished job for the run under one `FOR UPDATE OF j`, so the sibling cohort is
already in memory. Before escalating a `deadline_elapsed_only` pipe lane,
`cohortHasFresherLiveness` checks whether any sibling shows liveness fresher than
the window start; if so, the solo pipe deadline is treated as a likely misfire
and deferred (it folds into `progressAdvanced`). A single-pipe-lane run has no
siblings, so the Layer-4 cap — not corroboration — is its safety guarantee.

### Layer 4 — The escape valve: a finite cap keyed off forgery-resistant progress

**IMPLEMENTED (131-C, #336).** This is the load-bearing invariant. A
genuinely-hung pipe lane **never** produces a confirmed-dead signal, so
confidence alone can never escalate it — there must be a hard cap. Two rules make
the cap safe:

1. **The cap is an integer ceiling, not a ratchet.** Once
   `consecutive_silent_sweeps ≥ cap` (default `(maxRequeues*2)+3`, floored at 3,
   operator-overridable via `recovery_policy.max_silent_sweeps`), escalation
   fires regardless of `misfire_evidence_score` (`capFired`). The confidence
   reduction of Layer 3 **does not apply** when the session is provably silent
   across consecutive sweeps — silence itself is the oracle the pipe lane lacks.
2. **What resets the silent-sweep counter is forgery-resistant.** A
   dead-but-spinning agent (the #324 spinner class) can keep emitting stderr/PTY
   frames and trivial protocol chatter that *look like* activity. If the counter
   reset on raw output, such a loop would be un-escalatable forever. So the reset
   keys **only** off *sealed-work progress advancement* — `jobSealedProgressAt`
   reads the latest `created_at` across the `artifacts` and `verdicts` rows for
   the job (a published artifact anchor or a sealed verdict, both written only by
   the daemon's publish/verdict handlers) — evidence a dead loop cannot
   manufacture. Raw protocol chatter is explicitly **not** a reset signal.

Because silence monotonically advances `consecutive_silent_sweeps` and only real
sealed progress resets it, the lane is escalatable in bounded time by
construction. A deployment behind on migration 0035 degrades safely to today's
ungated single-sweep escalation (the gate reads tolerate the absent columns).

### Relationship to P0 (D209)

P1 gates the **decision to escalate**; P0 contains the **consequence**. When the
Layer-4 cap (or a confirmed-dead PTY oracle) finally fires, the job is handed to
P0's existing `escalateExhaustedJobs` / `quarantineExhaustedJob` path unchanged —
quarantine the one leaf, finalize the majority, honor the RFC 0118 provenance
gate. P1 simply ensures far fewer *misfired* pipe lanes ever reach that point.

## Alternatives considered (and why they are traps)

The parallel-frame design exploration surfaced several attractive mechanisms that
are traps; recording them so they are not re-proposed:

- **Exponential decay of the misfire score on any clean sweep.** Looks like tidy
  memory management; actually lets a bursty pipe lane (normal silent generation
  bursts) never accumulate enough to resist escalation, *or* — paired with a
  ceiling — makes a twice-misfired lane permanently immune. Use a monotone
  silent-sweep counter reset only by forgery-resistant progress, with a hard
  ceiling. Decay-by-time of a *misfire-rate* signal for threshold *widening* is
  acceptable; decay of the escalation counter is not.
- **Permanent immunity once the misfire score exceeds a ceiling.** The dual
  failure: an un-escalatable lane. Forbidden by Goal 3.
- **Naive global-quiescence inhibition.** "All pipe lanes silent ⇒ host pause ⇒
  inhibit escalation" is dangerous: a genuine fleet-wide wedge (daemon lost its
  MCP endpoint, host OOM) looks identical to a benign pause and would suppress
  escalation exactly when it matters most. If adopted at all, global inhibition
  must be **strictly weaker** than the per-job cap (it may delay, never reset)
  and must raise its own run-level alarm at a shorter cap.
- **Selling cross-lane corroboration as the general fix.** A single-pipe-lane run
  has no siblings, so corroboration gives it zero benefit. The escape-valve cap
  is the only guarantee there; corroboration is an optimization for the
  multi-lane case, not the safety mechanism.
- **Resetting confidence on raw protocol activity.** Defeated by the #324
  spinner. Covered by Layer 4 rule 2.

## Risks

- **Long legitimate silent generation.** A pipe lane mid-generation on a large
  artifact with no RPC and no checkpoint accrues silent sweeps like a hung lane.
  Mitigation: Layer 2 (RPC rung) plus an adapter-emitted cheap periodic
  `work.heartbeat` for pipe transports; the cap must be set so
  `cap × sweep_interval` exceeds the longest plausible silent turn.
- **Liveness misclassification races** (the #147 dead-PID-probe-vs-WARM-lease
  class). P1 inherits any latent bug in `supervisedAgentConfirmedDead`. Default
  to the *lower-confidence* classification on ambiguity.
- **Tuning surface.** New thresholds (`cap`, two-sweep window) are policy. Expose
  them on `recovery_policy` with conservative defaults; cover with fixtures.

## Migration and rollout

- One **runtime** migration (`0035_job_recovery_confidence_gate.sql`,
  substrate_version → 35) adding the three `job_recovery_state` columns. No owner
  bundle (the table is runtime-owned: `striatumd_rw` created it in the runtime
  migration 0020, so the runtime-role `ApplyMigrations` path may ALTER it). The
  future-runtime-owner-DDL guard (floor 27) carries a documented allowlist entry
  for `striatumd.job_recovery_state` so this legitimate runtime-owned-table column
  add is permitted while owner-held ALTERs stay forbidden.
- Layer 1 (131-A) shipped first with no migration and is independently valuable
  (legible `probe_basis` on events).
- Default policy preserves current behavior for PTY lanes; the gate engages only
  for a still-present, non-confirmed-dead lane on a `deadline_elapsed_only` basis.
  A deployment behind on the migration falls back to today's ungated escalation
  (degrade-safe, like P0's bundle-0012 guard): `readJobRecoveryBudget` retries
  on the legacy column set and the gate escalates immediately.

## Doctor and legibility

- `recovery.*` events carry `probe_basis` and, when deferred,
  `recovery.escalation_debounced` with the current
  `consecutive_silent_sweeps` / `misfire_evidence_score`.
- `striatum dashboard --run-id` and `run.summary` surface the gate state for a
  lane being debounced, so an operator sees "deferred, 2/N silent sweeps" rather
  than silence.
- A doctor warning when a pipe lane has exceeded the cap but the run did not
  escalate (an invariant breach), so the safety floor is itself observable.

## Test plan

- `sessionliveness` unit tests for `ProbeBasis` across PTY-dead, PTY-alive,
  pipe-silent, and pipe-RPC-active inputs.
- A recovery fixture with a 3-lane run (2 pipe + 1 PTY) asserting the
  **never-un-escalatable** invariant: a provably-silent pipe lane escalates at
  the cap; a forging (chatter-only) lane also escalates at the cap; an
  RPC-active pipe lane never escalates.
- A misfire regression: a pipe lane that goes silent then makes real progress
  resets and is not quarantined (the #311 incident, replayed).
- Run `make -C go vet lint check-tests` **uncontended** (the mutations `-race`
  pgtest suite times out at the CI 30m limit only under concurrent-PG load).

## Open questions

1. Should the escape-valve cap be **adaptive to run topology** (a generous cap
   for a leaf job, a tight cap for a critical-path job whose downstream is
   blocked)? Simpler to ship a single cap first.
2. Should Layer 2's pipe-transport rung be promoted to a synthetic
   `last_pipe_read_at` stamped by the adapter, so corroboration is only needed
   for the truly-silent case? (Captured as a child idea; deferrable.)
3. **Provocation / alternative worth deciding:** wrap every pipe lane in a thin
   PTY shim purely to manufacture a `last_pty_activity_at` oracle, making
   `confirmedDead()` universal and dissolving the confidence-modeling problem
   entirely. Is the pipe transport's signal-starvation worth *removing* at the
   transport layer rather than *compensating for* in the classifier? This would
   obsolete Layers 2–4; it trades a classifier change for an adapter/supervision
   change and a heavier per-lane cost.

## Appendix — design provenance

The layered design, the forgery-resistant-progress invariant, and the trap
catalog above were produced by a parallel-frame divergent-ideation run
(regulator, 3am-on-call, inversion, hostile-competitor, and biology frames over
isolated branches, then convergence) grounded against the real
`recovery_decision_tree.go` / `sessionliveness` source. The
"escape-valve must key off un-forgeable progress, not raw output" insight is the
load-bearing result and is the reason Layer 4 is specified the way it is.
