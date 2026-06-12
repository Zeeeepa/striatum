---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: adjudicator-unknown-model-006
workflow: rfc-0119-ratify
run_id: run_b089555bd70cd2dc2dc1d13c3cc35b53
cycle: 1
title: "RFC 0119 ratification gate — final summary: CLEARED with findings (accept_with_findings); four binding constraints C1–C4; proposed decision entry canonical ID D179 (not D178)"
inputs:
  - docs/rfcs/0119/adjudicator/COLLABORATION_LEDGER_cycle_1.md
  - docs/rfcs/0119/ACCEPTANCE.md
  - docs/rfcs/0119/HOLDER_CASE.md
  - docs/rfcs/0119/falsifier_1/FALSIFIER.md
  - docs/rfcs/0119/falsifier_2/FALSIFIER.md
  - docs/rfcs/0119-warm-tier-memory-boundary.md
  - docs/decisions/D178.md
---

# RFC 0119 Ratification Gate — Final Summary

Operator-facing close-out for the RFC 0119 ratification gate
(run `run_b089555bd70cd2dc2dc1d13c3cc35b53`). The authoritative record is the
adjudicator ledger
(`docs/rfcs/0119/adjudicator/COLLABORATION_LEDGER_cycle_1.md`); the committer
synthesis is `docs/rfcs/0119/ACCEPTANCE.md`; the proposed decision entry is
`docs/decisions/D178.md` (canonical ID **D179** — see §3).

## 1. Verdict — CLEARED, with binding findings

**The gate cleared.** The adjudicator ledger recorded `verdict:
accept_with_findings`, with all three posture branches —
`state_transition_dependence`, `durable_provenance`, and `export_class` — at
`cleared_with_constraints`. RFC 0119 is **accepted with findings**.

This is **not** a clean accept and **not** a refusal. It is a
**binding-conditional** clear: the gate clears *only with* four binding
constraints (C1–C4), each `final_review_required: true`. The ledger requires
that **"RFC 0119 must be amended to discharge C1–C4 before a decision record is
filed or Go implementation begins."**

Both falsifiers' decisive objections were verified and ruled **binding** (each
finding `converted_to_constraint`), but the adjudicator ruled them
**dischargeable without weakening the corpus invariants**: the hot-tier read can
move off the claim/worktree-create transactions, and the exhaust policy can
become a strict per-kind allow-list aligned with accepted RFC 0072. That is why
the gate clears with constraints rather than refusing — none of the four
objections forced a state transition to depend on memory, broke a corpus
invariant beyond repair, or eroded durable provenance in a way amendment cannot
fix.

**Honest status of the RFC text:** as of this gate,
`docs/rfcs/0119-warm-tier-memory-boundary.md` is **unamended** — its `Status:`
line is still `proposed` and the text **does not yet discharge C1–C4**. The
acceptance is therefore a conditional clear, not a description of a finished RFC.
The table in §2 maps each constraint to the exact RFC location the operator must
amend to discharge it.

## 2. The four binding constraints and where the RFC must discharge them

The acceptance discharges nothing in the RFC text itself — it records the
verdict and the per-constraint amendment each falsified seam needs. "Where it
must be discharged" below is the location the operator must amend; **none is
discharged in the RFC today.**

| Constraint | Posture / severity | Source finding | Requirement (from the ledger) | Where the RFC must discharge it |
| --- | --- | --- | --- | --- |
| **C1** `C1-HOT-TIER-SEPARATE-CONNECTION` | state_transition_dependence / **critical** | F-HOT-TX | Hot-tier recall must run on a **separate connection**, never the claim transaction; the digest write must be **fail-soft outside the transition**; and a recall-failure guardrail must assert claim **and** worktree-create still commit. | Amend D3 ("Hot tier", RFC `:88`–`:99`): drop the bare "this is **not** a state transition" framing — false, because `buildPacket` is called at `claim.go:229` inside the claim `tx`; require the recall read on a separate connection; make the digest write fail-soft outside the transition; add the C1 guardrail that **injects a recall-read failure and asserts claim + worktree-create still commit**. |
| **C2** `C2-WORKTREE-ORPHAN-GUARD` | state_transition_dependence / high | F-WORKTREE-ORPHAN | The worktree-create digest render must be **ordered and fail-soft** so a render failure cannot leave a dangling git worktree without a `job_worktrees` row. | Amend D3's worktree-create seam (`HandleWorktreeCreate`): order the render **after** the `job_worktrees` row is recorded (or otherwise make render failure non-fatal and non-orphaning). Gate: inject a digest-render failure during worktree create; assert no unrecorded worktree remains. |
| **C3** `C3-EXHAUST-ALLOWLIST` | durable_provenance / **critical** | F-EXHAUST-ALLOWLIST | Exhaust must be an **explicit per-kind allow-list** aligned with RFC 0072, excluding `operator_report`, `decision`, `escalation`, `operator_brief`, `work_plan`, `collaboration_ledger`, and all corpus-export durable-provenance kinds. | Replace the Open-Questions glob default (RFC `:137`–`:141`: "progress_note, operator_report, *_ledger, …") with an explicit per-kind allow-list that is a **strict subset of RFC 0072's blob set, never a glob**; RFC 0119 must explicitly **reconcile with / supersede RFC 0072's boundary table** rather than silently contradict it. Gate: RFC text and decision record list allowed exhaust kinds explicitly and forbid broad ledger globs. |
| **C4** `C4-LANE-TRAJECTORY-CONTRACT` | export_class / high | F-LANE-TRAJECTORY | `lane_trajectory` must explicitly **supersede the transcript-denial guardrail/decision** where required and either define **deterministic redaction-normalization to bytes** or stay **outside** the durable-provenance bundle. | Amend D2 (RFC `:77`–`:86`): name the supersession of the enforced `validateCorpusSourcePath` transcript denial (`redaction.go:24-29`, `:84-87`; `exports.go:408`) **and D028**; specify the deterministic redaction-normalization-to-bytes contract, **or** keep `lane_trajectory` out of the corpus durable-provenance bundle. |

