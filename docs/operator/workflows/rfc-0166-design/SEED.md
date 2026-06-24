# Design-Run Seed — RFC 0166 P0 (FRESH v1)

> This is the **fresh v1** `falsification_gate` design run for RFC 0166
> (completion deadline for an alive-but-never-completing lane — a
> "sealed-progress silence budget"; #576). The RFC has a developed **proposed
> direction** (a four-part mechanism) and explicitly asks to be **handed to
> RFC_REVIEW before acceptance** — this run is that review: harden the direction
> into falsifiable, build-bearing criteria and attack its named falsification
> surface (the AND-safety decision + the anti-gaming + the telomere ladder).
> **Required context docs** (read in full first):
> - `docs/operator/workflows/rfc-0166-design/RFC_0166_SNAPSHOT.md` — the RFC
>   (snapshotted from its review branch `rfc/0166-alive-but-never-completing-lane-deadline`;
>   the canonical RFC stays on that branch and is **not** merged to `docs/rfcs/`).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **falsifiable
implementation spec for RFC 0166 P0** the downstream `rfc-0166-build`
`code_change` run executes. The mechanism direction (the four-part
sealed-progress silence budget) is the RFC's proposal — harden it, do not invent
a different model. The SPEC turns each part into a concrete falsifiable
assertion + the test that would refute it, and ratifies the central safety
decision.

## The failure mode

A supervised lane can stay **alive but never complete**: a flailing agent (e.g.
a Claude reviewer looping) keeps emitting PTY output, so its supervisor renews
the work-lease every ~3.3 min and the stalled-lease classifier treats PTY output
as progress — indistinguishable from a healthy slow lane (one run held
`running` 1h42m with no `work.complete`/`review.submit`). `recovery
requeue-stale` refuses (warm lease); the only exits are `supervise stop` or a
whole-run `run cancel`. Prior liveness work (#147/#309/#311/RFC 0101 dead-lane;
#324 `StallToolProgress`/`wedged_no_tool_progress` for stopped tool-progress)
does **not** cover the **alive-and-loud but never forward-sealing** case.

## The hard core to PROVE

The deadline must distinguish **"alive but never completing"** from **"alive and
legitimately working"** — and must be **un-gameable** by a lane that emits fake
progress. Two properties, both load-bearing:

1. **No false-kill.** A legitimately slow-but-healthy lane is never killed
   mid-work. The acute case is the **single-final-seal lane** (only deliverable
   is one terminal `work.complete`, zero intermediate artifacts/verdicts → zero
   sealed events for its whole lifetime). False-killing it is exactly the #145
   CASE-2 mid-work artifact-loss regression the lease-heartbeat reprieve exists
   to prevent.
2. **Un-gameable.** A flailing lane cannot reset the clock by manufacturing
   cheap fake progress (re-publishing an unchanged blob, junk-byte blobs, minted
   verdicts).

## The proposed direction (harden these four parts)

- **Part 1 — the clock (detector).** A per-job deadline **derived, not stored**:
  wall-clock since the job's last forgery-resistant **sealed** event via the
  existing `jobSealedProgressAt` (`recovery_decision_tree.go` —
  `GREATEST(max(artifacts.created_at), max(verdicts.created_at))`), floored at
  `started_at`/lease `acquired_at`. New stall class
  `wedged_no_sealed_progress` next to `StallToolProgress`
  (`go/pkg/sessionliveness/liveness.go`), evaluated in `recoverStuckJobs` (not
  the pure `Classify()`, which never sees per-job `artifacts`/`verdicts`) —
  promoting the signal from a post-hoc `progressAdvanced` debounce to a
  first-class deadline. PTY heartbeats keep the **lease** warm but cannot touch
  this clock.
- **Part 2 — the correctness constraint (AND, not OR) — the central decision.**
  The rung fires **only when BOTH** the sealed clock is past budget **AND** the
  #324 tool-progress timeline is past `ToolProgressSeconds`. A lane silent on
  sealed work but still actively issuing tool calls (the single-final-seal lane
  genuinely working) is **not** trapped; only a lane gone quiet on *every*
  forgery-resistant axis, coasting on raw PTY noise, trips. **This AND is the
  safety decision the review must ratify.**
- **Part 3 — anti-gaming (reset on novelty).** A reset requires a **strict
  increase** in a daemon-derived monotonic progress position
  (`{count(distinct content_sha256), count(sealed verdicts), highest satisfied
  expected_artifacts milestone index}`) on `striatumd.job_recovery_state`. The
  deadline **consumes novelty, not events**: a re-published identical
  `content_sha256` does not advance the cursor. Grain is `content_sha256`
  (genuinely-new bytes count; idempotent re-anchor does not).
- **Part 4 — the action (self-heal ladder, telomere-bounded).** First breach
  auto-requeues **one** fresh session (no page) via
  `transfer_requeue`/`requeueJobSameAttempt`; the requeue budget resets **only
  on genuine sealed progress**, so a job burning fresh sessions without sealing
  **shortens toward a floor** (telomere) then escalates `needs_operator` with a
  copy-pasteable recovery verb (`recovery complete-stalled` if a durable
  artifact exists, else `recovery requeue-stale --override`).

## Open design points to DISCHARGE (each → a constraint + test)

- The **budget value(s)** — per-job-type or global; how chosen; the floor.
- **Restart/requeue consistency** of the derived clock + the
  `job_recovery_state` cursor (no spurious reset or carry-over).
- Whether Part 1/3 need a **migration** (new `job_recovery_state` columns).
- The **convergence proof** for the telomere ladder (no auto-requeue loop, no
  escalation storm) and the exact budget-counter interaction with the existing
  `maxRequeues`/`maxUnsealedRequeues`/`maxSilentSweeps`.

## P0 slice boundary

Define P0 as the minimum that closes #576 safely: the derived clock + the
`wedged_no_sealed_progress` rung gated by the AND constraint + the reset-on-novelty
cursor + the first-breach auto-requeue-one with telomere escalation. Name later
seams. Keep the local-first boundary.

## Falsifier guidance (attack the v1 proposal)

- **Falsifier 1 (false-kill / correctness lens):** find a legitimately
  slow-but-healthy lane the rung kills (single-final-seal during a long quiet
  computation; a long single tool invocation exceeding `ToolProgressSeconds`; the
  started_at floor crossing budget before first seal; a job_type that never
  seals; restart/requeue mis-fire; the AND degrading to an effective OR).
  Re-introducing #145 CASE-2 artifact-loss is a hard fail.
- **Falsifier 2 (anti-gaming / action-ladder lens):** game the clock (cheap
  manufactured novelty via junk `content_sha256` / minted verdicts / milestone
  advance); break convergence (auto-requeue loop, escalation storm); find a wrong
  budget-counter interaction or an incorrect escalation recovery verb; show the
  derived clock/cursor is restart-inconsistent.

The adjudicator ratifies the **AND no-false-kill** decision and gates clearing on
all four parts being build-bearing, un-gameable, convergent, and
restart-consistent — not merely asserted. A clearing verdict (`accept` /
`accept_with_findings`) requires that with no standing falsifier challenge. This
is the single allowed v1 revision cycle; a second `needs_revision` ends the gate
uncleared and routes to the operator (who spins a fresh `-v2` run with a revising
holder).