## 3. Proposed decision-log entry — canonical ID **D179** (not D178)

RFC 0119, the holder case, and the gate prompt all name the new decision
**"D178"**, but **`D178` is already an accepted decision** —
`docs/decisions/decision-log.md:36` assigns it to the RFC 0117
`striatum worktree gc` surface, and `D178` is the latest assigned decision ID in
the log. The ready-to-paste entry therefore takes the next free ID, **`D179`**. It
lives at `docs/decisions/D178.md` only because that is the path this workflow's
write scope mandated; **when pasting into `docs/decisions/decision-log.md`, use
`D179`.** Reusing `D178` would collide with an accepted decision — exactly the
failure mode the log's own D119–D123 correction note exists to prevent.

The proposed `D179` row records: **accept RFC 0119** (warm-tier memory boundary +
striatum-native read-only hot tier) **with binding constraints C1–C4**,
authorizing — **conditional on C1–C4 landing in the RFC text first** —
(1) a `spec.md` §"Warm-Tier Memory" pinning the three axes (canonical authority
never the warm tier; index scope = everything incl. durable provenance; eviction
scope = only run exhaust + unsynthesized intermediates leave the git tree);
(2) an optional, default-off, redacted, pull-only-and-deterministic
`lane_trajectory` corpus-export feedstock class; and (3) a read-only
`RecallMemory` projection (Postgres FTS over the daemon's own immutable artifact
stream — no pgvector/ML) feeding a fail-soft, honestly-headed scaffold-time
digest into new worktrees and the operator bootstrap packet, named `recall.*`
(capability `read`, `single_repo`) and **never** `memory.*`. The three corpus
invariants are retained and generalized to any external memory consumer.

**Merge gate:** the `D179` row must land **only together with** the RFC
amendment that discharges C1–C4 (§2) and the `proposed → accepted` status flip
(§4) — not before. The full ready-to-paste row and the C1–C4 discharge checklist
are in `docs/decisions/D178.md`.

## 4. Operator follow-up (owned by the operator after this gate)

No `go/` code lands in this run; this gate files the acceptance synthesis and the
proposed decision entry only. In order, the operator owns:

1. **Amend RFC 0119 to discharge C1–C4** (per §2). The RFC text is the gating
   artifact — nothing below proceeds until all four constraints are green.
2. **Flip the RFC status** `proposed → accepted` on
   `docs/rfcs/0119-warm-tier-memory-boundary.md` and its `docs/rfcs/README.md`
   row (both currently `proposed`). These edits were outside this job's write
   scope; apply them together with the C1–C4 amendment.
3. **Paste the decision entry into `docs/decisions/decision-log.md` as `D179`**
   (not `D178`), together with the status flip and the C1–C4 amendment.
4. **Land the Go hot tier** — the `RecallMemory` projection + scaffold-time
   digest injection + the `docs/reference/command-authority-matrix.md` row
   (`recall.*`, `read`, `single_repo`) + the guardrail tests (including the C1
   recall-failure injection gate and the no-`memory.*`-in-registry assertion) —
   **keeping the three corpus invariants green**:
   - no external-consumer import in daemon source;
   - no `memory.*` in the daemon method registry;
   - every state transition succeeding with the warm tier absent.

Implementation is gated on **both**: C1–C4 discharged in the amended RFC and the
`D179` row landed first; and the three corpus guardrails staying green. Until
both hold, no implementation work begins.
